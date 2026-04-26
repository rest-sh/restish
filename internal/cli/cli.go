package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	authpkg "github.com/rest-sh/restish/v2/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/content"
	"github.com/rest-sh/restish/v2/internal/hypermedia"
	"github.com/rest-sh/restish/v2/internal/output"
	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

// Version is the current build version, set at build time via -ldflags.
var Version = "2.0.0-dev"

// testHooks holds test-only overrides for CLI internals. The zero value is
// safe: every field is treated as "not set" when empty/nil.
type testHooks struct {
	// ConfigPath overrides the default config file location.
	ConfigPath string
	// PassReader, if non-nil, is used as the source for secret prompts.
	PassReader io.Reader
	// PromptFunc overrides visible prompt reads in tests.
	PromptFunc func(context.Context, string) (string, error)
	// SecretFunc overrides secret prompt reads in tests.
	SecretFunc func(context.Context, string) (string, error)
	// TokenCachePath overrides the default token cache file location.
	TokenCachePath string
	// CachePath overrides the default HTTP response cache directory.
	CachePath string
	// SpecCachePath overrides the default API spec cache directory.
	SpecCachePath string
	// HTTPTransport overrides the base HTTP transport used for outbound requests.
	HTTPTransport http.RoundTripper
	// PluginManifestCachePath overrides the default plugin manifest cache file.
	PluginManifestCachePath string
	// RetryBaseDelay overrides the 1 s default backoff base for retries.
	RetryBaseDelay time.Duration
}

// CLI holds all state for a Restish instance. Using a struct instead of
// package-level globals makes it safe to instantiate multiple independent
// instances and trivially testable with in-memory I/O.
type CLI struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	hooks testHooks

	// Paths holds computed config/cache locations. Tests may replace this with
	// a custom instance.
	Paths *config.Paths

	cfg                *config.Config
	content            *content.Registry
	loaders            []spec.Loader
	linkParsers        []hypermedia.Parser
	formatters         map[string]output.Formatter
	plugins            []internalplugin.Plugin
	pluginsByHook      map[string][]internalplugin.Plugin
	customAuthHandlers map[string]authpkg.Handler
	requestClosersMu   sync.Mutex
	requestClosers     []io.Closer
}

// New returns a CLI wired to the real OS stdin/stdout/stderr.
func New() *CLI {
	return &CLI{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Paths:       config.NewPaths(),
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

type flushWriter interface {
	Flush() error
}

func (c *CLI) flushStdout() error {
	if f, ok := c.Stdout.(flushWriter); ok {
		return f.Flush()
	}
	return nil
}

// AddAuthHandler registers a custom auth handler under the given type name.
// The name is used in the profile's auth.type config field.
// Built-in names (http-basic, oauth-client-credentials,
// oauth-authorization-code, oauth-device-code, external-tool) can be overridden.
// Call this before CLI.Run.
//
// Use the github.com/rest-sh/restish/v2/auth package for the Handler
// and Param types when implementing custom auth.
func (c *CLI) AddAuthHandler(name string, handler authpkg.Handler) {
	if c.customAuthHandlers == nil {
		c.customAuthHandlers = make(map[string]authpkg.Handler)
	}
	c.customAuthHandlers[name] = handler
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
	if c.hooks.ConfigPath != "" {
		return c.hooks.ConfigPath
	}
	return c.paths().ConfigFile()
}

// profileFromCmd returns the active profile name from the --rsh-profile flag,
// falling back to the RSH_PROFILE environment variable (handled in
// parseGlobalFlags), then "default".
func (c *CLI) profileFromCmd(cmd *cobra.Command) string {
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Profile != "" {
		return gf.Profile
	}
	return "default"
}

// specCacheDir returns the effective directory for API spec CBOR files.
func (c *CLI) specCacheDir() string {
	if c.hooks.SpecCachePath != "" {
		return c.hooks.SpecCachePath
	}
	return c.paths().SpecCache()
}

// pluginManifestCachePath returns the effective plugin manifest cache file path.
func (c *CLI) pluginManifestCachePath() string {
	if c.hooks.PluginManifestCachePath != "" {
		return c.hooks.PluginManifestCachePath
	}
	return c.paths().PluginManifestCache()
}

func (c *CLI) paths() *config.Paths {
	if c.Paths != nil {
		return c.Paths
	}
	return config.NewPaths()
}

// discoverSpec runs spec discovery for the named API using the registered loaders.
func (c *CLI) discoverSpec(ctx context.Context, apiName string) (*spec.APISpec, error) {
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return nil, nil
	}
	api := c.cfg.APIs[apiName]
	cfg := spec.DiscoverConfig{
		APIName:          apiName,
		BaseURL:          api.BaseURL,
		SpecURL:          api.SpecURL,
		SpecFiles:        api.SpecFiles,
		CacheDir:         c.specCacheDir(),
		OperationBase:    api.OperationBase,
		ServerVariables:  effectiveServerVariables(api, "default"),
		Version:          Version,
		Transport:        c.baseHTTPTransport(),
		AllowCrossOrigin: api.AllowCrossOriginSpec,
	}
	return spec.Discover(ctx, cfg, c.loaders)
}

func (c *CLI) baseHTTPTransport() http.RoundTripper {
	if c.hooks.HTTPTransport != nil {
		return c.hooks.HTTPTransport
	}
	return http.DefaultTransport
}

// Run executes the CLI with the provided arguments (pass os.Args from main).
func (c *CLI) Run(args []string) error {
	// Install a signal-aware context so that Ctrl-C / SIGTERM propagates to all
	// in-flight requests and spec discovery without needing explicit signal
	// handling elsewhere.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c.requestClosers = nil
	defer c.closeRequestClosers()

	if !output.IsTerminal(c.Stdout) {
		origStdout := c.Stdout
		buf := bufio.NewWriterSize(origStdout, 64*1024)
		c.Stdout = buf
		defer func() {
			_ = buf.Flush()
			c.Stdout = origStdout
		}()
	}

	// On first run (no config file yet), suggest shell setup if on a supported
	// shell so users discover the noglob alias before hitting the foot-gun.
	if _, statErr := os.Stat(c.configFilePath()); errors.Is(statErr, os.ErrNotExist) && output.IsTerminal(c.Stderr) {
		c.hintShellSetup()
	}

	cfg, err := config.Load(c.configFilePath())
	if err != nil {
		return err
	}
	if insecure, permErr := config.ConfigFileHasInsecurePermissions(c.configFilePath()); permErr == nil && insecure {
		c.warnf("%s is group/world-readable; credentials in config should be kept private (chmod 600)", c.configFilePath())
	}
	c.cfg = cfg
	if cfg.Migration != nil {
		c.infof("Migrated config from v1 at %s; kept backup at %s", cfg.Migration.SourcePath, cfg.Migration.BackupPath)
		for _, warning := range cfg.Migration.Warnings {
			c.warnf("migration: %s", warning)
		}
	}
	if err := output.SetTheme(output.ThemeEntries(cfg.Theme)); err != nil {
		return fmt.Errorf("config theme: %w", err)
	}

	// Discover hook plugins at startup; warn about broken plugins so users
	// know their plugin is not active rather than silently ignoring it.
	c.plugins = internalplugin.Discover(internalplugin.DefaultPluginDir(), func(path string, err error) {
		c.warnf("plugin %s: %v", filepath.Base(path), err)
	}, c.pluginManifestCachePath(), diagnosticPrefixWriter(c.Stderr))
	c.pluginsByHook = indexPluginsByHook(c.plugins)

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

	// Warn about any configured API names that shadow built-in commands.
	for apiName := range cfg.APIs {
		if isBuiltinCommandName(apiName) {
			c.warnf("API name %q shadows a built-in command and will not be reachable as a subcommand", apiName)
		}
	}

	// Register generated commands for APIs whose spec is already cached.
	// When the first positional arg names a configured API, only load that
	// API's cached spec to avoid eagerly parsing every registered API.
	// (Network discovery is not triggered here; use "restish api sync <name>"
	// to prime the cache for an API.)
	startupProfile := profileNameFromArgs(args)
	for _, apiName := range c.generatedAPINames(args, cfg) {
		apiCfg := cfg.APIs[apiName]
		opOpts := spec.OperationOptions{
			BaseURL:         apiCfg.BaseURL,
			OperationBase:   apiCfg.OperationBase,
			ServerVariables: effectiveServerVariables(apiCfg, startupProfile),
		}
		if set, ok := spec.LoadOperationSetFromCacheWithVariables(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, opOpts); ok {
			if apiCmd := c.buildAPICommandFromOperationSet(apiName, apiCfg, set); apiCmd != nil {
				root.AddCommand(apiCmd)
			}
			continue
		}
		s, err := spec.LoadFromCache(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, c.loaders)
		if err != nil {
			continue
		}
		if s == nil && spec.HasLocalSpecFiles(apiCfg.SpecFiles) {
			s, err = c.discoverSpec(ctx, apiName)
		}
		if err != nil || s == nil {
			continue
		}
		set, opsErr := s.OperationSetWithOptions(opOpts)
		if opsErr == nil {
			_ = spec.StoreOperationSetInCacheWithVariables(c.specCacheDir(), apiName, Version, opOpts, set)
		}
		if apiCmd := c.buildAPICommandFromOperationResult(apiName, apiCfg, set, opsErr); apiCmd != nil {
			root.AddCommand(apiCmd)
		}
	}
	c.addAPIShortNameCommands(root, cfg)

	root.SetArgs(args[1:])
	root.SetOut(c.Stdout)
	root.SetErr(c.Stderr)
	err = root.ExecuteContext(ctx)
	// When the context was cancelled by a signal (SIGINT/SIGTERM), return
	// ExitCodeError{130} so main exits with 130 without printing any extra message.
	if err != nil && errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return &ExitCodeError{Code: 130}
	}
	return err
}

func profileNameFromArgs(args []string) string {
	profile := os.Getenv("RSH_PROFILE")
	if profile == "" {
		profile = "default"
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--rsh-profile=") {
			value := strings.TrimPrefix(arg, "--rsh-profile=")
			if value != "" {
				profile = value
			}
			continue
		}
		if arg == "--rsh-profile" || arg == "-p" {
			if i+1 < len(args) && args[i+1] != "" {
				profile = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
			profile = strings.TrimPrefix(arg, "-p")
		}
	}
	return profile
}

func effectiveServerVariables(apiCfg *config.APIConfig, profileName string) map[string]string {
	if apiCfg == nil {
		return nil
	}
	var out map[string]string
	for key, value := range apiCfg.ServerVariables {
		if out == nil {
			out = map[string]string{}
		}
		out[key] = value
	}
	if profileName == "" {
		profileName = "default"
	}
	if prof := apiCfg.Profiles[profileName]; prof != nil {
		for key, value := range prof.ServerVariables {
			if out == nil {
				out = map[string]string{}
			}
			out[key] = value
		}
	}
	return out
}

func (c *CLI) addAPIShortNameCommands(root *cobra.Command, cfg *config.Config) {
	if cfg == nil || len(cfg.APIs) == 0 {
		return
	}
	names := make([]string, 0, len(cfg.APIs))
	for name := range cfg.APIs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, apiName := range names {
		apiCfg := cfg.APIs[apiName]
		if apiCfg == nil || isBuiltinCommandName(apiName) || rootHasCommand(root, apiName) {
			continue
		}
		apiName := apiName
		short := "GET requests using the registered API base URL"
		if apiCfg.BaseURL != "" {
			short = fmt.Sprintf("GET requests using %s", apiCfg.BaseURL)
		}
		root.AddCommand(&cobra.Command{
			Use:     apiName,
			Short:   short,
			GroupID: rootGroupAPI,
			Args:    cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.runHTTP(cmd, "GET", append([]string{apiName}, args...))
			},
		})
	}
}

func rootHasCommand(root *cobra.Command, name string) bool {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return false
}

func (c *CLI) registerRequestCloser(closer io.Closer) {
	if closer == nil {
		return
	}
	c.requestClosersMu.Lock()
	defer c.requestClosersMu.Unlock()
	c.requestClosers = append(c.requestClosers, closer)
}

func (c *CLI) closeRequestClosers() {
	c.requestClosersMu.Lock()
	defer c.requestClosersMu.Unlock()
	for i := len(c.requestClosers) - 1; i >= 0; i-- {
		_ = c.requestClosers[i].Close()
	}
	c.requestClosers = nil
}

// builtinCommands is the set of top-level subcommand names registered by the
// CLI itself. When the first non-flag argument is one of these, the fast-path
// skips API-name detection and loads all configured APIs.
var builtinCommands = map[string]bool{
	"api": true, "auth-header": true, "cache": true, "cert": true, "completion": true,
	"delete": true, "edit": true, "get": true, "head": true,
	"help": true, "links": true, "options": true, "patch": true,
	"plugin": true, "post": true, "put": true, "setup": true,
	"theme": true,
}

// isBuiltinCommandName reports whether name collides with a top-level built-in
// command, which would prevent the generated API subcommand from being reachable.
func isBuiltinCommandName(name string) bool {
	return builtinCommands[name]
}

// generatedAPINames returns the APIs whose generated commands should be
// registered for this invocation. When the first non-flag positional argument
// names a configured API, only that API's spec is loaded; otherwise all
// configured APIs are registered. This prevents eagerly parsing every cached
// spec on every invocation.
//
// Flags (tokens starting with "-") are skipped when scanning for the first
// positional argument, so `restish -v myapi op` still fast-paths on myapi.
// Flags of the form --flag=value are consumed as a single token; all other
// value-taking flags consume the next token as their value unless it starts
// with "-"; bool/count flags do not consume the API name.
func (c *CLI) generatedAPINames(args []string, cfg *config.Config) []string {
	// Scan past the program name and any leading flags.
	toks := args[1:]
	for len(toks) > 0 {
		t := toks[0]
		toks = toks[1:]
		if !strings.HasPrefix(t, "-") {
			// First positional: if it's a known built-in, break and load all.
			if !builtinCommands[t] {
				if _, ok := cfg.APIs[t]; ok {
					return []string{t}
				}
			}
			break
		}
		// Flag token: --flag=value is self-contained; otherwise only flags
		// that actually take values consume the following token.
		if flagConsumesNextArg(t) && len(toks) > 0 && !strings.HasPrefix(toks[0], "-") {
			toks = toks[1:]
		}
	}

	names := make([]string, 0, len(cfg.APIs))
	for name := range cfg.APIs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func flagConsumesNextArg(token string) bool {
	if strings.Contains(token, "=") {
		return false
	}
	name := strings.TrimLeft(token, "-")
	if strings.HasPrefix(token, "--") {
		_, noValue := boolLikeLongFlags[name]
		return !noValue
	}
	for _, r := range name {
		if !boolLikeShortFlags[r] {
			return true
		}
	}
	return false
}

var boolLikeLongFlags = map[string]bool{
	"help": true, "version": true,
	"rsh-silent": true, "rsh-headers": true, "rsh-raw": true,
	"rsh-verbose": true, "rsh-insecure": true, "rsh-ignore-status-code": true,
	"rsh-no-cache": true, "rsh-no-browser": true, "rsh-no-paginate": true,
	"rsh-collect": true,
}

var boolLikeShortFlags = map[rune]bool{
	'S': true, 'r': true, 'v': true,
}
