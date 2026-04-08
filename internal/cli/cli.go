package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/content"
	"github.com/danielgtaylor/restish/v2/internal/hypermedia"
	"github.com/danielgtaylor/restish/v2/internal/output"
	internalplugin "github.com/danielgtaylor/restish/v2/internal/plugin"
	"github.com/danielgtaylor/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

// Version is the current build version, set at build time via -ldflags.
var Version = "2.0.0-dev"

// CLI holds all state for a Restish instance. Using a struct instead of
// package-level globals makes it safe to instantiate multiple independent
// instances and trivially testable with in-memory I/O.
type CLI struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// ConfigPath overrides the default config file location. Used in tests
	// to point at a temp file; leave empty to use the platform default.
	ConfigPath string

	// PassReader, if non-nil, is used as the source for secret prompts (e.g.
	// password input). Falls back to Stdin when nil. Set in tests to provide
	// a password without consuming the body stdin.
	PassReader io.Reader

	// TokenCachePath overrides the default token cache file location.
	// Used in tests to point at a temp dir; leave empty to use the platform default.
	TokenCachePath string

	// CachePath overrides the default HTTP response cache directory.
	// Used in tests to point at a temp dir; leave empty to use the platform default.
	CachePath string

	// SpecCachePath overrides the default API spec cache directory.
	// Used in tests to point at a temp dir; leave empty to use the platform default.
	SpecCachePath string

	// RetryBaseDelay overrides the 1 s default backoff base for retries.
	// Set to a small value in tests to avoid slow retries.
	RetryBaseDelay time.Duration

	cfg         *config.Config
	content     *content.Registry
	loaders     []spec.Loader
	linkParsers []hypermedia.Parser
	formatters  map[string]output.Formatter
	plugins     []internalplugin.Plugin
}

// New returns a CLI wired to the real OS stdin/stdout/stderr.
func New() *CLI {
	return &CLI{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		content:     content.Default(),
		loaders:     spec.DefaultLoaders(),
		linkParsers: hypermedia.DefaultParsers(),
		formatters:  output.DefaultFormatters(),
	}
}

// AddLinkParser registers an additional hypermedia link parser. Parsers are
// called in registration order; later parsers can override earlier ones.
func (c *CLI) AddLinkParser(p hypermedia.Parser) {
	c.linkParsers = append(c.linkParsers, p)
}

// AddFormatter registers a named response formatter. Use the same name to
// override a built-in formatter (e.g. "json") or a new name to add a custom
// format selectable via -o <name>.
func (c *CLI) AddFormatter(name string, f output.Formatter) {
	if c.formatters == nil {
		c.formatters = output.DefaultFormatters()
	}
	c.formatters[name] = f
}

// AddContentType registers an additional content type with the CLI's registry.
func (c *CLI) AddContentType(ct *content.ContentType) {
	c.content.AddContentType(ct)
}

// AddEncoding registers an additional compression encoding with the CLI's registry.
func (c *CLI) AddEncoding(e *content.Encoding) {
	c.content.AddEncoding(e)
}

// AddLoader registers an additional spec loader. Higher-priority loaders
// that detect the same content type take precedence over built-in loaders.
func (c *CLI) AddLoader(l spec.Loader) {
	c.loaders = append(c.loaders, l)
}

// Config returns the loaded configuration after Run has been called, or nil
// if Run has not yet been called or configuration loading failed.
// Embedders can use this to inspect configured APIs and profiles.
func (c *CLI) Config() *config.Config {
	return c.cfg
}

// configFilePath returns the effective config file path.
func (c *CLI) configFilePath() string {
	if c.ConfigPath != "" {
		return c.ConfigPath
	}
	return config.DefaultPath()
}

// profileFromCmd returns the active profile name from the --rsh-profile flag,
// falling back to the RSH_PROFILE environment variable, then "default".
func (c *CLI) profileFromCmd(cmd *cobra.Command) string {
	name, _ := cmd.Flags().GetString("rsh-profile")
	if name == "" {
		name = os.Getenv("RSH_PROFILE")
	}
	if name == "" {
		return "default"
	}
	return name
}
// specCacheDir returns the effective directory for API spec CBOR files.
func (c *CLI) specCacheDir() string {
	if c.SpecCachePath != "" {
		return c.SpecCachePath
	}
	return config.DefaultSpecCacheDir()
}

// discoverSpec runs spec discovery for the named API using the registered loaders.
func (c *CLI) discoverSpec(ctx context.Context, apiName string) (*spec.APISpec, error) {
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return nil, nil
	}
	api := c.cfg.APIs[apiName]
	cfg := spec.DiscoverConfig{
		APIName:   apiName,
		BaseURL:   api.BaseURL,
		SpecURL:   api.SpecURL,
		SpecFiles: api.SpecFiles,
		CacheDir:  c.specCacheDir(),
		Version:   Version,
		Transport: http.DefaultTransport,
	}
	return spec.Discover(ctx, cfg, c.loaders)
}

// Run executes the CLI with the provided arguments (pass os.Args from main).
func (c *CLI) Run(args []string) error {
	cfg, err := config.Load(c.configFilePath())
	if err != nil {
		return err
	}
	c.cfg = cfg

	// Discover hook plugins at startup; warn about broken plugins so users
	// know their plugin is not active rather than silently ignoring it.
	c.plugins = internalplugin.Discover(internalplugin.DefaultPluginDir(), cfg.AllowedPlugins, func(path string, err error) {
		fmt.Fprintf(c.Stderr, "warning: plugin %s: %v\n", filepath.Base(path), err)
	})

	// Register plugin-provided formatters and loaders.
	for _, p := range c.plugins {
		for _, name := range p.Manifest.FormatterNames {
			c.formatters[name] = &output.PluginFormatter{
				PluginPath: p.Path,
				FormatName: name,
			}
		}
		if len(p.Manifest.LoaderContentTypes) > 0 {
			c.loaders = append(c.loaders, spec.PluginLoader{
				PluginPath:   p.Path,
				PluginName:   p.Manifest.Name,
				ContentTypes: p.Manifest.LoaderContentTypes,
			})
		}
	}

	root := c.newRootCmd()

	// Register generated commands for APIs whose spec is already cached.
	// (Network discovery is not triggered here; use "restish api sync <name>"
	// to prime the cache for an API.)
	for apiName, apiCfg := range cfg.APIs {
		s, err := spec.LoadFromCache(c.specCacheDir(), apiName, Version, c.loaders)
		if err != nil || s == nil {
			continue
		}
		if apiCmd := c.buildAPICommand(apiName, apiCfg, s); apiCmd != nil {
			root.AddCommand(apiCmd)
		}
	}

	root.SetArgs(args[1:])
	root.SetOut(c.Stdout)
	root.SetErr(c.Stderr)
	return root.Execute()
}
