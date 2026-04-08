package plugin

import (
	"fmt"
	"io"
	"sync"
)

// CommandClient is the plugin-side counterpart of the host command-plugin
// runner.  It wraps the host stdin/stdout pair and provides helpers for
// making authenticated HTTP calls, writing output, and signalling completion.
//
// Most command plugins should create one with NewCommandClient and use it
// for all communication with the host.
type CommandClient struct {
	in      io.Reader
	out     io.Writer
	writeMu sync.Mutex
}

// NewCommandClient returns a CommandClient backed by the given reader/writer.
// Pass os.Stdin and os.Stdout from a plugin's main function.
func NewCommandClient(in io.Reader, out io.Writer) *CommandClient {
	return &CommandClient{in: in, out: out}
}

// Do sends one HTTP request to the host and blocks until the response
// arrives.  It is safe to call from a single goroutine; for concurrent use
// build an async wrapper on top of WriteMessage / ReadMessage instead.
func (c *CommandClient) Do(req *HTTPRequestMsg) (*HTTPResponseMsg, error) {
	if req.Type == "" {
		req.Type = MsgTypeHTTPRequest
	}
	if err := c.WriteMessage(req); err != nil {
		return nil, err
	}
	var reply HTTPResponseMsg
	if err := ReadMessage(c.in, &reply); err != nil {
		return nil, err
	}
	if reply.Type != MsgTypeHTTPResponse {
		return nil, fmt.Errorf("plugin: unexpected reply %q waiting for http-response", reply.Type)
	}
	return &reply, nil
}

// Stdout writes data to the user's terminal via the host.
func (c *CommandClient) Stdout(data []byte) error {
	return c.WriteMessage(StdoutDataMsg{Type: MsgTypeStdoutData, Data: append([]byte(nil), data...)})
}

// Stderr writes data to the user's terminal stderr via the host.
func (c *CommandClient) Stderr(data []byte) error {
	return c.WriteMessage(StderrDataMsg{Type: MsgTypeStderrData, Data: append([]byte(nil), data...)})
}

// Warn sends a warning message to the host, which displays it to the user
// through whatever UI mechanism it uses.
func (c *CommandClient) Warn(text string) error {
	return c.WriteMessage(WarnMsg{Type: MsgTypeWarn, Text: text})
}

// Done signals that the plugin has finished.  exitCode 0 means success; any
// other value causes the host to exit with that code.
func (c *CommandClient) Done(exitCode int) error {
	return c.WriteMessage(DoneMsg{Type: MsgTypeDone, ExitCode: exitCode})
}

// WriteMessage serialises v as a length-prefixed CBOR frame and sends it to
// the host.  It is safe to call from multiple goroutines.
func (c *CommandClient) WriteMessage(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteMessage(c.out, v)
}

// StdoutWriter returns an io.Writer that routes writes through Stdout.
func (c *CommandClient) StdoutWriter() io.Writer {
	return &clientStreamWriter{fn: c.Stdout}
}

// StderrWriter returns an io.Writer that routes writes through Stderr.
func (c *CommandClient) StderrWriter() io.Writer {
	return &clientStreamWriter{fn: c.Stderr}
}

type clientStreamWriter struct {
	fn func([]byte) error
}

func (w *clientStreamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := w.fn(p); err != nil {
		return 0, err
	}
	return len(p), nil
}
