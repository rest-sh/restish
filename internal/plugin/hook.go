package plugin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

// callHookRaw spawns the plugin at path, writes in as a CBOR message to
// stdin, and returns all bytes written to stdout on success. Non-zero plugin
// exit is returned as an error together with any text on stderr.
func callHookRaw(path string, in any) ([]byte, error) {
	var stdin bytes.Buffer
	if err := pluginwire.WriteMessage(&stdin, in); err != nil {
		return nil, fmt.Errorf("hook %s: encode: %w", filepath.Base(path), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// CallFormatterHook spawns the plugin at path, writes in as a CBOR message to
// stdin, and returns all bytes written to stdout as raw (unframed) output.
// This is used for formatter plugins whose output is the formatted data itself,
// not a CBOR envelope.
func CallFormatterHook(path string, in any) ([]byte, error) {
	return callHookRaw(path, in)
}

// CallHook spawns the plugin at path, writes in as a length-prefixed CBOR
// message to the plugin's stdin, reads one CBOR reply from stdout, and
// unmarshals it into out (which must be a pointer).
//
// The plugin must exit 0 for the call to succeed; a non-zero exit is returned
// as an error along with any text the plugin wrote to stderr. The plugin has
// 30 seconds to respond before it is killed.
func CallHook(path string, in, out any) error {
	data, err := callHookRaw(path, in)
	if err != nil {
		return err
	}
	if err := pluginwire.ReadMessage(bytes.NewReader(data), out); err != nil {
		return fmt.Errorf("hook %s: decode: %w", filepath.Base(path), err)
	}
	return nil
}
