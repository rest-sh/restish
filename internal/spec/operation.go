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
	// RequestMediaType is the deterministic preferred content type from
	// requestBody.content, if the operation accepts a body.
	RequestMediaType string
	XCLI             OperationXCLI
}

// OperationSet is the extracted operation list plus API-level metadata needed
// to present the generated command group without reparsing the raw spec.
type OperationSet struct {
	Info       APIInfo
	Operations []Operation
}

// OperationSet returns all operations with top-level API metadata.
func (s *APISpec) OperationSet(baseURL, operationBase string) (OperationSet, error) {
	ops, err := s.Operations(baseURL, operationBase)
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
	key := opsKey{baseURL, operationBase}

	s.opsCacheMu.Lock()
	if s.opsCache != nil {
		if e, ok := s.opsCache[key]; ok {
			s.opsCacheMu.Unlock()
			return e.ops, e.err
		}
	}
	s.opsCacheMu.Unlock()

	ops, err := s.buildOperations(baseURL, operationBase)

	s.opsCacheMu.Lock()
	if s.opsCache == nil {
		s.opsCache = make(map[opsKey]opsEntry)
	}
	s.opsCache[key] = opsEntry{ops, err}
	s.opsCacheMu.Unlock()

	return ops, err
}

// buildOperations performs the actual extraction without caching.
func (s *APISpec) buildOperations(baseURL, operationBase string) ([]Operation, error) {
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Paths == nil {
		return nil, err
	}

	basePath, err := deriveBasePath(baseURL, operationBase, model.Model.Servers)
	if err != nil {
		return nil, fmt.Errorf("derive base path: %w", err)
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
		fullPath := joinOperationPath(basePath, rawPath)

		pathParams := pathItem.Parameters
		for _, mo := range PathItemMethods(pathItem) {
			if mo.Op == nil {
				continue
			}
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
		RequestMediaType: preferredRequestMediaType(op),
		XCLI: OperationXCLI{
			Ignore:      OpExtBool(op, "x-cli-ignore"),
			Hidden:      OpExtBool(op, "x-cli-hidden"),
			Name:        OpExtString(op, "x-cli-name"),
			Description: OpExtString(op, "x-cli-description"),
			Aliases:     OpExtStrings(op, "x-cli-aliases"),
		},
	}

	merged := mergeParameters(pathParams, op.Parameters)
	for _, p := range merged {
		if p == nil {
			continue
		}
		var enum []string
		var paramType, itemType, defaultValue string
		var hasDefault bool
		if p.Schema != nil {
			if schema := p.Schema.Schema(); schema != nil {
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
// inspected for a URL that shares the same scheme+host as baseURL.
func deriveBasePath(baseURL, operationBase string, servers []*v3.Server) (string, error) {
	if operationBase != "" || len(servers) == 0 {
		return "", nil
	}

	location, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("%s://%s", location.Scheme, location.Host)

	for _, server := range servers {
		if server == nil {
			continue
		}
		// Relative paths (starting with "/") always apply.
		if strings.HasPrefix(server.URL, "/") {
			return strings.TrimSuffix(server.URL, "/"), nil
		}

		// Absolute URL: expand server variables then match against baseURL host.
		endpoints := expandServerVariables(server)
		for _, endpoint := range endpoints {
			if !strings.HasPrefix(endpoint, prefix) {
				continue
			}
			parsed, err := url.Parse(endpoint)
			if err != nil {
				return "", err
			}
			return strings.TrimSuffix(parsed.Path, "/"), nil
		}
	}

	// No matching server found — fall back to the path of the base URL.
	return strings.TrimSuffix(location.Path, "/"), nil
}

// expandServerVariables returns all concrete URL strings for a server by
// substituting its variable defaults (and enums when present).
func expandServerVariables(server *v3.Server) []string {
	endpoints := []string{server.URL}
	if server.Variables == nil {
		return endpoints
	}
	for key, value := range server.Variables.FromOldest() {
		if value == nil {
			continue
		}
		placeholder := fmt.Sprintf("{%s}", key)
		if len(value.Enum) == 0 {
			for i := range endpoints {
				endpoints[i] = strings.ReplaceAll(endpoints[i], placeholder, value.Default)
			}
			continue
		}
		next := make([]string, 0, len(endpoints)*len(value.Enum))
		for _, enumVal := range value.Enum {
			for _, ep := range endpoints {
				next = append(next, strings.ReplaceAll(ep, placeholder, enumVal))
			}
		}
		endpoints = next
	}
	return endpoints
}

// mergeParameters merges path-level and operation-level parameters.
// Operation-level parameters override path-level ones with the same (in, name).
func mergeParameters(pathLevel, operationLevel []*v3.Parameter) []*v3.Parameter {
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
