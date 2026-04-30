package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rest-sh/restish/v2/internal/config"
	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
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
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "plugin <name>",
		Short: "Diagnose a Restish plugin executable",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runDoctorPlugin,
	})
	root.AddCommand(doctorCmd)
}

func (c *CLI) runDoctor(cmd *cobra.Command, args []string) error {
	cfgPath := c.configFilePath()
	fmt.Fprintf(c.Stderr, "Config file: %s\n", cfgPath)
	if cfg, err := c.loadConfig(); err != nil {
		fmt.Fprintf(c.Stderr, "Config parse: invalid\n  %v\n", err)
	} else {
		apiCount := 0
		if cfg.APIs != nil {
			apiCount = len(cfg.APIs)
		}
		fmt.Fprintf(c.Stderr, "Config parse: ok (%d APIs)\n", apiCount)
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
	if prof := api.Profiles[c.profileFromCmd(cmd)]; prof != nil && (prof.Auth != nil || prof.AuthRef != "" || len(prof.Credentials) > 0) {
		fmt.Fprintln(c.Stderr, "Auth: configured")
	} else {
		fmt.Fprintln(c.Stderr, "Auth: no profile auth configured")
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
	return nil
}

func configFileExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}

func defaultPluginDirForDoctor() string {
	return internalplugin.DefaultPluginDir()
}
