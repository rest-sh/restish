package spec

import (
	"fmt"
	"sort"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v3"
)

// XCLIConfig is the x-cli-config extension at the OpenAPI document root.
// It drives `restish api configure` pre-population of the config file.
type XCLIConfig struct {
	// Profiles maps profile names to their pre-populated settings.
	Profiles map[string]*XCLIProfile `json:"profiles,omitempty" yaml:"profiles,omitempty"`

	// Legacy v1 shape. These fields are normalized into Profiles["default"]
	// during API configuration so specs already in the wild keep working.
	Security string                   `json:"security,omitempty" yaml:"security,omitempty"`
	Headers  map[string]string        `json:"headers,omitempty" yaml:"headers,omitempty"`
	Prompt   map[string]XCLIPromptVar `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	Params   map[string]string        `json:"params,omitempty" yaml:"params,omitempty"`
}

// XCLIProfile holds pre-populated configuration for a single API profile.
type XCLIProfile struct {
	Headers []string  `json:"headers,omitempty" yaml:"headers,omitempty"`
	Query   []string  `json:"query,omitempty" yaml:"query,omitempty"`
	Auth    *XCLIAuth `json:"auth,omitempty" yaml:"auth,omitempty"`

	// Security, when non-empty, is the name of a security scheme in the spec's
	// components/securitySchemes. It is resolved to an Auth type and default
	// Params when applying the config; an explicit Auth field takes precedence.
	Security string `json:"security,omitempty" yaml:"security,omitempty"`

	// Params holds key→value pairs used for {var} template expansion in Headers.
	// Empty-string values indicate a placeholder that the operator should fill in.
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty"`

	// Prompt describes configuration-time questions. Prompted values are written
	// into Params before Resolve runs; prompts are never evaluated at request time.
	Prompt map[string]XCLIPromptVar `json:"prompt,omitempty" yaml:"prompt,omitempty"`

	// PromptValues holds configure-time prompt answers, including excluded
	// answers that are only available for template expansion.
	PromptValues map[string]string `json:"-" yaml:"-"`
	// PromptedParams marks Params entries that came directly from prompt input.
	// Prompt answers are literal values and are not expanded a second time.
	PromptedParams map[string]bool `json:"-" yaml:"-"`
}

// XCLIPromptVar describes a single configuration-time prompt from the legacy
// v1 x-cli-config.prompt extension.
type XCLIPromptVar struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Example     string `json:"example,omitempty" yaml:"example,omitempty"`
	Default     any    `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []any  `json:"enum,omitempty" yaml:"enum,omitempty"`

	// Exclude keeps the prompted value out of auth params; it can still be used
	// for {var} template expansion in Params, Headers, or explicit Auth params.
	Exclude bool `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// XCLIAuth holds pre-populated authentication configuration.
type XCLIAuth struct {
	// Type is the restish auth type (e.g. "bearer", "http-basic").
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	// Params holds auth parameters; secret values should be empty strings.
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
}

// ReadXCLIConfig extracts the x-cli-config extension from s.Raw.
// Returns nil, nil when the extension is absent or the spec cannot be parsed.
// Accepts both JSON and YAML specs (YAML is a superset of JSON).
func ReadXCLIConfig(s *APISpec) (*XCLIConfig, error) {
	var top struct {
		XCLIConfig *XCLIConfig `json:"x-cli-config" yaml:"x-cli-config"`
	}
	if err := yaml.Unmarshal(s.Raw, &top); err != nil || top.XCLIConfig == nil {
		return nil, nil
	}
	return top.XCLIConfig, nil
}

// FallbackXCLIConfig derives an XCLIConfig from the document's security
// schemes when the spec does not define x-cli-config. The first scheme named
// in the document-level security requirements is preferred; otherwise the first
// scheme in components/securitySchemes is used.
//
// Returns nil when no supported auth scheme can be derived.
func FallbackXCLIConfig(s *APISpec) *XCLIConfig {
	if s == nil {
		return nil
	}
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Components == nil ||
		model.Model.Components.SecuritySchemes == nil {
		return nil
	}

	schemes := model.Model.Components.SecuritySchemes

	// Collect names from document-level security requirements (in order).
	var preferredNames []string
	for _, req := range model.Model.Security {
		if req == nil || req.Requirements == nil {
			continue
		}
		for k := range req.Requirements.FromOldest() {
			preferredNames = append(preferredNames, k)
		}
	}

	// Try preferred scheme names first, then fall back to first in map.
	var chosenScheme *v3high.SecurityScheme
	for _, name := range preferredNames {
		if s := schemes.GetOrZero(name); s != nil {
			chosenScheme = s
			break
		}
	}
	if chosenScheme == nil {
		for _, v := range schemes.FromOldest() {
			chosenScheme = v
			break
		}
	}
	if chosenScheme == nil {
		return nil
	}

	xcliAuth := SchemeToXCLIAuth(chosenScheme, nil)
	if xcliAuth == nil {
		return nil
	}

	return &XCLIConfig{
		Profiles: map[string]*XCLIProfile{
			"default": {Auth: xcliAuth},
		},
	}
}

// SchemeToXCLIAuth converts an OpenAPI security scheme to an XCLIAuth.
// params overrides are applied on top of the defaults derived from the scheme.
// Returns nil for unsupported/unrecognised scheme types.
func SchemeToXCLIAuth(scheme *v3high.SecurityScheme, params map[string]string) *XCLIAuth {
	p := map[string]string{}
	var authType string

	switch scheme.Type {
	case "http":
		switch scheme.Scheme {
		case "basic":
			authType = "http-basic"
			p["username"] = ""
			p["password"] = ""
		default:
			return nil
		}
	case "oauth2":
		if scheme.Flows == nil {
			return nil
		}
		if scheme.Flows.AuthorizationCode != nil {
			ac := scheme.Flows.AuthorizationCode
			authType = "oauth-authorization-code"
			p["client_id"] = ""
			p["authorize_url"] = ac.AuthorizationUrl
			p["token_url"] = ac.TokenUrl
		} else if scheme.Flows.ClientCredentials != nil {
			cc := scheme.Flows.ClientCredentials
			authType = "oauth-client-credentials"
			p["client_id"] = ""
			p["client_secret"] = ""
			p["token_url"] = cc.TokenUrl
		} else {
			return nil
		}
	default:
		// apiKey, openIdConnect, and any future types are not natively
		// supported; the caller can always configure auth manually.
		return nil
	}

	for k, v := range params {
		p[k] = v
	}

	return &XCLIAuth{Type: authType, Params: p}
}

// Normalize returns a copy of xcli using the v2 profile-shaped structure.
// Legacy top-level v1 x-cli-config fields are mapped to the default profile.
func (xcli *XCLIConfig) Normalize() *XCLIConfig {
	if xcli == nil {
		return nil
	}
	out := &XCLIConfig{
		Profiles: make(map[string]*XCLIProfile, len(xcli.Profiles)),
	}
	for name, profile := range xcli.Profiles {
		out.Profiles[name] = cloneXCLIProfile(profile)
	}
	if len(out.Profiles) == 0 && xcli.hasLegacyDefaultProfile() {
		out.Profiles["default"] = &XCLIProfile{
			Headers:  sortedLegacyHeaders(xcli.Headers),
			Security: xcli.Security,
			Params:   cloneStringMap(xcli.Params),
			Prompt:   clonePromptMap(xcli.Prompt),
		}
	}
	return out
}

func (xcli *XCLIConfig) hasLegacyDefaultProfile() bool {
	return xcli.Security != "" || len(xcli.Headers) > 0 || len(xcli.Prompt) > 0 || len(xcli.Params) > 0
}

func cloneXCLIProfile(src *XCLIProfile) *XCLIProfile {
	if src == nil {
		return &XCLIProfile{}
	}
	dst := &XCLIProfile{
		Headers:        append([]string(nil), src.Headers...),
		Query:          append([]string(nil), src.Query...),
		Security:       src.Security,
		Params:         cloneStringMap(src.Params),
		Prompt:         clonePromptMap(src.Prompt),
		PromptValues:   cloneStringMap(src.PromptValues),
		PromptedParams: cloneBoolMap(src.PromptedParams),
	}
	if src.Auth != nil {
		dst.Auth = &XCLIAuth{
			Type:   src.Auth.Type,
			Params: cloneStringMap(src.Auth.Params),
		}
	}
	return dst
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func clonePromptMap(src map[string]XCLIPromptVar) map[string]XCLIPromptVar {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]XCLIPromptVar, len(src))
	for k, v := range src {
		v.Enum = append([]any(nil), v.Enum...)
		dst[k] = v
	}
	return dst
}

func sortedLegacyHeaders(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s: %s", k, headers[k]))
	}
	return out
}

// ExpandParams replaces {var} placeholders in s with the corresponding values
// from params. Unrecognised placeholders are left as-is.
func ExpandParams(s string, params map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '{' {
			if end := strings.IndexByte(s[i+1:], '}'); end >= 0 {
				key := s[i+1 : i+1+end]
				if value, ok := params[key]; ok {
					out.WriteString(value)
					i += end + 2
					continue
				}
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// Resolve returns a copy of xcli with all Security scheme names resolved to
// XCLIAuth values and {var} templates in Headers expanded using Params.
// s is used for scheme lookups; it may be nil when no document is available
// (in which case unresolved Security fields are silently dropped).
// The original XCLIConfig is not modified.
func (xcli *XCLIConfig) Resolve(s *APISpec) *XCLIConfig {
	xcli = xcli.Normalize()
	if xcli == nil {
		return &XCLIConfig{}
	}
	// Build the security scheme map once (best-effort).
	schemeMap := map[string]*v3high.SecurityScheme{}
	if s != nil && s.Document != nil {
		if model, err := s.V3Model(); err == nil &&
			model != nil &&
			model.Model.Components != nil &&
			model.Model.Components.SecuritySchemes != nil {
			for k, v := range model.Model.Components.SecuritySchemes.FromOldest() {
				schemeMap[k] = v
			}
		}
	}

	resolved := &XCLIConfig{
		Profiles: make(map[string]*XCLIProfile, len(xcli.Profiles)),
	}

	for name, xp := range xcli.Profiles {
		expansionParams := cloneStringMap(xp.Params)
		if expansionParams == nil {
			expansionParams = map[string]string{}
		}
		for k, v := range xp.PromptValues {
			expansionParams[k] = v
		}
		resolvedParams := expandXCLIParams(xp.Params, expansionParams, xp.PromptedParams)

		rp := &XCLIProfile{
			Query: xp.Query,
		}

		// Resolve Security → Auth when no explicit Auth is set.
		auth := cloneXCLIAuth(xp.Auth)
		if auth != nil && len(auth.Params) > 0 {
			auth.Params = expandStringMap(auth.Params, expansionParams)
		}
		if auth == nil && xp.Security != "" {
			if scheme, ok := schemeMap[xp.Security]; ok {
				auth = SchemeToXCLIAuth(scheme, resolvedParams)
			}
		}
		if auth == nil && xp.Security != "" {
			auth = &XCLIAuth{Type: xp.Security, Params: cloneStringMap(resolvedParams)}
		}
		if auth != nil && len(resolvedParams) > 0 {
			params := cloneStringMap(resolvedParams)
			for k, v := range auth.Params {
				params[k] = v
			}
			auth.Params = params
		}
		rp.Auth = auth

		// Expand {var} templates in headers.
		rp.Headers = make([]string, len(xp.Headers))
		for i, h := range xp.Headers {
			rp.Headers[i] = ExpandParams(h, expansionParams)
		}

		resolved.Profiles[name] = rp
	}

	return resolved
}

func expandXCLIParams(params map[string]string, expansionParams map[string]string, prompted map[string]bool) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		if prompted[k] {
			out[k] = v
			continue
		}
		out[k] = ExpandParams(v, expansionParams)
	}
	return out
}

func expandStringMap(params map[string]string, expansionParams map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		out[k] = ExpandParams(v, expansionParams)
	}
	return out
}

func cloneXCLIAuth(src *XCLIAuth) *XCLIAuth {
	if src == nil {
		return nil
	}
	return &XCLIAuth{
		Type:   src.Type,
		Params: cloneStringMap(src.Params),
	}
}
