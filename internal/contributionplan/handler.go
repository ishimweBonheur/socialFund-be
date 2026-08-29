package contributionplan

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"socialfund/internal/httpx"
	"strconv"
	"strings"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/users/{userID}/active", h.active)
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	return r
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var active *bool
	if raw := strings.TrimSpace(q.Get("active")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			httpx.WriteError(w, httpx.ErrValidation)
			return
		}
		active = &value
	}
	items, total, err := h.service.List(r.Context(), strings.TrimSpace(q.Get("search")), active, limit, offset)
	if err != nil {
		httpx.WriteError(w, httpx.ErrInternal)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": limit, "offset": offset, "total": total})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	var p ContributionPlan
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	out, err := h.service.Update(r.Context(), actor.UserID, id, p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": out})
}
func (h *Handler) active(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	p, err := h.service.GetActive(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, httpx.NewError(404, "CONTRIBUTION_PLAN_NOT_FOUND", "Contribution plan was not found"))
		return
	}
	jsonResponse(w, 200, p)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var p ContributionPlan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	created, err := h.service.Create(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	jsonResponse(w, 201, created)
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	httpx.WriteJSON(w, status, v)
}
