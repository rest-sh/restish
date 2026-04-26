package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

	for _, op := range ops {
		cmd, err := c.buildOperationCommand(apiName, op, apiCfg.BaseURL, apiCfg.OperationBase)
		if err != nil {
			c.warnf("skipping %s %s for API %q: %v", op.Method, op.Path, apiName, err)
			continue
		}
		if cmd == nil {
			continue
		}
		if commandNames[cmd.Name()] {
			originalName := cmd.Name()
			disambiguated := originalName + "-" + strings.ToLower(op.Method)
			if commandNames[disambiguated] {
				suffix := 2
				for commandNames[fmt.Sprintf("%s-%d", disambiguated, suffix)] {
					suffix++
				}
				disambiguated = fmt.Sprintf("%s-%d", disambiguated, suffix)
			}
			c.warnf("command name collision for API %q: %q; using %q for %s %s", apiName, originalName, disambiguated, op.Method, op.Path)
			cmd.Use = strings.Replace(cmd.Use, originalName, disambiguated, 1)
			cmd.Aliases = append(cmd.Aliases, originalName)
		}
		commandNames[cmd.Name()] = true
		if len(op.Tags) > 0 {
			tag := op.Tags[0]
			if !tagsSeen[tag] {
				tagsSeen[tag] = true
				apiCmd.AddGroup(&cobra.Group{ID: tag, Title: tag})
			}
			cmd.GroupID = tag
		}
		apiCmd.AddCommand(cmd)
	}

	return apiCmd
}

// paramInfo holds the information we need about a single parameter.
type paramInfo struct {
	name         string // original API parameter name
	flagName     string // kebab-case flag name
	in           string // "path", "query", "header", "cookie"
	required     bool
	hidden       bool
	desc         string
	schema       string
	typ          string
	itemType     string
	defaultValue string
	hasDefault   bool
	style        string
	explode      *bool
	enum         []string // allowed values from OpenAPI schema enum, if present
}

// buildOperationCommand creates a Cobra command for one OpenAPI operation.
// Returns nil when the operation is excluded via x-cli-ignore.
// operationBase, when non-empty, is resolved against baseURL and replaces the
// apiName short-name prefix in generated URLs.
func (c *CLI) buildOperationCommand(apiName string, op spec.Operation, baseURL, operationBase string) (*cobra.Command, error) {
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
		allParams[p.Name] = &paramInfo{
			name:         p.Name,
			flagName:     flagName,
			in:           p.In,
			required:     p.Required,
			hidden:       p.XCLI.Hidden,
			desc:         desc,
			schema:       p.Schema,
			typ:          p.Type,
			itemType:     p.ItemType,
			defaultValue: p.Default,
			hasDefault:   p.HasDefault,
			style:        p.Style,
			explode:      p.Explode,
			enum:         p.Enum,
		}
	}

	// Required: path params (in path order) then required query params.
	var required []*paramInfo
	for _, name := range pathParamOrder {
		pi, ok := allParams[name]
		if !ok {
			opID := op.ID
			if opID == "" {
				opID = op.Method + " " + op.Path
			}
			return nil, fmt.Errorf("operation %q references missing path parameter %q", opID, name)
		}
		required = append(required, pi)
	}
	for _, p := range op.Parameters {
		if p.In == "query" && p.Required {
			if pi := allParams[p.Name]; pi != nil {
				required = append(required, pi)
			}
		}
	}

	// Optional: non-required, non-path params.
	var optional []*paramInfo
	for _, p := range op.Parameters {
		if p.In == "path" {
			continue
		}
		if p.In == "header" || p.In == "cookie" {
			if pi := allParams[p.Name]; pi != nil {
				optional = append(optional, pi)
			}
			continue
		}
		if !p.Required {
			if pi := allParams[p.Name]; pi != nil {
				optional = append(optional, pi)
			}
		}
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
		Example: generatedOperationExamples(apiName, use, op.Help.Examples),
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
			if generateBody, _ := cmd.Flags().GetBool("rsh-generate-body"); generateBody {
				return c.printGeneratedBodyExample(op.Help.Request)
			}
			return c.runGeneratedOp(cmd, apiName, op.Path, op.Method, op.RequestMediaType, op.RequestSchemaTypes, op.NoAuth, required, optional, args, baseURL, operationBase)
		},
	}
	if !op.HasBody {
		cmd.Args = cobra.ExactArgs(len(required))
	} else {
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
		if p.required {
			_ = cmd.MarkFlagRequired(p.flagName)
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
			if p.schema == "" {
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
		if p.schema != "" {
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
	noAuth bool,
	required, optional []*paramInfo,
	args []string,
	baseURL string,
	operationBase string,
) error {
	// Substitute required params into the path, query string, and headers.
	path := opPath
	q := url.Values{}
	var extraHeaders []string
	bodyArgStart := len(required)

	for i, p := range required {
		val := args[i]
		path, extraHeaders = addGeneratedParam(path, q, extraHeaders, p, []string{val})
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
		path, extraHeaders = addGeneratedParam(path, q, extraHeaders, p, values)
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
	if qs := q.Encode(); qs != "" {
		rawURL += "?" + qs
	}

	bodyArgs := args[bodyArgStart:]
	return c.runHTTPInternal(cmd, method, append([]string{rawURL}, bodyArgs...), false, extraHeaders, noAuth, "", requestMediaType, requestSchemaTypes)
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

func addGeneratedParam(path string, q url.Values, extraHeaders []string, p *paramInfo, values []string) (string, []string) {
	if len(values) == 0 {
		return path, extraHeaders
	}
	switch p.in {
	case "path":
		path = strings.ReplaceAll(path, "{"+p.name+"}", url.PathEscape(values[0]))
	case "query":
		for _, val := range serializeGeneratedParamValues(p, values) {
			q.Add(p.name, val)
		}
	case "header":
		extraHeaders = append(extraHeaders, p.name+": "+strings.Join(serializeGeneratedParamValues(p, values), ","))
	case "cookie":
		extraHeaders = append(extraHeaders, "Cookie: "+p.name+"="+url.QueryEscape(strings.Join(serializeGeneratedParamValues(p, values), ",")))
	}
	return path, extraHeaders
}

func serializeGeneratedParamValues(p *paramInfo, values []string) []string {
	if p.typ != "array" {
		return values[:1]
	}
	style := p.style
	if style == "" {
		style = "form"
	}
	explode := true
	if p.explode != nil {
		explode = *p.explode
	}
	if p.in == "query" && style == "form" && explode {
		return values
	}
	return []string{strings.Join(values, ",")}
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
