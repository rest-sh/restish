package cli

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type errorLinkParser struct{}

func (p errorLinkParser) ParseLinks(r *Response) error {
	return fmt.Errorf("error parsing links")
}

func TestLinkParserFailure(t *testing.T) {
	AddLinkParser(errorLinkParser{})
	u, _ := url.Parse("https://example.com/test")
	r := &Response{
		Links:   Links{},
		Headers: map[string]string{},
		Body:    nil,
	}
	err := ParseLinks(u, r)
	assert.Error(t, err)
}

func TestLinkHeaderParser(t *testing.T) {
	r := &Response{
		Links: Links{},
		Headers: map[string]string{
			"Link": `</self>; rel="self", </foo>; rel="item", </bar>; rel="item"`,
		},
	}

	p := LinkHeaderParser{}
	err := p.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/self")
	assert.Equal(t, r.Links["item"][0].URI, "/foo")
	assert.Equal(t, r.Links["item"][1].URI, "/bar")

	// Test a bad link header
	r.Headers["Link"] = "bad value"
	err = p.ParseLinks(r)
	assert.Error(t, err)
}

func TestHALParser(t *testing.T) {
	r := &Response{
		Links: Links{},
		Body: map[string]any{
			"_links": map[string]any{
				"curies": nil,
				"self": map[string]any{
					"href": "/self",
				},
				"item": map[string]any{
					"href": "/item",
				},
			},
		},
	}

	p := HALParser{}
	err := p.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/self")
	assert.Equal(t, r.Links["item"][0].URI, "/item")
}

func TestHALParserArray(t *testing.T) {
	r := &Response{
		Links: Links{},
		Body: []any{
			map[string]any{
				"_links": map[string]any{
					"self": map[string]any{
						"href": "/one",
					},
				},
			},
			map[string]any{
				"_links": map[string]any{
					"self": map[string]any{
						"href": "/two",
					},
				},
			},
		},
	}

	p := HALParser{}
	err := p.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/one")
	assert.Equal(t, r.Links["self"][1].URI, "/two")
}

func TestTerrificallySimpleJSONParser(t *testing.T) {
	r := &Response{
		Links: Links{},
		Body: map[string]any{
			"self": "/self",
			"things": []any{
				map[string]any{
					"self": "/foo",
					"name": "Foo",
				},
				map[string]any{
					"self": "/bar",
					"name": "Bar",
				},
				// Weird object with int keys instead of strings? Possible with binary
				// formats but not JSON itself.
				&map[int]any{
					5: map[string]any{
						"self": "/weird",
					},
				},
			},
			"other": map[string]any{
				"self": map[string]any{
					"foo": "bar",
				},
			},
		},
	}

	p := TerrificallySimpleJSONParser{}
	err := p.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/self")
	assert.Equal(t, r.Links["things-item"][0].URI, "/foo")
	assert.Equal(t, r.Links["things-item"][1].URI, "/bar")
	assert.Equal(t, r.Links["5"][0].URI, "/weird")
	assert.NotContains(t, r.Links, "other")
	assert.NotContains(t, r.Links, "foo")
}

func TestSirenParser(t *testing.T) {
	r := &Response{
		Links: Links{},
		Body: map[string]any{
			"links": []map[string]any{
				{"rel": []string{"self"}, "href": "/self"},
				{"rel": []string{"one", "two"}, "href": "/multi"},
				{"rel": []string{"invalid"}},
			},
		},
	}

	s := SirenParser{}
	err := s.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/self")
	assert.Equal(t, r.Links["one"][0].URI, "/multi")
	assert.Equal(t, r.Links["two"][0].URI, "/multi")
}

func TestJSONAPIParser(t *testing.T) {
	r := &Response{
		Links: Links{},
		Body: map[string]any{
			"links": map[string]any{
				"self": "/self",
			},
			"data": []any{
				map[string]any{
					"links": map[string]any{
						"self": map[string]any{
							"href": "/item",
						},
					},
				},
			},
		},
	}

	j := JSONAPIParser{}
	err := j.ParseLinks(r)
	assert.NoError(t, err)
	assert.Equal(t, r.Links["self"][0].URI, "/self")
	assert.Equal(t, r.Links["item"][0].URI, "/item")
}
