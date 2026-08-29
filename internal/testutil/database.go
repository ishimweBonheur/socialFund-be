package testutil

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// RequireDisposableDatabase prevents integration tests from modifying an
// application database. Test database names must end in _test.
func RequireDisposableDatabase(t *testing.T, rawURL string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(databaseName), "_test") {
		t.Fatalf("refusing to use non-test database %q: TEST_DATABASE_URL database name must end in _test", databaseName)
	}
	if applicationURL := os.Getenv("DATABASE_URL"); applicationURL != "" && applicationURL == rawURL {
		t.Fatal("refusing to run integration tests because TEST_DATABASE_URL equals DATABASE_URL")
	}
}
