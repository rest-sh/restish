package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// HookTimeout returns the effective timeout for hookName from the manifest,
// using a default of 5 minutes for "auth" and 30 seconds for all other hooks.
func HookTimeout(m Manifest, hookName string) time.Duration {
	if d, ok := m.HookTimeouts[hookName]; ok && d > 0 {
		return d
	}
	if hookName == "auth" {
		return 5 * time.Minute
	}
	return 30 * time.Second
}

// callHookRaw spawns the plugin at path, writes in as a CBOR message to
// stdin, and returns all bytes written to stdout on success. Non-zero plugin
// exit is returned as an error together with any text on stderr.
func callHookRaw(path string, timeout time.Duration, in any) ([]byte, error) {
	var stdin bytes.Buffer
	if err := pluginwire.WriteMessage(&stdin, in); err != nil {
		return nil, fmt.Errorf("hook %s: encode: %w", filepath.Base(path), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = &stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("hook %s: exec: %w\n  plugin stderr: %s", filepath.Base(path), err, msg)
		}
		return nil, fmt.Errorf("hook %s: exec: %w", filepath.Base(path), err)
	}
	return stdout.Bytes(), nil
}

// FormatterStream is a long-lived formatter plugin process that receives
// sequential CBOR requests on stdin and writes formatted bytes directly to the
// provided stdout writer.
type FormatterStream struct {
	path   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr bytes.Buffer
}

// StartFormatterStream starts a formatter plugin subprocess, wires its stdout
// to w, sends the initial request, and returns a handle that can send
// additional stream messages before Close waits for plugin exit.
func StartFormatterStream(path string, w io.Writer, in any) (*FormatterStream, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("hook %s: stdin: %w", filepath.Base(path), err)
	}

	stream := &FormatterStream{
		path:  path,
		cmd:   cmd,
		stdin: stdin,
	}
	cmd.Stdout = w
	cmd.Stderr = &stream.stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("hook %s: start: %w", filepath.Base(path), err)
	}

	if err := pluginwire.WriteMessage(stdin, in); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil, fmt.Errorf("hook %s: encode: %w", filepath.Base(path), err)
	}

	return stream, nil
}

// Send writes one additional CBOR message to the formatter plugin.
func (s *FormatterStream) Send(in any) error {
	if err := pluginwire.WriteMessage(s.stdin, in); err != nil {
		return fmt.Errorf("hook %s: encode: %w", filepath.Base(s.path), err)
	}
	return nil
}

// formatterCloseTimeout is the grace period given to a formatter plugin to
// exit after its stdin is closed before it is killed.
const formatterCloseTimeout = 10 * time.Second

// Close closes plugin stdin and waits up to formatterCloseTimeout for the
// plugin to exit. If the plugin does not exit in time it is killed.
func (s *FormatterStream) Close() error {
	if s.stdin != nil {
		if err := s.stdin.Close(); err != nil {
			return fmt.Errorf("hook %s: close stdin: %w", filepath.Base(s.path), err)
		}
		s.stdin = nil
	}

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			if msg := strings.TrimSpace(s.stderr.String()); msg != "" {
				return fmt.Errorf("hook %s: exec: %w\n  plugin stderr: %s", filepath.Base(s.path), err, msg)
			}
			return fmt.Errorf("hook %s: exec: %w", filepath.Base(s.path), err)
		}
	case <-time.After(formatterCloseTimeout):
		_ = s.cmd.Process.Kill()
		<-done
		return fmt.Errorf("hook %s: plugin did not exit within %s; killed", filepath.Base(s.path), formatterCloseTimeout)
	}
	return nil
}

// CallHook spawns the plugin at path, writes in as a CBOR message to the
// plugin's stdin, reads one CBOR reply from stdout, and unmarshals it into out
// (which must be a pointer).
//
// The plugin must exit 0 for the call to succeed; a non-zero exit is returned
// as an error along with any text the plugin wrote to stderr. The plugin has
// 30 seconds to respond before it is killed. Use CallHookWithTimeout to
// override the deadline.
func CallHook(path string, in, out any) error {
	return CallHookWithTimeout(path, 30*time.Second, in, out)
}

// CallHookWithTimeout is like CallHook but uses the supplied deadline.
func CallHookWithTimeout(path string, timeout time.Duration, in, out any) error {
	data, err := callHookRaw(path, timeout, in)
	if err != nil {
		return err
	}
	if err := pluginwire.ReadMessage(bytes.NewReader(data), out); err != nil {
		return fmt.Errorf("hook %s: decode: %w", filepath.Base(path), err)
	}
	return nil
}
