package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type schemaHelpMode int

const (
	schemaHelpRead schemaHelpMode = iota
	schemaHelpWrite
)

const (
	schemaHelpMaxDepth       = 5
	schemaHelpMaxProperties  = 20
	schemaHelpMaxErrorGroups = 3
	schemaHelpMaxExampleCLI  = 150
)

type schemaHelpRenderer struct {
	mode  schemaHelpMode
	seen  map[uint64]bool
	depth int
}

func buildOperationHelp(op *v3.Operation, requestMediaType string) OperationHelp {
	if op == nil {
		return OperationHelp{}
	}

	help := OperationHelp{
		Request:  buildRequestHelp(op, requestMediaType),
		Examples: buildCommandExamples(op, requestMediaType),
	}
	help.Responses = buildResponseHelp(op)
	return help
}

func buildParameterSchemaHelp(schema *base.Schema) string {
	if schema == nil {
		return ""
	}
	return schemaHelpRenderer{mode: schemaHelpWrite, seen: map[uint64]bool{}}.render(schema, "")
}

func buildRequestHelp(op *v3.Operation, requestMediaType string) *OperationBodyHelp {
	if op.RequestBody == nil || op.RequestBody.Content == nil || requestMediaType == "" {
		return nil
	}
	mt := op.RequestBody.Content.GetOrZero(requestMediaType)
	if mt == nil {
		return nil
	}
	schema := mediaTypeSchema(mt)
	if schema == nil {
		return &OperationBodyHelp{MediaType: requestMediaType}
	}
	return &OperationBodyHelp{
		MediaType: requestMediaType,
		Schema:    schemaHelpRenderer{mode: schemaHelpWrite, seen: map[uint64]bool{}}.render(schema, ""),
		Example:   renderExampleJSON(firstMediaTypeExample(mt, schema, schemaHelpWrite)),
	}
}

func buildCommandExamples(op *v3.Operation, requestMediaType string) []string {
	if op.RequestBody == nil || op.RequestBody.Content == nil || requestMediaType == "" {
		return nil
	}
	mt := op.RequestBody.Content.GetOrZero(requestMediaType)
	if mt == nil {
		return nil
	}
	example := firstMediaTypeExample(mt, mediaTypeSchema(mt), schemaHelpWrite)
	if example == nil {
		return nil
	}
	if body, ok := example.(map[string]any); ok {
		rendered := shorthand.MarshalCLI(body)
		if rendered != "" && len(rendered) <= schemaHelpMaxExampleCLI {
			return []string{rendered}
		}
	}
	return []string{"<input.json"}
}

func buildResponseHelp(op *v3.Operation) []OperationResponseHelp {
	if op.Responses == nil {
		return nil
	}
	codes := responseCodes(op)
	if len(codes) == 0 {
		return nil
	}

	groupsByKey := map[string]*responseHelpGroup{}
	var groups []*responseHelpGroup
	for _, code := range codes {
		resp := responseForCode(op, code)
		if resp == nil {
			continue
		}
		mtName, mt := preferredResponseMediaType(resp)
		schema := mediaTypeSchema(mt)
		key := responseGroupKey(mtName, schema)
		group := groupsByKey[key]
		if group == nil {
			group = &responseHelpGroup{
				mediaType: mtName,
				schema:    schema,
				noBody:    mt == nil || schema == nil,
			}
			groupsByKey[key] = group
			groups = append(groups, group)
		}
		group.codes = append(group.codes, code)
		if group.description == "" {
			group.description = strings.TrimSpace(resp.Description)
		}
	}

	var success []OperationResponseHelp
	var errors []OperationResponseHelp
	for _, group := range groups {
		help := group.toHelp()
		if len(help.Codes) == 0 {
			continue
		}
		if group.hasSuccess() {
			success = append(success, help)
			continue
		}
		errors = append(errors, help)
	}
	sortResponseHelps(success)
	sortResponseHelps(errors)
	if len(errors) > schemaHelpMaxErrorGroups {
		errors = errors[:schemaHelpMaxErrorGroups]
	}

	var out []OperationResponseHelp
	if len(success) > 0 {
		out = append(out, success[0])
	}
	out = append(out, errors...)
	return out
}

type responseHelpGroup struct {
	codes       []string
	mediaType   string
	schema      *base.Schema
	noBody      bool
	description string
}

func (g *responseHelpGroup) hasSuccess() bool {
	for _, code := range g.codes {
		if isSuccessResponseCode(code) {
			return true
		}
	}
	return false
}

func (g *responseHelpGroup) toHelp() OperationResponseHelp {
	sort.SliceStable(g.codes, func(i, j int) bool {
		return responseCodeSortKey(g.codes[i]) < responseCodeSortKey(g.codes[j])
	})
	help := OperationResponseHelp{
		Codes:       append([]string(nil), g.codes...),
		MediaType:   g.mediaType,
		NoBody:      g.noBody,
		Description: g.description,
	}
	if g.schema != nil {
		help.Schema = schemaHelpRenderer{mode: schemaHelpRead, seen: map[uint64]bool{}}.render(g.schema, "")
		help.Example = renderExampleJSON(genSchemaExample(g.schema, schemaHelpRead, map[uint64]bool{}, 0))
	}
	return help
}

func sortResponseHelps(helps []OperationResponseHelp) {
	sort.SliceStable(helps, func(i, j int) bool {
		return responseCodeSortKey(helps[i].Codes[0]) < responseCodeSortKey(helps[j].Codes[0])
	})
}

func responseCodes(op *v3.Operation) []string {
	var codes []string
	if op.Responses.Codes != nil {
		for code := range op.Responses.Codes.FromOldest() {
			codes = append(codes, code)
		}
	}
	if op.Responses.Default != nil {
		codes = append(codes, "default")
	}
	sort.SliceStable(codes, func(i, j int) bool {
		return responseCodeSortKey(codes[i]) < responseCodeSortKey(codes[j])
	})
	return codes
}

func responseForCode(op *v3.Operation, code string) *v3.Response {
	if code == "default" {
		return op.Responses.Default
	}
	if op.Responses.Codes == nil {
		return nil
	}
	return op.Responses.Codes.GetOrZero(code)
}

func responseCodeSortKey(code string) string {
	if code == "default" {
		return "999"
	}
	return code
}

func isSuccessResponseCode(code string) bool {
	return len(code) == 3 && code[0] == '2'
}

func preferredResponseMediaType(resp *v3.Response) (string, *v3.MediaType) {
	if resp == nil || resp.Content == nil || resp.Content.Len() == 0 {
		return "", nil
	}
	names := make([]string, 0, resp.Content.Len())
	for name := range resp.Content.FromOldest() {
		names = append(names, name)
	}
	for _, name := range names {
		mt := strings.ToLower(strings.TrimSpace(strings.Split(name, ";")[0]))
		if mt == "application/json" || strings.HasSuffix(mt, "+json") {
			return name, resp.Content.GetOrZero(name)
		}
	}
	sort.Strings(names)
	return names[0], resp.Content.GetOrZero(names[0])
}

func responseGroupKey(mediaType string, schema *base.Schema) string {
	if schema == nil {
		return mediaType + ":empty"
	}
	return mediaType + ":" + fmt.Sprintf("%x", schema.GoLow().Hash())
}

func mediaTypeSchema(mt *v3.MediaType) *base.Schema {
	if mt == nil || mt.Schema == nil {
		return nil
	}
	return mt.Schema.Schema()
}

func firstMediaTypeExample(mt *v3.MediaType, schema *base.Schema, mode schemaHelpMode) any {
	if mt == nil {
		return nil
	}
	if v, ok := decodeYAMLNode(mt.Example); ok {
		return v
	}
	if mt.Examples != nil {
		for _, ex := range mt.Examples.FromOldest() {
			if ex == nil {
				continue
			}
			if v, ok := decodeYAMLNode(ex.Value); ok {
				return v
			}
		}
	}
	return genSchemaExample(schema, mode, map[uint64]bool{}, 0)
}

func renderExampleJSON(value any) string {
	if value == nil {
		return ""
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func decodeYAMLNode(node interface{ Decode(any) error }) (any, bool) {
	if node == nil {
		return nil, false
	}
	valueOf := reflect.ValueOf(node)
	if valueOf.Kind() == reflect.Ptr && valueOf.IsNil() {
		return nil, false
	}
	var value any
	if err := node.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

func genSchemaExample(s *base.Schema, mode schemaHelpMode, seen map[uint64]bool, depth int) any {
	if s == nil || depth > schemaHelpMaxDepth {
		return nil
	}
	if len(s.OneOf) > 0 {
		return genSchemaProxyExample(s.OneOf[0], mode, seen, depth+1)
	}
	if len(s.AnyOf) > 0 {
		return genSchemaProxyExample(s.AnyOf[0], mode, seen, depth+1)
	}
	if len(s.AllOf) > 0 {
		result := map[string]any{}
		for _, proxy := range s.AllOf {
			if part, ok := genSchemaProxyExample(proxy, mode, seen, depth+1).(map[string]any); ok {
				for k, v := range part {
					result[k] = v
				}
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if v, ok := decodeYAMLNode(s.Example); ok {
		return v
	}
	if len(s.Examples) > 0 {
		if v, ok := decodeYAMLNode(s.Examples[0]); ok {
			return v
		}
	}
	if v, ok := decodeYAMLNode(s.Default); ok {
		return v
	}
	if len(s.Enum) > 0 {
		if v, ok := decodeYAMLNode(s.Enum[0]); ok {
			return v
		}
	}

	switch schemaKind(s) {
	case "boolean":
		return true
	case "integer":
		return 1
	case "number":
		return 1.0
	case "array":
		if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
			return []any{genSchemaProxyExample(s.Items.A, mode, seen, depth+1)}
		}
		return []any{nil}
	case "object":
		hash := s.GoLow().Hash()
		if seen[hash] {
			return nil
		}
		seen[hash] = true
		defer func() { seen[hash] = false }()

		value := map[string]any{}
		if s.Properties != nil {
			count := 0
			for name, proxy := range s.Properties.FromOldest() {
				if count >= schemaHelpMaxProperties {
					value["..."] = nil
					break
				}
				prop := schemaFromProxy(proxy)
				if prop == nil || skipSchemaForMode(prop, mode) {
					continue
				}
				if isSensitiveSchemaName(name) {
					value[name] = "<redacted>"
				} else {
					value[name] = genSchemaExample(prop, mode, seen, depth+1)
				}
				count++
			}
		}
		if len(value) == 0 && s.AdditionalProperties != nil {
			if s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
				value["<any>"] = genSchemaProxyExample(s.AdditionalProperties.A, mode, seen, depth+1)
			} else if s.AdditionalProperties.IsB() && s.AdditionalProperties.B {
				value["<any>"] = nil
			}
		}
		return value
	case "string":
		switch s.Format {
		case "date":
			return "2020-05-14"
		case "time":
			return "23:44:51-07:00"
		case "date-time":
			return "2020-05-14T23:44:51-07:00"
		case "email", "idn-email":
			return "user@example.com"
		case "hostname", "idn-hostname":
			return "example.com"
		case "ipv4":
			return "192.0.2.1"
		case "ipv6":
			return "2001:db8::1"
		case "uuid":
			return "3e4666bf-d5e5-4aa7-b8ce-cefe41c7568a"
		case "uri", "iri":
			return "https://example.com/"
		case "uri-reference", "iri-reference":
			return "/example"
		case "password":
			return "<redacted>"
		}
		return "string"
	default:
		return nil
	}
}

func genSchemaProxyExample(proxy *base.SchemaProxy, mode schemaHelpMode, seen map[uint64]bool, depth int) any {
	return genSchemaExample(schemaFromProxy(proxy), mode, seen, depth)
}

func (r schemaHelpRenderer) render(s *base.Schema, indent string) string {
	if s == nil {
		return "<any>"
	}
	if r.depth >= schemaHelpMaxDepth {
		return "<...>"
	}
	if len(s.AllOf) > 0 || len(s.OneOf) > 0 || len(s.AnyOf) > 0 {
		return r.renderComposite(s, indent)
	}

	switch schemaKind(s) {
	case "boolean", "integer", "number", "string":
		return r.renderScalar(s)
	case "array":
		if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
			child := r.child()
			return "[\n  " + indent + child.render(schemaFromProxy(s.Items.A), indent+"  ") + "\n" + indent + "]"
		}
		return "[<any>]"
	case "object":
		return r.renderObject(s, indent)
	default:
		return "<any>"
	}
}

func (r schemaHelpRenderer) child() schemaHelpRenderer {
	r.depth++
	return r
}

func (r schemaHelpRenderer) renderComposite(s *base.Schema, indent string) string {
	for _, item := range []struct {
		label   string
		schemas []*base.SchemaProxy
	}{
		{"allOf", s.AllOf},
		{"oneOf", s.OneOf},
		{"anyOf", s.AnyOf},
	} {
		if len(item.schemas) == 0 {
			continue
		}
		var out strings.Builder
		out.WriteString(item.label + "{\n")
		child := r.child()
		for _, proxy := range item.schemas {
			out.WriteString(indent + "  " + child.render(schemaFromProxy(proxy), indent+"  ") + "\n")
		}
		out.WriteString(indent + "}")
		return out.String()
	}
	return "<any>"
}

func (r schemaHelpRenderer) renderObject(s *base.Schema, indent string) string {
	if s.Properties == nil || s.Properties.Len() == 0 {
		if s.AdditionalProperties == nil {
			return "(object)"
		}
	}
	hash := s.GoLow().Hash()
	if r.seen[hash] {
		return "<recursive ref>"
	}
	r.seen[hash] = true
	defer func() { r.seen[hash] = false }()

	var out strings.Builder
	out.WriteString("{\n")
	count := 0
	if s.Properties != nil {
		for _, name := range sortedSchemaPropertyNames(s) {
			if count >= schemaHelpMaxProperties {
				out.WriteString(indent + "  ...: <...>\n")
				break
			}
			prop := schemaFromProxy(s.Properties.GetOrZero(name))
			if prop == nil || skipSchemaForMode(prop, r.mode) {
				continue
			}
			label := name
			if stringInSlice(name, s.Required) {
				label += "*"
			}
			out.WriteString(indent + "  " + label + ": " + r.child().render(prop, indent+"  ") + "\n")
			count++
		}
	}
	if s.AdditionalProperties != nil {
		if s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
			out.WriteString(indent + "  <any>: " + r.child().render(schemaFromProxy(s.AdditionalProperties.A), indent+"  ") + "\n")
		} else if s.AdditionalProperties.IsB() && s.AdditionalProperties.B {
			out.WriteString(indent + "  <any>: <any>\n")
		}
	}
	out.WriteString(indent + "}")
	return out.String()
}

func (r schemaHelpRenderer) renderScalar(s *base.Schema) string {
	var tags []string
	if s.Format != "" {
		tags = append(tags, "format:"+s.Format)
	}
	if s.Default != nil {
		if v, ok := decodeYAMLNode(s.Default); ok {
			tags = append(tags, "default:"+fmt.Sprint(v))
		}
	}
	if len(s.Enum) > 0 {
		var vals []string
		for _, node := range s.Enum {
			if v, ok := decodeYAMLNode(node); ok {
				vals = append(vals, fmt.Sprint(v))
			}
		}
		if len(vals) > 0 {
			tags = append(tags, "enum:"+strings.Join(vals, ","))
		}
	}
	if s.Pattern != "" {
		tags = append(tags, "pattern:"+s.Pattern)
	}
	if s.MinLength != nil && *s.MinLength > 0 {
		tags = append(tags, fmt.Sprintf("minLen:%d", *s.MinLength))
	}
	if s.MaxLength != nil && *s.MaxLength > 0 {
		tags = append(tags, fmt.Sprintf("maxLen:%d", *s.MaxLength))
	}
	typ := schemaType(s.Type)
	if len(s.Type) > 1 {
		typ = strings.Join(s.Type, "|")
	}
	if len(tags) > 0 {
		typ += " " + strings.Join(tags, " ")
	}
	doc := strings.TrimSpace(s.Title)
	if doc == "" {
		doc = strings.TrimSpace(s.Description)
	}
	if doc != "" {
		return fmt.Sprintf("(%s) %s", typ, oneLine(doc))
	}
	return fmt.Sprintf("(%s)", typ)
}

func schemaKind(s *base.Schema) string {
	if s == nil {
		return ""
	}
	if t := schemaType(s.Type); t != "" && !(t == "string" && len(s.Type) == 0) {
		return t
	}
	if s.Items != nil {
		return "array"
	}
	if (s.Properties != nil && s.Properties.Len() > 0) || s.AdditionalProperties != nil {
		return "object"
	}
	return "string"
}

func schemaFromProxy(proxy *base.SchemaProxy) *base.Schema {
	if proxy == nil {
		return nil
	}
	return proxy.Schema()
}

func skipSchemaForMode(s *base.Schema, mode schemaHelpMode) bool {
	if s == nil {
		return true
	}
	if mode == schemaHelpWrite && boolValue(s.ReadOnly) {
		return true
	}
	if mode == schemaHelpRead && boolValue(s.WriteOnly) {
		return true
	}
	return false
}

func boolValue(v *bool) bool {
	return v != nil && *v
}

func sortedSchemaPropertyNames(s *base.Schema) []string {
	if s == nil || s.Properties == nil {
		return nil
	}
	var names []string
	for name := range s.Properties.KeysFromOldest() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func stringInSlice(needle string, haystack []string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func isSensitiveSchemaName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"password", "secret", "token", "api_key", "apikey", "private_key", "credential", "bearer"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
