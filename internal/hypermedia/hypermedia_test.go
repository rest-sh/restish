package hypermedia_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rest-sh/restish/v2/internal/hypermedia"
)

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

var base = mustURL("https://api.example.com/items")

// ─── Link header tests ────────────────────────────────────────────────────────

func TestLinkHeaderParser(t *testing.T) {
	hdr := http.Header{
		"Link": {`<https://api.example.com/items?page=2>; rel="next", </items?page=1>; rel="prev"`},
	}
	p := hypermedia.LinkHeaderParser{}
	links := p.ParseLinks(base, hdr, nil)

	got := map[string]string{}
	for _, l := range links {
		got[l.Rel] = l.URI
	}
	if got["next"] != "https://api.example.com/items?page=2" {
		t.Errorf("next: got %q", got["next"])
	}
	if got["prev"] != "https://api.example.com/items?page=1" {
		t.Errorf("prev: got %q", got["prev"])
	}
}

func TestLinkHeaderRelativeResolution(t *testing.T) {
	hdr := http.Header{
		"Link": {`</items?page=3>; rel="next"`},
	}
	p := hypermedia.LinkHeaderParser{}
	links := p.ParseLinks(base, hdr, nil)
	if len(links) == 0 {
		t.Fatal("expected at least one link")
	}
	if links[0].URI != "https://api.example.com/items?page=3" {
		t.Errorf("got %q", links[0].URI)
	}
}

func TestLinkHeaderParserAllowsCommasInURIAndQuotedParams(t *testing.T) {
	hdr := http.Header{
		"Link": {`<https://api.example.com/search?q=a,b>; rel="next"; title="a,b", </items?page=1>; rel="prev"`},
	}
	p := hypermedia.LinkHeaderParser{}
	links := p.ParseLinks(base, hdr, nil)

	got := map[string]string{}
	for _, l := range links {
		got[l.Rel] = l.URI
	}
	if got["next"] != "https://api.example.com/search?q=a,b" {
		t.Fatalf("next = %q", got["next"])
	}
	if got["prev"] != "https://api.example.com/items?page=1" {
		t.Fatalf("prev = %q", got["prev"])
	}
}

// ─── HAL tests ────────────────────────────────────────────────────────────────

func TestHALParser(t *testing.T) {
	body := map[string]any{
		"_links": map[string]any{
			"self": map[string]any{"href": "/items"},
			"next": map[string]any{"href": "/items?page=2"},
		},
		"items": []any{},
	}
	p := hypermedia.HALParser{}
	links := p.ParseLinks(base, nil, body)
	got := map[string]string{}
	for _, l := range links {
		got[l.Rel] = l.URI
	}
	if got["self"] != "https://api.example.com/items" {
		t.Errorf("self: got %q", got["self"])
	}
	if got["next"] != "https://api.example.com/items?page=2" {
		t.Errorf("next: got %q", got["next"])
	}
}

func TestHALParserNoLinks(t *testing.T) {
	body := map[string]any{"items": []any{}}
	p := hypermedia.HALParser{}
	if links := p.ParseLinks(base, nil, body); links != nil {
		t.Errorf("expected nil for non-HAL body, got %v", links)
	}
}

// ─── TSJ tests ────────────────────────────────────────────────────────────────

func TestTSJParser(t *testing.T) {
	body := map[string]any{
		"@id":      "/items/42",
		"@context": "https://schema.org/",
		"name":     "Widget",
	}
	p := hypermedia.TSJParser{}
	links := p.ParseLinks(base, nil, body)
	if len(links) == 0 {
		t.Fatal("expected self link")
	}
	if links[0].Rel != "self" {
		t.Errorf("rel: got %q", links[0].Rel)
	}
	if links[0].URI != "https://api.example.com/items/42" {
		t.Errorf("uri: got %q", links[0].URI)
	}
}

// ─── JSON:API tests ───────────────────────────────────────────────────────────

func TestJSONAPIParser(t *testing.T) {
	body := map[string]any{
		"data": []any{},
		"links": map[string]any{
			"self": "https://api.example.com/items",
			"next": "https://api.example.com/items?page=2",
		},
	}
	p := hypermedia.JSONAPIParser{}
	links := p.ParseLinks(base, nil, body)
	got := map[string]string{}
	for _, l := range links {
		got[l.Rel] = l.URI
	}
	if got["self"] != "https://api.example.com/items" {
		t.Errorf("self: got %q", got["self"])
	}
	if got["next"] != "https://api.example.com/items?page=2" {
		t.Errorf("next: got %q", got["next"])
	}
}

func TestJSONAPIParserRequiresDataKey(t *testing.T) {
	// Without "data" key, JSON:API parser must not fire (avoid HAL false positives).
	body := map[string]any{
		"links": map[string]any{"self": "/items"},
	}
	p := hypermedia.JSONAPIParser{}
	if links := p.ParseLinks(base, nil, body); links != nil {
		t.Errorf("expected nil without data key, got %v", links)
	}
}

func TestJSONAPIParserAcceptsErrorsDocument(t *testing.T) {
	body := map[string]any{
		"errors": []any{map[string]any{"title": "bad"}},
		"links":  map[string]any{"self": "/items"},
	}
	p := hypermedia.JSONAPIParser{}
	links := p.ParseLinks(base, nil, body)
	if len(links) != 1 || links[0].URI != "https://api.example.com/items" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

// ─── Siren tests ──────────────────────────────────────────────────────────────

func TestSirenParser(t *testing.T) {
	body := map[string]any{
		"class": []any{"collection"},
		"links": []any{
			map[string]any{"rel": []any{"self"}, "href": "/items"},
			map[string]any{"rel": []any{"next"}, "href": "/items?page=2"},
		},
	}
	p := hypermedia.SirenParser{}
	links := p.ParseLinks(base, nil, body)
	got := map[string]string{}
	for _, l := range links {
		got[l.Rel] = l.URI
	}
	if got["self"] != "https://api.example.com/items" {
		t.Errorf("self: got %q", got["self"])
	}
	if got["next"] != "https://api.example.com/items?page=2" {
		t.Errorf("next: got %q", got["next"])
	}
}

func TestSirenParserRequiresClassOrEntities(t *testing.T) {
	body := map[string]any{
		"links": []any{
			map[string]any{"rel": []any{"self"}, "href": "/items"},
		},
	}
	p := hypermedia.SirenParser{}
	if links := p.ParseLinks(base, nil, body); links != nil {
		t.Fatalf("expected nil without class/entities, got %#v", links)
	}
}

func TestResolveRejectsMalformedURL(t *testing.T) {
	links := hypermedia.Parse(base, nil, map[string]any{
		"_links": map[string]any{
			"next": map[string]any{"href": "://bad"},
		},
	}, hypermedia.DefaultParsers())
	if links != nil {
		t.Fatalf("expected malformed link to be dropped, got %#v", links)
	}
}

// ─── Parse() aggregation tests ────────────────────────────────────────────────

func TestParseAggregates(t *testing.T) {
	hdr := http.Header{
		"Link": {`<https://api.example.com/items?page=2>; rel="next"`},
	}
	body := map[string]any{
		"_links": map[string]any{
			"self": map[string]any{"href": "/items"},
		},
	}
	links := hypermedia.Parse(base, hdr, body, hypermedia.DefaultParsers())
	if links["next"] != "https://api.example.com/items?page=2" {
		t.Errorf("next: got %q", links["next"])
	}
	if links["self"] != "https://api.example.com/items" {
		t.Errorf("self: got %q", links["self"])
	}
}

func TestParseReturnsNilWhenEmpty(t *testing.T) {
	body := map[string]any{"data": "no links here"}
	links := hypermedia.Parse(base, nil, body, hypermedia.DefaultParsers())
	if links != nil {
		t.Errorf("expected nil, got %v", links)
	}
}
