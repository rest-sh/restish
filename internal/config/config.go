package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/tidwall/jsonc"
)

// Config is the top-level configuration for Restish, loaded from restish.json.
type Config struct {
	// APIs is a map of short API name to per-API configuration.
	APIs map[string]*APIConfig `json:"apis,omitempty"`

	// AuthProfiles holds named auth configurations that API profiles can
	// reference with auth_ref.
	AuthProfiles map[string]*AuthConfig `json:"auth_profiles,omitempty"`

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
	// OperationBase, when set, is an absolute path resolved against base_url for
	// paths generated from OpenAPI operations. Useful when operation paths should
	// escape or replace a sub-path in base_url.
	OperationBase string `json:"operation_base,omitempty"`
	// ServerVariables supplies explicit values for OpenAPI server URL variables.
	// Values are used for generated operation path resolution; enum values from
	// remote specs are never expanded eagerly.
	ServerVariables map[string]string `json:"server_variables,omitempty"`
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
	// ServerVariables overrides API-level OpenAPI server URL variables for this
	// profile when generating operation paths.
	ServerVariables map[string]string `json:"server_variables,omitempty"`
	// Auth holds authentication configuration for this profile.
	Auth *AuthConfig `json:"auth,omitempty"`
	// AuthRef names a top-level auth_profiles entry to use for this profile.
	AuthRef string `json:"auth_ref,omitempty"`
}

// AuthConfig holds authentication configuration for a profile.
type AuthConfig struct {
	// Type identifies the auth mechanism (e.g. "http-basic", "oauth-client-credentials").
	Type string `json:"type,omitempty"`
	// Params holds handler-specific configuration, e.g. {"username": "alice"}.
	Params map[string]string `json:"params,omitempty"`
}

// CacheConfig holds cache settings.
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

// LoadExplicit reads a user-selected config file. Unlike Load, a missing file
// is an error because explicit config selection must not silently fall back to
// an empty or platform-default config.
func LoadExplicit(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: explicit config file %s does not exist", path)
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
	if err := Validate(&cfg); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}

	return &cfg, nil
}

// Validate checks cross-field config invariants that JSON decoding alone cannot
// enforce.
func Validate(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	for name, api := range cfg.APIs {
		if api == nil {
			continue
		}
		if err := ValidateOperationBase(api.OperationBase); err != nil {
			return fmt.Errorf("apis.%s.operation_base: %w", name, err)
		}
		if api.OperationBase != "" {
			if err := ValidateBaseURLForOperationBase(api.BaseURL); err != nil {
				return fmt.Errorf("apis.%s.base_url: %w", name, err)
			}
		}
		for profileName, prof := range api.Profiles {
			if prof == nil {
				continue
			}
			if prof.Auth != nil && prof.AuthRef != "" {
				return fmt.Errorf("apis.%s.profiles.%s: auth and auth_ref are mutually exclusive", name, profileName)
			}
			if prof.AuthRef != "" {
				if cfg.AuthProfiles == nil || cfg.AuthProfiles[prof.AuthRef] == nil {
					return fmt.Errorf("apis.%s.profiles.%s.auth_ref: unknown auth profile %q", name, profileName, prof.AuthRef)
				}
			}
		}
	}
	return nil
}

// ValidateOperationBase enforces the v2 contract that operation_base is an
// absolute URL path prefix.
func ValidateOperationBase(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be an absolute path")
	}
	if u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return fmt.Errorf("must be an absolute path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("must not include query or fragment")
	}
	return nil
}

// ValidateBaseURLForOperationBase ensures operation_base can be resolved at
// load time instead of failing later when generated commands run.
func ValidateBaseURLForOperationBase(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return fmt.Errorf("must be an absolute http/https URL when operation_base is set")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must use http or https when operation_base is set")
	}
	if u.Host == "" {
		return fmt.Errorf("must be an absolute http/https URL when operation_base is set")
	}
	return nil
}

// ResolveOperationBaseURL resolves an absolute operation_base path against the
// API base URL using the same URL-reference semantics as Restish v1.
func ResolveOperationBaseURL(baseURL, operationBase string) (string, error) {
	if operationBase == "" {
		return baseURL, nil
	}
	if err := ValidateOperationBase(operationBase); err != nil {
		return "", err
	}
	base, err := url.Parse(baseURL)
	if err != nil || !base.IsAbs() || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		if validateErr := ValidateBaseURLForOperationBase(baseURL); validateErr != nil {
			return "", validateErr
		}
	}
	return base.ResolveReference(&url.URL{Path: operationBase}).String(), nil
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
	return atomicWriteFileLocked(path, data, fileMode, dirMode)
}

// LockSiblingFile acquires the sibling advisory lock used for config-style
// read-modify-write operations on path. Call Close on the returned closer to
// release the lock.
func LockSiblingFile(path string) (io.Closer, error) {
	return lockConfigFile(path)
}

func atomicWriteFileLocked(path string, data []byte, fileMode os.FileMode, dirMode os.FileMode) error {
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
