package spec

import (
	"bytes"
	"strings"

	"github.com/pb33f/libopenapi"
)

// OpenAPILoader handles OpenAPI 3.0 and 3.1 specifications.
type OpenAPILoader struct{}

func (OpenAPILoader) Priority() int { return 10 }

// Detect returns true when the content type or body look like an OpenAPI spec.
// It accepts JSON, YAML, and the official OpenAPI MIME types, then confirms by
// sniffing for an "openapi:" / `"openapi"` key in the first 512 bytes.
func (OpenAPILoader) Detect(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "openapi") {
		return true
	}
	// Accept OpenAPI-specific MIME types and common JSON/YAML types.
	if !strings.Contains(ct, "json") &&
		!strings.Contains(ct, "yaml") &&
		ct != "" {
		return false
	}

	// Body sniff: look for the "openapi" field. Some generated specs write
	// the top-level openapi field late in the document, so generic JSON/YAML
	// cannot rely on a tiny prefix sniff.
	sniff := body
	low := bytes.ToLower(sniff)
	return bytes.Contains(low, []byte(`"openapi"`)) ||
		bytes.Contains(low, []byte("openapi:"))
}

// Load parses body as an OpenAPI 3.x document.
func (OpenAPILoader) Load(body []byte) (*APISpec, error) {
	doc, err := libopenapi.NewDocument(body)
	if err != nil {
		return nil, &LoadError{Errors: []string{err.Error()}}
	}
	return &APISpec{Raw: body, Document: doc}, nil
}

// LoadError wraps one or more errors returned by the libopenapi parser.
type LoadError struct {
	Errors []string
}

func (e *LoadError) Error() string {
	return "openapi: " + strings.Join(e.Errors, "; ")
}
