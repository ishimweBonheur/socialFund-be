package fund

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Writer interface {
	Create(context.Context, database.DBTX, FundTransaction) (FundTransaction, error)
}
type Reader interface {
	Summary(context.Context) (Summary, error)
	SummaryForUser(context.Context, uuid.UUID) (MemberSummary, error)
	List(context.Context, Filter) ([]FundTransaction, error)
	Count(context.Context, Filter) (int, error)
}
type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(databases ...*pgxpool.Pool) *PostgresRepository {
	var db *pgxpool.Pool
	if len(databases) > 0 {
		db = databases[0]
	}
	return &PostgresRepository{db: db}
}
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, f FundTransaction) (FundTransaction, error) {
	err := db.QueryRow(ctx, `INSERT INTO fund_transactions(user_id,type,direction,amount,contribution_id,assistance_request_id,reference,description,recorded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`, f.UserID, f.Type, f.Direction, f.Amount, f.ContributionID, f.AssistanceRequestID, f.Reference, f.Description, f.RecordedBy).Scan(&f.ID, &f.CreatedAt)
	return f, err
}
func (r *PostgresRepository) Summary(ctx context.Context) (Summary, error) {
	var s Summary
	err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount) FILTER(WHERE direction='IN'),0),COALESCE(SUM(amount) FILTER(WHERE direction='OUT'),0),COALESCE(SUM(CASE direction WHEN 'IN' THEN amount ELSE -amount END),0) FROM fund_transactions`).Scan(&s.TotalIn, &s.TotalOut, &s.Balance)
	return s, err
}
func (r *PostgresRepository) SummaryForUser(ctx context.Context, userID uuid.UUID) (MemberSummary, error) {
	var s MemberSummary
	err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount) FILTER(WHERE type='CONTRIBUTION' AND direction='IN'),0),COALESCE(SUM(amount) FILTER(WHERE type='ASSISTANCE' AND direction='OUT'),0),COALESCE(SUM(CASE WHEN direction='IN' THEN amount ELSE -amount END),0) FROM fund_transactions WHERE user_id=$1`, userID).Scan(&s.TotalContributed, &s.AssistanceReceived, &s.NetPosition)
	return s, err
}
func (r *PostgresRepository) List(ctx context.Context, f Filter) ([]FundTransaction, error) {
	rows, err := r.db.Query(ctx, `SELECT ft.id,ft.user_id,u.full_name,ft.type,ft.direction,ft.amount,'COMPLETED',COALESCE(c.payment_method,ar.disbursement_method),ft.contribution_id,ft.assistance_request_id,COALESCE(ft.reference,c.transaction_reference,ar.disbursement_reference),ft.description,ft.recorded_by,ft.created_at FROM fund_transactions ft JOIN users u ON u.id=ft.user_id LEFT JOIN contributions c ON c.id=ft.contribution_id LEFT JOIN assistance_requests ar ON ar.id=ft.assistance_request_id WHERE ($1='' OR ft.type=$1) AND ($2='' OR ft.direction=$2) AND ($3::date IS NULL OR ft.created_at >= $3::date) AND ($4::date IS NULL OR ft.created_at < $4::date+1) AND ($5::uuid IS NULL OR ft.user_id=$5) ORDER BY ft.created_at DESC LIMIT $6 OFFSET $7`, f.Type, f.Direction, nullString(f.DateFrom), nullString(f.DateTo), f.UserID, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]FundTransaction, 0)
	for rows.Next() {
		var item FundTransaction
		if err = rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.Type, &item.Direction, &item.Amount, &item.Status, &item.PaymentMethod, &item.ContributionID, &item.AssistanceRequestID, &item.Reference, &item.Description, &item.RecordedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) Count(ctx context.Context, f Filter) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM fund_transactions ft WHERE ($1='' OR ft.type=$1) AND ($2='' OR ft.direction=$2) AND ($3::date IS NULL OR ft.created_at >= $3::date) AND ($4::date IS NULL OR ft.created_at < $4::date+1) AND ($5::uuid IS NULL OR ft.user_id=$5)`, f.Type, f.Direction, nullString(f.DateFrom), nullString(f.DateTo), f.UserID).Scan(&count)
	return count, err
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
