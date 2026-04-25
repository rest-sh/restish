package cli_test

import (
	"io"
	"net/http"
	"os"
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

	if err := c.Run([]string{"restish", "theme", "set", "https://themes.example.com/theme.json"}); err != nil {
		t.Fatalf("theme set: %v", err)
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

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#ffffff" {
		t.Fatalf("theme key = %q, want #ffffff", cfg.Theme["key"])
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

	if err := c.Run([]string{"restish", "theme", "set", "example/themes"}); err != nil {
		t.Fatalf("theme set: %v", err)
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
		if got, want := r.URL.String(), "https://raw.githubusercontent.com/example/themes/HEAD/dark.json"; got != want {
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

	if err := c.Run([]string{"restish", "theme", "set", "example/themes", "dark"}); err != nil {
		t.Fatalf("theme set: %v", err)
	}

	cfg, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Theme["key"] != "#111111" {
		t.Fatalf("theme key = %q, want #111111", cfg.Theme["key"])
	}
}

func TestThemeSetRejectsNameForURL(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "theme", "set", "https://themes.example.com/theme.json", "dark"})
	if err == nil {
		t.Fatal("expected URL with theme name to fail")
	}
	if !strings.Contains(err.Error(), "only supported with GitHub") {
		t.Fatalf("unexpected error: %v", err)
	}
}
