package assistance

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
func (h *Handler) Routes() chi.Router { r := chi.NewRouter(); r.Post("/{id}/pay", h.pay); return r }
func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	var in DisbursementInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	in.AssistanceRequestID = id
	if err = h.service.Pay(r.Context(), in); err != nil {
		if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidAmount) {
			httpx.WriteError(w, httpx.NewError(409, "ASSISTANCE_CANNOT_BE_PAID", "The assistance request cannot be paid"))
			return
		}
		httpx.WriteInternal(w, r, h.logger, "pay_assistance", err)
		return
	}
	w.WriteHeader(204)
}
