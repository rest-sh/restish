package cli

import (
	"context"
	"path/filepath"

	"github.com/rest-sh/restish/v2/config"
	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/spf13/cobra"
)

// DocsCommandOptions controls construction of a Cobra tree for documentation
// generation. It intentionally avoids loading the user's config and discovers
// plugins only from the supplied directory.
type DocsCommandOptions struct {
	PluginDir               string
	PluginManifestCachePath string
}

// RootCommandForDocs returns the same root command tree used by the CLI, but
// with an empty config and isolated plugin discovery. It is for maintainer docs
// generation and should not be used to execute user requests.
func (c *CLI) RootCommandForDocs(opts DocsCommandOptions) (*cobra.Command, error) {
	c.cfg = &config.Config{}
	c.runCtx = context.Background()
	c.plugins = nil
	if opts.PluginDir != "" {
		c.plugins = internalplugin.Discover(opts.PluginDir, func(path string, err error) {
			c.warnf("plugin %s: %v", filepath.Base(path), err)
		}, opts.PluginManifestCachePath, diagnosticPrefixWriter(c.Stderr))
	}
	c.pluginsByHook = indexPluginsByHook(c.plugins)
	c.globalAuthPlugins, c.authPluginsByAPI = indexAuthPluginsByAPI(c.pluginsByHook["auth"])
	return c.newRootCmd(), nil
}
