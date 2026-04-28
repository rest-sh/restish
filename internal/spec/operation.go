package spec

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// OperationXCLI holds x-cli-* extension values extracted from an operation.
type OperationXCLI struct {
	Ignore      bool
	Hidden      bool
	Name        string
	Description string
	Aliases     []string
}

// ParamXCLI holds x-cli-* extension values extracted from a parameter.
type ParamXCLI struct {
	Ignore bool
	Hidden bool
	// Name overrides the kebab-case parameter name used for the flag.
	Name string
	// Description overrides the OpenAPI parameter description.
	Description string
}

// Param is a single request parameter (path, query, header, or cookie).
type Param struct {
	Name       string
	In         string // "path", "query", "header", "cookie"
	Desc       string
	Schema     string
	Required   bool
	Type       string
	ItemType   string
	Default    string
	HasDefault bool
	Style      string
	Explode    *bool
	Enum       []string
	XCLI       ParamXCLI
}

// OperationBodyHelp is a compact request/response body example extracted from
// OpenAPI schemas for generated command help.
type OperationBodyHelp struct {
	MediaType string
	Schema    string
	Example   string
}

// OperationResponseHelp is a compact response shape for generated command help.
// Codes may contain several status codes when they share the same schema.
type OperationResponseHelp struct {
	Codes       []string
	MediaType   string
	Schema      string
	Example     string
	NoBody      bool
	Description string
}

// OperationHelp stores pre-rendered help snippets for generated commands.
type OperationHelp struct {
	Request   *OperationBodyHelp
	Responses []OperationResponseHelp
	Examples  []string
}

// Operation is a single HTTP operation extracted from a spec, expressed in
// format-neutral terms so command generators need not import libopenapi.
type Operation struct {
	// ID is the operationId, or a kebab-case fallback when absent.
	ID     string
	Method string
	// Path is the full path template after applying any server base prefix.
	Path        string
	Summary     string
	Description string
	Deprecated  bool
	Tags        []string
	Parameters  []Param
	// HasBody is true when the operation has a requestBody.
	HasBody bool
	// NoAuth is true when the operation explicitly declares security: [].
	NoAuth bool
	// RequestMediaType is the deterministic preferred content type from
	// requestBody.content, if the operation accepts a body.
	RequestMediaType string
	// RequestSchemaTypes maps dotted request-body property paths to simple
	// OpenAPI schema types for generated-command shorthand coercion.
	RequestSchemaTypes map[string]string
	XCLI               OperationXCLI
	Help               OperationHelp
}

// OperationSet is the extracted operation list plus API-level metadata needed
// to present the generated command group without reparsing the raw spec.
type OperationSet struct {
	Info       APIInfo
	Operations []Operation
}

// OperationSet returns all operations with top-level API metadata.
func (s *APISpec) OperationSet(baseURL, operationBase string) (OperationSet, error) {
	return s.OperationSetWithOptions(OperationOptions{BaseURL: baseURL, OperationBase: operationBase})
}

// OperationSetWithOptions returns all operations with config-sensitive
// OpenAPI server variable resolution applied.
func (s *APISpec) OperationSetWithOptions(opts OperationOptions) (OperationSet, error) {
	ops, err := s.OperationsWithOptions(opts)
	if err != nil {
		return OperationSet{}, err
	}
	info, err := s.Info()
	if err != nil {
		return OperationSet{}, err
	}
	return OperationSet{Info: info, Operations: ops}, nil
}

// Operations returns all HTTP operations extracted from the spec's V3 model,
// with paths prefixed by the base path derived from the spec's servers[] list
// and the provided baseURL. When operationBase is non-empty the spec servers
// are ignored and Operation.Path contains only the bare path template; the CLI
// resolves operationBase against baseURL at request time.
//
// Results are memoized per (baseURL, operationBase) pair.
func (s *APISpec) Operations(baseURL, operationBase string) ([]Operation, error) {
	return s.OperationsWithOptions(OperationOptions{BaseURL: baseURL, OperationBase: operationBase})
}

// OperationsWithOptions is the config-sensitive form of Operations. It uses
// OpenAPI server variable defaults plus explicit local values, never enum
// Cartesian-product expansion.
func (s *APISpec) OperationsWithOptions(opts OperationOptions) ([]Operation, error) {
	key := operationOptionsKey(opts)

	s.opsCacheMu.Lock()
	if s.opsCache != nil {
		if e, ok := s.opsCache[key]; ok {
			s.opsCacheMu.Unlock()
			return e.ops, e.err
		}
	}
	s.opsCacheMu.Unlock()

	ops, err := s.buildOperations(opts)

	s.opsCacheMu.Lock()
	if s.opsCache == nil {
		s.opsCache = make(map[opsKey]opsEntry)
	}
	s.opsCache[key] = opsEntry{ops, err}
	s.opsCacheMu.Unlock()

	return ops, err
}

// buildOperations performs the actual extraction without caching.
func (s *APISpec) buildOperations(opts OperationOptions) ([]Operation, error) {
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Paths == nil {
		return nil, err
	}

	// Use a non-nil empty slice so callers can distinguish "no paths in spec"
	// (nil return) from "paths exist but all were filtered" (empty non-nil slice).
	ops := make([]Operation, 0)
	for rawPath, pathItem := range model.Model.Paths.PathItems.FromOldest() {
		if pathItem == nil {
			continue
		}
		if PathItemExtBool(pathItem, "x-cli-ignore") {
			continue
		}
		pathHidden := PathItemExtBool(pathItem, "x-cli-hidden")

		pathParams := pathItem.Parameters
		for _, mo := range PathItemMethods(pathItem) {
			if mo.Op == nil {
				continue
			}
			servers := model.Model.Servers
			if len(pathItem.Servers) > 0 {
				servers = pathItem.Servers
			}
			if len(mo.Op.Servers) > 0 {
				servers = mo.Op.Servers
			}
			basePath, err := deriveBasePath(opts.BaseURL, opts.OperationBase, servers, opts.ServerVariables)
			if err != nil {
				return nil, fmt.Errorf("derive base path for %s %s: %w", mo.Method, rawPath, err)
			}
			fullPath := joinOperationPath(basePath, rawPath)
			op := extractOperation(mo.Method, fullPath, pathParams, mo.Op)
			if op.XCLI.Ignore {
				continue
			}
			if pathHidden && !op.XCLI.Hidden {
				op.XCLI.Hidden = true
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

// extractOperation converts a single libopenapi operation to the neutral form.
func extractOperation(method, path string, pathParams []*v3.Parameter, op *v3.Operation) Operation {
	o := Operation{
		ID:               op.OperationId,
		Method:           method,
		Path:             path,
		Summary:          op.Summary,
		Description:      op.Description,
		Deprecated:       op.Deprecated != nil && *op.Deprecated,
		Tags:             op.Tags,
		HasBody:          op.RequestBody != nil,
		NoAuth:           op.Security != nil && len(op.Security) == 0,
		RequestMediaType: preferredRequestMediaType(op),
		XCLI: OperationXCLI{
			Ignore:      OpExtBool(op, "x-cli-ignore"),
			Hidden:      OpExtBool(op, "x-cli-hidden"),
			Name:        OpExtString(op, "x-cli-name"),
			Description: OpExtString(op, "x-cli-description"),
			Aliases:     OpExtStrings(op, "x-cli-aliases"),
		},
	}
	o.Help = buildOperationHelp(op, o.RequestMediaType)
	o.RequestSchemaTypes = buildRequestSchemaTypes(op, o.RequestMediaType)

	merged := MergeParameters(pathParams, op.Parameters)
	for _, p := range merged {
		if p == nil {
			continue
		}
		var enum []string
		var paramType, itemType, defaultValue, schemaHelp string
		var hasDefault bool
		if p.Schema != nil {
			if schema := p.Schema.Schema(); schema != nil {
				schemaHelp = buildParameterSchemaHelp(schema)
				paramType = schemaType(schema.Type)
				if paramType == "array" && schema.Items != nil && schema.Items.IsA() && schema.Items.A != nil {
					if itemSchema := schema.Items.A.Schema(); itemSchema != nil {
						itemType = schemaType(itemSchema.Type)
					}
				}
				if schema.Default != nil {
					var decoded any
					if err := schema.Default.Decode(&decoded); err == nil {
						defaultValue = scalarString(decoded)
						hasDefault = true
					}
				}
				for _, node := range schema.Enum {
					if node != nil {
						enum = append(enum, node.Value)
					}
				}
			}
		}
		o.Parameters = append(o.Parameters, Param{
			Name:       p.Name,
			In:         p.In,
			Desc:       p.Description,
			Schema:     schemaHelp,
			Required:   p.Required != nil && *p.Required,
			Type:       paramType,
			ItemType:   itemType,
			Default:    defaultValue,
			HasDefault: hasDefault,
			Style:      p.Style,
			Explode:    p.Explode,
			Enum:       enum,
			XCLI: ParamXCLI{
				Ignore:      ParamExtBool(p, "x-cli-ignore"),
				Hidden:      ParamExtBool(p, "x-cli-hidden"),
				Name:        ParamExtString(p, "x-cli-name"),
				Description: ParamExtString(p, "x-cli-description"),
			},
		})
	}
	return o
}

// deriveBasePath computes the path prefix to prepend to all operation paths.
// When operationBase is set, no prefix is needed (the URL prefix is resolved
// from baseURL+operationBase at call time). Otherwise, the spec's servers[] list is
// inspected for a URL that resolves to the same scheme+host as baseURL.
func deriveBasePath(baseURL, operationBase string, servers []*v3.Server, serverVariables map[string]string) (string, error) {
	if operationBase != "" || len(servers) == 0 {
		return "", nil
	}
	if err := validateConfiguredServerVariables(servers, serverVariables); err != nil {
		return "", err
	}

	location, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	resolutionBase := serverResolutionBase(location)

	for _, server := range servers {
		if server == nil {
			continue
		}
		// Resolve each server variable once, using explicit local config values
		// where present and OpenAPI defaults otherwise. Enum values are
		// intentionally not expanded; remote specs must not be able to create a
		// Cartesian-product allocation during command generation.
		endpoint := resolveServerURLVariables(server, serverVariables)
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", err
		}
		resolved := resolutionBase.ResolveReference(parsed)
		if resolved.Scheme != location.Scheme || resolved.Host != location.Host {
			continue
		}
		if isRelativeServerURL(endpoint) {
			return strings.TrimSuffix(relativeServerBasePath(location.Path, resolved.Path), "/"), nil
		}
		return strings.TrimSuffix(resolved.Path, "/"), nil
	}

	// No matching server found — fall back to the path of the base URL.
	return strings.TrimSuffix(location.Path, "/"), nil
}

func serverResolutionBase(location *url.URL) *url.URL {
	base := *location
	base.RawQuery = ""
	base.Fragment = ""
	if base.Path == "" {
		base.Path = "/"
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	return &base
}

func isRelativeServerURL(endpoint string) bool {
	if endpoint == "" {
		return true
	}
	if strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, "//") {
		return false
	}
	u, err := url.Parse(endpoint)
	return err == nil && !u.IsAbs()
}

func relativeServerBasePath(basePath, resolvedPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return resolvedPath
	}
	if resolvedPath == basePath {
		return ""
	}
	if strings.HasPrefix(resolvedPath, basePath+"/") {
		return strings.TrimPrefix(resolvedPath, basePath)
	}
	return resolvedPath
}

// resolveServerURLVariables returns one concrete URL string for a server by
// substituting explicit local values or OpenAPI defaults. Enum values may be
// useful for validation/help, but are never eagerly expanded.
func resolveServerURLVariables(server *v3.Server, values map[string]string) string {
	if server == nil {
		return ""
	}
	endpoint := server.URL
	if server.Variables == nil {
		return endpoint
	}
	for key, value := range server.Variables.FromOldest() {
		if value == nil {
			continue
		}
		placeholder := fmt.Sprintf("{%s}", key)
		replacement := value.Default
		if configured, ok := values[key]; ok {
			replacement = configured
		}
		endpoint = strings.ReplaceAll(endpoint, placeholder, replacement)
	}
	return endpoint
}

func validateConfiguredServerVariables(servers []*v3.Server, values map[string]string) error {
	for configuredName, configuredValue := range values {
		declared := false
		allowedByAnyEnum := false
		hasEnum := false
		for _, server := range servers {
			if server == nil || server.Variables == nil {
				continue
			}
			variable := server.Variables.GetOrZero(configuredName)
			if variable == nil {
				continue
			}
			declared = true
			if len(variable.Enum) == 0 {
				allowedByAnyEnum = true
				continue
			}
			hasEnum = true
			for _, enumValue := range variable.Enum {
				if configuredValue == enumValue {
					allowedByAnyEnum = true
					break
				}
			}
		}
		if !declared {
			return fmt.Errorf("server variable %q is configured but not declared by the OpenAPI servers", configuredName)
		}
		if hasEnum && !allowedByAnyEnum {
			return fmt.Errorf("server variable %q value %q is not allowed by the OpenAPI enum", configuredName, configuredValue)
		}
	}
	return nil
}

// MergeParameters merges path-level and operation-level parameters.
// Operation-level parameters override path-level ones with the same (in, name).
func MergeParameters(pathLevel, operationLevel []*v3.Parameter) []*v3.Parameter {
	if len(pathLevel) == 0 {
		return operationLevel
	}
	if len(operationLevel) == 0 {
		return pathLevel
	}
	merged := make([]*v3.Parameter, 0, len(pathLevel)+len(operationLevel))
	indexes := make(map[string]int, len(pathLevel)+len(operationLevel))
	add := func(p *v3.Parameter) {
		if p == nil {
			return
		}
		key := p.In + "\x00" + p.Name
		if idx, ok := indexes[key]; ok {
			merged[idx] = p
			return
		}
		indexes[key] = len(merged)
		merged = append(merged, p)
	}
	for _, p := range pathLevel {
		add(p)
	}
	for _, p := range operationLevel {
		add(p)
	}
	return merged
}

func schemaType(types []string) string {
	for _, t := range types {
		if t != "null" {
			return t
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return "string"
}

func scalarString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprint(v)
	case int64:
		return fmt.Sprint(v)
	case float64:
		return fmt.Sprint(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, scalarString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

func preferredRequestMediaType(op *v3.Operation) string {
	if op == nil || op.RequestBody == nil || op.RequestBody.Content == nil {
		return ""
	}
	var names []string
	for name := range op.RequestBody.Content.FromOldest() {
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	for _, name := range names {
		mt := strings.ToLower(strings.TrimSpace(strings.Split(name, ";")[0]))
		if mt == "application/json" || strings.HasSuffix(mt, "+json") {
			return name
		}
	}
	sort.Strings(names)
	return names[0]
}

func joinOperationPath(basePath, opPath string) string {
	if basePath == "" || basePath == "/" {
		return opPath
	}
	return strings.TrimRight(basePath, "/") + opPath
}
