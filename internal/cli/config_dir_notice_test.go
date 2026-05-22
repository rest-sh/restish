package cli_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestRSHConfigDirMissingConfigDoesNotEmitNotice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", dir)
	c, _, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = ""
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	if err := c.Run([]string{"restish", "get", "--rsh-no-cache", "https://api.example.com/items"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stderr.String(); strings.Contains(got, "no config") || strings.Contains(got, "using defaults") {
		t.Fatalf("missing config should not emit startup notice, got:\n%s", got)
	}
}
