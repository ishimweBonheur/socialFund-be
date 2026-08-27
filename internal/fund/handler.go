package fund

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
	r.Get("/summary", h.summary)
	r.Get("/transactions", h.transactions)
	return r
}
func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.repo.Summary(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "fund_summary", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": s})
}
func (h *Handler) transactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	l, _ := strconv.Atoi(q.Get("limit"))
	o, _ := strconv.Atoi(q.Get("offset"))
	if l < 1 || l > 100 {
		l = 20
	}
	var uid *uuid.UUID
	if raw := q.Get("user_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(w, httpx.ErrValidation)
			return
		}
		uid = &id
	}
	f := Filter{Type: strings.ToUpper(q.Get("type")), Direction: strings.ToUpper(q.Get("direction")), DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), UserID: uid, Limit: l, Offset: o}
	items, err := h.repo.List(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "fund_transactions", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": l, "offset": o})
}
