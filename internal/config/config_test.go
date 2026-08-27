package config

import (
	"testing"
	"time"
)

func base(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
}
func TestPoolDefaults(t *testing.T) {
	base(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxConns != 20 || c.MinConns != 2 || c.MaxConnLifetime != 30*time.Minute || c.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}
func TestPoolCustomAndInvalid(t *testing.T) {
	base(t)
	t.Setenv("DB_MAX_CONNS", "8")
	t.Setenv("DB_MIN_CONNS", "3")
	t.Setenv("DB_MAX_CONN_LIFETIME", "1h")
	c, err := Load()
	if err != nil || c.MaxConns != 8 || c.MinConns != 3 || c.MaxConnLifetime != time.Hour {
		t.Fatalf("custom config: %+v %v", c, err)
	}
	t.Setenv("DB_MIN_CONNS", "9")
	if _, err = Load(); err == nil {
		t.Fatal("expected min/max validation error")
	}
}
