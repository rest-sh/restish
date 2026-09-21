package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rest-sh/restish/v2/internal/procutil"
	"github.com/rest-sh/restish/v2/internal/secrets"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
	"golang.org/x/term"
)

const (
	maxHookOutputBytes = 16 << 20
	maxHookStderrBytes = 64 << 10
)

var (
	formatterIsTerminal = term.IsTerminal
	formatterMakeRaw    = term.MakeRaw
	formatterRestore    = term.Restore
	formatterGetSize    = term.GetSize
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
func callHookRaw(ctx context.Context, path string, timeout time.Duration, in any) ([]byte, error) {
	var stdin bytes.Buffer
	if err := pluginwire.WriteMessage(&stdin, in); err != nil {
		return nil, fmt.Errorf("hook %s: encode: %w", filepath.Base(path), err)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout := &limitedBuffer{limit: maxHookOutputBytes + 1}
	stderr := &limitedBuffer{limit: maxHookStderrBytes}
	cmd := exec.CommandContext(ctx, path)
	procutil.ConfigureCommandTreeKill(ctx, cmd)
	cmd.Stdin = &stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return nil, fmt.Errorf("hook %s: exec: %w\n  plugin stderr: %s", filepath.Base(path), ctxErr, secrets.RedactDiagnosticText(msg))
			}
			return nil, fmt.Errorf("hook %s: exec: %w", filepath.Base(path), ctxErr)
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("hook %s: exec: %w\n  plugin stderr: %s", filepath.Base(path), err, secrets.RedactDiagnosticText(msg))
		}
		return nil, fmt.Errorf("hook %s: exec: %w", filepath.Base(path), err)
	}
	if stdout.Len() > maxHookOutputBytes || stdout.Truncated() {
		return nil, fmt.Errorf("hook %s: output exceeded %d bytes", filepath.Base(path), maxHookOutputBytes)
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
	stderr limitedBuffer
	mu     sync.Mutex
}

// StartFormatterStream starts a formatter plugin subprocess, wires its stdout
// to w, sends the initial request, and returns a handle that can send
// additional stream messages before Close waits for plugin exit.
func StartFormatterStream(ctx context.Context, path string, w io.Writer, in any) (*FormatterStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, path)
	procutil.ConfigureCommandTreeKill(ctx, cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("hook %s: stdin: %w", filepath.Base(path), err)
	}

	stream := &FormatterStream{
		path:  path,
		cmd:   cmd,
		stdin: stdin,
		stderr: limitedBuffer{
			limit: maxHookStderrBytes,
		},
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin == nil {
		return fmt.Errorf("hook %s: stdin is closed", filepath.Base(s.path))
	}
	if err := pluginwire.WriteMessage(s.stdin, in); err != nil {
		return fmt.Errorf("hook %s: encode: %w", filepath.Base(s.path), err)
	}
	return nil
}

type interactiveTerminalInput interface {
	io.Reader
	Fd() uintptr
}

type terminalOutput interface {
	Fd() uintptr
}

// Interact keeps a formatter alive after its final response event and forwards
// terminal input and size changes until the plugin exits.
func (s *FormatterStream) Interact(input io.Reader, output io.Writer) (result error) {
	in, ok := input.(interactiveTerminalInput)
	if !ok || !formatterIsTerminal(int(in.Fd())) {
		return errors.Join(fmt.Errorf("interactive formatter requires terminal stdin"), s.Close())
	}
	out, ok := output.(terminalOutput)
	if !ok || !formatterIsTerminal(int(out.Fd())) {
		return errors.Join(fmt.Errorf("interactive formatter requires terminal stdout"), s.Close())
	}

	state, err := formatterMakeRaw(int(in.Fd()))
	if err != nil {
		return errors.Join(fmt.Errorf("interactive formatter: raw terminal: %w", err), s.Close())
	}
	defer func() {
		result = errors.Join(result, formatterRestore(int(in.Fd()), state))
	}()

	columns, rows, err := formatterGetSize(int(out.Fd()))
	if err != nil {
		return errors.Join(fmt.Errorf("interactive formatter: terminal size: %w", err), s.Close())
	}
	if err := s.Send(pluginwire.TerminalResizeMsg{Type: pluginwire.MsgTypeTerminalResize, Columns: columns, Rows: rows}); err != nil {
		return errors.Join(err, s.Close())
	}

	done := make(chan struct{})
	inputErr := make(chan error, 1)
	var inputWG sync.WaitGroup
	inputWG.Add(1)
	go func() {
		defer inputWG.Done()
		inputErr <- s.forwardTerminalInput(in, done)
	}()
	wait := make(chan error, 1)
	go func() { wait <- s.cmd.Wait() }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var waitErr error
	for waitErr == nil {
		select {
		case waitErr = <-wait:
			if waitErr == nil {
				waitErr = io.EOF
			}
		case err := <-inputErr:
			if err != nil {
				_ = s.cmd.Process.Kill()
				waitErr = errors.Join(err, <-wait)
			}
			inputErr = nil
		case <-ticker.C:
			newColumns, newRows, sizeErr := formatterGetSize(int(out.Fd()))
			if sizeErr != nil {
				_ = s.cmd.Process.Kill()
				waitErr = errors.Join(sizeErr, <-wait)
				continue
			}
			if newColumns != columns || newRows != rows {
				columns, rows = newColumns, newRows
				if sendErr := s.Send(pluginwire.TerminalResizeMsg{Type: pluginwire.MsgTypeTerminalResize, Columns: columns, Rows: rows}); sendErr != nil {
					_ = s.cmd.Process.Kill()
					waitErr = errors.Join(sendErr, <-wait)
				}
			}
		}
	}
	close(done)
	inputWG.Wait()
	closeErr := s.closeStdin()
	if errors.Is(waitErr, io.EOF) {
		waitErr = nil
	}
	return errors.Join(s.processError(waitErr), closeErr)
}

func (s *FormatterStream) forwardTerminalInput(in interactiveTerminalInput, done <-chan struct{}) error {
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return nil
		default:
		}
		ready, err := terminalInputReady(in.Fd(), 100*time.Millisecond)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		n, err := in.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			if sendErr := s.Send(pluginwire.StdinDataMsg{Type: pluginwire.MsgTypeStdinData, Data: data}); sendErr != nil {
				return sendErr
			}
		}
		if errors.Is(err, io.EOF) {
			return s.Send(pluginwire.StdinCloseMsg{Type: pluginwire.MsgTypeStdinClose})
		}
		if err != nil {
			return err
		}
	}
}

func (s *FormatterStream) closeStdin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin == nil {
		return nil
	}
	err := s.stdin.Close()
	s.stdin = nil
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func (s *FormatterStream) processError(err error) error {
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(s.stderr.String()); msg != "" {
		return fmt.Errorf("hook %s: exec: %w\n  plugin stderr: %s", filepath.Base(s.path), err, secrets.RedactDiagnosticText(msg))
	}
	return fmt.Errorf("hook %s: exec: %w", filepath.Base(s.path), err)
}

// formatterCloseTimeout is the grace period given to a formatter plugin to
// exit after its stdin is closed before it is killed.
const formatterCloseTimeout = 10 * time.Second

// Close closes plugin stdin and waits up to formatterCloseTimeout for the
// plugin to exit. If the plugin does not exit in time it is killed.
func (s *FormatterStream) Close() error {
	if err := s.closeStdin(); err != nil {
		return fmt.Errorf("hook %s: close stdin: %w", filepath.Base(s.path), err)
	}

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		return s.processError(err)
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
	return CallHookContext(context.Background(), path, in, out)
}

// CallHookContext is like CallHook but derives the plugin deadline from ctx.
func CallHookContext(ctx context.Context, path string, in, out any) error {
	return CallHookWithTimeoutContext(ctx, path, 30*time.Second, in, out)
}

// CallHookWithTimeout is like CallHook but uses the supplied deadline.
func CallHookWithTimeout(path string, timeout time.Duration, in, out any) error {
	return CallHookWithTimeoutContext(context.Background(), path, timeout, in, out)
}

// CallHookWithTimeoutContext is like CallHookWithTimeout but derives the plugin
// deadline from ctx.
func CallHookWithTimeoutContext(ctx context.Context, path string, timeout time.Duration, in, out any) error {
	data, err := callHookRaw(ctx, path, timeout, in)
	if err != nil {
		return err
	}
	if err := pluginwire.ReadMessage(bytes.NewReader(data), out); err != nil {
		return fmt.Errorf("hook %s: decode: %w", filepath.Base(path), err)
	}
	return nil
}
