package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		api, apiWarnings, err := config.ConvertLegacyAPI(name, legacy)
		if err != nil {
			return nil, nil, err
		}
		cfg.APIs[name] = api
		warnings = append(warnings, apiWarnings...)
	}
	return cfg, warnings, nil
}

func parseLegacyAPIMap(path string, data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonc.ToJSON(data), &raw); err != nil {
		return nil, &config.ParseError{Path: path, Err: err}
	}

	result := make(map[string]json.RawMessage, len(raw))
	for name, value := range raw {
		if name == "$schema" {
			continue
		}
		result[name] = value
	}
	return result, nil
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
		api, _, err := config.ConvertLegacyAPI(name, r)
		if err != nil {
			return nil, fmt.Errorf("config: parsing %s entry %q: %w", v1Path, name, err)
		}
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
