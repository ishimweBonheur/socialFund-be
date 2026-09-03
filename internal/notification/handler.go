package notification

import (
	"encoding/json"
	"errors"
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

func NewHandler(s *Service, l *slog.Logger) *Handler {
	return &Handler{s: s, l: l}
}
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/{id}/retry", h.retry)
	r.Post("/{id}/read", h.read)
	r.Post("/read-all", h.readAll)
	return r
}
func (h *Handler) MemberRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listMine)
	r.Post("/support-requests", h.support)
	r.Post("/{id}/read", h.read)
	r.Post("/read-all", h.readAll)
	return r
}
func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	l, o := pageValues(r)
	items, err := h.s.List(r.Context(), Filter{UserID: &actor.UserID, Limit: l, Offset: o})
	if err != nil {
		httpx.WriteInternal(w, r, h.l, "list_member_notifications", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
func pageValues(r *http.Request) (int, int) {
	l, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	o, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return l, o
}
func (h *Handler) support(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	var body struct {
		Category       string     `json:"category"`
		Message        string     `json:"message"`
		ContributionID *uuid.UUID `json:"contribution_id"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	err := h.s.SubmitSupportRequest(r.Context(), SupportRequestInput{UserID: actor.UserID, Category: body.Category, Message: body.Message, ContributionID: body.ContributionID})
	if errors.Is(err, ErrInvalidSupportRequest) {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.l, "submit_support_request", err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": map[string]string{"status": "SENT"}})
}
func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	if err = h.s.MarkRead(r.Context(), actor.UserID, id); err != nil {
		if err == pgx.ErrNoRows {
			httpx.WriteError(w, httpx.NewError(404, "NOTIFICATION_NOT_FOUND", "Notification was not found"))
			return
		}
		httpx.WriteInternal(w, r, h.l, "read_notification", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	if err := h.s.MarkAllRead(r.Context(), actor.UserID); err != nil {
		httpx.WriteInternal(w, r, h.l, "read_all_notifications", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	q := r.URL.Query()
	l, _ := strconv.Atoi(q.Get("limit"))
	o, _ := strconv.Atoi(q.Get("offset"))
	uid := &actor.UserID
	if strings.EqualFold(q.Get("scope"), "all") {
		uid = nil
	}
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
