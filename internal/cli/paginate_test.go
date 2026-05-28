package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/cli"
)

type notifyWriter struct {
	writer io.Writer
	writes chan string
}

func (w notifyWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		select {
		case w.writes <- string(p[:n]):
		default:
		}
	}
	return n, err
}

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

func runThreePages(t *testing.T, args ...string) *testApp {
	t.Helper()
	app := newTestApp(t)
	useThreePageTransport(app.CLI)
	app.Run(append([]string{"get", "https://api.example.com/items"}, args...)...)
	return app
}

func runObjectPages(t *testing.T, args ...string) *testApp {
	t.Helper()
	app := newTestApp(t)
	useThreePageObjectTransport(app.CLI)
	app.Run(append([]string{"get", "https://api.example.com/items"}, args...)...)
	return app
}

// TestPaginationThreePagesWithExplicitJSON verifies that automatic pagination
// merges all pages into one valid JSON document when JSON output is requested.
func TestPaginationThreePagesWithExplicitJSON(t *testing.T) {
	got := runThreePages(t, "-o", "json").Stdout.String()
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

func TestPaginationDefaultRedirectPreservesFirstResponseBytes(t *testing.T) {
	app := runThreePages(t)
	if got, want := app.Stdout.String(), `[1,2,3]`; got != want {
		t.Fatalf("default redirected output = %q, want first response bytes %q", got, want)
	}
	if strings.Contains(app.Stderr.String(), "pagination") {
		t.Fatalf("default redirected raw output should not paginate, got stderr %q", app.Stderr.String())
	}
}

func TestPaginationStopsOnCrossOriginNextURL(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	crossOriginRequests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "evil.example.com" {
			crossOriginRequests++
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`[999]`)),
				Request:    r,
			}, nil
		}
		headers := http.Header{"Content-Type": []string{"application/json"}}
		headers.Set("Link", `<https://evil.example.com/items?page=2>; rel="next"`)
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[1,2,3]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if crossOriginRequests != 0 {
		t.Fatalf("cross-origin page requested %d times, want 0", crossOriginRequests)
	}
	if strings.Contains(out.String(), "999") {
		t.Fatalf("cross-origin page leaked into output:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "pagination next URL crosses origin") {
		t.Fatalf("expected cross-origin pagination warning, got:\n%s", errOut.String())
	}
}

// TestPaginationBlocksHTTPToHTTPSSchemeChange verifies that pagination compares
// full URL origins, including scheme, before following a next link.
func TestPaginationBlocksHTTPToHTTPSSchemeChange(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	upgradeRequests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "https" {
			upgradeRequests++
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`[999]`)),
				Request:    r,
			}, nil
		}
		headers := http.Header{"Content-Type": []string{"application/json"}}
		headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[1]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "http://api.example.com/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if upgradeRequests != 0 {
		t.Fatalf("scheme-changed page requested %d times, want 0", upgradeRequests)
	}
	if strings.Contains(out.String(), "999") {
		t.Fatalf("scheme-changed page leaked into output:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "pagination next URL crosses origin") {
		t.Fatalf("expected scheme-change pagination warning, got:\n%s", errOut.String())
	}
}

// TestPaginationBlocksHTTPSToHTTPDowngrade verifies that pagination from HTTPS
// to HTTP on the same host is blocked (scheme downgrade is unsafe).
func TestPaginationBlocksHTTPSToHTTPDowngrade(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	downgradeRequests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "http" {
			downgradeRequests++
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`[999]`)),
				Request:    r,
			}, nil
		}
		headers := http.Header{"Content-Type": []string{"application/json"}}
		headers.Set("Link", `<http://api.example.com/items?page=2>; rel="next"`)
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[1,2,3]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if downgradeRequests != 0 {
		t.Fatalf("downgraded page requested %d times, want 0", downgradeRequests)
	}
	if strings.Contains(out.String(), "999") {
		t.Fatalf("downgraded page leaked into output:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "pagination next URL crosses origin") {
		t.Fatalf("expected HTTPS downgrade warning, got:\n%s", errOut.String())
	}
}

// TestPaginationNoPaginate verifies that --rsh-no-paginate returns only the
// first page.
func TestPaginationNoPaginate(t *testing.T) {
	got := runThreePages(t, "--rsh-no-paginate").Stdout.String()
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

func TestPaginationLinksFilterDoesNotPaginate(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	requests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests++
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-f", "links", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !strings.Contains(out.String(), "next") {
		t.Fatalf("links output missing next: %q", out.String())
	}
}

func TestPaginationClearsOriginalQueryForAbsoluteNextURL(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	var rawQueries []string
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rawQueries = append(rawQueries, r.URL.RawQuery)
		headers := http.Header{"Content-Type": []string{"application/json"}}
		body := `[{"id":2}]`
		if len(rawQueries) == 1 {
			body = `[{"id":1}]`
			headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-q", "page=1", "-o", "ndjson"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(rawQueries) != 2 {
		t.Fatalf("queries = %#v, want two requests", rawQueries)
	}
	if rawQueries[1] != "page=2" {
		t.Fatalf("second query = %q, want page=2 without duplicated original query", rawQueries[1])
	}
}

// TestPaginationMaxPages verifies that --rsh-max-pages 1 stops after one page
// and emits a warning to stderr.
func TestPaginationMaxPages(t *testing.T) {
	app := runThreePages(t, "--rsh-max-pages", "1", "-o", "json")

	// Only first page items.
	got := app.Stdout.String()
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
	wantWarning := "pagination stopped at --rsh-max-pages=1; pass 0 for unlimited"
	if !strings.Contains(app.Stderr.String(), wantWarning) {
		t.Errorf("expected max-pages warning %q on stderr, got: %q", wantWarning, app.Stderr.String())
	}
}

// TestPaginationCollect verifies that --rsh-collect + -f length returns the
// total item count across all pages.
func TestPaginationCollect(t *testing.T) {
	got := strings.TrimSpace(runThreePages(t, "--rsh-collect", "-f", ".body | length").Stdout.String())
	if got != "9" {
		t.Errorf("expected length 9, got: %q", got)
	}
}

func TestPaginationStreamingYAMLOutputUsesFormatter(t *testing.T) {
	got := runObjectPages(t, "-o", "yaml").Stdout.String()
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
	got := strings.TrimSpace(runObjectPages(t, "-o", "ndjson").Stdout.String())
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

func TestPaginationStreamingPrintHeadersOnlyDoesNotRenderOrFetchItems(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	requests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests++
		headers := http.Header{"Content-Type": []string{"application/json"}}
		headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[{"id":1},{"id":2}]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-print", "h"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 200 OK") {
		t.Fatalf("response headers not found:\n%s", got)
	}
	if strings.Contains(got, `"id"`) {
		t.Fatalf("headers-only pagination output included body data:\n%s", got)
	}
	if requests != 1 {
		t.Fatalf("pagination fetched %d pages for headers-only output, want 1", requests)
	}
}

func TestPaginationDocumentPrintHeadersOnlyDoesNotFetchNextPage(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	requests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests++
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"X-Page":       []string{r.URL.Query().Get("page")},
		}
		if r.URL.Query().Get("page") == "" {
			headers.Set("X-Page", "1")
			headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[{"id":1},{"id":2}]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-max-pages", "2", "--rsh-print", "h"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 200 OK") || !strings.Contains(got, "X-Page: 1") {
		t.Fatalf("response headers not found:\n%s", got)
	}
	if strings.Contains(got, `"id"`) {
		t.Fatalf("headers-only pagination output included body data:\n%s", got)
	}
	if requests != 1 {
		t.Fatalf("pagination fetched %d pages for headers-only output, want 1", requests)
	}
	if strings.Contains(errOut.String(), "pagination stopped") {
		t.Fatalf("headers-only output should not emit pagination warnings, got: %q", errOut.String())
	}
}

func TestPaginationStreamingPrintBodyOnlyOmitsPrettyFraming(t *testing.T) {
	app := newTestApp(t)
	app.SetStdoutTTY(true)
	useThreePageObjectTransport(app.CLI)
	app.Run("get", "https://api.example.com/items", "--rsh-print", "b")

	got := stripANSI(app.Stdout.String())
	if strings.Contains(got, "HTTP/1.1 200 OK") || strings.Contains(got, "Content-Type:") {
		t.Fatalf("body-only pagination output included response transcript:\n%s", got)
	}
	for _, want := range []string{`{"id":1}`, `{"id":2}`, `{"id":3}`, `{"id":4}`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected compact item %q in output:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"id": 1`) || strings.Contains(got, "[\n") {
		t.Fatalf("body-only pagination output was pretty-framed:\n%s", got)
	}
}

func TestPaginationStreamingMaxItemsLimitsRecords(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	requests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests++
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

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "ndjson", "--rsh-max-items", "3"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines, got %d:\n%s", len(lines), got)
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
	if strings.Contains(got, `"id":4`) {
		t.Fatalf("expected max-items to stop before id 4, got:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "max-items") {
		t.Fatalf("expected max-items warning on stderr, got %q", errOut.String())
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestPaginationExplicitRenderedBodyNonTTYUsesDocumentRendering(t *testing.T) {
	got := runObjectPages(t, "--rsh-print", "b").Stdout.String()
	if strings.Contains(got, "HTTP/1.1 200 OK") || strings.Contains(got, "Content-Type:") {
		t.Fatalf("body-only output included response transcript:\n%s", got)
	}
	for _, want := range []string{`"id":1`, `"id":2`, `"id":3`, `"id":4`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in auto output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[\n") {
		t.Fatalf("expected non-TTY auto output to render compactly, got:\n%s", got)
	}
}

func TestPaginationStreamingAppliesFilterPerItem(t *testing.T) {
	got := runObjectPages(t, "-f", "body.id").Stdout.String()
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

func TestPaginationJSONFilterAppliesPerItemWithoutCollect(t *testing.T) {
	app := runObjectPages(t, "-f", "body[0]", "-o", "json")

	var values []any
	if err := json.Unmarshal(app.Stdout.Bytes(), &values); err != nil {
		t.Fatalf("expected valid filtered JSON array, got %q: %v", app.Stdout.String(), err)
	}
	if len(values) != 4 {
		t.Fatalf("values length = %d, want 4: %#v", len(values), values)
	}
	for i, value := range values {
		if value != nil {
			t.Fatalf("values[%d] = %#v, want nil", i, value)
		}
	}
}

func TestPaginationCollectFilterUsesMergedDocument(t *testing.T) {
	app := runObjectPages(t, "--rsh-collect", "-f", "body[0]", "-o", "json")

	var value map[string]int
	if err := json.Unmarshal(app.Stdout.Bytes(), &value); err != nil {
		t.Fatalf("expected valid filtered JSON object, got %q: %v", app.Stdout.String(), err)
	}
	if value["id"] != 1 {
		t.Fatalf("id = %d, want 1", value["id"])
	}
}

func TestPaginationStreamingFilterUsesFormatter(t *testing.T) {
	got := runObjectPages(t, "-f", "body", "-o", "yaml").Stdout.String()
	for _, want := range []string{"id: 1", "id: 2", "id: 3", "id: 4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"id":1}`) || strings.Contains(got, `"id": 1`) {
		t.Fatalf("expected filtered paginated stream output to use YAML formatting, got:\n%s", got)
	}
}

func TestPaginationExplicitJSONFilterOmitsResponseContext(t *testing.T) {
	got := runObjectPages(t, "-f", "body", "-o", "json").Stdout.String()
	if strings.Contains(got, "HTTP/1.1 200 OK") || strings.Contains(got, "Content-Type:") {
		t.Fatalf("explicit filtered pagination output included response preamble:\n%s", got)
	}
	for _, want := range []string{`"id": 1`, `"id": 2`, `"id": 3`, `"id": 4`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in filtered JSON output, got:\n%s", want, got)
		}
	}
}

func TestPaginationJSONOutputIsValidJSON(t *testing.T) {
	app := runObjectPages(t, "-o", "json")

	var values []map[string]int
	if err := json.Unmarshal(app.Stdout.Bytes(), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", app.Stdout.String(), err)
	}
	if len(values) != 4 || values[0]["id"] != 1 || values[3]["id"] != 4 {
		t.Fatalf("unexpected output: %#v", values)
	}
}

func TestPaginationCycleDetection(t *testing.T) {
	c, out, errOut := newTestCLI(t)
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
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "json"}); err != nil {
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
	c, out, errOut := newTestCLI(t)
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
	if err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(errOut.String(), "items_path") {
		t.Fatalf("expected items_path warning, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), `"data"`) {
		t.Fatalf("expected wrapped document output, got %q", out.String())
	}
}

func TestPaginationItemsPathNonObjectFails(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"items_path": "data"},
			},
		},
	})
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := os.WriteFile(c.Hooks().ConfigPath, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[1,2,3]`)),
			Request:    r,
		}, nil
	})
	err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"})
	if err == nil || !strings.Contains(err.Error(), "items_path") {
		t.Fatalf("expected items_path error, got %v", err)
	}
}

func TestPaginationNextPathNonStringFails(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"items_path": "data", "next_path": "next"},
			},
		},
	})
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := os.WriteFile(c.Hooks().ConfigPath, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[1],"next":42}`)),
			Request:    r,
		}, nil
	})
	err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"})
	if err == nil || !strings.Contains(err.Error(), "next_path") {
		t.Fatalf("expected next_path error, got %v", err)
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

	c, out, _ := newTestCLI(t)
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

func TestPaginationPageParamGenericRequestStopsOnEmptyPage(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url": "https://api.example.com",
				"pagination": map[string]any{
					"page_param": "page",
				},
			},
		},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	var pages []string
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		body := `[{"id":1}]`
		switch page {
		case "2":
			body = `[{"id":2}]`
		case "3":
			body = `[]`
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var values []map[string]int
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 2 || values[0]["id"] != 1 || values[1]["id"] != 2 {
		t.Fatalf("values = %#v, want ids 1 and 2", values)
	}
	if got, want := strings.Join(pages, ","), ",2,3"; got != want {
		t.Fatalf("page params = %q, want %q", got, want)
	}
}

func TestPaginationPageParamPreservesFlagQuery(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"page_param": "page"},
				"profiles": map[string]any{
					"default": map[string]any{
						"query": []string{"profile=prod"},
					},
				},
			},
		},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	var queries []string
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		queries = append(queries, r.URL.RawQuery)
		page := r.URL.Query().Get("page")
		body := `[{"id":3}]`
		switch page {
		case "4":
			body = `[{"id":4}]`
		case "5":
			body = `[]`
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit query = %q, want 2", got)
		}
		if got := r.URL.Query().Get("profile"); got != "prod" {
			t.Fatalf("profile query = %q, want prod", got)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "myapi/items", "-q", "page=3", "-q", "limit=2", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var values []map[string]int
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 2 || values[0]["id"] != 3 || values[1]["id"] != 4 {
		t.Fatalf("values = %#v, want ids 3 and 4", values)
	}
	for _, want := range []string{"page=3", "page=4", "page=5"} {
		if !strings.Contains(strings.Join(queries, "\n"), want) {
			t.Fatalf("queries %#v missing %q", queries, want)
		}
	}
}

func TestPaginationPageParamUsesExplicitNextFromLaterPage(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"page_param": "page"},
			},
		},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	var queries []string
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		queries = append(queries, r.URL.RawQuery)
		headers := http.Header{"Content-Type": []string{"application/json"}}
		body := `[{"id":1}]`
		switch {
		case r.URL.Query().Get("page") == "2":
			body = `[]`
			headers.Set("Link", `<https://api.example.com/items?cursor=abc>; rel="next"`)
		case r.URL.Query().Get("cursor") == "abc":
			body = `[{"id":3}]`
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var values []map[string]int
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 2 || values[0]["id"] != 1 || values[1]["id"] != 3 {
		t.Fatalf("values = %#v, want ids 1 and 3", values)
	}
	if got, want := strings.Join(queries, ","), ",page=2,cursor=abc"; got != want {
		t.Fatalf("queries = %q, want %q", got, want)
	}
}

func TestPaginationPageParamItemsPathPreservesWrapper(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url": "https://api.example.com",
				"pagination": map[string]any{
					"items_path": "data",
					"page_param": "page",
				},
			},
		},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":1}],"meta":{"page":1}}`
		switch r.URL.Query().Get("page") {
		case "2":
			body = `{"data":[{"id":2}],"meta":{"page":2}}`
		case "3":
			body = `{"data":[],"meta":{"page":3}}`
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var doc struct {
		Data []map[string]int `json:"data"`
		Meta map[string]int   `json:"meta"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("expected wrapped JSON output, got %q: %v", out.String(), err)
	}
	if len(doc.Data) != 2 || doc.Data[0]["id"] != 1 || doc.Data[1]["id"] != 2 {
		t.Fatalf("data = %#v, want ids 1 and 2", doc.Data)
	}
	if doc.Meta["page"] != 1 {
		t.Fatalf("meta = %#v, want first page wrapper metadata", doc.Meta)
	}
}

func TestPaginationPageParamLaterHTTPErrorStopsSuccessfully(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"page_param": "page"},
			},
		},
	})

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	var values []map[string]int
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 1 || values[0]["id"] != 1 {
		t.Fatalf("values = %#v, want only first page", values)
	}
	if !strings.Contains(errOut.String(), "pagination page 2 returned HTTP 404; stopping") {
		t.Fatalf("expected stopping warning, got %q", errOut.String())
	}
}

func TestPaginationPageParamFirstHTTPErrorStillFails(t *testing.T) {
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"myapi": map[string]any{
				"base_url":   "https://api.example.com",
				"pagination": map[string]any{"page_param": "page"},
			},
		},
	})

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, string(cfgData))
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "myapi/items", "-o", "json"})
	if exitCode(err) != 5 {
		t.Fatalf("exit code = %d, want 5 (err=%v)", exitCode(err), err)
	}
}

// TestPaginationProgressOnStderr verifies that progress output goes to stderr
// not stdout when paginating.
func TestPaginationProgressOnStderr(t *testing.T) {
	app := runThreePages(t, "--rsh-max-pages", "1", "-o", "json")

	// Warnings (stderr) must not appear in stdout.
	if strings.Contains(app.Stdout.String(), "warning") || strings.Contains(app.Stdout.String(), "max-pages") {
		t.Errorf("progress/warning leaked to stdout:\n%s", app.Stdout.String())
	}
	// Warning should be on stderr.
	if !strings.Contains(app.Stderr.String(), "max-pages") {
		t.Errorf("expected warning on stderr, got: %q", app.Stderr.String())
	}
	if strings.Contains(app.Stderr.String(), "fetching page") {
		t.Fatalf("pagination progress should not be printed by default, got: %q", app.Stderr.String())
	}
}

func TestPaginationSkipsForHeaderFilter(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	requests := 0
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests++
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"X-Page":       []string{r.URL.Query().Get("page")},
		}
		if r.URL.Query().Get("page") == "" {
			headers.Set("X-Page", "1")
			headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "-f", "headers", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !strings.Contains(out.String(), `"X-Page"`) || strings.Contains(out.String(), `"2"`) {
		t.Fatalf("expected only first-page headers, got:\n%s", out.String())
	}
}

func TestPaginationLinesFilteredStreamHasNoTrailingEmptyArray(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		pages := map[string]struct {
			body string
			next string
		}{
			"":  {`[{"self":"one"}]`, "https://api.example.com/items?page=2"},
			"2": {`[{"self":"two"}]`, ""},
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
	if err := c.Run([]string{"restish", "get", "-f", "body.self", "-o", "lines", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got, want := out.String(), "one\ntwo\n"; got != want {
		t.Fatalf("lines filtered pagination output = %q, want %q", got, want)
	}
}

func TestPaginationPerItemFilterSuggestsBodyPrefix(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		headers := http.Header{"Content-Type": []string{"application/json"}}
		if r.URL.Query().Get("page") == "" {
			headers.Set("Link", `<https://api.example.com/items?page=2>; rel="next"`)
		}
		body := `[{"self":"one"}]`
		if r.URL.Query().Get("page") == "2" {
			body = `[{"self":"two"}]`
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "-f", "self", "-o", "json", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	var values []any
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out.String(), err)
	}
	if len(values) != 2 || values[0] != nil || values[1] != nil {
		t.Fatalf("filtered output = %#v, want two null values", values)
	}
	if got := errOut.String(); !strings.Contains(got, "use 'body.self'") {
		t.Fatalf("expected body prefix hint, got %q", got)
	}
	if strings.Count(errOut.String(), "filter returned no results") != 1 {
		t.Fatalf("expected one hint, got %q", errOut.String())
	}
}

func TestPaginationLaterPageErrorFailsCollectedOutput(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return &http.Response{
				StatusCode: 500,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
			},
			Body:    io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request: r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "json"})
	if exitCode(err) != 5 {
		t.Fatalf("exit code = %d, want 5 (err=%v)", exitCode(err), err)
	}
	if out.String() != "" {
		t.Fatalf("collected pagination should not emit partial JSON document, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "pagination page 2 returned HTTP 500") {
		t.Fatalf("expected page status warning, got %q", errOut.String())
	}
}

func TestPaginationLaterPageErrorKeepsStreamedPartialOutput(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return &http.Response{
				StatusCode: 500,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
			},
			Body:    io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request: r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "ndjson"})
	if exitCode(err) != 5 {
		t.Fatalf("exit code = %d, want 5 (err=%v)", exitCode(err), err)
	}
	if !strings.Contains(out.String(), `"id":1`) {
		t.Fatalf("expected first page item to remain streamed, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "pagination page 2 returned HTTP 500") {
		t.Fatalf("expected page status warning, got %q", errOut.String())
	}
}

func TestPaginationFilteredNDJSONKeepsStreamedPartialOutput(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return &http.Response{
				StatusCode: 500,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
			},
			Body:    io.NopCloser(strings.NewReader(`[{"id":1,"name":"one"}]`)),
			Request: r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-f", "body.{id}", "-o", "ndjson"})
	if exitCode(err) != 5 {
		t.Fatalf("exit code = %d, want 5 (err=%v)", exitCode(err), err)
	}
	if got, want := out.String(), "{\"id\":1}\n"; got != want {
		t.Fatalf("expected filtered first-page NDJSON output to remain streamed, got %q", got)
	}
	if !strings.Contains(errOut.String(), "pagination page 2 returned HTTP 500") {
		t.Fatalf("expected page status warning, got %q", errOut.String())
	}
}

func TestPaginationFilteredNDJSONStreamsBeforeNextPageCompletes(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	writes := make(chan string, 4)
	c.Stdout = &notifyWriter{writer: out, writes: writes}
	page2Started := make(chan struct{})
	allowPage2 := make(chan struct{})
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			select {
			case <-page2Started:
			default:
				close(page2Started)
			}
			<-allowPage2
			return &http.Response{
				StatusCode: 500,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
			},
			Body:    io.NopCloser(strings.NewReader(`[{"id":1,"name":"one"}]`)),
			Request: r,
		}, nil
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", "-f", "body.{id}", "-o", "ndjson", "https://api.example.com/items"})
	}()

	select {
	case <-page2Started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for page 2 request")
	}
	select {
	case got := <-writes:
		if got != "{\"id\":1}\n" {
			t.Fatalf("streamed write = %q, want filtered first-page NDJSON", got)
		}
	case <-time.After(time.Second):
		close(allowPage2)
		t.Fatalf("timed out waiting for filtered first-page NDJSON before page 2 completed; stderr=%q", errOut.String())
	}
	close(allowPage2)
	err := <-errCh
	if exitCode(err) != 5 {
		t.Fatalf("exit code = %d, want 5 (err=%v)", exitCode(err), err)
	}
	if got, want := out.String(), "{\"id\":1}\n"; got != want {
		t.Fatalf("expected only streamed first-page output, got %q", got)
	}
	if !strings.Contains(errOut.String(), "pagination page 2 returned HTTP 500") {
		t.Fatalf("expected page status warning, got %q", errOut.String())
	}
}

func TestPaginationLaterPageErrorCanBeIgnored(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return &http.Response{
				StatusCode: 500,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`[{"id":2}]`)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
			},
			Body:    io.NopCloser(strings.NewReader(`[{"id":1}]`)),
			Request: r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--rsh-ignore-status-code", "-o", "json"}); err != nil {
		t.Fatalf("get with ignore-status failed: %v", err)
	}
	var values []map[string]int
	if err := json.Unmarshal(out.Bytes(), &values); err != nil {
		t.Fatalf("expected JSON output when status ignored, got %q: %v", out.String(), err)
	}
	if len(values) != 2 || values[0]["id"] != 1 || values[1]["id"] != 2 {
		t.Fatalf("expected both pages when status ignored, got %#v", values)
	}
}
