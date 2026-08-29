package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"socialfund/internal/audit"
	"socialfund/internal/contributionplan"
	"socialfund/internal/httpx"
	"socialfund/internal/notification"
	"socialfund/internal/testutil"
	"socialfund/internal/user"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	testutil.RequireDisposableDatabase(t, url)
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
func seedAdmin(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(full_name,email,phone,role,status) VALUES('Admin',$1,$2,'ADMIN','ACTIVE') RETURNING id`, "admin-"+suffix+"@example.test", "a-"+suffix[:20]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func serviceFor(pool *pgxpool.Pool) *user.Service {
	return user.NewService(pool, user.NewRepository(pool), contributionplan.NewRepository(pool), notification.NewRepository(pool), audit.NewRepository(), "http://localhost:3000")
}
func request(email, phone string, amount int64) user.CreateMemberRequest {
	day := 5
	return user.CreateMemberRequest{FullName: "Patience Ineza", Email: email, Phone: phone, Contribution: user.ContributionRequest{Amount: decimal.NewFromInt(amount), Frequency: "MONTHLY", DueDay: &day, StartDate: "2026-09-01"}, Reminder: user.ReminderRequest{Enabled: true, Frequency: "DAILY"}}
}
func TestCreateMemberTransaction(t *testing.T) {
	pool := testPool(t)
	admin := seedAdmin(t, pool)
	suffix := uuid.NewString()
	created, err := serviceFor(pool).CreateMember(context.Background(), admin, request("member-"+suffix+"@example.test", "m-"+strings.ReplaceAll(suffix, "-", "")[:20], 5000))
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "INACTIVE" || created.Role != "MEMBER" {
		t.Fatalf("unexpected member: %+v", created)
	}
	assertCount(t, pool, `SELECT count(*) FROM contribution_plans WHERE user_id=$1`, created.ID, 1)
	assertCount(t, pool, `SELECT count(*) FROM contributions WHERE user_id=$1`, created.ID, 1)
	assertCount(t, pool, `SELECT count(*) FROM notifications WHERE user_id=$1 AND type='ACCOUNT_CREATED' AND status='PENDING'`, created.ID, 1)
	assertCount(t, pool, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='USER_CREATED'`, created.ID, 1)
}
func TestCreateMemberRollsBackWhenPlanFails(t *testing.T) {
	pool := testPool(t)
	admin := seedAdmin(t, pool)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	email := "rollback-" + suffix + "@example.test"
	name := "test_plan_" + suffix
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE contribution_plans ADD CONSTRAINT %s CHECK (amount <> 7777)`, name)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE contribution_plans DROP CONSTRAINT IF EXISTS %s`, name))
	}()
	if _, err := serviceFor(pool).CreateMember(context.Background(), admin, request(email, "r-"+suffix[:20], 7777)); err == nil {
		t.Fatal("expected plan failure")
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email=$1`, email).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("users=%d, want 0", count)
	}
}
func TestDuplicateEmailReturnsPrivateConflict(t *testing.T) {
	pool := testPool(t)
	admin := seedAdmin(t, pool)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	email := "duplicate-" + suffix + "@example.test"
	svc := serviceFor(pool)
	if _, err := svc.CreateMember(context.Background(), admin, request(email, "d1-"+suffix[:18], 5000)); err != nil {
		t.Fatal(err)
	}
	handler := user.NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	payload, _ := json.Marshal(request(email, "d2-"+suffix[:18], 5000))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req = req.WithContext(httpx.WithIdentity(req.Context(), httpx.Identity{UserID: admin, Role: "ADMIN"}))
	rec := httptest.NewRecorder()
	handler.AdminRoutes().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "EMAIL_ALREADY_EXISTS") || strings.Contains(strings.ToLower(rec.Body.String()), "duplicate key") {
		t.Fatalf("unsafe response: %s", rec.Body.String())
	}
}
func assertCount(t *testing.T, pool *pgxpool.Pool, query string, id uuid.UUID, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count=%d want=%d", got, want)
	}
}
