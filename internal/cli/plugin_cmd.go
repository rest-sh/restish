package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	pluginwire "github.com/rest-sh/restish/v2/plugin"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/spf13/cobra"
)

const (
	maxPluginDebugCaptureBytes       = 64 << 20
	defaultPluginDownloadMaxBytes    = 128 << 20
	defaultPluginArchiveMemberBytes  = 128 << 20
	defaultPluginArchiveExtractBytes = 256 << 20
)

type pluginInstallSizeLimits struct {
	DownloadBytes       int64
	ArchiveMemberBytes  int64
	ArchiveExtractBytes int64
}

var pluginInstallLimits = pluginInstallSizeLimits{
	DownloadBytes:       defaultPluginDownloadMaxBytes,
	ArchiveMemberBytes:  defaultPluginArchiveMemberBytes,
	ArchiveExtractBytes: defaultPluginArchiveExtractBytes,
}

// addPluginCommand registers the "plugin" subcommand tree on root.
func (c *CLI) addPluginCommand(root *cobra.Command) {
	pluginCmd := &cobra.Command{
		Use:     "plugin",
		Short:   "Manage restish plugins",
		GroupID: rootGroupPlugin,
	}
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all discovered plugins",
		Args:  cobra.NoArgs,
		RunE:  c.runPluginList,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "install <source>",
		Short: "Install a plugin from a path, URL, PATH command, or GitHub release",
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
	plugins := plugin.Discover(plugin.DefaultPluginDir(), func(path string, err error) {
		c.warnf("plugin %s: %v", filepath.Base(path), err)
	}, c.pluginManifestCachePath(), diagnosticPrefixWriter(c.Stderr))

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

// runPluginInstall installs a plugin binary from a local path, PATH executable,
// direct archive URL, or GitHub release shorthand.
func (c *CLI) runPluginInstall(cmd *cobra.Command, args []string) error {
	resolved, err := c.resolvePluginInstallSource(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if resolved.Cleanup != nil {
		defer resolved.Cleanup()
	}
	if err := c.installResolvedPlugin(resolved); err != nil {
		return err
	}
	c.warnf("installed plugins are trusted executables and may run arbitrary code on future restish invocations")
	fmt.Fprintf(c.Stdout, "Installed plugin %s\n", resolved.Name)
	return nil
}

type resolvedPluginInstallSource struct {
	Path    string
	Name    string
	Cleanup func()
}

type githubRelease struct {
	Assets []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (c *CLI) resolvePluginInstallSource(ctx context.Context, source string) (resolvedPluginInstallSource, error) {
	if source == "" {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: source is required")
	}
	if isHTTPURL(source) {
		return c.downloadPluginURL(ctx, source, "")
	}
	if gh, ok := parseGitHubPluginSource(source); ok {
		return c.downloadGitHubPlugin(ctx, gh)
	}
	if path, ok, err := resolveLocalPluginSource(source); ok || err != nil {
		if err != nil {
			return resolvedPluginInstallSource{}, err
		}
		return resolvedPluginInstallSource{Path: path, Name: filepath.Base(path)}, nil
	}
	return resolvedPluginInstallSource{}, fmt.Errorf("install: cannot access %s", source)
}

func (c *CLI) installResolvedPlugin(resolved resolvedPluginInstallSource) error {
	info, err := os.Stat(resolved.Path)
	if err != nil {
		return fmt.Errorf("install: cannot access %s: %w", resolved.Path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("install: %s is a directory", resolved.Path)
	}

	pluginDir := plugin.DefaultPluginDir()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("install: cannot create plugin dir %s: %w", pluginDir, err)
	}

	dest := filepath.Join(pluginDir, resolved.Name)
	if err := copyFile(resolved.Path, dest); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	_ = os.Chmod(dest, 0o755)
	if _, err := plugin.LoadManifestWithWarnings(dest, diagnosticPrefixWriter(c.Stderr)); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("install: %w", err)
	}
	return nil
}

func resolveLocalPluginSource(source string) (string, bool, error) {
	if path, ok, err := statPluginSource(source); ok || err != nil {
		return path, ok, err
	}
	if looksPathLike(source) {
		return "", false, nil
	}
	path, err := exec.LookPath(source)
	if err == nil {
		return path, true, nil
	}
	return "", false, nil
}

func statPluginSource(source string) (string, bool, error) {
	info, err := os.Stat(source)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrPermission) {
			return "", false, fmt.Errorf("install: cannot access %s: %w", source, err)
		}
		return "", false, nil
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("install: %s is a directory", source)
	}
	return source, true, nil
}

func looksPathLike(source string) bool {
	return strings.Contains(source, "/") || strings.Contains(source, "\\") ||
		strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~")
}

type githubPluginSource struct {
	Owner      string
	Repo       string
	PluginName string
}

func parseGitHubPluginSource(source string) (githubPluginSource, bool) {
	before, pluginShort, ok := strings.Cut(source, ":")
	if !ok || pluginShort == "" || strings.Contains(pluginShort, "/") || strings.Contains(pluginShort, "\\") {
		return githubPluginSource{}, false
	}
	parts := strings.Split(before, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return githubPluginSource{}, false
	}
	if strings.ContainsAny(before, `\:`) || strings.HasPrefix(before, ".") {
		return githubPluginSource{}, false
	}
	return githubPluginSource{
		Owner:      parts[0],
		Repo:       parts[1],
		PluginName: pluginBinaryName(pluginShort),
	}, true
}

func pluginBinaryName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "restish-") {
		return name
	}
	return "restish-" + name
}

func (c *CLI) downloadGitHubPlugin(ctx context.Context, source githubPluginSource) (resolvedPluginInstallSource, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", url.PathEscape(source.Owner), url.PathEscape(source.Repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: github release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "restish/"+Version)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.installHTTPClient().Do(req)
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: github release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: github release %s/%s: %s", source.Owner, source.Repo, resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: github release: %w", err)
	}
	asset, err := selectPluginReleaseAsset(release.Assets, source.PluginName)
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: github release %s/%s: %w", source.Owner, source.Repo, err)
	}
	return c.downloadPluginURL(ctx, asset.BrowserDownloadURL, source.PluginName)
}

func selectPluginReleaseAsset(assets []githubReleaseAsset, pluginName string) (githubReleaseAsset, error) {
	var matches []githubReleaseAsset
	for _, asset := range assets {
		if asset.BrowserDownloadURL == "" {
			continue
		}
		name := strings.ToLower(asset.Name)
		if !hasPluginAssetPrefix(name, strings.ToLower(pluginName)) ||
			!strings.Contains(name, runtime.GOOS) ||
			!strings.Contains(name, runtime.GOARCH) {
			continue
		}
		if !isSupportedPluginDownloadName(name) {
			continue
		}
		matches = append(matches, asset)
	}
	if len(matches) == 0 {
		return githubReleaseAsset{}, fmt.Errorf("no %s asset for %s/%s", pluginName, runtime.GOOS, runtime.GOARCH)
	}
	if len(matches) > 1 {
		var names []string
		for _, match := range matches {
			names = append(names, match.Name)
		}
		return githubReleaseAsset{}, fmt.Errorf("multiple matching assets for %s: %s", pluginName, strings.Join(names, ", "))
	}
	return matches[0], nil
}

func hasPluginAssetPrefix(assetName, pluginName string) bool {
	if assetName == pluginName || assetName == pluginName+".exe" {
		return true
	}
	if !strings.HasPrefix(assetName, pluginName) {
		return false
	}
	next := strings.TrimPrefix(assetName, pluginName)
	return strings.HasPrefix(next, "_") || strings.HasPrefix(next, "-") || strings.HasPrefix(next, ".")
}

func isSupportedPluginDownloadName(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") ||
		strings.HasSuffix(name, ".tgz") ||
		strings.HasSuffix(name, ".zip") ||
		strings.HasSuffix(name, ".exe") ||
		!strings.Contains(filepath.Base(name), ".")
}

func (c *CLI) downloadPluginURL(ctx context.Context, sourceURL, pluginName string) (resolvedPluginInstallSource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: download: %w", err)
	}
	req.Header.Set("User-Agent", "restish/"+Version)
	resp, err := c.installHTTPClient().Do(req)
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: download %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: download %s: %s", sourceURL, resp.Status)
	}

	tempDir, err := os.MkdirTemp("", "restish-plugin-install-*")
	if err != nil {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: create temp dir: %w", err)
	}
	path, err := materializePluginDownload(resp.Body, sourceURL, tempDir, pluginName)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return resolvedPluginInstallSource{}, err
	}
	return resolvedPluginInstallSource{
		Path:    path,
		Name:    filepath.Base(path),
		Cleanup: func() { _ = os.RemoveAll(tempDir) },
	}, nil
}

func materializePluginDownload(r io.Reader, sourceURL, tempDir, pluginName string) (string, error) {
	name := downloadName(sourceURL)
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractPluginTarGz(r, tempDir, pluginName)
	case strings.HasSuffix(lower, ".zip"):
		return extractPluginZip(limitReader(r, pluginInstallLimits.DownloadBytes), tempDir, pluginName)
	default:
		if pluginName == "" {
			pluginName = name
		}
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(name), ".exe") && !strings.HasSuffix(strings.ToLower(pluginName), ".exe") {
			pluginName += ".exe"
		}
		if pluginName == "" {
			return "", fmt.Errorf("install: cannot determine plugin binary name from %s", sourceURL)
		}
		dest := filepath.Join(tempDir, filepath.Base(pluginName))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("install: create %s: %w", dest, err)
		}
		if _, err := copyPluginBytes(out, r, pluginInstallLimits.DownloadBytes); err != nil {
			_ = out.Close()
			return "", fmt.Errorf("install: write %s: %w", dest, err)
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return dest, nil
	}
}

func downloadName(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return filepath.Base(sourceURL)
	}
	return filepath.Base(u.Path)
}

func extractPluginTarGz(r io.Reader, tempDir, pluginName string) (string, error) {
	data, err := readPluginBytes(r, pluginInstallLimits.DownloadBytes)
	if err != nil {
		return "", fmt.Errorf("install: read tar.gz: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("install: read tar.gz: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var candidates []string
	var extracted int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("install: read tar.gz: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if !isWantedPluginArchiveEntry(h.Name, pluginName) {
			continue
		}
		if h.Size > pluginInstallLimits.ArchiveMemberBytes {
			return "", fmt.Errorf("install: extract %s: plugin archive member exceeds limit of %d bytes", h.Name, pluginInstallLimits.ArchiveMemberBytes)
		}
		if extracted+h.Size > pluginInstallLimits.ArchiveExtractBytes {
			return "", fmt.Errorf("install: extract %s: plugin archive exceeds extracted limit of %d bytes", h.Name, pluginInstallLimits.ArchiveExtractBytes)
		}
		dest := filepath.Join(tempDir, filepath.Base(h.Name))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("install: extract %s: %w", h.Name, err)
		}
		n, err := copyPluginBytes(out, tr, pluginInstallLimits.ArchiveMemberBytes)
		if err != nil {
			_ = out.Close()
			return "", fmt.Errorf("install: extract %s: %w", h.Name, err)
		}
		extracted += n
		if err := out.Close(); err != nil {
			return "", err
		}
		candidates = append(candidates, dest)
	}
	return selectExtractedPlugin(candidates, pluginName)
}

func extractPluginZip(r io.Reader, tempDir, pluginName string) (string, error) {
	data, err := readPluginBytes(r, pluginInstallLimits.DownloadBytes)
	if err != nil {
		return "", fmt.Errorf("install: read zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("install: read zip: %w", err)
	}
	var candidates []string
	var extracted int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isWantedPluginArchiveEntry(f.Name, pluginName) {
			continue
		}
		if f.UncompressedSize64 > uint64(pluginInstallLimits.ArchiveMemberBytes) {
			return "", fmt.Errorf("install: extract %s: plugin archive member exceeds limit of %d bytes", f.Name, pluginInstallLimits.ArchiveMemberBytes)
		}
		if extracted+int64(f.UncompressedSize64) > pluginInstallLimits.ArchiveExtractBytes {
			return "", fmt.Errorf("install: extract %s: plugin archive exceeds extracted limit of %d bytes", f.Name, pluginInstallLimits.ArchiveExtractBytes)
		}
		in, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("install: extract %s: %w", f.Name, err)
		}
		dest := filepath.Join(tempDir, filepath.Base(f.Name))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = in.Close()
			return "", fmt.Errorf("install: extract %s: %w", f.Name, err)
		}
		n, copyErr := copyPluginBytes(out, in, pluginInstallLimits.ArchiveMemberBytes)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return "", fmt.Errorf("install: extract %s: %w", f.Name, copyErr)
		}
		extracted += n
		if closeErr != nil {
			return "", closeErr
		}
		candidates = append(candidates, dest)
	}
	return selectExtractedPlugin(candidates, pluginName)
}

func limitReader(r io.Reader, limit int64) io.Reader {
	if limit <= 0 {
		return r
	}
	return io.LimitReader(r, limit+1)
}

func readPluginBytes(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(limitReader(r, limit))
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(data)) > limit {
		return nil, fmt.Errorf("plugin download exceeds limit of %d bytes", limit)
	}
	return data, nil
}

func copyPluginBytes(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	if limit <= 0 {
		return io.Copy(dst, src)
	}
	lr := &io.LimitedReader{R: src, N: limit + 1}
	n, err := io.Copy(dst, lr)
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("plugin download exceeds limit of %d bytes", limit)
	}
	return n, nil
}

func isWantedPluginArchiveEntry(name, pluginName string) bool {
	base := filepath.Base(name)
	if base == "." || base == "" {
		return false
	}
	if pluginName == "" {
		return strings.HasPrefix(strings.TrimSuffix(base, ".exe"), "restish-")
	}
	return strings.TrimSuffix(base, ".exe") == strings.TrimSuffix(pluginName, ".exe")
}

func selectExtractedPlugin(candidates []string, pluginName string) (string, error) {
	if len(candidates) == 0 {
		if pluginName != "" {
			return "", fmt.Errorf("install: archive does not contain %s", pluginName)
		}
		return "", fmt.Errorf("install: archive does not contain a restish-* plugin")
	}
	if len(candidates) > 1 {
		var names []string
		for _, candidate := range candidates {
			names = append(names, filepath.Base(candidate))
		}
		return "", fmt.Errorf("install: archive contains multiple plugin candidates: %s", strings.Join(names, ", "))
	}
	return candidates[0], nil
}

func (c *CLI) installHTTPClient() *http.Client {
	return &http.Client{Transport: c.baseHTTPTransport(), Timeout: 5 * time.Minute}
}

func isHTTPURL(source string) bool {
	u, err := url.Parse(source)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// runPluginRemove deletes a plugin from the plugin directory.
func (c *CLI) runPluginRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := validatePluginName(name); err != nil {
		return err
	}
	pluginDir := plugin.DefaultPluginDir()
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

	// Capture stdout for CBOR decoding only; raw CBOR bytes must not be written
	// to the terminal since they would corrupt it.
	stdoutBuf := &cappedBuffer{limit: maxPluginDebugCaptureBytes}
	pluginCmd.Stdout = stdoutBuf

	if err := pluginCmd.Run(); err != nil {
		// Non-zero exit is reported but not fatal in debug mode.
		fmt.Fprintf(c.Stderr, "plugin exited: %v\n", err)
	}

	// Attempt to decode all CBOR messages from the captured stdout.
	data := stdoutBuf.Bytes()
	if len(data) > 0 {
		dec := pluginwire.NewDecoder(bytes.NewReader(data))
		for {
			var v any
			if decErr := dec.ReadMessage(&v); decErr != nil {
				break
			}
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Fprintf(c.Stderr, "[debug] decoded CBOR message:\n%s\n", b)
		}
	}
	if stdoutBuf.Truncated() {
		c.warnf("plugin debug capture truncated after %d bytes", maxPluginDebugCaptureBytes)
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
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagStdoutTTY, stdoutTTY),
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagStderrTTY, stderrTTY),
		fmt.Sprintf("%s=%v", pluginwire.StartupFlagColor, color),
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
