package user

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
)

var ErrInactive = errors.New("user is not active")

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.GetByID(ctx, id)
}
func (s *Service) FindForVerifiedGoogleEmail(ctx context.Context, email string) (User, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}
	if u.Status != "ACTIVE" {
		return User{}, ErrInactive
	}
	return u, nil
}
func (s *Service) Create(ctx context.Context, u User) (User, error) {
	if u.FullName == "" || u.Email == "" || u.Phone == "" {
		return User{}, fmt.Errorf("name, email, and phone are required")
	}
	if u.Role == "" {
		u.Role = "MEMBER"
	}
	if u.Status == "" {
		u.Status = "ACTIVE"
	}
	return s.repo.Create(ctx, u)
}
