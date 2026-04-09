package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !strings.Contains(string(patched), `"profiles": {"default": {"auth": {"params": {"token": "secret"}}}}`) {
		t.Fatalf("expected nested object insertion:\n%s", string(patched))
	}
}

func TestJSONCSetPath_ReplacesScalarWithNestedObject(t *testing.T) {
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
	if err != nil {
		t.Fatalf("jsoncDeletePath: %v", err)
	}
	if string(got) != string(input) {
		t.Fatalf("expected missing delete to leave bytes unchanged:\n%s", string(got))
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
	if !strings.Contains(out, "\n    \"myapi\":") {
		t.Fatalf("expected multiline insertion indentation:\n%s", out)
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
	if !strings.Contains(out, `"myapi": {`) {
		t.Fatalf("expected inserted key:\n%s", out)
	}
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected multiline expansion for multiline value:\n%s", out)
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
	out := string(got)
	if !strings.Contains(out, "\n      \"base_url\": \"https://api.example.com\"") {
		t.Fatalf("expected multiline nested indentation:\n%s", out)
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
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_PreservesTabIndentation(t *testing.T) {
	input := []byte("{\n\t\"apis\": {\n\t\t\"myapi\": {}\n\t}\n}")

	got, err := jsoncSetPath(input, []string{"apis", "myapi", "base_url"}, "https://api.example.com")
	if err != nil {
		t.Fatalf("jsoncSetPath: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "\t\t\"myapi\": {\"base_url\": \"https://api.example.com\"}") {
		t.Fatalf("expected tab indentation to be reused:\n%s", out)
	}
	if _, err := parseConfigBytes("test", got); err != nil {
		t.Fatalf("patched config should still parse: %v", err)
	}
}

func TestJSONCSetPath_RejectsInvalidJSONC(t *testing.T) {
	input := []byte(`{"apis": [}`)

	if _, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{}); err == nil {
		t.Fatal("expected invalid JSONC to fail")
	}
}

func TestJSONCSetPath_RejectsNonObjectRoot(t *testing.T) {
	input := []byte(`[]`)

	if _, err := jsoncSetPath(input, []string{"apis", "myapi"}, map[string]any{}); err == nil {
		t.Fatal("expected non-object root to fail")
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
