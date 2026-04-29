package cli_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBasicAuthHeader verifies that an http-basic profile sends the correct
// Authorization: Basic header on every request.
func TestBasicAuthHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
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
						"auth": {
							"type": "http-basic",
							"params": {"username": "alice", "password": "s3cr3t"}
						}
					}
				}
			}
		}
	}`
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := rr.Last().Header.Get("Authorization")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cr3t"))
	if got != want {
		t.Errorf("Authorization header: got %q, want %q", got, want)
	}
}

// TestBasicAuthPasswordPrompt verifies that when password is absent from params
// a prompt is written to stderr and the password is read from stdin.
func TestBasicAuthPasswordPrompt(t *testing.T) {
	var rr requestRecorder
	c, _, errBuf := newTestCLI(t)
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
						"auth": {
							"type": "http-basic",
							"params": {"username": "bob"}
						}
					}
				}
			}
		}
	}`
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)
	// Provide the password via PassReader (keeps Stdin free for body reads).
	c.Hooks().PassReader = strings.NewReader("hunter2\n")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prompt should have been written to stderr.
	if !strings.Contains(errBuf.String(), "Password") {
		t.Errorf("expected password prompt on stderr, got: %q", errBuf.String())
	}

	got := rr.Last().Header.Get("Authorization")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:hunter2"))
	if got != want {
		t.Errorf("Authorization header: got %q, want %q", got, want)
	}
}

func TestAPIKeyAuthHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
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
						"auth": {
							"type": "api-key",
							"params": {"in": "header", "name": "X-API-Key", "value": "secret-key"}
						}
					}
				}
			}
		}
	}`
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := rr.Last().Header.Get("X-API-Key"); got != "secret-key" {
		t.Errorf("X-API-Key: got %q, want secret-key", got)
	}
}

func TestAPIKeyAuthQuery(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
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
						"auth": {
							"type": "api-key",
							"params": {"in": "query", "name": "api_key", "value": "secret-key"}
						}
					}
				}
			}
		}
	}`
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "myapi/items?page=1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	query := rr.Last().URL.Query()
	if got := query.Get("api_key"); got != "secret-key" {
		t.Errorf("api_key: got %q, want secret-key", got)
	}
	if got := query.Get("page"); got != "1" {
		t.Errorf("page: got %q, want 1", got)
	}
}

func TestAPIKeyAuthVerboseRedactsSecret(t *testing.T) {
	c, _, errBuf := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       http.NoBody,
			Request:    r,
		}, nil
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "api-key",
							"params": {"in": "header", "name": "X-API-Key", "value": "secret-key"}
						}
					}
				}
			}
		}
	}`
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "-v", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errBuf.String()
	if strings.Contains(stderr, "secret-key") {
		t.Fatalf("verbose output leaked API key:\n%s", stderr)
	}
	if !strings.Contains(stderr, "> X-Api-Key: <redacted>") {
		t.Fatalf("expected redacted API key header, got:\n%s", stderr)
	}
}

func TestAPIAuthInspectRawHeader(t *testing.T) {
	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "http-basic",
							"params": {"username": "carol", "password": "pass123"}
						}
					}
				}
			}
		}
	}`
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--raw-header", "Authorization"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("carol:pass123"))
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Errorf("raw header output: got %q, want %q", got, want)
	}
}

func TestAuthHeaderCommandRemoved(t *testing.T) {
	cfg := `{"apis": {}}`
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	err := c.Run([]string{"restish", "auth-header", "noapi"})
	if err == nil {
		t.Fatal("expected top-level auth-header to be removed")
	}
	if strings.Contains(err.Error(), "unknown API") {
		t.Fatalf("auth-header command still appears registered: %v", err)
	}
}

func TestUnknownAuthTypeListsSupportedValues(t *testing.T) {
	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "mystery"
						}
					}
				}
			}
		}
	}`
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	err := c.Run([]string{"restish", "get", "myapi/items"})
	if err == nil {
		t.Fatal("expected unknown auth type error")
	}
	for _, want := range []string{"api-key", "http-basic", "oauth-client-credentials", "oauth-authorization-code", "oauth-device-code", "external-tool"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected supported auth type %q in error, got %v", want, err)
		}
	}
}

func TestExternalToolAuthPromptsAndStoresApproval(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command test not supported on Windows")
	}
	var rr requestRecorder
	commandLine := `echo '{"headers":{"Authorization":["Bearer tool-token"]}}'`
	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "external-tool",
							"params": {"commandline": %q}
						}
					}
				}
			}
		}
	}`, commandLine)

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "restish.json")
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	c, _, errBuf := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	c.Hooks().ConfigPath = configPath
	c.Hooks().PassReader = strings.NewReader("y\n")
	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer tool-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(errBuf.String(), "Approve external auth tool") {
		t.Fatalf("expected approval prompt, got %q", errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(configDir, "external-tool-approvals.json")); err != nil {
		t.Fatalf("expected approval cache: %v", err)
	}

	c2, _, errBuf2 := newTestCLI(t)
	useTransport(c2, func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{}`), nil
	})
	c2.Hooks().ConfigPath = configPath
	c2.Hooks().PassReader = strings.NewReader("")
	if err := c2.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("second run should reuse approval: %v", err)
	}
	if strings.Contains(errBuf2.String(), "Approve external auth tool") {
		t.Fatalf("did not expect second approval prompt, got %q", errBuf2.String())
	}
}

func TestExternalToolAuthRejectsUnapprovedCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command test not supported on Windows")
	}
	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "external-tool",
							"params": {"commandline": "true"}
						}
					}
				}
			}
		}
	}`
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{}`), nil
	})
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)
	c.Hooks().PassReader = strings.NewReader("n\n")
	err := c.Run([]string{"restish", "get", "myapi/items"})
	if err == nil {
		t.Fatal("expected unapproved command error")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("expected approval error, got %v", err)
	}
}
