package notification

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"time"
)

type Service struct {
	repo  Repository
	pool  *pgxpool.Pool
	audit audit.Writer
}

func NewService(repo Repository, extras ...any) *Service {
	s := &Service{repo: repo}
	if len(extras) > 1 {
		s.pool, _ = extras[0].(*pgxpool.Pool)
		s.audit, _ = extras[1].(audit.Writer)
	}
	return s
}
func (s *Service) Claim(ctx context.Context, limit int) ([]Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.repo.ListReady(ctx, limit)
}
func (s *Service) List(ctx context.Context, f Filter) ([]Notification, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.(AdminRepository).List(ctx, f)
}
func (s *Service) Retry(ctx context.Context, adminID, id uuid.UUID) error {
	if s.pool == nil {
		return pgx.ErrNoRows
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.(AdminRepository).Retry(ctx, tx, id); err != nil {
			return err
		}
		admin := adminID
		_, err := s.audit.Create(ctx, tx, audit.AuditLog{UserID: &admin, Action: "NOTIFICATION_RETRY_REQUESTED", EntityType: "NOTIFICATION", EntityID: id})
		return err
	})
}
func (s *Service) MarkSent(ctx context.Context, id uuid.UUID) error { return s.repo.MarkSent(ctx, id) }
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, message string, attempt int) error {
	delay := time.Minute * time.Duration(1<<min(attempt, 6))
	return s.repo.MarkFailed(ctx, id, message, time.Now().Add(delay))
}
