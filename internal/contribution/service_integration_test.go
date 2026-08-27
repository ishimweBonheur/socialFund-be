package contribution_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/audit"
	"socialfund/internal/contribution"
	"socialfund/internal/fund"
	"socialfund/internal/notification"
	"socialfund/internal/user"
)

type fixture struct {
	pool                    *pgxpool.Pool
	adminID, contributionID uuid.UUID
	email                   string
}

func openPool(t *testing.T) *pgxpool.Pool {
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
func seed(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var f fixture
	f.pool = pool
	f.email = "member-" + suffix + "@example.test"
	var memberID, planID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,role,status) VALUES('Admin',$1,$2,'ADMIN','ACTIVE') RETURNING id`, "admin-"+suffix+"@example.test", "a-"+suffix[:20]).Scan(&f.adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,role,status,created_by) VALUES('Member',$1,$2,'MEMBER','ACTIVE',$3) RETURNING id`, f.email, "m-"+suffix[:20], f.adminID).Scan(&memberID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO contribution_plans(user_id,amount,frequency,start_date,reminder_frequency,created_by) VALUES($1,100,'DAILY',CURRENT_DATE,'DAILY',$2) RETURNING id`, memberID, f.adminID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO contributions(user_id,contribution_plan_id,expected_amount,due_date,paid_amount,payment_date,payment_method,status) VALUES($1,$2,100,CURRENT_DATE,100,NOW(),'CASH','PENDING') RETURNING id`, memberID, planID).Scan(&f.contributionID); err != nil {
		t.Fatal(err)
	}
	return f
}
func newService(f fixture) *contribution.Service {
	return contribution.NewService(f.pool, contribution.NewRepository(f.pool), fund.NewRepository(), audit.NewRepository(), notification.NewRepository(f.pool), user.NewRepository(f.pool))
}
func TestApprovalSuccess(t *testing.T) {
	f := seed(t, openPool(t))
	if err := newService(f).Approve(context.Background(), contribution.ApprovalInput{ContributionID: f.contributionID, AdminID: f.adminID}); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, f, "APPROVED")
	assertCount(t, f.pool, `SELECT count(*) FROM fund_transactions WHERE contribution_id=$1`, f.contributionID, 1)
	assertCount(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='CONTRIBUTION_APPROVED'`, f.contributionID, 1)
	assertCount(t, f.pool, `SELECT count(*) FROM notifications WHERE contribution_id=$1`, f.contributionID, 1)
}
func TestApprovalRollback(t *testing.T) {
	f := seed(t, openPool(t))
	constraint := rejectRecipient(t, f.pool, f.email)
	defer dropConstraint(t, f.pool, constraint)
	if err := newService(f).Approve(context.Background(), contribution.ApprovalInput{ContributionID: f.contributionID, AdminID: f.adminID}); err == nil {
		t.Fatal("expected failure")
	}
	assertStatus(t, f, "PENDING")
	assertCount(t, f.pool, `SELECT count(*) FROM fund_transactions WHERE contribution_id=$1`, f.contributionID, 0)
	assertCount(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='CONTRIBUTION_APPROVED'`, f.contributionID, 0)
}
func TestConcurrentDoubleApproval(t *testing.T) {
	f := seed(t, openPool(t))
	svc := newService(f)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- svc.Approve(context.Background(), contribution.ApprovalInput{ContributionID: f.contributionID, AdminID: f.adminID})
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes=%d, want 1", success)
	}
	assertCount(t, f.pool, `SELECT count(*) FROM fund_transactions WHERE contribution_id=$1`, f.contributionID, 1)
}
func rejectRecipient(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	name := "test_reject_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	literal := strings.ReplaceAll(email, "'", "''")
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE notifications ADD CONSTRAINT %s CHECK (recipient <> '%s')`, name, literal)); err != nil {
		t.Fatal(err)
	}
	return name
}
func dropConstraint(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE notifications DROP CONSTRAINT IF EXISTS %s`, name)); err != nil {
		t.Error(err)
	}
}
func assertStatus(t *testing.T, f fixture, want string) {
	t.Helper()
	var got string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM contributions WHERE id=$1`, f.contributionID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("status=%s, want %s", got, want)
	}
}
func assertCount(t *testing.T, pool *pgxpool.Pool, query string, id uuid.UUID, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count=%d, want %d", got, want)
	}
}
