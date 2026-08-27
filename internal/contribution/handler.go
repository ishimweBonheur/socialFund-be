package contribution

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
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
			http.Error(w, "invalid JSON", 400)
		}
		return
	}
	if err := h.service.Approve(r.Context(), in); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	var in RejectionInput
	if !decodeID(w, r, &in.ContributionID) || json.NewDecoder(r.Body).Decode(&in) != nil {
		if in.ContributionID != uuid.Nil {
			http.Error(w, "invalid JSON", 400)
		}
		return
	}
	if err := h.service.Reject(r.Context(), in); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(204)
}
func decodeID(w http.ResponseWriter, r *http.Request, target *uuid.UUID) bool {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid contribution id", 400)
		return false
	}
	*target = id
	return true
}
func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidAmount) {
		http.Error(w, err.Error(), 409)
		return
	}
	http.Error(w, "operation failed", 500)
}
