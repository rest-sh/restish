package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


// threePageServer creates a test server with three pages of items linked via
// RFC 5988 Link headers (rel="next").
func threePageServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	pages := []struct {
		body string
		next string
	}{
		{`[1,2,3]`, srv.URL + "/items?page=2"},
		{`[4,5,6]`, srv.URL + "/items?page=3"},
		{`[7,8,9]`, ""},
	}

	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		var idx int
		switch page {
		case "2":
			idx = 1
		case "3":
			idx = 2
		default:
			idx = 0
		}
		p := pages[idx]
		if p.next != "" {
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, p.next))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, p.body)
	})
	return srv
}

// TestPaginationThreePages verifies that auto-pagination collects items from
// all three pages when streaming.
func TestPaginationThreePages(t *testing.T) {
	srv := threePageServer(t)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	// Stream mode prints each item as JSON on its own line.
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		if !strings.Contains(got, n) {
			t.Errorf("expected item %s in output, got:\n%s", n, got)
		}
	}
}

// TestPaginationNoPaginate verifies that --rsh-no-paginate returns only the
// first page.
func TestPaginationNoPaginate(t *testing.T) {
	srv := threePageServer(t)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "--rsh-no-paginate"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	// Should contain first page items.
	for _, n := range []string{"1", "2", "3"} {
		if !strings.Contains(got, n) {
			t.Errorf("expected item %s, got:\n%s", n, got)
		}
	}
	// Should NOT contain second page items.
	for _, n := range []string{"4", "5", "6", "7"} {
		if strings.Contains(got, n) {
			t.Errorf("unexpected item %s in output, got:\n%s", n, got)
		}
	}
}

// TestPaginationMaxPages verifies that --rsh-max-pages 1 stops after one page
// and emits a warning to stderr.
func TestPaginationMaxPages(t *testing.T) {
	srv := threePageServer(t)

	c, out, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "--rsh-max-pages", "1"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	// Only first page items.
	got := out.String()
	for _, n := range []string{"1", "2", "3"} {
		if !strings.Contains(got, n) {
			t.Errorf("expected item %s, got:\n%s", n, got)
		}
	}
	for _, n := range []string{"4", "5"} {
		if strings.Contains(got, n) {
			t.Errorf("unexpected item %s, got:\n%s", n, got)
		}
	}

	// Warning must appear on stderr.
	if !strings.Contains(errOut.String(), "max-pages") {
		t.Errorf("expected max-pages warning on stderr, got: %q", errOut.String())
	}
}

// TestPaginationCollect verifies that --rsh-collect + -f length returns the
// total item count across all pages.
func TestPaginationCollect(t *testing.T) {
	srv := threePageServer(t)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "--rsh-collect", "-f", ".body | length"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "9" {
		t.Errorf("expected length 9, got: %q", got)
	}
}

// TestPaginationItemsPath verifies that per-API items_path extracts items from
// a nested field.
func TestPaginationItemsPath(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[1,2,3],"meta":{"total":3}}`)
	})

	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   srv.URL,
				"pagination": map[string]any{"items_path": "data"},
			},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := writeFile(cfgFile, cfgData); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, out, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "get", srv.URL + "/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	// Should contain items 1, 2, 3 (from "data" field).
	for _, n := range []string{"1", "2", "3"} {
		if !strings.Contains(got, n) {
			t.Errorf("expected item %s in output, got:\n%s", n, got)
		}
	}
}

// TestPaginationProgressOnStderr verifies that progress output goes to stderr
// not stdout when paginating.
func TestPaginationProgressOnStderr(t *testing.T) {
	srv := threePageServer(t)

	// Use the full CLI so we can inspect stdout vs stderr.
	c, out, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "--rsh-max-pages", "1"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	// Warnings (stderr) must not appear in stdout.
	if strings.Contains(out.String(), "warning") || strings.Contains(out.String(), "max-pages") {
		t.Errorf("progress/warning leaked to stdout:\n%s", out.String())
	}
	// Warning should be on stderr.
	if !strings.Contains(errOut.String(), "max-pages") {
		t.Errorf("expected warning on stderr, got: %q", errOut.String())
	}
}
