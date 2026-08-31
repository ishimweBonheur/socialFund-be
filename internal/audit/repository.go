package audit

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Writer interface {
	Create(context.Context, database.DBTX, AuditLog) (AuditLog, error)
}
type Filter struct {
	UserID             *uuid.UUID
	Action, EntityType string
	EntityID           *uuid.UUID
	DateFrom, DateTo   string
	Limit, Offset      int
}
type Reader interface {
	List(context.Context, Filter) ([]AuditLog, error)
	Count(context.Context, Filter) (int, error)
}

func (r *PostgresRepository) Count(ctx context.Context, f Filter) (int, error) {
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE ($1::uuid IS NULL OR user_id=$1) AND ($2='' OR action=$2) AND ($3='' OR entity_type=$3) AND ($4::uuid IS NULL OR entity_id=$4) AND ($5::date IS NULL OR created_at >= $5::date) AND ($6::date IS NULL OR created_at < $6::date+1)`, f.UserID, f.Action, f.EntityType, f.EntityID, nullable(f.DateFrom), nullable(f.DateTo)).Scan(&total)
	return total, err
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(databases ...*pgxpool.Pool) *PostgresRepository {
	var db *pgxpool.Pool
	if len(databases) > 0 {
		db = databases[0]
	}
	return &PostgresRepository{db: db}
}
func (r *PostgresRepository) Create(ctx context.Context, db database.DBTX, a AuditLog) (AuditLog, error) {
	a = enrichFromContext(ctx, a)
	err := db.QueryRow(ctx, `INSERT INTO audit_logs(user_id,action,entity_type,entity_id,old_data,new_data,ip_address,user_agent) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, a.UserID, a.Action, a.EntityType, a.EntityID, a.OldData, a.NewData, a.IPAddress, a.UserAgent).Scan(&a.ID, &a.CreatedAt)
	return a, err
}
func (r *PostgresRepository) List(ctx context.Context, f Filter) ([]AuditLog, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,action,entity_type,entity_id,old_data,new_data,host(ip_address),user_agent,created_at FROM audit_logs WHERE ($1::uuid IS NULL OR user_id=$1) AND ($2='' OR action=$2) AND ($3='' OR entity_type=$3) AND ($4::uuid IS NULL OR entity_id=$4) AND ($5::date IS NULL OR created_at >= $5::date) AND ($6::date IS NULL OR created_at < $6::date+1) ORDER BY created_at DESC LIMIT $7 OFFSET $8`, f.UserID, f.Action, f.EntityType, f.EntityID, nullable(f.DateFrom), nullable(f.DateTo), f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AuditLog, 0)
	for rows.Next() {
		var a AuditLog
		if err = rows.Scan(&a.ID, &a.UserID, &a.Action, &a.EntityType, &a.EntityID, &a.OldData, &a.NewData, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
