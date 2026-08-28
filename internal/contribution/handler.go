package contribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"socialfund/internal/httpx"
)

const maxProofBytes = 10 << 20

type Handler struct {
	service *Service
	storage FileStorage
	logger  *slog.Logger
}

func (h *Handler) ReviewProof(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	c, err := h.service.ValidateProofToken(r.Context(), id, r.URL.Query().Get("token"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if c.ProofURL == nil {
		httpx.WriteError(w, httpx.NewError(404, "PROOF_REQUIRED", "Contribution has no proof"))
		return
	}
	reader, filename, err := h.storage.Open(r.Context(), *c.ProofURL)
	if err != nil {
		httpx.WriteError(w, httpx.NewError(503, "STORAGE_UNAVAILABLE", "Proof storage is temporarily unavailable"))
		return
	}
	defer reader.Close()
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err = io.Copy(w, reader); err != nil {
		h.logger.WarnContext(r.Context(), "stream proof failed", "error", err)
	}
}

func NewHandler(service *Service, storage FileStorage, logger *slog.Logger) *Handler {
	return &Handler{service: service, storage: storage, logger: logger}
}
func (h *Handler) Routes(middlewares ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listMine)
	r.Get("/outstanding", h.outstanding)
	r.Get("/{id}", h.get)
	if len(middlewares) > 1 {
		r.With(middlewares[1]).Post("/{id}/proof", h.proof)
	} else {
		r.Post("/{id}/proof", h.proof)
	}
	r.Get("/{id}/proof", h.proofURL)
	if len(middlewares) > 0 {
		r.With(middlewares[0]).Post("/{id}/approve", h.approve)
		r.With(middlewares[0]).Post("/{id}/reject", h.reject)
	}
	return r
}
func (h *Handler) proofURL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	c, err := h.service.GetFor(r.Context(), id, actor.UserID, actor.Role == "ADMIN")
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if c.ProofURL == nil {
		httpx.WriteError(w, httpx.NewError(404, "PROOF_REQUIRED", "Contribution has no proof"))
		return
	}
	url, err := h.storage.SignedURL(r.Context(), *c.ProofURL, 5*time.Minute)
	if err != nil {
		httpx.WriteError(w, httpx.NewError(503, "STORAGE_UNAVAILABLE", "Proof storage is temporarily unavailable"))
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]any{"url": url, "expires_in": 300}})
}
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/pending", h.pending)
	return r
}
func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	var in ReviewTokenRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	out, err := h.service.ValidateReviewToken(r.Context(), id, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": out})
}
func pagination(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpx.IdentityFrom(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.ErrUnauthorized)
		return
	}
	l, o := pagination(r)
	items, err := h.service.ListMine(r.Context(), actor.UserID, l, o)
	if err != nil {
		h.internal(w, r, "list_contributions", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
func (h *Handler) outstanding(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	value, err := h.service.Outstanding(r.Context(), actor.UserID)
	if err != nil {
		h.internal(w, r, "contribution_outstanding", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": value})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	item, err := h.service.GetFor(r.Context(), id, actor.UserID, actor.Role == "ADMIN")
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": item})
}
func (h *Handler) proof(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, maxProofBytes+(1<<20))
	if err = r.ParseMultipartForm(maxProofBytes); err != nil {
		httpx.WriteError(w, httpx.NewError(413, "PROOF_TOO_LARGE", "Proof must not exceed 10 MB"))
		return
	}
	amount, err := decimal.NewFromString(r.FormValue("amount"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	file, header, err := r.FormFile("proof")
	if err != nil {
		httpx.WriteError(w, httpx.NewError(400, "PROOF_REQUIRED", "A payment proof is required"))
		return
	}
	defer file.Close()
	ext, ok := proofExtension(header, file)
	if !ok {
		httpx.WriteError(w, httpx.NewError(400, "INVALID_PROOF_TYPE", "Proof must be a JPG, PNG, or PDF"))
		return
	}
	url, err := h.storage.Save(r.Context(), ext, file)
	if err != nil {
		h.internal(w, r, "save_proof", err)
		return
	}
	in := ProofInput{ContributionID: id, UserID: actor.UserID, Amount: amount, PaymentMethod: strings.ToUpper(r.FormValue("payment_method")), TransactionReference: strings.TrimSpace(r.FormValue("transaction_reference")), ProofURL: url}
	if err = h.service.SubmitProof(r.Context(), in); err != nil {
		if cleanupErr := h.storage.Delete(r.Context(), url); cleanupErr != nil {
			h.logger.WarnContext(r.Context(), "proof cleanup failed", "operation", "delete_orphaned_proof", "error", cleanupErr)
		}
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]string{"status": "PENDING", "proof_url": url}})
}
func proofExtension(header *multipart.FileHeader, file multipart.File) (string, bool) {
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	_, _ = file.Seek(0, 0)
	allowed := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "application/pdf": ".pdf"}
	ext, ok := allowed[http.DetectContentType(buf[:n])]
	if !ok {
		return "", false
	}
	provided := strings.ToLower(filepath.Ext(header.Filename))
	if provided == ".jpeg" {
		provided = ".jpg"
	}
	return ext, provided == ext
}
func (h *Handler) pending(w http.ResponseWriter, r *http.Request) {
	l, o := pagination(r)
	items, err := h.service.ListPending(r.Context(), l, o)
	if err != nil {
		h.internal(w, r, "list_pending_contributions", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in ApprovalInput
	if r.ContentLength > 0 && json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.ContributionID = id
	in.AdminID = actor.UserID
	if err = h.service.Approve(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in RejectionInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.ContributionID = id
	in.AdminID = actor.UserID
	if err = h.service.Reject(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.WriteError(w, httpx.NewError(404, "CONTRIBUTION_NOT_FOUND", "Contribution was not found"))
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, httpx.ErrForbidden)
	case errors.Is(err, ErrInvalidAmount):
		httpx.WriteError(w, httpx.NewError(409, "INVALID_PAYMENT_AMOUNT", "Payment must cover the full amount due"))
	case errors.Is(err, ErrProofRequired):
		httpx.WriteError(w, httpx.NewError(409, "PROOF_REQUIRED", "Payment proof is required"))
	case errors.Is(err, ErrInvalidState):
		httpx.WriteError(w, httpx.NewError(409, "INVALID_STATUS_TRANSITION", "Contribution cannot be processed in its current state"))
	default:
		h.internal(w, r, "process_contribution", err)
	}
}
func (h *Handler) internal(w http.ResponseWriter, r *http.Request, operation string, err error) {
	httpx.WriteInternal(w, r, h.logger, operation, err)
}
