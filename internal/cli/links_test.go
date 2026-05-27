package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func linksResponse(r *http.Request, status int, headers http.Header, body string) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: status,
		Proto:      "HTTP/1.1",
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func decodeLinksOutput(t *testing.T, out string) map[string]string {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out)
	}
	return got
}

// TestLinksCommandLinkHeader verifies that "links" extracts Link header relations.
func TestLinksCommandLinkHeader(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 200, http.Header{
			"Link": []string{`</items?page=2>; rel="next", </items?page=0>; rel="prev"`},
		}, `[]`), nil
	})

	app.Run("links", "https://api.example.com/items")
	got := decodeLinksOutput(t, app.Stdout.String())
	if !strings.Contains(got["next"], "page=2") {
		t.Errorf("next: got %q", got["next"])
	}
	if !strings.Contains(got["prev"], "page=0") {
		t.Errorf("prev: got %q", got["prev"])
	}
}

// TestLinksCommandHAL verifies that "links" parses HAL _links.
func TestLinksCommandHAL(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 200, nil, `{"_links":{"self":{"href":"/items"},"next":{"href":"/items?page=2"}},"items":[]}`), nil
	})

	app.Run("links", "https://api.example.com/items")
	got := decodeLinksOutput(t, app.Stdout.String())
	if !strings.Contains(got["next"], "page=2") {
		t.Errorf("next: got %q", got["next"])
	}
}

func TestLinksCommandJSONOutputFlag(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 200, http.Header{
			"Link": []string{`</items?page=2>; rel="next"`},
		}, `[]`), nil
	})

	app.Run("links", "https://api.example.com/items", "-o", "json")
	got := decodeLinksOutput(t, app.Stdout.String())
	if !strings.Contains(got["next"], "page=2") {
		t.Errorf("next: got %q", got["next"])
	}
}

func TestLinksCommandRejectsUnsupportedOutputFormat(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	err := app.RunErr("links", "https://api.example.com/items", "-o", "yaml")
	if err == nil {
		t.Fatal("expected unsupported output format error")
	}
	if !strings.Contains(err.Error(), "supports -o json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLinksCommandFilterRel verifies that [rel...] args filter the output.
func TestLinksCommandFilterRel(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 200, http.Header{
			"Link": []string{`<https://api.example.com/items?page=2>; rel="next", <https://api.example.com/>; rel="self"`},
		}, `[]`), nil
	})

	app.Run("links", "https://api.example.com/items", "next")
	got := decodeLinksOutput(t, app.Stdout.String())
	if len(got) != 1 {
		t.Errorf("expected exactly 1 relation, got %d: %v", len(got), got)
	}
	if _, ok := got["next"]; !ok {
		t.Errorf("expected next relation, got: %v", got)
	}
}

func TestLinksCommandWarnsForMissingRel(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 200, http.Header{
			"Link": []string{`<https://api.example.com/items?page=2>; rel="next"`},
		}, `[]`), nil
	})
	app.Run("links", "https://api.example.com/items", "missing")
	if strings.TrimSpace(app.Stdout.String()) != "{}" {
		t.Fatalf("expected empty object for missing rel, got:\n%s", app.Stdout.String())
	}
	requireContains(t, app.Stderr.String(), `rel "missing" not found`, "next")
}

func TestLinksCommandReturnsStatusError(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 500, nil, `{"error":"boom"}`), nil
	})
	err := app.RunErr("links", "https://api.example.com/items")
	if err == nil {
		t.Fatal("expected links to return a status error")
	}
	if strings.TrimSpace(app.Stdout.String()) != "{}" {
		t.Fatalf("expected links output before status error, got:\n%s", app.Stdout.String())
	}
}

func TestLinksCommandIgnoreStatusCode(t *testing.T) {
	app := newTestApp(t)
	app.FreshConfigPath()
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		return linksResponse(r, 404, nil, `{"error":"missing"}`), nil
	})
	app.Run("links", "https://api.example.com/items", "--rsh-ignore-status-code")
}
