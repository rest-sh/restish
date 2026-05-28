package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// APIKey implements OpenAPI-style API key authentication in a header, query
// parameter, or cookie.
type APIKey struct{}

func (h *APIKey) Parameters() []Param {
	return []Param{
		{Name: "in", Description: "API key location: header, query, or cookie", Required: true},
		{Name: "name", Description: "API key header, query parameter, or cookie name", Required: true},
		{Name: "value", Description: "API key value", Required: true, Secret: true},
	}
}

func (h *APIKey) Authenticate(_ context.Context, req *http.Request, ac AuthContext) error {
	location := ac.Params["in"]
	name := ac.Params["name"]
	value := ac.Params["value"]
	if location == "" {
		return fmt.Errorf("api-key: in is required")
	}
	if name == "" {
		return fmt.Errorf("api-key: name is required")
	}
	if value == "" {
		return fmt.Errorf("api-key: value is required")
	}

	switch location {
	case "header":
		if getHeaderCaseInsensitive(req.Header, name) == "" {
			req.Header.Set(name, value)
		}
	case "query":
		q := req.URL.Query()
		if !q.Has(name) {
			q.Set(name, value)
			req.URL.RawQuery = q.Encode()
		}
	case "cookie":
		if _, err := req.Cookie(name); err != nil {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
	default:
		return fmt.Errorf("api-key: unsupported in %q (supported: header, query, cookie)", location)
	}
	return nil
}

func getHeaderCaseInsensitive(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}
	for existing, values := range header {
		if strings.EqualFold(existing, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
