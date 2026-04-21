package cli_test

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// TestBasicAuthHeader verifies that an http-basic profile sends the correct
// Authorization: Basic header on every request.
func TestBasicAuthHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI()
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
	c.ConfigPath = writeAPIConfig(t, cfg)

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
	c, _, errBuf := newTestCLI()
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
	c.ConfigPath = writeAPIConfig(t, cfg)
	// Provide the password via PassReader (keeps Stdin free for body reads).
	c.PassReader = strings.NewReader("hunter2\n")

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

// TestAuthHeaderCommand verifies that "restish auth-header <api>" prints the
// Authorization header value for the named API's active profile.
func TestAuthHeaderCommand(t *testing.T) {
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
	c, out, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "auth-header", "myapi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("carol:pass123"))
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Errorf("auth-header output: got %q, want %q", got, want)
	}
}

// TestAuthHeaderCommandUnknownAPI verifies that auth-header returns an error
// for an unregistered API name.
func TestAuthHeaderCommandUnknownAPI(t *testing.T) {
	cfg := `{"apis": {}}`
	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)

	err := c.Run([]string{"restish", "auth-header", "noapi"})
	if err == nil {
		t.Fatal("expected error for unknown API, got nil")
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
	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)

	err := c.Run([]string{"restish", "get", "myapi/items"})
	if err == nil {
		t.Fatal("expected unknown auth type error")
	}
	for _, want := range []string{"http-basic", "oauth-client-credentials", "oauth-authorization-code", "oauth-device-code", "external-tool"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected supported auth type %q in error, got %v", want, err)
		}
	}
}
