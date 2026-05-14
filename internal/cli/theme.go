package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	officialthemes "github.com/rest-sh/restish/v2/themes"
	"github.com/spf13/cobra"
)

const maxThemeBytes = 256 << 10
const officialThemeSourcePrefix = "official:"
const legacyOfficialThemeRepo = "rest-sh/restish"

var githubThemeShorthand = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var githubThemeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (c *CLI) newThemeSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <theme|path-or-url-or-user/repo> [name]",
		Short: "Install a theme JSON or JSONC file and save it in config",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  c.runThemeSet,
	}
	cmd.Flags().Bool("yes", false, "Install without confirmation prompt")
	return cmd
}

func (c *CLI) newThemeListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List official theme names",
		Args:  cobra.NoArgs,
		RunE:  c.runThemeList,
	}
}

func (c *CLI) newThemeResetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "reset",
		Aliases: []string{"unset"},
		Short:   "Reset auto output highlighting to the built-in theme",
		Args:    cobra.NoArgs,
		RunE:    c.runThemeReset,
	}
}

func (c *CLI) runThemeList(cmd *cobra.Command, args []string) error {
	current := ""
	if c.cfg != nil {
		current = c.cfg.ThemeSource
	}
	for _, name := range officialthemes.Names() {
		source := officialThemeSource(name)
		marker := " "
		if current == source || current == legacyOfficialThemeSource(name) {
			marker = "*"
		}
		fmt.Fprintf(c.Stdout, "%s %s\n", marker, name)
	}
	return nil
}

func (c *CLI) runThemeSet(cmd *cobra.Command, args []string) error {
	source, err := resolveThemeSource(args)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Theme %s: %s\n", themeSourceLabel(source), themeSourceDisplay(source))

	yes, _ := cmd.Flags().GetBool("yes")
	if c.themeSourceNeedsConfirmation(source) && !yes {
		ok, err := c.Confirm(requestContext(cmd), "Install theme from this source? [Y/n] ")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("config theme set: confirmation required; rerun with --yes for automation")
		}
	}

	entries, err := c.fetchTheme(cmd, source)
	if err != nil {
		return err
	}

	if err := output.SetTheme(entries); err != nil {
		return err
	}

	if err := c.saveThemeConfig(map[string]string(entries), source); err != nil {
		return err
	}

	fmt.Fprintf(c.Stdout, "Set theme from %s\n", themeSourceDisplay(source))
	return nil
}

func (c *CLI) runThemeReset(cmd *cobra.Command, args []string) error {
	if err := output.SetTheme(nil); err != nil {
		return err
	}
	if err := c.resetThemeConfig(); err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, "Reset theme to built-in default")
	return nil
}

func (c *CLI) themeSourceNeedsConfirmation(source string) bool {
	if isOfficialThemeSource(source) {
		return false
	}
	if isLocalThemeSource(source) {
		return false
	}
	return c.cfg == nil || c.cfg.ThemeSource != source
}

func (c *CLI) fetchTheme(cmd *cobra.Command, source string) (output.ThemeEntries, error) {
	if name, ok := officialThemeNameFromSource(source); ok {
		return readOfficialTheme(name)
	}
	if isLocalThemeSource(source) {
		return readThemeFile(source)
	}

	entries, err := c.fetchThemeURL(cmd, source)
	if err == nil {
		return entries, nil
	}
	var statusErr themeHTTPStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		if fallback, ok := githubNamedThemeFallbackSource(source); ok {
			if entries, fallbackErr := c.fetchThemeURL(cmd, fallback); fallbackErr == nil {
				return entries, nil
			} else {
				return nil, fallbackErr
			}
		}
	}
	return nil, err
}

type themeHTTPStatusError struct {
	Source     string
	StatusCode int
}

func (e themeHTTPStatusError) Error() string {
	return fmt.Sprintf("config theme set: fetch %s: HTTP %d", e.Source, e.StatusCode)
}

func (c *CLI) fetchThemeURL(cmd *cobra.Command, source string) (output.ThemeEntries, error) {
	u, err := url.Parse(source)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("config theme set: expected http(s) URL or GitHub user/repo shorthand")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("config theme set: unsupported URL scheme %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(requestContext(cmd), http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("config theme set: request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Transport: c.baseHTTPTransport()}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("config theme set: fetch %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, themeHTTPStatusError{Source: source, StatusCode: resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxThemeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("config theme set: read response: %w", err)
	}
	if len(data) > maxThemeBytes {
		return nil, fmt.Errorf("config theme set: theme is larger than %d bytes", maxThemeBytes)
	}

	entries, err := output.ParseThemeJSON(data)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func githubNamedThemeFallbackSource(source string) (string, bool) {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" || u.Host != "raw.githubusercontent.com" {
		return "", false
	}
	const namedThemeDir = "/HEAD/themes/"
	if !strings.Contains(u.Path, namedThemeDir) {
		return "", false
	}
	u.Path = strings.Replace(u.Path, namedThemeDir, "/HEAD/", 1)
	return u.String(), true
}

func resolveThemeSource(args []string) (string, error) {
	source := args[0]
	if len(args) == 2 {
		if githubThemeShorthand.MatchString(source) {
			name := args[1]
			if !githubThemeName.MatchString(name) {
				return "", fmt.Errorf("config theme set: invalid GitHub theme name %q", name)
			}
			return "https://raw.githubusercontent.com/" + source + "/HEAD/themes/" + name + ".json", nil
		}
		return "", fmt.Errorf("config theme set: theme name is only supported with GitHub user/repo shorthand")
	}

	if path, ok, err := resolveLocalThemePath(source); ok || err != nil {
		return path, err
	}

	if githubThemeShorthand.MatchString(source) {
		return "https://raw.githubusercontent.com/" + source + "/HEAD/theme.json", nil
	}
	if isOfficialThemeName(source) {
		return officialThemeSource(source), nil
	}
	return source, nil
}

func officialThemeSource(name string) string {
	return officialThemeSourcePrefix + name
}

func legacyOfficialThemeSource(name string) string {
	return "https://raw.githubusercontent.com/" + legacyOfficialThemeRepo + "/HEAD/themes/" + name + ".json"
}

func isOfficialThemeName(name string) bool {
	for _, official := range officialthemes.Names() {
		if name == official {
			return true
		}
	}
	return false
}

func officialThemeNameFromSource(source string) (string, bool) {
	name, ok := strings.CutPrefix(source, officialThemeSourcePrefix)
	if !ok || !isOfficialThemeName(name) {
		return "", false
	}
	return name, true
}

func isOfficialThemeSource(source string) bool {
	_, ok := officialThemeNameFromSource(source)
	return ok
}

func readOfficialTheme(name string) (output.ThemeEntries, error) {
	data, err := officialthemes.Read(name)
	if err != nil {
		return nil, fmt.Errorf("config theme set: %w", err)
	}
	entries, err := output.ParseThemeJSON(data)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func readThemeFile(path string) (output.ThemeEntries, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config theme set: read %s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxThemeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("config theme set: read %s: %w", path, err)
	}
	if len(data) > maxThemeBytes {
		return nil, fmt.Errorf("config theme set: theme is larger than %d bytes", maxThemeBytes)
	}

	entries, err := output.ParseThemeJSON(data)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func resolveLocalThemePath(source string) (string, bool, error) {
	if hasURLScheme(source) {
		return "", false, nil
	}
	path := expandHomePath(source)
	if !looksLikeLocalThemePath(source) {
		if _, err := os.Stat(path); err != nil {
			return "", false, nil
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", true, fmt.Errorf("config theme set: resolve path %s: %w", source, err)
	}
	return abs, true, nil
}

func looksLikeLocalThemePath(source string) bool {
	if filepath.IsAbs(source) {
		return true
	}
	if strings.HasPrefix(source, "."+string(filepath.Separator)) ||
		strings.HasPrefix(source, ".."+string(filepath.Separator)) ||
		strings.HasPrefix(source, "~"+string(filepath.Separator)) {
		return true
	}
	ext := filepath.Ext(source)
	return strings.ContainsAny(source, `/\`) &&
		(strings.EqualFold(ext, ".json") || strings.EqualFold(ext, ".jsonc"))
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	prefix := "~" + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, prefix))
		}
	}
	return path
}

func isLocalThemeSource(source string) bool {
	if isOfficialThemeSource(source) {
		return false
	}
	if hasURLScheme(source) {
		return false
	}
	return true
}

func themeSourceLabel(source string) string {
	if isOfficialThemeSource(source) {
		return "name"
	}
	if isLocalThemeSource(source) {
		return "path"
	}
	return "URL"
}

func themeSourceDisplay(source string) string {
	if name, ok := officialThemeNameFromSource(source); ok {
		return name
	}
	return source
}

func hasURLScheme(source string) bool {
	u, err := url.Parse(source)
	return err == nil && u.Scheme != ""
}
