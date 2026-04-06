package spec

import (
	"strings"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v3"
)

// XCLIConfig is the x-cli-config extension at the OpenAPI document root.
// It drives `restish api configure` pre-population of the config file.
type XCLIConfig struct {
	// Profiles maps profile names to their pre-populated settings.
	Profiles map[string]*XCLIProfile `json:"profiles,omitempty" yaml:"profiles,omitempty"`
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
func FallbackXCLIConfig(doc libopenapi.Document) *XCLIConfig {
	model, err := doc.BuildV3Model()
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

// ExpandParams replaces {var} placeholders in s with the corresponding values
// from params. Unrecognised placeholders are left as-is.
func ExpandParams(s string, params map[string]string) string {
	for k, v := range params {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// Resolve returns a copy of xcli with all Security scheme names resolved to
// XCLIAuth values and {var} templates in Headers expanded using Params.
// s is used for scheme lookups; it may be nil when no document is available
// (in which case unresolved Security fields are silently dropped).
// The original XCLIConfig is not modified.
func (xcli *XCLIConfig) Resolve(s *APISpec) *XCLIConfig {
	// Build the security scheme map once (best-effort).
	schemeMap := map[string]*v3high.SecurityScheme{}
	if s != nil && s.Document != nil {
		if model, err := s.Document.BuildV3Model(); err == nil &&
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
		rp := &XCLIProfile{
			Query: xp.Query,
		}

		// Resolve Security → Auth when no explicit Auth is set.
		auth := xp.Auth
		if auth == nil && xp.Security != "" {
			if scheme, ok := schemeMap[xp.Security]; ok {
				auth = SchemeToXCLIAuth(scheme, xp.Params)
			}
		}
		rp.Auth = auth

		// Expand {var} templates in headers.
		rp.Headers = make([]string, len(xp.Headers))
		for i, h := range xp.Headers {
			rp.Headers[i] = ExpandParams(h, xp.Params)
		}

		resolved.Profiles[name] = rp
	}

	return resolved
}
