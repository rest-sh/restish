package cli

import (
	"context"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
)

// FetchOptions configures one programmatic request without changing CLI defaults.
type FetchOptions struct {
	// ProfileName selects an API profile. An empty value uses "default".
	ProfileName string
	// Headers adds "Name: Value" headers after the profile's persistent headers.
	Headers []string
	// NoBrowser prevents automatic browser launch during authentication.
	// Interactive callers can enter an authorization code manually.
	NoBrowser bool
}

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
	return c.FetchResponseWithOptions(ctx, method, rawURL, FetchOptions{ProfileName: profileName, Headers: rawHeaders})
}

// FetchResponseWithOptions executes one request using caller-owned context and
// explicit authentication options. It does not run commands, paginate, filter,
// stream, or write the response to CLI output.
func (client *CLI) FetchResponseWithOptions(ctx context.Context, method, rawURL string, options FetchOptions) (*output.Response, error) {
	if options.ProfileName == "" {
		options.ProfileName = "default"
	}
	opts := request.Options{
		AcceptHeader:         client.content.AcceptHeader(),
		AcceptEncodingHeader: client.content.AcceptEncodingHeader(),
		UserAgent:            "restish/" + Version,
		Transport:            client.baseHTTPTransport(),
	}
	if len(options.Headers) > 0 {
		opts.Headers = options.Headers
	}

	prepared, err := client.prepareRequest(ctx, method, rawURL, options.ProfileName, opts, nil, nil, false, authHandlerOptions{NoBrowser: options.NoBrowser}, nil, false, "")
	if err != nil {
		return nil, err
	}
	defer client.closePreparedTransport(prepared)

	httpResp, err := client.sendPreparedRequest(ctx, method, prepared)
	if err != nil {
		return nil, err
	}
	return client.normalizeHTTPResponse(httpResp, output.DefaultMaxBodyBytes)
}
