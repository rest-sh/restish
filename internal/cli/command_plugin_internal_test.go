package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
)

func TestHandleCommandPluginMessageRejectsMalformedDone(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cli := &CLI{Stdout: &stdout, Stderr: &stderr}
	cmd := &cobra.Command{Use: "test"}
	cmd.SetErr(&stderr)

	raw, err := cbor.Marshal(map[string]any{
		"type":      "done",
		"exit_code": "boom",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	done, gotErr := cli.handleCommandPluginMessage(cmd, nil, nil, "done", raw)
	if !done {
		t.Fatal("expected malformed done message to stop processing")
	}
	if gotErr == nil {
		t.Fatal("expected malformed done message to return an error")
	}
	if !strings.Contains(gotErr.Error(), "decode done") {
		t.Fatalf("expected decode error, got: %v", gotErr)
	}
}

func TestLoadCommandPluginCommandsReturnsExecError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "restish-broken")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	_, err := loadCommandPluginCommands(path)
	if err == nil {
		t.Fatal("expected command discovery error")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("plugin %s: command discovery", filepath.Base(path))) {
		t.Fatalf("unexpected error: %v", err)
	}
}
