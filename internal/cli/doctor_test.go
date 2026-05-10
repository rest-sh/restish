package cli_test

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	if !strings.Contains(errOut.String(), "Use --json for machine-readable output.") {
		t.Fatalf("expected redirected-output JSON hint on stderr, got:\n%s", errOut.String())
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
	if strings.Contains(errOut.String(), "Use --json for machine-readable output.") {
		t.Fatalf("tty doctor should not print redirected-output JSON hint, got:\n%s", errOut.String())
	}
}

func TestDoctorJSONWritesMachineReadableReport(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"demo":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor", "--json"}); err != nil {
		t.Fatalf("doctor --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor --json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		ConfigFile  string `json:"config_file"`
		ConfigParse struct {
			Status   string `json:"status"`
			APICount int    `json:"api_count"`
		} `json:"config_parse"`
		PluginDirectory string `json:"plugin_directory"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor --json output is not JSON: %v\n%s", err, out.String())
	}
	if report.ConfigFile != c.Hooks().ConfigPath {
		t.Fatalf("config_file = %q, want %q", report.ConfigFile, c.Hooks().ConfigPath)
	}
	if report.ConfigParse.Status != "ok" || report.ConfigParse.APICount != 1 {
		t.Fatalf("unexpected config parse report: %#v", report.ConfigParse)
	}
	if report.PluginDirectory == "" {
		t.Fatal("expected plugin_directory in doctor report")
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
	if err := c.Run([]string{"restish", "doctor", "--json"}); err != nil {
		t.Fatalf("doctor --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor --json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		ConfigParse struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"config_parse"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor --json output is not JSON: %v\n%s", err, out.String())
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
	if err := c.Run([]string{"restish", "doctor", "--json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api --json should keep stderr quiet, got:\n%s", errOut.String())
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
		t.Fatalf("doctor api --json output is not JSON: %v\n%s", err, out.String())
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
	if err := c.Run([]string{"restish", "doctor", "--json", "api", "demo"}); err != nil {
		t.Fatalf("doctor api --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api --json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Auth struct {
			Status  string   `json:"status"`
			Sources []string `json:"sources"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api --json output is not JSON: %v\n%s", err, out.String())
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
	if err := c.Run([]string{"restish", "doctor", "--json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api --json should keep stderr quiet, got:\n%s", errOut.String())
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
		t.Fatalf("doctor api --json output is not JSON: %v\n%s", err, out.String())
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
	if err := c.Run([]string{"restish", "doctor", "--json", "api", "demo", "--check-network"}); err != nil {
		t.Fatalf("doctor api --json returned error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor api --json should keep stderr quiet, got:\n%s", errOut.String())
	}
	var report struct {
		Reachability struct {
			Status     string `json:"status"`
			Reachable  bool   `json:"reachable"`
			StatusCode int    `json:"status_code"`
		} `json:"reachability"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor api --json output is not JSON: %v\n%s", err, out.String())
	}
	if report.Reachability.Status != "ok" || !report.Reachability.Reachable || report.Reachability.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected reachability report: %#v", report.Reachability)
	}
}
