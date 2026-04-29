package cli_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// enumSpec is an OpenAPI spec with a parameter that has enum values.
func enumSpec(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Enum API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "summary": "List items",
        "parameters": [
          {
            "name": "status",
            "in": "query",
            "required": false,
            "schema": {
              "type": "string",
              "enum": ["active", "inactive", "pending"]
            },
            "description": "Filter by status"
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
}

// TestEnumFlagCompletion verifies that OpenAPI enum values are registered as
// completion candidates for the corresponding flag.
func TestEnumFlagCompletion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, enumSpec)

	// Cobra registers a hidden __complete command that we can invoke to test
	// completion. Run: restish __complete tapi list-items --status ""
	c, out := env.newCaptureCLI()
	err := c.Run([]string{"restish", "__complete", "tapi", "list-items", "--status", ""})
	// __complete always exits 0 (success) even when it returns no results.
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}

	got := out.String()
	for _, want := range []string{"active", "inactive", "pending"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected completion candidate %q, got:\n%s", want, got)
		}
	}
}

func TestGeneratedSecurityFlagCompletion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/reports", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Security API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "UserOAuth": {"type": "oauth2", "flows": {"authorizationCode": {"authorizationUrl": "https://auth.example.com/authorize", "tokenUrl": "https://auth.example.com/token", "scopes": {"read": "Read"}}}},
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "paths": {
    "/reports": {
      "get": {
        "operationId": "getReports",
        "security": [
          {"UserOAuth": ["read"]},
          {"PartnerKey": []},
          {}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	err := c.Run([]string{"restish", "__complete", "tapi", "get-reports", "--rsh-security", ""})
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}

	got := out.String()
	for _, want := range []string{"UserOAuth", "PartnerKey", "anonymous"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected completion candidate %q, got:\n%s", want, got)
		}
	}
}
