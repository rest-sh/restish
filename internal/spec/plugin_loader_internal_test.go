package spec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPluginLoaderUsesLoadOptionsContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "restish-loader-block")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("write loader: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loader := PluginLoader{
		PluginPath:   path,
		PluginName:   "block",
		ContentTypes: []string{"application/x-block"},
	}

	start := time.Now()
	_, err := loader.LoadWithOptions([]byte(`{}`), LoadOptions{Context: ctx, ContentType: "application/x-block"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("loader cancellation waited too long: %v", elapsed)
	}
}
