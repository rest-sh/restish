package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/restish/v2/internal/hypermedia"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
)

type preparedRequest struct {
	rawURL  string
	apiName string
	opts    request.Options
	body    io.Reader
}

func (c *CLI) prepareRequest(
	rawURL, profileName string,
	opts request.Options,
	bodyValue any,
	extraHeaders []string,
	noAuth bool,
) (*preparedRequest, error) {
	opts = cloneRequestOptions(opts)
	if len(extraHeaders) > 0 {
		opts.Headers = append(opts.Headers, extraHeaders...)
	}

	rawURL, apiName, opts, err := c.applyAPIProfile(rawURL, profileName, opts)
	if err != nil {
		return nil, err
	}
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return nil, err
	}

	// When following a cross-host redirect, strip credentials to prevent
	// a compromised plugin from issuing credentialed requests to arbitrary hosts.
	if noAuth {
		opts.OnRequest = nil
		filtered := opts.Headers[:0]
		for _, h := range opts.Headers {
			lower := strings.ToLower(h)
			if !strings.HasPrefix(lower, "authorization:") &&
				!strings.HasPrefix(lower, "cookie:") &&
				!strings.HasPrefix(lower, "proxy-authorization:") {
				filtered = append(filtered, h)
			}
		}
		opts.Headers = filtered
	}

	// Build the transport once so follow-up requests can reuse the same
	// connection pool via the returned opts value.
	opts.Transport = request.BuildTransport(opts)
	if closer, ok := opts.Transport.(io.Closer); ok {
		c.registerRequestCloser(closer)
	}

	// Chain request-middleware plugins after auth.
	origOnReq := opts.OnRequest
	opts.OnRequest = func(req *http.Request) error {
		if origOnReq != nil {
			if err := origOnReq(req); err != nil {
				return err
			}
		}
		return c.runRequestMiddlewarePlugins(req)
	}

	body, err := c.requestBodyReader(opts.ContentType, bodyValue, &opts.Headers)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	return &preparedRequest{
		rawURL:  rawURL,
		apiName: apiName,
		opts:    opts,
		body:    body,
	}, nil
}

func cloneRequestOptions(opts request.Options) request.Options {
	cloned := opts
	if len(opts.Headers) > 0 {
		cloned.Headers = append([]string(nil), opts.Headers...)
	}
	if len(opts.Query) > 0 {
		cloned.Query = append([]string(nil), opts.Query...)
	}
	if len(opts.TLSSignerParams) > 0 {
		cloned.TLSSignerParams = make(map[string]string, len(opts.TLSSignerParams))
		for k, v := range opts.TLSSignerParams {
			cloned.TLSSignerParams[k] = v
		}
	}
	return cloned
}

func (c *CLI) requestBodyReader(contentType string, bodyValue any, headers *[]string) (io.Reader, error) {
	if bodyValue == nil {
		return nil, nil
	}

	ct := contentType
	if ct == "" {
		ct = "application/json"
	}
	mimeType := c.content.MIMETypeForName(ct)
	if mimeType == "" {
		mimeType = ct
	}
	encoded, actualContentType, err := c.content.EncodeWithType(mimeType, bodyValue)
	if err != nil {
		return nil, err
	}
	*headers = append(*headers, "Content-Type: "+actualContentType)
	return bytes.NewReader(encoded), nil
}

func (c *CLI) sendPreparedRequest(ctx context.Context, method string, prepared *preparedRequest) (*http.Response, error) {
	return request.Do(ctx, method, prepared.rawURL, prepared.body, prepared.opts)
}

func (c *CLI) closePreparedTransport(prepared *preparedRequest) {
	if prepared == nil || prepared.opts.Transport == nil {
		return
	}
	if closer, ok := prepared.opts.Transport.(io.Closer); ok {
		_ = closer.Close()
	}
}

func (c *CLI) normalizeHTTPResponse(httpResp *http.Response, maxBodyBytes int64) (*output.Response, error) {
	resp, err := output.Normalize(httpResp, c.content, maxBodyBytes)
	if err != nil {
		return nil, err
	}

	// httpResp headers/request are still accessible after Normalize has closed
	// and consumed the body.
	if httpResp.Request != nil {
		if links := hypermedia.Parse(httpResp.Request.URL, httpResp.Header, resp.Body, c.linkParsers); len(links) > 0 {
			resp.Links = make(map[string]any, len(links))
			for k, v := range links {
				resp.Links[k] = v
			}
		}
	}

	return resp, nil
}
