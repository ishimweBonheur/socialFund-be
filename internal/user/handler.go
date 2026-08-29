package user

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}", h.get)
	return r
}
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.createMember)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Post("/{id}/suspend", h.suspend)
	r.Post("/{id}/activate", h.activate)
	return r
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	l, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	o, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := h.service.List(r.Context(), ListFilter{Status: strings.ToUpper(r.URL.Query().Get("status")), Role: strings.ToUpper(r.URL.Query().Get("role")), Search: r.URL.Query().Get("search"), Limit: l, Offset: o})
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "list_users", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var in UpdateInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	out, err := h.service.Update(r.Context(), actor.UserID, id, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": out})
}
func (h *Handler) suspend(w http.ResponseWriter, r *http.Request) {
	h.status(w, r, false)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	h.status(w, r, true)
}
func (h *Handler) status(w http.ResponseWriter, r *http.Request, activate bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	if err = h.service.ChangeStatus(r.Context(), actor.UserID, id, activate); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]any{"id": id, "status": map[bool]string{true: "ACTIVE", false: "SUSPENDED"}[activate]}})
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
