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
	return LinkHeaderLinks(baseURL, header)
}

// LinkHeaderLinks extracts all rel/URI pairs from HTTP Link headers.
func LinkHeaderLinks(baseURL *url.URL, header http.Header) []Link {
	var result []Link
	for _, h := range header["Link"] {
		for _, part := range splitLinkHeaderValues(h) {
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

func splitLinkHeaderValues(value string) []string {
	var parts []string
	start := 0
	inAngle := false
	inQuote := false
	escaped := false
	for i, r := range value {
		switch {
		case escaped:
			escaped = false
		case inQuote && r == '\\':
			escaped = true
		case r == '"' && !inAngle:
			inQuote = !inQuote
		case r == '<' && !inQuote:
			inAngle = true
		case r == '>' && !inQuote:
			inAngle = false
		case r == ',' && !inAngle && !inQuote:
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	parts = append(parts, value[start:])
	return parts
}

// ─── HAL (application/hal+json) ──────────────────────────────────────────────

// HALParser extracts links from the `_links` object.
type HALParser struct{}

func (HALParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	switch typed := body.(type) {
	case map[string]any:
		return halLinksFromMap(baseURL, typed)
	case []any:
		var result []Link
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				result = append(result, halLinksFromMap(baseURL, m)...)
			}
		}
		return result
	default:
		return nil
	}
}

func halLinksFromMap(baseURL *url.URL, m map[string]any) []Link {
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
	if ok && id != "" {
		return []Link{{Rel: "self", URI: resolve(baseURL, id)}}
	}
	var result []Link
	collectSimpleJSONLinks(baseURL, body, "", &result)
	return result
}

func collectSimpleJSONLinks(baseURL *url.URL, body any, relPrefix string, result *[]Link) {
	switch typed := body.(type) {
	case map[string]any:
		if self, ok := typed["self"].(string); ok && self != "" {
			rel := relPrefix
			if rel == "" {
				rel = "self"
			}
			*result = append(*result, Link{Rel: rel, URI: resolve(baseURL, self)})
		}
		for key, value := range typed {
			if key == "self" || key == "links" || key == "_links" || key == "data" {
				continue
			}
			collectSimpleJSONLinks(baseURL, value, key, result)
		}
	case []any:
		rel := "item"
		if relPrefix != "" {
			rel = relPrefix + "-item"
		}
		for _, item := range typed {
			collectSimpleJSONLinks(baseURL, item, rel, result)
		}
	}
}

// ─── JSON:API ─────────────────────────────────────────────────────────────────

// JSONAPIParser extracts links from the top-level `links` object.
// Requires either a `jsonapi` object or one of the JSON:API document members.
type JSONAPIParser struct{}

func (JSONAPIParser) ParseLinks(baseURL *url.URL, _ http.Header, body any) []Link {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	if _, hasJSONAPI := m["jsonapi"]; !hasJSONAPI {
		_, hasData := m["data"]
		_, hasErrors := m["errors"]
		if !hasData && !hasErrors {
			return nil
		}
	}
	var result []Link
	if links, ok := m["links"].(map[string]any); ok {
		appendJSONAPILinks(baseURL, links, "", &result)
	}
	appendJSONAPIResourceLinks(baseURL, m["data"], &result)
	return result
}

func appendJSONAPIResourceLinks(baseURL *url.URL, data any, result *[]Link) {
	switch typed := data.(type) {
	case map[string]any:
		if links, ok := typed["links"].(map[string]any); ok {
			appendJSONAPILinks(baseURL, links, "item", result)
		}
	case []any:
		for _, item := range typed {
			appendJSONAPIResourceLinks(baseURL, item, result)
		}
	}
}

func appendJSONAPILinks(baseURL *url.URL, links map[string]any, selfRel string, result *[]Link) {
	for rel, v := range links {
		outRel := rel
		if rel == "self" && selfRel != "" {
			outRel = selfRel
		}
		switch lv := v.(type) {
		case string:
			if lv != "" {
				*result = append(*result, Link{Rel: outRel, URI: resolve(baseURL, lv)})
			}
		case map[string]any:
			if href, ok := lv["href"].(string); ok && href != "" {
				*result = append(*result, Link{Rel: outRel, URI: resolve(baseURL, href)})
			}
		}
	}
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
	if _, hasClass := m["class"]; !hasClass {
		if _, hasEntities := m["entities"]; !hasEntities {
			return nil
		}
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
