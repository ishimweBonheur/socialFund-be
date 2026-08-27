package audit

import (
	"context"
	"socialfund/internal/database"
)

type Writer interface {
	Create(context.Context, database.DBTX, AuditLog) (AuditLog, error)
}
type PostgresRepository struct{}

func NewRepository() *PostgresRepository { return &PostgresRepository{} }
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, a AuditLog) (AuditLog, error) {
	err := db.QueryRow(ctx, `INSERT INTO audit_logs(user_id,action,entity_type,entity_id,old_data,new_data,ip_address,user_agent) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, a.UserID, a.Action, a.EntityType, a.EntityID, a.OldData, a.NewData, a.IPAddress, a.UserAgent).Scan(&a.ID, &a.CreatedAt)
	return a, err
}
