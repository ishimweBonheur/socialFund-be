package notification

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
}
type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, n Notification) (Notification, error) {
	err := db.QueryRow(ctx, `INSERT INTO notifications(user_id,contribution_id,assistance_request_id,type,channel,recipient,subject,message,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,attempts,created_at`, n.UserID, n.ContributionID, n.AssistanceRequestID, n.Type, n.Channel, n.Recipient, n.Subject, n.Message, n.Status).Scan(&n.ID, &n.Attempts, &n.CreatedAt)
	return n, err
}
func (r *PostgresRepository) ListReady(ctx context.Context, limit int) ([]Notification, error) {
	rows, err := r.db.Query(ctx, `UPDATE notifications SET status='PROCESSING',attempts=attempts+1 WHERE id IN (SELECT id FROM notifications WHERE status='PENDING' OR (status='FAILED' AND next_retry_at<=NOW()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id,user_id,contribution_id,assistance_request_id,type,channel,recipient,subject,message,status,attempts,last_error,next_retry_at,sent_at,created_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, &n.ContributionID, &n.AssistanceRequestID, &n.Type, &n.Channel, &n.Recipient, &n.Subject, &n.Message, &n.Status, &n.Attempts, &n.LastError, &n.NextRetryAt, &n.SentAt, &n.CreatedAt); err != nil {
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
