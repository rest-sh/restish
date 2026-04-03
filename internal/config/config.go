package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/tidwall/jsonc"
)

// Config is the top-level configuration for Restish, loaded from restish.json.
// Fields are added incrementally as steps are implemented.
type Config struct {
	// APIs is a map of short API name to per-API configuration.
	APIs map[string]*APIConfig `json:"apis,omitempty"`

	// Cache holds global cache settings.
	Cache CacheConfig `json:"cache,omitempty"`
}

// APIConfig holds per-API configuration.
type APIConfig struct {
	// BaseURL is the base URL for all requests to this API.
	BaseURL string `json:"base_url,omitempty"`
	// SpecURL is the URL of the OpenAPI spec for this API (optional).
	SpecURL string `json:"spec_url,omitempty"`
	// Profiles is a map of profile name to profile configuration.
	Profiles map[string]*ProfileConfig `json:"profiles,omitempty"`
}

// ProfileConfig holds per-profile overrides for an API.
type ProfileConfig struct {
	// BaseURL overrides the API-level base_url when this profile is active.
	BaseURL string `json:"base_url,omitempty"`
	// Headers is a list of persistent "Name: Value" headers sent with every request.
	Headers []string `json:"headers,omitempty"`
	// Query is a list of persistent "key=value" query params sent with every request.
	Query []string `json:"query,omitempty"`
	// Auth holds authentication configuration for this profile.
	Auth *AuthConfig `json:"auth,omitempty"`
}

// AuthConfig holds authentication configuration for a profile.
// The full set of auth fields is populated in Steps 9–10.
type AuthConfig struct {
	// Type identifies the auth mechanism (e.g. "http-basic", "oauth-client-credentials").
	Type string `json:"type,omitempty"`
}

// CacheConfig holds cache settings.
// Additional fields are added in Step 11.
type CacheConfig struct {
	// MaxSize is the maximum cache size (e.g. "100MB"). Default: "100MB".
	MaxSize string `json:"max_size,omitempty"`
}

// DefaultPath returns the path to the default config file, honoring
// the RSH_CONFIG_DIR environment variable override.
func DefaultPath() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "restish.json")
	}
	return filepath.Join(defaultDir(), "restish.json")
}

// defaultDir returns the platform-appropriate config directory.
// macOS and Linux use ~/.config/restish (XDG convention).
// Windows uses %APPDATA%\restish.
func defaultDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "restish")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "restish")
	}
	return ".restish"
}

// Load reads and parses the JSONC config file at path.
// If the file does not exist, an empty default Config is returned without error —
// a missing config file is normal for first-time users.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	// Strip JSONC comments before parsing so users can annotate their config.
	stripped := jsonc.ToJSON(data)

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}

	return &cfg, nil
}

// ParseError is returned when the config file contains invalid JSON or
// an unrecognized field.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("config: invalid config %s\n  %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }
