package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rest-sh/restish/v2/internal/config"
	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

func (c *CLI) addDoctorCommand(root *cobra.Command) {
	doctorCmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Diagnose Restish configuration and runtime paths",
		GroupID: rootGroupUtility,
		Args:    cobra.NoArgs,
		RunE:    c.runDoctor,
	}
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "api <name>",
		Short: "Diagnose a registered API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runDoctorAPI,
	})
	doctorAPI := doctorCmd.Commands()[0]
	doctorAPI.Flags().Bool("check-network", false, "Make a bounded network request to check API reachability")
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "plugin <name>",
		Short: "Diagnose a Restish plugin executable",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runDoctorPlugin,
	})
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "migrate-v1",
		Short: "Run default-location v1 config migration if eligible",
		Args:  cobra.NoArgs,
		RunE:  c.runDoctorMigrateV1,
	})
	root.AddCommand(doctorCmd)
}

func (c *CLI) runDoctor(cmd *cobra.Command, args []string) error {
	cfgPath := c.configFilePath()
	fmt.Fprintf(c.Stderr, "Config file: %s\n", cfgPath)
	if cfg, err := c.loadConfig(); err != nil {
		fmt.Fprintf(c.Stderr, "Config parse: invalid\n  %v\n", err)
		c.printConfigDiagnostics(cfgPath)
	} else {
		apiCount := 0
		if cfg.APIs != nil {
			apiCount = len(cfg.APIs)
		}
		fmt.Fprintf(c.Stderr, "Config parse: ok (%d APIs)\n", apiCount)
		c.printConfigDiagnostics(cfgPath)
	}
	if insecure, err := config.ConfigFileHasInsecurePermissions(cfgPath); err != nil {
		fmt.Fprintf(c.Stderr, "Config permissions: unknown (%v)\n", err)
	} else if insecure {
		fmt.Fprintf(c.Stderr, "Config permissions: insecure (run chmod 600 %s)\n", cfgPath)
	} else {
		fmt.Fprintln(c.Stderr, "Config permissions: ok")
	}
	fmt.Fprintf(c.Stderr, "HTTP cache: %s\n", c.configScopedCacheDir(c.paths().Cache()))
	fmt.Fprintf(c.Stderr, "Spec cache: %s\n", c.specCacheDir())
	fmt.Fprintf(c.Stderr, "Plugin directory: %s\n", defaultPluginDirForDoctor())
	c.printShellSetupDiagnostic()
	return nil
}

func (c *CLI) runDoctorAPI(cmd *cobra.Command, args []string) error {
	cfg, err := c.loadConfig()
	if err != nil {
		fmt.Fprintf(c.Stderr, "Config parse: invalid\n  %v\n", err)
		return nil
	}
	name := args[0]
	api := cfg.APIs[name]
	if api == nil {
		fmt.Fprintf(c.Stderr, "API %q: not registered\n", name)
		return nil
	}
	fmt.Fprintf(c.Stderr, "API %q: registered\n", name)
	fmt.Fprintf(c.Stderr, "Base URL: %s\n", api.BaseURL)
	if api.SpecURL != "" {
		fmt.Fprintf(c.Stderr, "Spec URL: %s\n", api.SpecURL)
	}
	if len(api.SpecFiles) > 0 {
		fmt.Fprintf(c.Stderr, "Spec files: %v\n", api.SpecFiles)
	}
	if _, ok := configFileExists(filepath.Join(c.specCacheDir(), name+".cbor")); ok {
		fmt.Fprintln(c.Stderr, "Spec cache: present")
	} else {
		fmt.Fprintln(c.Stderr, "Spec cache: missing")
	}
	if set, ok := spec.LoadOperationSetFromCacheWithVariables(c.specCacheDir(), name, Version, api.SpecFiles, spec.OperationOptions{
		BaseURL:       api.BaseURL,
		OperationBase: api.OperationBase,
	}); ok {
		fmt.Fprintf(c.Stderr, "Generated operations: %d available\n", len(set.Operations))
	} else {
		fmt.Fprintln(c.Stderr, "Generated operations: unavailable")
	}
	if prof := api.Profiles[c.profileFromCmd(cmd)]; prof != nil && (prof.Auth != nil || prof.AuthRef != "" || len(prof.Credentials) > 0) {
		fmt.Fprintln(c.Stderr, "Auth: configured")
	} else {
		fmt.Fprintln(c.Stderr, "Auth: no profile auth configured")
	}
	checkNetwork, _ := cmd.Flags().GetBool("check-network")
	if checkNetwork {
		c.checkAPIReachability(api.BaseURL)
	} else {
		fmt.Fprintln(c.Stderr, "Reachability: skipped (use --check-network)")
	}
	return nil
}

func (c *CLI) runDoctorPlugin(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := name
	if !filepath.IsAbs(path) && filepath.Base(path) == path {
		path = filepath.Join(defaultPluginDirForDoctor(), name)
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(c.Stderr, "Plugin %q: not found at %s\n", name, path)
		return nil
	}
	if err != nil {
		fmt.Fprintf(c.Stderr, "Plugin %q: stat failed: %v\n", name, err)
		return nil
	}
	fmt.Fprintf(c.Stderr, "Plugin %q: found at %s\n", name, path)
	if info.Mode()&0o111 == 0 {
		fmt.Fprintln(c.Stderr, "Executable: no")
	} else {
		fmt.Fprintln(c.Stderr, "Executable: yes")
	}
	manifest, err := internalplugin.LoadManifestWithWarnings(path, diagnosticPrefixWriter(c.Stderr))
	if err != nil {
		fmt.Fprintf(c.Stderr, "Manifest: invalid (%v)\n", err)
		return nil
	}
	fmt.Fprintf(c.Stderr, "Manifest: %s %s\n", manifest.Name, manifest.Version)
	fmt.Fprintf(c.Stderr, "Declared capabilities: %s\n", pluginCapabilitySummary(*manifest))
	fmt.Fprintf(c.Stderr, "Protocol startup: ok (API v%d)\n", manifest.RestishAPIVersion)
	return nil
}

func (c *CLI) runDoctorMigrateV1(cmd *cobra.Command, args []string) error {
	if c.explicitConfigFile {
		fmt.Fprintln(c.Stderr, "Migration: skipped (explicit config file selected)")
		return nil
	}
	if _, err := os.Stat(c.configFilePath()); err == nil {
		fmt.Fprintf(c.Stderr, "Migration: skipped (%s already exists)\n", c.configFilePath())
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(c.Stderr, "Migration: cannot inspect %s: %v\n", c.configFilePath(), err)
		return nil
	}
	cfg, err := c.loadConfig()
	if err != nil {
		fmt.Fprintf(c.Stderr, "Migration: failed\n  %v\n", err)
		return nil
	}
	if cfg.Migration == nil {
		fmt.Fprintln(c.Stderr, "Migration: no eligible v1 config found")
		return nil
	}
	fmt.Fprintf(c.Stderr, "Migration: migrated v1 config from %s\n", cfg.Migration.SourcePath)
	fmt.Fprintf(c.Stderr, "Backup: %s\n", cfg.Migration.BackupPath)
	return nil
}

func configFileExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}

func defaultPluginDirForDoctor() string {
	return internalplugin.DefaultPluginDir()
}

func (c *CLI) printConfigDiagnostics(path string) {
	diags, err := config.DiagnoseConfig(path)
	if err != nil {
		return
	}
	for _, diag := range diags.UnknownFields {
		if diag.Line > 0 {
			fmt.Fprintf(c.Stderr, "Unknown field: %s at %d:%d\n", diag.Path, diag.Line, diag.Column)
		} else {
			fmt.Fprintf(c.Stderr, "Unknown field: %s\n", diag.Path)
		}
		if diag.Suggestion != "" {
			fmt.Fprintf(c.Stderr, "  Did you mean %q?\n", diag.Suggestion)
		}
		if diag.Hint != "" {
			fmt.Fprintf(c.Stderr, "  %s\n", diag.Hint)
		}
	}
}

func (c *CLI) printShellSetupDiagnostic() {
	shell, source := detectRunningShell()
	if shell == "" {
		fmt.Fprintln(c.Stderr, "Shell setup: unknown")
		return
	}
	if _, ok := shellSetups[shell]; !ok {
		fmt.Fprintf(c.Stderr, "Shell setup: unsupported shell %s\n", shell)
		return
	}
	if source == "$SHELL" {
		fmt.Fprintf(c.Stderr, "Shell setup: run `restish setup %s` if glob expansion causes trouble (detected via $SHELL)\n", shell)
		return
	}
	fmt.Fprintf(c.Stderr, "Shell setup: run `restish setup %s` if glob expansion causes trouble\n", shell)
}

func (c *CLI) checkAPIReachability(baseURL string) {
	if baseURL == "" {
		fmt.Fprintln(c.Stderr, "Reachability: skipped (no base URL)")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL, nil)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Reachability: invalid base URL (%v)\n", err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Reachability: failed (%v)\n", err)
		return
	}
	_ = resp.Body.Close()
	fmt.Fprintf(c.Stderr, "Reachability: HTTP %s\n", resp.Status)
}
