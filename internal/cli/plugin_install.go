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

	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/spf13/cobra"
)

const (
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

func (c *CLI) runPluginInstall(cmd *cobra.Command, args []string) error {
	resolved, err := c.resolvePluginInstallSource(cmd.Context(), args)
	if err != nil {
		return err
	}
	if resolved.Cleanup != nil {
		defer resolved.Cleanup()
	}
	fmt.Fprintf(c.Stderr, "Plugin source: %s\n", strings.Join(args, " "))
	fmt.Fprintf(c.Stderr, "Resolved path: %s\n", resolved.Path)
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		ok, err := c.Confirm(cmd.Context(), "Inspect and trust this plugin? This runs it once to read its manifest. [y/N] ")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("install: confirmation required; rerun with --yes for automation")
		}
	}
	manifest, err := plugin.LoadManifest(resolved.Path, diagnosticPrefixWriter(c.Stderr))
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := validatePluginManifestName(manifest.Name); err != nil {
		return fmt.Errorf("install: manifest name: %w", err)
	}
	fmt.Fprintf(c.Stderr, "Manifest: %s %s\n", manifest.Name, manifest.Version)
	fmt.Fprintf(c.Stderr, "Capabilities: %s\n", pluginCapabilitySummary(*manifest))
	installedName, err := c.installResolvedPlugin(resolved, *manifest)
	if err != nil {
		return err
	}
	c.warnf("installed plugins are trusted executables and may run arbitrary code on future restish invocations")
	fmt.Fprintf(c.Stdout, "Installed plugin %s\n", installedName)
	return nil
}

func pluginCapabilitySummary(m plugin.Manifest) string {
	var caps []string
	caps = append(caps, m.Hooks...)
	if pluginDeclaresHook(m, "formatter") && len(m.FormatterNames) > 0 {
		caps = append(caps, "formatter("+strings.Join(m.FormatterNames, ",")+")")
	}
	if pluginDeclaresHook(m, "loader") && len(m.LoaderContentTypes) > 0 {
		caps = append(caps, "loader("+strings.Join(m.LoaderContentTypes, ",")+")")
	}
	if len(caps) == 0 {
		return "(none)"
	}
	return strings.Join(caps, ", ")
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

func (c *CLI) resolvePluginInstallSource(ctx context.Context, args []string) (resolvedPluginInstallSource, error) {
	source := args[0]
	if source == "" {
		return resolvedPluginInstallSource{}, fmt.Errorf("install: source is required")
	}
	if len(args) == 2 {
		gh, ok, err := parseGitHubPluginSource(source, args[1])
		if err != nil {
			return resolvedPluginInstallSource{}, err
		}
		if !ok {
			return resolvedPluginInstallSource{}, fmt.Errorf("install: plugin name is only supported with GitHub owner/repo shorthand")
		}
		return c.downloadGitHubPlugin(ctx, gh)
	}
	if isHTTPURL(source) {
		return c.downloadPluginURL(ctx, source, "")
	}
	if path, ok, err := resolveLocalPluginSource(source); ok || err != nil {
		if err != nil {
			return resolvedPluginInstallSource{}, err
		}
		return resolvedPluginInstallSource{Path: path, Name: filepath.Base(path)}, nil
	}
	return resolvedPluginInstallSource{}, fmt.Errorf("install: cannot access %s", source)
}

func (c *CLI) installResolvedPlugin(resolved resolvedPluginInstallSource, manifest plugin.Manifest) (string, error) {
	info, err := os.Stat(resolved.Path)
	if err != nil {
		return "", fmt.Errorf("install: cannot access %s: %w", resolved.Path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("install: %s is a directory", resolved.Path)
	}

	pluginDir := c.pluginDir()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return "", fmt.Errorf("install: cannot create plugin dir %s: %w", pluginDir, err)
	}

	installedName, err := installedPluginFileName(manifest.Name, resolved.Name)
	if err != nil {
		return "", fmt.Errorf("install: manifest name: %w", err)
	}
	dest := filepath.Join(pluginDir, installedName)
	if !pathWithinDir(pluginDir, dest) {
		return "", fmt.Errorf("install: destination %s escapes plugin dir %s", dest, pluginDir)
	}
	tmpDir, err := os.MkdirTemp(pluginDir, "."+installedName+".install-*")
	if err != nil {
		return "", fmt.Errorf("install: cannot create temp plugin dir in %s: %w", pluginDir, err)
	}
	tmpPath := filepath.Join(tmpDir, installedName)
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := pluginInstallCopyFile(resolved.Path, tmpPath); err != nil {
		return "", fmt.Errorf("install: %w", err)
	}
	_ = os.Chmod(tmpPath, 0o755)
	if _, err := plugin.LoadManifest(tmpPath, diagnosticPrefixWriter(c.Stderr)); err != nil {
		return "", fmt.Errorf("install: %w", err)
	}
	if err := replaceInstalledPlugin(tmpPath, dest); err != nil {
		return "", fmt.Errorf("install: replace %s: %w", dest, err)
	}
	cleanupTemp = false
	_ = os.RemoveAll(tmpDir)
	return installedName, nil
}

func replaceInstalledPlugin(tmpPath, dest string) error {
	if err := os.Rename(tmpPath, dest); err == nil {
		return nil
	} else if _, statErr := os.Stat(dest); statErr != nil {
		return err
	}

	backup := dest + ".old-" + filepath.Base(tmpPath)
	if err := os.Rename(dest, backup); err != nil {
		return err
	}
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Rename(backup, dest)
		}
	}()
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	renamed = true
	_ = os.Remove(backup)
	return nil
}

func installedPluginFileName(manifestName, sourceName string) (string, error) {
	name := strings.TrimSpace(manifestName)
	name = strings.TrimPrefix(name, "restish-")
	if name == "" {
		name = strings.TrimPrefix(strings.TrimSuffix(sourceName, filepath.Ext(sourceName)), "restish-")
	}
	if err := validatePluginManifestName(name); err != nil {
		return "", err
	}
	installed := "restish-" + name
	if runtime.GOOS == "windows" && filepath.Ext(installed) == "" && strings.EqualFold(filepath.Ext(sourceName), ".exe") {
		installed += ".exe"
	}
	return installed, nil
}

func validatePluginManifestName(name string) error {
	name = strings.TrimSpace(strings.TrimPrefix(name, "restish-"))
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	if filepath.Base(name) != name || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("invalid plugin name %q", name)
	}
	return nil
}

func pathWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
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

func parseGitHubPluginSource(source, pluginShort string) (githubPluginSource, bool, error) {
	parts := strings.Split(source, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return githubPluginSource{}, false, nil
	}
	if strings.ContainsAny(source, `\:`) || strings.HasPrefix(source, ".") {
		return githubPluginSource{}, false, nil
	}
	if err := validatePluginManifestName(pluginShort); err != nil {
		return githubPluginSource{}, false, fmt.Errorf("install: plugin name: %w", err)
	}
	return githubPluginSource{
		Owner:      parts[0],
		Repo:       parts[1],
		PluginName: pluginBinaryName(pluginShort),
	}, true, nil
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
		if extracted+h.Size > pluginInstallLimits.ArchiveExtractBytes {
			return "", fmt.Errorf("install: extract %s: plugin archive exceeds extracted limit of %d bytes", h.Name, pluginInstallLimits.ArchiveExtractBytes)
		}
		// filepath.Base intentionally flattens accepted archive paths, including
		// POSIX names that contain backslashes, so extraction never recreates
		// archive-controlled directories.
		dest := filepath.Join(tempDir, filepath.Base(h.Name))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("install: extract %s: %w", h.Name, err)
		}
		n, err := copyArchiveMemberBytes(out, tr, pluginInstallLimits.ArchiveMemberBytes)
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
		// filepath.Base intentionally flattens accepted archive paths, including
		// POSIX names that contain backslashes, so extraction never recreates
		// archive-controlled directories.
		dest := filepath.Join(tempDir, filepath.Base(f.Name))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = in.Close()
			return "", fmt.Errorf("install: extract %s: %w", f.Name, err)
		}
		n, copyErr := copyArchiveMemberBytes(out, in, pluginInstallLimits.ArchiveMemberBytes)
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

func copyArchiveMemberBytes(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := copyPluginBytes(dst, src, limit)
	if err != nil && limit > 0 && n > limit {
		return n, fmt.Errorf("plugin archive member exceeds limit of %d bytes", limit)
	}
	return n, err
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
