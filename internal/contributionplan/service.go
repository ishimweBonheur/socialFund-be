package contributionplan

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"socialfund/internal/httpx"
	"strings"
)

type Service struct {
	repo  Repository
	pool  *pgxpool.Pool
	audit audit.Writer
}

func NewService(repo Repository, extras ...any) *Service {
	s := &Service{repo: repo}
	if len(extras) > 1 {
		s.pool, _ = extras[0].(*pgxpool.Pool)
		s.audit, _ = extras[1].(audit.Writer)
	}
	return s
}
func (s *Service) GetActive(ctx context.Context, userID uuid.UUID) (ContributionPlan, error) {
	return s.repo.GetActiveByUserID(ctx, userID)
}
func (s *Service) List(ctx context.Context, filter ListFilter) ([]ListItem, int, error) {
	return s.repo.List(ctx, filter)
}
func (s *Service) Update(ctx context.Context, adminID, id uuid.UUID, p ContributionPlan) (ContributionPlan, error) {
	if s.pool == nil {
		return ContributionPlan{}, httpx.ErrInternal
	}
	p.ID = id
	if err := validate(p); err != nil {
		return ContributionPlan{}, err
	}
	var old ContributionPlan
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		old, err = s.repo.Lock(ctx, tx, id)
		if err != nil {
			return httpx.NewError(404, "CONTRIBUTION_PLAN_NOT_FOUND", "Contribution plan was not found")
		}
		p.UserID = old.UserID
		p.StartDate = old.StartDate
		p.IsActive = old.IsActive
		p.CreatedBy = old.CreatedBy
		if err = s.repo.Update(ctx, tx, p); err != nil {
			return err
		}
		admin := adminID
		a, _ := json.Marshal(old)
		b, _ := json.Marshal(p)
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "CONTRIBUTION_PLAN_UPDATED", EntityType: "CONTRIBUTION_PLAN", EntityID: id, OldData: a, NewData: b})
		return err
	})
	return p, err
}
func validate(p ContributionPlan) error {
	if !p.Amount.IsPositive() || p.GracePeriodDays < 0 || p.PreDueReminderDaysBeforeDue < 0 || p.PreDueReminderDaysBeforeDue > 365 {
		return httpx.ErrValidation
	}
	p.Frequency = strings.ToUpper(p.Frequency)
	if p.Frequency == "MONTHLY" && (p.DueDay == nil || *p.DueDay < 1 || *p.DueDay > 31) {
		return httpx.ErrValidation
	}
	if p.Frequency == "CUSTOM" && (p.IntervalValue == nil || *p.IntervalValue < 1) {
		return httpx.ErrValidation
	}
	if p.ReminderEnabled && !validReminder(p.ReminderFrequency, p.ReminderInterval) {
		return httpx.ErrValidation
	}
	if p.PreDueReminderEnabled && !validReminder(p.PreDueReminderFrequency, p.PreDueReminderInterval) {
		return httpx.ErrValidation
	}
	if p.LateFeeEnabled && (p.LateFeePercentage == nil || p.LateFeePercentage.IsNegative() || p.LateFeePercentage.GreaterThan(decimal.NewFromInt(100))) {
		return httpx.ErrValidation
	}
	return nil
}

func validReminder(frequency *string, interval *int) bool {
	if frequency == nil {
		return false
	}
	switch strings.ToUpper(*frequency) {
	case "DAILY", "WEEKLY", "MONTHLY":
		return true
	case "CUSTOM":
		return interval != nil && *interval >= 1 && *interval <= 365
	default:
		return false
	}
}
func (s *Service) Create(ctx context.Context, p ContributionPlan) (ContributionPlan, error) {
	return s.repo.Create(ctx, p)
}
