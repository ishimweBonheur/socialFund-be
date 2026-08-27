package fund

import (
	"context"
	"socialfund/internal/database"
)

type Writer interface {
	Create(context.Context, database.DBTX, FundTransaction) (FundTransaction, error)
}
type PostgresRepository struct{}

func NewRepository() *PostgresRepository { return &PostgresRepository{} }
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, f FundTransaction) (FundTransaction, error) {
	err := db.QueryRow(ctx, `INSERT INTO fund_transactions(user_id,type,direction,amount,contribution_id,assistance_request_id,reference,description,recorded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`, f.UserID, f.Type, f.Direction, f.Amount, f.ContributionID, f.AssistanceRequestID, f.Reference, f.Description, f.RecordedBy).Scan(&f.ID, &f.CreatedAt)
	return f, err
}
