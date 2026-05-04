package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	authpkg "github.com/rest-sh/restish/v2/internal/auth"
)

func installFakeEditor(t *testing.T, replacement string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("edit command tests use a POSIX shell helper")
	}

	dir := t.TempDir()
	replacementPath := filepath.Join(dir, "replacement.txt")
	if err := os.WriteFile(replacementPath, []byte(replacement), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	scriptPath := filepath.Join(dir, "editor.sh")
	script := "#!/bin/sh\nset -eu\ncp \"$1\" \"$RESTISH_EDIT_CAPTURE\"\ncp \"$RESTISH_EDIT_REPLACEMENT\" \"$1\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	capturePath := filepath.Join(dir, "captured.txt")
	t.Setenv("VISUAL", scriptPath)
	t.Setenv("EDITOR", "")
	t.Setenv("RESTISH_EDIT_CAPTURE", capturePath)
	t.Setenv("RESTISH_EDIT_REPLACEMENT", replacementPath)
	return capturePath
}

func TestEditCommandFetchesEditsAndPuts(t *testing.T) {
	captured := installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	var rr requestRecorder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originalFile, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("read captured file: %v", err)
	}
	if !strings.Contains(string(originalFile), `"name": "before"`) {
		t.Fatalf("captured file did not contain fetched JSON: %s", originalFile)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.body, &body); err != nil {
		t.Fatalf("update body is not valid JSON: %v", err)
	}
	if body["name"] != "after" {
		t.Fatalf("expected updated name, got %v", body["name"])
	}
	if !strings.Contains(out.String(), `"name"`) {
		t.Fatalf("expected final response in stdout, got %q", out.String())
	}
}

func TestEditCommandIgnoresEditorFormattingOnlyChanges(t *testing.T) {
	installFakeEditor(t, "{\"name\":\"before\"}\n")

	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			t.Fatalf("did not expect PUT for formatting-only editor save")
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := strings.Join(methods, ","); got != http.MethodGet {
		t.Fatalf("methods = %q, want GET", got)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no diff or response on stdout, got %q", out.String())
	}
	if got := errOut.String(); !strings.Contains(got, "No changes made.") {
		t.Fatalf("expected no-changes message, got %q", got)
	}
}

func TestEditCommandYAMLEditKeepsOriginalJSONWireType(t *testing.T) {
	captured := installFakeEditor(t, "name: after\n")

	var rr requestRecorder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			rr.capture(r)
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
				return
			}
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "--edit-format", "yaml", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originalFile, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("read captured file: %v", err)
	}
	if !strings.Contains(string(originalFile), "name: before") {
		t.Fatalf("captured file did not contain fetched YAML: %s", originalFile)
	}
	if got := rr.Last().Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("PUT Content-Type = %q, want application/json", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.body, &body); err != nil {
		t.Fatalf("update body is not valid JSON: %v; body=%q", err, rr.body)
	}
	if body["name"] != "after" {
		t.Fatalf("expected updated name, got %v", body["name"])
	}
}

func TestEditCommandInteractiveFlagOpensEditor(t *testing.T) {
	captured := installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-i", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(captured); err != nil {
		t.Fatalf("expected editor to capture fetched resource: %v", err)
	}
}

func TestEditCommandInteractiveFlagIsNoopWithPatchArgs(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			method = r.Method
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-i", "-y", srv.URL, "name:", "after"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("expected patch args to update without editor, got method %q", method)
	}
}

func TestEditCommandNoEditorPrintsEditableResource(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"before"}`)
	}))
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "--no-editor", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(methods, ","); got != http.MethodGet {
		t.Fatalf("methods = %q, want GET", got)
	}
	if !strings.Contains(out.String(), `"name": "before"`) {
		t.Fatalf("expected editable JSON on stdout, got %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}

func TestEditCommandUpdateUsesProfileHeaders(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	var putAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cfg-token" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			putAuth = r.Header.Get("Authorization")
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"headers": ["Authorization: Bearer cfg-token"]
					}
				}
			}
		}
	}`, srv.URL)
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)
	if err := c.Run([]string{"restish", "edit", "-y", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if putAuth != "Bearer cfg-token" {
		t.Fatalf("PUT Authorization = %q", putAuth)
	}
}

type editRetryAuthHandler struct {
	calls atomic.Int32
}

func (h *editRetryAuthHandler) Parameters() []authpkg.Param { return nil }

func (h *editRetryAuthHandler) Authenticate(_ context.Context, req *http.Request, ac authpkg.AuthContext) error {
	h.calls.Add(1)
	if ac.Force {
		req.Header.Set("Authorization", "Bearer fresh-token")
	} else {
		req.Header.Set("Authorization", "Bearer stale-token")
	}
	return nil
}

func (h *editRetryAuthHandler) SupportsForce() {}

func TestEditCommandUpdateRetriesUnauthorizedWithFreshAuth(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var putAttempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			putAttempts.Add(1)
			if r.Header.Get("Authorization") != "Bearer fresh-token" {
				http.Error(w, "stale token", http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"auth": {"type": "edit-retry"}
					}
				}
			}
		}
	}`, srv.URL)
	handler := &editRetryAuthHandler{}
	c, out, _ := newTestCLI(t)
	c.AddAuthHandler("edit-retry", handler)
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)
	if err := c.Run([]string{"restish", "edit", "-y", "myapi/item", "name:", "after"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := putAttempts.Load(); got != 2 {
		t.Fatalf("PUT attempts = %d, want 2", got)
	}
	if got := handler.calls.Load(); got != 3 {
		t.Fatalf("auth calls = %d, want 3 (GET, stale PUT, fresh retry)", got)
	}
	if !strings.Contains(out.String(), `"after"`) {
		t.Fatalf("expected retried update response, got %q", out.String())
	}
}

func TestEditCommandSendsIfMatch(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	var ifMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("ETag", `"abc123"`)
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			ifMatch = r.Header.Get("If-Match")
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifMatch != `"abc123"` {
		t.Fatalf("expected If-Match header, got %q", ifMatch)
	}
}

func TestEditCommandFallsBackToIfUnmodifiedSince(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	var ifUnmodifiedSince string
	var ifMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			ifMatch = r.Header.Get("If-Match")
			ifUnmodifiedSince = r.Header.Get("If-Unmodified-Since")
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifMatch != "" {
		t.Fatalf("did not expect If-Match when ETag is absent, got %q", ifMatch)
	}
	if ifUnmodifiedSince != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("expected If-Unmodified-Since header, got %q", ifUnmodifiedSince)
	}
}

func TestEditCommandWarnsWithoutPreconditionValidator(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"name":"before"}`)
		case http.MethodPut:
			fmt.Fprint(w, `{"name":"after"}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := errOut.String(); !strings.Contains(got, "not guarded against concurrent edits") {
		t.Fatalf("expected no-precondition warning, got %q", got)
	}
}

func TestEditCommandDryRunShowsDiffWithoutSending(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	methods := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"before"}`)
	}))
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "--dry-run", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(methods) != 1 || methods[0] != http.MethodGet {
		t.Fatalf("expected only GET during dry-run, got %v", methods)
	}
	got := out.String()
	if !strings.Contains(got, "--- original") || !strings.Contains(got, "+++ modified") {
		t.Fatalf("expected diff header in output, got %q", got)
	}
	if !strings.Contains(got, `"name": "before"`) || !strings.Contains(got, `"name": "after"`) {
		t.Fatalf("expected before/after diff in output, got %q", got)
	}
}

func TestEditCommandYesSkipsPrompt(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\"\n}\n")

	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"name":"before"}`)
			return
		}
		fmt.Fprint(w, `{"name":"after"}`)
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	c.Stdin = strings.NewReader("n\n")
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(methods, ",") != "GET,PUT" {
		t.Fatalf("expected GET followed by PUT, got %v", methods)
	}
}

func TestEditCommandUsesPatchWhenAdvertised(t *testing.T) {
	installFakeEditor(t, "{\n  \"name\": \"after\",\n  \"meta\": {\n    \"a\": 1\n  }\n}\n")

	var patchBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Accept-Patch", "application/merge-patch+json")
			fmt.Fprint(w, `{"name":"before","meta":{"a":1,"b":2}}`)
		case http.MethodPatch:
			var err error
			patchBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read patch body: %v", err)
			}
			fmt.Fprint(w, `{"name":"after","meta":{"a":1}}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "edit", "-y", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var patch map[string]any
	if err := json.Unmarshal(patchBody, &patch); err != nil {
		t.Fatalf("invalid patch json: %v", err)
	}
	if patch["name"] != "after" {
		t.Fatalf("expected patched name, got %v", patch["name"])
	}
	meta, ok := patch["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested meta patch, got %T", patch["meta"])
	}
	if _, ok := meta["a"]; ok {
		t.Fatalf("did not expect unchanged field a in patch: %v", meta)
	}
	if meta["b"] != nil {
		t.Fatalf("expected removed field b to be nil in patch, got %v", meta["b"])
	}
}

func TestEditCommandSupportsNonInteractivePatchArgs(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var method string
	var reqBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"string":"before","tags":["one"]}`)
		case http.MethodPut:
			method = r.Method
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			fmt.Fprint(w, `{"string":"changed","tags":["one","another"]}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{
		"restish", "edit", "-y", srv.URL,
		"string:", "changed,", "tags[]:", "another",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if method != http.MethodPut {
		t.Fatalf("expected PUT update, got %q", method)
	}
	var body map[string]any
	if err := json.Unmarshal(reqBody, &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body["string"] != "changed" {
		t.Fatalf("expected updated string field, got %v", body["string"])
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 2 || tags[1] != "another" {
		t.Fatalf("expected appended tags array, got %#v", body["tags"])
	}
	if !strings.Contains(out.String(), `"changed"`) {
		t.Fatalf("expected updated response in stdout, got %q", out.String())
	}
}
