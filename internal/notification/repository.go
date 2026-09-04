package notification

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"socialfund/internal/database"
	"time"
)

type Writer interface {
	Create(context.Context, database.DBTX, Notification) (Notification, error)
}
type Repository interface {
	Writer
	ListReady(context.Context, int) ([]Notification, error)
	MarkSent(context.Context, uuid.UUID) error
	MarkFailed(context.Context, uuid.UUID, string, time.Time) error
	LoadAccountCreatedEmailData(context.Context, uuid.UUID) (AccountCreatedEmailData, error)
}
type AdminRepository interface {
	List(context.Context, Filter) ([]Notification, error)
	Retry(context.Context, database.DBTX, uuid.UUID) error
	MarkRead(context.Context, uuid.UUID, uuid.UUID) error
	MarkAllRead(context.Context, uuid.UUID) error
}

func (r *PostgresRepository) List(ctx context.Context, f Filter) ([]Notification, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,contribution_id,type,channel,recipient,subject,message,status,attempts,last_error,next_retry_at,sent_at,read_at,attachment_key,proof_url,approve_url,reject_url,created_at FROM notifications WHERE ($1='' OR status=$1) AND ($2='' OR type=$2) AND ($3::uuid IS NULL OR user_id=$3) AND ($4::date IS NULL OR created_at >= $4::date) AND ($5::date IS NULL OR created_at < $5::date+1) ORDER BY created_at DESC LIMIT $6 OFFSET $7`, f.Status, f.Type, f.UserID, nullable(f.DateFrom), nullable(f.DateTo), f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, &n.ContributionID, &n.Type, &n.Channel, &n.Recipient, &n.Subject, &n.Message, &n.Status, &n.Attempts, &n.LastError, &n.NextRetryAt, &n.SentAt, &n.ReadAt, &n.AttachmentKey, &n.ProofURL, &n.ApproveURL, &n.RejectURL, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,NOW()) WHERE id=$1 AND user_id=$2`, id, userID)
	if err == nil && tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}
func (r *PostgresRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, userID)
	return err
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (r *PostgresRepository) Retry(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `UPDATE notifications SET status='PENDING',next_retry_at=NOW(),last_error=NULL WHERE id=$1 AND status='FAILED'`, id)
	if err == nil && tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, n Notification) (Notification, error) {
	err := db.QueryRow(ctx, `INSERT INTO notifications(user_id,contribution_id,type,channel,recipient,subject,message,status,attachment_key,proof_url,approve_url,reject_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,attempts,created_at`, n.UserID, n.ContributionID, n.Type, n.Channel, n.Recipient, n.Subject, n.Message, n.Status, n.AttachmentKey, n.ProofURL, n.ApproveURL, n.RejectURL).Scan(&n.ID, &n.Attempts, &n.CreatedAt)
	return n, err
}
func (r *PostgresRepository) ListReady(ctx context.Context, limit int) ([]Notification, error) {
	rows, err := r.db.Query(ctx, `UPDATE notifications SET status='PROCESSING',attempts=attempts+1 WHERE id IN (SELECT id FROM notifications WHERE channel='EMAIL' AND (status='PENDING' OR (status='FAILED' AND next_retry_at<=NOW())) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id,user_id,contribution_id,type,channel,recipient,subject,message,status,attempts,last_error,next_retry_at,sent_at,read_at,attachment_key,proof_url,approve_url,reject_url,created_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, &n.ContributionID, &n.Type, &n.Channel, &n.Recipient, &n.Subject, &n.Message, &n.Status, &n.Attempts, &n.LastError, &n.NextRetryAt, &n.SentAt, &n.ReadAt, &n.AttachmentKey, &n.ProofURL, &n.ApproveURL, &n.RejectURL, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE notifications SET status='SENT',sent_at=NOW(),last_error=NULL,next_retry_at=NULL WHERE id=$1 AND status='PROCESSING'`, id)
	if err == nil && tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}
func (r *PostgresRepository) MarkFailed(ctx context.Context, id uuid.UUID, message string, next time.Time) error {
	tag, err := r.db.Exec(ctx, `UPDATE notifications SET status='FAILED',last_error=$2,next_retry_at=$3 WHERE id=$1 AND status='PROCESSING'`, id, message, next)
	if err == nil && tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}
func (r *PostgresRepository) LoadAccountCreatedEmailData(ctx context.Context, userID uuid.UUID) (AccountCreatedEmailData, error) {
	var data AccountCreatedEmailData
	var amount decimal.Decimal
	var frequency string
	var dueDay, interval *int
	err := r.db.QueryRow(ctx, `SELECT u.full_name,u.email,u.phone,p.amount,p.frequency,p.due_day,p.interval_value FROM users u JOIN contribution_plans p ON p.user_id=u.id AND p.is_active WHERE u.id=$1`, userID).Scan(&data.FullName, &data.Email, &data.Phone, &amount, &frequency, &dueDay, &interval)
	if err != nil {
		return AccountCreatedEmailData{}, err
	}
	data.ContributionAmount = formatAmount(amount)
	data.ContributionFrequency = formatFrequency(frequency)
	data.PaymentDue = formatPaymentDue(frequency, dueDay, interval)
	return data, nil
}
