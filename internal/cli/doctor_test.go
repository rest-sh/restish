package cli_test

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func TestDoctorReportsInsecureTokenCachePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not authoritative on Windows")
	}
	c, out, errOut := newTestCLI(t)
	tokenPath := c.Hooks().TokenCachePath
	if err := os.WriteFile(tokenPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write token cache: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Token cache permissions: insecure") ||
		!strings.Contains(got, "before the next OAuth request") {
		t.Fatalf("expected token cache remediation, got:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "Tip: use -o json for machine-readable output.") {
		t.Fatalf("expected redirected-output JSON hint on stderr, got:\n%s", errOut.String())
	}
}

func assertDoctorExitCode(t *testing.T, err error, code int) {
	t.Helper()
	var exitErr *cli.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError{%d}, got %T: %v", code, err, err)
	}
	if exitErr.Code != code {
		t.Fatalf("exit code = %d, want %d", exitErr.Code, code)
	}
}

func TestDoctorTextWritesToStderrWhenStdoutIsTTY(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }

	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("doctor should not write text report to tty stdout, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "Config file:") {
		t.Fatalf("expected text report on stderr, got:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Content types:") ||
		!strings.Contains(errOut.String(), "json") {
		t.Fatalf("expected content type summary on stderr, got:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "Use -o json for machine-readable output.") {
		t.Fatalf("tty doctor should not print redirected-output JSON hint, got:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Restish version:") ||
		!strings.Contains(errOut.String(), "Platform:") ||
		!strings.Contains(errOut.String(), "HTTP cache summary:") ||
		!strings.Contains(errOut.String(), "APIs:") ||
		!strings.Contains(errOut.String(), "Theme:") {
		t.Fatalf("expected support-bundle fields on stderr, got:\n%s", errOut.String())
	}
}

func TestDoctorTextColorizesStatusesWhenColorEnabled(t *testing.T) {
	t.Setenv("NOCOLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLOR", "1")

	c, _, errOut := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }

	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	got := errOut.String()
	if !strings.Contains(got, "Config parse:") ||
		!strings.Contains(got, "Config permissions:") {
		t.Fatalf("expected doctor status lines, got:\n%s", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colorized doctor statuses, got:\n%q", got)
	}
}

func TestDoctorJSONWritesMachineReadableReport(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "-o", "json"}); err != nil {
		t.Fatalf("doctor -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		ConfigFile string `json:"config_file"`
		Runtime    struct {
			Version        string `json:"version"`
			GOOS           string `json:"goos"`
			GOARCH         string `json:"goarch"`
			StdoutTerminal bool   `json:"stdout_terminal"`
			ColorEnabled   bool   `json:"color_enabled"`
		} `json:"runtime"`
		ConfigParse struct {
			Status   string `json:"status"`
			APICount int    `json:"api_count"`
		} `json:"config_parse"`
		HTTPCacheSummary struct {
			Directory string `json:"directory"`
			Size      string `json:"size"`
			Entries   int    `json:"entries"`
		} `json:"http_cache_summary"`
		Theme struct {
			Status string `json:"status"`
		} `json:"theme"`
		APIs struct {
			Count int      `json:"count"`
			Names []string `json:"names"`
		} `json:"apis"`
		ContentTypes []struct {
			Name      string   `json:"name"`
			MIMETypes []string `json:"mime_types"`
		} `json:"content_types"`
		PluginDirectory  string `json:"plugin_directory"`
		InstalledPlugins []struct {
			Name string `json:"name"`
		} `json:"installed_plugins"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.ConfigFile != c.Hooks().ConfigPath {
		t.Fatalf("config_file = %q, want %q", report.ConfigFile, c.Hooks().ConfigPath)
	}
	if report.ConfigParse.Status != "ok" || report.ConfigParse.APICount != 1 {
		t.Fatalf("unexpected config parse report: %#v", report.ConfigParse)
	}
	if report.Runtime.Version == "" || report.Runtime.GOOS == "" || report.Runtime.GOARCH == "" {
		t.Fatalf("runtime report missing fields: %#v", report.Runtime)
	}
	if report.HTTPCacheSummary.Directory == "" {
		t.Fatalf("cache summary missing directory: %#v", report.HTTPCacheSummary)
	}
	if report.Theme.Status == "" {
		t.Fatalf("theme report missing status: %#v", report.Theme)
	}
	if report.APIs.Count != 1 || !reflect.DeepEqual(report.APIs.Names, []string{"demo"}) {
		t.Fatalf("API inventory = %#v, want demo", report.APIs)
	}
	if report.PluginDirectory == "" {
		t.Fatal("expected plugin_directory in doctor report")
	}
	if report.InstalledPlugins == nil {
		t.Fatal("expected installed_plugins in doctor report")
	}
	foundJSON := false
	for _, ct := range report.ContentTypes {
		if ct.Name == "json" {
			foundJSON = true
			if len(ct.MIMETypes) == 0 {
				t.Fatalf("json content type missing MIME types: %#v", ct)
			}
		}
	}
	if !foundJSON {
		t.Fatalf("expected json content type in doctor report: %#v", report.ContentTypes)
	}
}

func TestDoctorReportsInstalledPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script plugin tests not supported on Windows")
	}
	c, out, errOut := newTestCLI(t)
	pluginDir := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "restish-csv")
	script := `#!/bin/sh
echo '{"name":"csv","version":"test","restish_api_version":2,"hooks":["formatter"],"formatter_names":["csv"]}'
`
	if err := os.WriteFile(pluginPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Installed plugins:") ||
		!strings.Contains(got, "csv test capabilities: formatter, formatter(csv)") {
		t.Fatalf("expected installed plugin summary, got:\n%s\nstderr:\n%s", got, errOut.String())
	}
}

func TestDoctorJSONReportsInstalledPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script plugin tests not supported on Windows")
	}
	c, out, errOut := newTestCLI(t)
	pluginDir := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "restish-csv")
	script := `#!/bin/sh
echo '{"name":"csv","version":"test","restish_api_version":2,"hooks":["formatter"],"formatter_names":["csv"]}'
`
	if err := os.WriteFile(pluginPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	if err := c.Run([]string{"restish", "doctor", "-o", "json"}); err != nil {
		t.Fatalf("doctor -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		InstalledPlugins []struct {
			Name         string   `json:"name"`
			Version      string   `json:"version"`
			Path         string   `json:"path"`
			Capabilities []string `json:"capabilities"`
			Formatters   []string `json:"formatters"`
		} `json:"installed_plugins"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor -o json output is not JSON: %v\n%s", err, out.String())
	}
	if len(report.InstalledPlugins) != 1 {
		t.Fatalf("installed_plugins = %#v, want one plugin", report.InstalledPlugins)
	}
	plugin := report.InstalledPlugins[0]
	if plugin.Name != "csv" || plugin.Version != "test" || plugin.Path != pluginPath ||
		!reflect.DeepEqual(plugin.Capabilities, []string{"formatter"}) ||
		!reflect.DeepEqual(plugin.Formatters, []string{"csv"}) {
		t.Fatalf("unexpected plugin report: %#v", plugin)
	}
}

func TestDoctorRejectsUnsupportedOutputFormats(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "root", args: []string{"restish", "doctor", "-o", "yaml"}},
		{name: "api", args: []string{"restish", "doctor", "api", "demo", "-o", "yaml"}},
		{name: "plugin", args: []string{"restish", "doctor", "plugin", "demo", "-o", "yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, out, errOut := newTestCLI(t)
			if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v should reject unsupported output format", tt.args)
			}
			if !strings.Contains(err.Error(), "supports -o json for structured output, not -o yaml") {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Len() != 0 || errOut.Len() != 0 {
				t.Fatalf("unsupported output format should not print report, stdout=%q stderr=%q", out.String(), errOut.String())
			}
		})
	}
}

func TestDoctorPluginBareNameUsesRestishExecutablePrefix(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	assertDoctorExitCode(t, c.Run([]string{"restish", "doctor", "plugin", "csv", "-o", "json"}), 2)
	if errOut.Len() != 0 {
		t.Fatalf("doctor plugin -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Plugin string `json:"plugin"`
		Path   string `json:"path"`
		Found  bool   `json:"found"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor plugin -o json output is not JSON: %v\n%s", err, out.String())
	}
	wantPath := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "plugins", "restish-csv")
	if report.Plugin != "csv" || report.Path != wantPath {
		t.Fatalf("plugin report = %#v, want plugin csv path %s", report, wantPath)
	}
	if report.Found || report.Error != "not found" {
		t.Fatalf("expected missing prefixed plugin report, got %#v", report)
	}
}

func TestDoctorAPIUnknownTargetExitsNonZero(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	assertDoctorExitCode(t, c.Run([]string{"restish", "doctor", "api", "missing"}), 2)
	if !strings.Contains(out.String(), `API "missing": not registered`) {
		t.Fatalf("expected missing API report, got stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestDoctorAPIUnknownTargetJSONExitsNonZero(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	assertDoctorExitCode(t, c.Run([]string{"restish", "doctor", "api", "missing", "-o", "json"}), 2)
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		API        string `json:"api"`
		Registered bool   `json:"registered"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.API != "missing" || report.Registered {
		t.Fatalf("unexpected missing API report: %#v", report)
	}
}

func TestDoctorAPIHintsSyncWhenSpecCacheMissing(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := c.Run([]string{"restish", "doctor", "api", "demo"}); err != nil {
		t.Fatalf("doctor api: %v", err)
	}
	requireContains(t, out.String(), "Spec cache: missing", `run "restish api sync demo"`, "Generated operations: unavailable")
	if !strings.Contains(errOut.String(), "Tip: use -o json for machine-readable output.") {
		t.Fatalf("doctor api should print redirected-output JSON hint, got:\n%s", errOut.String())
	}
}

func TestDoctorPluginUnknownTargetTextExitsNonZero(t *testing.T) {
	c, out, errOut := newTestCLI(t)

	assertDoctorExitCode(t, c.Run([]string{"restish", "doctor", "plugin", "csv"}), 2)
	if !strings.Contains(out.String(), `Plugin "csv": not found`) {
		t.Fatalf("expected missing plugin report, got stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestDoctorJSONReportsUnsupportedReferencedAuthProfile(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{
  "apis": {
    "demo": {
      "base_url": "https://api.example.com",
      "profiles": {"secondary": {"auth_ref": "shared-basic"}}
    }
  },
  "auth_profiles": {
    "shared-basic": {
      "type": "basic",
      "params": {"username": "demo", "password": "secret"}
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "-o", "json"}); err != nil {
		t.Fatalf("doctor -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		ConfigParse struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"config_parse"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.ConfigParse.Status != "invalid" {
		t.Fatalf("config_parse.status = %q, want invalid", report.ConfigParse.Status)
	}
	if !strings.Contains(report.ConfigParse.Error, "auth_profiles.shared-basic") ||
		!strings.Contains(report.ConfigParse.Error, `unknown auth type "basic"`) {
		t.Fatalf("unexpected runtime validation error: %q", report.ConfigParse.Error)
	}
}

func TestDoctorAPIJSONTreats405AsReachable(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodHead {
			t.Fatalf("doctor reachability used %s, want HEAD", r.Method)
		}
		return &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			Status:     "405 Method Not Allowed",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Registered   bool `json:"registered"`
		Reachability struct {
			Status     string `json:"status"`
			Checked    bool   `json:"checked"`
			Reachable  bool   `json:"reachable"`
			Method     string `json:"method"`
			StatusCode int    `json:"status_code"`
			Note       string `json:"note"`
		} `json:"reachability"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if !report.Registered {
		t.Fatal("expected registered API")
	}
	if report.Reachability.Status != "ok" ||
		!report.Reachability.Checked ||
		!report.Reachability.Reachable ||
		report.Reachability.Method != http.MethodHead ||
		report.Reachability.StatusCode != http.StatusMethodNotAllowed ||
		report.Reachability.Note == "" {
		t.Fatalf("unexpected reachability report: %#v", report.Reachability)
	}
}

func TestDoctorAPIReportsPersistentCredentialSettings(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{
  "apis": {
    "demo": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "headers": ["Authorization: Bearer secret", "X-Env: dev"],
          "query": ["api_key=query-secret", "page=1"]
        }
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "demo"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Auth struct {
			Status  string   `json:"status"`
			Sources []string `json:"sources"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.Auth.Status != "configured" {
		t.Fatalf("auth.status = %q, want configured", report.Auth.Status)
	}
	if strings.Join(report.Auth.Sources, ",") != "headers,query" {
		t.Fatalf("auth.sources = %#v, want headers/query", report.Auth.Sources)
	}

	out.Reset()
	errOut.Reset()
	if err := c.Run([]string{"restish", "doctor", "api", "demo"}); err != nil {
		t.Fatalf("doctor api returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Auth: configured (headers, query)") {
		t.Fatalf("doctor api text did not report persistent credentials:\n%s", out.String())
	}
}

func TestDoctorAPIReportsStaleGeneratedOperations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	expireGeneratedSpecCache(t, env.cacheDir, "tapi")
	c, out := env.newCaptureCLI()

	if err := c.Run([]string{"restish", "doctor", "api", "tapi"}); err != nil {
		t.Fatalf("doctor api returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Spec cache: stale") || !strings.Contains(got, "Generated operations: 7 available (stale") {
		t.Fatalf("doctor api did not report stale generated operations:\n%s", got)
	}
}

func TestDoctorAPIReportsLocalSpecGeneratedOperations(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Local Test API", "version": "1.0"},
  "servers": [{"url": "https://api.example.com"}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "security": [{"BearerAuth": []}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	c, out, errOut := newTestCLI(t)
	configBody := fmt.Sprintf(`{"apis":{"local":{"base_url":"https://api.example.com","spec_files":[%q]}}}`, specPath)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "local"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		GeneratedOperations struct {
			Status string   `json:"status"`
			Count  int      `json:"count"`
			Issues []string `json:"issues"`
		} `json:"generated_operations"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.GeneratedOperations.Status != "available" || report.GeneratedOperations.Count != 1 {
		t.Fatalf("generated_operations = %#v, want available count 1", report.GeneratedOperations)
	}
	if len(report.GeneratedOperations.Issues) != 1 || !strings.Contains(report.GeneratedOperations.Issues[0], `security scheme "BearerAuth" is referenced`) {
		t.Fatalf("generated_operations issues = %#v, want undeclared BearerAuth issue", report.GeneratedOperations.Issues)
	}

	out.Reset()
	errOut.Reset()
	if err := c.Run([]string{"restish", "doctor", "api", "local"}); err != nil {
		t.Fatalf("doctor api returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Generated operations: 1 available") {
		t.Fatalf("doctor api text did not report local generated operations:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `Issue: security scheme "BearerAuth" is referenced`) {
		t.Fatalf("doctor api text did not report undeclared security issue:\n%s", out.String())
	}
}

func TestDoctorAPIReportsMissingEnvAuth(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{
  "apis": {
    "demo": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "bearer",
            "params": {"token": "env:MISSING_DOCTOR_TOKEN"}
          }
        }
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "demo"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Auth struct {
			Status string   `json:"status"`
			Issues []string `json:"issues"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.Auth.Status != "configured-but-unresolved" {
		t.Fatalf("auth.status = %q, want configured-but-unresolved", report.Auth.Status)
	}
	if strings.Join(report.Auth.Issues, ",") != "env missing: MISSING_DOCTOR_TOKEN" {
		t.Fatalf("auth.issues = %#v", report.Auth.Issues)
	}

	out.Reset()
	errOut.Reset()
	if err := c.Run([]string{"restish", "doctor", "api", "demo"}); err != nil {
		t.Fatalf("doctor api returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Auth: configured but unresolved (env missing: MISSING_DOCTOR_TOKEN)") {
		t.Fatalf("doctor api text did not report missing env:\n%s", out.String())
	}
}

func TestDoctorAPIJSONWarnsOnServerErrorReachability(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Reachability struct {
			Status     string `json:"status"`
			Reachable  bool   `json:"reachable"`
			StatusCode int    `json:"status_code"`
			Note       string `json:"note"`
		} `json:"reachability"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.Reachability.Status != "warn" || report.Reachability.Reachable || report.Reachability.StatusCode != http.StatusInternalServerError || !strings.Contains(report.Reachability.Note, "server error") {
		t.Fatalf("unexpected reachability report: %#v", report.Reachability)
	}
}

func TestDoctorAPIReachabilityUsesProfileCACert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("doctor reachability used %s, want HEAD", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}

	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(fmt.Sprintf(`{
  "apis": {
    "demo": {
      "base_url": %q,
      "profiles": {
        "default": {"ca_cert": %q}
      }
    }
  }
}`, srv.URL, caPath)), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "-o", "json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api -o json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api -o json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Reachability struct {
			Status     string `json:"status"`
			Reachable  bool   `json:"reachable"`
			StatusCode int    `json:"status_code"`
		} `json:"reachability"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api -o json output is not JSON: %v\n%s", err, out.String())
	}
	if report.Reachability.Status != "ok" || !report.Reachability.Reachable || report.Reachability.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected reachability report: %#v", report.Reachability)
	}
}
