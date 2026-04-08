package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/plugin"
	"github.com/danielgtaylor/restish/v2/internal/request"
)

// pluginsForHook returns all discovered plugins that declare the given hook.
func (c *CLI) pluginsForHook(hook string) []plugin.Plugin {
	var result []plugin.Plugin
	for _, p := range c.plugins {
		for _, h := range p.Manifest.Hooks {
			if h == hook {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func (c *CLI) pluginForHook(name, hook string) (plugin.Plugin, bool) {
	for _, p := range c.pluginsForHook(hook) {
		if p.Manifest.Name == name {
			return p, true
		}
	}
	return plugin.Plugin{}, false
}

func (c *CLI) resolveTLSSigner(opts request.Options) (request.Options, error) {
	if opts.TLSSignerName == "" || opts.TLSSignerPath != "" {
		return opts, nil
	}
	if p, ok := c.pluginForHook(opts.TLSSignerName, "tls-signer"); ok {
		opts.TLSSignerPath = p.Path
		return opts, nil
	}
	path, err := exec.LookPath(opts.TLSSignerName)
	if err == nil {
		opts.TLSSignerPath = path
		return opts, nil
	}
	if !strings.HasPrefix(opts.TLSSignerName, "restish-") {
		path, err = exec.LookPath("restish-" + opts.TLSSignerName)
		if err == nil {
			opts.TLSSignerPath = path
			return opts, nil
		}
	}
	return opts, fmt.Errorf("tls signer plugin %q not found", opts.TLSSignerName)
}

// runAuthHookPlugins invokes all "auth" hook plugins for the given API request.
// The returned headers from each plugin are merged into req. rawParams are the
// profile auth params (without internal keys) and are forwarded to the plugin;
// params whose key appears in secretKeys are omitted unless the plugin manifest
// declares NeedsAuthSecrets.
// Plugins that declare auth_api_names in their manifest are only called when
// apiName appears in that list.
func (c *CLI) runAuthHookPlugins(apiName, profileName string, rawParams map[string]string, secretKeys map[string]bool, req *http.Request) error {
	for _, p := range c.pluginsForHook("auth") {
		if len(p.Manifest.AuthAPINames) > 0 {
			matched := false
			for _, name := range p.Manifest.AuthAPINames {
				if name == apiName {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		params := rawParams
		if !p.Manifest.NeedsAuthSecrets && len(secretKeys) > 0 {
			// Build a copy with secret params removed.
			redacted := make(map[string]string, len(rawParams))
			for k, v := range rawParams {
				if !secretKeys[k] {
					redacted[k] = v
				}
			}
			params = redacted
		}
		headers := headerMap(req.Header)
		in := map[string]any{
			"type":    "auth",
			"api":     apiName,
			"profile": profileName,
			"params":  params,
			"request": map[string]any{
				"method":  req.Method,
				"uri":     req.URL.String(),
				"headers": headers,
			},
		}
		var out map[string]any
		if err := plugin.CallHook(p.Path, in, &out); err != nil {
			return fmt.Errorf("auth plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out)
	}
	return nil
}

// runRequestMiddlewarePlugins invokes all "request-middleware" hook plugins.
// The returned headers/method/uri from each plugin are applied to req.
func (c *CLI) runRequestMiddlewarePlugins(req *http.Request) error {
	for _, p := range c.pluginsForHook("request-middleware") {
		in := map[string]any{
			"type": "request-middleware",
			"request": map[string]any{
				"method":  req.Method,
				"uri":     req.URL.String(),
				"headers": headerMap(req.Header),
			},
		}
		var out map[string]any
		if err := plugin.CallHook(p.Path, in, &out); err != nil {
			return fmt.Errorf("request-middleware plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out)
	}
	return nil
}

// HookFollowRequest is returned by response-middleware plugins that want to
// chain a new request.
type HookFollowRequest struct {
	Method string
	URI    string
}

// runResponseMiddlewarePlugins invokes all "response-middleware" hook plugins.
// Returns (drop, follow, err). drop=true means suppress all output. follow is
// non-nil when the plugin wants Restish to issue a new request.
func (c *CLI) runResponseMiddlewarePlugins(req *http.Request, resp *output.Response) (drop bool, follow *HookFollowRequest, err error) {
	for _, p := range c.pluginsForHook("response-middleware") {
		in := map[string]any{
			"type": "response-middleware",
			"request": map[string]any{
				"method":  req.Method,
				"uri":     req.URL.String(),
				"headers": headerMap(req.Header),
			},
			"response": map[string]any{
				"status":  resp.Status,
				"headers": resp.Headers,
				"body":    resp.Body,
			},
		}
		var out map[string]any
		if err := plugin.CallHook(p.Path, in, &out); err != nil {
			return false, nil, fmt.Errorf("response-middleware plugin %s: %w", p.Manifest.Name, err)
		}

		if d, ok := out["drop"].(bool); ok && d {
			return true, nil, nil
		}

		if f, ok := out["follow"].(map[string]any); ok {
			method, _ := f["method"].(string)
			uri, _ := f["uri"].(string)
			if method == "" {
				method = "GET"
			}
			// Warn about keys that are not yet honoured so plugin authors know
			// their follow payload was partially ignored.
			for _, unsupported := range []string{"body", "headers", "query"} {
				if _, has := f[unsupported]; has {
					fmt.Fprintf(c.Stderr, "warning: response-middleware 'follow.%s' is not supported and was ignored\n", unsupported)
				}
			}
			return false, &HookFollowRequest{Method: method, URI: uri}, nil
		}

		if r, ok := out["response"].(map[string]any); ok {
			if body, hasBody := r["body"]; hasBody {
				resp.Body = body
			}
			if hdrs, ok := r["headers"].(map[string]any); ok {
				if resp.Headers == nil {
					resp.Headers = make(map[string]string)
				}
				for k, v := range hdrs {
					switch vt := v.(type) {
					case string:
						resp.Headers[k] = vt
					case []any:
						if len(vt) > 0 {
							if sv, ok := vt[0].(string); ok {
								resp.Headers[k] = sv
							}
						}
					}
				}
			}
		}
	}
	return false, nil, nil
}

// applyRequestUpdate merges the "request" section of a plugin reply into req.
// Only the "headers" field is currently applied; method/uri changes are ignored
// since the request has already been prepared for sending.
func applyRequestUpdate(req *http.Request, out map[string]any) {
	reqOut, ok := out["request"].(map[string]any)
	if !ok {
		return
	}
	hdrs, ok := reqOut["headers"].(map[string]any)
	if !ok {
		return
	}
	for k, v := range hdrs {
		switch vt := v.(type) {
		case []any:
			req.Header.Del(k)
			for _, s := range vt {
				if sv, ok := s.(string); ok {
					req.Header.Add(k, sv)
				}
			}
		case string:
			req.Header.Set(k, vt)
		}
	}
}

// headerMap converts an http.Header (map[string][]string) to map[string][]string
// as a plain Go map suitable for CBOR serialization.
func headerMap(h http.Header) map[string][]string {
	m := make(map[string][]string, len(h))
	for k, vs := range h {
		m[k] = vs
	}
	return m
}
