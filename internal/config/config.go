package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/tidwall/jsonc"
)

// Config is the top-level configuration for Restish, loaded from restish.json.
// Fields are added incrementally as steps are implemented.
type Config struct {
	// APIs is a map of short API name to per-API configuration.
	APIs map[string]*APIConfig `json:"apis,omitempty"`

	// Cache holds global cache settings.
	Cache CacheConfig `json:"cache,omitempty"`

	// Theme customizes syntax highlighting for readable terminal output.
	// Keys are Chroma token names or Restish theme aliases; values are Chroma
	// style descriptors such as "#afd787" or "bold #ff5f87".
	Theme map[string]string `json:"theme,omitempty"`

	// Plugins holds per-plugin configuration keyed by plugin name (without the
	// "restish-" prefix). Each value is stored as raw JSON so that restish
	// itself does not need to know the shape of each plugin's config.
	// Plugins can read their config via the "config-read" message.
	//
	// Example restish.json entry:
	//   "plugins": {
	//     "bulk": { "concurrency": 4, "retry": true }
	//   }
	Plugins map[string]json.RawMessage `json:"plugins,omitempty"`

	// Migration describes a one-time v1 -> v2 config migration that happened
	// while loading this config. It is not persisted back into restish.json.
	Migration *MigrationInfo `json:"-"`
}

// APIConfig holds per-API configuration.
type APIConfig struct {
	// BaseURL is the base URL for all requests to this API.
	BaseURL string `json:"base_url,omitempty"`
	// SpecURL is the URL of the OpenAPI spec for this API (optional).
	// Mutually exclusive with SpecFiles; SpecFiles takes precedence when both are set.
	SpecURL string `json:"spec_url,omitempty"`
	// AllowCrossOriginSpec permits discovery from Link-header spec URLs on
	// hosts other than base_url. Private, loopback, link-local, and
	// unspecified IP literal targets are still rejected.
	AllowCrossOriginSpec bool `json:"allow_cross_origin_spec,omitempty"`
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

// DefaultPath returns the path to the default config file, honoring
// the RSH_CONFIG_DIR and XDG environment variable overrides.
func DefaultPath() string {
	return NewPaths().ConfigFile()
}

// DefaultTokenCachePath returns the path to the token cache file.
func DefaultTokenCachePath() string {
	return NewPaths().TokenCache()
}

// DefaultSpecCacheDir returns the directory for cached API spec CBOR files.
// Spec cache files live at <dir>/<apiname>.cbor.
func DefaultSpecCacheDir() string {
	return NewPaths().SpecCache()
}

// DefaultCacheDir returns the directory for cached HTTP responses,
// honoring the RSH_CACHE_DIR and XDG environment variable overrides.
func DefaultCacheDir() string {
	return NewPaths().Cache()
}

// NeedsPatchToPreserveFormatting reports whether the config file at path
// contains JSONC comments and should use patch-based writes to preserve formatting.
// Returns false when the file does not exist or cannot be read.
func NeedsPatchToPreserveFormatting(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(jsonc.ToJSON(data)) != string(data)
}

// ConfigFileHasInsecurePermissions reports whether path is readable by group or others.
// On Windows this returns false because Unix permission bits are not authoritative.
func ConfigFileHasInsecurePermissions(path string) (bool, error) {
	if runtime.GOOS == "windows" {
		return false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().Perm()&0o077 != 0, nil
}

// Save serialises cfg as indented JSON and writes it to path, creating the
// parent directory if necessary.  Existing JSONC comments are not preserved.
func Save(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return atomicWriteFile(path, append(data, '\n'), 0o600, 0o700)
}

// Load reads and parses the JSONC config file at path.
// If the file does not exist, an empty default Config is returned without error —
// a missing config file is normal for first-time users.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if filepath.Clean(path) != filepath.Clean(DefaultPath()) {
			return &Config{}, nil
		}
		return loadOrMigrate(path)
	}
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	return parseConfigBytes(path, data)
}

func parseConfigBytes(path string, data []byte) (*Config, error) {

	// Strip JSONC comments before parsing so users can annotate their config.
	stripped := jsonc.ToJSON(data)

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		err = withUnknownFieldSuggestion(err, cfg)
		line, col := extractJSONErrorPosition(err, stripped)
		return nil, &ParseError{Path: path, Err: err, Line: line, Column: col}
	}
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing content")
		}
		line, col := extractJSONErrorPosition(err, stripped)
		return nil, &ParseError{Path: path, Err: err, Line: line, Column: col}
	}

	return &cfg, nil
}

// extractJSONErrorPosition attempts to extract line:column from a JSON decode error.
// Returns (0, 0) if the position cannot be determined.
func extractJSONErrorPosition(err error, data []byte) (int, int) {
	// json.SyntaxError has Offset field in Go 1.11+
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		line, col := byteOffsetToLineColumn(data, syntaxErr.Offset)
		return line, col
	}
	return 0, 0
}

func withUnknownFieldSuggestion(err error, cfg Config) error {
	const prefix = "json: unknown field "
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return err
	}
	field := strings.Trim(msg[len(prefix):], `"`)
	best := closestJSONTag(field, reflect.TypeOf(cfg))
	if best == "" {
		return err
	}
	return fmt.Errorf("%w (did you mean %q?)", err, best)
}

func closestJSONTag(input string, t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	best := ""
	bestDistance := 3
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		d := levenshteinDistance(strings.ToLower(input), strings.ToLower(name))
		if d < bestDistance {
			bestDistance = d
			best = name
		}
	}
	return best
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			insert := curr[j-1] + 1
			delete := prev[j] + 1
			replace := prev[j-1] + cost
			curr[j] = min(insert, min(delete, replace))
		}
		prev = curr
	}
	return prev[len(b)]
}

// ParseError is returned when the config file contains invalid JSON or
// an unrecognized field. It includes line:column position when available.
type ParseError struct {
	Path   string
	Err    error
	Line   int
	Column int
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("config: invalid config %s:%d:%d: %v", e.Path, e.Line, e.Column, e.Err)
	}
	return fmt.Sprintf("config: invalid config %s\n  %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

// byteOffsetToLineColumn translates a byte offset in data to a 1-indexed line:column.
func byteOffsetToLineColumn(data []byte, offset int64) (line int, col int) {
	line = 1
	col = 1
	for i := 0; i < int(offset) && i < len(data); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return
}

func atomicWriteFile(path string, data []byte, fileMode os.FileMode, dirMode os.FileMode) error {
	lock, err := lockConfigFile(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	return atomicWriteFileLocked(path, data, fileMode, dirMode, lock)
}

// LockSiblingFile acquires the sibling advisory lock used for config-style
// read-modify-write operations on path. Call Close on the returned closer to
// release the lock.
func LockSiblingFile(path string) (io.Closer, error) {
	return lockConfigFile(path)
}

func atomicWriteFileLocked(path string, data []byte, fileMode os.FileMode, dirMode os.FileMode, lock *fileLock) error {
	_ = lock
	dir := filepath.Dir(path)

	// Check if directory exists before creating
	_, err := os.Stat(dir)
	dirExists := err == nil

	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}

	// Only chmod if we just created the directory
	if !dirExists {
		if err := os.Chmod(dir, dirMode); err != nil {
			return fmt.Errorf("config: chmod dir: %w", err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("config: temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("config: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("config: write temp file: %w", err)
	}
	// Sync temp file to ensure durability before close
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("config: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("config: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("config: rename temp file: %w", err)
	}

	// Sync parent directory after rename to ensure durability on all filesystems
	dirFile, err := os.Open(dir)
	if err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}

// PluginConfig unmarshals the configuration stored under plugins[name] into v.
// Returns nil without modifying v when no config exists for that plugin.
// name should be the plugin's short name (without the "restish-" prefix).
func (c *Config) PluginConfig(name string, v any) error {
	if c == nil || c.Plugins == nil {
		return nil
	}
	raw, ok := c.Plugins[name]
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, v)
}
