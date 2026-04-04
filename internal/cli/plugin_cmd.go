package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/restish/v2/internal/plugin"
	"github.com/spf13/cobra"
)

// addPluginCommand registers the "plugin" subcommand tree on root.
func (c *CLI) addPluginCommand(root *cobra.Command) {
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage restish plugins",
	}
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all discovered plugins",
		Args:  cobra.NoArgs,
		RunE:  c.runPluginList,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "install <source>",
		Short: "Install a plugin from a local path",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runPluginInstall,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runPluginRemove,
	})
	root.AddCommand(pluginCmd)
}

// runPluginList discovers and prints all available plugins with their hooks.
func (c *CLI) runPluginList(cmd *cobra.Command, args []string) error {
	plugins := plugin.Discover(plugin.DefaultPluginDir(), func(path string, err error) {
		fmt.Fprintf(c.Stderr, "warning: plugin %s: %v\n", filepath.Base(path), err)
	})

	if len(plugins) == 0 {
		fmt.Fprintln(c.Stdout, "No plugins found.")
		return nil
	}

	for _, p := range plugins {
		m := p.Manifest
		hooks := strings.Join(m.Hooks, ", ")
		if hooks == "" {
			hooks = "(none)"
		}
		fmt.Fprintf(c.Stdout, "%-20s %-10s hooks: %s\n", m.Name, m.Version, hooks)
		if m.Description != "" {
			fmt.Fprintf(c.Stdout, "  %s\n", m.Description)
		}
	}
	return nil
}

// runPluginInstall copies a plugin binary from source into the plugin directory.
func (c *CLI) runPluginInstall(cmd *cobra.Command, args []string) error {
	source := args[0]
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("install: cannot access %s: %w", source, err)
	}
	if info.IsDir() {
		return fmt.Errorf("install: %s is a directory", source)
	}

	pluginDir := plugin.DefaultPluginDir()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("install: cannot create plugin dir %s: %w", pluginDir, err)
	}

	dest := filepath.Join(pluginDir, filepath.Base(source))
	if err := copyFile(source, dest); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	// Make it executable.
	_ = os.Chmod(dest, 0o755)

	fmt.Fprintf(c.Stdout, "Installed plugin %s\n", filepath.Base(source))
	return nil
}

// runPluginRemove deletes a plugin from the plugin directory.
func (c *CLI) runPluginRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	pluginDir := plugin.DefaultPluginDir()
	path := filepath.Join(pluginDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("remove: plugin %q not found in %s", name, pluginDir)
		}
		return fmt.Errorf("remove: %w", err)
	}
	fmt.Fprintf(c.Stdout, "Removed plugin %s\n", name)
	return nil
}

// copyFile copies src to dst, creating dst with the same permissions as src.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to %s: %w", dst, err)
	}
	return nil
}
