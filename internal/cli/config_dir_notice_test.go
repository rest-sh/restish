package cli_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestRSHConfigDirMissingConfigNotice(t *testing.T) {
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
	want := filepath.Join(dir, "restish.json")
	if got := stderr.String(); !strings.Contains(got, "no config at "+want) {
		t.Fatalf("expected missing config notice for %s, got:\n%s", want, got)
	}
}
