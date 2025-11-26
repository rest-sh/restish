package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
)

// apis holds the per-API configuration.
var apis *viper.Viper

// APIAuth describes the auth type and parameters for an API.
type APIAuth struct {
	Name   string            `json:"name" yaml:"name"`
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
}

// TLSConfig contains the TLS setup for the HTTP client
type TLSConfig struct {
	InsecureSkipVerify bool          `json:"insecure,omitempty" yaml:"insecure,omitempty" mapstructure:"insecure"`
	Cert               string        `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key                string        `json:"key,omitempty" yaml:"key,omitempty"`
	CACert             string        `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty" mapstructure:"ca_cert"`
	PKCS11             *PKCS11Config `json:"pkcs11,omitempty" yaml:"pkcs11,omitempty"`
}

// PKCS11Config contains information about how to get a client certificate
// from a hardware device via PKCS#11.
type PKCS11Config struct {
	Path  string `json:"path,omitempty" yaml:"path,omitempty"`
	Label string `json:"label" yaml:"label"`
}

// APIProfile contains account-specific API information
type APIProfile struct {
	Base    string            `json:"base,omitempty" yaml:"base,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Auth    *APIAuth          `json:"auth,omitempty" yaml:"auth,omitempty"`
}

// APIConfig describes per-API configuration options like the base URI and
// auth scheme, if any.
type APIConfig struct {
	name          string
	Base          string                 `json:"base" yaml:"base"`
	OperationBase string                 `json:"operation_base,omitempty" yaml:"operation_base,omitempty" mapstructure:"operation_base,omitempty"`
	SpecFiles     []string               `json:"spec_files,omitempty" yaml:"spec_files,omitempty" mapstructure:"spec_files,omitempty"`
	Profiles      map[string]*APIProfile `json:"profiles,omitempty" yaml:"profiles,omitempty" mapstructure:",omitempty"`
	TLS           *TLSConfig             `json:"tls,omitempty" yaml:"tls,omitempty" mapstructure:",omitempty"`
}

// Save the API configuration to disk.
// If the API came from local config file(s), choose which one to save to.
// Otherwise, save to the global config.
func (a APIConfig) Save() error {
	// Check if this API came from local config file(s)
	if sourcePaths, isLocal := localConfigSources[a.name]; isLocal && len(sourcePaths) > 0 {
		// Use the top-most (closest to cwd) config by default
		targetPath := sourcePaths[len(sourcePaths)-1]
		return a.saveToLocalConfig(targetPath)
	}

	// Save to global config
	apis.Set(a.name, a)
	return apis.WriteConfig()
}

// SaveWithPrompt saves the API configuration, optionally prompting the user
// to choose which config file to save to when multiple sources exist.
func (a APIConfig) SaveWithPrompt(asker asker) error {
	// Check if this API came from local config file(s)
	sourcePaths, isLocal := localConfigSources[a.name]

	if !isLocal || len(sourcePaths) == 0 {
		// Save to global config
		apis.Set(a.name, a)
		return apis.WriteConfig()
	}

	if len(sourcePaths) == 1 {
		// Only one source, save there
		return a.saveToLocalConfig(sourcePaths[0])
	}

	// Multiple sources - let user choose
	options := make([]string, len(sourcePaths))
	for i, path := range sourcePaths {
		// Make paths relative to cwd for easier reading
		if cwd, err := os.Getwd(); err == nil {
			if relPath, err := filepath.Rel(cwd, path); err == nil {
				options[i] = relPath
			} else {
				options[i] = path
			}
		} else {
			options[i] = path
		}
	}
	options = append(options, "Global config")

	choice := asker.askSelect("Choose where to save the configuration", options, nil, "")

	if choice == "Global config" {
		// Save to global config
		apis.Set(a.name, a)
		return apis.WriteConfig()
	}

	// Find the selected path
	for i, opt := range options[:len(sourcePaths)] {
		if opt == choice {
			return a.saveToLocalConfig(sourcePaths[i])
		}
	}

	// Fallback to last path
	return a.saveToLocalConfig(sourcePaths[len(sourcePaths)-1])
}

// saveToLocalConfig saves the API configuration to a specific local config file.
func (a APIConfig) saveToLocalConfig(configPath string) error {
	localApis := viper.New()
	localApis.SetConfigFile(configPath)

	// Try to read existing config
	if err := localApis.ReadInConfig(); err != nil {
		// If file doesn't exist, that's ok, we'll create it
		if !os.IsNotExist(err) {
			return fmt.Errorf("error reading local config %s: %w", configPath, err)
		}
	}

	// Update or add this API's config
	localApis.Set(a.name, a)

	// Write back to the file
	if err := localApis.WriteConfig(); err != nil {
		// If WriteConfig fails because file doesn't exist, try SafeWriteConfig
		if err := localApis.SafeWriteConfig(); err != nil {
			return fmt.Errorf("error writing local config %s: %w", configPath, err)
		}
	}

	return nil
}

// Return colorized string of configuration in JSON or YAML
func (a APIConfig) GetPrettyDisplay(outFormat string) ([]byte, error) {
	// marshal
	if outFormat == "auto" {
		outFormat = "json"
	}
	marshalled, err := MarshalShort(outFormat, true, a)
	if err != nil {
		return nil, errors.New("unable to render configuration")
	}

	if !useColor {
		return marshalled, nil
	}

	// colorize
	marshalled, err = Highlight(outFormat, marshalled)
	if err != nil {
		return nil, errors.New("unable to colorize output")
	}

	return marshalled, nil
}

type apiConfigs map[string]*APIConfig

var configs apiConfigs
var apiCommand *cobra.Command
var localConfigPath string
var localConfigSources map[string][]string // Track ALL config files for each API (from root to cwd)

// deepMergeProfile merges source profile into dest profile.
// Fields in source override fields in dest, except for maps which are merged.
func deepMergeProfile(dest, source *APIProfile) {
	if source.Base != "" {
		dest.Base = source.Base
	}

	// Merge headers
	if len(source.Headers) > 0 {
		if dest.Headers == nil {
			dest.Headers = make(map[string]string)
		}
		for k, v := range source.Headers {
			dest.Headers[k] = v
		}
	}

	// Merge query params
	if len(source.Query) > 0 {
		if dest.Query == nil {
			dest.Query = make(map[string]string)
		}
		for k, v := range source.Query {
			dest.Query[k] = v
		}
	}

	// Override auth if specified
	if source.Auth != nil {
		if dest.Auth == nil {
			dest.Auth = &APIAuth{}
		}
		dest.Auth.Name = source.Auth.Name
		if source.Auth.Params != nil {
			if dest.Auth.Params == nil {
				dest.Auth.Params = make(map[string]string)
			}
			for k, v := range source.Auth.Params {
				dest.Auth.Params[k] = v
			}
		}
	}
}

// deepMergeAPIConfig merges source config into dest config.
// This allows configs to be built up across multiple files.
func deepMergeAPIConfig(dest, source *APIConfig) {
	if source.Base != "" {
		dest.Base = source.Base
	}

	if source.OperationBase != "" {
		dest.OperationBase = source.OperationBase
	}

	// Merge spec files (append, no duplicates)
	if len(source.SpecFiles) > 0 {
		existingSpecs := make(map[string]bool)
		for _, spec := range dest.SpecFiles {
			existingSpecs[spec] = true
		}
		for _, spec := range source.SpecFiles {
			if !existingSpecs[spec] {
				dest.SpecFiles = append(dest.SpecFiles, spec)
			}
		}
	}

	// Deep merge profiles
	if len(source.Profiles) > 0 {
		if dest.Profiles == nil {
			dest.Profiles = make(map[string]*APIProfile)
		}
		for profileName, sourceProfile := range source.Profiles {
			if sourceProfile == nil {
				continue
			}
			if dest.Profiles[profileName] == nil {
				// Profile doesn't exist in dest, create it
				dest.Profiles[profileName] = &APIProfile{}
			}
			deepMergeProfile(dest.Profiles[profileName], sourceProfile)
		}
	}

	// Merge TLS config
	if source.TLS != nil {
		if dest.TLS == nil {
			dest.TLS = &TLSConfig{}
		}
		if source.TLS.InsecureSkipVerify {
			dest.TLS.InsecureSkipVerify = true
		}
		if source.TLS.Cert != "" {
			dest.TLS.Cert = source.TLS.Cert
		}
		if source.TLS.Key != "" {
			dest.TLS.Key = source.TLS.Key
		}
		if source.TLS.CACert != "" {
			dest.TLS.CACert = source.TLS.CACert
		}
		if source.TLS.PKCS11 != nil {
			if dest.TLS.PKCS11 == nil {
				dest.TLS.PKCS11 = &PKCS11Config{}
			}
			if source.TLS.PKCS11.Path != "" {
				dest.TLS.PKCS11.Path = source.TLS.PKCS11.Path
			}
			if source.TLS.PKCS11.Label != "" {
				dest.TLS.PKCS11.Label = source.TLS.PKCS11.Label
			}
		}
	}
}

// validateProfile checks if a profile exists in the given config.
// If not, it returns an error with available profiles listed.
func validateProfile(profileName string, config *APIConfig, configName string) error {
	if profileName == "default" && config.Profiles[profileName] == nil {
		// Default profile is always valid, even if not explicitly defined
		return nil
	}

	if config.Profiles[profileName] != nil {
		return nil
	}

	// Profile doesn't exist, build helpful error message
	available := []string{}
	for p := range config.Profiles {
		available = append(available, p)
	}
	sort.Strings(available)

	if len(available) == 0 {
		return fmt.Errorf("profile '%s' not found for API '%s' (no profiles defined)", profileName, configName)
	}

	return fmt.Errorf("profile '%s' not found for API '%s'. Available profiles: %s",
		profileName, configName, strings.Join(available, ", "))
}

// findAllLocalConfigs searches for all local API configuration files.
// It walks up the directory tree from current directory to root and returns
// all found config files in order from root to current directory.
func findAllLocalConfigs() []string {
	var configPaths []string

	// First check if explicitly specified via flag
	if configPath := viper.GetString("rsh-config"); configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return []string{configPath}
		}
		// If specified but doesn't exist, log and continue
		LogDebug("Specified config file does not exist: %s", configPath)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	// Walk up the directory tree and collect all config paths
	dir := cwd
	for {
		// Check for .restish.json first
		configPath := filepath.Join(dir, ".restish.json")
		if _, err := os.Stat(configPath); err == nil {
			configPaths = append(configPaths, configPath)
		} else {
			// Check for .restish.yaml
			configPath = filepath.Join(dir, ".restish.yaml")
			if _, err := os.Stat(configPath); err == nil {
				configPaths = append(configPaths, configPath)
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	// Reverse the list so configs are loaded from root to current directory
	// This way, configs closer to cwd override those further away
	for i, j := 0, len(configPaths)-1; i < j; i, j = i+1, j-1 {
		configPaths[i], configPaths[j] = configPaths[j], configPaths[i]
	}

	return configPaths
}

// loadLocalConfig loads and merges a local configuration file with the global config.
func loadLocalConfig(configPath string) error {
	localApis := viper.New()

	// Set config file path
	localApis.SetConfigFile(configPath)

	// Read the local config
	if err := localApis.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading local config %s: %w", configPath, err)
	}

	// Get the directory containing the config file for resolving relative paths
	configDir := filepath.Dir(configPath)

	// Merge local configs into global configs
	for k, v := range localApis.AllSettings() {
		if k == "$schema" {
			continue
		}

		// Convert to APIConfig to process spec_files
		tmp := viper.New()
		tmp.Set("config", v)
		var localConfig APIConfig
		if err := tmp.UnmarshalKey("config", &localConfig); err != nil {
			LogError("Error unmarshaling local config for %s: %v", k, err)
			continue
		}

		// Set the name field
		localConfig.name = k

		// Resolve relative spec_files paths
		for i, specFile := range localConfig.SpecFiles {
			// Skip if it's a URL
			if strings.HasPrefix(specFile, "http://") || strings.HasPrefix(specFile, "https://") {
				continue
			}
			// If it's a relative path, make it relative to the config file location
			if !filepath.IsAbs(specFile) {
				localConfig.SpecFiles[i] = filepath.Join(configDir, specFile)
			}
		}

		// Deep merge with existing config if it exists
		existingConfig := configs[k]
		if existingConfig != nil {
			// Config already exists, deep merge
			deepMergeAPIConfig(existingConfig, &localConfig)
			existingConfig.name = k
		} else {
			// New config, just add it
			localConfig.name = k
			configs[k] = &localConfig

			// Set in global apis config
			apis.Set(k, localConfig)
		}

		// Track all config files that contribute to this API
		// They are tracked in order from root to cwd
		if localConfigSources == nil {
			localConfigSources = make(map[string][]string)
		}
		localConfigSources[k] = append(localConfigSources[k], configPath)
	}

	return nil
}

func initAPIConfig() {
	apis = viper.New()

	apis.SetConfigName("apis")
	apis.AddConfigPath(viper.GetString("config-directory"))

	// Write a blank cache if no file is already there. Later you can use
	// configs.SaveConfig() to write new values.
	filename := filepath.Join(viper.GetString("config-directory"), "apis.json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := os.WriteFile(filename, []byte("{}"), 0600); err != nil {
			panic(err)
		}
	}

	err := apis.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if apis.GetString("$schema") == "" {
		// Attempt to update the config to add the schema for docs/validation.
		apis.Set("$schema", "https://rest.sh/schemas/apis.json")
		apis.WriteConfig()
	}

	// Initialize configs map before loading local config
	configs = apiConfigs{}

	// Load all local configurations found up the directory tree
	localConfigPaths := findAllLocalConfigs()
	if len(localConfigPaths) > 0 {
		LogDebug("Found %d local config(s)", len(localConfigPaths))
		for _, configPath := range localConfigPaths {
			LogDebug("Loading local config: %s", configPath)
			if err := loadLocalConfig(configPath); err != nil {
				LogError("Error loading local config %s: %v", configPath, err)
			}
		}
		// Store the closest (last) config path for reference
		localConfigPath = localConfigPaths[len(localConfigPaths)-1]
	}

	// Register api init sub-command to register the API.
	apiCommand = &cobra.Command{
		GroupID: "generic",
		Use:     "api",
		Short:   "API management commands",
	}
	Root.AddCommand(apiCommand)

	apiCommand.AddCommand(&cobra.Command{
		Use:     "content-types",
		Aliases: []string{"ct", "cts"},
		Short:   "Show content types",
		Long:    "Show registered content-type information",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			keys := []string{}
			for k := range contentTypes {
				if contentTypes[k].name != "" {
					keys = append(keys, k)
				}
			}

			// Sort content types by priority
			sort.Slice(keys, func(i, j int) bool {
				return contentTypes[keys[i]].q > contentTypes[keys[j]].q
			})

			fmt.Fprintln(Stdout, "Content types (most to least preferred):")
			for _, k := range keys {
				fmt.Fprintln(Stdout, contentTypes[k].name)
			}

			// Sort output formats alphabetically
			keys = maps.Keys(contentTypes)
			sort.Strings(keys)
			fmt.Fprintln(Stdout, "\nOutput formats:")
			for _, k := range keys {
				fmt.Fprintln(Stdout, k)
			}
		},
	})

	apiCommand.AddCommand(&cobra.Command{
		Use:     "configure short-name",
		Aliases: []string{"config"},
		Short:   "Initialize an API",
		Long:    "Initializes an API with a short interactive prompt session to set up the base URI and auth if needed.",
		Args:    cobra.MinimumNArgs(1),
		Run:     askInitAPIDefault,
	})

	apiCommand.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Edit APIs configuration",
		Long:  "Edit the APIs configuration in your default editor.",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { editAPIs(os.Exit) },
	})

	apiCommand.AddCommand(&cobra.Command{
		Use:   "clear-auth-cache short-name",
		Short: "Clear API auth token cache",
		Long:  "Clear the API auth token cache for the current profile. This will force a re-authentication the next time you make a request.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			apiName := args[0]
			api := configs[apiName]
			if api == nil {
				panic("API " + apiName + " not found")
			}

			// Remove the cache entry.
			Cache.Set(apiName+":"+viper.GetString("rsh-profile"), "")

			if err := Cache.WriteConfig(); err != nil {
				panic(fmt.Errorf("Unable to write cache file: %w", err))
			}
		},
	})

	apiCommand.AddCommand(&cobra.Command{
		Use:   "show short-name",
		Short: "Show API config",
		Long:  "Show an API configuration as JSON/YAML.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			config := configs[args[0]]
			if config == nil {
				panic("API " + args[0] + " not found")
			}

			outFormat := viper.GetString("rsh-output-format")
			if prettyString, err := config.GetPrettyDisplay(outFormat); err == nil {
				Stdout.Write(prettyString)
			} else {
				panic(err)
			}
		},
	})

	apiCommand.AddCommand(&cobra.Command{
		Use:   "sync short-name",
		Short: "Sync an API",
		Long:  "Force-fetch the latest API description and update the local cache.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			viper.Set("rsh-no-cache", true)
			_, err := Load(fixAddress(args[0]), Root)
			if err != nil {
				panic(err)
			}
		},
	})

	// Register API sub-commands
	tmp := viper.New()
	for k, v := range apis.AllSettings() {
		if k == "$schema" {
			continue
		}
		tmp.Set(k, v)
	}
	if err := tmp.Unmarshal(&configs); err != nil {
		panic(err)
	}

	seen := map[string]bool{}
	for apiName, config := range configs {
		func(config *APIConfig) {
			if seen[config.Base] {
				panic(fmt.Errorf("multiple APIs configured with the same base URL: %s", config.Base))
			}
			seen[config.Base] = true
			config.name = apiName
			configs[apiName] = config

			n := apiName
			cmd := &cobra.Command{
				GroupID: "api",
				Use:     n,
				Short:   config.Base,
				Run: func(cmd *cobra.Command, args []string) {
					cmd.Help()
				},
			}
			Root.AddCommand(cmd)
		}(config)
	}
}

func findAPI(uri string) (string, *APIConfig) {
	apiName := viper.GetString("api-name")

	for name, config := range configs {
		// fixes https://github.com/rest-sh/restish/issues/128
		if len(apiName) > 0 && name != apiName {
			continue
		}

		profile := viper.GetString("rsh-profile")
		if profile != "default" {
			if config.Profiles[profile] == nil {
				continue
			}
			if config.Profiles[profile].Base != "" {
				if strings.HasPrefix(uri, config.Profiles[profile].Base) {
					return name, config
				}
			} else if strings.HasPrefix(uri, config.Base) {
				return name, config
			}
		} else {
			if strings.HasPrefix(uri, config.Base) {
				// TODO: find the longest matching base?
				return name, config
			}
		}
	}

	return "", nil
}

func editAPIs(exitFunc func(int)) {
	editor := getEditor()
	if editor == "" {
		fmt.Fprintln(os.Stderr, `Please set the VISUAL or EDITOR environment variable with your preferred editor. Examples:

export VISUAL="code --wait"
export EDITOR="vim"`)
		exitFunc(1)
		return
	}

	parts, err := shlex.Split(editor)
	panicOnErr(err)
	name := parts[0]
	args := append(parts[1:], path.Join(
		getConfigDir(viper.GetString("app-name")), "apis.json",
	))

	c := exec.Command(name, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	panicOnErr(c.Run())
}
