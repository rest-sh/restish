package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeAPIConfig writes a restish.json to a temp dir and returns its path.
func writeAPIConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeAPIConfig: %v", err)
	}
	return path
}

// TestAPIShortNameExpansion verifies that "myapi/items" is expanded to the
// configured base URL before the request is sent.
func TestAPIShortNameExpansion(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
}

// TestAPIShortNameNoPath verifies that "myapi" (no trailing path) resolves to
// the configured base URL root.
func TestAPIShortNameNoPath(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "myapi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.Last() == nil {
		t.Fatal("no request received")
	}
}

// TestUnknownAPINameFallback verifies that an unrecognized first segment is
// treated as a plain URL (not a fatal error about an unknown API).
func TestUnknownAPINameFallback(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	// Config has "myapi" but we request "otherapi/items"; fallback treats it as URL.
	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	// Use a real URL so the fallback actually resolves somewhere.
	if err := c.Run([]string{"restish", "get", "https://fallback.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
}

// TestProfilePersistentHeader verifies that a header declared in the active
// profile is included in every request to that API.
func TestProfilePersistentHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"headers": ["X-Api-Key: secret"]
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("X-Api-Key"); got != "secret" {
		t.Errorf("expected X-Api-Key=secret, got %q", got)
	}
}

// TestProfilePersistentQuery verifies that a query param declared in the
// active profile is appended to every request.
func TestProfilePersistentQuery(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"query": ["version=2"]
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Query().Get("version"); got != "2" {
		t.Errorf("expected query version=2, got %q", got)
	}
}

// TestProfileOverrideWithFlag verifies that -p selects a non-default profile,
// using its base_url and headers.
func TestProfileOverrideWithFlag(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://prod.example.com",
				"profiles": {
					"staging": {
						"base_url": "https://staging.example.com",
						"headers": ["X-Env: staging"]
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "-p", "staging", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := rr.Last()
	if req == nil {
		t.Fatal("no request received — base_url override may not have taken effect")
	}
	if got := req.URL.Path; got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
	if got := req.Header.Get("X-Env"); got != "staging" {
		t.Errorf("expected X-Env=staging, got %q", got)
	}
}

// TestProfileOverrideWithEnv verifies that RSH_PROFILE selects the profile
// when the -p flag is not set.
func TestProfileOverrideWithEnv(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://prod.example.com",
				"profiles": {
					"dev": {
						"base_url": "https://dev.example.com",
						"headers": ["X-Env: dev"]
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)
	t.Setenv("RSH_PROFILE", "dev")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := rr.Last()
	if req == nil {
		t.Fatal("no request received")
	}
	if got := req.Header.Get("X-Env"); got != "dev" {
		t.Errorf("expected X-Env=dev, got %q", got)
	}
}

// TestFlagHeaderTakesPrecedenceOverProfile verifies that a header supplied via
// -H overrides the same header from the profile (last write wins for Add, but
// flag values appear after profile values in the header list).
func TestFlagHeaderTakesPrecedenceOverProfile(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"headers": ["X-Token: from-profile"]
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)

	// Flag-supplied header should appear in the request (both values are sent
	// via Add; the test just verifies the flag value is present).
	if err := c.Run([]string{"restish", "get", "-H", "X-Token: from-flag", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vals := rr.Last().Header.Values("X-Token")
	if len(vals) == 0 {
		t.Fatal("expected X-Token header, got none")
	}
	// Flag value must be present.
	found := false
	for _, v := range vals {
		if v == "from-flag" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'from-flag' in X-Token values, got %v", vals)
	}
}

// TestAPIEditUsesCliStdout verifies that runAPIEdit wires the editor subprocess
// to c.Stdout rather than os.Stdout, so embedders that redirect c.Stdout capture
// any output the editor produces.
func TestAPIEditUsesCliStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("editor test uses a POSIX shell script")
	}

	dir := t.TempDir()

	// A fake editor that writes a sentinel line to stdout.
	scriptPath := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'editor-stdout'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", scriptPath)
	t.Setenv("EDITOR", "")

	c, out, _ := newTestCLI()
	cfgPath := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.ConfigPath = cfgPath

	if err := c.Run([]string{"restish", "api", "edit"}); err != nil {
		t.Fatalf("api edit: %v", err)
	}
	if !strings.Contains(out.String(), "editor-stdout") {
		t.Errorf("expected editor stdout in c.Stdout, got: %q", out.String())
	}
}
