package fund

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
	"strconv"
	"strings"
	"time"
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
	r.Get("/statement.pdf", h.adminStatement)
	return r
}
func (h *Handler) MemberRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/summary", h.memberSummary)
	r.Get("/transactions", h.memberTransactions)
	r.Get("/statement.pdf", h.memberStatement)
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
	f, ok := filterFromRequest(w, r, true)
	if !ok {
		return
	}
	items, err := h.repo.List(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "fund_transactions", err)
		return
	}
	total, err := h.repo.Count(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "fund_transactions_count", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": items, "limit": f.Limit, "offset": f.Offset, "total": total})
}

func (h *Handler) memberSummary(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	s, err := h.repo.SummaryForUser(r.Context(), actor.UserID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "member_fund_summary", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": s})
}

func (h *Handler) memberTransactions(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	f, ok := filterFromRequest(w, r, false)
	if !ok {
		return
	}
	f.UserID = &actor.UserID
	items, err := h.repo.List(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "member_fund_transactions", err)
		return
	}
	total, err := h.repo.Count(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "member_fund_transactions_count", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "limit": f.Limit, "offset": f.Offset, "total": total})
}

func filterFromRequest(w http.ResponseWriter, r *http.Request, allowUser bool) (Filter, bool) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	f := Filter{Type: strings.ToUpper(q.Get("type")), Direction: strings.ToUpper(q.Get("direction")), DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), Limit: limit, Offset: offset}
	for _, value := range []string{f.DateFrom, f.DateTo} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				httpx.WriteError(w, httpx.ErrValidation)
				return Filter{}, false
			}
		}
	}
	if allowUser && q.Get("user_id") != "" {
		id, err := uuid.Parse(q.Get("user_id"))
		if err != nil {
			httpx.WriteError(w, httpx.ErrValidation)
			return Filter{}, false
		}
		f.UserID = &id
	}
	return f, true
}

func (h *Handler) memberStatement(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	f, ok := filterFromRequest(w, r, false)
	if !ok {
		return
	}
	f.UserID = &actor.UserID
	h.writeStatement(w, r, f, "Personal transaction statement")
}
func (h *Handler) adminStatement(w http.ResponseWriter, r *http.Request) {
	f, ok := filterFromRequest(w, r, true)
	if !ok {
		return
	}
	h.writeStatement(w, r, f, "Fund transaction statement")
}
func (h *Handler) writeStatement(w http.ResponseWriter, r *http.Request, f Filter, title string) {
	f.Limit, f.Offset = 5000, 0
	items, err := h.repo.List(r.Context(), f)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "fund_statement", err)
		return
	}
	reference := "STMT-" + strings.ToUpper(uuid.NewString()[:8])
	pdf := StatementPDF(title, reference, f.DateFrom, f.DateTo, items)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="transaction-statement-`+time.Now().UTC().Format("20060102")+`.pdf"`)
	w.Header().Set("X-Statement-Reference", reference)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
