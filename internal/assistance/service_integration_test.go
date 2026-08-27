package assistance_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"os"
	"socialfund/internal/assistance"
	"socialfund/internal/audit"
	"socialfund/internal/fund"
	"socialfund/internal/notification"
	"strings"
	"testing"
)

type fixture struct {
	pool               *pgxpool.Pool
	adminID, requestID uuid.UUID
	email              string
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
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	f := fixture{pool: pool, email: "member-" + suffix + "@example.test"}
	var memberID uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(full_name,email,phone,role,status) VALUES('Admin',$1,$2,'ADMIN','ACTIVE') RETURNING id`, "admin-"+suffix+"@example.test", "a-"+suffix[:20]).Scan(&f.adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(full_name,email,phone,role,status,created_by) VALUES('Member',$1,$2,'MEMBER','ACTIVE',$3) RETURNING id`, f.email, "m-"+suffix[:20], f.adminID).Scan(&memberID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO assistance_requests(user_id,amount_requested,reason,status,amount_approved,reviewed_by,reviewed_at) VALUES($1,75,'Medical','APPROVED',75,$2,NOW()) RETURNING id`, memberID, f.adminID).Scan(&f.requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO fund_transactions(user_id,type,direction,amount,description,recorded_by) VALUES($1,'ADJUSTMENT','IN',1000,'test opening balance',$1)`, f.adminID); err != nil {
		t.Fatal(err)
	}
	return f
}
func newService(f fixture) *assistance.Service {
	return assistance.NewService(f.pool, assistance.NewRepository(f.pool), fund.NewRepository(), audit.NewRepository(), notification.NewRepository(f.pool))
}
func input(f fixture) assistance.DisbursementInput {
	return assistance.DisbursementInput{AssistanceRequestID: f.requestID, AdminID: f.adminID, Amount: decimal.NewFromInt(75), Method: "BANK_TRANSFER", Reference: "pay-" + uuid.NewString()}
}
func TestDisbursementRollback(t *testing.T) {
	f := seed(t, openPool(t))
	name := "test_reject_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE notifications ADD CONSTRAINT %s CHECK (recipient <> '%s')`, name, f.email)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = f.pool.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE notifications DROP CONSTRAINT IF EXISTS %s`, name))
	}()
	if err := newService(f).Pay(context.Background(), input(f)); err == nil {
		t.Fatal("expected failure")
	}
	assertStatus(t, f, "APPROVED")
	assertCount(t, f, 0)
}
func TestDuplicateDisbursement(t *testing.T) {
	f := seed(t, openPool(t))
	svc := newService(f)
	if err := svc.Pay(context.Background(), input(f)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Pay(context.Background(), input(f)); err == nil {
		t.Fatal("expected duplicate failure")
	}
	assertStatus(t, f, "PAID")
	assertCount(t, f, 1)
}
func assertStatus(t *testing.T, f fixture, want string) {
	t.Helper()
	var got string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM assistance_requests WHERE id=$1`, f.requestID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("status=%s, want %s", got, want)
	}
}
func assertCount(t *testing.T, f fixture, want int) {
	t.Helper()
	var got int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM fund_transactions WHERE assistance_request_id=$1`, f.requestID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count=%d, want %d", got, want)
	}
}
