package cli_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestDoctorReportsInsecureTokenCachePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not authoritative on Windows")
	}
	c, _, errOut := newTestCLI(t)
	tokenPath := c.Hooks().TokenCachePath
	if err := os.WriteFile(tokenPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write token cache: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	got := errOut.String()
	if !strings.Contains(got, "Token cache permissions: insecure") ||
		!strings.Contains(got, "before the next OAuth request") {
		t.Fatalf("expected token cache remediation, got:\n%s", got)
	}
}
