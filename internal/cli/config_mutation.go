package cli

import (
	"fmt"
	"path/filepath"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

func (c *CLI) requireAPI(apiName string) (*config.APIConfig, error) {
	if c.cfg == nil || c.cfg.APIs == nil || c.cfg.APIs[apiName] == nil {
		return nil, fmt.Errorf("unknown API %q", apiName)
	}
	return c.cfg.APIs[apiName], nil
}

func (c *CLI) saveConfigValues(label string, ops []config.ConfigPatchOperation) error {
	oldCfg := c.cfg
	if err := config.SaveConfigValues(c.configFilePath(), ops); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation(label, oldCfg)
}

func (c *CLI) saveAPIConfig(label, apiName string, cfg *config.Config, apiCfg *config.APIConfig) error {
	oldCfg := c.cfg
	cfgPath := c.configFilePath()
	if config.NeedsPatchToPreserveFormatting(cfgPath) {
		if err := config.SaveAPIConfig(cfgPath, apiName, apiCfg); err != nil {
			return err
		}
	} else {
		if cfg == nil {
			cfg = &config.Config{}
		}
		if cfg.APIs == nil {
			cfg.APIs = map[string]*config.APIConfig{}
		}
		cfg.APIs[apiName] = apiCfg
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
	}
	return c.reloadConfigAfterMutation(label, oldCfg)
}

func (c *CLI) deleteAPIConfig(label, apiName string, cfg *config.Config, oldCfg *config.Config) error {
	cfgPath := c.configFilePath()
	if config.NeedsPatchToPreserveFormatting(cfgPath) {
		if err := config.DeleteAPIConfig(cfgPath, apiName); err != nil {
			return err
		}
	} else if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation(label, oldCfg)
}

func (c *CLI) saveThemeConfig(entries map[string]string, source string) error {
	oldCfg := c.cfg
	cfgPath := c.configFilePath()
	if err := config.SaveConfigValues(cfgPath, []config.ConfigPatchOperation{
		{Path: []string{"theme"}, Value: entries},
		{Path: []string{"theme_source"}, Value: source},
	}); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation("config theme set", oldCfg)
}

func (c *CLI) printConfigWrittenPath() {
	cfgPath := c.configFilePath()
	if abs, err := filepath.Abs(cfgPath); err == nil {
		cfgPath = abs
	}
	fmt.Fprintf(c.Stdout, "Wrote config: %s\n", cfgPath)
}

func (c *CLI) reloadConfigAfterMutation(label string, oldCfg *config.Config) error {
	newCfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	for _, apiName := range apiNamesWithSpecCacheRelevantChanges(oldCfg, newCfg) {
		if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
			return fmt.Errorf("%s: invalidate spec cache for %q: %w", label, apiName, err)
		}
	}
	c.cfg = newCfg
	return nil
}
