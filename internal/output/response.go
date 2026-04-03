package output

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Response is the normalized form of every HTTP response before formatting.
// All formatters receive this struct; nothing downstream touches *http.Response.
type Response struct {
	Proto   string            `json:"proto"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	// Links is populated by hypermedia parsers (Step 18); empty until then.
	Links map[string]any `json:"links,omitempty"`
	Body  any            `json:"body"`
}

// Normalize reads resp.Body, decodes it, and returns a Response.
// resp.Body is fully consumed and closed before this returns.
func Normalize(resp *http.Response) (*Response, error) {
	defer resp.Body.Close()

	// Canonicalise headers. Go's http package already canonicalises keys;
	// we flatten multi-value headers to the first value for simplicity.
	headers := make(map[string]string, len(resp.Header))
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	body, err := decodeBody(resp)
	if err != nil {
		return nil, err
	}

	return &Response{
		Proto:   resp.Proto,
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    body,
	}, nil
}

// decodeBody reads the response body and decodes it based on Content-Type.
// JSON bodies are parsed into Go values; everything else is kept as a string.
// Content-type handling is expanded in Step 5.
func decodeBody(resp *http.Response) (any, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	ct := resp.Header.Get("Content-Type")
	if isJSON(ct) {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			return v, nil
		}
		// Malformed JSON: fall through and return as string.
	}

	return string(data), nil
}

func isJSON(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}

// StatusToExitCode maps an HTTP status code to a CLI exit code.
//
//	2xx → 0  (success)
//	3xx → 3  (redirect — often unintentional in scripts)
//	4xx → 4  (client error)
//	5xx → 5  (server error)
func StatusToExitCode(status int) int {
	switch {
	case status >= 200 && status < 300:
		return 0
	case status >= 300 && status < 400:
		return 3
	case status >= 400 && status < 500:
		return 4
	default:
		return 5
	}
}
