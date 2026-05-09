package cli_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestThemeSetFromURL(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, []byte(`{
  // keep me
  "cache": {"max_size": "10MB"}
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.String(), "https://themes.example.com/theme.json"; got != want {
			t.Fatalf("URL = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"#ffffff","status_2xx":"bold #00ff00"}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "https://themes.example.com/theme.json", "--yes"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}
	if !strings.Contains(out.String(), "Theme URL: https://themes.example.com/theme.json") {
		t.Fatalf("expected resolved theme URL, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Set theme from https://themes.example.com/theme.json") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "// keep me") {
		t.Fatalf("config comments were not preserved:\n%s", string(data))
	}
	for _, want := range []string{
		"\"theme_source\": \"https://themes.example.com/theme.json\"",
		"\"theme\": {",
		"\n    \"key\": \"#ffffff\"",
		"\n    \"status_2xx\": \"bold #00ff00\"",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("expected nicely formatted theme config containing %q:\n%s", want, string(data))
		}
	}
	if strings.Contains(string(data), `"theme": {"`) {
		t.Fatalf("theme config was written compactly:\n%s", string(data))
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#ffffff" {
		t.Fatalf("theme key = %q, want #ffffff", cfg.Theme["key"])
	}
	if cfg.ThemeSource != "https://themes.example.com/theme.json" {
		t.Fatalf("theme source = %q, want URL", cfg.ThemeSource)
	}
}

func TestThemeSetGithubShorthand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.String(), "https://raw.githubusercontent.com/example/themes/HEAD/theme.json"; got != want {
			t.Fatalf("URL = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"#ffffff"}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "example/themes", "--yes"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}

	cfg, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#ffffff" {
		t.Fatalf("theme key = %q, want #ffffff", cfg.Theme["key"])
	}
}

func TestThemeSetGithubShorthandNamedTheme(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.String(), "https://raw.githubusercontent.com/example/themes/HEAD/themes/dark.json"; got != want {
			t.Fatalf("URL = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"#111111"}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "example/themes", "dark", "--yes"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}

	cfg, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#111111" {
		t.Fatalf("theme key = %q, want #111111", cfg.Theme["key"])
	}
}

func TestThemeSetOfficialThemeName(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.String(), "https://raw.githubusercontent.com/rest-sh/restish/HEAD/themes/dracula.json"; got != want {
			t.Fatalf("URL = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"#282a36"}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "dracula", "--yes"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}

	cfg, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#282a36" {
		t.Fatalf("theme key = %q, want #282a36", cfg.Theme["key"])
	}
}

func TestThemeList(t *testing.T) {
	c, out, _ := newTestCLI(t)

	if err := c.Run([]string{"restish", "config", "theme", "list"}); err != nil {
		t.Fatalf("config theme list: %v", err)
	}

	got := out.String()
	for _, want := range []string{"catppuccin-mocha", "dracula", "vscode-dark"} {
		if !strings.Contains(got, want) {
			t.Fatalf("theme list missing %q:\n%s", want, got)
		}
	}
}

func TestThemeSetGithubShorthandNamedThemeFallsBackToRoot(t *testing.T) {
	c, _, _ := newTestCLI(t)
	var requests []string
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String())
		switch r.URL.String() {
		case "https://raw.githubusercontent.com/example/themes/HEAD/themes/dark.json":
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		case "https://raw.githubusercontent.com/example/themes/HEAD/dark.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"key":"#222222"}`)),
				Request:    r,
			}, nil
		default:
			t.Fatalf("unexpected URL: %s", r.URL.String())
			return nil, nil
		}
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "example/themes", "dark", "--yes"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}

	wantRequests := []string{
		"https://raw.githubusercontent.com/example/themes/HEAD/themes/dark.json",
		"https://raw.githubusercontent.com/example/themes/HEAD/dark.json",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
	cfg, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#222222" {
		t.Fatalf("theme key = %q, want #222222", cfg.Theme["key"])
	}
}

func TestThemeSetFromLocalPath(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "restish.json")
	themeDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themeDir, 0o700); err != nil {
		t.Fatalf("make theme dir: %v", err)
	}
	themeFile := filepath.Join(themeDir, "dark.jsonc")
	if err := os.WriteFile(themeFile, []byte(`{
		// Theme authors can explain palette choices.
		"text": "#eeeeee",
		"key": "#abb2bf",
		"status_2xx": "bold #98c379"
	}`), 0o600); err != nil {
		t.Fatalf("write theme file: %v", err)
	}
	if err := os.WriteFile(cfgFile, []byte(`{"cache":{"max_size":"10MB"}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected theme HTTP fetch: %s", r.URL.String())
		return nil, nil
	})

	wantSource, err := filepath.Abs("themes/dark.jsonc")
	if err != nil {
		t.Fatalf("resolve theme path: %v", err)
	}
	if err := c.Run([]string{"restish", "config", "theme", "set", "themes/dark.jsonc"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}
	if !strings.Contains(out.String(), "Theme path: "+wantSource) {
		t.Fatalf("expected resolved theme path, got: %q", out.String())
	}
	if strings.Contains(errOut.String(), "Install theme") {
		t.Fatalf("unexpected confirmation prompt: %q", errOut.String())
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#abb2bf" {
		t.Fatalf("theme key = %q, want #abb2bf", cfg.Theme["key"])
	}
	if cfg.Theme["text"] != "#eeeeee" {
		t.Fatalf("theme text = %q, want #eeeeee", cfg.Theme["text"])
	}
	if cfg.ThemeSource != wantSource {
		t.Fatalf("theme source = %q, want %q", cfg.ThemeSource, wantSource)
	}
}

func TestThemeSetRejectsNameForURL(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "config", "theme", "set", "https://themes.example.com/theme.json", "dark"})
	if err == nil {
		t.Fatal("expected URL with theme name to fail")
	}
	if !strings.Contains(err.Error(), "only supported with GitHub") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestThemeResetRemovesThemeConfig(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, []byte(`{
  // keep me
  "theme_source": "https://themes.example.com/theme.json",
  "theme": {
    "key": "#ffffff",
    "status_2xx": "bold #00ff00"
  },
  // keep me
  "cache": {"max_size": "10MB"}
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "config", "theme", "reset"}); err != nil {
		t.Fatalf("config theme reset: %v", err)
	}
	if !strings.Contains(out.String(), "Reset theme to built-in default") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// keep me") {
		t.Fatalf("config comments were not preserved:\n%s", got)
	}
	if strings.Contains(got, "theme_source") || strings.Contains(got, `"theme"`) {
		t.Fatalf("expected theme keys to be removed:\n%s", got)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ThemeSource != "" {
		t.Fatalf("theme source = %q, want empty", cfg.ThemeSource)
	}
	if len(cfg.Theme) != 0 {
		t.Fatalf("theme = %#v, want empty", cfg.Theme)
	}
}

func TestThemeUnsetAliasResetsTheme(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, []byte(`{"theme_source":"local","theme":{"key":"#ffffff"}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "config", "theme", "unset"}); err != nil {
		t.Fatalf("config theme unset: %v", err)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ThemeSource != "" || len(cfg.Theme) != 0 {
		t.Fatalf("theme was not reset: source=%q theme=%#v", cfg.ThemeSource, cfg.Theme)
	}
}

func TestThemeSetPromptsBeforeFetchingNewSource(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	var fetched bool
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		fetched = true
		return nil, nil
	})

	err := c.Run([]string{"restish", "config", "theme", "set", "example/themes"})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if fetched {
		t.Fatal("theme was fetched before confirmation")
	}
	if !strings.Contains(out.String(), "Theme URL: https://raw.githubusercontent.com/example/themes/HEAD/theme.json") {
		t.Fatalf("expected resolved URL before prompt, got: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "Install theme from this source?") {
		t.Fatalf("expected confirmation prompt, got: %q", errOut.String())
	}
}

func TestThemeSetSkipsPromptForSameSource(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, []byte(`{
  "theme_source": "https://themes.example.com/theme.json",
  "theme": {"key":"#111111"}
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"#ffffff"}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "config", "theme", "set", "https://themes.example.com/theme.json"}); err != nil {
		t.Fatalf("config theme set: %v", err)
	}
	if strings.Contains(errOut.String(), "Install theme") {
		t.Fatalf("unexpected confirmation prompt: %q", errOut.String())
	}
}

func TestThemeSetRejectsLargeResponse(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(strings.Repeat(" ", 256*1024+1))),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "config", "theme", "set", "https://themes.example.com/theme.json", "--yes"})
	if err == nil {
		t.Fatal("expected oversized theme error")
	}
	if !strings.Contains(err.Error(), "larger than 262144 bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}
