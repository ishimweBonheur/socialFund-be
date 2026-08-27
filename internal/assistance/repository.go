package assistance

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Repository interface {
	GetByID(context.Context, uuid.UUID) (AssistanceRequest, error)
	Create(context.Context, AssistanceRequest) (AssistanceRequest, error)
	CreateWithDB(context.Context, database.DBTX, AssistanceRequest) (AssistanceRequest, error)
	Lock(context.Context, database.DBTX, uuid.UUID) (AssistanceRequest, string, error)
	SetPaid(context.Context, database.DBTX, DisbursementInput) error
	SetApproved(context.Context, database.DBTX, ApprovalInput) error
	SetRejected(context.Context, database.DBTX, RejectionInput) error
	List(context.Context, ListFilter) ([]ReviewItem, error)
}
type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (AssistanceRequest, error) {
	var a AssistanceRequest
	err := r.db.QueryRow(ctx, `SELECT id,user_id,amount_requested,reason,description,attachment_url,status,amount_approved,reviewed_by,reviewed_at,rejection_reason,amount_disbursed,disbursement_method,disbursement_reference,disbursed_by,disbursed_at,created_at,updated_at FROM assistance_requests WHERE id=$1`, id).Scan(&a.ID, &a.UserID, &a.AmountRequested, &a.Reason, &a.Description, &a.AttachmentURL, &a.Status, &a.AmountApproved, &a.ReviewedBy, &a.ReviewedAt, &a.RejectionReason, &a.AmountDisbursed, &a.DisbursementMethod, &a.DisbursementReference, &a.DisbursedBy, &a.DisbursedAt, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}
func (r *PostgresRepository) Create(ctx context.Context, a AssistanceRequest) (AssistanceRequest, error) {
	return r.CreateWithDB(ctx, r.db, a)
}
func (r *PostgresRepository) CreateWithDB(ctx context.Context, db database.DBTX, a AssistanceRequest) (AssistanceRequest, error) {
	err := db.QueryRow(ctx, `INSERT INTO assistance_requests(user_id,amount_requested,reason,description,attachment_url) VALUES($1,$2,$3,$4,$5) RETURNING id,status,created_at,updated_at`, a.UserID, a.AmountRequested, a.Reason, a.Description, a.AttachmentURL).Scan(&a.ID, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}
func (r *PostgresRepository) Lock(ctx context.Context, db database.DBTX, id uuid.UUID) (AssistanceRequest, string, error) {
	var a AssistanceRequest
	var email string
	err := db.QueryRow(ctx, `SELECT a.id,a.user_id,a.amount_requested,a.reason,a.description,a.attachment_url,a.status,a.amount_approved,a.reviewed_by,a.reviewed_at,a.rejection_reason,a.amount_disbursed,a.disbursement_method,a.disbursement_reference,a.disbursed_by,a.disbursed_at,a.created_at,a.updated_at,u.email FROM assistance_requests a JOIN users u ON u.id=a.user_id WHERE a.id=$1 FOR UPDATE OF a`, id).Scan(&a.ID, &a.UserID, &a.AmountRequested, &a.Reason, &a.Description, &a.AttachmentURL, &a.Status, &a.AmountApproved, &a.ReviewedBy, &a.ReviewedAt, &a.RejectionReason, &a.AmountDisbursed, &a.DisbursementMethod, &a.DisbursementReference, &a.DisbursedBy, &a.DisbursedAt, &a.CreatedAt, &a.UpdatedAt, &email)
	return a, email, err
}
func (r *PostgresRepository) SetPaid(ctx context.Context, db database.DBTX, in DisbursementInput) error {
	tag, err := db.Exec(ctx, `UPDATE assistance_requests SET status='PAID',amount_disbursed=$2,disbursement_method=$3,disbursement_reference=$4,disbursed_by=$5,disbursed_at=NOW(),updated_at=NOW() WHERE id=$1 AND status='APPROVED'`, in.AssistanceRequestID, in.Amount, in.Method, in.Reference, in.AdminID)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) SetApproved(ctx context.Context, db database.DBTX, in ApprovalInput) error {
	tag, err := db.Exec(ctx, `UPDATE assistance_requests SET status='APPROVED',amount_approved=$2,reviewed_by=$3,reviewed_at=NOW(),rejection_reason=NULL,updated_at=NOW() WHERE id=$1 AND status='PENDING'`, in.AssistanceRequestID, in.AmountApproved, in.AdminID)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) SetRejected(ctx context.Context, db database.DBTX, in RejectionInput) error {
	tag, err := db.Exec(ctx, `UPDATE assistance_requests SET status='REJECTED',rejection_reason=$2,reviewed_by=$3,reviewed_at=NOW(),updated_at=NOW() WHERE id=$1 AND status='PENDING'`, in.AssistanceRequestID, in.Reason, in.AdminID)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return err
}
func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]ReviewItem, error) {
	rows, err := r.db.Query(ctx, `SELECT a.id,a.user_id,a.amount_requested,a.reason,a.description,a.attachment_url,a.status,a.amount_approved,a.reviewed_by,a.reviewed_at,a.rejection_reason,a.amount_disbursed,a.disbursement_method,a.disbursement_reference,a.disbursed_by,a.disbursed_at,a.created_at,a.updated_at,u.full_name,u.email FROM assistance_requests a JOIN users u ON u.id=a.user_id WHERE ($1::uuid IS NULL OR a.user_id=$1) AND ($2='' OR a.status=$2) ORDER BY a.created_at DESC LIMIT $3 OFFSET $4`, f.UserID, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ReviewItem, 0)
	for rows.Next() {
		var i ReviewItem
		err = rows.Scan(&i.ID, &i.UserID, &i.AmountRequested, &i.Reason, &i.Description, &i.AttachmentURL, &i.Status, &i.AmountApproved, &i.ReviewedBy, &i.ReviewedAt, &i.RejectionReason, &i.AmountDisbursed, &i.DisbursementMethod, &i.DisbursementReference, &i.DisbursedBy, &i.DisbursedAt, &i.CreatedAt, &i.UpdatedAt, &i.MemberName, &i.MemberEmail)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
