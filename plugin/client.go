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
//
// Plugins that declare passthrough_stdio should register StdinDataHandler and
// StdinCloseHandler before calling Do(). These handlers are invoked for any
// stdin-data or stdin-close frames that arrive while Do() is waiting for an
// http-response reply.
type CommandClient struct {
	dec     *Decoder
	out     io.Writer
	writeMu sync.Mutex

	// StdinDataHandler is called with each chunk of stdin data received from
	// the host while Do() is waiting for an http-response. Set this before
	// calling Do() when passthrough_stdio is active.
	StdinDataHandler func(data []byte)

	// StdinCloseHandler is called when the host signals that stdin has reached
	// EOF while Do() is waiting for an http-response.
	StdinCloseHandler func()
}

// NewCommandClient returns a CommandClient backed by the given reader/writer.
// Pass os.Stdin and os.Stdout from a plugin's main function.
// A single Decoder is created from in and reused for all reads, so it is safe
// to call ReadMessage on the client after NewCommandClient returns.
func NewCommandClient(in io.Reader, out io.Writer) *CommandClient {
	return &CommandClient{dec: NewDecoder(in), out: out}
}

// ReadMessage reads one CBOR message from the host into v. Use this when the
// plugin needs to receive messages that are not covered by Do (for example,
// stdin-data frames in passthrough-stdio commands).
func (c *CommandClient) ReadMessage(v any) error {
	return c.dec.ReadMessage(v)
}

// Do sends one HTTP request to the host and blocks until the http-response
// arrives. While waiting, any stdin-data or stdin-close frames are dispatched
// to StdinDataHandler / StdinCloseHandler if registered — this is required
// for plugins that declare passthrough_stdio, where the host interleaves
// stdin frames with the HTTP response.
//
// Do is safe to call from a single goroutine. For concurrent use, build an
// async wrapper on top of WriteMessage / ReadMessage instead.
func (c *CommandClient) Do(req *HTTPRequestMsg) (*HTTPResponseMsg, error) {
	if req.Type == "" {
		req.Type = MsgTypeHTTPRequest
	}
	if err := c.WriteMessage(req); err != nil {
		return nil, err
	}
	for {
		raw, err := c.dec.ReadRaw()
		if err != nil {
			return nil, err
		}
		msgType := MessageType(raw)
		switch msgType {
		case MsgTypeHTTPResponse:
			var reply HTTPResponseMsg
			if err := DecMode.Unmarshal(raw, &reply); err != nil {
				return nil, fmt.Errorf("plugin: decode http-response: %w", err)
			}
			return &reply, nil
		case MsgTypeStdinData:
			if c.StdinDataHandler != nil {
				var msg StdinDataMsg
				_ = DecMode.Unmarshal(raw, &msg)
				c.StdinDataHandler(msg.Data)
			}
		case MsgTypeStdinClose:
			if c.StdinCloseHandler != nil {
				c.StdinCloseHandler()
			}
		default:
			return nil, fmt.Errorf("plugin: unexpected message %q waiting for http-response", msgType)
		}
	}
}

// WriteStdout writes data to the user's terminal via the host.
func (c *CommandClient) WriteStdout(data []byte) error {
	return c.WriteMessage(StdoutDataMsg{Type: MsgTypeStdoutData, Data: append([]byte(nil), data...)})
}

// WriteStderr writes data to the user's terminal stderr via the host.
func (c *CommandClient) WriteStderr(data []byte) error {
	return c.WriteMessage(StderrDataMsg{Type: MsgTypeStderrData, Data: append([]byte(nil), data...)})
}

// Warn sends a warning message to the host, which displays it to the user
// through whatever UI mechanism it uses.
func (c *CommandClient) Warn(text string) error {
	return c.WriteMessage(WarnMsg{Type: MsgTypeWarn, Text: text})
}

// Progress sends an informational progress message to the host.
func (c *CommandClient) Progress(text string) error {
	return c.WriteMessage(ProgressMsg{Type: MsgTypeProgress, Text: text})
}

// Done signals that the plugin has finished.  exitCode 0 means success; any
// other value causes the host to exit with that code.
func (c *CommandClient) Done(exitCode int) error {
	return c.WriteMessage(DoneMsg{Type: MsgTypeDone, ExitCode: exitCode})
}

// WriteMessage serialises v as a CBOR data item and sends it to the host.
// It is safe to call from multiple goroutines.
func (c *CommandClient) WriteMessage(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteMessage(c.out, v)
}

// StdoutWriter returns an io.Writer that routes writes through WriteStdout.
func (c *CommandClient) StdoutWriter() io.Writer {
	return &clientStreamWriter{fn: c.WriteStdout}
}

// StderrWriter returns an io.Writer that routes writes through WriteStderr.
func (c *CommandClient) StderrWriter() io.Writer {
	return &clientStreamWriter{fn: c.WriteStderr}
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
