package contribution

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{id}/approve", h.approve)
	r.Post("/{id}/reject", h.reject)
	return r
}
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	var in ApprovalInput
	if !decodeID(w, r, &in.ContributionID) || json.NewDecoder(r.Body).Decode(&in) != nil {
		if in.ContributionID != uuid.Nil {
			httpx.WriteError(w, httpx.ErrValidation)
		}
		return
	}
	if err := h.service.Approve(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	var in RejectionInput
	if !decodeID(w, r, &in.ContributionID) || json.NewDecoder(r.Body).Decode(&in) != nil {
		if in.ContributionID != uuid.Nil {
			httpx.WriteError(w, httpx.ErrValidation)
		}
		return
	}
	if err := h.service.Reject(r.Context(), in); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func decodeID(w http.ResponseWriter, r *http.Request, target *uuid.UUID) bool {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return false
	}
	*target = id
	return true
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidAmount) {
		httpx.WriteError(w, httpx.NewError(409, "CONTRIBUTION_ALREADY_PROCESSED", "The contribution cannot be processed"))
		return
	}
	httpx.WriteInternal(w, r, h.logger, "process_contribution", err)
}
