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
// Additional fields (profiles, auth, spec discovery) are added in later steps.
type APIConfig struct {
	// BaseURL is the base URL for all requests to this API.
	BaseURL string `json:"base_url,omitempty"`
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
