package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/tailscale/hujson"
)

func writeJSONCConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeJSONCConfig: %v", err)
	}
	return path
}

func TestJSONCSetPath_ReplacesValueAndPreservesInlineComment(t *testing.T) {
	input := []byte(`{
  "apis": {
    "myapi": {
      "base_url": "https://old.example.com" // keep
    }
  }
}`)

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://new.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, `"base_url": "https://new.example.com" // keep`) {
		t.Fatalf("expected updated value and preserved inline comment:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_CreatesNestedObjects(t *testing.T) {
	input := []byte(`{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com"
    }
  }
}`)

	patched, err := jsoncSetPath(input, []string{"apis", "myapi", "profiles", "default", "auth", "params", "token"}, "secret")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	cfg, err := parseConfigBytes("test", patched)
	if err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
	if token := cfg.APIs["myapi"].Profiles["default"].Auth.Params["token"]; token != "secret" {
		t.Fatalf("token = %q, want secret", token)
	}
}

func TestJSONCSetPath_RejectsScalarWhenNestedObjectRequired(t *testing.T) {
	input := []byte(`{
  "apis": {
    "myapi": {
      "profiles": {
        "default": {
          "auth": "legacy"
        }
      }
    }
  }
}`)

	_, err := jsoncSetPath(input, []string{"apis", "myapi", "profiles", "default", "auth", "params", "token"}, "secret")
	if err == nil {
		t.Fatal("expected jsoncSetPath to fail")
	}
	want := `cannot set nested path "auth.params.token": value at "auth" is not an object (got literal)`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJSONCDeletePath_RemovesMiddleMemberAndKeepsNeighbors(t *testing.T) {
	input := []byte(`{
  "apis": {
    "first": {
      "base_url": "https://first.example.com"
    },
    // remove me
    "remove": {
      "base_url": "https://remove.example.com"
    },
    "last": {
      "base_url": "https://last.example.com"
    }
  }
}`)

	got, err := jsoncDeletePath(input, []string{"apis", "remove"})
	if err != nil {
		t.Fatalf("jsoncDeletePath: %v", err)
	}

	out := string(got)
	if strings.Contains(out, "remove.example.com") {
		t.Fatalf("expected target member to be removed:\n%s", out)
	}
	if !strings.Contains(out, `"first"`) || !strings.Contains(out, `"last"`) {
		t.Fatalf("expected neighbors to remain:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCDeletePath_RemovesFirstMember(t *testing.T) {
	input := []byte(`{
  "apis": {
    // first
    "first": {
      "base_url": "https://first.example.com"
    },
    "second": {
      "base_url": "https://second.example.com"
    }
  }
}`)

	got, err := jsoncDeletePath(input, []string{"apis", "first"})
	if err != nil {
		t.Fatalf("jsoncDeletePath: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "first.example.com") {
		t.Fatalf("expected first member removed:\n%s", out)
	}
	if !strings.Contains(out, "second.example.com") {
		t.Fatalf("expected second member to remain:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCDeletePath_RemovesLastMember(t *testing.T) {
	input := []byte(`{
  "apis": {
    "first": {
      "base_url": "https://first.example.com"
    },
    // last
    "last": {
      "base_url": "https://last.example.com"
    }
  }
}`)

	got, err := jsoncDeletePath(input, []string{"apis", "last"})
	if err != nil {
		t.Fatalf("jsoncDeletePath: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "last.example.com") {
		t.Fatalf("expected last member removed:\n%s", out)
	}
	if strings.Contains(out, `},\n  }`) {
		t.Fatalf("expected no trailing comma before object close:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCDeletePath_RemovesOnlyMember(t *testing.T) {
	input := []byte(`{
  "apis": {
    "only": {
      "base_url": "https://only.example.com"
    }
  }
}`)

	got, err := jsoncDeletePath(input, []string{"apis", "only"})
	if err != nil {
		t.Fatalf("jsoncDeletePath: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "only.example.com") {
		t.Fatalf("expected only member removed:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCDeletePath_MissingPathLeavesBytesUnchanged(t *testing.T) {
	input := []byte(`{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`)

	got, err := jsoncDeletePath(input, []string{"apis", "missing"})
	if !errors.Is(err, ErrPathNotFound) {
		t.Fatalf("jsoncDeletePath err = %v, want ErrPathNotFound", err)
	}
	if got != nil {
		t.Fatalf("expected nil bytes on missing delete, got:\n%s", string(got))
	}
}

func TestJSONCSetPathOverwritesNullParent(t *testing.T) {
	input := []byte(`{"apis": null}`)

	got, err := jsoncSetPath(input, []string{"apis", "demo", "base_url"}, "https://api.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	if !strings.Contains(string(got), `"demo"`) || !strings.Contains(string(got), `"base_url"`) {
		t.Fatalf("expected null parent overwritten with object, got:\n%s", got)
	}
}

func TestClosestJSONFieldScalesWithLengthAndRunes(t *testing.T) {
	fields := jsonFieldTypes(reflect.TypeOf(APIConfig{}))
	if got := closestJSONField("allow_cross_origin_specs", fields); got != "allow_cross_origin_spec" {
		t.Fatalf("long suggestion = %q, want allow_cross_origin_spec", got)
	}
	if got := closestJSONField("x", fields); got != "" {
		t.Fatalf("short suggestion = %q, want none", got)
	}
	if got := levenshteinDistance("café", "cafe"); got != 1 {
		t.Fatalf("rune levenshtein = %d, want 1", got)
	}
}

func TestJSONCSetPath_InsertsIntoEmptyMultilineObject(t *testing.T) {
	input := []byte("{\n  \"apis\": {}\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{"base_url": "https://api.example.com"})
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "\"myapi\"") {
		t.Fatalf("expected inserted key:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_InsertsIntoEmptyInlineObject(t *testing.T) {
	input := []byte(`{"apis":{}}`)

	got, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{"base_url": "https://api.example.com"})
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `"myapi"`) {
		t.Fatalf("expected inserted key:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_ReplacesValueWithMultilineObject(t *testing.T) {
	input := []byte("{\n  \"apis\": {\n    \"myapi\": {}\n  }\n}")

	value := map[string]any{
		"base_url": "https://api.example.com",
		"profiles": map[string]any{
			"default": map[string]any{
				"auth": map[string]any{
					"type": "bearer",
				},
			},
		},
	}

	got, err := jsoncSetPath(input, []string{"apis", "myapi"}, value)
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_PreservesBlockComment(t *testing.T) {
	input := []byte(`{
  "apis": {
    /* keep this block */
    "myapi": {
      "base_url": "https://old.example.com"
    }
  }
}`)

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://new.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "/* keep this block */") {
		t.Fatalf("expected block comment preserved:\n%s", out)
	}
	if !strings.Contains(out, "https://new.example.com") {
		t.Fatalf("expected value update:\n%s", out)
	}
}

func TestJSONCSetPath_PreservesUntouchedDeepSiblings(t *testing.T) {
	input := []byte(`{
  "apis": {
    "target": {
      "base_url": "https://old.example.com"
    },
    "other": {
      // keep this nested structure exactly
      "profiles": [
        {
          "name": "default",
          "auth": {
            "type": "bearer",
            "params": {
              "token": "secret"
            }
          }
        }
      ]
    }
  }
}`)

	got, err := jsoncSetPath(input, []string{"apis", "target", "base_url"}, "https://new.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, `"base_url": "https://new.example.com"`) {
		t.Fatalf("expected target value update:\n%s", out)
	}
	if !strings.Contains(out, `// keep this nested structure exactly`) {
		t.Fatalf("expected untouched sibling comment preserved:\n%s", out)
	}
	if !strings.Contains(out, `"token": "secret"`) {
		t.Fatalf("expected untouched sibling nested value preserved:\n%s", out)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestJSONCSetPath_PreservesTriviaAroundValue(t *testing.T) {
	input := []byte(`{
  "apis": {
    "myapi": {
      "base_url" /* before colon */ : /* before value */ "https://old.example.com" // keep
    }
  }
}`)

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://new.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, `"base_url" /* before colon */ : /* before value */ "https://new.example.com" // keep`) {
		t.Fatalf("expected trivia around value preserved:\n%s", out)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestJSONCSetPath_PreservesCRLFInput(t *testing.T) {
	input := []byte("{\r\n  \"apis\": {\r\n    \"myapi\": {\r\n      \"base_url\": \"https://old.example.com\"\r\n    }\r\n  }\r\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://new.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "\r\n") {
		t.Fatalf("expected CRLF newlines to remain present:\n%q", out)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestJSONCSetPath_PreservesTabIndentation(t *testing.T) {
	input := []byte("{\n\t\"apis\": {\n\t\t\"myapi\": {}\n\t}\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://api.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	cfg, err := parseConfigBytes("test", got)
	if err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
	if gotURL := cfg.APIs["myapi"].BaseURL; gotURL != "https://api.example.com" {
		t.Fatalf("base_url = %q, want https://api.example.com", gotURL)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestJSONCSetPath_RejectsInvalidJSONC(t *testing.T) {
	input := []byte(`{"apis": [}`)

	if _, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{}); err == nil {
		t.Fatal("expected invalid JSONC to fail")
	}
}

func TestJSONCSetPath_RejectsInvalidUntouchedNestedJSONC(t *testing.T) {
	input := []byte(`{
  "apis": {
    "target": {
      "base_url": "https://old.example.com"
    },
    "other": {
      "profiles": [}
    }
  }
}`)

	if _, err := jsoncSetPath(input, []string{"apis", "target", "base_url"}, "https://new.example.com"); err == nil {
		t.Fatal("expected invalid nested JSONC to fail even off the edited path")
	}
}

func TestJSONCSetPath_RejectsNonObjectRoot(t *testing.T) {
	input := []byte(`[]`)

	if _, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{}); err == nil {
		t.Fatal("expected non-object root to fail")
	}
}

func TestJSONCSetPath_EscapedObjectKey(t *testing.T) {
	input := []byte("{\n  \"apis\": {\n    \"myapi\": {\n      \"quote\\\"key\": \"old\"\n    }\n  }\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "quote\"key"}, "new")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, `"quote\"key": "new"`) {
		t.Fatalf("expected escaped key to be updated:\n%s", out)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestJSONCSetPath_UnicodeEscapedObjectKey(t *testing.T) {
	input := []byte("{\n  \"apis\": {\n    \"myapi\": {\n      \"na\\u006de\": \"old\"\n    }\n  }\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "name"}, "new")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}

	out := string(got)
	if !strings.Contains(out, `"na\u006de": "new"`) {
		t.Fatalf("expected unicode-escaped key to be updated in place:\n%s", out)
	}
	if _, err := hujson.Parse(got); err != nil {
		t.Fatalf("patched JSONC should still parse: %v", err)
	}
}

func TestSaveConfigValue_RejectsInvalidConfigShape(t *testing.T) {
	path := writeJSONCConfig(t, `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}`)

	err := SaveConfigValue(path, []string{"apis"}, []string{"not", "an", "object"})
	if err == nil {
		t.Fatal("expected invalid config shape to fail")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(data) != `{"apis":{"myapi":{"base_url":"https://api.example.com"}}}` {
		t.Fatalf("expected file to remain unchanged:\n%s", string(data))
	}
}

func TestSaveConfigValue_PreservesCommentsOnDisk(t *testing.T) {
	path := writeJSONCConfig(t, `{
  // API registrations
  "apis": {
    "myapi": {
      "base_url": "https://old.example.com" // keep
    }
  }
}`)

	if err := SaveConfigValue(path, []string{"apis", "myapi", "base_url"}, "https://new.example.com"); err != nil {
		t.Fatalf("SaveConfigValue: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "// API registrations") {
		t.Fatalf("expected file comment to be preserved:\n%s", out)
	}
	if !strings.Contains(out, `"base_url": "https://new.example.com" // keep`) {
		t.Fatalf("expected updated value and inline comment to remain:\n%s", out)
	}
}

func TestSaveConfigValue_CreatesMissingConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "restish.json")

	if err := SaveConfigValue(path, []string{"apis", "myapi", "base_url"}, "https://api.example.com"); err != nil {
		t.Fatalf("SaveConfigValue: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url = %q, want https://api.example.com", got)
	}
}

func TestDeleteAPIConfig_MissingFileIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")

	if err := DeleteAPIConfig(path, "missing"); err != nil {
		t.Fatalf("DeleteAPIConfig: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected missing config file to remain missing, stat err = %v", err)
	}
}

func TestDeleteAPIConfig_PreservesOtherCommentsOnDisk(t *testing.T) {
	path := writeJSONCConfig(t, `{
  "apis": {
    // Keep this API
    "keep": {
      "base_url": "https://keep.example.com"
    },
    // Remove this API
    "remove": {
      "base_url": "https://remove.example.com"
    }
  }
}`)

	if err := DeleteAPIConfig(path, "remove"); err != nil {
		t.Fatalf("DeleteAPIConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "// Keep this API") {
		t.Fatalf("expected kept comment to remain:\n%s", out)
	}
	if strings.Contains(out, "remove.example.com") {
		t.Fatalf("expected removed API to be gone:\n%s", out)
	}
}

func TestSaveConfigValue_CreatesConfigDirWithSecurePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	path := filepath.Join(dir, "restish.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"apis":{}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SaveConfigValue(path, []string{"apis", "myapi"}, map[string]any{"base_url": "https://api.example.com"}); err != nil {
		t.Fatalf("SaveConfigValue: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Fatalf("expected existing config dir permission to remain 0755, got %04o", perm)
	}
}

func TestSaveConfigValue_ConcurrentWritesPreserveBothEdits(t *testing.T) {
	path := writeJSONCConfig(t, `{"apis":{}}`)

	start := make(chan struct{})
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, tc := range []struct {
		objectPath []string
		value      string
	}{
		{objectPath: []string{"apis", "foo", "base_url"}, value: "https://foo.example.com"},
		{objectPath: []string{"apis", "bar", "base_url"}, value: "https://bar.example.com"},
	} {
		wg.Add(1)
		go func(objectPath []string, value string) {
			defer wg.Done()
			<-start
			errCh <- SaveConfigValue(path, objectPath, value)
		}(tc.objectPath, tc.value)
	}

	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("SaveConfigValue: %v", err)
		}
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.APIs["foo"].BaseURL; got != "https://foo.example.com" {
		t.Fatalf("foo base_url = %q", got)
	}
	if got := cfg.APIs["bar"].BaseURL; got != "https://bar.example.com" {
		t.Fatalf("bar base_url = %q", got)
	}
}

func TestSaveConfigValue_RejectsNestedPathThroughArray(t *testing.T) {
	path := writeJSONCConfig(t, `{"apis":{"foo":{"profiles":[]}}}`)

	err := SaveConfigValue(path, []string{"apis", "foo", "profiles", "default", "auth", "type"}, "bearer")
	if err == nil {
		t.Fatal("expected error")
	}
	want := `cannot set nested path "profiles.default.auth.type": value at "profiles" is not an object (got array)`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("unexpected error: %v", err)
	}
}
