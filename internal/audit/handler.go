package audit

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
	"strconv"
	"strings"
)

type Handler struct {
	repo   Reader
	logger *slog.Logger
}

func NewHandler(repo Reader, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	l, _ := strconv.Atoi(q.Get("limit"))
	o, _ := strconv.Atoi(q.Get("offset"))
	if l < 1 || l > 100 {
		l = 20
	}
	uid, ok := parseOptional(q.Get("user_id"))
	if !ok {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	entity, ok := parseOptional(q.Get("entity_id"))
	if !ok {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	filter := Filter{UserID: uid, Action: strings.ToUpper(q.Get("action")), EntityType: strings.ToUpper(q.Get("entity_type")), EntityID: entity, DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), Limit: l, Offset: o}
	items, err := h.repo.List(r.Context(), filter)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "audit_logs", err)
		return
	}
	total, err := h.repo.Count(r.Context(), filter)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "audit_logs_count", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o, "total": total})
}
func parseOptional(raw string) (*uuid.UUID, bool) {
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	return &id, err == nil
}
