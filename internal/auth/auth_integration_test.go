package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/auth"
	"socialfund/internal/user"
)

type verifier struct {
	identity auth.VerifiedIdentity
	err      error
}

func (v verifier) Verify(context.Context, string) (auth.VerifiedIdentity, error) {
	return v.identity, v.err
}
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func seed(t *testing.T, pool *pgxpool.Pool, status, email string) (uuid.UUID, *user.PostgresRepository) {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(full_name,email,phone,role,status) VALUES('Member',$1,$2,'MEMBER',$3) RETURNING id`, email, "m-"+suffix[:20], status).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id, user.NewRepository(pool)
}
func service(pool *pgxpool.Pool, repo user.Repository, v auth.Verifier) *auth.Service {
	return auth.NewService(pool, repo, audit.NewRepository(), v, auth.NewTokenManager("test-secret-with-enough-entropy", time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
}
func TestFirstGoogleLoginActivatesAndIssuesJWT(t *testing.T) {
	pool := testPool(t)
	email := "login-" + uuid.NewString() + "@example.test"
	subject := "google-" + uuid.NewString()
	id, repo := seed(t, pool, "INACTIVE", email)
	response, err := service(pool, repo, verifier{identity: auth.VerifiedIdentity{Subject: subject, Email: email, EmailVerified: true}}).LoginGoogle(context.Background(), "valid")
	if err != nil {
		t.Fatal(err)
	}
	if response.AccessToken == "" {
		t.Fatal("JWT is empty")
	}
	var status string
	var googleID *string
	var lastLogin *time.Time
	if err = pool.QueryRow(context.Background(), `SELECT status,google_id,last_login_at FROM users WHERE id=$1`, id).Scan(&status, &googleID, &lastLogin); err != nil {
		t.Fatal(err)
	}
	if status != "ACTIVE" || googleID == nil || *googleID != subject || lastLogin == nil {
		t.Fatalf("activation incomplete")
	}
}
func TestWrongGoogleEmailDoesNotActivate(t *testing.T) {
	pool := testPool(t)
	email := "registered-" + uuid.NewString() + "@example.test"
	id, repo := seed(t, pool, "INACTIVE", email)
	_, err := service(pool, repo, verifier{identity: auth.VerifiedIdentity{Subject: "sub", Email: "other-" + uuid.NewString() + "@example.test", EmailVerified: true}}).LoginGoogle(context.Background(), "valid")
	if !errors.Is(err, auth.ErrAccountNotRegistered) {
		t.Fatalf("err=%v", err)
	}
	var status string
	_ = pool.QueryRow(context.Background(), `SELECT status FROM users WHERE id=$1`, id).Scan(&status)
	if status != "INACTIVE" {
		t.Fatalf("status=%s", status)
	}
}
func TestSuspendedAccountCannotLogin(t *testing.T) {
	pool := testPool(t)
	email := "suspended-" + uuid.NewString() + "@example.test"
	_, repo := seed(t, pool, "SUSPENDED", email)
	_, err := service(pool, repo, verifier{identity: auth.VerifiedIdentity{Subject: "sub", Email: email, EmailVerified: true}}).LoginGoogle(context.Background(), "valid")
	if !errors.Is(err, auth.ErrAccountSuspended) {
		t.Fatalf("err=%v", err)
	}
}
func TestInvalidGoogleToken(t *testing.T) {
	pool := testPool(t)
	repo := user.NewRepository(pool)
	_, err := service(pool, repo, verifier{err: errors.New("bad token")}).LoginGoogle(context.Background(), "invalid")
	if !errors.Is(err, auth.ErrInvalidGoogleToken) {
		t.Fatalf("err=%v", err)
	}
}
