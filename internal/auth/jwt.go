package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"socialfund/internal/httpx"
	"time"
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}
type TokenManager struct {
	secret     []byte
	expiration time.Duration
}

func NewTokenManager(secret string, expiration time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), expiration: expiration}
}
func (m *TokenManager) Issue(id uuid.UUID, role string) (string, int64, error) {
	now := time.Now()
	expires := now.Add(m.expiration)
	claims := Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: id.String(), Issuer: "social-fund", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return token, int64(m.expiration.Seconds()), err
}
func (m *TokenManager) Verify(raw string) (httpx.Identity, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer("social-fund"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return httpx.Identity{}, ErrUnauthorized
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return httpx.Identity{}, ErrUnauthorized
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil || claims.Role == "" {
		return httpx.Identity{}, ErrUnauthorized
	}
	return httpx.Identity{UserID: id, Role: claims.Role}, nil
}
