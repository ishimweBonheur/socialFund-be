package auth

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
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
	r.Post("/google", h.google)
	return r
}
func (h *Handler) google(w http.ResponseWriter, r *http.Request) {
	var request GoogleLoginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	response, err := h.service.LoginGoogle(r.Context(), request.Credential)
	if err != nil {
		if _, ok := err.(*httpx.Error); ok {
			httpx.WriteError(w, err)
			return
		}
		httpx.WriteInternal(w, r, h.logger, "google_login", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}
