package contributionplan

import (
	"context"
	"github.com/google/uuid"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) GetActive(ctx context.Context, userID uuid.UUID) (ContributionPlan, error) {
	return s.repo.GetActiveByUserID(ctx, userID)
}
func (s *Service) Create(ctx context.Context, p ContributionPlan) (ContributionPlan, error) {
	return s.repo.Create(ctx, p)
}
