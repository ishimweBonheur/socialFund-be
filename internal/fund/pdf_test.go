package fund

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestStatementPDFContainsEvidenceAndSpansPages(t *testing.T) {
	items := make([]FundTransaction, 50)
	for i := range items {
		ref := "PAYMENT-REFERENCE"
		items[i] = FundTransaction{ID: uuid.New(), UserName: "Member Name", Type: "CONTRIBUTION", Direction: "IN", Amount: decimal.NewFromInt(1000), Reference: &ref, CreatedAt: time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)}
	}
	pdf := StatementPDF("Personal transaction statement", "STMT-12345678", "2026-08-01", "2026-08-31", items)
	for _, expected := range [][]byte{[]byte("%PDF-1.4"), []byte("SOCIAL FUND"), []byte("STMT-12345678"), []byte("PAYMENT-REFERENCE"), []byte("0.129 0.204 0.282 rg"), []byte("0.329 0.467 0.573 rg"), []byte("0.580 0.706 0.757 rg"), []byte("0.918 0.878 0.812 rg"), []byte("/Count 3"), []byte("%%EOF")} {
		if !bytes.Contains(pdf, expected) {
			t.Fatalf("PDF does not contain %q", expected)
		}
	}
}
