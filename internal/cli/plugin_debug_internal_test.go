package cli

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func TestDecodePluginDebugStreamEmitsBeforeEOF(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	done := make(chan error, 1)
	out := &lockedStringWriter{}
	go func() {
		_, err := decodePluginDebugStream(pr, out)
		done <- err
	}()

	if err := pluginwire.WriteMessage(pw, pluginwire.ProgressMsg{Type: pluginwire.MsgTypeProgress, Text: "starting"}); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	deadline := time.After(time.Second)
	for !strings.Contains(out.String(), `"text": "starting"`) {
		select {
		case err := <-done:
			t.Fatalf("decoder exited before EOF: %v", err)
		case <-deadline:
			t.Fatalf("decoded output did not arrive before EOF: %q", out.String())
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if err := pw.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("decodePluginDebugStream: %v", err)
	}
}

func TestDecodePluginDebugStreamReturnsMalformedCBORAfterDrain(t *testing.T) {
	var input strings.Builder
	var valid strings.Builder
	if err := pluginwire.WriteMessage(&valid, pluginwire.ProgressMsg{Type: pluginwire.MsgTypeProgress, Text: "starting"}); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	input.WriteString(valid.String())
	input.WriteString("\xff\xffnot-cbor")

	out := &lockedStringWriter{}
	_, err := decodePluginDebugStream(strings.NewReader(input.String()), out)
	if err == nil {
		t.Fatal("expected malformed CBOR error")
	}
	if !strings.Contains(err.Error(), "decode stdout") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"text": "starting"`) {
		t.Fatalf("expected valid message before decode error, got %q", out.String())
	}
}

func TestDecodePluginDebugStreamTreatsClosedFileAfterMessageAsEOF(t *testing.T) {
	var input strings.Builder
	if err := pluginwire.WriteMessage(&input, pluginwire.ProgressMsg{Type: pluginwire.MsgTypeProgress, Text: "starting"}); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	out := &lockedStringWriter{}
	_, err := decodePluginDebugStream(&errAfterStringReader{
		r:   strings.NewReader(input.String()),
		err: errors.New("read |0: file already closed"),
	}, out)
	if err != nil {
		t.Fatalf("decodePluginDebugStream: %v", err)
	}
	if !strings.Contains(out.String(), `"text": "starting"`) {
		t.Fatalf("expected valid message before closed-file EOF, got %q", out.String())
	}
}

type errAfterStringReader struct {
	r   *strings.Reader
	err error
}

func (r *errAfterStringReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err == io.EOF {
		return 0, r.err
	}
	return n, err
}

type lockedStringWriter struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *lockedStringWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lockedStringWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}
