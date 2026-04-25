package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ExternalTool delegates authentication to an external program. The program
// receives the outbound request as JSON on stdin and returns header updates
// (and optionally a new URI) as JSON on stdout.
//
// Config params:
//
//	commandline  (required) shell command to run; executed via $SHELL -c
//	omitbody     (optional) "true" skips sending the request body to the tool
//
// Wire format (stdin → tool):
//
//	{"method":"GET","uri":"https://...","headers":{...},"body":"..."}
//
// Wire format (tool → stdout):
//
//	{"headers":{"X-Sig":"..."}}          — merge headers only
//	{"uri":"https://...","headers":{...}} — also rewrite the URI
//
// An empty or absent stdout response is a no-op (tool declined to mutate).
// Compatible with the v1 external-tool auth wire format.
type ExternalTool struct {
	Stderr  io.Writer
	Timeout time.Duration
}

func (a *ExternalTool) Parameters() []Param {
	return []Param{
		{Name: "commandline", Description: "Shell command to run for auth (executed via $SHELL -c)", Required: true},
		{Name: "omitbody", Description: "Set to \"true\" to skip sending the request body to the tool"},
	}
}

// externalToolRequest is the JSON structure sent to the tool on stdin.
type externalToolRequest struct {
	Method  string      `json:"method"`
	URI     string      `json:"uri"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

// externalToolResponse is the JSON structure read from the tool's stdout.
type externalToolResponse struct {
	URI     string      `json:"uri,omitempty"`
	Headers http.Header `json:"headers,omitempty"`
}

func (a *ExternalTool) OnRequest(req *http.Request, params map[string]string) error {
	return a.run(req.Context(), req, params)
}

func (a *ExternalTool) run(ctx context.Context, req *http.Request, params map[string]string) error {
	commandLine := params["commandline"]
	if commandLine == "" {
		return fmt.Errorf("external-tool auth: missing required param \"commandline\"")
	}
	omitBody := strings.EqualFold(params["omitbody"], "true")

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Read and restore the request body if we need to forward it.
	bodyStr := ""
	if req.Body != nil && !omitBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("external-tool auth: reading request body: %w", err)
		}
		bodyStr = string(bodyBytes)
		req.Body = io.NopCloser(strings.NewReader(bodyStr))
	}

	payload, err := json.Marshal(externalToolRequest{
		Method:  req.Method,
		URI:     req.URL.String(),
		Headers: req.Header,
		Body:    bodyStr,
	})
	if err != nil {
		return fmt.Errorf("external-tool auth: marshalling request: %w", err)
	}

	timeout := a.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if _, ok := ctx.Deadline(); !ok && timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, shell, "-c", commandLine)
	// Assign stdin directly so exec's internals copy the bytes after Start.
	// Using StdinPipe + manual write before Start would deadlock for payloads
	// larger than the OS pipe buffer (~64 KB).
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stderr = a.Stderr

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("external-tool auth: tool timed out or was canceled: %w", ctx.Err())
		}
		return fmt.Errorf("external-tool auth: tool exited with error: %w", err)
	}
	if len(out) == 0 {
		return nil
	}

	var updates externalToolResponse
	if err := json.Unmarshal(out, &updates); err != nil {
		return fmt.Errorf("external-tool auth: parsing tool response: %w", err)
	}

	if updates.URI != "" {
		parsed, err := req.URL.Parse(updates.URI)
		if err != nil {
			return fmt.Errorf("external-tool auth: parsing updated URI: %w", err)
		}
		req.URL = parsed
		req.Host = parsed.Host
	}

	// Use Del+Add so multi-value headers are fully replaced, not overwritten
	// one value at a time (which would only keep the last value).
	for key, vals := range updates.Headers {
		req.Header.Del(key)
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}
	return nil
}

func (a *ExternalTool) Authenticate(ctx context.Context, req *http.Request, ac AuthContext) error {
	stderr := a.Stderr
	if stderr == nil {
		stderr = ac.Stderr
	}
	if req.Context() != nil {
		ctx = req.Context()
	}
	return (&ExternalTool{Stderr: stderr, Timeout: a.Timeout}).run(ctx, req, ac.Params)
}
