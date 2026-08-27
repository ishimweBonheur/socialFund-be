package notification

import (
	"context"
	"github.com/google/uuid"
	"time"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) Claim(ctx context.Context, limit int) ([]Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.repo.ListReady(ctx, limit)
}
func (s *Service) MarkSent(ctx context.Context, id uuid.UUID) error { return s.repo.MarkSent(ctx, id) }
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, message string, attempt int) error {
	delay := time.Minute * time.Duration(1<<min(attempt, 6))
	return s.repo.MarkFailed(ctx, id, message, time.Now().Add(delay))
}
