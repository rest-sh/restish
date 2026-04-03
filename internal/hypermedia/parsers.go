package hypermedia

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ─── Link header (RFC 5988) ───────────────────────────────────────────────────

// LinkHeaderParser extracts links from HTTP Link headers.
// Example: Link: <https://api.example.com/page2>; rel="next"
type LinkHeaderParser struct{}

// linkURI matches the <uri> portion of a single link element.
var linkURIRe = regexp.MustCompile(`<([^>]*)>`)

// relParam extracts rel= (quoted or unquoted) from link parameters.
var relParamRe = regexp.MustCompile(`(?i);\s*rel\s*=\s*(?:"([^"]*)"|([\w!#$&'*+.^_` + "`" + `|~-]+))`)

func (LinkHeaderParser) ParseLinks(baseURL *url.URL, header http.Header, _ any) []Link {
	var result []Link
	for _, h := range header["Link"] {
		// Each comma-separated segment is one link element. Commas inside <>
		// or quotes are unlikely in Link headers, so a simple split suffices.
		for _, part := range strings.Split(h, ",") {
			part = strings.TrimSpace(part)
			uriM := linkURIRe.FindStringSubmatch(part)
			relM := relParamRe.FindStringSubmatch(part)
			if len(uriM) < 2 || len(relM) < 3 {
				continue
			}
			rel := relM[1]
			if rel == "" {
				rel = relM[2]
			}
			// rel may contain multiple space-separated relation types; use each.
			for _, r := range strings.Fields(rel) {
				result = append(result, Link{Rel: r, URI: resolve(baseURL, uriM[1])})
			}
		}
	}
	return result
}

// ─── HAL (application/hal+json) ──────────────────────────────────────────────

// HALParser extracts links from the `_links` object.
type HALParser struct{}

func (HALParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	linksRaw, ok := m["_links"]
	if !ok {
		return nil
	}
	links, ok := linksRaw.(map[string]any)
	if !ok {
		return nil
	}
	var result []Link
	for rel, v := range links {
		switch lv := v.(type) {
		case map[string]any:
			if href, ok := lv["href"].(string); ok {
				result = append(result, Link{Rel: rel, URI: resolve(baseURL, href)})
			}
		case []any:
			for _, item := range lv {
				if lm, ok := item.(map[string]any); ok {
					if href, ok := lm["href"].(string); ok {
						result = append(result, Link{Rel: rel, URI: resolve(baseURL, href)})
					}
				}
			}
		}
	}
	return result
}

// ─── TSJ / JSON-LD ───────────────────────────────────────────────────────────

// TSJParser extracts a self link from the `@id` field (JSON-LD / TSJ).
type TSJParser struct{}

func (TSJParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	id, ok := m["@id"].(string)
	if !ok || id == "" {
		return nil
	}
	return []Link{{Rel: "self", URI: resolve(baseURL, id)}}
}

// ─── JSON:API ─────────────────────────────────────────────────────────────────

// JSONAPIParser extracts links from the top-level `links` object.
// Requires the response to also have a `data` key to avoid false positives.
type JSONAPIParser struct{}

func (JSONAPIParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	// Require `data` key to identify JSON:API (avoid matching Siren/other).
	if _, hasData := m["data"]; !hasData {
		return nil
	}
	linksRaw, ok := m["links"]
	if !ok {
		return nil
	}
	links, ok := linksRaw.(map[string]any)
	if !ok {
		return nil
	}
	var result []Link
	for rel, v := range links {
		switch lv := v.(type) {
		case string:
			if lv != "" {
				result = append(result, Link{Rel: rel, URI: resolve(baseURL, lv)})
			}
		case map[string]any:
			if href, ok := lv["href"].(string); ok && href != "" {
				result = append(result, Link{Rel: rel, URI: resolve(baseURL, href)})
			}
		}
	}
	return result
}

// ─── Siren ────────────────────────────────────────────────────────────────────

// SirenParser extracts links from the `links` array.
// Siren link objects have `rel` (array of strings) and `href`.
type SirenParser struct{}

func (SirenParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	linksRaw, ok := m["links"]
	if !ok {
		return nil
	}
	linksArr, ok := linksRaw.([]any)
	if !ok {
		return nil
	}
	var result []Link
	for _, item := range linksArr {
		lm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		href, _ := lm["href"].(string)
		if href == "" {
			continue
		}
		switch r := lm["rel"].(type) {
		case []any:
			for _, rv := range r {
				if s, ok := rv.(string); ok && s != "" {
					result = append(result, Link{Rel: s, URI: resolve(baseURL, href)})
				}
			}
		case string:
			if r != "" {
				result = append(result, Link{Rel: r, URI: resolve(baseURL, href)})
			}
		}
	}
	return result
}
