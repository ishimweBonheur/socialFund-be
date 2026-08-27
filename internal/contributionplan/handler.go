package contributionplan

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"socialfund/internal/httpx"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/users/{userID}/active", h.active)
	r.Post("/", h.create)
	return r
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
func jsonResponse(w http.ResponseWriter, status int, v any) { httpx.WriteJSON(w, status, v) }
