package contribution_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
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
	"socialfund/internal/testutil"
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
	if err := pool.QueryRow(ctx, `INSERT INTO contributions(user_id,contribution_plan_id,expected_amount,due_date,paid_amount,payment_date,payment_method,transaction_reference,proof_url,proof_uploaded_at,status) VALUES($1,$2,100,CURRENT_DATE,100,NOW(),'CASH',$3,'/uploads/proofs/test.pdf',NOW(),'PENDING') RETURNING id`, memberID, planID, "proof-"+suffix).Scan(&f.contributionID); err != nil {
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
func TestSchedulerPenaltyReminderAndStateEligibility(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var adminID, memberID, planID, contributionID uuid.UUID
	email := "scheduler-" + suffix + "@example.test"
	if err := pool.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,role,status) VALUES('Admin',$1,$2,'ADMIN','ACTIVE') RETURNING id`, "scheduler-admin-"+suffix+"@example.test", "sa-"+suffix[:20]).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(full_name,email,phone,role,status,created_by) VALUES('Member',$1,$2,'MEMBER','ACTIVE',$3) RETURNING id`, email, "sm-"+suffix[:20], adminID).Scan(&memberID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO contribution_plans(user_id,amount,frequency,start_date,reminder_enabled,reminder_frequency,late_fee_enabled,late_fee_percentage,grace_period_days,created_by) VALUES($1,5000,'DAILY',CURRENT_DATE-1,TRUE,'DAILY',TRUE,2,0,$2) RETURNING id`, memberID, adminID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO contributions(user_id,contribution_plan_id,expected_amount,due_date,status) VALUES($1,$2,5000,CURRENT_DATE-1,'DUE') RETURNING id`, memberID, planID).Scan(&contributionID); err != nil {
		t.Fatal(err)
	}
	svc := contribution.NewService(pool, contribution.NewRepository(pool), fund.NewRepository(), audit.NewRepository(), notification.NewRepository(pool), user.NewRepository(pool))
	if _, err := svc.ProcessOverdue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	var status string
	var fee decimal.Decimal
	var overdueAt any
	if err := pool.QueryRow(ctx, `SELECT status,late_fee_amount,overdue_at FROM contributions WHERE id=$1`, contributionID).Scan(&status, &fee, &overdueAt); err != nil {
		t.Fatal(err)
	}
	if status != "OVERDUE" || !fee.Equal(decimal.NewFromInt(100)) || overdueAt == nil {
		t.Fatalf("status=%s fee=%s overdue_at=%v", status, fee, overdueAt)
	}
	if _, err := svc.ProcessOverdue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	assertCount(t, pool, `SELECT count(*) FROM notifications WHERE contribution_id=$1 AND type='CONTRIBUTION_OVERDUE'`, contributionID, 1)
	if _, err := pool.Exec(ctx, `UPDATE contributions SET status='PENDING' WHERE id=$1`, contributionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notifications SET created_at=NOW()-INTERVAL '2 days' WHERE contribution_id=$1`, contributionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProcessOverdue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	assertCount(t, pool, `SELECT count(*) FROM notifications WHERE contribution_id=$1 AND type='CONTRIBUTION_OVERDUE'`, contributionID, 1)
	if _, err := pool.Exec(ctx, `UPDATE contributions SET status='REJECTED',rejection_reason='unclear' WHERE id=$1`, contributionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProcessOverdue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	assertCount(t, pool, `SELECT count(*) FROM notifications WHERE contribution_id=$1 AND type='CONTRIBUTION_OVERDUE'`, contributionID, 2)
}
func TestProofSubmissionOwnershipAndResubmission(t *testing.T) {
	f := seed(t, openPool(t))
	ctx := context.Background()
	svc := newService(f)
	var memberID uuid.UUID
	if err := f.pool.QueryRow(ctx, `SELECT user_id FROM contributions WHERE id=$1`, f.contributionID).Scan(&memberID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE contributions SET status='DUE',paid_amount=NULL,payment_date=NULL,payment_method=NULL,transaction_reference=NULL,proof_url=NULL,proof_uploaded_at=NULL WHERE id=$1`, f.contributionID); err != nil {
		t.Fatal(err)
	}
	base := contribution.ProofInput{ContributionID: f.contributionID, UserID: uuid.New(), Amount: decimal.NewFromInt(100), PaymentMethod: "CASH", TransactionReference: "wrong-" + uuid.NewString(), ProofURL: "/uploads/proofs/test.pdf"}
	if err := svc.SubmitProof(ctx, base); !errors.Is(err, contribution.ErrForbidden) {
		t.Fatalf("wrong owner error=%v", err)
	}
	base.UserID = memberID
	base.TransactionReference = "first-" + uuid.NewString()
	if err := svc.SubmitProof(ctx, base); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, f, "PENDING")
	assertCount(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='PROOF_UPLOADED'`, f.contributionID, 1)
	if _, err := f.pool.Exec(ctx, `UPDATE contributions SET status='REJECTED',rejection_reason='unclear' WHERE id=$1`, f.contributionID); err != nil {
		t.Fatal(err)
	}
	base.TransactionReference = "second-" + uuid.NewString()
	if err := svc.SubmitProof(ctx, base); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, f, "PENDING")
	assertCount(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='CONTRIBUTION_PROOF_RESUBMITTED'`, f.contributionID, 1)
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
