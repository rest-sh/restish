package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/request"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// pluginsForHook returns all discovered plugins that declare the given hook.
func (c *CLI) pluginsForHook(hook string) []plugin.Plugin {
	return c.pluginsByHook[hook]
}

func (c *CLI) pluginForHook(name, hook string) (plugin.Plugin, bool) {
	for _, p := range c.pluginsForHook(hook) {
		if p.Manifest.Name == name {
			return p, true
		}
	}
	return plugin.Plugin{}, false
}

func indexPluginsByHook(plugins []plugin.Plugin) map[string][]plugin.Plugin {
	if len(plugins) == 0 {
		return map[string][]plugin.Plugin{}
	}
	indexed := make(map[string][]plugin.Plugin)
	for _, p := range plugins {
		for _, hook := range p.Manifest.Hooks {
			indexed[hook] = append(indexed[hook], p)
		}
	}
	return indexed
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
			redacted := make(map[string]string, len(rawParams))
			for k, v := range rawParams {
				if !secretKeys[k] {
					redacted[k] = v
				}
			}
			params = redacted
		}
		in := pluginwire.AuthHookInput{
			Type:    "auth",
			API:     apiName,
			Profile: profileName,
			Params:  params,
			Request: hookRequestForPlugin(req, p),
		}
		var out pluginwire.AuthHookOutput
		if err := plugin.CallHookWithTimeout(p.Path, plugin.HookTimeout(p.Manifest, "auth"), in, &out); err != nil {
			return fmt.Errorf("auth plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out.Request)
	}
	return nil
}

// runRequestMiddlewarePlugins invokes all "request-middleware" hook plugins.
// The returned headers from each plugin are applied to req.
func (c *CLI) runRequestMiddlewarePlugins(req *http.Request) error {
	for _, p := range c.pluginsForHook("request-middleware") {
		in := pluginwire.RequestMiddlewareInput{
			Type:    "request-middleware",
			Request: hookRequestForPlugin(req, p),
		}
		var out pluginwire.RequestMiddlewareOutput
		if err := plugin.CallHookWithTimeout(p.Path, plugin.HookTimeout(p.Manifest, "request-middleware"), in, &out); err != nil {
			return fmt.Errorf("request-middleware plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out.Request)
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
		in := pluginwire.ResponseMiddlewareInput{
			Type:    "response-middleware",
			Request: hookRequestForPlugin(req, p),
			Response: pluginwire.HookResponse{
				Status:  resp.Status,
				Headers: resp.Headers,
				Body:    resp.Body,
			},
		}
		var out pluginwire.ResponseMiddlewareOutput
		if err := plugin.CallHookWithTimeout(p.Path, plugin.HookTimeout(p.Manifest, "response-middleware"), in, &out); err != nil {
			return false, nil, fmt.Errorf("response-middleware plugin %s: %w", p.Manifest.Name, err)
		}

		if out.Drop {
			return true, nil, nil
		}

		if out.Follow != nil {
			method := out.Follow.Method
			if method == "" {
				method = "GET"
			}
			return false, &HookFollowRequest{Method: method, URI: out.Follow.URI}, nil
		}

		if out.Response != nil {
			if out.Response.Body != nil {
				resp.Body = out.Response.Body
			}
			if out.Response.Headers != nil {
				if resp.Headers == nil {
					resp.Headers = make(map[string]string)
				}
				for k, v := range out.Response.Headers {
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

// applyRequestUpdate merges headers from a hook plugin reply into req.
// Only headers are applied; method and URI changes are not supported because
// the request has already been prepared for sending.
func applyRequestUpdate(req *http.Request, update *pluginwire.HookRequestHeaderUpdate) {
	if update == nil {
		return
	}
	for k, v := range update.Headers {
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

func hookRequestForPlugin(req *http.Request, p plugin.Plugin) pluginwire.HookRequest {
	headers := headerMap(req.Header)
	if !p.Manifest.NeedsAuthSecrets {
		redactHookRequestHeaders(headers)
	}
	return pluginwire.HookRequest{
		Method:  req.Method,
		URI:     req.URL.String(),
		Headers: headers,
	}
}

func redactHookRequestHeaders(headers map[string][]string) {
	for name := range headers {
		if isHookSecretHeader(name) {
			headers[name] = []string{"<redacted>"}
		}
	}
}

func isHookSecretHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Cookie", "Proxy-Authorization":
		return true
	default:
		return false
	}
}

// headerMap converts an http.Header (map[string][]string) to a plain Go map
// suitable for CBOR serialization.
func headerMap(h http.Header) map[string][]string {
	m := make(map[string][]string, len(h))
	for k, vs := range h {
		m[k] = append([]string(nil), vs...)
	}
	return m
}
