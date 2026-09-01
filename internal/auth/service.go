package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/database"
	"socialfund/internal/httpx"
	"socialfund/internal/user"
)

var (
	ErrInvalidGoogleToken   = httpx.NewError(401, "INVALID_GOOGLE_TOKEN", "The Google credential is invalid")
	ErrEmailNotVerified     = httpx.NewError(401, "GOOGLE_EMAIL_NOT_VERIFIED", "The Google email is not verified")
	ErrAccountNotRegistered = httpx.NewError(403, "ACCOUNT_NOT_REGISTERED", "No Social Fund account exists for this Google account")
	ErrAccountSuspended     = httpx.NewError(403, "ACCOUNT_SUSPENDED", "Your account has been suspended. Contact support for help getting back online.")
	ErrAccountInactive      = httpx.NewError(403, "ACCOUNT_INACTIVE", "Your Social Fund account is inactive")
	ErrIdentityMismatch     = httpx.NewError(403, "GOOGLE_IDENTITY_MISMATCH", "This account could not be verified")
	ErrUnauthorized         = httpx.ErrUnauthorized
)

type Verifier interface {
	Verify(context.Context, string) (VerifiedIdentity, error)
}
type Service struct {
	pool     *pgxpool.Pool
	users    user.Repository
	audit    audit.Writer
	verifier Verifier
	tokens   *TokenManager
	logger   *slog.Logger
}

func NewService(pool *pgxpool.Pool, users user.Repository, auditWriter audit.Writer, verifier Verifier, tokens *TokenManager, logger *slog.Logger) *Service {
	return &Service{pool: pool, users: users, audit: auditWriter, verifier: verifier, tokens: tokens, logger: logger}
}
func (s *Service) LoginGoogle(ctx context.Context, credential string) (LoginResponse, error) {
	if strings.TrimSpace(credential) == "" {
		return LoginResponse{}, httpx.ErrValidation
	}
	identity, err := s.verifier.Verify(ctx, credential)
	if err != nil {
		return LoginResponse{}, ErrInvalidGoogleToken
	}
	if !identity.EmailVerified {
		return LoginResponse{}, ErrEmailNotVerified
	}
	var account user.User
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		account, err = s.users.LockByEmail(ctx, tx, identity.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAccountNotRegistered
		}
		if err != nil {
			return err
		}
		if account.Status == "SUSPENDED" {
			return ErrAccountSuspended
		}
		if account.GoogleID != nil && *account.GoogleID != identity.Subject {
			s.logger.WarnContext(ctx, "Google identity mismatch", "user_id", account.ID, "request_id", httpx.RequestID(ctx))
			return ErrIdentityMismatch
		}
		activated := false
		switch account.Status {
		case "INACTIVE":
			if err = s.users.Activate(ctx, tx, account.ID, identity.Subject); err != nil {
				return err
			}
			account.Status = "ACTIVE"
			account.GoogleID = &identity.Subject
			activated = true
		case "ACTIVE":
			if err = s.users.RecordLogin(ctx, tx, account.ID); err != nil {
				return err
			}
		default:
			return ErrAccountInactive
		}
		actor := account.ID
		if activated {
			if _, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &actor, Action: "USER_ACTIVATED", EntityType: "USER", EntityID: account.ID, NewData: jsonData(map[string]string{"status": "ACTIVE"})}); err != nil {
				return err
			}
		}
		_, err = s.audit.Create(ctx, tx, audit.AuditLog{UserID: &actor, Action: "USER_LOGIN", EntityType: "USER", EntityID: account.ID})
		return err
	})
	if err != nil {
		return LoginResponse{}, err
	}
	token, expires, err := s.tokens.Issue(account.ID, account.Role)
	if err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: expires, User: UserResponse{ID: account.ID.String(), FullName: account.FullName, Email: account.Email, Role: account.Role, Status: account.Status}}, nil
}
func jsonData(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
