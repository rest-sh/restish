package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

// CallHook spawns the plugin at path, writes in as a length-prefixed CBOR
// message to the plugin's stdin, reads one CBOR reply from stdout, and
// unmarshals it into out (which must be a pointer).
//
// The plugin must exit 0 for the call to succeed; a non-zero exit is returned
// as an error. The plugin has 30 seconds to respond before it is killed.
func CallHook(path string, in, out any) error {
	var stdin bytes.Buffer
	if err := pluginwire.WriteMessage(&stdin, in); err != nil {
		return fmt.Errorf("hook %s: encode: %w", filepath.Base(path), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = &stdin
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s: exec: %w", filepath.Base(path), err)
	}

	if err := pluginwire.ReadMessage(&stdout, out); err != nil {
		return fmt.Errorf("hook %s: decode: %w", filepath.Base(path), err)
	}
	return nil
}
