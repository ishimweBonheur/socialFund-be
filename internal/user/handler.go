package user

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
	r.Get("/{id}", h.get)
	r.Post("/", h.create)
	return r
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid user id", 400)
		return
	}
	u, err := h.service.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "user not found", 404)
		return
	}
	writeJSON(w, 200, u)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	created, err := h.service.Create(r.Context(), u)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 201, created)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
