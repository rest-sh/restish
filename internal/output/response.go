package output

import (
	"fmt"
	"io"
	"net/http"

	"github.com/rest-sh/restish/v2/internal/content"
)

// DefaultMaxBodyBytes is the default cap on response body reads (100 MiB).
// A server cannot allocate more than this per response.
const DefaultMaxBodyBytes int64 = 100 * 1024 * 1024

// Response is the normalized form of every HTTP response before formatting.
// All formatters receive this struct; nothing downstream touches *http.Response.
type Response struct {
	Proto   string              `json:"proto"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	// URL is the final response URL. It is used for presentation concerns such
	// as syntax highlighting file-like text responses by extension.
	URL string `json:"-"`
	// Links is populated by hypermedia parsers; empty until then.
	Links map[string]any `json:"links,omitempty"`
	Body  any            `json:"body"`
	// Raw holds the unformatted response body after Content-Encoding
	// decompression. Used by raw CLI output and binary/content-aware formatters
	// to write body bytes without formatter re-encoding.
	Raw []byte `json:"-"`
}

// Normalize reads resp.Body, decodes it using the provided content registry,
// and returns a Response. resp.Body is fully consumed and closed before this
// returns. maxBytes caps the body read; pass DefaultMaxBodyBytes or 0 to use
// the default.
func Normalize(resp *http.Response, reg *content.Registry, maxBytes int64) (*Response, error) {
	defer resp.Body.Close()

	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}

	// Canonicalise headers. Go's http package already canonicalises keys; keep
	// all values so repeated headers such as Set-Cookie and Link are preserved.
	headers := make(map[string][]string, len(resp.Header))
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = append([]string(nil), vals...)
		}
	}

	body, raw, err := decodeBody(resp, reg, maxBytes)
	if err != nil {
		return nil, err
	}

	return &Response{
		Proto:   resp.Proto,
		Status:  resp.StatusCode,
		Headers: headers,
		URL:     responseURL(resp),
		Body:    body,
		Raw:     raw,
	}, nil
}

func responseURL(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	return resp.Request.URL.String()
}

// decodeBody reads the response body, decompresses Content-Encoding if needed,
// then decodes it using the content registry. Returns the decoded body bytes so
// callers can write them without formatter re-encoding if needed.
func decodeBody(resp *http.Response, reg *content.Registry, maxBytes int64) (decoded any, raw []byte, err error) {
	encoding := resp.Header.Get("Content-Encoding")
	reader, err := reg.Decompress(encoding, resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decompressing response: %w", err)
	}
	defer reader.Close()

	// Read up to maxBytes+1 so we can detect a body that exceeds the limit.
	raw, err = io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("reading response body: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, nil, fmt.Errorf("response body exceeds limit of %d bytes; use --rsh-max-body-size to increase", maxBytes)
	}
	if len(raw) == 0 {
		return nil, nil, nil
	}

	ct := resp.Header.Get("Content-Type")
	decoded, err = reg.Decode(ct, raw)
	return decoded, raw, err
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
