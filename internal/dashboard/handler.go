package dashboard

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"log/slog"
	"net/http"
	"socialfund/internal/httpx"
	"strconv"
	"time"
)

type Handler struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewHandler(db *pgxpool.Pool, l *slog.Logger) *Handler {
	return &Handler{db: db, logger: l}
}
func (h *Handler) MemberRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.member)
	return r
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.admin)
	return r
}

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
	ExpectedMonth      decimal.Decimal  `json:"expected_this_month"`
	PaidMonth          decimal.Decimal  `json:"paid_this_month"`
	GracePeriodDays    *int             `json:"grace_period_days,omitempty"`
	EffectiveOverdueAt *string          `json:"effective_overdue_date,omitempty"`
}

type MonthlyAmount struct {
	Month     string          `json:"month"`
	Expected  decimal.Decimal `json:"expected"`
	Collected decimal.Decimal `json:"collected"`
	Paid      decimal.Decimal `json:"paid,omitempty"`
	Inflow    decimal.Decimal `json:"inflow,omitempty"`
	Outflow   decimal.Decimal `json:"outflow,omitempty"`
}
type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type RecentContribution struct {
	DueDate  string          `json:"due_date"`
	Expected decimal.Decimal `json:"expected"`
	Paid     decimal.Decimal `json:"paid"`
	Status   string          `json:"status"`
}
type FrequencySummary struct {
	Frequency   string          `json:"frequency"`
	Expected    decimal.Decimal `json:"expected"`
	Collected   decimal.Decimal `json:"collected"`
	Outstanding decimal.Decimal `json:"outstanding"`
	Total       int             `json:"total"`
	Approved    int             `json:"approved"`
	Pending     int             `json:"pending"`
	Overdue     int             `json:"overdue"`
	Rejected    int             `json:"rejected"`
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
	_ = h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(expected_amount) FILTER(WHERE status<>'FROZEN' AND date_trunc('month',due_date)=date_trunc('month',CURRENT_DATE)),0),COALESCE(SUM(paid_amount) FILTER(WHERE status='APPROVED' AND date_trunc('month',payment_date)=date_trunc('month',CURRENT_DATE)),0) FROM contributions WHERE user_id=$1`, actor.UserID).Scan(&s.ExpectedMonth, &s.PaidMonth)
	_ = h.db.QueryRow(r.Context(), `SELECT p.grace_period_days,(c.due_date+p.grace_period_days)::text FROM contributions c JOIN contribution_plans p ON p.id=c.contribution_plan_id WHERE c.user_id=$1 AND c.status IN('UPCOMING','DUE') ORDER BY c.due_date LIMIT 1`, actor.UserID).Scan(&s.GracePeriodDays, &s.EffectiveOverdueAt)
	months := dashboardMonths(r)
	history := make([]MonthlyAmount, 0)
	rows, queryErr := h.db.Query(r.Context(), `WITH months(period_start) AS (SELECT generate_series(date_trunc('month',CURRENT_DATE)-($2-1)*INTERVAL '1 month',date_trunc('month',CURRENT_DATE),INTERVAL '1 month')) SELECT to_char(m.period_start,'Mon YYYY'),COALESCE(SUM(c.expected_amount) FILTER(WHERE c.status<>'FROZEN'),0),COALESCE(SUM(c.paid_amount) FILTER(WHERE c.status='APPROVED'),0) FROM months m LEFT JOIN contributions c ON c.user_id=$1 AND date_trunc('month',c.due_date)=m.period_start GROUP BY m.period_start ORDER BY m.period_start`, actor.UserID, months)
	if queryErr == nil {
		defer rows.Close()
		for rows.Next() {
			var point MonthlyAmount
			if rows.Scan(&point.Month, &point.Expected, &point.Paid) == nil {
				history = append(history, point)
			}
		}
	}
	paymentStatuses := h.namedCounts(r, `SELECT status,COUNT(*) FROM contributions WHERE user_id=$1 GROUP BY status ORDER BY status`, actor.UserID)
	recent := make([]RecentContribution, 0)
	recentRows, recentErr := h.db.Query(r.Context(), `SELECT due_date::text,expected_amount,COALESCE(paid_amount,0),status FROM contributions WHERE user_id=$1 ORDER BY due_date DESC LIMIT 5`, actor.UserID)
	if recentErr == nil {
		defer recentRows.Close()
		for recentRows.Next() {
			var item RecentContribution
			if recentRows.Scan(&item.DueDate, &item.Expected, &item.Paid, &item.Status) == nil {
				recent = append(recent, item)
			}
		}
	}
	frequencySummary, frequencyErr := h.frequencySummary(r, &actor.UserID)
	if frequencyErr != nil {
		httpx.WriteInternal(w, r, h.logger, "member_dashboard_frequency_summary", frequencyErr)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]any{"summary": s, "contribution_history": history, "contribution_by_frequency": frequencySummary, "payment_statuses": paymentStatuses, "recent_contributions": recent}})
}

type AdminSummary struct {
	MembersTotal        int             `json:"members_total"`
	MembersActive       int             `json:"members_active"`
	MembersInactive     int             `json:"members_inactive"`
	MembersSuspended    int             `json:"members_suspended"`
	ExpectedMonth       decimal.Decimal `json:"expected_this_month"`
	CollectedMonth      decimal.Decimal `json:"collected_this_month"`
	Outstanding         decimal.Decimal `json:"outstanding_amount"`
	PendingApprovals    int             `json:"pending_contributions"`
	OverdueMembers      int             `json:"overdue_members"`
	FundIn              decimal.Decimal `json:"fund_inflow"`
	FundOut             decimal.Decimal `json:"fund_outflow"`
	FundBalance         decimal.Decimal `json:"fund_balance"`
	NotificationsFailed int             `json:"notifications_failed"`
}

func (h *Handler) admin(w http.ResponseWriter, r *http.Request) {
	targetMonth := time.Now().UTC().Format("2006-01")
	if raw := r.URL.Query().Get("target_month"); raw != "" {
		if _, err := time.Parse("2006-01", raw); err != nil {
			httpx.WriteError(w, httpx.ErrValidation)
			return
		}
		targetMonth = raw
	}
	var s AdminSummary
	err := h.db.QueryRow(r.Context(), `SELECT COUNT(*) FILTER(WHERE role='MEMBER'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='ACTIVE'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='INACTIVE'),COUNT(*) FILTER(WHERE role='MEMBER' AND status='SUSPENDED') FROM users`).Scan(&s.MembersTotal, &s.MembersActive, &s.MembersInactive, &s.MembersSuspended)
	if err == nil {
		err = h.db.QueryRow(r.Context(), `WITH bounds AS (
			SELECT to_date($1,'YYYY-MM')::date AS month_start,
			       (to_date($1,'YYYY-MM')+INTERVAL '1 month'-INTERVAL '1 day')::date AS month_end
		)
		SELECT COALESCE(SUM(p.amount * CASE p.frequency
			WHEN 'DAILY' THEN EXTRACT(DAY FROM b.month_end)::numeric
			WHEN 'WEEKLY' THEN 4
			WHEN 'MONTHLY' THEN 1
			WHEN 'CUSTOM' THEN CEIL(EXTRACT(DAY FROM b.month_end)::numeric/GREATEST(COALESCE(p.interval_value,1),1))
			ELSE 0
		END),0)
		FROM contribution_plans p
		JOIN users u ON u.id=p.user_id AND u.status='ACTIVE'
		CROSS JOIN bounds b
		WHERE p.is_active
		  AND p.start_date <= b.month_end
		  AND (p.end_date IS NULL OR p.end_date >= b.month_start)`, targetMonth).Scan(&s.ExpectedMonth)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(paid_amount) FILTER(WHERE status='APPROVED' AND date_trunc('month',approved_at)=date_trunc('month',to_date($1,'YYYY-MM'))),0),COALESCE(SUM(expected_amount+late_fee_amount) FILTER(WHERE status IN('OVERDUE','REJECTED')),0),COUNT(*) FILTER(WHERE status='PENDING'),COUNT(DISTINCT user_id) FILTER(WHERE status='OVERDUE') FROM contributions`, targetMonth).Scan(&s.CollectedMonth, &s.Outstanding, &s.PendingApprovals, &s.OverdueMembers)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(amount) FILTER(WHERE direction='IN'),0),COALESCE(SUM(amount) FILTER(WHERE direction='OUT'),0),COALESCE(SUM(CASE direction WHEN 'IN' THEN amount ELSE -amount END),0) FROM fund_transactions`).Scan(&s.FundIn, &s.FundOut, &s.FundBalance)
	}
	if err == nil {
		err = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM notifications WHERE status='FAILED'`).Scan(&s.NotificationsFailed)
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, "admin_dashboard", err)
		return
	}
	months := dashboardMonths(r)
	contributions := make([]MonthlyAmount, 0)
	rows, queryErr := h.db.Query(r.Context(), `WITH month_series(period_start) AS (
		SELECT generate_series(
			date_trunc('month',CURRENT_DATE)-($1-1)*INTERVAL '1 month',
			date_trunc('month',CURRENT_DATE),
			INTERVAL '1 month'
		)
	)
	SELECT
		to_char(m.period_start,'Mon YYYY'),
		(SELECT COALESCE(SUM(p.amount),0)
			 FROM contribution_plans p JOIN users u ON u.id=p.user_id AND u.status='ACTIVE'
		 WHERE p.is_active
		   AND p.start_date < m.period_start+INTERVAL '1 month'
		   AND (p.end_date IS NULL OR p.end_date >= m.period_start)),
		(SELECT COALESCE(SUM(c.paid_amount),0)
		 FROM contributions c
		 WHERE c.status='APPROVED'
		   AND date_trunc('month',COALESCE(c.payment_date,c.approved_at))=m.period_start)
	FROM month_series m
	ORDER BY m.period_start`, months)
	if queryErr != nil {
		httpx.WriteInternal(w, r, h.logger, "admin_dashboard_contribution_performance", queryErr)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var point MonthlyAmount
		if err = rows.Scan(&point.Month, &point.Expected, &point.Collected); err != nil {
			httpx.WriteInternal(w, r, h.logger, "admin_dashboard_contribution_performance_scan", err)
			return
		}
		contributions = append(contributions, point)
	}
	if err = rows.Err(); err != nil {
		httpx.WriteInternal(w, r, h.logger, "admin_dashboard_contribution_performance_rows", err)
		return
	}
	fundMovement := make([]MonthlyAmount, 0)
	fundRows, fundErr := h.db.Query(r.Context(), `WITH months(period_start) AS (SELECT generate_series(date_trunc('month',CURRENT_DATE)-($1-1)*INTERVAL '1 month',date_trunc('month',CURRENT_DATE),INTERVAL '1 month')) SELECT to_char(m.period_start,'Mon YYYY'),COALESCE(SUM(f.amount) FILTER(WHERE f.direction='IN'),0),COALESCE(SUM(f.amount) FILTER(WHERE f.direction='OUT'),0) FROM months m LEFT JOIN fund_transactions f ON date_trunc('month',f.created_at)=m.period_start GROUP BY m.period_start ORDER BY m.period_start`, months)
	if fundErr == nil {
		defer fundRows.Close()
		for fundRows.Next() {
			var point MonthlyAmount
			if fundRows.Scan(&point.Month, &point.Inflow, &point.Outflow) == nil {
				fundMovement = append(fundMovement, point)
			}
		}
	}
	memberStatuses := []NamedCount{{Name: "ACTIVE", Count: s.MembersActive}, {Name: "INACTIVE", Count: s.MembersInactive}, {Name: "SUSPENDED", Count: s.MembersSuspended}}
	var overdue1, overdue2, overdue3, overdue4 int
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FILTER(WHERE CURRENT_DATE-(c.due_date+p.grace_period_days) BETWEEN 1 AND 7),COUNT(*) FILTER(WHERE CURRENT_DATE-(c.due_date+p.grace_period_days) BETWEEN 8 AND 14),COUNT(*) FILTER(WHERE CURRENT_DATE-(c.due_date+p.grace_period_days) BETWEEN 15 AND 30),COUNT(*) FILTER(WHERE CURRENT_DATE-(c.due_date+p.grace_period_days)>30) FROM contributions c JOIN contribution_plans p ON p.id=c.contribution_plan_id WHERE c.status IN('OVERDUE','REJECTED')`).Scan(&overdue1, &overdue2, &overdue3, &overdue4)
	overdueBuckets := []NamedCount{{Name: "1-7 days", Count: overdue1}, {Name: "8-14 days", Count: overdue2}, {Name: "15-30 days", Count: overdue3}, {Name: "30+ days", Count: overdue4}}
	frequencySummary, frequencyErr := h.frequencySummary(r, nil)
	if frequencyErr != nil {
		httpx.WriteInternal(w, r, h.logger, "admin_dashboard_frequency_summary", frequencyErr)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": map[string]any{"summary": s, "contribution_performance": contributions, "contribution_by_frequency": frequencySummary, "fund_movement": fundMovement, "member_statuses": memberStatuses, "overdue_buckets": overdueBuckets}})
}

func (h *Handler) frequencySummary(r *http.Request, userID *uuid.UUID) ([]FrequencySummary, error) {
	query := `SELECT p.frequency,
		COALESCE(SUM(c.expected_amount+c.late_fee_amount) FILTER(WHERE c.status<>'FROZEN'),0),
		COALESCE(SUM(c.paid_amount) FILTER(WHERE c.status='APPROVED'),0),
		COALESCE(SUM(c.expected_amount+c.late_fee_amount) FILTER(WHERE c.status IN('OVERDUE','REJECTED')),0),
		COUNT(*),COUNT(*) FILTER(WHERE c.status='APPROVED'),COUNT(*) FILTER(WHERE c.status='PENDING'),
		COUNT(*) FILTER(WHERE c.status='OVERDUE'),COUNT(*) FILTER(WHERE c.status='REJECTED')
		FROM contributions c JOIN contribution_plans p ON p.id=c.contribution_plan_id`
	args := make([]any, 0, 1)
	if userID != nil {
		query += ` WHERE c.user_id=$1`
		args = append(args, *userID)
	}
	query += ` GROUP BY p.frequency ORDER BY CASE p.frequency WHEN 'DAILY' THEN 1 WHEN 'WEEKLY' THEN 2 WHEN 'MONTHLY' THEN 3 WHEN 'CUSTOM' THEN 4 ELSE 5 END`
	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]FrequencySummary, 0, 4)
	for rows.Next() {
		var item FrequencySummary
		if err = rows.Scan(&item.Frequency, &item.Expected, &item.Collected, &item.Outstanding, &item.Total, &item.Approved, &item.Pending, &item.Overdue, &item.Rejected); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func dashboardMonths(r *http.Request) int {
	months, _ := strconv.Atoi(r.URL.Query().Get("months"))
	if months != 10 {
		return 6
	}
	return months
}
func (h *Handler) namedCounts(r *http.Request, query string, args ...any) []NamedCount {
	items := make([]NamedCount, 0)
	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		return items
	}
	defer rows.Close()
	for rows.Next() {
		var item NamedCount
		if rows.Scan(&item.Name, &item.Count) == nil {
			items = append(items, item)
		}
	}
	return items
}
