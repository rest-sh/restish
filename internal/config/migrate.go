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

	"github.com/rest-sh/restish/v2/config"
	"github.com/rest-sh/restish/v2/internal/fileutil"
	"github.com/tidwall/jsonc"
)

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
	Cert   string              `json:"cert,omitempty"`
	Key    string              `json:"key,omitempty"`
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

func loadOrMigrate(path string) (*config.Config, error) {
	source, err := findLegacyConfigSource()
	if err != nil || source == nil {
		return &config.Config{}, err
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

	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}

	if userDir, err := os.UserConfigDir(); err == nil && userDir != "" {
		dir := filepath.Join(userDir, "restish")
		add(dir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, "Library", "Application Support", "restish"))
		add(filepath.Join(home, ".config", "restish"))
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

func migrateLegacyConfig(path string, source *legacyConfigSource) (*config.Config, error) {
	cfg, warnings, err := parseLegacyConfig(source)
	if err != nil {
		return nil, err
	}

	backupDir, err := prepareLegacyBackup(source, source.dir+".bak.v1")
	if err != nil {
		return nil, err
	}

	data, err := renderMigratedConfig(cfg, backupDir)
	if err != nil {
		return nil, err
	}
	if err := fileutil.AtomicWriteFile(path, data, fileutil.AtomicWriteOptions{FileMode: 0o600, DirMode: 0o700}); err != nil {
		return nil, err
	}

	loaded, err := config.ParseConfigBytes(path, data)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, cleanupLegacyFiles(source)...)
	loaded.Migration = &config.MigrationInfo{
		SourcePath: source.dir,
		BackupPath: backupDir,
		Warnings:   warnings,
	}
	return loaded, nil
}

func parseLegacyConfig(source *legacyConfigSource) (*config.Config, []string, error) {
	cfg := &config.Config{}
	if len(source.apisData) == 0 {
		return cfg, nil, nil
	}

	raw, err := parseLegacyAPIMap(source.apisPath, source.apisData)
	if err != nil {
		return nil, nil, err
	}
	if len(raw) == 0 {
		return cfg, nil, nil
	}

	var warnings []string
	cfg.APIs = make(map[string]*config.APIConfig, len(raw))
	for name, legacy := range raw {
		api, apiWarnings := convertLegacyAPIConfig(name, legacy)
		cfg.APIs[name] = api
		warnings = append(warnings, apiWarnings...)
	}
	return cfg, warnings, nil
}

func parseLegacyAPIMap(path string, data []byte) (map[string]*legacyAPIConfig, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonc.ToJSON(data), &raw); err != nil {
		return nil, &config.ParseError{Path: path, Err: err}
	}

	result := make(map[string]*legacyAPIConfig, len(raw))
	for name, value := range raw {
		if name == "$schema" {
			continue
		}
		var cfg legacyAPIConfig
		if err := json.Unmarshal(value, &cfg); err != nil {
			return nil, &config.ParseError{Path: path, Err: err}
		}
		result[name] = &cfg
	}
	return result, nil
}

func convertLegacyAPIConfig(name string, legacy *legacyAPIConfig) (*config.APIConfig, []string) {
	if legacy == nil {
		return &config.APIConfig{}, nil
	}
	operationBase, warning := convertLegacyOperationBase(name, legacy.OperationBase)

	api := &config.APIConfig{
		BaseURL:       legacy.Base,
		OperationBase: operationBase,
		SpecFiles:     append([]string(nil), legacy.SpecFiles...),
	}
	if len(legacy.Profiles) > 0 {
		api.Profiles = make(map[string]*config.ProfileConfig, len(legacy.Profiles))
		for name, profile := range legacy.Profiles {
			api.Profiles[name] = convertLegacyProfile(profile)
		}
	}

	var migratedPKCS11 bool
	if legacy.TLS != nil && legacy.TLS.PKCS11 != nil {
		if api.Profiles == nil {
			api.Profiles = map[string]*config.ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &config.ProfileConfig{}
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
		migratedPKCS11 = true
	}

	if legacy.TLS != nil && (legacy.TLS.Cert != "" || legacy.TLS.Key != "") {
		if api.Profiles == nil {
			api.Profiles = map[string]*config.ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &config.ProfileConfig{}
			api.Profiles["default"] = prof
		}
		if prof.ClientCertPath == "" && legacy.TLS.Cert != "" {
			prof.ClientCertPath = legacy.TLS.Cert
		}
		if prof.ClientKeyPath == "" && legacy.TLS.Key != "" {
			prof.ClientKeyPath = legacy.TLS.Key
		}
	}

	var warnings []string
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if migratedPKCS11 {
		warnings = append(warnings, fmt.Sprintf("api %q: migrated PKCS#11 TLS config; install the restish-pkcs11 plugin to continue using it (see https://github.com/rest-sh/restish)", name))
	}
	return api, warnings
}

func convertLegacyOperationBase(apiName, operationBase string) (string, string) {
	if operationBase == "" {
		return "", ""
	}
	if err := config.ValidateOperationBase(operationBase); err == nil {
		return operationBase, ""
	}
	return "", fmt.Sprintf("api %q: dropped invalid legacy operation_base %q; v2 operation_base must be an absolute path", apiName, operationBase)
}

func convertLegacyProfile(legacy *legacyAPIProfile) *config.ProfileConfig {
	if legacy == nil {
		return &config.ProfileConfig{}
	}

	prof := &config.ProfileConfig{
		BaseURL: legacy.Base,
		Headers: sortedHeaderList(legacy.Headers),
		Query:   sortedQueryList(legacy.Query),
	}
	if legacy.Auth != nil {
		prof.Auth = &config.AuthConfig{
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

func prepareLegacyBackup(source *legacyConfigSource, preferredDir string) (string, error) {
	if _, err := os.Stat(preferredDir); err == nil {
		if ok, matchErr := legacyBackupMatches(source, preferredDir); matchErr != nil {
			return "", matchErr
		} else if ok {
			return preferredDir, nil
		}
		backupDir, err := nextLegacyBackupDir(preferredDir)
		if err != nil {
			return "", err
		}
		if err := backupLegacyFiles(source, backupDir); err != nil {
			return "", err
		}
		return backupDir, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("config: check backup directory %s: %w", preferredDir, err)
	}
	if err := backupLegacyFiles(source, preferredDir); err != nil {
		return "", err
	}
	return preferredDir, nil
}

func legacyBackupMatches(source *legacyConfigSource, backupDir string) (bool, error) {
	for _, file := range legacyBackupFiles(source) {
		if len(file.data) == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(backupDir, file.name))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("config: read existing backup %s: %w", file.name, err)
		}
		if !bytes.Equal(data, file.data) {
			return false, nil
		}
	}
	return true, nil
}

func nextLegacyBackupDir(preferredDir string) (string, error) {
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s.%d", preferredDir, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("config: check backup directory %s: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("config: cannot find available v1 backup directory near %s; move old backup directories and retry migration", preferredDir)
}

func legacyBackupFiles(source *legacyConfigSource) []struct {
	name string
	path string
	data []byte
} {
	return []struct {
		name string
		path string
		data []byte
	}{
		{name: "apis.json", path: source.apisPath, data: source.apisData},
		{name: "config.json", path: source.configPath, data: source.configData},
	}
}

func backupLegacyFiles(source *legacyConfigSource, backupDir string) error {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("config: backup mkdir: %w", err)
	}

	for _, file := range legacyBackupFiles(source) {
		if len(file.data) == 0 {
			continue
		}
		target := filepath.Join(backupDir, file.name)
		if err := fileutil.AtomicWriteFile(target, file.data, fileutil.AtomicWriteOptions{FileMode: 0o600, DirMode: 0o700}); err != nil {
			return fmt.Errorf("config: backup %s: %w", file.name, err)
		}
	}
	return nil
}

func cleanupLegacyFiles(source *legacyConfigSource) []string {
	var warnings []string
	for _, file := range legacyBackupFiles(source) {
		if len(file.data) == 0 {
			continue
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, fmt.Sprintf("could not remove legacy %s after migration: %v", file.path, err))
		}
	}
	return warnings
}

func renderMigratedConfig(cfg *config.Config, backupDir string) ([]byte, error) {
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("config: marshal migrated config: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("// Migrated from Restish v1.\n")
	fmt.Fprintf(&out, "// Original v1 files were copied to %s.\n", backupDir)
	out.WriteString("// Secrets are intentionally not duplicated in comments.\n")
	out.Write(body)
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// ReadProfile loads the named API from folder, returning its v2-shaped
// *config.APIConfig. It tries restish.json (v2) first and falls back to
// apis.json (v1), converting legacy entries on the fly.
func ReadProfile(folder, apiName string) (*config.APIConfig, error) {
	all, err := ReadAll(folder)
	if err != nil {
		return nil, err
	}
	api, ok := all[apiName]
	if !ok {
		return nil, fmt.Errorf("config: api %q not found in %s", apiName, folder)
	}
	return api, nil
}

// ReadAll returns every API defined in folder, in v2 shape. Tries
// restish.json first, then apis.json, converting legacy entries.
func ReadAll(folder string) (map[string]*config.APIConfig, error) {
	v2Path := filepath.Join(folder, "restish.json")
	data, err := os.ReadFile(v2Path)
	if err == nil {
		cfg, err := config.ParseConfigBytes(v2Path, data)
		if err != nil {
			return nil, err
		}
		out := make(map[string]*config.APIConfig, len(cfg.APIs))
		for name, api := range cfg.APIs {
			if api != nil {
				out[name] = api
			}
		}
		return out, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: cannot read %s: %w", v2Path, err)
	}

	v1Path := filepath.Join(folder, "apis.json")
	data, err = os.ReadFile(v1Path)
	if err != nil {
		return nil, fmt.Errorf("config: no restish config found in %s (tried restish.json and apis.json)", folder)
	}

	stripped := jsonc.ToJSON(data)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stripped, &raw); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", v1Path, err)
	}
	out := make(map[string]*config.APIConfig, len(raw))
	for name, r := range raw {
		if strings.HasPrefix(name, "$") {
			continue
		}
		var v1 legacyAPIConfig
		if err := json.Unmarshal(r, &v1); err != nil {
			return nil, fmt.Errorf("config: parsing %s entry %q: %w", v1Path, name, err)
		}
		api, _ := convertLegacyAPIConfig(name, &v1)
		out[name] = api
	}
	return out, nil
}

// TryMigrate inspects the user's config directory for a legacy v1 config
// (apis.json / config.json) and migrates it to a v2 restish.json at path.
// Returns (nil, nil) when no legacy source is present. The restish CLI
// calls this explicitly before config.Load; embedders using config.Load
// do not see a migration triggered automatically.
func TryMigrate(path string) (*config.MigrationInfo, error) {
	source, err := findLegacyConfigSource()
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, nil
	}
	loaded, err := migrateLegacyConfig(path, source)
	if err != nil {
		return nil, err
	}
	return loaded.Migration, nil
}
