package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const DefaultMaxResultBytes = 16 * 1024

func Run(stdin io.Reader, stdout io.Writer, fetchSpec SpecFetcher, exec HTTPExecutor, args []string) error {
	cfg, err := ParseArgs(args)
	if err != nil {
		return err
	}
	tools, err := LoadTools(fetchSpec, cfg.APINames, cfg.Options)
	if err != nil {
		return err
	}
	server := &Server{
		Tools:          tools,
		ToolIndex:      indexTools(tools),
		Exec:           exec,
		MaxResultBytes: cfg.Options.MaxResultBytes,
		RequestTimeout: cfg.Options.RequestTimeout,
	}
	if server.MaxResultBytes <= 0 {
		server.MaxResultBytes = DefaultMaxResultBytes
	}
	return server.ServeStdio(stdin, stdout)
}

type ServeConfig struct {
	APINames []string
	Options  Options
}

func ParseArgs(args []string) (*ServeConfig, error) {
	if len(args) == 0 {
		return nil, errors.New("mcp requires a subcommand; use: mcp serve <api...>")
	}
	if args[0] != "serve" {
		return nil, fmt.Errorf("unknown mcp command %q; use: mcp serve <api...>", args[0])
	}
	args = args[1:]
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var operations string
	var maxResultBytes int
	var requestTimeout int
	var readOnly bool
	var allowWriteTools bool
	fs.StringVar(&operations, "operations", "", "Comma-separated operationId allowlist")
	fs.IntVar(&maxResultBytes, "max-result-bytes", DefaultMaxResultBytes, "Maximum tool result payload size")
	fs.IntVar(&requestTimeout, "request-timeout", 60, "Per-tool HTTP request timeout in seconds (0 disables)")
	fs.BoolVar(&readOnly, "read-only", false, "Expose only GET/HEAD operations")
	fs.BoolVar(&allowWriteTools, "allow-write-tools", false, "Expose POST, PUT, PATCH, and DELETE operations as MCP tools")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	apiNames := fs.Args()
	if len(apiNames) == 0 {
		return nil, errors.New("mcp serve requires at least one API name")
	}
	ops := map[string]bool{}
	for _, item := range strings.Split(operations, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			ops[item] = true
		}
	}
	return &ServeConfig{
		APINames: apiNames,
		Options: Options{
			Operations:      ops,
			ReadOnly:        readOnly,
			AllowWriteTools: allowWriteTools,
			MaxResultBytes:  maxResultBytes,
			RequestTimeout:  requestTimeout,
		},
	}, nil
}
