package contributionplan

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
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
		http.Error(w, "invalid user id", 400)
		return
	}
	p, err := h.service.GetActive(r.Context(), id)
	if err != nil {
		http.Error(w, "active plan not found", 404)
		return
	}
	jsonResponse(w, 200, p)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var p ContributionPlan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	created, err := h.service.Create(r.Context(), p)
	if err != nil {
		http.Error(w, "could not create plan", 400)
		return
	}
	jsonResponse(w, 201, created)
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
