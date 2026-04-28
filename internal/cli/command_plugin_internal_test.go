package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/rest-sh/restish/v2/internal/config"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
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

func TestRunCommandPluginReturnsOnContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "restish-block")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cli := &CLI{Stdout: &stdout, Stderr: &stderr}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := &cobra.Command{Use: "block"}
	cmd.SetContext(ctx)
	cmd.SetErr(&stderr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- cli.runCommandPlugin(cmd, path, pluginwire.CommandDecl{Name: "block"}, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runCommandPlugin error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runCommandPlugin did not return after context cancellation")
	}
}

func TestValidatePluginCommandNameRejectsCollisions(t *testing.T) {
	c := &CLI{cfg: &config.Config{APIs: map[string]*config.APIConfig{"svc": {}}}}
	root := &cobra.Command{Use: "restish"}
	root.AddCommand(&cobra.Command{Use: "get"})

	cases := []struct {
		name string
	}{
		{name: "get"},
		{name: "svc"},
		{name: "Bad_Name"},
	}
	for _, tc := range cases {
		if err := c.validatePluginCommandName(root, map[string]string{}, "plugin", tc.name); err == nil {
			t.Fatalf("expected %q to be rejected", tc.name)
		}
	}

	seen := map[string]string{"tool": "one"}
	if err := c.validatePluginCommandName(root, seen, "two", "tool"); err == nil {
		t.Fatal("expected duplicate plugin command to be rejected")
	}
	if err := c.validatePluginCommandName(root, map[string]string{}, "plugin", "valid-tool"); err != nil {
		t.Fatalf("expected valid command name, got %v", err)
	}
}
