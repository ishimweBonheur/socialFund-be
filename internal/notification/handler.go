package notification

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
	"strconv"
	"strings"
)

type Handler struct {
	s *Service
	l *slog.Logger
}

func NewHandler(s *Service, l *slog.Logger) *Handler { return &Handler{s: s, l: l} }
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/{id}/retry", h.retry)
	return r
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	l, _ := strconv.Atoi(q.Get("limit"))
	o, _ := strconv.Atoi(q.Get("offset"))
	var uid *uuid.UUID
	if raw := q.Get("user_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(w, httpx.ErrValidation)
			return
		}
		uid = &id
	}
	items, err := h.s.List(r.Context(), Filter{Status: strings.ToUpper(q.Get("status")), Type: strings.ToUpper(q.Get("type")), UserID: uid, DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), Limit: l, Offset: o})
	if err != nil {
		httpx.WriteInternal(w, r, h.l, "list_notifications", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	actor, _ := httpx.IdentityFrom(r.Context())
	if err = h.s.Retry(r.Context(), actor.UserID, id); err != nil {
		if err == pgx.ErrNoRows {
			httpx.WriteError(w, httpx.NewError(409, "NOTIFICATION_NOT_RETRYABLE", "Notification is not failed or was not found"))
			return
		}
		httpx.WriteInternal(w, r, h.l, "retry_notification", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]any{"id": id, "status": "PENDING"}})
}
