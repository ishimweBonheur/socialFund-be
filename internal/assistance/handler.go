package assistance

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"socialfund/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}
func (h *Handler) Routes(adminOnly ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.listMine)
	if len(adminOnly) > 0 {
		r.With(adminOnly[0]).Post("/{id}/approve", h.approve)
		r.With(adminOnly[0]).Post("/{id}/reject", h.reject)
		r.With(adminOnly[0]).Post("/{id}/pay", h.pay)
	}
	return r
}
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listAdmin)
	return r
}
func page(r *http.Request) (int, int) {
	l, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	o, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if l < 1 || l > 100 {
		l = 20
	}
	if o < 0 {
		o = 0
	}
	return l, o
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	var in CreateInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.UserID = actor.UserID
	out, err := h.service.Create(r.Context(), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 201, map[string]any{"data": out})
}
func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	l, o := page(r)
	f := assistanceFilter(r, l, o)
	items, total, err := h.service.ListMine(r.Context(), f, actor.UserID)
	if err != nil {
		h.internal(w, r, "list_assistance", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o, "total": total})
}
func (h *Handler) listAdmin(w http.ResponseWriter, r *http.Request) {
	l, o := page(r)
	f := assistanceFilter(r, l, o)
	items, total, err := h.service.ListAdmin(r.Context(), f)
	if err != nil {
		h.internal(w, r, "list_admin_assistance", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o, "total": total})
}
func assistanceFilter(r *http.Request, limit, offset int) ListFilter {
	q := r.URL.Query()
	return ListFilter{Status: strings.ToUpper(q.Get("status")), Search: strings.TrimSpace(q.Get("search")), DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), AmountMin: q.Get("amount_min"), AmountMax: q.Get("amount_max"), Limit: limit, Offset: offset}
}
func requestID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return uuid.Nil, false
	}
	return id, true
}
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in ApprovalInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.AssistanceRequestID = id
	in.AdminID = actor.UserID
	if err := h.service.Approve(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in RejectionInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.AssistanceRequestID = id
	in.AdminID = actor.UserID
	if err := h.service.Reject(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in DisbursementInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.AssistanceRequestID = id
	in.AdminID = actor.UserID
	if err := h.service.Pay(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.WriteError(w, httpx.NewError(404, "ASSISTANCE_REQUEST_NOT_FOUND", "Assistance request was not found"))
	case errors.Is(err, ErrInvalidAmount):
		httpx.WriteError(w, httpx.NewError(409, "INVALID_PAYMENT_AMOUNT", "The requested amount is invalid"))
	case errors.Is(err, ErrInvalidReason):
		httpx.WriteError(w, httpx.ErrValidation)
	case errors.Is(err, ErrInsufficientFunds):
		httpx.WriteError(w, httpx.NewError(409, "INSUFFICIENT_FUND_BALANCE", "The fund balance is insufficient"))
	case errors.Is(err, ErrInvalidState):
		httpx.WriteError(w, httpx.NewError(409, "ASSISTANCE_ALREADY_PROCESSED", "Assistance request cannot be processed in its current state"))
	default:
		h.internal(w, r, "process_assistance", err)
	}
}
func (h *Handler) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	httpx.WriteInternal(w, r, h.logger, op, err)
}
