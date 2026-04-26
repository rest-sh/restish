package cli

import (
	"io"
	"net/http"
)

type verboseTransport struct {
	inner   http.RoundTripper
	cli     *CLI
	verbose int
}

func (t *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if resp == nil {
		return resp, err
	}
	if resp.Request == nil {
		resp.Request = req
	}
	t.cli.logVerbose(resp, t.verbose)
	t.cli.logVerboseRequestBody(resp.Request)
	return resp, err
}

func (t *verboseTransport) Close() error {
	if closer, ok := t.inner.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (t *verboseTransport) CloseIdleConnections() {
	if closer, ok := t.inner.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (t *verboseTransport) Unwrap() http.RoundTripper {
	return t.inner
}

var _ http.RoundTripper = (*verboseTransport)(nil)
var _ io.Closer = (*verboseTransport)(nil)
