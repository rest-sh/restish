package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLinksCommandLinkHeader verifies that "links" extracts Link header relations.
func TestLinksCommandLinkHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `</items?page=2>; rel="next", </items?page=0>; rel="prev"`)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "links", srv.URL + "/items"}); err != nil {
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
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		// application/json is a valid content type for HAL bodies.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"_links":{"self":{"href":"/items"},"next":{"href":"/items?page=2"}},"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "links", srv.URL + "/items"}); err != nil {
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
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://api.example.com/items?page=2>; rel="next", <https://api.example.com/>; rel="self"`)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	// Ask only for the "next" relation.
	if err := c.Run([]string{"restish", "links", srv.URL + "/items", "next"}); err != nil {
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
