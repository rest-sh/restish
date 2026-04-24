package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

const maxThemeBytes = 1 << 20

var githubThemeShorthand = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var githubThemeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (c *CLI) addThemeCommand(root *cobra.Command) {
	themeCmd := &cobra.Command{
		Use:     "theme",
		Short:   "Manage readable output highlighting theme",
		GroupID: rootGroupConfig,
	}
	themeCmd.AddCommand(&cobra.Command{
		Use:   "set <url-or-user/repo> [name]",
		Short: "Fetch a theme JSON file and save it in config",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  c.runThemeSet,
	})
	root.AddCommand(themeCmd)
}

func (c *CLI) runThemeSet(cmd *cobra.Command, args []string) error {
	source, err := resolveThemeSource(args)
	if err != nil {
		return err
	}
	entries, err := c.fetchTheme(cmd, source)
	if err != nil {
		return err
	}

	if c.cfg == nil {
		c.cfg = &config.Config{}
	}
	c.cfg.Theme = map[string]string(entries)
	if err := output.SetTheme(entries); err != nil {
		return err
	}

	cfgPath := c.configFilePath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := config.Save(cfgPath, c.cfg); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("theme set: stat config: %w", err)
	} else if err := config.SaveConfigValue(cfgPath, []string{"theme"}, map[string]string(entries)); err != nil {
		return err
	}

	fmt.Fprintf(c.Stdout, "Set theme from %s\n", source)
	return nil
}

func (c *CLI) fetchTheme(cmd *cobra.Command, source string) (output.ThemeEntries, error) {
	u, err := url.Parse(source)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("theme set: expected http(s) URL or GitHub user/repo shorthand")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("theme set: unsupported URL scheme %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(requestContext(cmd), http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("theme set: request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Transport: c.baseHTTPTransport()}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("theme set: fetch %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("theme set: fetch %s: HTTP %d", source, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxThemeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("theme set: read response: %w", err)
	}
	if len(data) > maxThemeBytes {
		return nil, fmt.Errorf("theme set: theme is larger than %d bytes", maxThemeBytes)
	}

	entries, err := output.ParseThemeJSON(data)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func resolveThemeSource(args []string) (string, error) {
	source := args[0]
	if githubThemeShorthand.MatchString(source) {
		name := "theme"
		if len(args) == 2 {
			name = args[1]
		}
		if !githubThemeName.MatchString(name) {
			return "", fmt.Errorf("theme set: invalid GitHub theme name %q", name)
		}
		return "https://raw.githubusercontent.com/" + source + "/HEAD/" + name + ".json", nil
	}
	if len(args) == 2 {
		return "", fmt.Errorf("theme set: theme name is only supported with GitHub user/repo shorthand")
	}
	return source, nil
}
