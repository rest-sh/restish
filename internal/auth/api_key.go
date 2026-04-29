package auth

import (
	"context"
	"fmt"
	"net/http"
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
		req.Header.Set(name, value)
	case "query":
		q := req.URL.Query()
		q.Set(name, value)
		req.URL.RawQuery = q.Encode()
	case "cookie":
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	default:
		return fmt.Errorf("api-key: unsupported in %q (supported: header, query, cookie)", location)
	}
	return nil
}
