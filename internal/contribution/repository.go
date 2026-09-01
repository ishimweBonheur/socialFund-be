package contribution

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
	"time"
)

type Repository interface {
	GetByID(context.Context, uuid.UUID) (Contribution, error)
	ListByUser(context.Context, uuid.UUID, int, int) ([]Contribution, error)
	Create(context.Context, Contribution) (Contribution, error)
	Lock(context.Context, database.DBTX, uuid.UUID) (Contribution, string, error)
	SetApproved(context.Context, database.DBTX, ApprovalInput) error
	SetRejected(context.Context, database.DBTX, RejectionInput) error
	SubmitProof(context.Context, database.DBTX, ProofInput) error
	SetReviewToken(context.Context, database.DBTX, uuid.UUID, string, time.Time) error
	ListPending(context.Context, int, int) ([]ReviewItem, error)
	ListAdmin(context.Context, AdminListFilter) ([]ReviewItem, int, error)
	Outstanding(context.Context, uuid.UUID) (Outstanding, error)
	AdvanceLifecycle(context.Context, database.DBTX) (int64, error)
	ListReminderCandidates(context.Context, database.DBTX, int) ([]Contribution, error)
	ReviewData(context.Context, uuid.UUID) (Contribution, string, error)
}

func (r *PostgresRepository) ListAdmin(ctx context.Context, f AdminListFilter) ([]ReviewItem, int, error) {
	where := `WHERE ($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR COALESCE(c.transaction_reference,'') ILIKE '%'||$1||'%') AND ($2='' OR c.status=$2) AND ($3::date IS NULL OR c.due_date >= $3::date) AND ($4::date IS NULL OR c.due_date <= $4::date) AND ($5='' OR c.payment_method=$5) AND ($6='' OR ($6='WITH' AND c.proof_uploaded_at IS NOT NULL) OR ($6='WITHOUT' AND c.proof_uploaded_at IS NULL)) AND ($7='' OR ($7='UNPAID' AND COALESCE(c.paid_amount,0)=0) OR ($7='PARTIAL' AND COALESCE(c.paid_amount,0)>0 AND COALESCE(c.paid_amount,0)<c.expected_amount+c.late_fee_amount) OR ($7='PAID' AND COALESCE(c.paid_amount,0)>=c.expected_amount+c.late_fee_amount)) AND ($8='' OR ($8='WITH' AND c.late_fee_amount>0) OR ($8='WITHOUT' AND c.late_fee_amount=0)) AND ($9='' OR ($9='WITH' AND c.transaction_reference IS NOT NULL) OR ($9='WITHOUT' AND c.transaction_reference IS NULL)) AND ($10::date IS NULL OR c.payment_date >= $10::date) AND ($11::date IS NULL OR c.payment_date < $11::date+1) AND ($12::numeric IS NULL OR c.expected_amount+c.late_fee_amount >= $12::numeric) AND ($13::numeric IS NULL OR c.expected_amount+c.late_fee_amount <= $13::numeric)`
	args := []any{f.Search, f.Status, nullableContribution(f.DueFrom), nullableContribution(f.DueTo), f.Method, f.Proof, f.PaymentState, f.LateFee, f.Reference, nullableContribution(f.PaidFrom), nullableContribution(f.PaidTo), nullableContribution(f.AmountMin), nullableContribution(f.AmountMax)}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM contributions c JOIN users u ON u.id=c.user_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+columns+`,u.full_name,u.email FROM contributions c JOIN users u ON u.id=c.user_id `+where+` ORDER BY c.created_at DESC LIMIT $14 OFFSET $15`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]ReviewItem, 0)
	for rows.Next() {
		var item ReviewItem
		var c Contribution
		if err = rows.Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.LateFeePercentage, &c.LateFeeAmount, &c.OverdueAt, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.ApprovalTokenAction, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &item.MemberName, &item.MemberEmail); err != nil {
			return nil, 0, err
		}
		item.Contribution = c
		item.TotalDue = c.TotalDue()
		items = append(items, item)
	}
	return items, total, rows.Err()
}
func nullableContribution(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *PostgresRepository) ReviewData(ctx context.Context, id uuid.UUID) (Contribution, string, error) {
	var name string
	c, err := scanReview(r.db.QueryRow(ctx, `SELECT `+columns+`,u.full_name FROM contributions c JOIN users u ON u.id=c.user_id WHERE c.id=$1`, id), &name)
	return c, name, err
}
func scanReview(row pgx.Row, name *string) (Contribution, error) {
	var c Contribution
	err := row.Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.LateFeePercentage, &c.LateFeeAmount, &c.OverdueAt, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.ApprovalTokenAction, &c.Notes, &c.CreatedAt, &c.UpdatedAt, name)
	return c, err
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const columns = `c.id,c.user_id,c.contribution_plan_id,c.expected_amount,c.late_fee_percentage,c.late_fee_amount,c.overdue_at,c.due_date,c.paid_amount,c.payment_date,c.payment_method,c.transaction_reference,c.proof_url,c.proof_uploaded_at,c.status,c.rejection_reason,c.approved_by,c.approved_at,c.approval_token_hash,c.approval_token_expires_at,c.approval_token_used_at,c.approval_token_action,c.notes,c.created_at,c.updated_at`

func scan(row pgx.Row) (Contribution, error) {
	var c Contribution
	err := row.Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.LateFeePercentage, &c.LateFeeAmount, &c.OverdueAt, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.ApprovalTokenAction, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Contribution, error) {
	return scan(r.db.QueryRow(ctx, `SELECT `+columns+` FROM contributions c WHERE c.id=$1`, id))
}
func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Contribution, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM contributions c WHERE c.user_id=$1 ORDER BY c.due_date DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
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
	err := db.QueryRow(ctx, `SELECT `+columns+`,u.email FROM contributions c JOIN users u ON u.id=c.user_id WHERE c.id=$1 FOR UPDATE OF c`, id).Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.LateFeePercentage, &c.LateFeeAmount, &c.OverdueAt, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.ApprovalTokenAction, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &email)
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
func (r *PostgresRepository) SubmitProof(ctx context.Context, db database.DBTX, in ProofInput) error {
	tag, err := db.Exec(ctx, `UPDATE contributions SET paid_amount=$3,payment_date=NOW(),payment_method=$4,transaction_reference=$5,proof_url=$6,proof_uploaded_at=NOW(),status='PENDING',rejection_reason=NULL,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('DUE','OVERDUE','REJECTED')`, in.ContributionID, in.UserID, in.Amount, in.PaymentMethod, in.TransactionReference, in.ProofURL)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) SetReviewToken(ctx context.Context, db database.DBTX, id uuid.UUID, hash string, expires time.Time) error {
	_, err := db.Exec(ctx, `UPDATE contributions SET approval_token_hash=$2,approval_token_expires_at=$3,approval_token_used_at=NULL,approval_token_action=NULL WHERE id=$1`, id, hash, expires)
	return err
}
func (r *PostgresRepository) ListPending(ctx context.Context, limit, offset int) ([]ReviewItem, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+`,u.full_name,u.email FROM contributions c JOIN users u ON u.id=c.user_id WHERE c.status='PENDING' ORDER BY c.proof_uploaded_at LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ReviewItem, 0)
	for rows.Next() {
		var item ReviewItem
		var c Contribution
		if err = rows.Scan(&c.ID, &c.UserID, &c.ContributionPlanID, &c.ExpectedAmount, &c.LateFeePercentage, &c.LateFeeAmount, &c.OverdueAt, &c.DueDate, &c.PaidAmount, &c.PaymentDate, &c.PaymentMethod, &c.TransactionReference, &c.ProofURL, &c.ProofUploadedAt, &c.Status, &c.RejectionReason, &c.ApprovedBy, &c.ApprovedAt, &c.ApprovalTokenHash, &c.ApprovalTokenExpiresAt, &c.ApprovalTokenUsedAt, &c.ApprovalTokenAction, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &item.MemberName, &item.MemberEmail); err != nil {
			return nil, err
		}
		item.Contribution = c
		item.TotalDue = c.TotalDue()
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) Outstanding(ctx context.Context, userID uuid.UUID) (Outstanding, error) {
	var o Outstanding
	err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(expected_amount+late_fee_amount),0),COUNT(*) FROM contributions WHERE user_id=$1 AND status IN ('OVERDUE','REJECTED')`, userID).Scan(&o.OutstandingAmount, &o.OverdueCount)
	return o, err
}
func (r *PostgresRepository) AdvanceLifecycle(ctx context.Context, db database.DBTX) (int64, error) {
	_, err := db.Exec(ctx, `
WITH plan_dates AS (
    SELECT p.id plan_id,p.user_id,p.amount,u.status_changed_at::date active_since,
           CASE p.frequency
             WHEN 'MONTHLY' THEN (month_start + (LEAST(p.due_day,EXTRACT(DAY FROM month_start + INTERVAL '1 month - 1 day')::int)-1) * INTERVAL '1 day')::date
             ELSE generated::date
           END due_date
    FROM contribution_plans p
    JOIN users u ON u.id=p.user_id AND u.status='ACTIVE'
    CROSS JOIN LATERAL generate_series(
      CASE WHEN p.frequency='MONTHLY' THEN date_trunc('month',GREATEST(p.start_date,u.status_changed_at::date))::date ELSE GREATEST(p.start_date,u.status_changed_at::date) END,
      LEAST(CURRENT_DATE,COALESCE(p.end_date,CURRENT_DATE)),
      CASE p.frequency WHEN 'DAILY' THEN INTERVAL '1 day' WHEN 'WEEKLY' THEN INTERVAL '7 days' WHEN 'MONTHLY' THEN INTERVAL '1 month' ELSE make_interval(days=>p.interval_value) END
    ) generated
    CROSS JOIN LATERAL (SELECT date_trunc('month',generated)::date month_start) m
    WHERE p.is_active
)
INSERT INTO contributions(user_id,contribution_plan_id,expected_amount,due_date,status)
SELECT user_id,plan_id,amount,due_date,CASE WHEN due_date<=CURRENT_DATE THEN 'DUE' ELSE 'UPCOMING' END
FROM plan_dates WHERE due_date>=active_since
ON CONFLICT(contribution_plan_id,due_date) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	_, err = db.Exec(ctx, `UPDATE contributions c SET status='DUE',updated_at=NOW() FROM users u WHERE u.id=c.user_id AND u.status='ACTIVE' AND c.status='UPCOMING' AND c.due_date<=CURRENT_DATE`)
	if err != nil {
		return 0, err
	}
	tag, err := db.Exec(ctx, `UPDATE contributions c SET status='OVERDUE',late_fee_percentage=CASE WHEN p.late_fee_enabled THEN p.late_fee_percentage END,late_fee_amount=CASE WHEN p.late_fee_enabled THEN ROUND(c.expected_amount*p.late_fee_percentage/100,2) ELSE 0 END,overdue_at=NOW(),updated_at=NOW() FROM contribution_plans p JOIN users u ON u.id=p.user_id AND u.status='ACTIVE' WHERE p.id=c.contribution_plan_id AND c.status='DUE' AND CURRENT_DATE>c.due_date+p.grace_period_days`)
	return tag.RowsAffected(), err
}
func (r *PostgresRepository) ListReminderCandidates(ctx context.Context, db database.DBTX, limit int) ([]Contribution, error) {
	rows, err := db.Query(ctx, `SELECT `+columns+` FROM contributions c JOIN contribution_plans p ON p.id=c.contribution_plan_id JOIN users u ON u.id=c.user_id AND u.status='ACTIVE' WHERE c.status IN ('OVERDUE','REJECTED') AND p.reminder_enabled AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.contribution_id=c.id AND n.type='CONTRIBUTION_OVERDUE' AND n.created_at>=NOW()-make_interval(days=>CASE p.reminder_frequency WHEN 'WEEKLY' THEN 7 WHEN 'CUSTOM' THEN p.reminder_interval ELSE 1 END)) ORDER BY c.due_date LIMIT $1 FOR UPDATE OF c SKIP LOCKED`, limit)
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
