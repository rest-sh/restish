package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func useThreePageTransport(c *cli.CLI) {
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		pages := map[string]struct {
			body string
			next string
		}{
			"":  {`[1,2,3]`, "https://api.example.com/items?page=2"},
			"2": {`[4,5,6]`, "https://api.example.com/items?page=3"},
			"3": {`[7,8,9]`, ""},
		}
		p := pages[r.URL.Query().Get("page")]
		headers := http.Header{"Content-Type": []string{"application/json"}}
		if p.next != "" {
			headers.Set("Link", `<`+p.next+`>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(p.body)),
			Request:    r,
		}, nil
	})
}

func useThreePageObjectTransport(c *cli.CLI) {
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		pages := map[string]struct {
			body string
			next string
		}{
			"":  {`[{"id":1},{"id":2}]`, "https://api.example.com/items?page=2"},
			"2": {`[{"id":3},{"id":4}]`, ""},
		}
		p := pages[r.URL.Query().Get("page")]
		headers := http.Header{"Content-Type": []string{"application/json"}}
		if p.next != "" {
			headers.Set("Link", `<`+p.next+`>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(p.body)),
			Request:    r,
		}, nil
	})
}

// TestPaginationThreePages verifies that automatic pagination merges all pages
// into one valid JSON document by default on non-TTY stdout.
func TestPaginationThreePages(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	var values []int
	if err := json.Unmarshal([]byte(got), &values); err != nil {
		t.Fatalf("expected valid JSON array, got %q: %v", got, err)
	}
	for i, want := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9} {
		if values[i] != want {
			t.Fatalf("values[%d] = %d, want %d", i, values[i], want)
		}
	}
}

// TestPaginationNoPaginate verifies that --rsh-no-paginate returns only the
// first page.
func TestPaginationNoPaginate(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-no-paginate"}); err != nil {
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
	c, out, errOut := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-max-pages", "1"}); err != nil {
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
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-collect", "-f", ".body | length"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "9" {
		t.Errorf("expected length 9, got: %q", got)
	}
}

func TestPaginationStreamingYAMLOutputUsesFormatter(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "yaml"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	for _, want := range []string{"id: 1", "id: 2", "id: 3", "id: 4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"id":1}`) || strings.Contains(got, `"id": 1`) {
		t.Fatalf("expected paginated stream output to use YAML formatting, got:\n%s", got)
	}
}

func TestPaginationNDJSONOutputStreamsRecords(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "ndjson"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 NDJSON lines, got %d:\n%s", len(lines), got)
	}
	for i, line := range lines {
		var item map[string]int
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("line %d is not valid JSON: %q: %v", i+1, line, err)
		}
		if item["id"] != i+1 {
			t.Fatalf("line %d id = %d, want %d", i+1, item["id"], i+1)
		}
	}
}

func TestPaginationReadableOutputNonTTYUsesDocumentRendering(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "readable"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	if strings.Count(got, "HTTP/1.1 200 OK") != 1 {
		t.Fatalf("expected readable preamble once, got:\n%s", got)
	}
	for _, want := range []string{`"id": 1`, `"id": 2`, `"id": 3`, `"id": 4`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in readable output, got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "[\n") {
		t.Fatalf("expected non-TTY readable output to render the collected array, got:\n%s", got)
	}
}

func TestPaginationStreamingAppliesFilterPerItem(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-f", ".body | map(.id)"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	var values []int
	if err := json.Unmarshal([]byte(got), &values); err != nil {
		t.Fatalf("expected valid filtered JSON array, got %q: %v", got, err)
	}
	for i, want := range []int{1, 2, 3, 4} {
		if values[i] != want {
			t.Fatalf("values[%d] = %d, want %d", i, values[i], want)
		}
	}
}

func TestPaginationStreamingFilterUsesFormatter(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-f", "body", "-o", "yaml"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	for _, want := range []string{"id: 1", "id: 2", "id: 3", "id: 4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"id":1}`) || strings.Contains(got, `"id": 1`) {
		t.Fatalf("expected filtered paginated stream output to use YAML formatting, got:\n%s", got)
	}
}

func TestPaginationJSONOutputIsValidJSON(t *testing.T) {
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageObjectTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var values []map[string]int
	if err := json.Unmarshal([]byte(out.String()), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 4 || values[0]["id"] != 1 || values[3]["id"] != 4 {
		t.Fatalf("unexpected output: %#v", values)
	}
}

func TestPaginationCycleDetection(t *testing.T) {
	c, out, errOut := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"Link":         []string{`<https://api.example.com/items>; rel="next"`},
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[1,2,3]`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(errOut.String(), "cycle detected") {
		t.Fatalf("expected cycle warning, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("expected first page output, got %q", out.String())
	}
}

func TestPaginationItemsPathScalarWarns(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url": "https://api.example.com",
				"pagination": map[string]any{
					"items_path": "data",
				},
			},
		},
	})
	c, out, errOut := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := os.WriteFile(c.Hooks().ConfigPath, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		headers := http.Header{"Content-Type": []string{"application/json"}}
		if r.URL.Query().Get("page") == "" {
			headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`{"data":1}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(errOut.String(), "items_path") {
		t.Fatalf("expected items_path warning, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), `"data"`) {
		t.Fatalf("expected wrapped document output, got %q", out.String())
	}
}

// TestPaginationItemsPath verifies that per-API items_path extracts items from
// a nested field.
func TestPaginationItemsPath(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"items_path": "data"},
			},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := writeFile(cfgFile, cfgData); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = cfgFile
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[1,2,3],"meta":{"total":3}}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
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
	// Use the full CLI so we can inspect stdout vs stderr.
	c, out, errOut := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-max-pages", "1"}); err != nil {
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
