package dashboard

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
)

type Handler struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewHandler(db *pgxpool.Pool, l *slog.Logger) *Handler { return &Handler{db: db, logger: l} }
func (h *Handler) MemberRoutes() chi.Router                { r := chi.NewRouter(); r.Get("/", h.member); return r }
func (h *Handler) AdminRoutes() chi.Router                 { r := chi.NewRouter(); r.Get("/", h.admin); return r }

type MemberSummary struct {
	TotalContributed   decimal.Decimal  `json:"total_contributed"`
	OutstandingAmount  decimal.Decimal  `json:"outstanding_amount"`
	LateFeesTotal      decimal.Decimal  `json:"late_fees_total"`
	ApprovedCount      int              `json:"approved_count"`
	PendingCount       int              `json:"pending_count"`
	OverdueCount       int              `json:"overdue_count"`
	RejectedCount      int              `json:"rejected_count"`
	ContributionRate   decimal.Decimal  `json:"contribution_rate"`
	NextDueDate        *string          `json:"next_due_date,omitempty"`
	NextExpectedAmount *decimal.Decimal `json:"next_expected_amount,omitempty"`
	PlanAmount         *decimal.Decimal `json:"plan_amount,omitempty"`
	PlanFrequency      *string          `json:"plan_frequency,omitempty"`
	ReminderFrequency  *string          `json:"reminder_frequency,omitempty"`
	PlanLateFee        *decimal.Decimal `json:"plan_late_fee,omitempty"`
}

func (h *Handler) member(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.IdentityFrom(r.Context())
	var s MemberSummary
	err := h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(paid_amount) FILTER(WHERE status='APPROVED'),0),COALESCE(SUM(expected_amount+late_fee_amount) FILTER(WHERE status IN('OVERDUE','REJECTED')),0),COALESCE(SUM(late_fee_amount),0),COUNT(*) FILTER(WHERE status='APPROVED'),COUNT(*) FILTER(WHERE status='PENDING'),COUNT(*) FILTER(WHERE status='OVERDUE'),COUNT(*) FILTER(WHERE status='REJECTED'),COALESCE(ROUND(100*COUNT(*) FILTER(WHERE status='APPROVED')/NULLIF(COUNT(*) FILTER(WHERE status IN('APPROVED','OVERDUE','REJECTED')),0),2),0) FROM contributions WHERE user_id=$1`, actor.UserID).Scan(&s.TotalContributed, &s.OutstandingAmount, &s.LateFeesTotal, &s.ApprovedCount, &s.PendingCount, &s.OverdueCount, &s.RejectedCount, &s.ContributionRate)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "member_dashboard", err)
		return
	}
	_ = h.db.QueryRow(r.Context(), `SELECT due_date::text,expected_amount FROM contributions WHERE user_id=$1 AND status IN('UPCOMING','DUE') ORDER BY due_date LIMIT 1`, actor.UserID).Scan(&s.NextDueDate, &s.NextExpectedAmount)
	_ = h.db.QueryRow(r.Context(), `SELECT amount,frequency,reminder_frequency,late_fee_percentage FROM contribution_plans WHERE user_id=$1 AND is_active`, actor.UserID).Scan(&s.PlanAmount, &s.PlanFrequency, &s.ReminderFrequency, &s.PlanLateFee)
	httpx.WriteJSON(w, 200, map[string]any{"data": s})
}

type AdminSummary struct {
	MembersTotal, MembersActive, MembersInactive, MembersSuspended int
	ExpectedMonth, CollectedMonth, Outstanding                     decimal.Decimal
	PendingApprovals, OverdueMembers                               int
	FundIn, FundOut, FundBalance                                   decimal.Decimal
	AssistancePending, AssistanceApproved, NotificationsFailed     int
}

func (h *Handler) admin(w http.ResponseWriter, r *http.Request) {
	var s AdminSummary
	err := h.db.QueryRow(r.Context(), `SELECT COUNT(*) FILTER(WHERE role='MEMBER'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='ACTIVE'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='INACTIVE'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='SUSPENDED') FROM users`).Scan(&s.MembersTotal, &s.MembersActive, &s.MembersInactive, &s.MembersSuspended)
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(expected_amount) FILTER(WHERE date_trunc('month',due_date)=date_trunc('month',CURRENT_DATE)),0),COALESCE(SUM(paid_amount) FILTER(WHERE status='APPROVED' AND date_trunc('month',approved_at)=date_trunc('month',CURRENT_DATE)),0),COALESCE(SUM(expected_amount+late_fee_amount) FILTER(WHERE status IN('OVERDUE','REJECTED')),0),COUNT(*) FILTER(WHERE status='PENDING'),COUNT(DISTINCT user_id) FILTER(WHERE status='OVERDUE') FROM contributions`).Scan(&s.ExpectedMonth, &s.CollectedMonth, &s.Outstanding, &s.PendingApprovals, &s.OverdueMembers)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(amount) FILTER(WHERE direction='IN'),0),COALESCE(SUM(amount) FILTER(WHERE direction='OUT'),0),COALESCE(SUM(CASE direction WHEN 'IN' THEN amount ELSE -amount END),0) FROM fund_transactions`).Scan(&s.FundIn, &s.FundOut, &s.FundBalance)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FILTER(WHERE status='PENDING'),COUNT(*) FILTER(WHERE status='APPROVED') FROM assistance_requests`).Scan(&s.AssistancePending, &s.AssistanceApproved)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM notifications WHERE status='FAILED'`).Scan(&s.NotificationsFailed)
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "admin_dashboard", err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": s})
}
