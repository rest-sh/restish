package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestLinksCommandLinkHeader verifies that "links" extracts Link header relations.
func TestLinksCommandLinkHeader(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Link":         []string{`</items?page=2>; rel="next", </items?page=0>; rel="prev"`},
				"Content-Type": []string{"application/json"},
			},
			Body:    io.NopCloser(strings.NewReader(`[]`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "links", "https://api.example.com/items"}); err != nil {
		t.Fatalf("links: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &got); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if !strings.Contains(got["next"], "page=2") {
		t.Errorf("next: got %q", got["next"])
	}
	if !strings.Contains(got["prev"], "page=0") {
		t.Errorf("prev: got %q", got["prev"])
	}
}

// TestLinksCommandHAL verifies that "links" parses HAL _links.
func TestLinksCommandHAL(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"_links":{"self":{"href":"/items"},"next":{"href":"/items?page=2"}},"items":[]}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "links", "https://api.example.com/items"}); err != nil {
		t.Fatalf("links: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &got); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if !strings.Contains(got["next"], "page=2") {
		t.Errorf("next: got %q", got["next"])
	}
}

// TestLinksCommandFilterRel verifies that [rel...] args filter the output.
func TestLinksCommandFilterRel(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Link":         []string{`<https://api.example.com/items?page=2>; rel="next", <https://api.example.com/>; rel="self"`},
				"Content-Type": []string{"application/json"},
			},
			Body:    io.NopCloser(strings.NewReader(`[]`)),
			Request: r,
		}, nil
	})
	// Ask only for the "next" relation.
	if err := c.Run([]string{"restish", "links", "https://api.example.com/items", "next"}); err != nil {
		t.Fatalf("links: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &got); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Errorf("expected exactly 1 relation, got %d: %v", len(got), got)
	}
	if _, ok := got["next"]; !ok {
		t.Errorf("expected next relation, got: %v", got)
	}
}
