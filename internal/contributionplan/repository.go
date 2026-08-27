package contributionplan

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Repository interface {
	GetActiveByUserID(context.Context, uuid.UUID) (ContributionPlan, error)
	Create(context.Context, ContributionPlan) (ContributionPlan, error)
	CreateWithDB(context.Context, database.DBTX, ContributionPlan) (ContributionPlan, error)
}
type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }
func (r *PostgresRepository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) (ContributionPlan, error) {
	var p ContributionPlan
	err := r.db.QueryRow(ctx, `SELECT id,user_id,amount,frequency,interval_value,due_day,start_date,end_date,reminder_enabled,reminder_frequency,reminder_interval,is_active,created_by,created_at,updated_at FROM contribution_plans WHERE user_id=$1 AND is_active`, userID).Scan(&p.ID, &p.UserID, &p.Amount, &p.Frequency, &p.IntervalValue, &p.DueDay, &p.StartDate, &p.EndDate, &p.ReminderEnabled, &p.ReminderFrequency, &p.ReminderInterval, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (r *PostgresRepository) Create(ctx context.Context, p ContributionPlan) (ContributionPlan, error) {
	return r.CreateWithDB(ctx, r.db, p)
}
func (r *PostgresRepository) CreateWithDB(ctx context.Context, db database.DBTX, p ContributionPlan) (ContributionPlan, error) {
	err := db.QueryRow(ctx, `INSERT INTO contribution_plans(user_id,amount,frequency,interval_value,due_day,start_date,end_date,reminder_enabled,reminder_frequency,reminder_interval,is_active,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,created_at,updated_at`, p.UserID, p.Amount, p.Frequency, p.IntervalValue, p.DueDay, p.StartDate, p.EndDate, p.ReminderEnabled, p.ReminderFrequency, p.ReminderInterval, p.IsActive, p.CreatedBy).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
