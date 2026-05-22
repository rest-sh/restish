package cli

import (
	"fmt"
	"path/filepath"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/request"
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

func (c *CLI) saveConfigShorthand(label string, rootPath []string, exprs []string) error {
	oldCfg := c.cfg
	if err := config.SaveConfigShorthand(c.configFilePath(), rootPath, exprs, c.validateConfigRuntime); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation(label, oldCfg)
}

func (c *CLI) validateConfigRuntime(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	for name, authCfg := range cfg.AuthProfiles {
		if err := c.validateAuthConfig(authCfg); err != nil {
			return fmt.Errorf("auth_profiles.%s: %w", name, err)
		}
	}
	for apiName, apiCfg := range cfg.APIs {
		if apiCfg == nil {
			continue
		}
		for profileName, prof := range apiCfg.Profiles {
			if prof == nil {
				continue
			}
			if err := c.validateAuthConfig(prof.Auth); err != nil {
				return fmt.Errorf("apis.%s.profiles.%s.auth: %w", apiName, profileName, err)
			}
			for i, header := range prof.Headers {
				if _, _, err := request.ParseHeaderOption(header); err != nil {
					return fmt.Errorf("apis.%s.profiles.%s.headers[%d]: %w", apiName, profileName, i, err)
				}
			}
			for i, query := range prof.Query {
				if _, _, err := request.ParseQueryOption(query); err != nil {
					return fmt.Errorf("apis.%s.profiles.%s.query[%d]: %w", apiName, profileName, i, err)
				}
			}
			if prof.TLSSigner != "" {
				if _, ok := c.pluginForHook(prof.TLSSigner, "tls-signer"); !ok {
					return fmt.Errorf("apis.%s.profiles.%s.tls_signer: %q is not a registered tls-signer plugin", apiName, profileName, prof.TLSSigner)
				}
			}
			for credentialID, credential := range prof.Credentials {
				if credential == nil {
					continue
				}
				if err := c.validateAuthConfig(credential.Auth); err != nil {
					return fmt.Errorf("apis.%s.profiles.%s.credentials.%s.auth: %w", apiName, profileName, credentialID, err)
				}
			}
		}
	}
	return nil
}

func (c *CLI) validateAuthConfig(authCfg *config.AuthConfig) error {
	if authCfg == nil || authCfg.Type == "" {
		return nil
	}
	if _, err := c.authHandlerFor(authCfg, authHandlerOptions{}); err != nil {
		return fmt.Errorf("invalid auth.type %q: %w", authCfg.Type, err)
	}
	return nil
}

func (c *CLI) saveAPIConfig(label, apiName string, cfg *config.Config, apiCfg *config.APIConfig) error {
	oldCfg := c.cfg
	cfgPath := c.configFilePath()
	if err := config.SaveAPIConfig(cfgPath, apiName, apiCfg); err != nil {
		return err
	}
	return c.reloadConfigAfterAPIMutation(label, oldCfg, apiName)
}

func (c *CLI) saveConfigMutation(label string, mutate func(*config.Config) error) error {
	oldCfg := c.cfg
	if err := config.SaveConfigMutation(c.configFilePath(), mutate, c.validateConfigRuntime); err != nil {
		return err
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
		{Path: []string{"theme_source"}, Value: source},
		{Path: []string{"theme"}, Value: entries},
	}); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation("config theme set", oldCfg)
}

func (c *CLI) resetThemeConfig() error {
	oldCfg := c.cfg
	cfgPath := c.configFilePath()
	if err := config.SaveConfigValues(cfgPath, []config.ConfigPatchOperation{
		{Path: []string{"theme"}, Delete: true},
		{Path: []string{"theme_source"}, Delete: true},
	}); err != nil {
		return err
	}
	return c.reloadConfigAfterMutation("config theme reset", oldCfg)
}

func (c *CLI) printConfigWrittenPath() {
	cfgPath := c.configFilePath()
	if abs, err := filepath.Abs(cfgPath); err == nil {
		cfgPath = abs
	}
	style := humanTextStyleFor(c.Stdout)
	fmt.Fprintf(c.Stdout, "%s config: %s\n", style.ok("Wrote"), cfgPath)
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

func (c *CLI) reloadConfigAfterAPIMutation(label string, oldCfg *config.Config, apiName string) error {
	newCfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	if apiSpecCacheRelevantFieldsChanged(apiConfigByName(oldCfg, apiName), apiConfigByName(newCfg, apiName)) {
		if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
			return fmt.Errorf("%s: invalidate spec cache for %q: %w", label, apiName, err)
		}
	}
	c.cfg = newCfg
	return nil
}

func apiConfigByName(cfg *config.Config, apiName string) *config.APIConfig {
	if cfg == nil || cfg.APIs == nil {
		return nil
	}
	return cfg.APIs[apiName]
}
