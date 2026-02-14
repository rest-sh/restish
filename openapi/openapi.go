package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/danielgtaylor/casing"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/gosimple/slug"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"github.com/rest-sh/restish/cli"
)

// reOpenAPI3 is a regex used to detect OpenAPI files from their contents.
var reOpenAPI3 = regexp.MustCompile(`['"]?openapi['"]?\s*:\s*['"]?3`)

// OpenAPI Extensions
const (
	// Change the CLI name for an operation or parameter
	ExtName = "x-cli-name"

	// Set additional command aliases for an operation
	ExtAliases = "x-cli-aliases"

	// Change the description of an operation or parameter
	ExtDescription = "x-cli-description"

	// Ignore a path, operation, or parameter
	ExtIgnore = "x-cli-ignore"

	// Create a hidden command for an operation. It will not show in the help,
	// but can still be called.
	ExtHidden = "x-cli-hidden"

	// Custom auto-configuration for CLIs
	ExtCLIConfig = "x-cli-config"
)

type autoConfig struct {
	Security string                       `json:"security"`
	Headers  map[string]string            `json:"headers,omitempty"`
	Prompt   map[string]cli.AutoConfigVar `json:"prompt,omitempty"`
	Params   map[string]string            `json:"params,omitempty"`
}

// Resolver is able to resolve relative URIs against a base.
type Resolver interface {
	GetBase() *url.URL
	Resolve(uri string) (*url.URL, error)
}

func getExt[T any](v *orderedmap.Map[string, *yaml.Node], key string) (T, error) {
	var val T
	if v == nil {
		return val, errors.New("extension map is unset")
	}

	ext, exists := v.Get(key)
	if !exists {
		return val, fmt.Errorf("extension %q not found", key)
	}

	err := ext.Decode(&val)
	if err == nil {
		return val, nil
	}

	return val, err
}

// getExt returns an extension converted to some type with the given default
// returned if the extension is not found or cannot be cast to that type.
func getExtOr[T any](v *orderedmap.Map[string, *yaml.Node], key string, def T) T {
	if val, err := getExt[T](v, key); err == nil {
		return val
	}

	return def
}

// getExtSlice returns an extension converted to some type with the given
// default returned if the extension is not found or cannot be converted to
// a slice of the correct type.
func getExtSlice[T any](v *orderedmap.Map[string, *yaml.Node], key string, def []T) []T {
	return getExtOr(v, key, def)
}

func decodeYAML(node *yaml.Node) (any, error) {
	var value any
	err := node.Decode(&value)
	return value, err
}

// getBasePath returns the basePath to which the operation paths need to be appended (if any)
// It assumes the open-api description has been validated before: the casts should always succeed
// if the description adheres to the openapi spec schema.
func getBasePath(location *url.URL, servers []*v3.Server) (string, error) {
	prefix := fmt.Sprintf("%s://%s", location.Scheme, location.Host)

	for _, s := range servers {
		// Interpret all operation paths as relative to the provided location
		if strings.HasPrefix(s.URL, "/") {
			return s.URL, nil
		}

		// localhost special casing?

		// Create a list with all possible parametrised server names
		endpoints := []string{s.URL}
		for k, v := range s.Variables.FromOldest() {
			key := fmt.Sprintf("{%s}", k)
			if len(v.Enum) == 0 {
				for i := range endpoints {
					endpoints[i] = strings.ReplaceAll(
						endpoints[i],
						key,
						v.Default,
					)
				}
			} else {
				nEndpoints := make([]string, len(v.Enum)*len(endpoints))
				for j := range v.Enum {
					val := v.Enum[j]
					for i := range endpoints {
						nEndpoints[i+j*len(endpoints)] = strings.ReplaceAll(
							endpoints[i],
							key,
							val,
						)
					}
				}
				endpoints = nEndpoints
			}
		}

		for i := range endpoints {
			if strings.HasPrefix(endpoints[i], prefix) {
				base, err := url.Parse(endpoints[i])
				if err != nil {
					return "", err
				}
				return strings.TrimSuffix(base.Path, "/"), nil
			}
		}
	}
	return location.Path, nil
}

func getRequestInfo(op *v3.Operation) (string, *base.Schema, []any) {
	mts := make(map[string][]any)

	if op.RequestBody != nil {
		for mt, item := range op.RequestBody.Content.FromOldest() {
			var examples []any

			if item.Example != nil {
				ex, err := decodeYAML(item.Example)
				if err != nil {
					log.Fatal(err)
				}
				examples = append(examples, ex)
			}
			if item.Examples != nil && item.Examples.Len() > 0 {
				keys := slices.Sorted(item.Examples.KeysFromOldest())
				for _, key := range keys {
					ex := item.Examples.GetOrZero(key)
					if ex != nil {
						exVal, err := decodeYAML(ex.Value)
						if err != nil {
							log.Fatal(err)
						}
						examples = append(examples, exVal)
					}
				}
			}

			var schema *base.Schema
			if item.Schema != nil && item.Schema.Schema() != nil {
				schema = item.Schema.Schema()
			}

			if schema != nil && len(examples) == 0 {
				examples = append(examples, GenExample(schema, modeWrite))
			}

			mts[mt] = []any{schema, examples}
		}
	}

	// Prefer JSON, fall back to YAML next, otherwise return the first one.
	for _, short := range []string{"json", "yaml", "*"} {
		for mt, item := range mts {
			if strings.Contains(mt, short) || short == "*" {
				return mt, item[0].(*base.Schema), item[1].([]any)
			}
		}
	}

	return "", nil, nil
}

// paramSchema returns a rendered schema line for a given parameter, falling
// back to the param type info if no schema is available.
func paramSchema(p *cli.Param, s *base.Schema) string {
	schemaDesc := fmt.Sprintf("(%s): %s", p.Type, p.Description)
	if s != nil {
		schemaDesc = renderSchema(s, "  ", modeWrite)
	}
	return schemaDesc
}

func openapiOperation(_ *cobra.Command, method string, uriTemplate *url.URL, path *v3.PathItem, op *v3.Operation) cli.Operation {
	var pathParams, queryParams, headerParams []*cli.Param
	var pathSchemas, querySchemas, headerSchemas []*base.Schema = []*base.Schema{}, []*base.Schema{}, []*base.Schema{}

	// Combine path and operation parameters, with operation params having
	// precedence when there are name conflicts.
	combinedParams := []*v3.Parameter{}
	seen := map[string]bool{}
	for _, p := range op.Parameters {
		combinedParams = append(combinedParams, p)
		seen[p.Name] = true
	}
	for _, p := range path.Parameters {
		if !seen[p.Name] {
			combinedParams = append(combinedParams, p)
		}
	}

	for _, p := range combinedParams {
		if getExtOr(p.Extensions, ExtIgnore, false) {
			continue
		}

		var def any
		var example any

		typ := "string"
		var schema *base.Schema
		if p.Schema != nil && p.Schema.Schema() != nil {
			s := p.Schema.Schema()
			schema = s
			if len(s.Type) > 0 {
				// TODO: support params of multiple types?
				typ = s.Type[0]
			}

			if typ == "array" {
				if s.Items != nil && s.Items.IsA() {
					items := s.Items.A.Schema()
					if len(items.Type) > 0 {
						typ += "[" + items.Type[0] + "]"
					}
				}
			}

			if s.Default != nil {
				newDef, err := decodeYAML(s.Default)
				if err != nil {
					log.Fatal(err)
				}
				def = newDef
			}
			if s.Example != nil {
				newExample, err := decodeYAML(s.Example)
				if err != nil {
					log.Fatal(err)
				}
				example = newExample
			}
		}

		if p.Example != nil {
			var newExample any
			if err := p.Example.Decode(&newExample); err != nil {
				log.Fatal(err)
			}
			example = newExample
		}

		style := cli.StyleSimple
		if p.Style == "form" {
			style = cli.StyleForm
		}

		displayName := getExtOr(p.Extensions, ExtName, "")
		description := getExtOr(p.Extensions, ExtDescription, p.Description)

		param := &cli.Param{
			Type:        typ,
			Name:        p.Name,
			DisplayName: displayName,
			Description: description,
			Style:       style,
			Default:     def,
			Example:     example,
		}

		if p.Explode != nil {
			param.Explode = *p.Explode
		}

		switch p.In {
		case "path":
			if pathParams == nil {
				pathParams = []*cli.Param{}
			}
			pathParams = append(pathParams, param)
			pathSchemas = append(pathSchemas, schema)
		case "query":
			if queryParams == nil {
				queryParams = []*cli.Param{}
			}
			queryParams = append(queryParams, param)
			querySchemas = append(querySchemas, schema)
		case "header":
			if headerParams == nil {
				headerParams = []*cli.Param{}
			}
			headerParams = append(headerParams, param)
			headerSchemas = append(headerSchemas, schema)
		}
	}

	aliases := getExtSlice(op.Extensions, ExtAliases, []string{})

	name := casing.Kebab(op.OperationId)
	if name == "" {
		name = casing.Kebab(method + "-" + strings.Trim(uriTemplate.Path, "/"))
	}
	if override := getExtOr(op.Extensions, ExtName, ""); override != "" {
		name = override
	} else if oldName := slug.Make(op.OperationId); oldName != "" && oldName != name {
		// For backward-compatibility, add the old naming scheme as an alias
		// if it is different. See https://github.com/rest-sh/restish/issues/29
		// for additional context; we prefer kebab casing for readability.
		aliases = append(aliases, oldName)
	}

	desc := getExtOr(op.Extensions, ExtDescription, op.Description)
	hidden := getExtOr(op.Extensions, ExtHidden, false)

	if len(pathParams) > 0 {
		desc += "\n## Argument Schema:\n```schema\n{\n"
		for i, p := range pathParams {
			desc += "  " + p.OptionName() + ": " + paramSchema(p, pathSchemas[i]) + "\n"
		}
		desc += "}\n```\n"
	}

	if len(queryParams) > 0 || len(headerParams) > 0 {
		desc += "\n## Option Schema:\n```schema\n{\n"
		for i, p := range queryParams {
			desc += "  --" + p.OptionName() + ": " + paramSchema(p, querySchemas[i]) + "\n"
		}
		for i, p := range headerParams {
			desc += "  --" + p.OptionName() + ": " + paramSchema(p, headerSchemas[i]) + "\n"
		}
		desc += "}\n```\n"
	}

	mediaType := ""
	var examples []string
	if op.RequestBody != nil {
		mt, reqSchema, reqExamples := getRequestInfo(op)
		mediaType = mt

		if len(reqExamples) > 0 {
			wroteHeader := false
			for _, ex := range reqExamples {
				var exContent string
				if exString, ok := ex.(string); ok {
					if exString == "<input.json" {
						continue
					}
					exContent = "\n```\n" + strings.Trim(exString, "\n") + "\n```\n"
				} else {
					// Not a string, so it's structured data. Let's marshal it to the
					// shorthand syntax if we can.
					if m, ok := ex.(map[string]any); ok {
						exs := shorthand.MarshalCLI(m)

						if len(exs) < 150 {
							examples = append(examples, exs)
						} else {
							found := false
							for _, e := range examples {
								if e == "<input.json" {
									found = true
									break
								}
							}
							if !found {
								examples = append(examples, "<input.json")
							}
						}
					}

					// Since we use `<` and `>` we need to disable HTML escaping.
					buffer := &bytes.Buffer{}
					encoder := json.NewEncoder(buffer)
					encoder.SetIndent("", "  ")
					encoder.SetEscapeHTML(false)
					encoder.Encode(ex)
					b := buffer.Bytes()

					exContent = "\n```json\n" + strings.Trim(string(b), "\n") + "\n```\n"
				}
				if !wroteHeader {
					desc += "\n## Input Example\n"
					wroteHeader = true
				}
				desc += exContent
			}
		}

		if reqSchema != nil {
			desc += "\n## Request Schema (" + mt + ")\n\n```schema\n" + renderSchema(reqSchema, "", modeWrite) + "\n```\n"
		}
	}

	codes := []string{}
	respMap := map[string]*v3.Response{}
	for k, v := range op.Responses.Codes.FromOldest() {
		codes = append(codes, k)
		respMap[k] = v
	}
	if op.Responses.Default != nil {
		codes = append(codes, "default")
		respMap["default"] = op.Responses.Default
	}
	sort.Strings(codes)

	type schemaEntry struct {
		code   string
		ct     string
		schema *base.Schema
	}
	schemaMap := map[[32]byte][]schemaEntry{}
	for _, code := range codes {
		var resp *v3.Response
		if respMap[code] == nil {
			continue
		}

		resp = respMap[code]

		hash := [32]byte{}
		if resp.Content != nil && resp.Content.Len() > 0 {
			for ct, typeInfo := range resp.Content.FromOldest() {
				var s *base.Schema
				hash = [32]byte{}
				if typeInfo.Schema != nil {
					s = typeInfo.Schema.Schema()
					hash = s.GoLow().Hash()
				}
				if schemaMap[hash] == nil {
					schemaMap[hash] = []schemaEntry{}
				}
				schemaMap[hash] = append(schemaMap[hash], schemaEntry{
					code:   code,
					ct:     ct,
					schema: s,
				})
			}
		} else {
			if schemaMap[hash] == nil {
				schemaMap[hash] = []schemaEntry{}
			}
			schemaMap[hash] = append(schemaMap[hash], schemaEntry{
				code: code,
			})
		}
	}

	schemaKeys := maps.Keys(schemaMap)
	sort.Slice(schemaKeys, func(i, j int) bool {
		return schemaMap[schemaKeys[i]][0].code < schemaMap[schemaKeys[j]][0].code
	})

	for _, s := range schemaKeys {
		entries := schemaMap[s]

		var resp *v3.Response
		if len(entries) == 1 && respMap[entries[0].code] != nil {
			resp = respMap[entries[0].code]
		}

		codeNums := []string{}
		for _, v := range entries {
			codeNums = append(codeNums, v.code)
		}

		hasSchema := s != [32]byte{}

		ct := ""
		if hasSchema {
			ct = " (" + entries[0].ct + ")"
		}

		if resp != nil {
			desc += "\n## Response " + entries[0].code + ct + "\n"
			respDesc := getExtOr(resp.Extensions, ExtDescription, resp.Description)
			if respDesc != "" {
				desc += "\n" + respDesc + "\n"
			} else if !hasSchema {
				desc += "\nResponse has no body\n"
			}
		} else {
			desc += "\n## Responses " + strings.Join(codeNums, "/") + ct + "\n"
			if !hasSchema {
				desc += "\nResponse has no body\n"
			}
		}

		headers := respMap[entries[0].code].Headers
		if headers != nil && headers.Len() > 0 {
			keys := slices.Sorted(headers.KeysFromOldest())
			desc += "\nHeaders: " + strings.Join(keys, ", ") + "\n"
		}

		if hasSchema {
			desc += "\n```schema\n" + renderSchema(entries[0].schema, "", modeRead) + "\n```\n"
		}
	}

	tmpl := uriTemplate.String()
	if s, err := url.PathUnescape(uriTemplate.String()); err == nil {
		tmpl = s
	}

	// Try to add a group: if there's more than 1 tag, we'll just pick the
	// first one as a best guess
	group := ""
	if len(op.Tags) > 0 {
		group = op.Tags[0]
	}

	dep := ""
	if op.Deprecated != nil && *op.Deprecated {
		dep = "do not use"
	}

	return cli.Operation{
		Name:          name,
		Group:         group,
		Aliases:       aliases,
		Short:         op.Summary,
		Long:          strings.Trim(desc, "\n") + "\n",
		Method:        method,
		URITemplate:   tmpl,
		PathParams:    pathParams,
		QueryParams:   queryParams,
		HeaderParams:  headerParams,
		BodyMediaType: mediaType,
		Examples:      examples,
		Hidden:        hidden,
		Deprecated:    dep,
	}
}

// cliExtensionNames contains all the x-cli-* extension names we look for
var cliExtensionNames = []string{
	ExtName,
	ExtAliases,
	ExtDescription,
	ExtIgnore,
	ExtHidden,
	ExtCLIConfig,
}

// collectCLIExtensions scans an OpenAPI document and collects all x-cli-* extensions found
func collectCLIExtensions(model *v3.Document) cli.CLIExtensions {
	var extensions []cli.CLIExtension

	// Helper to check and add extensions from an extension map
	addExtensions := func(extMap *orderedmap.Map[string, *yaml.Node], location string) {
		if extMap == nil {
			return
		}
		for _, extName := range cliExtensionNames {
			if val, err := getExt[any](extMap, extName); err == nil {
				extensions = append(extensions, cli.CLIExtension{
					Location: location,
					Name:     extName,
					Value:    val,
				})
			}
		}
	}

	// Check document-level extensions
	if model.Extensions != nil {
		addExtensions(model.Extensions, "document")
	}

	// Check info-level extensions
	if model.Info != nil && model.Info.Extensions != nil {
		addExtensions(model.Info.Extensions, "info")
	}

	// Check path and operation level extensions
	if model.Paths != nil {
		for pathURI, pathItem := range model.Paths.PathItems.FromOldest() {
			if pathItem.Extensions != nil {
				addExtensions(pathItem.Extensions, "path:"+pathURI)
			}

			// Check each operation
			for method, operation := range pathItem.GetOperations().FromOldest() {
				if operation == nil {
					continue
				}
				opLocation := "operation:" + method + ":" + pathURI

				if operation.Extensions != nil {
					addExtensions(operation.Extensions, opLocation)
				}

				// Check parameters
				for _, param := range operation.Parameters {
					if param.Extensions != nil {
						addExtensions(param.Extensions, opLocation+":param:"+param.Name)
					}
				}

				// Check responses
				if operation.Responses != nil {
					for code, resp := range operation.Responses.Codes.FromOldest() {
						if resp != nil && resp.Extensions != nil {
							addExtensions(resp.Extensions, opLocation+":response:"+code)
						}
					}
					if operation.Responses.Default != nil && operation.Responses.Default.Extensions != nil {
						addExtensions(operation.Responses.Default.Extensions, opLocation+":response:default")
					}
				}
			}

			// Check path-level parameters
			for _, param := range pathItem.Parameters {
				if param.Extensions != nil {
					addExtensions(param.Extensions, "path:"+pathURI+":param:"+param.Name)
				}
			}
		}
	}

	return cli.CLIExtensions{Extensions: extensions}
}

func loadOpenAPI3(cfg Resolver, cmd *cobra.Command, location *url.URL, resp *http.Response) (cli.API, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return cli.API{}, err
	}

	config := datamodel.NewDocumentConfiguration()
	schemeLower := strings.ToLower(location.Scheme)
	if schemeLower == "http" || schemeLower == "https" {
		// Set the base URL to resolve relative references.
		config.BaseURL = &url.URL{Scheme: location.Scheme, Host: location.Host, Path: path.Dir(location.Path)}
	} else {
		// Set the base local directory path to resolve relative references.
		config.BasePath = path.Dir(location.Path)
	}
	config.IgnorePolymorphicCircularReferences = true
	config.IgnoreArrayCircularReferences = true

	doc, err := libopenapi.NewDocumentWithConfiguration(data, config)
	if err != nil {
		return cli.API{}, err
	}

	var model v3.Document
	switch doc.GetSpecInfo().SpecType {
	case utils.OpenApi3:
		result, errs := doc.BuildV3Model()
		if len(errs) > 0 {
			return cli.API{}, fmt.Errorf("failed to load the OpenAPI document: %w", errors.Join(errs...))
		}

		model = result.Model
	default:
		return cli.API{}, fmt.Errorf("unsupported OpenAPI document")
	}

	// See if this server has any base path prefix we need to account for.
	basePath, err := getBasePath(cfg.GetBase(), model.Servers)
	if err != nil {
		return cli.API{}, err
	}

	operations := []cli.Operation{}
	if model.Paths != nil {
		for uri, pathItem := range model.Paths.PathItems.FromOldest() {
			if getExtOr(pathItem.Extensions, ExtIgnore, false) {
				continue
			}

			resolved, err := cfg.Resolve(strings.TrimSuffix(basePath, "/") + uri)
			if err != nil {
				return cli.API{}, err
			}

			for method, operation := range pathItem.GetOperations().FromOldest() {
				if operation == nil || getExtOr(operation.Extensions, ExtIgnore, false) {
					continue
				}

				operations = append(operations, openapiOperation(cmd, strings.ToUpper(method), resolved, pathItem, operation))
			}
		}
	}

	authSchemes := []cli.APIAuth{}
	if model.Components != nil && model.Components.SecuritySchemes != nil {
		keys := slices.Sorted(model.Components.SecuritySchemes.KeysFromOldest())

		for _, key := range keys {
			scheme := model.Components.SecuritySchemes.Value(key)
			switch scheme.Type {
			case "apiKey":
				// TODO: api key auth
			case "http":
				if scheme.Scheme == "basic" {
					authSchemes = append(authSchemes, cli.APIAuth{
						Name: "http-basic",
						Params: map[string]string{
							"username": "",
							"password": "",
						},
					})
				}
				// TODO: bearer
			case "oauth2":
				flows := scheme.Flows
				if flows != nil {
					if flows.ClientCredentials != nil {
						cc := flows.ClientCredentials
						authSchemes = append(authSchemes, cli.APIAuth{
							Name: "oauth-client-credentials",
							Params: map[string]string{
								"client_id":     "",
								"client_secret": "",
								"token_url":     cc.TokenUrl,
								// TODO: scopes
							},
						})
					}

					if flows.AuthorizationCode != nil {
						ac := flows.AuthorizationCode
						authSchemes = append(authSchemes, cli.APIAuth{
							Name: "oauth-authorization-code",
							Params: map[string]string{
								"client_id":     "",
								"authorize_url": ac.AuthorizationUrl,
								"token_url":     ac.TokenUrl,
								// TODO: scopes
							},
						})
					}
				}
			}
		}
	}

	short := ""
	long := ""
	if model.Info != nil {
		short = getExtOr(model.Info.Extensions, ExtName, model.Info.Title)
		long = getExtOr(model.Info.Extensions, ExtDescription, model.Info.Description)
	}

	// Collect all CLI extensions found in the document
	cliExtensions := collectCLIExtensions(&model)

	api := cli.API{
		Short:         short,
		Long:          long,
		Operations:    operations,
		CLIExtensions: cliExtensions,
	}

	if len(authSchemes) > 0 {
		api.Auth = authSchemes
	}

	loadAutoConfig(&api, &model)

	return api, nil
}

func loadAutoConfig(api *cli.API, model *v3.Document) {
	var config *autoConfig

	if model.Extensions == nil {
		return
	}

	cfg := model.Extensions.Value(ExtCLIConfig)
	if cfg == nil {
		return
	}

	low := model.GoLow()
	for k, v := range low.Extensions.FromOldest() {
		if k.Value == ExtCLIConfig {
			if err := v.ValueNode.Decode(&config); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to unmarshal x-cli-config: %v", err)
				return
			}
			break
		}
	}

	authName := config.Security
	params := map[string]string{}

	if model.Components.SecuritySchemes != nil {
		scheme := model.Components.SecuritySchemes.Value(config.Security)

		// Convert it to the Restish security type and set some default params.
		switch scheme.Type {
		case "http":
			if scheme.Scheme == "basic" {
				authName = "http-basic"
			}
		case "oauth2":
			if scheme.Flows != nil {
				if scheme.Flows.AuthorizationCode != nil {
					// Prefer auth code if multiple auth types are available.
					authName = "oauth-authorization-code"
					ac := scheme.Flows.AuthorizationCode
					params["client_id"] = ""
					params["authorize_url"] = ac.AuthorizationUrl
					params["token_url"] = ac.TokenUrl
				} else if scheme.Flows.ClientCredentials != nil {
					authName = "oauth-client-credentials"
					cc := scheme.Flows.ClientCredentials
					params["client_id"] = ""
					params["client_secret"] = ""
					params["token_url"] = cc.TokenUrl
				}
			}
		}
	}

	// Params can override the values above if needed.
	for k, v := range config.Params {
		params[k] = v
	}

	api.AutoConfig = cli.AutoConfig{
		Headers: config.Headers,
		Prompt:  config.Prompt,
		Auth: cli.APIAuth{
			Name:   authName,
			Params: params,
		},
	}
}

type loader struct {
	location *url.URL
	base     *url.URL
}

func (l *loader) GetBase() *url.URL {
	return l.base
}

func (l *loader) Resolve(relURI string) (*url.URL, error) {
	parsed, err := url.Parse(relURI)
	if err != nil {
		return nil, err
	}

	return l.base.ResolveReference(parsed), nil
}

func (l *loader) LocationHints() []string {
	return []string{"/openapi.json", "/openapi.yaml", "openapi.json", "openapi.yaml"}
}

func (l *loader) Detect(resp *http.Response) bool {
	// Try to detect via header first
	if strings.HasPrefix(resp.Header.Get("content-type"), "application/vnd.oai.openapi") {
		return true
	}

	// Fall back to looking for the OpenAPI version in the body.
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return reOpenAPI3.Match(body)
}

func (l *loader) Load(entrypoint, spec url.URL, resp *http.Response) (cli.API, error) {
	l.location = &spec
	l.base = &entrypoint
	return loadOpenAPI3(l, cli.Root, &spec, resp)
}

// New creates a new OpenAPI loader.
func New() cli.Loader {
	return &loader{}
}
