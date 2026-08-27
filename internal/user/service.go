package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"socialfund/internal/audit"
	"socialfund/internal/contributionplan"
	"socialfund/internal/database"
	"socialfund/internal/httpx"
	"socialfund/internal/notification"
)

var (
	ErrNotFound    = httpx.NewError(404, "USER_NOT_FOUND", "User was not found")
	ErrEmailExists = httpx.NewError(409, "EMAIL_ALREADY_EXISTS", "A member with this email already exists")
	ErrPhoneExists = httpx.NewError(409, "PHONE_ALREADY_EXISTS", "A member with this phone number already exists")
)

type PlanWriter interface {
	CreateWithDB(context.Context, database.DBTX, contributionplan.ContributionPlan) (contributionplan.ContributionPlan, error)
}
type Service struct {
	pool          *pgxpool.Pool
	repo          Repository
	plans         PlanWriter
	notifications notification.Writer
	audit         audit.Writer
	frontendURL   string
}

func NewService(pool *pgxpool.Pool, repo Repository, plans PlanWriter, notifications notification.Writer, auditWriter audit.Writer, frontendURL string) *Service {
	return &Service{pool: pool, repo: repo, plans: plans, notifications: notifications, audit: auditWriter, frontendURL: strings.TrimRight(frontendURL, "/")}
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (s *Service) List(ctx context.Context, f ListFilter) ([]User, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.List(ctx, f)
}
func (s *Service) Update(ctx context.Context, adminID, id uuid.UUID, in UpdateInput) (User, error) {
	var out User
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		old, err := s.repo.LockByID(ctx, tx, id)
		if err != nil {
			return ErrNotFound
		}
		if old.Role != "MEMBER" {
			return httpx.ErrValidation
		}
		if err = s.repo.Update(ctx, tx, id, in); err != nil {
			return mapCreateError(err)
		}
		out, err = s.repo.LockByID(ctx, tx, id)
		if err != nil {
			return err
		}
		admin := adminID
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "USER_UPDATED", EntityType: "USER", EntityID: id, OldData: auditData(old), NewData: auditData(out)})
		return err
	})
	return out, err
}
func (s *Service) ChangeStatus(ctx context.Context, adminID, id uuid.UUID, activate bool) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		u, err := s.repo.LockByID(ctx, tx, id)
		if err != nil {
			return ErrNotFound
		}
		if u.Role != "MEMBER" {
			return httpx.ErrValidation
		}
		from, to, action := "ACTIVE", "SUSPENDED", "USER_SUSPENDED"
		if activate {
			from, to, action = "SUSPENDED", "ACTIVE", "USER_ACTIVATED"
		}
		if err = s.repo.SetStatus(ctx, tx, id, from, to); err != nil {
			return httpx.NewError(409, "INVALID_STATUS_TRANSITION", "User cannot be changed from the current status")
		}
		admin := adminID
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: action, EntityType: "USER", EntityID: id, OldData: auditData(map[string]string{"status": from}), NewData: auditData(map[string]string{"status": to})})
		return err
	})
}
func (s *Service) CreateMember(ctx context.Context, adminID uuid.UUID, in CreateMemberRequest) (MemberResponse, error) {
	startDate, err := validateCreateMember(in)
	if err != nil {
		return MemberResponse{}, err
	}
	var created User
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		admin := adminID
		created, err = s.repo.CreateWithDB(ctx, tx, User{FullName: strings.TrimSpace(in.FullName), Email: strings.ToLower(strings.TrimSpace(in.Email)), Phone: strings.TrimSpace(in.Phone), Role: "MEMBER", Status: "INACTIVE", CreatedBy: &admin})
		if err != nil {
			return mapCreateError(err)
		}
		var reminderFrequency *string
		if in.Reminder.Enabled {
			value := strings.ToUpper(in.Reminder.Frequency)
			reminderFrequency = &value
		}
		_, err = s.plans.CreateWithDB(ctx, tx, contributionplan.ContributionPlan{UserID: created.ID, Amount: in.Contribution.Amount, Frequency: strings.ToUpper(in.Contribution.Frequency), IntervalValue: in.Contribution.IntervalValue, DueDay: in.Contribution.DueDay, StartDate: startDate, ReminderEnabled: in.Reminder.Enabled, ReminderFrequency: reminderFrequency, ReminderInterval: in.Reminder.Interval, LateFeeEnabled: in.Contribution.LateFeeEnabled, LateFeePercentage: in.Contribution.LateFeePercentage, GracePeriodDays: in.Contribution.GracePeriodDays, IsActive: true, CreatedBy: adminID})
		if err != nil {
			return fmt.Errorf("create contribution plan: %w", err)
		}
		subject := "Welcome to Social Fund"
		message := welcomeMessage(in, s.frontendURL)
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: created.ID, Type: "ACCOUNT_CREATED", Channel: "EMAIL", Recipient: created.Email, Subject: &subject, Message: &message, Status: "PENDING"})
		if err != nil {
			return fmt.Errorf("queue welcome notification: %w", err)
		}
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "USER_CREATED", EntityType: "USER", EntityID: created.ID, NewData: auditData(map[string]string{"role": "MEMBER", "status": "INACTIVE"})})
		return err
	})
	if err != nil {
		return MemberResponse{}, err
	}
	return responseFromUser(created), nil
}

func validateCreateMember(in CreateMemberRequest) (time.Time, error) {
	if strings.TrimSpace(in.FullName) == "" || strings.TrimSpace(in.Email) == "" || strings.TrimSpace(in.Phone) == "" || !in.Contribution.Amount.IsPositive() || in.Contribution.StartDate == "" {
		return time.Time{}, httpx.ErrValidation
	}
	address, err := mail.ParseAddress(in.Email)
	if err != nil || !strings.EqualFold(address.Address, in.Email) {
		return time.Time{}, httpx.ErrValidation
	}
	frequency := strings.ToUpper(in.Contribution.Frequency)
	switch frequency {
	case "DAILY", "WEEKLY", "MONTHLY", "CUSTOM":
	default:
		return time.Time{}, httpx.ErrValidation
	}
	if frequency == "MONTHLY" && (in.Contribution.DueDay == nil || *in.Contribution.DueDay < 1 || *in.Contribution.DueDay > 31) {
		return time.Time{}, httpx.ErrValidation
	}
	if frequency == "CUSTOM" && (in.Contribution.IntervalValue == nil || *in.Contribution.IntervalValue < 1) {
		return time.Time{}, httpx.ErrValidation
	}
	if in.Contribution.GracePeriodDays < 0 || (in.Contribution.LateFeeEnabled && (in.Contribution.LateFeePercentage == nil || in.Contribution.LateFeePercentage.IsNegative() || in.Contribution.LateFeePercentage.GreaterThan(decimal.NewFromInt(100)))) {
		return time.Time{}, httpx.ErrValidation
	}
	if in.Reminder.Enabled {
		switch strings.ToUpper(in.Reminder.Frequency) {
		case "DAILY", "WEEKLY", "CUSTOM":
		default:
			return time.Time{}, httpx.ErrValidation
		}
		if strings.EqualFold(in.Reminder.Frequency, "CUSTOM") && (in.Reminder.Interval == nil || *in.Reminder.Interval < 1) {
			return time.Time{}, httpx.ErrValidation
		}
	}
	startDate, err := time.Parse("2006-01-02", in.Contribution.StartDate)
	if err != nil {
		return time.Time{}, httpx.ErrValidation
	}
	return startDate, nil
}
func mapCreateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "users_email_unique", "users_email_case_insensitive_unique":
		return ErrEmailExists
	case "users_phone_unique":
		return ErrPhoneExists
	}
	return err
}

func welcomeMessage(in CreateMemberRequest, frontendURL string) string {
	due := ""
	if in.Contribution.DueDay != nil {
		due = fmt.Sprintf("\nPayment Due Day: %d of every month", *in.Contribution.DueDay)
	}
	return fmt.Sprintf("Hello %s,\n\nYour Social Fund account has been created.\n\nName: %s\nEmail: %s\nPhone: %s\nContribution: %s\nContribution Frequency: %s%s\n\nYour account is currently inactive.\n\nTo activate your account, access Social Fund and sign in using Google with:\n%s\n\nAccess Social Fund: %s/login\n\nYour account will become active only after successful Google verification.", in.FullName, in.FullName, in.Email, in.Phone, in.Contribution.Amount.StringFixed(2), strings.Title(strings.ToLower(in.Contribution.Frequency)), due, in.Email, frontendURL)
}
func auditData(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
