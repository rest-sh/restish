package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// addHTTPCommands registers the generic HTTP verb commands on root.
func (c *CLI) addHTTPCommands(root *cobra.Command) {
	verbs := []struct {
		name  string
		short string
	}{
		{"get", "Perform an HTTP GET request"},
		{"head", "Perform an HTTP HEAD request"},
		{"options", "Perform an HTTP OPTIONS request"},
		{"post", "Perform an HTTP POST request"},
		{"put", "Perform an HTTP PUT request"},
		{"patch", "Perform an HTTP PATCH request"},
		{"delete", "Perform an HTTP DELETE request"},
	}

	for _, v := range verbs {
		v := v
		method := strings.ToUpper(v.name)
		cmd := &cobra.Command{
			Use:     v.name + " <url>",
			Aliases: []string{method},
			Short:   v.short,
			Args:    cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.runHTTP(cmd, method, args)
			},
		}
		root.AddCommand(cmd)
	}
}

// runHTTP reads global flags, executes the HTTP request, and writes the
// response body to stdout. Response normalization and formatting will be
// added in Step 4.
func (c *CLI) runHTTP(cmd *cobra.Command, method string, args []string) error {
	rawURL := args[0]

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	resp, err := request.Do(context.Background(), method, rawURL, nil, opts)
	if err != nil {
		return fmt.Errorf("network: %w", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(c.Stdout, resp.Body)
	return err
}

// httpOptsFromFlags reads the global HTTP flags from cmd and builds an Options.
func (c *CLI) httpOptsFromFlags(cmd *cobra.Command) (request.Options, error) {
	headers, _ := cmd.Flags().GetStringArray("rsh-header")
	query, _ := cmd.Flags().GetStringArray("rsh-query")
	server, _ := cmd.Flags().GetString("rsh-server")
	insecure, _ := cmd.Flags().GetBool("rsh-insecure")
	timeoutStr, _ := cmd.Flags().GetString("rsh-timeout")

	var timeout time.Duration
	if timeoutStr != "" {
		var parseErr error
		timeout, parseErr = time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return request.Options{}, fmt.Errorf("invalid --rsh-timeout %q: %w", timeoutStr, parseErr)
		}
	}

	return request.Options{
		Headers:  headers,
		Query:    query,
		Server:   server,
		Insecure: insecure,
		Timeout:  timeout,
	}, nil
}
