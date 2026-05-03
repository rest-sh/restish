package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	pluginwire "github.com/rest-sh/restish/v2/plugin"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/spf13/cobra"
)

const (
	maxPluginDebugCaptureBytes = 64 << 20
)

// addPluginCommand registers the "plugin" subcommand tree on root.
func (c *CLI) addPluginCommand(root *cobra.Command) {
	pluginCmd := &cobra.Command{
		Use:     "plugin",
		Short:   "Manage restish plugins",
		GroupID: rootGroupPlugin,
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all discovered plugins",
		Args:  cobra.NoArgs,
		RunE:  c.runPluginList,
	}
	listCmd.Flags().Bool("json", false, "Write machine-readable plugin metadata as JSON")
	pluginCmd.AddCommand(listCmd)
	installCmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install a plugin from a path, URL, PATH command, or GitHub release",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runPluginInstall,
	}
	installCmd.Flags().Bool("yes", false, "Trust and install without an interactive confirmation")
	pluginCmd.AddCommand(installCmd)
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
	plugins := plugin.Discover(c.pluginDir(), func(path string, err error) {
		c.warnf("plugin %s: %v", filepath.Base(path), err)
	}, c.pluginManifestCachePath(), diagnosticPrefixWriter(c.Stderr))

	if len(plugins) == 0 {
		asJSON, _ := cmd.Flags().GetBool("json")
		if asJSON {
			fmt.Fprintln(c.Stdout, "[]")
			return nil
		}
		fmt.Fprintln(c.Stdout, "No plugins found.")
		return nil
	}

	type pluginListEntry struct {
		Name         string   `json:"name"`
		Version      string   `json:"version,omitempty"`
		Description  string   `json:"description,omitempty"`
		Path         string   `json:"path"`
		Capabilities []string `json:"capabilities"`
		Commands     []string `json:"commands,omitempty"`
		Formatters   []string `json:"formatters,omitempty"`
		Loaders      []string `json:"loaders,omitempty"`
	}
	entries := make([]pluginListEntry, 0, len(plugins))
	for _, p := range plugins {
		m := p.Manifest
		entry := pluginListEntry{
			Name:         m.Name,
			Version:      m.Version,
			Description:  m.Description,
			Path:         p.Path,
			Capabilities: append([]string(nil), m.Hooks...),
			Formatters:   append([]string(nil), m.FormatterNames...),
			Loaders:      append([]string(nil), m.LoaderContentTypes...),
		}
		if pluginDeclaresHook(m, "command") {
			decls, err := loadCommandPluginCommands(p.Path)
			if err != nil {
				c.warnf("plugin %s: %v", filepath.Base(p.Path), err)
			}
			for _, decl := range decls {
				if decl.Name != "" {
					entry.Commands = append(entry.Commands, decl.Name)
				}
			}
		}
		entries = append(entries, entry)
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		enc := json.NewEncoder(c.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	for _, entry := range entries {
		m := plugin.Manifest{
			Name:               entry.Name,
			Version:            entry.Version,
			Description:        entry.Description,
			Hooks:              entry.Capabilities,
			FormatterNames:     entry.Formatters,
			LoaderContentTypes: entry.Loaders,
		}
		fmt.Fprintf(c.Stdout, "%-20s %-10s capabilities: %s\n", m.Name, m.Version, pluginCapabilitySummary(m))
		if len(entry.Commands) > 0 {
			fmt.Fprintf(c.Stdout, "  commands: %s\n", strings.Join(entry.Commands, ", "))
		}
		if len(entry.Formatters) > 0 {
			fmt.Fprintf(c.Stdout, "  formatters: %s\n", strings.Join(entry.Formatters, ", "))
		}
		if m.Description != "" {
			fmt.Fprintf(c.Stdout, "  %s\n", m.Description)
		}
	}
	return nil
}

// runPluginInstall installs a plugin binary from a local path, PATH executable,
// direct archive URL, or GitHub release shorthand.

// runPluginRemove deletes a plugin from the plugin directory.
func (c *CLI) runPluginRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := validatePluginName(name); err != nil {
		return err
	}
	pluginDir := c.pluginDir()
	path := filepath.Join(pluginDir, name)
	if err := os.Remove(path); err != nil {
		if runtime.GOOS == "windows" && filepath.Ext(name) == "" && errors.Is(err, os.ErrNotExist) {
			if exeErr := os.Remove(path + ".exe"); exeErr == nil {
				fmt.Fprintf(c.Stdout, "Removed plugin %s\n", name)
				return nil
			} else if !errors.Is(exeErr, os.ErrNotExist) {
				return fmt.Errorf("remove: %w", exeErr)
			}
		}
		if errors.Is(err, os.ErrNotExist) {
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

	// Decode stdout as CBOR incrementally. Raw CBOR bytes must not be written to
	// the terminal since they would corrupt it.
	stdout, err := pluginCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("plugin debug: stdout pipe: %w", err)
	}

	if err := pluginCmd.Start(); err != nil {
		return fmt.Errorf("plugin debug: start: %w", err)
	}
	decodeCh := make(chan pluginDebugDecodeResult, 1)
	go func() {
		n, err := decodePluginDebugStream(stdout, c.Stderr)
		decodeCh <- pluginDebugDecodeResult{bytes: n, err: err}
	}()

	if err := pluginCmd.Wait(); err != nil {
		// Non-zero exit is reported but not fatal in debug mode.
		fmt.Fprintf(c.Stderr, "plugin exited: %v\n", err)
	}
	result := <-decodeCh
	if result.err != nil {
		return result.err
	}
	if result.bytes > maxPluginDebugCaptureBytes {
		c.warnf("plugin debug decoded more than %d stdout bytes", maxPluginDebugCaptureBytes)
	}
	return nil
}

type pluginDebugDecodeResult struct {
	bytes int64
	err   error
}

func decodePluginDebugStream(r io.Reader, w io.Writer) (int64, error) {
	counter := &countingReader{r: r}
	dec := pluginwire.NewDecoder(counter)
	for {
		var v any
		if err := dec.ReadMessage(&v); err != nil {
			if !errors.Is(err, io.EOF) {
				_, _ = io.Copy(io.Discard, counter)
				return counter.n, fmt.Errorf("plugin debug: decode stdout: %w", err)
			}
			return counter.n, nil
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		if _, err := fmt.Fprintf(w, "[debug] decoded CBOR message:\n%s\n", b); err != nil {
			return counter.n, err
		}
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

// terminalContextFlags returns the standard terminal context flags that Restish
// passes to every plugin invocation.
func terminalContextFlags(c *CLI) []string {
	stdoutTTY := output.IsTerminal(c.Stdout)
	stderrTTY := output.IsTerminal(c.Stderr)
	color := output.ColorEnabled(c.Stdout)
	return []string{
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagStdoutTTY, stdoutTTY),
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagStderrTTY, stderrTTY),
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagColor, color),
	}
}

// copyFile copies src to dst, creating dst with the same permissions as src.
var pluginInstallCopyFile = copyFile

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

func validatePluginName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("remove: invalid plugin name %q", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || filepath.Base(name) != name {
		return fmt.Errorf("remove: invalid plugin name %q", name)
	}
	return nil
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit < 0 {
		return b.buf.Write(p)
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
			b.truncated = true
			return len(p), nil
		}
		return b.buf.Write(p)
	}
	if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *cappedBuffer) Truncated() bool {
	return b.truncated
}
