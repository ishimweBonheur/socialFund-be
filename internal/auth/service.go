package auth

import (
	"context"
	"errors"
	"socialfund/internal/user"
)

var ErrUnverifiedIdentity = errors.New("Google identity is not verified")

type VerifiedIdentity struct {
	Email         string
	EmailVerified bool
}
type Verifier interface {
	Verify(context.Context, string) (VerifiedIdentity, error)
}
type Service struct {
	verifier Verifier
	users    *user.Service
}

func NewService(verifier Verifier, users *user.Service) *Service {
	return &Service{verifier: verifier, users: users}
}
func (s *Service) AuthenticateGoogle(ctx context.Context, token string) (user.User, error) {
	identity, err := s.verifier.Verify(ctx, token)
	if err != nil {
		return user.User{}, err
	}
	if !identity.EmailVerified || identity.Email == "" {
		return user.User{}, ErrUnverifiedIdentity
	}
	return s.users.FindForVerifiedGoogleEmail(ctx, identity.Email)
}
