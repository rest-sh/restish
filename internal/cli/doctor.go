package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		Example: fmt.Sprintf(`  %s doctor
  %s doctor -o json
  %s doctor api demo --check-network`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.NoArgs,
		RunE: c.runDoctor,
	}
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "api <name>",
		Short: "Diagnose a registered API",
		Example: fmt.Sprintf(`  %s doctor api demo
  %s doctor api demo --check-network`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.ExactArgs(1),
		RunE: c.runDoctorAPI,
	})
	doctorAPI := doctorCmd.Commands()[0]
	doctorAPI.Flags().Bool("check-network", false, "Make a bounded network request to check API reachability")
	doctorCmd.AddCommand(&cobra.Command{
		Use:     "plugin <name>",
		Short:   "Diagnose a Restish plugin executable",
		Example: fmt.Sprintf("  %s doctor plugin mcp", c.commandNameOrDefault()),
		Args:    cobra.ExactArgs(1),
		RunE:    c.runDoctorPlugin,
	})
	doctorCmd.AddCommand(&cobra.Command{
		Use:   "migrate-v1",
		Short: "Run default-location v1 config migration if eligible",
		Example: fmt.Sprintf(`  %s doctor migrate-v1
  %s doctor migrate-v1 --to ./restish.json`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.NoArgs,
		RunE: c.runDoctorMigrateV1,
	})
	root.AddCommand(doctorCmd)
}

type doctorConfigParseReport struct {
	Status        string                     `json:"status"`
	APICount      int                        `json:"api_count,omitempty"`
	Error         string                     `json:"error,omitempty"`
	UnknownFields []doctorUnknownFieldReport `json:"unknown_fields,omitempty"`
}

type doctorUnknownFieldReport struct {
	Path       string `json:"path"`
	Field      string `json:"field,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

type doctorPermissionReport struct {
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type doctorShellSetupReport struct {
	Status string `json:"status"`
	Shell  string `json:"shell,omitempty"`
	Source string `json:"source,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

type doctorRootReport struct {
	ConfigFile            string                  `json:"config_file"`
	ConfigParse           doctorConfigParseReport `json:"config_parse"`
	ConfigPermissions     doctorPermissionReport  `json:"config_permissions"`
	HTTPCache             string                  `json:"http_cache"`
	SpecCache             string                  `json:"spec_cache"`
	TokenCache            string                  `json:"token_cache"`
	TokenCachePermissions doctorPermissionReport  `json:"token_cache_permissions"`
	PluginDirectory       string                  `json:"plugin_directory"`
	ShellSetup            doctorShellSetupReport  `json:"shell_setup"`
}

type doctorStatusReport struct {
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type doctorGeneratedOperationsReport struct {
	Status    string   `json:"status"`
	Count     int      `json:"count,omitempty"`
	Stale     bool     `json:"stale,omitempty"`
	FetchedAt string   `json:"fetched_at,omitempty"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Issues    []string `json:"issues,omitempty"`
}

type doctorAuthReport struct {
	Status  string   `json:"status"`
	Sources []string `json:"sources,omitempty"`
	Issues  []string `json:"issues,omitempty"`
}

type doctorReachabilityReport struct {
	Status     string `json:"status"`
	Checked    bool   `json:"checked"`
	Reachable  bool   `json:"reachable,omitempty"`
	Method     string `json:"method,omitempty"`
	HTTPStatus string `json:"http_status,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
	Note       string `json:"note,omitempty"`
}

type doctorAPIReport struct {
	API                 string                          `json:"api"`
	Registered          bool                            `json:"registered"`
	BaseURL             string                          `json:"base_url,omitempty"`
	SpecURL             string                          `json:"spec_url,omitempty"`
	SpecFiles           []string                        `json:"spec_files,omitempty"`
	SpecCache           doctorStatusReport              `json:"spec_cache"`
	GeneratedOperations doctorGeneratedOperationsReport `json:"generated_operations"`
	Auth                doctorAuthReport                `json:"auth"`
	Reachability        doctorReachabilityReport        `json:"reachability"`
}

type doctorOperationInfo struct {
	Set         spec.OperationSet
	CacheStatus spec.OperationCacheStatus
	Available   bool
	Cached      bool
}

type doctorManifestReport struct {
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Capabilities string `json:"capabilities,omitempty"`
	APIVersion   int    `json:"api_version,omitempty"`
}

type doctorPluginReport struct {
	Plugin     string               `json:"plugin"`
	Path       string               `json:"path"`
	Found      bool                 `json:"found"`
	Executable bool                 `json:"executable,omitempty"`
	Error      string               `json:"error,omitempty"`
	Manifest   doctorManifestReport `json:"manifest"`
}

type doctorMigrationReport struct {
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	ConfigFile string `json:"config_file,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
	BackupPath string `json:"backup_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (c *CLI) runDoctor(cmd *cobra.Command, args []string) error {
	jsonOutput, err := doctorJSON(cmd)
	if err != nil {
		return err
	}
	if jsonOutput {
		return c.writeDoctorJSON(c.doctorRootReport())
	}
	out := c.doctorTextOutput()
	cfgPath := c.configFilePath()
	fmt.Fprintf(out, "Config file: %s\n", cfgPath)
	if cfg, err := c.loadConfig(); err != nil {
		fmt.Fprintf(out, "Config parse: invalid\n  %v\n", err)
		c.printConfigDiagnostics(out, cfgPath)
	} else if err := c.validateConfigRuntime(cfg); err != nil {
		fmt.Fprintf(out, "Config parse: invalid\n  %v\n", err)
		c.printConfigDiagnostics(out, cfgPath)
	} else {
		apiCount := 0
		if cfg.APIs != nil {
			apiCount = len(cfg.APIs)
		}
		fmt.Fprintf(out, "Config parse: ok (%d APIs)\n", apiCount)
		c.printConfigDiagnostics(out, cfgPath)
	}
	if insecure, err := config.ConfigFileHasInsecurePermissions(cfgPath); err != nil {
		fmt.Fprintf(out, "Config permissions: unknown (%v)\n", err)
	} else if insecure {
		fmt.Fprintf(out, "Config permissions: insecure (run chmod 600 %s)\n", cfgPath)
	} else {
		fmt.Fprintln(out, "Config permissions: ok")
	}
	fmt.Fprintf(out, "HTTP cache: %s\n", c.configScopedCacheDir(c.paths().Cache()))
	fmt.Fprintf(out, "Spec cache: %s\n", c.specCacheDir())
	tokenCachePath := c.tokenCachePath()
	fmt.Fprintf(out, "Token cache: %s\n", tokenCachePath)
	if insecure, err := config.ConfigFileHasInsecurePermissions(tokenCachePath); err != nil {
		fmt.Fprintf(out, "Token cache permissions: unknown (%v)\n", err)
	} else if insecure {
		fmt.Fprintf(out, "Token cache permissions: insecure (run chmod 600 %s before the next OAuth request)\n", tokenCachePath)
	} else {
		fmt.Fprintln(out, "Token cache permissions: ok")
	}
	fmt.Fprintf(out, "Plugin directory: %s\n", c.pluginDir())
	c.printShellSetupDiagnostic(out)
	return nil
}

func (c *CLI) runDoctorAPI(cmd *cobra.Command, args []string) error {
	jsonOutput, err := doctorJSON(cmd)
	if err != nil {
		return err
	}
	if jsonOutput {
		return c.writeDoctorJSON(c.doctorAPIReport(cmd, args[0]))
	}
	out := c.doctorTextOutput()
	cfg, err := c.loadConfig()
	if err != nil {
		fmt.Fprintf(out, "Config parse: invalid\n  %v\n", err)
		return nil
	}
	name := args[0]
	api := cfg.APIs[name]
	if api == nil {
		fmt.Fprintf(out, "API %q: not registered\n", name)
		return nil
	}
	fmt.Fprintf(out, "API %q: registered\n", name)
	fmt.Fprintf(out, "Base URL: %s\n", api.BaseURL)
	if api.SpecURL != "" {
		fmt.Fprintf(out, "Spec URL: %s\n", api.SpecURL)
	}
	if len(api.SpecFiles) > 0 {
		fmt.Fprintf(out, "Spec files: %v\n", api.SpecFiles)
	}
	profileName := c.profileFromCmd(cmd)
	opInfo := c.doctorOperationSetStatus(requestContext(cmd), name, api, profileName)
	if _, ok := configFileExists(filepath.Join(c.specCacheDir(), name+".cbor")); ok {
		if opInfo.Cached && opInfo.CacheStatus.Stale {
			fmt.Fprintf(out, "Spec cache: stale (last synced %s, expired %s)\n", formatCacheTime(opInfo.CacheStatus.FetchedAt), formatCacheTime(opInfo.CacheStatus.ExpiresAt))
		} else if opInfo.Cached {
			fmt.Fprintf(out, "Spec cache: fresh (last synced %s, expires %s)\n", formatCacheTime(opInfo.CacheStatus.FetchedAt), formatCacheTime(opInfo.CacheStatus.ExpiresAt))
		} else {
			fmt.Fprintln(out, "Spec cache: present")
		}
	} else {
		fmt.Fprintln(out, "Spec cache: missing")
	}
	if opInfo.Available {
		if opInfo.Cached && opInfo.CacheStatus.Stale {
			fmt.Fprintf(out, "Generated operations: %d available (stale; refresh with \"restish api sync %s\")\n", len(opInfo.Set.Operations), name)
		} else {
			fmt.Fprintf(out, "Generated operations: %d available\n", len(opInfo.Set.Operations))
		}
		for _, issue := range operationSecurityIssues(opInfo.Set.Operations) {
			fmt.Fprintf(out, "  Issue: %s\n", issue)
		}
	} else {
		fmt.Fprintln(out, "Generated operations: unavailable")
	}
	if auth := c.doctorAuthForProfile(name, profileName, profileForName(api, profileName)); auth.Status == "configured" {
		if len(auth.Sources) > 0 {
			fmt.Fprintf(out, "Auth: configured (%s)\n", strings.Join(auth.Sources, ", "))
		} else {
			fmt.Fprintln(out, "Auth: configured")
		}
	} else if auth.Status == "configured-but-unresolved" {
		fmt.Fprintf(out, "Auth: configured but unresolved%s", formatAuthIssues(auth.Issues))
		if len(auth.Sources) > 0 {
			fmt.Fprintf(out, " (%s)", strings.Join(auth.Sources, ", "))
		}
		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "Auth: no profile auth configured")
	}
	checkNetwork, _ := cmd.Flags().GetBool("check-network")
	if checkNetwork {
		c.printAPIReachability(out, c.checkAPIReachability(requestContext(cmd), effectiveProfileBaseURL(api, profileName), api, profileName))
	} else {
		fmt.Fprintln(out, "Reachability: skipped (use --check-network)")
	}
	return nil
}

func (c *CLI) runDoctorPlugin(cmd *cobra.Command, args []string) error {
	jsonOutput, err := doctorJSON(cmd)
	if err != nil {
		return err
	}
	if jsonOutput {
		return c.writeDoctorJSON(c.doctorPluginReport(args[0]))
	}
	out := c.doctorTextOutput()
	name := args[0]
	path := name
	if !filepath.IsAbs(path) && filepath.Base(path) == path {
		path = filepath.Join(c.pluginDir(), name)
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(out, "Plugin %q: not found at %s\n", name, path)
		return nil
	}
	if err != nil {
		fmt.Fprintf(out, "Plugin %q: stat failed: %v\n", name, err)
		return nil
	}
	fmt.Fprintf(out, "Plugin %q: found at %s\n", name, path)
	if info.Mode()&0o111 == 0 {
		fmt.Fprintln(out, "Executable: no")
	} else {
		fmt.Fprintln(out, "Executable: yes")
	}
	manifest, err := internalplugin.LoadManifest(path, diagnosticPrefixWriter(c.Stderr))
	if err != nil {
		fmt.Fprintf(out, "Manifest: invalid (%v)\n", err)
		return nil
	}
	fmt.Fprintf(out, "Manifest: %s %s\n", manifest.Name, manifest.Version)
	fmt.Fprintf(out, "Declared capabilities: %s\n", pluginCapabilitySummary(*manifest))
	fmt.Fprintf(out, "Protocol startup: ok (API v%d)\n", manifest.RestishAPIVersion)
	return nil
}

func (c *CLI) runDoctorMigrateV1(cmd *cobra.Command, args []string) error {
	jsonOutput, err := doctorJSON(cmd)
	if err != nil {
		return err
	}
	if jsonOutput {
		return c.writeDoctorJSON(c.doctorMigrationReport())
	}
	out := c.doctorTextOutput()
	if c.explicitConfigFile {
		fmt.Fprintln(out, "Migration: skipped (explicit config file selected)")
		return nil
	}
	if _, err := os.Stat(c.configFilePath()); err == nil {
		fmt.Fprintf(out, "Migration: skipped (%s already exists)\n", c.configFilePath())
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(out, "Migration: cannot inspect %s: %v\n", c.configFilePath(), err)
		return nil
	}
	cfg, err := c.loadConfig()
	if err != nil {
		fmt.Fprintf(out, "Migration: failed\n  %v\n", err)
		return nil
	}
	if cfg.Migration == nil {
		fmt.Fprintln(out, "Migration: no eligible v1 config found")
		return nil
	}
	fmt.Fprintf(out, "Migration: migrated v1 config from %s\n", cfg.Migration.SourcePath)
	fmt.Fprintf(out, "Backup: %s\n", cfg.Migration.BackupPath)
	return nil
}

func (c *CLI) doctorTextOutput() io.Writer {
	if c.doctorStdoutIsTerminal() {
		return c.Stderr
	}
	fmt.Fprintln(c.Stderr, "Use -o json for machine-readable output.")
	return c.Stdout
}

func (c *CLI) doctorStdoutIsTerminal() bool {
	return c.stdoutIsTerminal()
}

func doctorJSON(cmd *cobra.Command) (bool, error) {
	return commandJSONOutputRequested(cmd)
}

func (c *CLI) writeDoctorJSON(report any) error {
	return c.writePrettyJSON(report)
}

func (c *CLI) doctorRootReport() doctorRootReport {
	cfgPath := c.configFilePath()
	return doctorRootReport{
		ConfigFile:            cfgPath,
		ConfigParse:           c.doctorConfigParseReport(cfgPath),
		ConfigPermissions:     doctorFilePermissionReport(cfgPath, "run chmod 600 "+cfgPath),
		HTTPCache:             c.configScopedCacheDir(c.paths().Cache()),
		SpecCache:             c.specCacheDir(),
		TokenCache:            c.tokenCachePath(),
		TokenCachePermissions: doctorFilePermissionReport(c.tokenCachePath(), "run chmod 600 "+c.tokenCachePath()+" before the next OAuth request"),
		PluginDirectory:       c.pluginDir(),
		ShellSetup:            doctorShellSetupReportValue(),
	}
}

func (c *CLI) doctorConfigParseReport(path string) doctorConfigParseReport {
	report := doctorConfigParseReport{Status: "ok"}
	if cfg, err := c.loadConfig(); err != nil {
		report.Status = "invalid"
		report.Error = err.Error()
	} else if cfg != nil {
		if cfg.APIs != nil {
			report.APICount = len(cfg.APIs)
		}
		if err := c.validateConfigRuntime(cfg); err != nil {
			report.Status = "invalid"
			report.Error = err.Error()
		}
	}
	if diags, err := config.DiagnoseConfig(path); err == nil {
		for _, diag := range diags.UnknownFields {
			report.UnknownFields = append(report.UnknownFields, doctorUnknownFieldReport{
				Path:       diag.Path,
				Field:      diag.Field,
				Line:       diag.Line,
				Column:     diag.Column,
				Suggestion: diag.Suggestion,
				Hint:       diag.Hint,
			})
		}
	}
	return report
}

func doctorFilePermissionReport(path, remediation string) doctorPermissionReport {
	if insecure, err := config.ConfigFileHasInsecurePermissions(path); err != nil {
		return doctorPermissionReport{Status: "unknown", Error: err.Error()}
	} else if insecure {
		return doctorPermissionReport{Status: "insecure", Remediation: remediation}
	}
	return doctorPermissionReport{Status: "ok"}
}

func doctorShellSetupReportValue() doctorShellSetupReport {
	shell, source := detectRunningShell()
	if shell == "" {
		return doctorShellSetupReport{Status: "unknown"}
	}
	if _, ok := shellSetups[shell]; !ok {
		return doctorShellSetupReport{Status: "unsupported", Shell: shell, Source: source}
	}
	hint := fmt.Sprintf("run `restish shell setup %s` if glob expansion causes trouble", shell)
	if source == "$SHELL" {
		hint += " (detected via $SHELL)"
	}
	return doctorShellSetupReport{Status: "recommended", Shell: shell, Source: source, Hint: hint}
}

func (c *CLI) doctorAPIReport(cmd *cobra.Command, name string) doctorAPIReport {
	report := doctorAPIReport{
		API:                 name,
		SpecCache:           doctorStatusReport{Status: "missing"},
		GeneratedOperations: doctorGeneratedOperationsReport{Status: "unavailable"},
		Auth:                doctorAuthReport{Status: "unknown"},
		Reachability:        doctorReachabilityReport{Status: "skipped", Checked: false, Note: "use --check-network"},
	}
	cfg, err := c.loadConfig()
	if err != nil {
		report.SpecCache = doctorStatusReport{Status: "unknown", Error: err.Error()}
		report.GeneratedOperations = doctorGeneratedOperationsReport{Status: "unknown"}
		report.Auth = doctorAuthReport{Status: "unknown"}
		return report
	}
	api := cfg.APIs[name]
	if api == nil {
		report.Auth = doctorAuthReport{Status: "none"}
		return report
	}
	report.Registered = true
	report.BaseURL = api.BaseURL
	report.SpecURL = api.SpecURL
	report.SpecFiles = append([]string(nil), api.SpecFiles...)
	profileName := c.profileFromCmd(cmd)
	opInfo := c.doctorOperationSetStatus(requestContext(cmd), name, api, profileName)
	if _, ok := configFileExists(filepath.Join(c.specCacheDir(), name+".cbor")); ok {
		report.SpecCache = doctorStatusReport{Status: "present"}
		if opInfo.Cached {
			report.SpecCache.FetchedAt = opInfo.CacheStatus.FetchedAt.Format(time.RFC3339)
			report.SpecCache.ExpiresAt = opInfo.CacheStatus.ExpiresAt.Format(time.RFC3339)
			if opInfo.CacheStatus.Stale {
				report.SpecCache.Status = "stale"
			} else {
				report.SpecCache.Status = "fresh"
			}
		}
	}
	if opInfo.Available {
		report.GeneratedOperations = doctorGeneratedOperationsReport{
			Status: "available",
			Count:  len(opInfo.Set.Operations),
			Issues: operationSecurityIssues(opInfo.Set.Operations),
		}
		if opInfo.Cached {
			report.GeneratedOperations.Stale = opInfo.CacheStatus.Stale
			report.GeneratedOperations.FetchedAt = opInfo.CacheStatus.FetchedAt.Format(time.RFC3339)
			report.GeneratedOperations.ExpiresAt = opInfo.CacheStatus.ExpiresAt.Format(time.RFC3339)
		}
	}
	report.Auth = c.doctorAuthForProfile(name, profileName, profileForName(api, profileName))
	checkNetwork, _ := cmd.Flags().GetBool("check-network")
	if checkNetwork {
		report.Reachability = c.checkAPIReachability(requestContext(cmd), effectiveProfileBaseURL(api, profileName), api, profileName)
	}
	return report
}

func (c *CLI) doctorOperationSetStatus(ctx context.Context, apiName string, apiCfg *config.APIConfig, profileName string) doctorOperationInfo {
	if set, status, ok := c.cachedOperationSetStatusForAPI(apiName, apiCfg, profileName); ok {
		return doctorOperationInfo{
			Set:         set,
			CacheStatus: status,
			Available:   true,
			Cached:      true,
		}
	}
	if apiCfg == nil || !spec.HasLocalSpecFiles(apiCfg.SpecFiles) {
		return doctorOperationInfo{}
	}
	set, ok, err := c.operationSetForAPI(ctx, apiName, apiCfg, profileName, false)
	if err != nil || !ok {
		return doctorOperationInfo{}
	}
	return doctorOperationInfo{
		Set:       set,
		Available: true,
	}
}

func (c *CLI) doctorAuthForProfile(apiName, profileName string, prof *config.ProfileConfig) doctorAuthReport {
	if prof == nil {
		return doctorAuthReport{Status: "none"}
	}
	var sources []string
	var readiness []authReadiness
	if prof.Auth != nil || prof.AuthRef != "" {
		sources = append(sources, "profile_auth")
		_, ready, _ := c.profileAuthReadiness(apiName, profileName, prof)
		readiness = append(readiness, ready)
	}
	if len(prof.Credentials) > 0 {
		sources = append(sources, "credentials")
		for id, credential := range prof.Credentials {
			_, ready, _ := c.credentialReadiness(apiName, profileName, id, credential)
			readiness = append(readiness, ready)
		}
	}
	sources = append(sources, profileCredentialSettingSources(prof)...)
	if len(sources) == 0 {
		return doctorAuthReport{Status: "none"}
	}
	issues := authReadinessIssues(readiness...)
	if len(issues) > 0 {
		return doctorAuthReport{Status: "configured-but-unresolved", Sources: sources, Issues: issues}
	}
	return doctorAuthReport{Status: "configured", Sources: sources}
}

func (c *CLI) doctorPluginReport(name string) doctorPluginReport {
	path := name
	if !filepath.IsAbs(path) && filepath.Base(path) == path {
		path = filepath.Join(c.pluginDir(), name)
	}
	report := doctorPluginReport{
		Plugin:   name,
		Path:     path,
		Manifest: doctorManifestReport{Status: "not_checked"},
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		report.Error = "not found"
		return report
	}
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Found = true
	report.Executable = info.Mode()&0o111 != 0
	manifest, err := internalplugin.LoadManifest(path, diagnosticPrefixWriter(c.Stderr))
	if err != nil {
		report.Manifest = doctorManifestReport{Status: "invalid", Error: err.Error()}
		return report
	}
	report.Manifest = doctorManifestReport{
		Status:       "ok",
		Name:         manifest.Name,
		Version:      manifest.Version,
		Capabilities: pluginCapabilitySummary(*manifest),
		APIVersion:   manifest.RestishAPIVersion,
	}
	return report
}

func (c *CLI) doctorMigrationReport() doctorMigrationReport {
	report := doctorMigrationReport{ConfigFile: c.configFilePath()}
	if c.explicitConfigFile {
		report.Status = "skipped"
		report.Reason = "explicit config file selected"
		return report
	}
	if _, err := os.Stat(c.configFilePath()); err == nil {
		report.Status = "skipped"
		report.Reason = c.configFilePath() + " already exists"
		return report
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		report.Status = "failed"
		report.Error = err.Error()
		return report
	}
	cfg, err := c.loadConfig()
	if err != nil {
		report.Status = "failed"
		report.Error = err.Error()
		return report
	}
	if cfg.Migration == nil {
		report.Status = "none"
		report.Reason = "no eligible v1 config found"
		return report
	}
	report.Status = "migrated"
	report.SourcePath = cfg.Migration.SourcePath
	report.BackupPath = cfg.Migration.BackupPath
	return report
}

func configFileExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}

func (c *CLI) printConfigDiagnostics(out io.Writer, path string) {
	diags, err := config.DiagnoseConfig(path)
	if err != nil {
		return
	}
	for _, diag := range diags.UnknownFields {
		if diag.Line > 0 {
			fmt.Fprintf(out, "Unknown field: %s at %d:%d\n", diag.Path, diag.Line, diag.Column)
		} else {
			fmt.Fprintf(out, "Unknown field: %s\n", diag.Path)
		}
		if diag.Suggestion != "" {
			fmt.Fprintf(out, "  Did you mean %q?\n", diag.Suggestion)
		}
		if diag.Hint != "" {
			fmt.Fprintf(out, "  %s\n", diag.Hint)
		}
	}
}

func (c *CLI) printShellSetupDiagnostic(out io.Writer) {
	shell, source := detectRunningShell()
	if shell == "" {
		fmt.Fprintln(out, "Shell setup: unknown")
		return
	}
	if _, ok := shellSetups[shell]; !ok {
		fmt.Fprintf(out, "Shell setup: unsupported shell %s\n", shell)
		return
	}
	if source == "$SHELL" {
		fmt.Fprintf(out, "Shell setup: run `restish shell setup %s` if glob expansion causes trouble (detected via $SHELL)\n", shell)
		return
	}
	fmt.Fprintf(out, "Shell setup: run `restish shell setup %s` if glob expansion causes trouble\n", shell)
}

func (c *CLI) checkAPIReachability(ctx context.Context, baseURL string, apiCfg *config.APIConfig, profileName string) doctorReachabilityReport {
	if baseURL == "" {
		return doctorReachabilityReport{Status: "skipped", Checked: false, Note: "no base URL"}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL, nil)
	if err != nil {
		return doctorReachabilityReport{Status: "invalid_url", Checked: false, Method: http.MethodHead, Error: err.Error()}
	}
	transport, closer, err := c.discoveryTransport(ctx, apiCfg, profileName)
	if err != nil {
		return doctorReachabilityReport{Status: "tls_config_error", Checked: false, Method: http.MethodHead, Error: err.Error(), Note: "profile TLS settings could not be resolved"}
	}
	if closer != nil {
		defer closer.Close()
	}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return doctorReachabilityReport{Status: "failed", Checked: true, Method: http.MethodHead, Error: err.Error()}
	}
	_ = resp.Body.Close()
	report := doctorReachabilityReport{
		Status:     "ok",
		Checked:    true,
		Method:     http.MethodHead,
		HTTPStatus: resp.Status,
		StatusCode: resp.StatusCode,
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		report.Reachable = true
	case resp.StatusCode == http.StatusMethodNotAllowed:
		report.Reachable = true
		report.Note = "HEAD not allowed, but the server responded"
	default:
		report.Status = "warn"
		if resp.StatusCode >= 500 {
			report.Note = "server error; base URL may be wrong"
		} else {
			report.Note = "HTTP error; base URL may require authentication or may be wrong"
		}
	}
	return report
}

func (c *CLI) printAPIReachability(out io.Writer, report doctorReachabilityReport) {
	switch report.Status {
	case "skipped":
		if report.Note == "no base URL" {
			fmt.Fprintln(out, "Reachability: skipped (no base URL)")
		} else {
			fmt.Fprintln(out, "Reachability: skipped (use --check-network)")
		}
	case "invalid_url":
		fmt.Fprintf(out, "Reachability: invalid base URL (%v)\n", report.Error)
	case "failed":
		fmt.Fprintf(out, "Reachability: failed (%v)\n", report.Error)
	case "ok":
		if report.StatusCode == http.StatusMethodNotAllowed {
			fmt.Fprintf(out, "Reachability: HTTP %s (network ok; HEAD not allowed)\n", report.HTTPStatus)
			return
		}
		fmt.Fprintf(out, "Reachability: HTTP %s\n", report.HTTPStatus)
	case "warn":
		if report.Note != "" {
			fmt.Fprintf(out, "Reachability: HTTP %s (%s)\n", report.HTTPStatus, report.Note)
			return
		}
		fmt.Fprintf(out, "Reachability: HTTP %s (warning)\n", report.HTTPStatus)
	}
}
