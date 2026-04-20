package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tidwall/jsonc"
)

// MigrationInfo describes a one-time v1 -> v2 config migration performed while
// loading restish.json.
type MigrationInfo struct {
	SourcePath string
	BackupPath string
}

type legacyConfigSource struct {
	dir        string
	apisPath   string
	configPath string
	apisData   []byte
	configData []byte
}

type legacyAPIAuth struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

type legacyPKCS11Config struct {
	Path  string `json:"path,omitempty"`
	Label string `json:"label,omitempty"`
}

type legacyTLSConfig struct {
	PKCS11 *legacyPKCS11Config `json:"pkcs11,omitempty"`
}

type legacyAPIProfile struct {
	Base    string            `json:"base,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Auth    *legacyAPIAuth    `json:"auth,omitempty"`
}

type legacyAPIConfig struct {
	Base          string                       `json:"base"`
	OperationBase string                       `json:"operation_base,omitempty"`
	SpecFiles     []string                     `json:"spec_files,omitempty"`
	Profiles      map[string]*legacyAPIProfile `json:"profiles,omitempty"`
	TLS           *legacyTLSConfig             `json:"tls,omitempty"`
}

func loadOrMigrate(path string) (*Config, error) {
	source, err := findLegacyConfigSource()
	if err != nil || source == nil {
		return &Config{}, err
	}

	cfg, err := migrateLegacyConfig(path, source)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func findLegacyConfigSource() (*legacyConfigSource, error) {
	for _, dir := range legacyConfigDirs() {
		source, err := loadLegacyConfigSource(dir)
		if err != nil {
			return nil, err
		}
		if source != nil {
			return source, nil
		}
	}
	return nil, nil
}

func legacyConfigDirs() []string {
	seen := map[string]bool{}
	var dirs []string

	if userDir, err := os.UserConfigDir(); err == nil && userDir != "" {
		dir := filepath.Join(userDir, "restish")
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".config", "restish")
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func loadLegacyConfigSource(dir string) (*legacyConfigSource, error) {
	source := &legacyConfigSource{
		dir:        dir,
		apisPath:   filepath.Join(dir, "apis.json"),
		configPath: filepath.Join(dir, "config.json"),
	}

	var found bool
	if data, err := os.ReadFile(source.apisPath); err == nil {
		source.apisData = data
		found = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: cannot read %s: %w", source.apisPath, err)
	}
	if data, err := os.ReadFile(source.configPath); err == nil {
		source.configData = data
		found = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: cannot read %s: %w", source.configPath, err)
	}
	if !found {
		return nil, nil
	}
	return source, nil
}

func migrateLegacyConfig(path string, source *legacyConfigSource) (*Config, error) {
	cfg, err := parseLegacyConfig(source)
	if err != nil {
		return nil, err
	}

	backupDir := source.dir + ".bak.v1"
	if err := backupLegacyFiles(source, backupDir); err != nil {
		return nil, err
	}

	data, err := renderMigratedConfig(cfg, source)
	if err != nil {
		return nil, err
	}
	if err := atomicWriteFile(path, data, 0o600, 0o700); err != nil {
		return nil, err
	}

	loaded, err := parseConfigBytes(path, data)
	if err != nil {
		return nil, err
	}
	loaded.Migration = &MigrationInfo{
		SourcePath: source.dir,
		BackupPath: backupDir,
	}
	return loaded, nil
}

func parseLegacyConfig(source *legacyConfigSource) (*Config, error) {
	cfg := &Config{}
	if len(source.apisData) == 0 {
		return cfg, nil
	}

	raw, err := parseLegacyAPIMap(source.apisPath, source.apisData)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return cfg, nil
	}

	cfg.APIs = make(map[string]*APIConfig, len(raw))
	for name, legacy := range raw {
		cfg.APIs[name] = convertLegacyAPIConfig(legacy)
	}
	return cfg, nil
}

func parseLegacyAPIMap(path string, data []byte) (map[string]*legacyAPIConfig, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonc.ToJSON(data), &raw); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}

	result := make(map[string]*legacyAPIConfig, len(raw))
	for name, value := range raw {
		if name == "$schema" {
			continue
		}
		var cfg legacyAPIConfig
		if err := json.Unmarshal(value, &cfg); err != nil {
			return nil, &ParseError{Path: path, Err: err}
		}
		result[name] = &cfg
	}
	return result, nil
}

func convertLegacyAPIConfig(legacy *legacyAPIConfig) *APIConfig {
	if legacy == nil {
		return &APIConfig{}
	}

	api := &APIConfig{
		BaseURL:       legacy.Base,
		OperationBase: legacy.OperationBase,
		SpecFiles:     append([]string(nil), legacy.SpecFiles...),
	}
	if len(legacy.Profiles) > 0 {
		api.Profiles = make(map[string]*ProfileConfig, len(legacy.Profiles))
		for name, profile := range legacy.Profiles {
			api.Profiles[name] = convertLegacyProfile(profile)
		}
	}

	if legacy.TLS != nil && legacy.TLS.PKCS11 != nil {
		if api.Profiles == nil {
			api.Profiles = map[string]*ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &ProfileConfig{}
			api.Profiles["default"] = prof
		}
		if prof.TLSSigner == "" {
			prof.TLSSigner = "pkcs11"
		}
		if prof.TLSSignerParams == nil {
			prof.TLSSignerParams = map[string]string{}
		}
		if legacy.TLS.PKCS11.Path != "" && prof.TLSSignerParams["path"] == "" {
			prof.TLSSignerParams["path"] = legacy.TLS.PKCS11.Path
		}
		if legacy.TLS.PKCS11.Label != "" && prof.TLSSignerParams["label"] == "" {
			prof.TLSSignerParams["label"] = legacy.TLS.PKCS11.Label
		}
	}

	return api
}

func convertLegacyProfile(legacy *legacyAPIProfile) *ProfileConfig {
	if legacy == nil {
		return &ProfileConfig{}
	}

	prof := &ProfileConfig{
		BaseURL: legacy.Base,
		Headers: sortedHeaderList(legacy.Headers),
		Query:   sortedQueryList(legacy.Query),
	}
	if legacy.Auth != nil {
		prof.Auth = &AuthConfig{
			Type:   legacy.Auth.Name,
			Params: cloneStringMap(legacy.Auth.Params),
		}
	}
	return prof
}

func sortedHeaderList(values map[string]string) []string {
	return sortedPairs(values, ": ")
}

func sortedQueryList(values map[string]string) []string {
	return sortedPairs(values, "=")
}

func sortedPairs(values map[string]string, sep string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key+sep+values[key])
	}
	return items
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func backupLegacyFiles(source *legacyConfigSource, backupDir string) error {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("config: backup mkdir: %w", err)
	}
	if err := os.Chmod(backupDir, 0o700); err != nil {
		return fmt.Errorf("config: backup chmod dir: %w", err)
	}

	for _, file := range []struct {
		name string
		data []byte
	}{
		{name: "apis.json", data: source.apisData},
		{name: "config.json", data: source.configData},
	} {
		if len(file.data) == 0 {
			continue
		}
		target := filepath.Join(backupDir, file.name)
		if err := os.WriteFile(target, file.data, 0o600); err != nil {
			return fmt.Errorf("config: backup %s: %w", file.name, err)
		}
	}
	return nil
}

func renderMigratedConfig(cfg *Config, source *legacyConfigSource) ([]byte, error) {
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("config: marshal migrated config: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("// Migrated from Restish v1.\n")
	appendLegacySnapshot := func(label string, data []byte) {
		if len(data) == 0 {
			return
		}
		out.WriteString("//\n")
		out.WriteString("// Original " + label + " preserved for reference:\n")
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				out.WriteString("//\n")
				continue
			}
			out.WriteString("// " + line + "\n")
		}
	}
	appendLegacySnapshot(filepath.Base(source.apisPath), source.apisData)
	appendLegacySnapshot(filepath.Base(source.configPath), source.configData)
	out.Write(body)
	out.WriteByte('\n')
	return out.Bytes(), nil
}
