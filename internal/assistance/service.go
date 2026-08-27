package assistance

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
)

var ErrInvalidState = errors.New("assistance request is not approved")
var ErrInvalidAmount = errors.New("disbursement amount is invalid")

type Service struct {
	pool          *pgxpool.Pool
	repo          Repository
	fund          fund.Writer
	audit         audit.Writer
	notifications notification.Writer
}

func NewService(pool *pgxpool.Pool, repo Repository, fundWriter fund.Writer, auditWriter audit.Writer, notificationWriter notification.Writer) *Service {
	return &Service{pool: pool, repo: repo, fund: fundWriter, audit: auditWriter, notifications: notificationWriter}
}
func (s *Service) Pay(ctx context.Context, in DisbursementInput) error {
	if !in.Amount.IsPositive() || in.Method == "" || in.Reference == "" {
		return ErrInvalidAmount
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		a, email, err := s.repo.Lock(ctx, tx, in.AssistanceRequestID)
		if err != nil {
			return fmt.Errorf("lock assistance request: %w", err)
		}
		if a.Status != "APPROVED" {
			return ErrInvalidState
		}
		if a.AmountApproved == nil || !in.Amount.Equal(*a.AmountApproved) {
			return ErrInvalidAmount
		}
		if err = s.repo.SetPaid(ctx, tx, in); err != nil {
			return err
		}
		id := a.ID
		reference := in.Reference
		if _, err = s.fund.Create(ctx, tx, fund.FundTransaction{UserID: a.UserID, Type: "ASSISTANCE", Direction: "OUT", Amount: in.Amount, AssistanceRequestID: &id, Reference: &reference, RecordedBy: in.AdminID}); err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}
		oldData, _ := json.Marshal(map[string]string{"status": a.Status})
		newData, _ := json.Marshal(map[string]string{"status": "PAID", "amount": in.Amount.StringFixed(2)})
		admin := in.AdminID
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "ASSISTANCE_PAID", EntityType: "ASSISTANCE_REQUEST", EntityID: a.ID, OldData: oldData, NewData: newData}); err != nil {
			return err
		}
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: a.UserID, AssistanceRequestID: &id, Type: "ASSISTANCE_PAID", Channel: "EMAIL", Recipient: email, Status: "PENDING"})
		return err
	})
}
