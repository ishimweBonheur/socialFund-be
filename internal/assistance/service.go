package assistance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"socialfund/internal/fund"
	"socialfund/internal/notification"
)

var ErrInvalidState = errors.New("assistance request is not approved")
var ErrInvalidAmount = errors.New("disbursement amount is invalid")
var ErrInvalidReason = errors.New("reason is required")
var ErrInsufficientFunds = errors.New("insufficient fund balance")

type Service struct {
	pool          *pgxpool.Pool
	repo          Repository
	fund          fund.Writer
	audit         audit.Writer
	notifications notification.Writer
}

func (s *Service) Create(ctx context.Context, in CreateInput) (AssistanceRequest, error) {
	if !in.AmountRequested.IsPositive() || in.Reason == "" {
		return AssistanceRequest{}, ErrInvalidAmount
	}
	var out AssistanceRequest
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.repo.CreateWithDB(ctx, tx, AssistanceRequest{UserID: in.UserID, AmountRequested: in.AmountRequested, Reason: in.Reason, Description: in.Description, AttachmentURL: in.AttachmentURL})
		if err != nil {
			return err
		}
		actor := in.UserID
		data, _ := json.Marshal(map[string]string{"status": "PENDING", "amount": in.AmountRequested.StringFixed(2)})
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &actor, Action: "ASSISTANCE_REQUEST_CREATED", EntityType: "ASSISTANCE_REQUEST", EntityID: out.ID, NewData: data})
		return err
	})
	return out, err
}
func (s *Service) ListMine(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ReviewItem, error) {
	return s.repo.List(ctx, ListFilter{UserID: &userID, Limit: limit, Offset: offset})
}
func (s *Service) ListAdmin(ctx context.Context, status string, limit, offset int) ([]ReviewItem, error) {
	return s.repo.List(ctx, ListFilter{Status: status, Limit: limit, Offset: offset})
}
func (s *Service) Approve(ctx context.Context, in ApprovalInput) error {
	if !in.AmountApproved.IsPositive() {
		return ErrInvalidAmount
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		a, email, err := s.repo.Lock(ctx, tx, in.AssistanceRequestID)
		if err != nil {
			return err
		}
		if a.Status != "PENDING" {
			return ErrInvalidState
		}
		if in.AmountApproved.GreaterThan(a.AmountRequested) {
			return ErrInvalidAmount
		}
		if err = s.repo.SetApproved(ctx, tx, in); err != nil {
			return err
		}
		admin := in.AdminID
		old, _ := json.Marshal(map[string]string{"status": a.Status})
		next, _ := json.Marshal(map[string]string{"status": "APPROVED", "amount": in.AmountApproved.StringFixed(2)})
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "ASSISTANCE_APPROVED", EntityType: "ASSISTANCE_REQUEST", EntityID: a.ID, OldData: old, NewData: next}); err != nil {
			return err
		}
		id := a.ID
		subject, message := "Assistance request approved", fmt.Sprintf("Your assistance request was approved for %s.", in.AmountApproved.StringFixed(2))
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: a.UserID, AssistanceRequestID: &id, Type: "ASSISTANCE_APPROVED", Channel: "EMAIL", Recipient: email, Subject: &subject, Message: &message, Status: "PENDING"})
		return err
	})
}
func (s *Service) Reject(ctx context.Context, in RejectionInput) error {
	if in.Reason == "" {
		return ErrInvalidReason
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		a, email, err := s.repo.Lock(ctx, tx, in.AssistanceRequestID)
		if err != nil {
			return err
		}
		if a.Status != "PENDING" {
			return ErrInvalidState
		}
		if err = s.repo.SetRejected(ctx, tx, in); err != nil {
			return err
		}
		admin := in.AdminID
		old, _ := json.Marshal(map[string]string{"status": a.Status})
		next, _ := json.Marshal(map[string]string{"status": "REJECTED", "reason": in.Reason})
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "ASSISTANCE_REJECTED", EntityType: "ASSISTANCE_REQUEST", EntityID: a.ID, OldData: old, NewData: next}); err != nil {
			return err
		}
		id := a.ID
		subject, message := "Assistance request rejected", fmt.Sprintf("Your assistance request was rejected: %s", in.Reason)
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: a.UserID, AssistanceRequestID: &id, Type: "ASSISTANCE_REJECTED", Channel: "EMAIL", Recipient: email, Subject: &subject, Message: &message, Status: "PENDING"})
		return err
	})
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
		var balance decimal.Decimal
		if err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(CASE direction WHEN 'IN' THEN amount ELSE -amount END),0) FROM fund_transactions`).Scan(&balance); err != nil {
			return fmt.Errorf("calculate fund balance: %w", err)
		}
		if balance.LessThan(in.Amount) {
			return ErrInsufficientFunds
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
		subject, message := "Assistance payment sent", fmt.Sprintf("Your assistance payment of %s has been recorded. Reference: %s.", in.Amount.StringFixed(2), in.Reference)
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: a.UserID, AssistanceRequestID: &id, Type: "ASSISTANCE_PAID", Channel: "EMAIL", Recipient: email, Subject: &subject, Message: &message, Status: "PENDING"})
		return err
	})
}
