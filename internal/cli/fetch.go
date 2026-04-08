package cli

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
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

	rawURL, _, opts = c.applyAPIProfile(rawURL, profileName, opts)
	var err error
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return nil, err
	}
	opts.Transport = request.BuildTransport(opts)

	// Wrap OnRequest so request middleware plugins are also invoked.
	origOnReq := opts.OnRequest
	opts.OnRequest = func(req *http.Request) error {
		if origOnReq != nil {
			if err := origOnReq(req); err != nil {
				return err
			}
		}
		return c.runRequestMiddlewarePlugins(req)
	}

	httpResp, err := request.Do(ctx, method, rawURL, nil, opts)
	if err != nil {
		return nil, err
	}
	return output.Normalize(httpResp, c.content, output.DefaultMaxBodyBytes)
}
