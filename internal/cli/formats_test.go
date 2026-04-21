package cli_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/output"
)

func arrayServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":1,"name":"Alice","status":"active"},{"id":2,"name":"Bob","status":"inactive"},{"id":3,"name":"Carol","status":"active"}]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestSilentMode verifies that --rsh-silent suppresses all output.
func TestSilentMode(t *testing.T) {
	srv := arrayServer(t)

	c, out, errOut := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "--rsh-silent"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected empty stdout with --rsh-silent, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("expected empty stderr with --rsh-silent, got: %q", errOut.String())
	}
}

// TestTableFormat verifies that -o table produces a box-drawing table.
func TestTableFormat(t *testing.T) {
	srv := arrayServer(t)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "-o", "table"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "┌") {
		t.Errorf("expected Unicode table, got:\n%s", got)
	}
	for _, name := range []string{"Alice", "Bob", "Carol"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected %q in table, got:\n%s", name, got)
		}
	}
}

// TestTableFormatColumns verifies that --rsh-columns restricts columns.
func TestTableFormatColumns(t *testing.T) {
	srv := arrayServer(t)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "-o", "table", "--rsh-columns", "name,status"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "name") || !strings.Contains(got, "status") {
		t.Errorf("expected name/status in table, got:\n%s", got)
	}
	// Verify "id" is not a column header.
	lines := strings.Split(got, "\n")
	headerLine := ""
	for _, l := range lines {
		if strings.Contains(l, "│") && strings.Contains(l, "name") {
			headerLine = l
			break
		}
	}
	if strings.Contains(headerLine, " id ") {
		t.Errorf("id column should be excluded by --rsh-columns, header: %q", headerLine)
	}
}

// TestGronFormat verifies that -o gron produces gron-style output.
func TestGronFormat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/obj", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Alice","age":30}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/obj", "-o", "gron"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "json.name") {
		t.Errorf("expected gron path in output, got:\n%s", got)
	}
}

// TestAddFormatter verifies that a custom formatter registered via AddFormatter
// is invoked when selected by name with -o.
func TestAddFormatter(t *testing.T) {
	srv := arrayServer(t)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	// Register a custom formatter that just writes a sentinel string.
	c.AddFormatter("custom", &sentinelFormatter{sentinel: "CUSTOM_OUTPUT"})

	if err := c.Run([]string{"restish", "get", srv.URL + "/items", "-o", "custom"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(out.String(), "CUSTOM_OUTPUT") {
		t.Errorf("expected custom formatter output, got: %q", out.String())
	}
}

// sentinelFormatter is a test formatter that writes a fixed sentinel string.
type sentinelFormatter struct {
	sentinel string
}

func (f *sentinelFormatter) Format(w io.Writer, resp *output.Response, color bool) error {
	_, err := fmt.Fprintln(w, f.sentinel)
	return err
}

var _ output.Formatter = (*sentinelFormatter)(nil)
