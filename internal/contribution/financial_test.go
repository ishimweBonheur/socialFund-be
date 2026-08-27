package contribution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
)

func TestTotalDueIncludesFixedLateFee(t *testing.T) {
	c := Contribution{ExpectedAmount: decimal.RequireFromString("5000.00"), LateFeeAmount: decimal.RequireFromString("100.00")}
	if got := c.TotalDue().StringFixed(2); got != "5100.00" {
		t.Fatalf("TotalDue() = %s, want 5100.00", got)
	}
}

func TestReviewTokenStoresHashOnly(t *testing.T) {
	raw, hash, err := newReviewToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == hash || raw == "" {
		t.Fatal("raw token must be non-empty and differ from stored hash")
	}
	sum := sha256.Sum256([]byte(raw))
	if got := fmt.Sprintf("%x", sum[:]); got != hash {
		t.Fatalf("hash = %s, want %s", hash, got)
	}
}

func TestLocalFileStorageSaveAndDelete(t *testing.T) {
	root := t.TempDir()
	storage := NewLocalFileStorage(root, "/uploads/proofs")
	url, err := storage.Save(context.Background(), ".pdf", bytes.NewBufferString("proof"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.Base(url))
	if data, err := os.ReadFile(path); err != nil || string(data) != "proof" {
		t.Fatalf("stored proof = %q, %v", data, err)
	}
	if err = storage.Delete(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("proof still exists: %v", err)
	}
}
