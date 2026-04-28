package plugin

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
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
// stdin-data or stdin-close frames that arrive after Do starts the background
// response reader.
type CommandClient struct {
	dec     *Decoder
	out     io.Writer
	writeMu sync.Mutex

	readOnce sync.Once
	readErr  atomic.Value
	nextID   atomic.Uint64
	pending  sync.Map

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
// A single Decoder is created from in and reused for all reads. Call
// ReadMessage for startup messages before calling Do; after Do starts the
// background response reader, all host replies are routed through Do.
func NewCommandClient(in io.Reader, out io.Writer) *CommandClient {
	return &CommandClient{dec: NewDecoder(in), out: out}
}

// ReadMessage reads one CBOR message from the host into v. Use this when the
// plugin needs to receive startup messages that are not covered by Do.
func (c *CommandClient) ReadMessage(v any) error {
	return c.dec.ReadMessage(v)
}

// Do sends one HTTP request to the host and blocks until the matching
// http-response arrives. While waiting, any stdin-data or stdin-close frames
// are dispatched to StdinDataHandler / StdinCloseHandler if registered.
//
// Do is safe to call concurrently. Requests without RequestID are assigned one
// automatically so replies can be routed back to the caller.
func (c *CommandClient) Do(req *HTTPRequestMsg) (*HTTPResponseMsg, error) {
	if req.Type == "" {
		req.Type = MsgTypeHTTPRequest
	}
	if req.RequestID == "" {
		req.RequestID = "req-" + strconv.FormatUint(c.nextID.Add(1), 10)
	}
	replyCh := make(chan commandReply, 1)
	if _, loaded := c.pending.LoadOrStore(req.RequestID, replyCh); loaded {
		return nil, fmt.Errorf("plugin: duplicate request_id %q", req.RequestID)
	}
	defer c.pending.Delete(req.RequestID)

	c.startReadLoop()
	if errValue := c.readErr.Load(); errValue != nil {
		return nil, errValue.(error)
	}
	if err := c.WriteMessage(req); err != nil {
		return nil, err
	}
	reply := <-replyCh
	if reply.err != nil {
		return nil, reply.err
	}
	return &reply.resp, nil
}

type commandReply struct {
	resp HTTPResponseMsg
	err  error
}

func (c *CommandClient) startReadLoop() {
	c.readOnce.Do(func() {
		go c.readLoop()
	})
}

func (c *CommandClient) readLoop() {
	for {
		raw, err := c.dec.ReadRaw()
		if err != nil {
			c.failPending(err)
			return
		}
		msgType := MessageType(raw)
		switch msgType {
		case MsgTypeHTTPResponse:
			var reply HTTPResponseMsg
			if err := DecMode.Unmarshal(raw, &reply); err != nil {
				c.failPending(fmt.Errorf("plugin: decode http-response: %w", err))
				return
			}
			c.deliverHTTPResponse(reply)
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
			continue
		}
	}
}

func (c *CommandClient) deliverHTTPResponse(reply HTTPResponseMsg) {
	if reply.RequestID != "" {
		if ch, ok := c.pending.Load(reply.RequestID); ok {
			ch.(chan commandReply) <- commandReply{resp: reply}
		}
		return
	}

	var only chan commandReply
	count := 0
	c.pending.Range(func(_, value any) bool {
		only = value.(chan commandReply)
		count++
		return count < 2
	})
	if count == 1 {
		only <- commandReply{resp: reply}
	}
}

func (c *CommandClient) failPending(err error) {
	c.readErr.Store(err)
	c.pending.Range(func(_, value any) bool {
		value.(chan commandReply) <- commandReply{err: err}
		return true
	})
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
