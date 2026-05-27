// Package hypermedia provides parsers that extract typed links from HTTP
// responses in various hypermedia formats (Link headers, HAL, TSJ, JSON:API,
// Siren). All URIs are resolved to absolute form before being returned.
package hypermedia

import (
	"net/http"
	"net/url"
)

// Link is a normalized hypermedia link.
type Link struct {
	Rel string // relation type, e.g. "next", "self"
	URI string // always an absolute URI
}

// Parser extracts links from a response.
type Parser interface {
	// ParseLinks returns all links it can find. Implementations return nil
	// when they cannot recognise the format.
	ParseLinks(baseURL *url.URL, header http.Header, body any) []Link
}

// Parse runs all parsers against the response and returns a rel→uri map.
// When multiple parsers produce the same rel, the last value wins.
// Returns nil if no links are found.
func Parse(baseURL *url.URL, header http.Header, body any, parsers []Parser) map[string]string {
	links := make(map[string]string)
	for _, p := range parsers {
		for _, l := range p.ParseLinks(baseURL, header, body) {
			if l.URI != "" && l.Rel != "" {
				links[l.Rel] = l.URI
			}
		}
	}
	if len(links) == 0 {
		return nil
	}
	return links
}

// DefaultParsers returns the built-in set of hypermedia parsers in priority
// order (Link header first, then body-based formats).
func DefaultParsers() []Parser {
	return []Parser{
		LinkHeaderParser{},
		HALParser{},
		TSJParser{},
		JSONAPIParser{},
		SirenParser{},
	}
}

// resolve resolves ref against base, returning an absolute URI string.
// Returns ref unchanged if it is already absolute or base is nil.
func resolve(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.String()
}
