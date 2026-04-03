package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/output"
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

// runHTTP reads global flags, executes the HTTP request, normalizes the
// response, formats it, and handles exit codes.
func (c *CLI) runHTTP(cmd *cobra.Command, method string, args []string) error {
	rawURL := args[0]

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	httpResp, err := request.Do(context.Background(), method, rawURL, nil, opts)
	if err != nil {
		return fmt.Errorf("network: %w", err)
	}

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		return err
	}

	if err := c.formatResponse(cmd, resp); err != nil {
		return err
	}

	ignoreStatus, _ := cmd.Flags().GetBool("rsh-ignore-status-code")
	if !ignoreStatus {
		if code := output.StatusToExitCode(resp.Status); code != 0 {
			return &ExitCodeError{Code: code}
		}
	}
	return nil
}

// formatResponse selects and applies the right formatter for this invocation.
func (c *CLI) formatResponse(cmd *cobra.Command, resp *output.Response) error {
	fmtName, _ := cmd.Flags().GetString("rsh-output-format")
	fmts := output.DefaultFormatters()

	formatter, ok := output.Select(fmts, fmtName, output.IsTerminal(c.Stdout))
	if !ok {
		return fmt.Errorf("unknown output format %q; available: readable, json, raw", fmtName)
	}

	return formatter.Format(c.Stdout, resp, output.ColorEnabled(c.Stdout))
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
		Headers:              headers,
		Query:                query,
		Server:               server,
		Insecure:             insecure,
		Timeout:              timeout,
		AcceptHeader:         c.content.AcceptHeader(),
		AcceptEncodingHeader: c.content.AcceptEncodingHeader(),
	}, nil
}
