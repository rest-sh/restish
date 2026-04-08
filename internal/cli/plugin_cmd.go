package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"

	"github.com/danielgtaylor/restish/v2/internal/output"
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
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "debug <name> [args...]",
		Short: "Spawn a plugin and print decoded CBOR messages to stderr",
		Args:  cobra.MinimumNArgs(1),
		RunE:  c.runPluginDebug,
	})
	root.AddCommand(pluginCmd)
}

// runPluginList discovers and prints all available plugins with their hooks.
func (c *CLI) runPluginList(cmd *cobra.Command, args []string) error {
	plugins := plugin.Discover(plugin.DefaultPluginDir(), c.cfg.AllowedPlugins, func(path string, err error) {
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

// runPluginDebug spawns a plugin binary with terminal context flags and tees
// its stdin/stdout through a CBOR-to-JSON decoder, printing decoded messages
// to stderr for debugging.
func (c *CLI) runPluginDebug(cmd *cobra.Command, args []string) error {
	name := args[0]
	extraArgs := args[1:]

	// Locate the plugin binary.
	path, err := exec.LookPath(name)
	if err != nil {
		// Try with restish- prefix.
		path, err = exec.LookPath("restish-" + name)
		if err != nil {
			return fmt.Errorf("plugin debug: cannot find plugin %q", name)
		}
	}

	ttyFlags := terminalContextFlags(c)
	allArgs := append(ttyFlags, extraArgs...)
	pluginCmd := exec.Command(path, allArgs...)
	pluginCmd.Stdin = c.Stdin
	pluginCmd.Stderr = c.Stderr

	// Capture stdout for CBOR decoding while also passing it through.
	var stdoutBuf bytes.Buffer
	pluginCmd.Stdout = io.MultiWriter(c.Stdout, &stdoutBuf)

	if err := pluginCmd.Run(); err != nil {
		// Non-zero exit is reported but not fatal in debug mode.
		fmt.Fprintf(c.Stderr, "plugin exited: %v\n", err)
	}

	// Attempt to decode all CBOR messages from the captured stdout.
	data := stdoutBuf.Bytes()
	if len(data) > 0 {
		r := bytes.NewReader(data)
		for r.Len() > 0 {
			var v any
			if decErr := pluginwire.ReadMessage(r, &v); decErr != nil {
				break
			}
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Fprintf(c.Stderr, "[debug] decoded CBOR message:\n%s\n", b)
		}
	}
	return nil
}

// terminalContextFlags returns the standard terminal context flags that Restish
// passes to every plugin invocation.
func terminalContextFlags(c *CLI) []string {
	stdoutTTY := output.IsTerminal(c.Stdout)
	stderrTTY := output.IsTerminal(c.Stderr)
	color := output.ColorEnabled(c.Stdout)
	return []string{
		fmt.Sprintf("--rsh-stdout-tty=%v", stdoutTTY),
		fmt.Sprintf("--rsh-stderr-tty=%v", stderrTTY),
		fmt.Sprintf("--rsh-color=%v", color),
	}
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
