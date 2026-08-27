package contribution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"socialfund/internal/fund"
	"socialfund/internal/notification"
	"socialfund/internal/user"
)

var ErrInvalidState = errors.New("contribution is not pending")
var ErrInvalidAmount = errors.New("paid amount must be positive")

type Service struct {
	pool          *pgxpool.Pool
	repo          Repository
	fund          fund.Writer
	audit         audit.Writer
	notifications notification.Writer
	users         user.Repository
}

func NewService(pool *pgxpool.Pool, repo Repository, fundWriter fund.Writer, auditWriter audit.Writer, notificationWriter notification.Writer, users user.Repository) *Service {
	return &Service{pool: pool, repo: repo, fund: fundWriter, audit: auditWriter, notifications: notificationWriter, users: users}
}
func (s *Service) Approve(ctx context.Context, in ApprovalInput) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		c, email, err := s.repo.Lock(ctx, tx, in.ContributionID)
		if err != nil {
			return fmt.Errorf("lock contribution: %w", err)
		}
		if c.Status != "PENDING" {
			return ErrInvalidState
		}
		if c.PaidAmount == nil || !c.PaidAmount.IsPositive() {
			return ErrInvalidAmount
		}
		if err = s.repo.SetApproved(ctx, tx, in); err != nil {
			return err
		}
		id := c.ID
		if _, err = s.fund.Create(ctx, tx, fund.FundTransaction{UserID: c.UserID, Type: "CONTRIBUTION", Direction: "IN", Amount: *c.PaidAmount, ContributionID: &id, RecordedBy: in.AdminID}); err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}
		oldData, _ := json.Marshal(map[string]string{"status": c.Status})
		newData, _ := json.Marshal(map[string]string{"status": "APPROVED"})
		admin := in.AdminID
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "CONTRIBUTION_APPROVED", EntityType: "CONTRIBUTION", EntityID: c.ID, OldData: oldData, NewData: newData}); err != nil {
			return err
		}
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_APPROVED", Channel: "EMAIL", Recipient: email, Status: "PENDING"})
		return err
	})
}
func (s *Service) Reject(ctx context.Context, in RejectionInput) error {
	if in.Reason == "" {
		return fmt.Errorf("rejection reason is required")
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		c, email, err := s.repo.Lock(ctx, tx, in.ContributionID)
		if err != nil {
			return err
		}
		if c.Status != "PENDING" {
			return ErrInvalidState
		}
		if err = s.repo.SetRejected(ctx, tx, in); err != nil {
			return err
		}
		oldData, _ := json.Marshal(map[string]string{"status": c.Status})
		newData, _ := json.Marshal(map[string]string{"status": "REJECTED", "reason": in.Reason})
		admin := in.AdminID
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "CONTRIBUTION_REJECTED", EntityType: "CONTRIBUTION", EntityID: c.ID, OldData: oldData, NewData: newData}); err != nil {
			return err
		}
		id := c.ID
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_REJECTED", Channel: "EMAIL", Recipient: email, Status: "PENDING"})
		return err
	})
}
func (s *Service) ProcessOverdue(ctx context.Context, limit int) (int, error) {
	guard, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = guard.Rollback(ctx) }()
	if _, err = guard.Exec(ctx, `SELECT pg_advisory_xact_lock(73662411)`); err != nil {
		return 0, err
	}
	if _, err = s.repo.MarkDueAsOverdue(ctx); err != nil {
		return 0, err
	}
	items, err := s.repo.ListOverdueForReminders(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, c := range items {
		u, e := s.users.GetByID(ctx, c.UserID)
		if e != nil {
			return 0, e
		}
		id := c.ID
		if _, e = s.notifications.Create(ctx, s.pool, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_OVERDUE", Channel: "EMAIL", Recipient: u.Email, Status: "PENDING"}); e != nil {
			return 0, e
		}
	}
	if err = guard.Commit(ctx); err != nil {
		return 0, err
	}
	return len(items), nil
}
