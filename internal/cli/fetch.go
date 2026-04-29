package cli

import (
	"context"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
)

// FetchResponse executes a single HTTP request and returns the normalized
// response. It applies authentication and profile settings when rawURL matches
// a configured API or API short name, but does not paginate, filter, stream,
// or write any output.
//
// profileName selects the active profile; an empty string uses "default".
// rawHeaders contains zero or more "Name: Value" strings that are appended
// after any persistent headers from the matched profile.
//
// FetchResponse is intended for embedders that need programmatic access to API
// data. For full CLI behaviour (output formatting, retries, pagination) use
// CLI.Run instead.
func (c *CLI) FetchResponse(ctx context.Context, method, rawURL, profileName string, rawHeaders []string) (*output.Response, error) {
	if profileName == "" {
		profileName = "default"
	}
	opts := request.Options{}
	if len(rawHeaders) > 0 {
		opts.Headers = rawHeaders
	}

	prepared, err := c.prepareRequest(rawURL, profileName, opts, nil, nil, false, authHandlerOptions{}, nil)
	if err != nil {
		return nil, err
	}
	defer c.closePreparedTransport(prepared)

	httpResp, err := c.sendPreparedRequest(ctx, method, prepared)
	if err != nil {
		return nil, err
	}
	return c.normalizeHTTPResponse(httpResp, output.DefaultMaxBodyBytes)
}
