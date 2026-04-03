// Package spec handles API specification discovery, loading, and caching.
package spec

import "github.com/pb33f/libopenapi"

// Loader detects and parses an API specification format.
// Multiple loaders may be registered; the highest-priority one that detects
// a given body is used.
type Loader interface {
	// Detect returns true if this loader recognises the content type and/or body.
	Detect(contentType string, body []byte) bool
	// Load parses body and returns a structured APISpec.
	Load(body []byte) (*APISpec, error)
	// Priority determines loader selection order; higher priority wins.
	Priority() int
}

// APISpec is a parsed API specification.
type APISpec struct {
	// ContentType is the MIME type the spec was fetched with.
	ContentType string
	// Raw is the original spec bytes (JSON or YAML).
	Raw []byte
	// Document is the libopenapi parsed representation.
	// Nil when loaded from cache before re-parsing.
	Document libopenapi.Document
}

// DefaultLoaders returns the built-in set of loaders.
func DefaultLoaders() []Loader {
	return []Loader{OpenAPILoader{}}
}

// load tries each loader (highest priority first) and returns the first match.
// Returns nil, nil if no loader recognises the content.
func load(contentType string, body []byte, loaders []Loader) (*APISpec, error) {
	best := pickLoader(contentType, body, loaders)
	if best == nil {
		return nil, nil
	}
	spec, err := best.Load(body)
	if err != nil {
		return nil, err
	}
	spec.ContentType = contentType
	spec.Raw = body
	return spec, nil
}

// pickLoader returns the highest-priority loader that detects the content, or nil.
func pickLoader(contentType string, body []byte, loaders []Loader) Loader {
	var best Loader
	for _, l := range loaders {
		if l.Detect(contentType, body) {
			if best == nil || l.Priority() > best.Priority() {
				best = l
			}
		}
	}
	return best
}
