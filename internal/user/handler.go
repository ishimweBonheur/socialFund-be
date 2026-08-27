package user

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"socialfund/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}
func (h *Handler) Routes() chi.Router { r := chi.NewRouter(); r.Get("/{id}", h.get); return r }
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.createMember)
	return r
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	u, err := h.service.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, u)
}
func (h *Handler) createMember(w http.ResponseWriter, r *http.Request) {
	identity, ok := httpx.IdentityFrom(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.ErrUnauthorized)
		return
	}
	var request CreateMemberRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	created, err := h.service.CreateMember(r.Context(), identity.UserID, request)
	if err != nil {
		if _, ok := err.(*httpx.Error); ok {
			httpx.WriteError(w, err)
			return
		}
		httpx.WriteInternal(w, r, h.logger, "create_member", err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, created)
}
