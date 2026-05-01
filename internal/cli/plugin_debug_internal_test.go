package cli

import (
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
