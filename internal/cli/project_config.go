package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/fileutil"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/tidwall/jsonc"
)

const projectConfigFileName = ".restish.json"

type projectConfigState struct {
	Path      string
	Hash      string
	Namespace string
	Trusted   bool
	APIs      map[string]bool
}

type projectTrustFile struct {
	Projects map[string]projectTrustEntry `json:"projects,omitempty"`
}

type projectTrustEntry struct {
	SHA256    string `json:"sha256"`
	TrustedAt string `json:"trusted_at,omitempty"`
}

func (c *CLI) prepareProjectConfig(ctx context.Context, scan cliArgScan) error {
	if c.hooks.ConfigPath != "" || c.explicitConfigFile {
		return nil
	}
	if c.paths().ConfigError() != nil {
		return nil
	}
	project, err := discoverProjectConfig()
	if err != nil || project == nil {
		return err
	}
	c.projectConfig = project
	trusted, err := c.projectConfigTrusted(project)
	if err != nil {
		return err
	}
	if trusted {
		project.Trusted = true
		return nil
	}
	if scan.Bootstrap {
		return nil
	}
	if scan.FirstCommand == "config" && scan.SecondCommand == "trust" {
		return nil
	}
	if !output.IsTerminalReader(c.Stdin) {
		c.warnf("project config %s is not trusted; run %q to use it", project.Path, c.commandNameOrDefault()+" config trust")
		return nil
	}
	summary, err := c.projectConfigTrustSummary(project)
	if err != nil {
		return err
	}
	label := fmt.Sprintf("Trust and use project config %s%s? [y/N] ", project.Path, summary)
	ok, err := c.promptYesNoDefault(ctx, label, false)
	if err != nil {
		return err
	}
	if ok {
		if err := c.trustProjectConfig(project); err != nil {
			return err
		}
		project.Trusted = true
	}
	return nil
}

func discoverProjectConfig() (*projectConfigState, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(wd, projectConfigFileName)
		info, statErr := os.Stat(candidate)
		if statErr == nil {
			if info.IsDir() {
				return nil, fmt.Errorf("project config %s is a directory", candidate)
			}
			return newProjectConfigState(candidate)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return nil, statErr
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return nil, nil
		}
		wd = parent
	}
}

func newProjectConfigState(path string) (*projectConfigState, error) {
	canonical, err := canonicalProjectConfigPath(path)
	if err != nil {
		return nil, err
	}
	hash, err := projectConfigHash(canonical)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(canonical + "\x00" + hash))
	return &projectConfigState{
		Path:      canonical,
		Hash:      hash,
		Namespace: hex.EncodeToString(sum[:8]),
		APIs:      map[string]bool{},
	}, nil
}

func canonicalProjectConfigPath(path string) (string, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path), nil
}

func projectConfigHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (c *CLI) projectTrustPath() string {
	return filepath.Join(c.paths().Config(), "project-trust.json")
}

func (c *CLI) projectConfigTrusted(project *projectConfigState) (bool, error) {
	trust, err := c.loadProjectTrust()
	if err != nil {
		return false, err
	}
	entry, ok := trust.Projects[project.Path]
	if !ok {
		return false, nil
	}
	return entry.SHA256 == project.Hash, nil
}

func (c *CLI) trustProjectConfig(project *projectConfigState) error {
	if project == nil {
		return fmt.Errorf("project config: no %s found from the current directory", projectConfigFileName)
	}
	projectCfg, _, err := c.projectConfigRuntimeSubset(project)
	if err != nil {
		return err
	}
	if err := c.validateProjectConfigWithBase(project, projectCfg); err != nil {
		return err
	}
	trust, err := c.loadProjectTrust()
	if err != nil {
		return err
	}
	if trust.Projects == nil {
		trust.Projects = map[string]projectTrustEntry{}
	}
	trust.Projects[project.Path] = projectTrustEntry{
		SHA256:    project.Hash,
		TrustedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(trust, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileutil.AtomicWriteFile(c.projectTrustPath(), data, fileutil.AtomicWriteOptions{
		FileMode: 0o600,
		DirMode:  0o700,
	})
}

func (c *CLI) loadProjectTrust() (projectTrustFile, error) {
	path := c.projectTrustPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return projectTrustFile{Projects: map[string]projectTrustEntry{}}, nil
	}
	if err != nil {
		return projectTrustFile{}, err
	}
	var trust projectTrustFile
	if err := json.Unmarshal(data, &trust); err != nil {
		return projectTrustFile{}, fmt.Errorf("project config trust: cannot parse %s: %w", path, err)
	}
	if trust.Projects == nil {
		trust.Projects = map[string]projectTrustEntry{}
	}
	return trust, nil
}

func (c *CLI) loadTrustedProjectConfig() (*config.Config, error) {
	project := c.projectConfig
	if project == nil || !project.Trusted {
		return nil, nil
	}
	currentHash, err := projectConfigHash(project.Path)
	if err != nil {
		return nil, err
	}
	if currentHash != project.Hash {
		return nil, fmt.Errorf("project config %s changed since it was trusted; run %q again to review and trust the new contents", project.Path, c.commandNameOrDefault()+" config trust")
	}
	projectCfg, apiNames, err := c.projectConfigRuntimeSubset(project)
	if err != nil {
		return nil, err
	}
	project.APIs = apiNames
	return projectCfg, nil
}

func (c *CLI) projectConfigTrustSummary(project *projectConfigState) (string, error) {
	projectCfg, apiNames, err := c.projectConfigRuntimeSubset(project)
	if err != nil {
		return "", err
	}
	var parts []string
	if len(apiNames) > 0 {
		parts = append(parts, fmt.Sprintf("APIs: %s", strings.Join(sortedProjectAPINames(apiNames), ", ")))
	}
	if len(projectCfg.Theme) > 0 {
		parts = append(parts, fmt.Sprintf("theme overrides: %d", len(projectCfg.Theme)))
	}
	if len(parts) == 0 {
		return " (no APIs or theme overrides)", nil
	}
	return " (" + strings.Join(parts, "; ") + ")", nil
}

func (c *CLI) projectConfigRuntimeSubset(project *projectConfigState) (*config.Config, map[string]bool, error) {
	if project == nil {
		return nil, nil, nil
	}
	if insecure, permErr := config.ConfigFileHasInsecurePermissions(project.Path); permErr == nil && insecure {
		return nil, nil, fmt.Errorf("%s is group/world-readable; project config can contain credentials (chmod 600)", project.Path)
	}
	cfg, err := parseProjectConfigFile(project.Path)
	if err != nil {
		return nil, nil, err
	}
	if projectConfigHasUnsupportedTopLevel(cfg) {
		return nil, nil, fmt.Errorf("project config: only apis and theme are supported in %s for now", projectConfigFileName)
	}
	subset := &config.Config{}
	apiNames := map[string]bool{}
	if len(cfg.APIs) > 0 {
		subset.APIs = map[string]*config.APIConfig{}
		baseDir := filepath.Dir(project.Path)
		for name, apiCfg := range cfg.APIs {
			cloned, err := cloneAPIConfig(apiCfg)
			if err != nil {
				return nil, nil, err
			}
			resolveProjectAPIPaths(cloned, baseDir)
			subset.APIs[name] = cloned
			apiNames[name] = true
		}
	}
	if len(cfg.Theme) > 0 {
		subset.Theme = map[string]string{}
		for key, value := range cfg.Theme {
			subset.Theme[key] = value
		}
	}
	return subset, apiNames, nil
}

func parseProjectConfigFile(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}
	stripped := jsonc.ToJSON(data)
	var cfg config.Config
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: invalid config %s\n  %w", path, err)
	}
	if err := dec.Decode(new(struct{})); err == nil {
		return nil, fmt.Errorf("config: invalid config %s\n  unexpected trailing content", path)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("config: invalid config %s\n  %w", path, err)
	}
	return &cfg, nil
}

func (c *CLI) validateProjectConfigWithBase(project *projectConfigState, projectCfg *config.Config) error {
	base, err := c.loadBaseConfig()
	if err != nil {
		return err
	}
	merged := mergeDefaultConfigForEmbedding(c.defaultConfig, base)
	merged = mergeProjectConfig(merged, projectCfg)
	if err := config.Validate(merged); err != nil {
		return fmt.Errorf("project config %s: %w", project.Path, err)
	}
	return nil
}

func projectConfigHasUnsupportedTopLevel(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.AuthProfiles) > 0 ||
		cfg.Cache != (config.CacheConfig{}) ||
		len(cfg.Plugins) > 0 ||
		cfg.ThemeSource != ""
}

func resolveProjectAPIPaths(apiCfg *config.APIConfig, baseDir string) {
	if apiCfg == nil {
		return
	}
	for i, file := range apiCfg.SpecFiles {
		if file == "" || strings.Contains(file, "://") || filepath.IsAbs(file) {
			continue
		}
		apiCfg.SpecFiles[i] = filepath.Join(baseDir, file)
	}
}

func mergeProjectConfig(base, project *config.Config) *config.Config {
	if project == nil {
		return base
	}
	if base == nil {
		base = &config.Config{}
	}
	if len(project.APIs) > 0 {
		if base.APIs == nil {
			base.APIs = map[string]*config.APIConfig{}
		}
		for name, apiCfg := range project.APIs {
			base.APIs[name] = apiCfg
		}
	}
	if len(project.Theme) > 0 {
		if base.Theme == nil {
			base.Theme = map[string]string{}
		}
		for key, value := range project.Theme {
			base.Theme[key] = value
		}
	}
	return base
}

func (c *CLI) projectAPI(apiName string) bool {
	return c.projectConfig != nil && c.projectConfig.Trusted && c.projectConfig.APIs[apiName]
}

func (c *CLI) hasProjectAPIs() bool {
	return c.projectConfig != nil && c.projectConfig.Trusted && len(c.projectConfig.APIs) > 0
}

func (c *CLI) ensureMutableAPI(apiName string) error {
	if c.projectAPI(apiName) {
		return fmt.Errorf("API %q comes from trusted project config %s and is read-only; edit that file directly or pass --rsh-config %s to make it the selected config", apiName, c.projectConfig.Path, c.projectConfig.Path)
	}
	return nil
}

func (c *CLI) apiStateName(apiName string) string {
	if c.projectAPI(apiName) {
		return "project-" + c.projectConfig.Namespace + "-" + apiName
	}
	return apiName
}

func (c *CLI) apiCacheNamespace(apiName, profileName string) string {
	return c.apiStateName(apiName) + ":" + profileName
}

func (c *CLI) configSourceSummary() string {
	if c.projectConfig != nil && c.projectConfig.Trusted {
		return fmt.Sprintf("%s + %s", c.configFilePath(), c.projectConfig.Path)
	}
	return c.configFilePath()
}

func sortedProjectAPINames(items map[string]bool) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
