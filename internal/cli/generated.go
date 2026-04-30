package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/spf13/cobra"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

// buildAPICommand constructs a Cobra command group for a registered API and
// populates it with one subcommand per OpenAPI operation found in s.
// Returns nil when the spec cannot be built into a v3 model.
func (c *CLI) buildAPICommand(apiName string, apiCfg *config.APIConfig, s *spec.APISpec) *cobra.Command {
	set, err := s.OperationSetWithOptions(spec.OperationOptions{
		BaseURL:         apiCfg.BaseURL,
		OperationBase:   apiCfg.OperationBase,
		ServerVariables: effectiveServerVariables(apiCfg, "default"),
	})
	return c.buildAPICommandFromOperationResult(apiName, apiCfg, set, err)
}

func (c *CLI) buildAPICommandFromOperationResult(apiName string, apiCfg *config.APIConfig, set spec.OperationSet, err error) *cobra.Command {
	if err != nil {
		source := apiCfg.SpecURL
		if source == "" && len(apiCfg.SpecFiles) > 0 {
			source = apiCfg.SpecFiles[0]
		}
		if source == "" {
			source = apiCfg.BaseURL
		}
		c.warnf("skipping generated commands for API %q from %s: %v", apiName, source, err)
		return nil
	}
	return c.buildAPICommandFromOperationSet(apiName, apiCfg, set)
}

func (c *CLI) buildAPICommandFromOperations(apiName string, apiCfg *config.APIConfig, ops []spec.Operation) *cobra.Command {
	return c.buildAPICommandFromOperationSet(apiName, apiCfg, spec.OperationSet{Operations: ops})
}

func (c *CLI) buildAPICommandFromOperationSet(apiName string, apiCfg *config.APIConfig, set spec.OperationSet) *cobra.Command {
	ops := set.Operations
	// ops == nil means no V3 model or no paths section — nothing to generate.
	if ops == nil {
		return nil
	}

	long := strings.TrimSpace(set.Info.Description)
	if long == "" {
		long = strings.TrimSpace(set.Info.Summary)
	}

	apiCmd := &cobra.Command{
		Use:     apiName,
		Short:   fmt.Sprintf("Commands generated from the %s API spec", apiName),
		Long:    long,
		GroupID: rootGroupAPI,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q; run %q to see generated operations", args[0], cmd.Name(), cmd.CommandPath()+" --help")
			}
			return cmd.Help()
		},
	}

	tagsSeen := map[string]bool{}
	commandNames := map[string]bool{}
	tagCommands := map[string]*cobra.Command{}
	tagCommandOrder := []string{}
	tagCommandNameByTag := map[string]string{}
	commandNamesByTag := map[string]map[string]bool{}
	useTagLayout := apiCfg.CommandLayout == "tags"

	for _, op := range ops {
		tagCommandName := ""
		examplePrefix := apiName
		if useTagLayout && len(op.Tags) > 0 {
			tagCommandName = generatedTagCommandName(op.Tags[0], tagCommandNameByTag, tagCommands)
			examplePrefix = apiName + " " + tagCommandName
		}
		cmd, err := c.buildOperationCommand(apiName, examplePrefix, op, apiCfg.BaseURL, apiCfg.OperationBase)
		if err != nil {
			c.warnf("skipping %s %s for API %q: %v", op.Method, op.Path, apiName, err)
			continue
		}
		if cmd == nil {
			continue
		}
		collisionScope := commandNames
		if useTagLayout && tagCommandName != "" {
			if tagCommands[tagCommandName] == nil {
				tagCommands[tagCommandName] = newGeneratedTagCommand(tagCommandName, op.Tags[0])
				tagCommandOrder = append(tagCommandOrder, tagCommandName)
				commandNamesByTag[tagCommandName] = map[string]bool{}
			}
			collisionScope = commandNamesByTag[tagCommandName]
		}
		if collisionScope[cmd.Name()] {
			originalName := cmd.Name()
			disambiguated := originalName + "-" + strings.ToLower(op.Method)
			if collisionScope[disambiguated] {
				suffix := 2
				for collisionScope[fmt.Sprintf("%s-%d", disambiguated, suffix)] {
					suffix++
				}
				disambiguated = fmt.Sprintf("%s-%d", disambiguated, suffix)
			}
			c.warnf("command name collision for API %q: %q; using %q for %s %s", apiName, originalName, disambiguated, op.Method, op.Path)
			cmd.Use = strings.Replace(cmd.Use, originalName, disambiguated, 1)
			cmd.Aliases = append(cmd.Aliases, originalName)
		}
		collisionScope[cmd.Name()] = true
		cmd.Aliases = c.filterGeneratedAliases(apiName, cmd, collisionScope)
		if useTagLayout && tagCommandName != "" {
			tagCommands[tagCommandName].AddCommand(cmd)
			continue
		}
		commandNames[cmd.Name()] = true
		if !useTagLayout && len(op.Tags) > 0 {
			tag := op.Tags[0]
			if !tagsSeen[tag] {
				tagsSeen[tag] = true
				apiCmd.AddGroup(&cobra.Group{ID: tag, Title: tag})
			}
			cmd.GroupID = tag
		}
		apiCmd.AddCommand(cmd)
	}
	if useTagLayout {
		for _, name := range tagCommandOrder {
			apiCmd.AddCommand(tagCommands[name])
		}
	}

	return apiCmd
}

func (c *CLI) filterGeneratedAliases(apiName string, cmd *cobra.Command, collisionScope map[string]bool) []string {
	if len(cmd.Aliases) == 0 {
		return nil
	}
	aliases := make([]string, 0, len(cmd.Aliases))
	for _, alias := range cmd.Aliases {
		if alias == "" {
			continue
		}
		if collisionScope[alias] {
			c.warnf("command alias collision for API %q: dropping alias %q on %q", apiName, alias, cmd.Name())
			continue
		}
		collisionScope[alias] = true
		aliases = append(aliases, alias)
	}
	return aliases
}

func generatedTagCommandName(tag string, namesByTag map[string]string, existing map[string]*cobra.Command) string {
	if name := namesByTag[tag]; name != "" {
		return name
	}
	base := toKebabCase(tag)
	if base == "" {
		base = "operations"
	}
	name := base
	for suffix := 2; existing[name] != nil; suffix++ {
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
	namesByTag[tag] = name
	return name
}

func newGeneratedTagCommand(name, tag string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Operations tagged %s", tag),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q; run %q to see tagged operations", args[0], cmd.Name(), cmd.CommandPath()+" --help")
			}
			return cmd.Help()
		},
	}
	return cmd
}

// paramInfo holds the information we need about a single parameter.
type paramInfo struct {
	name             string // original API parameter name
	flagName         string // kebab-case flag name
	in               string // "path", "query", "header", "cookie"
	required         bool
	hidden           bool
	desc             string
	schema           string
	typ              string
	itemType         string
	defaultValue     string
	hasDefault       bool
	style            string
	explode          *bool
	allowReserved    bool
	contentMediaType string
	enum             []string // allowed values from OpenAPI schema enum, if present
}

// buildOperationCommand creates a Cobra command for one OpenAPI operation.
// Returns nil when the operation is excluded via x-cli-ignore.
// operationBase, when non-empty, is resolved against baseURL and replaces the
// apiName short-name prefix in generated URLs.
func (c *CLI) buildOperationCommand(apiName, examplePrefix string, op spec.Operation, baseURL, operationBase string) (*cobra.Command, error) {
	// Derive command name from operationId, with x-cli-name override.
	cmdName := toKebabCase(op.ID)
	if cmdName == "" {
		cmdName = fallbackOperationName(op.Method, op.Path)
	}
	if op.XCLI.Name != "" {
		cmdName = op.XCLI.Name
	}

	// Build param lists, honouring per-parameter x-cli-name / x-cli-description.
	pathParamOrder := extractPathParamNames(op.Path)
	seenPathParams := map[string]bool{}
	for _, name := range pathParamOrder {
		if seenPathParams[name] {
			opID := op.ID
			if opID == "" {
				opID = op.Method + " " + op.Path
			}
			return nil, fmt.Errorf("operation %q repeats path parameter %q", opID, name)
		}
		seenPathParams[name] = true
	}

	allParams := make(map[string]*paramInfo)
	for _, p := range op.Parameters {
		if p.XCLI.Ignore {
			continue
		}
		flagName := toKebabCase(p.Name)
		if p.XCLI.Name != "" {
			flagName = p.XCLI.Name
		}
		desc := p.Desc
		if p.XCLI.Description != "" {
			desc = p.XCLI.Description
		}
		desc = appendGeneratedParamSupportNote(desc, p)
		allParams[paramKey(p.In, p.Name)] = &paramInfo{
			name:             p.Name,
			flagName:         flagName,
			in:               p.In,
			required:         p.Required,
			hidden:           p.XCLI.Hidden,
			desc:             desc,
			schema:           p.Schema,
			typ:              p.Type,
			itemType:         p.ItemType,
			defaultValue:     p.Default,
			hasDefault:       p.HasDefault,
			style:            p.Style,
			explode:          p.Explode,
			allowReserved:    p.AllowReserved,
			contentMediaType: p.ContentMediaType,
			enum:             p.Enum,
		}
	}

	// Required positional args are path params in path-template order, followed
	// by required non-path params in spec order. Optional params become flags.
	var required []*paramInfo
	for _, name := range pathParamOrder {
		pi, ok := allParams[paramKey("path", name)]
		if !ok {
			opID := op.ID
			if opID == "" {
				opID = op.Method + " " + op.Path
			}
			return nil, fmt.Errorf("operation %q references missing path parameter %q", opID, name)
		}
		required = append(required, pi)
	}
	var optional []*paramInfo
	for _, p := range op.Parameters {
		if p.In == "path" {
			continue
		}
		pi := allParams[paramKey(p.In, p.Name)]
		if pi == nil {
			continue
		}
		if p.Required {
			required = append(required, pi)
			continue
		}
		optional = append(optional, pi)
	}

	// Build Use string.
	use := cmdName
	for _, p := range required {
		use += " <" + p.flagName + ">"
	}
	if op.HasBody {
		use += " [body...]"
	}

	// Short and long descriptions, with x-cli-description override.
	short := op.Summary
	if op.XCLI.Description != "" {
		short = op.XCLI.Description
	}
	if short == "" {
		short = fmt.Sprintf("%s %s", op.Method, op.Path)
	}

	long := op.Description
	if op.XCLI.Description != "" {
		long = op.XCLI.Description
	}
	if len(required) > 0 {
		var argDocs strings.Builder
		if long != "" {
			argDocs.WriteString("\n\n")
		}
		argDocs.WriteString("Arguments:\n")
		for _, p := range required {
			if p.desc != "" {
				argDocs.WriteString(fmt.Sprintf("  %-20s %s\n", p.flagName, p.desc))
			}
		}
		long += argDocs.String()
	}
	long = appendGeneratedOperationHelp(long, required, optional, op.Help)

	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: generatedOperationExamples(examplePrefix, use, op.Help.Examples),
		Aliases: op.XCLI.Aliases,
		Args: func(cmd *cobra.Command, args []string) error {
			if generateBody, _ := cmd.Flags().GetBool("rsh-generate-body"); generateBody {
				return nil
			}
			return cobra.MinimumNArgs(len(required))(cmd, args)
		},
		Hidden:     op.XCLI.Hidden,
		Deprecated: deprecatedNotice(op.Deprecated),
		RunE: func(cmd *cobra.Command, args []string) error {
			if helpAll, _ := cmd.Flags().GetBool("help-all"); helpAll {
				return showGeneratedOperationHelpAll(cmd)
			}
			if generateBody, _ := cmd.Flags().GetBool("rsh-generate-body"); generateBody {
				return c.printGeneratedBodyExample(op.Help.Request)
			}
			return c.runGeneratedOp(cmd, apiName, op.Path, op.Method, op.RequestMediaType, op.RequestSchemaTypes, op.RequestMultipartContentTypes, op.NoAuth, op.OptionalAuth, op.CredentialAlternatives, required, optional, args, baseURL, operationBase)
		},
	}
	if candidates := authOverrideCandidates(op.OptionalAuth, op.CredentialAlternatives); len(candidates) > 0 {
		cmd.Annotations = map[string]string{
			securityCompletionAnnotation: strings.Join(candidates, "\n"),
		}
	}
	cmd.Flags().Bool("help-all", false, "Show all inherited Restish flags in help")
	cmd.SetUsageTemplate(generatedOperationUsageTemplate)
	if !op.HasBody {
		cmd.Args = generatedOperationArgs(required, false)
	} else {
		cmd.Args = generatedOperationArgs(required, true)
		cmd.Flags().Bool("rsh-generate-body", false, "Print an example request body and exit")
	}

	for _, p := range optional {
		switch p.typ {
		case "boolean":
			def, _ := strconv.ParseBool(p.defaultValue)
			cmd.Flags().Bool(p.flagName, def, p.desc)
		case "integer":
			def, _ := strconv.Atoi(p.defaultValue)
			cmd.Flags().Int(p.flagName, def, p.desc)
		case "number":
			def, _ := strconv.ParseFloat(p.defaultValue, 64)
			cmd.Flags().Float64(p.flagName, def, p.desc)
		case "array":
			var def []string
			if p.hasDefault && p.defaultValue != "" {
				def = strings.Split(p.defaultValue, ",")
			}
			cmd.Flags().StringArray(p.flagName, def, p.desc)
		default:
			cmd.Flags().String(p.flagName, p.defaultValue, p.desc)
		}
		if p.hidden {
			_ = cmd.Flags().MarkHidden(p.flagName)
		}
		if len(p.enum) > 0 {
			vals := p.enum
			_ = cmd.RegisterFlagCompletionFunc(p.flagName, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
				return vals, cobra.ShellCompDirectiveNoFileComp
			})
		}
	}

	// Register valid args for required positional params that have enum values.
	// Cobra uses ValidArgsFunction for the first positional arg completion.
	if len(required) > 0 {
		reqWithEnum := required // capture
		cmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			idx := len(args)
			if idx < len(reqWithEnum) && len(reqWithEnum[idx].enum) > 0 {
				return reqWithEnum[idx].enum, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return cmd, nil
}

func paramKey(in, name string) string {
	return in + "\x00" + name
}

func appendGeneratedParamSupportNote(desc string, p spec.Param) string {
	var note string
	if p.ContentMediaType != "" && !isJSONMediaType(p.ContentMediaType) {
		note = fmt.Sprintf("parameter content type %s is sent as raw text", p.ContentMediaType)
	} else if p.Style != "" && !supportedGeneratedParamStyle(p.In, p.Style) {
		note = fmt.Sprintf("OpenAPI style %q is not fully supported; using default serialization", p.Style)
	}
	if note == "" {
		return desc
	}
	if desc == "" {
		return note
	}
	return desc + " (" + note + ")"
}

func supportedGeneratedParamStyle(in, style string) bool {
	switch in {
	case "path":
		return style == "simple" || style == "label" || style == "matrix"
	case "query":
		return style == "form" || style == "spaceDelimited" || style == "pipeDelimited" || style == "deepObject"
	case "header":
		return style == "simple"
	case "cookie":
		return style == "form"
	default:
		return true
	}
}

func generatedOperationArgs(required []*paramInfo, hasBody bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if helpAll, _ := cmd.Flags().GetBool("help-all"); helpAll {
			return nil
		}
		if generateBody, _ := cmd.Flags().GetBool("rsh-generate-body"); generateBody {
			return nil
		}
		requiredCount := len(required)
		if len(args) < requiredCount {
			missing := make([]string, 0, requiredCount-len(args))
			for _, p := range required[len(args):] {
				missing = append(missing, p.flagName)
			}
			return fmt.Errorf("missing required argument(s): %s; run %q for usage", strings.Join(missing, ", "), cmd.CommandPath()+" --help")
		}
		if hasBody {
			return nil
		}
		if len(args) > requiredCount {
			return fmt.Errorf("too many arguments: expected %d, got %d; run %q for usage", requiredCount, len(args), cmd.CommandPath()+" --help")
		}
		return nil
	}
}

func showGeneratedOperationHelpAll(cmd *cobra.Command) error {
	orig := cmd.UsageTemplate()
	cmd.SetUsageTemplate(groupedUsageTemplate)
	err := cmd.Help()
	cmd.SetUsageTemplate(orig)
	return err
}

func (c *CLI) printGeneratedBodyExample(request *spec.OperationBodyHelp) error {
	if request != nil && strings.TrimSpace(request.Example) != "" {
		fmt.Fprintln(c.Stdout, request.Example)
		return nil
	}
	fmt.Fprintln(c.Stdout, "{}")
	return nil
}

func appendGeneratedOperationHelp(long string, required, optional []*paramInfo, help spec.OperationHelp) string {
	var b strings.Builder
	if strings.TrimSpace(long) != "" {
		b.WriteString(strings.TrimRight(long, "\n"))
	}
	appendSection := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(title)
		b.WriteString("\n")
	}

	if hasParamSchemas(required) {
		appendSection("Argument Schema:")
		b.WriteString("```schema\n{\n")
		for _, p := range required {
			if p.schema == "" {
				continue
			}
			b.WriteString("  " + p.flagName + ": " + indentSchemaContinuation(p.schema, "  ") + "\n")
		}
		b.WriteString("}\n```")
	}

	if hasParamSchemas(optional) {
		appendSection("Option Schema:")
		b.WriteString("```schema\n{\n")
		for _, p := range optional {
			if p.schema == "" || p.hidden {
				continue
			}
			b.WriteString("  --" + p.flagName + ": " + indentSchemaContinuation(p.schema, "  ") + "\n")
		}
		b.WriteString("}\n```")
	}

	if help.Request != nil {
		if help.Request.Example != "" {
			appendSection("Input Example:")
			b.WriteString("```json\n")
			b.WriteString(help.Request.Example)
			b.WriteString("\n```")
		}
		if help.Request.Schema != "" {
			title := "Request Schema"
			if help.Request.MediaType != "" {
				title += " (" + help.Request.MediaType + ")"
			}
			appendSection(title + ":")
			b.WriteString("```schema\n")
			b.WriteString(help.Request.Schema)
			b.WriteString("\n```")
		}
	}

	for _, resp := range help.Responses {
		if len(resp.Codes) == 0 {
			continue
		}
		prefix := "Response"
		if len(resp.Codes) > 1 {
			prefix = "Responses"
		}
		title := prefix + " " + strings.Join(resp.Codes, "/")
		if resp.MediaType != "" {
			title += " (" + resp.MediaType + ")"
		}
		appendSection(title + ":")
		if resp.Description != "" && len(resp.Codes) == 1 {
			b.WriteString(resp.Description)
			b.WriteString("\n\n")
		}
		if len(resp.Headers) > 0 {
			b.WriteString("Headers: ")
			b.WriteString(strings.Join(resp.Headers, ", "))
			b.WriteString("\n\n")
		}
		if resp.NoBody && resp.Schema == "" {
			b.WriteString("Response has no body")
			continue
		}
		if resp.Example != "" {
			b.WriteString("```json\n")
			b.WriteString(resp.Example)
			b.WriteString("\n```\n\n")
		}
		if resp.Schema != "" {
			b.WriteString("```schema\n")
			b.WriteString(resp.Schema)
			b.WriteString("\n```")
		}
	}

	return b.String()
}

func hasParamSchemas(params []*paramInfo) bool {
	for _, p := range params {
		if p.schema != "" && !p.hidden {
			return true
		}
	}
	return false
}

func indentSchemaContinuation(schema, indent string) string {
	lines := strings.Split(schema, "\n")
	if len(lines) <= 1 {
		return schema
	}
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func generatedOperationExamples(apiName, use string, examples []string) string {
	if len(examples) == 0 {
		return ""
	}
	use = strings.TrimSuffix(use, " [body...]")
	var b strings.Builder
	for _, ex := range examples {
		b.WriteString("  restish ")
		b.WriteString(apiName)
		b.WriteString(" ")
		b.WriteString(use)
		if ex != "" {
			b.WriteString(" ")
			b.WriteString(ex)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// runGeneratedOp is the RunE handler for generated operation commands.
func (c *CLI) runGeneratedOp(
	cmd *cobra.Command,
	apiName, opPath, method, requestMediaType string,
	requestSchemaTypes map[string]string,
	requestMultipartContentTypes map[string]string,
	noAuth bool,
	optionalAuth bool,
	credentialAlternatives []spec.CredentialAlternative,
	required, optional []*paramInfo,
	args []string,
	baseURL string,
	operationBase string,
) error {
	// Substitute required params into the path, query string, and headers.
	path := opPath
	var query []generatedQueryParam
	var extraHeaders []string
	bodyArgStart := len(required)

	for i, p := range required {
		val := args[i]
		var err error
		path, query, extraHeaders, err = addGeneratedParam(path, query, extraHeaders, p, []string{val})
		if err != nil {
			return err
		}
	}

	// Collect optional param flags.
	for _, p := range optional {
		if !cmd.Flags().Changed(p.flagName) && !p.hasDefault {
			continue
		}
		values, err := generatedFlagValues(cmd, p)
		if err != nil {
			return err
		}
		path, query, extraHeaders, err = addGeneratedParam(path, query, extraHeaders, p, values)
		if err != nil {
			return err
		}
	}

	// Build the raw URL. When operation_base is set, resolve its absolute path
	// against base_url using v1 semantics so generated operations can escape a
	// base URL sub-path.
	var rawURL string
	if operationBase != "" {
		resolvedBase, err := config.ResolveOperationBaseURL(baseURL, operationBase)
		if err != nil {
			return fmt.Errorf("operation_base: %w", err)
		}
		rawURL = strings.TrimRight(resolvedBase, "/") + path
	} else {
		rawURL = apiName + path
	}
	if qs := encodeGeneratedQuery(query); qs != "" {
		rawURL += "?" + qs
	}

	bodyArgs := args[bodyArgStart:]
	gf := globalFlagsFromContext(requestContext(cmd))
	return c.runHTTPInternalWithBodyOptions(cmd, method, append([]string{rawURL}, bodyArgs...), false, extraHeaders, noAuth, "", requestMediaType, requestBodyOptions{
		schemaTypes:               requestSchemaTypes,
		multipartPartContentTypes: requestMultipartContentTypes,
		operationAuth: &operationAuthPolicy{
			OptionalAuth:           optionalAuth,
			CredentialAlternatives: credentialAlternatives,
			Override:               gf.Auth,
		},
	})
}

func generatedFlagValues(cmd *cobra.Command, p *paramInfo) ([]string, error) {
	switch p.typ {
	case "boolean":
		v, err := cmd.Flags().GetBool(p.flagName)
		if err != nil {
			return nil, err
		}
		return []string{strconv.FormatBool(v)}, nil
	case "integer":
		v, err := cmd.Flags().GetInt(p.flagName)
		if err != nil {
			return nil, err
		}
		return []string{strconv.Itoa(v)}, nil
	case "number":
		v, err := cmd.Flags().GetFloat64(p.flagName)
		if err != nil {
			return nil, err
		}
		return []string{strconv.FormatFloat(v, 'f', -1, 64)}, nil
	case "array":
		values, err := cmd.Flags().GetStringArray(p.flagName)
		if err != nil {
			return nil, err
		}
		if len(values) == 0 && p.hasDefault && p.defaultValue != "" {
			values = strings.Split(p.defaultValue, ",")
		}
		return values, nil
	default:
		v, err := cmd.Flags().GetString(p.flagName)
		if err != nil {
			return nil, err
		}
		if v == "" && !p.hasDefault {
			return nil, nil
		}
		return []string{v}, nil
	}
}

type generatedQueryParam struct {
	name          string
	value         string
	allowReserved bool
}

func addGeneratedParam(path string, q []generatedQueryParam, extraHeaders []string, p *paramInfo, values []string) (string, []generatedQueryParam, []string, error) {
	if len(values) == 0 {
		return path, q, extraHeaders, nil
	}
	switch p.in {
	case "path":
		serialized, err := serializeGeneratedPathParam(p, values)
		if err != nil {
			return path, q, extraHeaders, err
		}
		path = strings.ReplaceAll(path, "{"+p.name+"}", serialized)
	case "query":
		parts, err := serializeGeneratedQueryParam(p, values)
		if err != nil {
			return path, q, extraHeaders, err
		}
		q = append(q, parts...)
	case "header":
		headerValues, err := serializeGeneratedHeaderParam(p, values)
		if err != nil {
			return path, q, extraHeaders, err
		}
		for _, value := range headerValues {
			extraHeaders = append(extraHeaders, p.name+": "+value)
		}
	case "cookie":
		cookies, err := serializeGeneratedCookieParam(p, values)
		if err != nil {
			return path, q, extraHeaders, err
		}
		if len(cookies) > 0 {
			extraHeaders = append(extraHeaders, "Cookie: "+strings.Join(cookies, "; "))
		}
	}
	return path, q, extraHeaders, nil
}

func serializeGeneratedPathParam(p *paramInfo, values []string) (string, error) {
	if p.contentMediaType != "" {
		encoded, err := serializeGeneratedContentParam(p, values)
		if err != nil {
			return "", err
		}
		return url.PathEscape(encoded), nil
	}
	style := defaultParamStyle(p)
	explode := paramExplode(p)
	switch style {
	case "label":
		return "." + pathDelimitedParamValue(p, values, ".", explode), nil
	case "matrix":
		switch {
		case p.typ == "array" && explode:
			parts := normalizeGeneratedArrayValues(values)
			var b strings.Builder
			for _, value := range parts {
				b.WriteString(";")
				b.WriteString(url.PathEscape(p.name))
				b.WriteString("=")
				b.WriteString(url.PathEscape(value))
			}
			return b.String(), nil
		case p.typ == "object" && explode:
			fields, err := generatedObjectFields(values)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, field := range fields {
				b.WriteString(";")
				b.WriteString(url.PathEscape(field.key))
				b.WriteString("=")
				b.WriteString(url.PathEscape(field.value))
			}
			return b.String(), nil
		default:
			return ";" + url.PathEscape(p.name) + "=" + pathDelimitedParamValue(p, values, ",", false), nil
		}
	default:
		return pathDelimitedParamValue(p, values, ",", explode), nil
	}
}

func serializeGeneratedQueryParam(p *paramInfo, values []string) ([]generatedQueryParam, error) {
	if p.contentMediaType != "" {
		encoded, err := serializeGeneratedContentParam(p, values)
		if err != nil {
			return nil, err
		}
		return []generatedQueryParam{{name: p.name, value: encoded, allowReserved: p.allowReserved}}, nil
	}
	style := defaultParamStyle(p)
	explode := paramExplode(p)
	switch {
	case p.typ == "array":
		parts := normalizeGeneratedArrayValues(values)
		switch style {
		case "spaceDelimited":
			return []generatedQueryParam{{name: p.name, value: strings.Join(parts, " "), allowReserved: p.allowReserved}}, nil
		case "pipeDelimited":
			return []generatedQueryParam{{name: p.name, value: strings.Join(parts, "|"), allowReserved: p.allowReserved}}, nil
		default:
			if explode {
				out := make([]generatedQueryParam, 0, len(parts))
				for _, part := range parts {
					out = append(out, generatedQueryParam{name: p.name, value: part, allowReserved: p.allowReserved})
				}
				return out, nil
			}
			return []generatedQueryParam{{name: p.name, value: strings.Join(parts, ","), allowReserved: p.allowReserved}}, nil
		}
	case p.typ == "object":
		fields, err := generatedObjectFields(values)
		if err != nil {
			return nil, err
		}
		switch {
		case style == "deepObject":
			out := make([]generatedQueryParam, 0, len(fields))
			for _, field := range fields {
				out = append(out, generatedQueryParam{name: p.name + "[" + field.key + "]", value: field.value, allowReserved: p.allowReserved})
			}
			return out, nil
		case explode:
			out := make([]generatedQueryParam, 0, len(fields))
			for _, field := range fields {
				out = append(out, generatedQueryParam{name: field.key, value: field.value, allowReserved: p.allowReserved})
			}
			return out, nil
		default:
			return []generatedQueryParam{{name: p.name, value: commaDelimitedObject(fields), allowReserved: p.allowReserved}}, nil
		}
	default:
		return []generatedQueryParam{{name: p.name, value: values[0], allowReserved: p.allowReserved}}, nil
	}
}

func serializeGeneratedHeaderParam(p *paramInfo, values []string) ([]string, error) {
	if p.contentMediaType != "" {
		encoded, err := serializeGeneratedContentParam(p, values)
		if err != nil {
			return nil, err
		}
		return []string{encoded}, nil
	}
	if p.typ == "object" {
		fields, err := generatedObjectFields(values)
		if err != nil {
			return nil, err
		}
		if paramExplode(p) {
			parts := make([]string, 0, len(fields))
			for _, field := range fields {
				parts = append(parts, field.key+"="+field.value)
			}
			return []string{strings.Join(parts, ",")}, nil
		}
		return []string{commaDelimitedObject(fields)}, nil
	}
	if p.typ == "array" {
		return []string{strings.Join(normalizeGeneratedArrayValues(values), ",")}, nil
	}
	return []string{values[0]}, nil
}

func serializeGeneratedCookieParam(p *paramInfo, values []string) ([]string, error) {
	if p.contentMediaType != "" {
		encoded, err := serializeGeneratedContentParam(p, values)
		if err != nil {
			return nil, err
		}
		return []string{p.name + "=" + url.QueryEscape(encoded)}, nil
	}
	if p.typ == "object" {
		fields, err := generatedObjectFields(values)
		if err != nil {
			return nil, err
		}
		if paramExplode(p) {
			out := make([]string, 0, len(fields))
			for _, field := range fields {
				out = append(out, url.QueryEscape(field.key)+"="+url.QueryEscape(field.value))
			}
			return out, nil
		}
		return []string{url.QueryEscape(p.name) + "=" + url.QueryEscape(commaDelimitedObject(fields))}, nil
	}
	if p.typ == "array" {
		parts := normalizeGeneratedArrayValues(values)
		if paramExplode(p) {
			out := make([]string, 0, len(parts))
			for _, value := range parts {
				out = append(out, url.QueryEscape(p.name)+"="+url.QueryEscape(value))
			}
			return out, nil
		}
		return []string{url.QueryEscape(p.name) + "=" + url.QueryEscape(strings.Join(parts, ","))}, nil
	}
	return []string{url.QueryEscape(p.name) + "=" + url.QueryEscape(values[0])}, nil
}

func defaultParamStyle(p *paramInfo) string {
	if p.style != "" {
		return p.style
	}
	switch p.in {
	case "query", "cookie":
		return "form"
	case "path", "header":
		return "simple"
	default:
		return "form"
	}
}

func paramExplode(p *paramInfo) bool {
	if p.explode != nil {
		return *p.explode
	}
	return defaultParamStyle(p) == "form"
}

func pathDelimitedParamValue(p *paramInfo, values []string, delimiter string, explode bool) string {
	switch p.typ {
	case "array":
		parts := normalizeGeneratedArrayValues(values)
		escaped := make([]string, 0, len(parts))
		for _, value := range parts {
			escaped = append(escaped, url.PathEscape(value))
		}
		return strings.Join(escaped, delimiter)
	case "object":
		fields, err := generatedObjectFields(values)
		if err != nil {
			return url.PathEscape(strings.Join(values, " "))
		}
		parts := make([]string, 0, len(fields)*2)
		for _, field := range fields {
			if explode {
				parts = append(parts, url.PathEscape(field.key)+"="+url.PathEscape(field.value))
				continue
			}
			parts = append(parts, url.PathEscape(field.key), url.PathEscape(field.value))
		}
		return strings.Join(parts, delimiter)
	default:
		return url.PathEscape(values[0])
	}
}

func normalizeGeneratedArrayValues(values []string) []string {
	if len(values) == 1 {
		parts := splitEscapedComma(values[0])
		if len(parts) > 1 {
			return parts
		}
	}
	return values
}

func splitEscapedComma(value string) []string {
	var parts []string
	var b strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ',' {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}

type objectField struct {
	key   string
	value string
}

func generatedObjectFields(values []string) ([]objectField, error) {
	parsed, err := shorthand.Unmarshal(strings.Join(values, " "), shorthand.ParseOptions{EnableObjectDetection: true}, nil)
	if err != nil {
		return nil, fmt.Errorf("parse structured parameter: %w", err)
	}
	fields := map[string]string{}
	switch v := parsed.(type) {
	case map[string]any:
		for key, value := range v {
			fields[key] = fmt.Sprint(value)
		}
	case map[any]any:
		for key, value := range v {
			fields[fmt.Sprint(key)] = fmt.Sprint(value)
		}
	default:
		return nil, fmt.Errorf("structured parameter must be an object, got %T", parsed)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]objectField, 0, len(keys))
	for _, key := range keys {
		out = append(out, objectField{key: key, value: fields[key]})
	}
	return out, nil
}

func commaDelimitedObject(fields []objectField) string {
	parts := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		parts = append(parts, field.key, field.value)
	}
	return strings.Join(parts, ",")
}

func serializeGeneratedContentParam(p *paramInfo, values []string) (string, error) {
	if !isJSONMediaType(p.contentMediaType) {
		return strings.Join(values, " "), nil
	}
	var value any
	if len(values) == 1 {
		if err := json.Unmarshal([]byte(values[0]), &value); err == nil {
			data, _ := json.Marshal(value)
			return string(data), nil
		}
	}
	parsed, err := shorthand.Unmarshal(strings.Join(values, " "), shorthand.ParseOptions{EnableObjectDetection: true}, nil)
	if err != nil {
		return "", fmt.Errorf("parse JSON parameter: %w", err)
	}
	data, jerr := json.Marshal(parsed)
	if jerr != nil {
		return "", jerr
	}
	return string(data), nil
}

func isJSONMediaType(mediaType string) bool {
	mt := strings.ToLower(strings.TrimSpace(strings.Split(mediaType, ";")[0]))
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

func encodeGeneratedQuery(parts []generatedQueryParam) string {
	if len(parts) == 0 {
		return ""
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, url.QueryEscape(part.name)+"="+encodeGeneratedQueryValue(part.value, part.allowReserved))
	}
	return strings.Join(out, "&")
}

const openAPIReservedChars = ":/?#[]@!$&'()*+,;="

func encodeGeneratedQueryValue(value string, allowReserved bool) string {
	encoded := url.QueryEscape(value)
	if !allowReserved {
		return encoded
	}
	for _, r := range openAPIReservedChars {
		encoded = strings.ReplaceAll(encoded, url.QueryEscape(string(r)), string(r))
	}
	return encoded
}

// extractPathParamNames returns path parameter names in left-to-right order
// from a path template like "/stores/{storeId}/items/{itemId}".
var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

func extractPathParamNames(path string) []string {
	var names []string
	for _, m := range pathParamRe.FindAllStringSubmatch(path, -1) {
		names = append(names, m[1])
	}
	return names
}

// toKebabCase converts a camelCase or PascalCase identifier to kebab-case.
// "getItemById" → "get-item-by-id", "ListUsers" → "list-users".
func toKebabCase(s string) string {
	var b strings.Builder
	var prev rune
	for i, r := range s {
		if prev != 0 && unicode.IsUpper(r) {
			// Look ahead to get the next rune without allocating a rune slice.
			var next rune
			if j := i + utf8.RuneLen(r); j < len(s) {
				next, _ = utf8.DecodeRuneInString(s[j:])
			}
			if unicode.IsLower(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
				b.WriteRune('-')
			}
		}
		b.WriteRune(unicode.ToLower(r))
		prev = r
	}
	return slugify(b.String())
}

func fallbackOperationName(method, path string) string {
	return slugify(strings.ToLower(method) + "-" + strings.Trim(path, "/"))
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// deprecatedNotice returns the cobra Deprecated string when flagged, else "".
func deprecatedNotice(deprecated bool) string {
	if deprecated {
		return "this operation is deprecated"
	}
	return ""
}
