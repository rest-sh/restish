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
	// Mutually exclusive with SpecFiles; SpecFiles takes precedence when both are set.
	SpecURL string `json:"spec_url,omitempty"`
	// SpecFiles is an ordered list of local file paths or URLs to load the API
	// spec from. Multiple files are deep-merged in order (later entries win on
	// conflict). When set, network spec discovery is skipped entirely.
	SpecFiles []string `json:"spec_files,omitempty"`
	// OperationBase, when set, is used as the URL prefix for paths generated
	// from OpenAPI operations instead of base_url. Useful when the spec's
	// servers block differs from the actual base URL, or when operations are
	// served from a different host than the API root.
	OperationBase string `json:"operation_base,omitempty"`
	// Profiles is a map of profile name to profile configuration.
	Profiles map[string]*ProfileConfig `json:"profiles,omitempty"`
	// Pagination holds optional per-API pagination configuration.
	Pagination *PaginationConfig `json:"pagination,omitempty"`
}

// PaginationConfig holds per-API pagination settings.
type PaginationConfig struct {
	// ItemsPath is a filter expression that extracts the items array from the
	// response body (e.g. "data" for JSON:API, "results" for some REST APIs).
	// When empty, the body itself is used (if it is an array).
	ItemsPath string `json:"items_path,omitempty"`
	// NextPath is a filter expression that extracts the next-page URL from the
	// response body (alternative to Link header rel="next").
	NextPath string `json:"next_path,omitempty"`
}

// ProfileConfig holds per-profile overrides for an API.
type ProfileConfig struct {
	// BaseURL overrides the API-level base_url when this profile is active.
	BaseURL string `json:"base_url,omitempty"`
	// Headers is a list of persistent "Name: Value" headers sent with every request.
	Headers []string `json:"headers,omitempty"`
	// Query is a list of persistent "key=value" query params sent with every request.
	Query []string `json:"query,omitempty"`
	// TLSSigner selects a tls-signer plugin for mTLS client certificate signing.
	TLSSigner string `json:"tls_signer,omitempty"`
	// TLSSignerParams passes plugin-specific configuration to the tls-signer.
	TLSSignerParams map[string]string `json:"tls_signer_params,omitempty"`
	// Auth holds authentication configuration for this profile.
	Auth *AuthConfig `json:"auth,omitempty"`
}

// AuthConfig holds authentication configuration for a profile.
type AuthConfig struct {
	// Type identifies the auth mechanism (e.g. "http-basic", "oauth-client-credentials").
	Type string `json:"type,omitempty"`
	// Params holds handler-specific configuration, e.g. {"username": "alice"}.
	Params map[string]string `json:"params,omitempty"`
}

// CacheConfig holds cache settings.
// Additional fields are added in Step 11.
type CacheConfig struct {
	// MaxSize is the maximum cache size (e.g. "100MB"). Default: "100MB".
	MaxSize string `json:"max_size,omitempty"`
}

// configDir returns the effective config directory, honoring the
// RSH_CONFIG_DIR environment variable override.
func configDir() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return dir
	}
	return defaultDir()
}

// DefaultPath returns the path to the default config file, honoring
// the RSH_CONFIG_DIR environment variable override.
func DefaultPath() string {
	return filepath.Join(configDir(), "restish.json")
}

// DefaultTokenCachePath returns the path to the token cache file.
func DefaultTokenCachePath() string {
	return filepath.Join(configDir(), "tokens.json")
}

// DefaultSpecCacheDir returns the directory for cached API spec CBOR files.
// Spec cache files live at <dir>/<apiname>.cbor.
func DefaultSpecCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "restish")
}

// DefaultCacheDir returns the directory for cached HTTP responses,
// honoring the RSH_CACHE_DIR environment variable override.
func DefaultCacheDir() string {
	if dir := os.Getenv("RSH_CACHE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "restish", "responses")
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

// HasComments reports whether the config file at path contains JSONC comments.
// Returns false when the file does not exist or cannot be read.
// Use this before calling Save to warn users that comments will be lost.
func HasComments(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(jsonc.ToJSON(data)) != string(data)
}

// Save serialises cfg as indented JSON and writes it to path, creating the
// parent directory if necessary.  Existing JSONC comments are not preserved.
func Save(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
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
