package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/request"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func (c *CLI) pluginForHook(name, hook string) (plugin.Plugin, bool) {
	for _, p := range c.pluginsByHook[hook] {
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

func indexAuthPluginsByAPI(plugins []plugin.Plugin) ([]plugin.Plugin, map[string][]plugin.Plugin) {
	byAPI := map[string][]plugin.Plugin{}
	var global []plugin.Plugin
	for _, p := range plugins {
		if len(p.Manifest.AuthAPINames) == 0 {
			global = append(global, p)
			continue
		}
		for _, name := range p.Manifest.AuthAPINames {
			byAPI[name] = append(byAPI[name], p)
		}
	}
	return global, byAPI
}

func pluginDeclaresHook(manifest plugin.Manifest, hook string) bool {
	for _, declared := range manifest.Hooks {
		if declared == hook {
			return true
		}
	}
	return false
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
	if c.hooks.AuthHookFunc != nil {
		return c.hooks.AuthHookFunc(apiName, profileName, rawParams, secretKeys, req)
	}
	plugins := append([]plugin.Plugin(nil), c.globalAuthPlugins...)
	plugins = append(plugins, c.authPluginsByAPI[apiName]...)
	for _, p := range plugins {
		if trace := requestTraceFromContext(req.Context()); trace != nil {
			trace.AddInfo("Plugin", pluginInvocationTrace("auth", p))
			trace.Step("auth-plugin")
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
		if err := plugin.CallHookWithTimeoutContext(req.Context(), p.Path, plugin.HookTimeout(p.Manifest, "auth"), in, &out); err != nil {
			return fmt.Errorf("auth plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out.Request)
	}
	return nil
}

// runRequestMiddlewarePlugins invokes all "request-middleware" hook plugins.
// The returned headers from each plugin are applied to req.
func (c *CLI) runRequestMiddlewarePlugins(req *http.Request) error {
	for _, p := range c.pluginsByHook["request-middleware"] {
		if trace := requestTraceFromContext(req.Context()); trace != nil {
			trace.AddInfo("Plugin", pluginInvocationTrace("request", p))
			trace.Step("request-plugin")
		}
		in := pluginwire.RequestMiddlewareInput{
			Type:    "request-middleware",
			Request: hookRequestForPlugin(req, p),
		}
		var out pluginwire.RequestMiddlewareOutput
		if err := plugin.CallHookWithTimeoutContext(req.Context(), p.Path, plugin.HookTimeout(p.Manifest, "request-middleware"), in, &out); err != nil {
			return fmt.Errorf("request-middleware plugin %s: %w", p.Manifest.Name, err)
		}
		applyRequestUpdate(req, out.Request)
	}
	return nil
}

// HookFollowRequest is returned by response-middleware plugins that want to
// chain a new request.
type HookFollowRequest struct {
	Method      string
	URI         string
	Headers     map[string]string
	Body        any
	ContentType string
}

// runResponseMiddlewarePlugins invokes all "response-middleware" hook plugins.
// Returns (drop, follow, err). drop=true means suppress all output. follow is
// non-nil when the plugin wants Restish to issue a new request.
func (c *CLI) runResponseMiddlewarePlugins(req *http.Request, resp *output.Response) (drop bool, follow *HookFollowRequest, err error) {
	for _, p := range c.pluginsByHook["response-middleware"] {
		if trace := requestTraceFromContext(req.Context()); trace != nil {
			trace.AddInfo("Plugin", pluginInvocationTrace("response", p))
			trace.Step("response-plugin")
		}
		responseHeaders := cloneHeaderMap(resp.Headers)
		if !p.Manifest.NeedsAuthSecrets {
			redactCredentialHeaders(nil, responseHeaders)
		}
		in := pluginwire.ResponseMiddlewareInput{
			Type:    "response-middleware",
			Request: hookRequestForPlugin(req, p),
			Response: pluginwire.HookResponse{
				Status:  resp.Status,
				Headers: responseHeaders,
				Body:    resp.Body,
			},
		}
		var out pluginwire.ResponseMiddlewareOutput
		if err := plugin.CallHookWithTimeoutContext(req.Context(), p.Path, plugin.HookTimeout(p.Manifest, "response-middleware"), in, &out); err != nil {
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
			return false, &HookFollowRequest{
				Method:      method,
				URI:         out.Follow.URI,
				Headers:     out.Follow.Headers,
				Body:        out.Follow.Body,
				ContentType: out.Follow.ContentType,
			}, nil
		}

		if out.Response != nil {
			if out.Response.Body != nil {
				resp.Body = out.Response.Body
			}
			if out.Response.Headers != nil {
				if resp.Headers == nil {
					resp.Headers = make(map[string][]string)
				}
				for k, v := range out.Response.Headers {
					switch vt := v.(type) {
					case string:
						resp.Headers[k] = []string{vt}
					case []any:
						var values []string
						for _, item := range vt {
							if sv, ok := item.(string); ok {
								values = append(values, sv)
							}
						}
						if len(values) > 0 {
							resp.Headers[k] = values
						}
					}
				}
			}
		}
	}
	return false, nil, nil
}

func pluginInvocationTrace(kind string, p plugin.Plugin) string {
	name := p.Manifest.Name
	if name == "" {
		name = filepath.Base(p.Path)
	}
	return kind + " " + name
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
		case nil:
			req.Header.Del(k)
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
	headers := cloneHeaderMap(req.Header)
	uri := req.URL.String()
	if !p.Manifest.NeedsAuthSecrets {
		redactCredentialHeaders(req, headers)
		uri = request.RedactedRequestURL(req)
	}
	hookReq := pluginwire.HookRequest{
		Method:  req.Method,
		URI:     uri,
		Headers: headers,
	}
	if body, ok := hookRequestBody(req); ok {
		sum := sha256.Sum256(body)
		hookReq.BodySHA256 = hex.EncodeToString(sum[:])
		if p.Manifest.NeedsAuthSecrets || manifestRequiresFeature(p.Manifest, pluginwire.FeatureRequestFinalBody) {
			hookReq.Body = body
		}
	}
	return hookReq
}

func hookRequestBody(req *http.Request) ([]byte, bool) {
	if req == nil || req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, false
	}
	return data, true
}

func manifestRequiresFeature(m plugin.Manifest, feature string) bool {
	for _, required := range m.RequiredFeatures {
		if required == feature {
			return true
		}
	}
	return false
}

// redactCredentialHeaders replaces values for known credential-bearing headers
// with "<redacted>" so plugins receive the request shape without the secrets.
func redactCredentialHeaders(req *http.Request, headers map[string][]string) {
	for name := range headers {
		if request.IsCredentialHeader(name) || request.IsMarkedCredentialHeader(req, name) {
			headers[name] = []string{"<redacted>"}
		}
	}
}

// cloneHeaderMap returns a deep copy of h suitable for CBOR serialization
// without aliasing the source slices.
func cloneHeaderMap(h map[string][]string) map[string][]string {
	m := make(map[string][]string, len(h))
	for k, v := range h {
		m[k] = append([]string(nil), v...)
	}
	return m
}
