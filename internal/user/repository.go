package user

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/database"
)

type Repository interface {
	GetByID(context.Context, uuid.UUID) (User, error)
	GetByEmail(context.Context, string) (User, error)
	Create(context.Context, User) (User, error)
	CreateWithDB(context.Context, database.DBTX, User) (User, error)
	LockByEmail(context.Context, database.DBTX, string) (User, error)
	Activate(context.Context, database.DBTX, uuid.UUID, string) error
	RecordLogin(context.Context, database.DBTX, uuid.UUID) error
	List(context.Context, ListFilter) ([]User, error)
	Count(context.Context, ListFilter) (int, error)
	LockByID(context.Context, database.DBTX, uuid.UUID) (User, error)
	Update(context.Context, database.DBTX, uuid.UUID, UpdateInput) error
	SetStatus(context.Context, database.DBTX, uuid.UUID, string, string) error
}

func (r *PostgresRepository) Count(ctx context.Context, f ListFilter) (int, error) {
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE ($1='' OR status=$1) AND ($2='' OR role=$2) AND ($3='' OR full_name ILIKE '%'||$3||'%' OR email ILIKE '%'||$3||'%' OR phone ILIKE '%'||$3||'%') AND ($4::date IS NULL OR created_at >= $4::date) AND ($5::date IS NULL OR created_at < $5::date+1)`, f.Status, f.Role, f.Search, nullableDate(f.DateFrom), nullableDate(f.DateTo)).Scan(&total)
	return total, err
}

func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]User, error) {
	rows, err := r.db.Query(ctx, `SELECT id,full_name,email,phone,google_id,role,status,last_login_at,created_by,created_at,updated_at FROM users WHERE ($1='' OR status=$1) AND ($2='' OR role=$2) AND ($3='' OR full_name ILIKE '%'||$3||'%' OR email ILIKE '%'||$3||'%' OR phone ILIKE '%'||$3||'%') AND ($4::date IS NULL OR created_at >= $4::date) AND ($5::date IS NULL OR created_at < $5::date+1) ORDER BY created_at DESC LIMIT $6 OFFSET $7`, f.Status, f.Role, f.Search, nullableDate(f.DateFrom), nullableDate(f.DateTo), f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]User, 0)
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.GoogleID, &u.Role, &u.Status, &u.LastLoginAt, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, rows.Err()
}
func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func (r *PostgresRepository) LockByID(ctx context.Context, db database.DBTX, id uuid.UUID) (User, error) {
	var u User
	err := db.QueryRow(ctx, `SELECT id,full_name,email,phone,google_id,role,status,last_login_at,created_by,created_at,updated_at FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.GoogleID, &u.Role, &u.Status, &u.LastLoginAt, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
func (r *PostgresRepository) Update(ctx context.Context, db database.DBTX, id uuid.UUID, in UpdateInput) error {
	_, err := db.Exec(ctx, `UPDATE users SET full_name=COALESCE($2,full_name),email=COALESCE(lower($3),email),phone=COALESCE($4,phone),updated_at=NOW() WHERE id=$1`, id, in.FullName, in.Email, in.Phone)
	return err
}
func (r *PostgresRepository) SetStatus(ctx context.Context, db database.DBTX, id uuid.UUID, from, to string) error {
	tag, err := db.Exec(ctx, `UPDATE users SET status=$3,updated_at=NOW() WHERE id=$1 AND status=$2`, id, from, to)
	if err == nil && tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

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
	return r.CreateWithDB(ctx, r.db, u)
}
func (r *PostgresRepository) CreateWithDB(ctx context.Context, db database.DBTX, u User) (User, error) {
	err := db.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,google_id,role,status,created_by) VALUES($1,lower($2),$3,$4,$5,$6,$7) RETURNING id,created_at,updated_at`, u.FullName, u.Email, u.Phone, u.GoogleID, u.Role, u.Status, u.CreatedBy).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
func (r *PostgresRepository) LockByEmail(ctx context.Context, db database.DBTX, email string) (User, error) {
	var u User
	err := db.QueryRow(ctx, `SELECT id,full_name,email,phone,google_id,role,status,last_login_at,created_by,created_at,updated_at FROM users WHERE lower(email)=lower($1) FOR UPDATE`, email).Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.GoogleID, &u.Role, &u.Status, &u.LastLoginAt, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
func (r *PostgresRepository) Activate(ctx context.Context, db database.DBTX, id uuid.UUID, googleID string) error {
	_, err := db.Exec(ctx, `UPDATE users SET status='ACTIVE',google_id=$2,last_login_at=NOW(),updated_at=NOW() WHERE id=$1 AND status='INACTIVE'`, id, googleID)
	return err
}
func (r *PostgresRepository) RecordLogin(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE users SET last_login_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
	return err
}
