package user

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetByID(context.Context, uuid.UUID) (User, error)
	GetByEmail(context.Context, string) (User, error)
	Create(context.Context, User) (User, error)
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	return r.get(ctx, `SELECT id,full_name,email,phone,google_id,role,status,last_login_at,created_by,created_at,updated_at FROM users WHERE id=$1`, id)
}
func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	return r.get(ctx, `SELECT id,full_name,email,phone,google_id,role,status,last_login_at,created_by,created_at,updated_at FROM users WHERE lower(email)=lower($1)`, email)
}
func (r *PostgresRepository) get(ctx context.Context, query string, arg any) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, query, arg).Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.GoogleID, &u.Role, &u.Status, &u.LastLoginAt, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
func (r *PostgresRepository) Create(ctx context.Context, u User) (User, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,google_id,role,status,created_by) VALUES($1,lower($2),$3,$4,$5,$6,$7) RETURNING id,created_at,updated_at`, u.FullName, u.Email, u.Phone, u.GoogleID, u.Role, u.Status, u.CreatedBy).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
