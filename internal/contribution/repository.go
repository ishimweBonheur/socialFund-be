package contribution

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Repository interface {
	GetByID(context.Context, uuid.UUID) (Contribution, error)
	ListByUser(context.Context, uuid.UUID, int, int) ([]Contribution, error)
	Create(context.Context, Contribution) (Contribution, error)
	Lock(context.Context, database.DBTX, uuid.UUID) (Contribution, string, error)
	SetApproved(context.Context, database.DBTX, ApprovalInput) error
	SetRejected(context.Context, database.DBTX, RejectionInput) error
	MarkDueAsOverdue(context.Context) (int64, error)
	ListOverdueForReminders(context.Context, int) ([]Contribution, error)
}
type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

const columns = `id,user_id,contribution_plan_id,expected_amount,due_date,paid_amount,payment_date,payment_method,transaction_reference,proof_url,proof_uploaded_at,status,rejection_reason,approved_by,approved_at,approval_token_hash,approval_token_expires_at,approval_token_used_at,notes,created_at,updated_at`

func scan(row pgx.Row) (Contribution, error) {
	var c Contribution
	err := row.Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Contribution, error) {
	return scan(r.db.QueryRow(ctx, `SELECT `+columns+` FROM contributions WHERE id=$1`, id))
}
func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Contribution, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM contributions WHERE user_id=$1 ORDER BY due_date DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Contribution, 0)
	for rows.Next() {
		c, e := scan(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) Create(ctx context.Context, c Contribution) (Contribution, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO contributions(user_id,contribution_plan_id,expected_amount,due_date,status) VALUES($1,$2,$3,$4,$5) RETURNING id,created_at,updated_at`, c.UserID, c.ContributionPlanID, c.ExpectedAmount, c.DueDate, c.Status).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (r *PostgresRepository) Lock(ctx context.Context, db database.DBTX, id uuid.UUID) (Contribution, string, error) {
	var c Contribution
	var email string
	err := db.QueryRow(ctx, `SELECT c.id,c.user_id,c.contribution_plan_id,c.expected_amount,c.due_date,c.paid_amount,c.payment_date,c.payment_method,c.transaction_reference,c.proof_url,c.proof_uploaded_at,c.status,c.rejection_reason,c.approved_by,c.approved_at,c.approval_token_hash,c.approval_token_expires_at,c.approval_token_used_at,c.notes,c.created_at,c.updated_at,u.email FROM contributions c JOIN users u ON u.id=c.user_id WHERE c.id=$1 FOR UPDATE OF c`, id).Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &email)
	return c, email, err
}
func (r *PostgresRepository) SetApproved(ctx context.Context, db database.DBTX, in ApprovalInput) error {
	tag, err := db.Exec(ctx, `UPDATE contributions SET status='APPROVED',approved_by=$2,approved_at=NOW(),approval_token_used_at=COALESCE(approval_token_used_at,NOW()),notes=COALESCE($3,notes),updated_at=NOW() WHERE id=$1 AND status='PENDING'`, in.ContributionID, in.AdminID, in.Notes)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) SetRejected(ctx context.Context, db database.DBTX, in RejectionInput) error {
	tag, err := db.Exec(ctx, `UPDATE contributions SET status='REJECTED',rejection_reason=$2,approved_by=NULL,approved_at=NULL,approval_token_used_at=COALESCE(approval_token_used_at,NOW()),updated_at=NOW() WHERE id=$1 AND status='PENDING'`, in.ContributionID, in.Reason)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) MarkDueAsOverdue(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `UPDATE contributions SET status='OVERDUE',updated_at=NOW() WHERE status='DUE' AND due_date<CURRENT_DATE`)
	return tag.RowsAffected(), err
}
func (r *PostgresRepository) ListOverdueForReminders(ctx context.Context, limit int) ([]Contribution, error) {
	rows, err := r.db.Query(ctx, `SELECT c.id,c.user_id,c.contribution_plan_id,c.expected_amount,c.due_date,c.paid_amount,c.payment_date,c.payment_method,c.transaction_reference,c.proof_url,c.proof_uploaded_at,c.status,c.rejection_reason,c.approved_by,c.approved_at,c.approval_token_hash,c.approval_token_expires_at,c.approval_token_used_at,c.notes,c.created_at,c.updated_at FROM contributions c JOIN contribution_plans p ON p.id=c.contribution_plan_id WHERE c.status='OVERDUE' AND p.reminder_enabled AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.contribution_id=c.id AND n.type='CONTRIBUTION_OVERDUE' AND n.created_at>=NOW()-make_interval(days=>COALESCE(p.reminder_interval,CASE p.reminder_frequency WHEN 'WEEKLY' THEN 7 ELSE 1 END))) ORDER BY c.due_date LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Contribution, 0)
	for rows.Next() {
		c, e := scan(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
