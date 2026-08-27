package assistance

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Routes() chi.Router      { r := chi.NewRouter(); r.Post("/{id}/pay", h.pay); return r }
func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid assistance request id", 400)
		return
	}
	var in DisbursementInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	in.AssistanceRequestID = id
	if err = h.service.Pay(r.Context(), in); err != nil {
		if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidAmount) {
			http.Error(w, err.Error(), 409)
			return
		}
		http.Error(w, "operation failed", 500)
		return
	}
	w.WriteHeader(204)
}
