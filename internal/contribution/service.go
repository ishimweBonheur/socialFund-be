package contribution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"socialfund/internal/fund"
	"socialfund/internal/httpx"
	"socialfund/internal/notification"
	"socialfund/internal/user"
	"strings"
	"time"
)

var ErrInvalidState = errors.New("contribution is not pending")
var ErrInvalidAmount = errors.New("paid amount must be positive")
var ErrForbidden = errors.New("contribution does not belong to member")
var ErrProofRequired = errors.New("payment proof is required")
var ErrDuplicateTransactionReference = errors.New("transaction reference has already been used")

type Service struct {
	pool          *pgxpool.Pool
	repo          Repository
	fund          fund.Writer
	audit         audit.Writer
	notifications notification.Writer
	users         user.Repository
	frontendURL   string
	apiPublicURL  string
}

func (s *Service) RunOverdueScheduler(ctx context.Context, interval time.Duration, limit int) error {
	if _, err := s.ProcessOverdue(ctx, limit); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := s.ProcessOverdue(ctx, limit); err != nil && ctx.Err() == nil {
				return err
			}
		}
	}
}

func NewService(pool *pgxpool.Pool, repo Repository, fundWriter fund.Writer, auditWriter audit.Writer, notificationWriter notification.Writer, users user.Repository, frontendURLs ...string) *Service {
	frontendURL := ""
	if len(frontendURLs) > 0 {
		frontendURL = strings.TrimRight(frontendURLs[0], "/")
	}
	apiPublicURL := ""
	if len(frontendURLs) > 1 {
		apiPublicURL = strings.TrimRight(frontendURLs[1], "/")
	}
	return &Service{pool: pool, repo: repo, fund: fundWriter, audit: auditWriter, notifications: notificationWriter, users: users, frontendURL: frontendURL, apiPublicURL: apiPublicURL}
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
		if c.ProofURL == nil || *c.ProofURL == "" || c.PaymentMethod == nil || c.TransactionReference == nil {
			return ErrProofRequired
		}
		if err = s.repo.SetApproved(ctx, tx, in); err != nil {
			return err
		}
		id := c.ID
		if _, err = s.fund.Create(ctx, tx, fund.FundTransaction{UserID: c.UserID, Type: "CONTRIBUTION", Direction: "IN", Amount: *c.PaidAmount, ContributionID: &id, Reference: c.TransactionReference, RecordedBy: in.AdminID}); err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}
		oldData, _ := json.Marshal(map[string]string{"status": c.Status})
		newData, _ := json.Marshal(map[string]string{"status": "APPROVED"})
		admin := in.AdminID
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "CONTRIBUTION_APPROVED", EntityType: "CONTRIBUTION", EntityID: c.ID, OldData: oldData, NewData: newData}); err != nil {
			return err
		}
		subject, message := "Contribution approved", fmt.Sprintf("Your contribution of %s has been approved. Payment method: %s. Reference: %s. Approval date: %s. Status: APPROVED.", c.PaidAmount.StringFixed(2), *c.PaymentMethod, *c.TransactionReference, time.Now().Format(time.RFC3339))
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_APPROVED", Channel: "EMAIL", Recipient: email, Subject: &subject, Message: &message, Status: "PENDING"})
		return err
	})
}
func (s *Service) ListMine(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Contribution, error) {
	return s.repo.ListByUser(ctx, userID, limit, offset)
}
func (s *Service) GetFor(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (Contribution, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err == nil && !isAdmin && c.UserID != userID {
		return Contribution{}, ErrForbidden
	}
	return c, err
}
func (s *Service) ListPending(ctx context.Context, limit, offset int) ([]ReviewItem, error) {
	return s.repo.ListPending(ctx, limit, offset)
}
func (s *Service) ListAdmin(ctx context.Context, filter AdminListFilter) ([]ReviewItem, int, error) {
	return s.repo.ListAdmin(ctx, filter)
}
func (s *Service) Outstanding(ctx context.Context, userID uuid.UUID) (Outstanding, error) {
	return s.repo.Outstanding(ctx, userID)
}
func (s *Service) SubmitProof(ctx context.Context, in ProofInput) error {
	in.TransactionReference = strings.ToUpper(strings.TrimSpace(in.TransactionReference))
	methods := map[string]bool{"MOBILE_MONEY": true, "BANK_TRANSFER": true, "CASH": true, "OTHER": true}
	if !in.Amount.IsPositive() || !methods[in.PaymentMethod] || in.TransactionReference == "" || in.ProofURL == "" {
		return ErrInvalidAmount
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		c, email, err := s.repo.Lock(ctx, tx, in.ContributionID)
		if err != nil {
			return fmt.Errorf("lock contribution: %w", err)
		}
		if c.UserID != in.UserID {
			return ErrForbidden
		}
		if c.Status != "DUE" && c.Status != "OVERDUE" && c.Status != "REJECTED" {
			return ErrInvalidState
		}
		if !in.Amount.Equal(c.TotalDue()) {
			return ErrInvalidAmount
		}
		resubmitted := c.Status == "REJECTED"
		if err = s.repo.SubmitProof(ctx, tx, in); err != nil {
			var postgresError *pgconn.PgError
			if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "contributions_transaction_reference_unique" {
				return ErrDuplicateTransactionReference
			}
			return err
		}
		rawToken, tokenHash, err := newReviewToken()
		if err != nil {
			return err
		}
		if err = s.repo.SetReviewToken(ctx, tx, c.ID, tokenHash, time.Now().Add(24*time.Hour)); err != nil {
			return err
		}
		action := "PROOF_UPLOADED"
		if resubmitted {
			action = "CONTRIBUTION_PROOF_RESUBMITTED"
		}
		actor := in.UserID
		oldData, _ := json.Marshal(map[string]string{"status": c.Status})
		newData, _ := json.Marshal(map[string]string{"status": "PENDING", "amount": in.Amount.StringFixed(2)})
		if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &actor, Action: action, EntityType: "CONTRIBUTION", EntityID: c.ID, OldData: oldData, NewData: newData}); err != nil {
			return err
		}
		// Queue for the configured administrator; falling back to the member recipient is deliberately avoided.
		var adminID uuid.UUID
		var adminEmail string
		if err = tx.QueryRow(ctx, `SELECT id,email FROM users WHERE role='ADMIN' AND status='ACTIVE' ORDER BY created_at LIMIT 1`).Scan(&adminID, &adminEmail); err != nil {
			return fmt.Errorf("find notification administrator: %w", err)
		}
		var memberName string
		if err = tx.QueryRow(ctx, `SELECT full_name FROM users WHERE id=$1`, c.UserID).Scan(&memberName); err != nil {
			return fmt.Errorf("load proof member: %w", err)
		}
		id := c.ID
		approveURL := fmt.Sprintf("%s/admin/contributions/%s/review?action=approve&token=%s.approve", s.frontendURL, c.ID, rawToken)
		rejectURL := fmt.Sprintf("%s/admin/contributions/%s/review?action=reject&token=%s.reject", s.frontendURL, c.ID, rawToken)
		proofURL := fmt.Sprintf("%s/api/v1/contributions/%s/proof/review?token=%s", s.apiPublicURL, c.ID, rawToken)
		subject, message := "Contribution proof submitted", fmt.Sprintf("Member: %s\nEmail: %s\nPaid amount: %s\nExpected contribution: %s\nLate fee: %s\nTotal due: %s\nPayment method: %s\nTransaction reference: %s\nDue date: %s\n\nUse the buttons below to view the attached proof and choose a review action.", memberName, email, in.Amount.StringFixed(2), c.ExpectedAmount.StringFixed(2), c.LateFeeAmount.StringFixed(2), c.TotalDue().StringFixed(2), in.PaymentMethod, in.TransactionReference, c.DueDate.Format("2006-01-02"))
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: adminID, ContributionID: &id, Type: "PROOF_SUBMITTED", Channel: "EMAIL", Recipient: adminEmail, Subject: &subject, Message: &message, Status: "PENDING", AttachmentKey: &in.ProofURL, ProofURL: &proofURL, ApproveURL: &approveURL, RejectURL: &rejectURL})
		return err
	})
}
func (s *Service) ValidateProofToken(ctx context.Context, id uuid.UUID, token string) (Contribution, error) {
	c, _, err := s.repo.ReviewData(ctx, id)
	if err != nil || c.Status != "PENDING" || c.ApprovalTokenHash == nil || c.ApprovalTokenExpiresAt == nil || time.Now().After(*c.ApprovalTokenExpiresAt) {
		return Contribution{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid or expired")
	}
	sum := sha256.Sum256([]byte(token))
	if fmt.Sprintf("%x", sum[:]) != *c.ApprovalTokenHash {
		return Contribution{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid or expired")
	}
	return c, nil
}
func (s *Service) ValidateReviewToken(ctx context.Context, id uuid.UUID, in ReviewTokenRequest) (ReviewPreview, error) {
	action := strings.ToLower(in.Action)
	parts := strings.Split(in.Token, ".")
	if len(parts) != 2 || parts[1] != action || (action != "approve" && action != "reject") {
		return ReviewPreview{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid")
	}
	c, name, err := s.repo.ReviewData(ctx, id)
	if err != nil {
		return ReviewPreview{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid")
	}
	if c.ApprovalTokenUsedAt != nil {
		return ReviewPreview{}, httpx.NewError(409, "REVIEW_TOKEN_ALREADY_USED", "Review token has already been used")
	}
	if c.ApprovalTokenExpiresAt == nil || time.Now().After(*c.ApprovalTokenExpiresAt) {
		return ReviewPreview{}, httpx.NewError(400, "REVIEW_TOKEN_EXPIRED", "Review token has expired")
	}
	if c.Status != "PENDING" || c.ApprovalTokenHash == nil {
		return ReviewPreview{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid")
	}
	sum := sha256.Sum256([]byte(parts[0]))
	if fmt.Sprintf("%x", sum[:]) != *c.ApprovalTokenHash {
		return ReviewPreview{}, httpx.NewError(400, "INVALID_REVIEW_TOKEN", "Review token is invalid")
	}
	return ReviewPreview{Valid: true, Action: action, ContributionID: c.ID, MemberName: name, ExpectedAmount: c.ExpectedAmount, LateFeeAmount: c.LateFeeAmount, PaidAmount: c.PaidAmount, PaymentMethod: c.PaymentMethod, TransactionReference: c.TransactionReference}, nil
}
func newReviewToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", fmt.Errorf("generate review token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(value)
	sum := sha256.Sum256([]byte(raw))
	return raw, fmt.Sprintf("%x", sum[:]), nil
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
		subject, message := "Contribution proof rejected", fmt.Sprintf("Your contribution of %s was rejected: %s. Sign in at %s/login and upload a new proof.", c.TotalDue().StringFixed(2), in.Reason, s.frontendURL)
		_, err = s.notifications.Create(ctx, tx, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_REJECTED", Channel: "EMAIL", Recipient: email, Subject: &subject, Message: &message, Status: "PENDING"})
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
	if _, err = s.repo.AdvanceLifecycle(ctx, guard); err != nil {
		return 0, err
	}
	items, err := s.repo.ListReminderCandidates(ctx, guard, limit)
	if err != nil {
		return 0, err
	}
	var adminID uuid.UUID
	var adminEmail string
	if len(items) > 0 {
		if err = guard.QueryRow(ctx, `SELECT id,email FROM users WHERE role='ADMIN' AND status='ACTIVE' ORDER BY created_at LIMIT 1`).Scan(&adminID, &adminEmail); err != nil {
			return 0, fmt.Errorf("find overdue notification administrator: %w", err)
		}
	}
	for _, c := range items {
		u, e := s.users.GetByID(ctx, c.UserID)
		if e != nil {
			return 0, e
		}
		id := c.ID
		subject, message := "Contribution overdue", fmt.Sprintf("Your contribution is overdue. Total amount due: %s. Please sign in and submit payment proof.", c.TotalDue().StringFixed(2))
		if _, e = s.notifications.Create(ctx, guard, notification.Notification{UserID: c.UserID, ContributionID: &id, Type: "CONTRIBUTION_OVERDUE", Channel: "EMAIL", Recipient: u.Email, Subject: &subject, Message: &message, Status: "PENDING"}); e != nil {
			return 0, e
		}
		adminSubject := fmt.Sprintf("Overdue contribution: %s", u.FullName)
		adminMessage := fmt.Sprintf("%s has an overdue contribution. Total amount due: %s. Due date: %s. Please follow up with the member.", u.FullName, c.TotalDue().StringFixed(2), c.DueDate.Format("2006-01-02"))
		if _, e = s.notifications.Create(ctx, guard, notification.Notification{UserID: adminID, ContributionID: &id, Type: "CONTRIBUTION_OVERDUE", Channel: "EMAIL", Recipient: adminEmail, Subject: &adminSubject, Message: &adminMessage, Status: "PENDING"}); e != nil {
			return 0, e
		}
	}
	if err = guard.Commit(ctx); err != nil {
		return 0, err
	}
	return len(items), nil
}
