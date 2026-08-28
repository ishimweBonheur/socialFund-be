package notification

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"strings"
	"time"
)

var ErrInvalidSupportRequest = errors.New("invalid support request")

type SupportRequestInput struct {
	UserID         uuid.UUID
	Category       string
	Message        string
	ContributionID *uuid.UUID
}

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
func (s *Service) Claim(ctx context.Context, limit int) ([]Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.repo.ListReady(ctx, limit)
}
func (s *Service) List(ctx context.Context, f Filter) ([]Notification, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.(AdminRepository).List(ctx, f)
}
func (s *Service) Retry(ctx context.Context, adminID, id uuid.UUID) error {
	if s.pool == nil {
		return pgx.ErrNoRows
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.(AdminRepository).Retry(ctx, tx, id); err != nil {
			return err
		}
		admin := adminID
		_, err := s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "NOTIFICATION_RETRY_REQUESTED", EntityType: "NOTIFICATION", EntityID: id})
		return err
	})
}
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.(AdminRepository).MarkRead(ctx, userID, id)
}
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.(AdminRepository).MarkAllRead(ctx, userID)
}
func (s *Service) SubmitSupportRequest(ctx context.Context, in SupportRequestInput) error {
	in.Category = strings.TrimSpace(strings.ToUpper(in.Category))
	in.Message = strings.TrimSpace(in.Message)
	if in.Category == "" || in.Message == "" || len(in.Message) > 4000 || s.pool == nil {
		return ErrInvalidSupportRequest
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var memberName, memberEmail string
		if err := tx.QueryRow(ctx, `SELECT full_name,email FROM users WHERE id=$1 AND status='ACTIVE'`, in.UserID).Scan(&memberName, &memberEmail); err != nil {
			return err
		}
		var adminID uuid.UUID
		var adminEmail string
		if err := tx.QueryRow(ctx, `SELECT id,email FROM users WHERE role='ADMIN' AND status='ACTIVE' ORDER BY created_at LIMIT 1`).Scan(&adminID, &adminEmail); err != nil {
			return err
		}
		var contributionDetails string
		if in.ContributionID != nil {
			var status, reference string
			if err := tx.QueryRow(ctx, `SELECT status,COALESCE(transaction_reference,'') FROM contributions WHERE id=$1 AND user_id=$2`, *in.ContributionID, in.UserID).Scan(&status, &reference); err != nil {
				return ErrInvalidSupportRequest
			}
			if status != "PENDING" {
				return ErrInvalidSupportRequest
			}
			contributionDetails = fmt.Sprintf("\nContribution ID: %s\nStatus: %s\nTransaction reference: %s", in.ContributionID.String(), status, reference)
		}
		subject := "Member support request: " + strings.ReplaceAll(strings.ToLower(in.Category), "_", " ")
		message := fmt.Sprintf("Member: %s\nEmail: %s\nCategory: %s%s\n\nMessage:\n%s", memberName, memberEmail, in.Category, contributionDetails, in.Message)
		if _, err := s.repo.Create(ctx, tx, Notification{UserID: adminID, ContributionID: in.ContributionID, Type: "SUPPORT_REQUEST", Channel: "EMAIL", Recipient: adminEmail, Subject: &subject, Message: &message, Status: "PENDING"}); err != nil {
			return err
		}
		confirmationSubject := "Support request sent"
		confirmationMessage := "Your message was sent to the administrator. You will see further account and payment actions in Notifications."
		_, err := s.repo.Create(ctx, tx, Notification{UserID: in.UserID, ContributionID: in.ContributionID, Type: "SUPPORT_REQUEST_RECEIVED", Channel: "IN_APP", Recipient: memberEmail, Subject: &confirmationSubject, Message: &confirmationMessage, Status: "SENT"})
		return err
	})
}
func (s *Service) MarkSent(ctx context.Context, id uuid.UUID) error { return s.repo.MarkSent(ctx, id) }
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, message string, attempt int) error {
	delay := time.Minute * time.Duration(1<<min(attempt, 6))
	return s.repo.MarkFailed(ctx, id, message, time.Now().Add(delay))
}
