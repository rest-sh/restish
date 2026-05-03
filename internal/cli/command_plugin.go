package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/procutil"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
	"github.com/spf13/cobra"
)

const maxCommandPluginDiscoveryOutputBytes = 1 << 20
const maxCommandPluginStderrBytes = 64 << 10

func loadCommandPluginCommands(path string) ([]pluginwire.CommandDecl, error) {
	timeout := commandPluginDiscoveryTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, pluginwire.StartupFlagCommands)
	procutil.ConfigureCommandTreeKill(ctx, cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: command discovery stdout: %w", filepath.Base(path), err)
	}
	stderr := &cappedBuffer{limit: maxCommandPluginStderrBytes}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin %s: command discovery start: %w", filepath.Base(path), err)
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, maxCommandPluginDiscoveryOutputBytes+1))
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, fmt.Errorf("plugin %s: command discovery read: %w", filepath.Base(path), readErr)
	}
	if len(out) > maxCommandPluginDiscoveryOutputBytes {
		return nil, fmt.Errorf("plugin %s: command discovery output exceeded %d bytes", filepath.Base(path), maxCommandPluginDiscoveryOutputBytes)
	}
	if waitErr != nil {
		if ctx.Err() != nil {
			return nil, commandPluginDiscoveryError(filepath.Base(path), fmt.Sprintf("command discovery timed out after %s", timeout), ctx.Err(), stderr)
		}
		return nil, commandPluginDiscoveryError(filepath.Base(path), "command discovery", waitErr, stderr)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return decodeCommandPluginDiscovery(filepath.Base(path), out)
}

func commandPluginDiscoveryError(pluginName, action string, err error, stderr *cappedBuffer) error {
	excerpt := strings.TrimSpace(string(stderr.Bytes()))
	if excerpt == "" {
		return fmt.Errorf("plugin %s: %s: %w", pluginName, action, err)
	}
	if stderr.Truncated() {
		excerpt += "..."
	}
	return fmt.Errorf("plugin %s: %s: %w: stderr: %s", pluginName, action, err, redactDiagnosticSecretText(excerpt))
}

func decodeCommandPluginDiscovery(pluginName string, out []byte) ([]pluginwire.CommandDecl, error) {
	var resp pluginwire.CommandDiscoveryResponse
	if err := pluginwire.DecMode.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("plugin %s: commands decode: %w", pluginName, err)
	}
	if resp.ProtocolVersion > pluginwire.CommandPluginProtocolVersion {
		return nil, fmt.Errorf("plugin %s: plugin requires restish >= a version that supports command plugin protocol %d", pluginName, resp.ProtocolVersion)
	}
	return resp.Commands, nil
}

func commandPluginDiscoveryTimeout() time.Duration {
	if value := strings.TrimSpace(os.Getenv("RSH_COMMAND_PLUGIN_DISCOVERY_TIMEOUT")); value != "" {
		if d, err := time.ParseDuration(value); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Second
}

func (c *CLI) addCommandPlugins(root *cobra.Command) {
	seen := map[string]string{}
	c.pluginCommandNames = map[string]string{}
	for _, p := range c.pluginsByHook["command"] {
		cmds, err := loadCommandPluginCommands(p.Path)
		if err != nil {
			c.warnf("plugin %s: %v", filepath.Base(p.Path), err)
			continue
		}
		if len(cmds) == 0 {
			continue
		}
		for _, decl := range cmds {
			decl := decl
			pluginPath := p.Path
			if err := c.validatePluginCommandName(root, seen, filepath.Base(pluginPath), decl.Name); err != nil {
				c.warnf("%v", err)
				continue
			}
			seen[decl.Name] = filepath.Base(pluginPath)
			c.pluginCommandNames[decl.Name] = filepath.Base(pluginPath)
			root.AddCommand(&cobra.Command{
				Use:                decl.Name,
				Short:              decl.Short,
				Long:               decl.Long,
				GroupID:            rootGroupPlugin,
				Args:               cobra.ArbitraryArgs,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					return c.runCommandPlugin(cmd, pluginPath, decl, args)
				},
			})
		}
	}
}

var pluginCommandNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (c *CLI) validatePluginCommandName(root *cobra.Command, seen map[string]string, pluginName, name string) error {
	if !pluginCommandNamePattern.MatchString(name) {
		return fmt.Errorf("plugin %s: command name %q is invalid; use lower-case letters, digits, and dashes", pluginName, name)
	}
	if rootHasCommand(root, name) || isBuiltinCommandName(name) {
		return fmt.Errorf("plugin %s: command %q collides with a built-in command; skipping", pluginName, name)
	}
	if c.cfg != nil && c.cfg.APIs[name] != nil {
		return fmt.Errorf("plugin %s: command %q collides with a registered API; skipping", pluginName, name)
	}
	if previous := seen[name]; previous != "" {
		return fmt.Errorf("plugin %s: command %q duplicates command from plugin %s; skipping", pluginName, name, previous)
	}
	return nil
}
