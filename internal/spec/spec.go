// Package spec handles API specification discovery, loading, and caching.
package spec

import (
	"sync"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

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
	Document libopenapi.Document

	// modelOnce guards lazy construction of the V3 model.
	modelOnce   sync.Once
	modelResult *libopenapi.DocumentModel[v3.Document]
	modelErr    error

	// opsCacheMu guards the operations cache.
	opsCacheMu sync.Mutex
	opsCache   map[opsKey]opsEntry
}

// APIInfo is the top-level OpenAPI info object fields Restish surfaces in
// generated command help and cached operation metadata.
type APIInfo struct {
	Title       string
	Summary     string
	Description string
	Version     string
}

type opsKey struct{ baseURL, operationBase string }
type opsEntry struct {
	ops []Operation
	err error
}

// V3Model returns the built V3 document model, memoizing the result so that
// Document.BuildV3Model() is called at most once per APISpec.
func (s *APISpec) V3Model() (*libopenapi.DocumentModel[v3.Document], error) {
	s.modelOnce.Do(func() {
		s.modelResult, s.modelErr = s.Document.BuildV3Model()
	})
	return s.modelResult, s.modelErr
}

// Info returns top-level OpenAPI info metadata from the V3 model.
func (s *APISpec) Info() (APIInfo, error) {
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Info == nil {
		return APIInfo{}, err
	}
	info := model.Model.Info
	return APIInfo{
		Title:       info.Title,
		Summary:     info.Summary,
		Description: info.Description,
		Version:     info.Version,
	}, nil
}

// DefaultLoaders returns the built-in set of loaders.
func DefaultLoaders() []Loader {
	return []Loader{OpenAPILoader{}}
}

// load tries each loader (highest priority first) and returns the first match.
// Returns nil, nil if no loader recognises the content.
// Loaders that set ContentType or Raw in the returned spec retain their values;
// the caller-supplied contentType and body are only used as fallbacks.
func load(contentType string, body []byte, loaders []Loader) (*APISpec, error) {
	best := pickLoader(contentType, body, loaders)
	if best == nil {
		return nil, nil
	}
	spec, err := best.Load(body)
	if err != nil {
		return nil, err
	}
	if spec.ContentType == "" {
		spec.ContentType = contentType
	}
	if len(spec.Raw) == 0 {
		spec.Raw = body
	}
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
