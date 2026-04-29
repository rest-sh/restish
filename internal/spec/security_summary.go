package spec

import (
	"sort"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// SecuritySchemeSummary describes one declared security scheme in a
// loader-neutral shape suitable for CLI setup and diagnostics.
type SecuritySchemeSummary struct {
	ID            string
	Kind          string
	Detail        string
	Supported     bool
	GlobalDefault bool
}

// SecuritySchemeSummaries returns the declared OpenAPI security schemes in
// document order with compact display metadata.
func (s *APISpec) SecuritySchemeSummaries() ([]SecuritySchemeSummary, error) {
	if s == nil {
		return nil, nil
	}
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Components == nil ||
		model.Model.Components.SecuritySchemes == nil {
		return nil, err
	}

	global := map[string]bool{}
	for _, req := range model.Model.Security {
		if req == nil || req.Requirements == nil {
			continue
		}
		for id := range req.Requirements.FromOldest() {
			global[id] = true
		}
	}

	var out []SecuritySchemeSummary
	for id, scheme := range model.Model.Components.SecuritySchemes.FromOldest() {
		out = append(out, SecuritySchemeSummary{
			ID:            id,
			Kind:          credentialRequirementKind(scheme),
			Detail:        securitySchemeDetail(scheme),
			Supported:     SchemeToXCLIAuth(scheme, nil) != nil,
			GlobalDefault: global[id],
		})
	}
	return out, nil
}

func securitySchemeDetail(scheme *v3high.SecurityScheme) string {
	if scheme == nil {
		return "unknown"
	}
	switch scheme.Type {
	case "apiKey":
		detail := strings.TrimSpace(strings.Join([]string{"apiKey", scheme.In, scheme.Name}, " "))
		return detail
	case "http":
		if scheme.Scheme == "" {
			return "http"
		}
		return "http " + scheme.Scheme
	case "oauth2":
		return "oauth2 " + oauthFlowSummary(scheme)
	case "openIdConnect":
		return "openIdConnect"
	case "mutualTLS":
		return "mutualTLS"
	default:
		if scheme.Type == "" {
			return "unknown"
		}
		return scheme.Type
	}
}

func oauthFlowSummary(scheme *v3high.SecurityScheme) string {
	if scheme == nil || scheme.Flows == nil {
		return "unknown"
	}
	var flows []string
	if scheme.Flows.AuthorizationCode != nil {
		flows = append(flows, "authorizationCode")
	}
	if scheme.Flows.ClientCredentials != nil {
		flows = append(flows, "clientCredentials")
	}
	if scheme.Flows.Device != nil {
		flows = append(flows, "deviceCode")
	}
	if scheme.Flows.Implicit != nil {
		flows = append(flows, "implicit")
	}
	if len(flows) == 0 {
		return "unknown"
	}
	sort.Strings(flows)
	return strings.Join(flows, "/")
}
