package auth

import (
	"context"
	"fmt"
	"google.golang.org/api/idtoken"
)

type GoogleVerifier struct{ clientID string }

func NewGoogleVerifier(clientID string) *GoogleVerifier { return &GoogleVerifier{clientID: clientID} }
func (v *GoogleVerifier) Verify(ctx context.Context, credential string) (VerifiedIdentity, error) {
	payload, err := idtoken.Validate(ctx, credential, v.clientID)
	if err != nil {
		return VerifiedIdentity{}, ErrInvalidGoogleToken
	}
	subject, ok := payload.Claims["sub"].(string)
	if !ok || subject == "" {
		return VerifiedIdentity{}, ErrInvalidGoogleToken
	}
	email, ok := payload.Claims["email"].(string)
	if !ok || email == "" {
		return VerifiedIdentity{}, ErrInvalidGoogleToken
	}
	verified, ok := payload.Claims["email_verified"].(bool)
	if !ok {
		return VerifiedIdentity{}, fmt.Errorf("email_verified claim missing")
	}
	return VerifiedIdentity{Subject: subject, Email: email, EmailVerified: verified}, nil
}
