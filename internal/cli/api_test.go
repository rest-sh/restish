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
	if configDir := os.Getenv("RSH_CONFIG_DIR"); configDir != "" {
		dir = configDir
	}
	path := filepath.Join(dir, "restish.json")
	writeTestFile(t, path, content)
	return path
}

func newAPIRecorderApp(t *testing.T, cfg string) (*testApp, *requestRecorder) {
	t.Helper()
	var rr requestRecorder
	app := newTestApp(t)
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	app.SetConfigPath(writeAPIConfig(t, cfg))
	return app, &rr
}

// TestAPIShortNameExpansion verifies that "myapi/items" is expanded to the
// configured base URL before the request is sent.
func TestAPIShortNameExpansion(t *testing.T) {
	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "myapi/items")
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
}

// TestAPIShortNameNoPath verifies that "myapi" (no trailing path) resolves to
// the configured base URL root.
func TestAPIShortNameNoPath(t *testing.T) {
	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("myapi")
	if rr.Last() == nil {
		t.Fatal("no request received")
	}
}

// TestUnknownAPINameFallback verifies that an unrecognized first segment is
// treated as a plain URL (not a fatal error about an unknown API).
func TestUnknownAPINameFallback(t *testing.T) {
	// Config has "myapi" but we request "otherapi/items"; fallback treats it as URL.
	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`
	app, rr := newAPIRecorderApp(t, cfg)

	// Use a real URL so the fallback actually resolves somewhere.
	app.Run("get", "https://fallback.example.com/items")
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
}

// TestProfilePersistentHeader verifies that a header declared in the active
// profile is included in every request to that API.
func TestProfilePersistentHeader(t *testing.T) {
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
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "myapi/items")
	if got := rr.Last().Header.Get("X-Api-Key"); got != "secret" {
		t.Errorf("expected X-Api-Key=secret, got %q", got)
	}
}

func TestProfilePersistentHostHeader(t *testing.T) {
	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"headers": ["Host: tenant.example.com"]
					}
				}
			}
		}
	}`
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "myapi/items")
	if got := rr.Last().Host; got != "tenant.example.com" {
		t.Errorf("expected Host tenant.example.com, got %q", got)
	}
}

// TestProfilePersistentQuery verifies that a query param declared in the
// active profile is appended to every request.
func TestProfilePersistentQuery(t *testing.T) {
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
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "myapi/items")
	if got := rr.Last().URL.Query().Get("version"); got != "2" {
		t.Errorf("expected query version=2, got %q", got)
	}
}

// TestProfileOverrideWithFlag verifies that -p selects a non-default profile,
// using its base_url and headers.
func TestProfileOverrideWithFlag(t *testing.T) {
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
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "-p", "staging", "myapi/items")
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
	app, rr := newAPIRecorderApp(t, cfg)
	t.Setenv("RSH_PROFILE", "dev")

	app.Run("get", "myapi/items")
	req := rr.Last()
	if req == nil {
		t.Fatal("no request received")
	}
	if got := req.Header.Get("X-Env"); got != "dev" {
		t.Errorf("expected X-Env=dev, got %q", got)
	}
}

// TestFlagHeaderTakesPrecedenceOverProfile verifies that a header supplied via
// -H replaces the same header from the profile.
func TestFlagHeaderTakesPrecedenceOverProfile(t *testing.T) {
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
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "-H", "X-Token: from-flag", "myapi/items")
	vals := rr.Last().Header.Values("X-Token")
	if len(vals) != 1 || vals[0] != "from-flag" {
		t.Fatalf("X-Token values = %#v, want [from-flag]", vals)
	}
}

// TestConfigEditUsesCliStdout verifies that runConfigEdit wires the editor
// subprocess to c.Stdout rather than os.Stdout, so embedders that redirect
// c.Stdout capture any output the editor produces.
func TestConfigEditUsesCliStdout(t *testing.T) {
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

	c, out, _ := newTestCLI(t)
	cfgPath := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Hooks().ConfigPath = cfgPath

	if err := c.Run([]string{"restish", "config", "edit"}); err != nil {
		t.Fatalf("config edit: %v", err)
	}
	if !strings.Contains(out.String(), "editor-stdout") {
		t.Errorf("expected editor stdout in c.Stdout, got: %q", out.String())
	}
	if strings.Contains(out.String(), "Wrote config:") {
		t.Errorf("expected unchanged config edit not to print written path, got: %q", out.String())
	}
}

func TestConfigEditPrintsWrittenPathWhenFileChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("editor test uses a POSIX shell script")
	}

	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nprintf '{\"theme\":{\"ok\":\"green\"}}\\n' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", scriptPath)
	t.Setenv("EDITOR", "")

	c, out, _ := newTestCLI(t)
	cfgPath := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Hooks().ConfigPath = cfgPath

	if err := c.Run([]string{"restish", "config", "edit"}); err != nil {
		t.Fatalf("config edit: %v", err)
	}
	if !strings.Contains(out.String(), "Wrote config: "+cfgPath) {
		t.Errorf("expected written config path in c.Stdout, got: %q", out.String())
	}
}

func TestAPIEditInvalidatesChangedAPISpecCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("editor test uses a POSIX shell script")
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(cfgPath, []byte(`{
  "apis": {
    "changed": {"base_url": "https://old.example.com"},
    "same": {"base_url": "https://same.example.com"}
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	changedCache := filepath.Join(cacheDir, "changed.cbor")
	sameCache := filepath.Join(cacheDir, "same.cbor")
	for _, path := range []string{changedCache, sameCache} {
		if err := os.WriteFile(path, []byte("cached"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	scriptPath := filepath.Join(dir, "editor.sh")
	script := `#!/bin/sh
cat > "$1" <<'JSON'
{
  "apis": {
    "changed": {"base_url": "https://new.example.com"},
    "same": {"base_url": "https://same.example.com"}
  }
}
JSON
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", scriptPath)
	t.Setenv("EDITOR", "")

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgPath
	c.Hooks().SpecCachePath = cacheDir

	if err := c.Run([]string{"restish", "config", "edit"}); err != nil {
		t.Fatalf("config edit: %v", err)
	}
	if _, err := os.Stat(changedCache); !os.IsNotExist(err) {
		t.Fatalf("expected changed API cache to be invalidated, stat err=%v", err)
	}
	if _, err := os.Stat(sameCache); err != nil {
		t.Fatalf("expected unchanged API cache to remain, stat err=%v", err)
	}
}
