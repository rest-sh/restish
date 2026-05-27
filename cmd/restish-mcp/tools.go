package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/rest-sh/restish/v2/plugin"
)

type APISpec struct {
	Name        string
	ContentType string
	Raw         []byte
	Document    libopenapi.Document
	Operations  []plugin.APIOperation
}

type HTTPExecutor func(*HTTPRequest) (*HTTPResponse, error)
type SpecFetcher func(name string) (*APISpec, error)

type Tool struct {
	APIName         string
	Name            string
	Description     string
	Method          string
	Path            string
	InputSchema     map[string]any
	Params          []Param
	BodyContentType string
	BodyRequired    bool
}

type Param struct {
	Name             string
	In               string
	Required         bool
	Description      string
	Type             string
	ItemType         string
	Style            string
	Explode          *bool
	AllowReserved    bool
	ContentMediaType string
	Schema           map[string]any
}

type Options struct {
	Operations      map[string]bool
	ReadOnly        bool
	AllowWriteTools bool
	MaxResultBytes  int
	RequestTimeout  int
}

type ToolLoadStats struct {
	HiddenWriteOperations int
}

func LoadTools(fetchSpec SpecFetcher, apiNames []string, opts Options) ([]*Tool, error) {
	tools, _, err := LoadToolsWithStats(fetchSpec, apiNames, opts)
	return tools, err
}

func LoadToolsWithStats(fetchSpec SpecFetcher, apiNames []string, opts Options) ([]*Tool, ToolLoadStats, error) {
	multiAPI := len(apiNames) > 1
	var tools []*Tool
	var stats ToolLoadStats
	for _, apiName := range apiNames {
		s, err := fetchSpec(apiName)
		if err != nil {
			return nil, stats, err
		}
		apiTools, apiStats, err := toolsFromSpecWithStats(apiName, multiAPI, s, opts)
		if err != nil {
			return nil, stats, err
		}
		tools = append(tools, apiTools...)
		stats.HiddenWriteOperations += apiStats.HiddenWriteOperations
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, stats, nil
}

func toolsFromSpec(apiName string, multiAPI bool, s *APISpec, opts Options) ([]*Tool, error) {
	tools, _, err := toolsFromSpecWithStats(apiName, multiAPI, s, opts)
	return tools, err
}

func toolsFromSpecWithStats(apiName string, multiAPI bool, s *APISpec, opts Options) ([]*Tool, ToolLoadStats, error) {
	if len(s.Operations) > 0 {
		tools, stats := toolsFromOperations(apiName, multiAPI, s.Operations, opts)
		return tools, stats, nil
	}
	model, err := s.Document.BuildV3Model()
	if err != nil || model == nil || model.Model.Paths == nil {
		return nil, ToolLoadStats{}, fmt.Errorf("building OpenAPI model for %q: %w", apiName, err)
	}

	var tools []*Tool
	var stats ToolLoadStats
	for path, pathItem := range model.Model.Paths.PathItems.FromOldest() {
		for _, item := range spec.PathItemMethods(pathItem) {
			if item.Op == nil || item.Op.OperationId == "" {
				continue
			}
			if spec.OpExtBool(item.Op, "x-cli-ignore") || spec.OpExtBool(item.Op, "x-mcp-ignore") {
				continue
			}
			if !mcpMethodAllowed(item.Method, opts) {
				if mcpWriteMethod(item.Method) && !opts.AllowWriteTools {
					stats.HiddenWriteOperations++
				}
				continue
			}
			if len(opts.Operations) > 0 && !opts.Operations[item.Op.OperationId] {
				continue
			}
			tool, err := buildTool(apiName, multiAPI, path, item.Method, pathItem.Parameters, item.Op)
			if err != nil {
				return nil, stats, err
			}
			tools = append(tools, tool)
		}
	}
	return tools, stats, nil
}

func toolsFromOperations(apiName string, multiAPI bool, ops []plugin.APIOperation, opts Options) ([]*Tool, ToolLoadStats) {
	var tools []*Tool
	var stats ToolLoadStats
	for _, op := range ops {
		if op.ID == "" || op.MCPIgnore {
			continue
		}
		if !mcpMethodAllowed(op.Method, opts) {
			if mcpWriteMethod(op.Method) && !opts.AllowWriteTools {
				stats.HiddenWriteOperations++
			}
			continue
		}
		if len(opts.Operations) > 0 && !opts.Operations[op.ID] {
			continue
		}
		tools = append(tools, buildToolFromOperation(apiName, multiAPI, op))
	}
	return tools, stats
}

func mcpMethodAllowed(method string, opts Options) bool {
	upper := strings.ToUpper(method)
	if opts.ReadOnly {
		return upper == "GET" || upper == "HEAD"
	}
	if opts.AllowWriteTools {
		return true
	}
	if mcpWriteMethod(upper) {
		return false
	}
	return true
}

func mcpWriteMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func buildToolFromOperation(apiName string, multiAPI bool, op plugin.APIOperation) *Tool {
	name := op.ID
	if multiAPI {
		name = apiName + "__" + name
	}
	description := strings.TrimSpace(op.Summary)
	if description == "" {
		description = strings.TrimSpace(op.Description)
	}
	if description == "" {
		description = op.Method + " " + op.Path
	}

	properties := map[string]any{}
	var required []string
	params := make([]Param, 0, len(op.Parameters))
	for _, p := range op.Parameters {
		var paramSchema map[string]any
		if len(p.Schema) > 0 {
			paramSchema = schemaMap(p.Schema)
		}
		params = append(params, Param{
			Name:             p.Name,
			In:               p.In,
			Required:         p.Required,
			Description:      p.Description,
			Type:             p.Type,
			ItemType:         p.ItemType,
			Style:            p.Style,
			Explode:          p.Explode,
			AllowReserved:    p.AllowReserved,
			ContentMediaType: p.ContentMediaType,
			Schema:           paramSchema,
		})
		prop := schemaFromOperationParam(p)
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	if op.HasBody {
		properties["body"] = map[string]any{"type": "object"}
		if op.BodyRequired {
			required = append(required, "body")
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}
	return &Tool{
		APIName:         apiName,
		Name:            name,
		Description:     description,
		Method:          op.Method,
		Path:            op.Path,
		InputSchema:     inputSchema,
		Params:          params,
		BodyContentType: op.RequestMediaType,
		BodyRequired:    op.BodyRequired,
	}
}

func schemaFromOperationParam(p plugin.APIParam) map[string]any {
	if len(p.Schema) > 0 {
		prop := schemaMap(p.Schema)
		if p.Description != "" {
			prop["description"] = p.Description
		}
		if len(p.Enum) > 0 {
			enum := make([]any, len(p.Enum))
			for i, value := range p.Enum {
				enum[i] = value
			}
			prop["enum"] = enum
		}
		return prop
	}
	typ := p.Type
	if typ == "" {
		typ = "string"
	}
	prop := map[string]any{"type": typ}
	if typ == "array" && p.ItemType != "" {
		prop["items"] = map[string]any{"type": p.ItemType}
	}
	if p.Description != "" {
		prop["description"] = p.Description
	}
	if len(p.Enum) > 0 {
		enum := make([]any, len(p.Enum))
		for i, value := range p.Enum {
			enum[i] = value
		}
		prop["enum"] = enum
	}
	return prop
}

func buildTool(apiName string, multiAPI bool, path, method string, pathParams []*v3high.Parameter, op *v3high.Operation) (*Tool, error) {
	name := op.OperationId
	if multiAPI {
		name = apiName + "__" + name
	}
	description := strings.TrimSpace(op.Summary)
	if description == "" {
		description = strings.TrimSpace(op.Description)
	}
	if description == "" {
		description = method + " " + path
	}

	properties := map[string]any{}
	var required []string
	var params []Param
	for _, p := range spec.MergeParameters(pathParams, op.Parameters) {
		prop := schemaMap(nil)
		contentMediaType := parameterContentMediaType(p)
		if contentMediaType != "" {
			prop = parameterContentSchemaMap(p, contentMediaType)
		} else if p.Schema != nil && p.Schema.Schema() != nil {
			prop = schemaMap(p.Schema.Schema())
		}
		typ, itemType := schemaTypeInfo(prop)
		params = append(params, Param{
			Name:             p.Name,
			In:               p.In,
			Required:         p.Required != nil && *p.Required,
			Description:      p.Description,
			Type:             typ,
			ItemType:         itemType,
			Style:            p.Style,
			Explode:          p.Explode,
			AllowReserved:    p.AllowReserved,
			ContentMediaType: contentMediaType,
			Schema:           schemaMap(prop),
		})
		if p.Description != "" {
			prop["description"] = p.Description
		}
		properties[p.Name] = prop
		if p.Required != nil && *p.Required {
			required = append(required, p.Name)
		}
	}

	bodyContentType, bodyRequired := "", false
	if op.RequestBody != nil {
		bodyContentType, properties["body"] = requestBodyProperty(op.RequestBody)
		bodyRequired = requestBodyRequired(op.RequestBody)
		if bodyRequired {
			required = append(required, "body")
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}
	return &Tool{
		APIName:         apiName,
		Name:            name,
		Description:     description,
		Method:          method,
		Path:            path,
		InputSchema:     inputSchema,
		Params:          params,
		BodyContentType: bodyContentType,
		BodyRequired:    bodyRequired,
	}, nil
}

func requestBodyProperty(body *v3high.RequestBody) (string, map[string]any) {
	if body == nil || body.Content == nil {
		return "", map[string]any{"type": "object"}
	}
	for _, contentType := range []string{"application/json", "application/merge-patch+json"} {
		if media := body.Content.GetOrZero(contentType); media != nil {
			return contentType, bodySchema(media, body.Description)
		}
	}
	for contentType, media := range body.Content.FromOldest() {
		if media != nil {
			return contentType, bodySchema(media, body.Description)
		}
	}
	return "", map[string]any{"type": "object"}
}

func requestBodyRequired(body *v3high.RequestBody) bool {
	return body != nil && body.Required != nil && *body.Required
}

func bodySchema(media *v3high.MediaType, description string) map[string]any {
	prop := map[string]any{"type": "object"}
	if media != nil && media.Schema != nil && media.Schema.Schema() != nil {
		prop = schemaMap(media.Schema.Schema())
	}
	if description != "" {
		prop["description"] = description
	}
	return prop
}

func parameterContentMediaType(p *v3high.Parameter) string {
	if p == nil || p.Content == nil {
		return ""
	}
	var names []string
	for name := range p.Content.FromOldest() {
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

func parameterContentSchemaMap(p *v3high.Parameter, contentType string) map[string]any {
	if p == nil || p.Content == nil || contentType == "" {
		return schemaMap(nil)
	}
	media := p.Content.GetOrZero(contentType)
	if media == nil || media.Schema == nil || media.Schema.Schema() == nil {
		return schemaMap(nil)
	}
	return schemaMap(media.Schema.Schema())
}

func schemaMap(v any) map[string]any {
	if v == nil {
		return map[string]any{"type": "string"}
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) == 0 {
		return map[string]any{"type": "string"}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
		return map[string]any{"type": "string"}
	}
	return out
}

func schemaTypeInfo(prop map[string]any) (string, string) {
	typ, _ := prop["type"].(string)
	itemType := ""
	if items, ok := prop["items"].(map[string]any); ok {
		itemType, _ = items["type"].(string)
	}
	return typ, itemType
}

func indexTools(tools []*Tool) map[string]*Tool {
	out := make(map[string]*Tool, len(tools))
	for _, tool := range tools {
		out[tool.Name] = tool
	}
	return out
}
