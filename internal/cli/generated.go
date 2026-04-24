package cli

import (
	"fmt"
	"net/url"
	"regexp"
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
	ops, err := s.Operations(apiCfg.BaseURL, apiCfg.OperationBase)
	if err != nil {
		source := apiCfg.SpecURL
		if source == "" && len(apiCfg.SpecFiles) > 0 {
			source = apiCfg.SpecFiles[0]
		}
		if source == "" {
			source = apiCfg.BaseURL
		}
		fmt.Fprintf(c.Stderr, "warning: skipping generated commands for API %q from %s: %v\n", apiName, source, err)
		return nil
	}
	// ops == nil means no V3 model or no paths section — nothing to generate.
	if ops == nil {
		return nil
	}

	apiCmd := &cobra.Command{
		Use:     apiName,
		Short:   fmt.Sprintf("Commands generated from the %s API spec", apiName),
		GroupID: rootGroupAPI,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.Name())
			}
			return cmd.Help()
		},
	}

	tagsSeen := map[string]bool{}
	commandNames := map[string]bool{}

	for _, op := range ops {
		cmd, err := c.buildOperationCommand(apiName, op, apiCfg.OperationBase)
		if err != nil {
			fmt.Fprintf(c.Stderr, "warning: skipping %s %s for API %q: %v\n", op.Method, op.Path, apiName, err)
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
			fmt.Fprintf(c.Stderr, "warning: command name collision for API %q: %q; using %q for %s %s\n", apiName, originalName, disambiguated, op.Method, op.Path)
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
	name     string // original API parameter name
	flagName string // kebab-case flag name
	in       string // "path", "query", "header", "cookie"
	required bool
	hidden   bool
	desc     string
	enum     []string // allowed values from OpenAPI schema enum, if present
}

// buildOperationCommand creates a Cobra command for one OpenAPI operation.
// Returns nil when the operation is excluded via x-cli-ignore.
// operationBase, when non-empty, replaces the apiName short-name prefix in
// generated URLs so operations are served from a different host/path than
// the API root.
func (c *CLI) buildOperationCommand(apiName string, op spec.Operation, operationBase string) (*cobra.Command, error) {
	// Derive command name from operationId, with x-cli-name override.
	cmdName := toKebabCase(op.ID)
	if cmdName == "" {
		cmdName = toKebabCase(op.Method + "-" + strings.Trim(op.Path, "/"))
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
			name:     p.Name,
			flagName: flagName,
			in:       p.In,
			required: p.Required,
			hidden:   p.XCLI.Hidden,
			desc:     desc,
			enum:     p.Enum,
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

	cmd := &cobra.Command{
		Use:        use,
		Short:      short,
		Long:       long,
		Aliases:    op.XCLI.Aliases,
		Args:       cobra.MinimumNArgs(len(required)),
		Hidden:     op.XCLI.Hidden,
		Deprecated: deprecatedNotice(op.Deprecated),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runGeneratedOp(cmd, apiName, op.Path, op.Method, required, optional, args, operationBase)
		},
	}
	if !op.HasBody {
		cmd.Args = cobra.ExactArgs(len(required))
	}

	for _, p := range optional {
		cmd.Flags().String(p.flagName, "", p.desc)
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

// runGeneratedOp is the RunE handler for generated operation commands.
func (c *CLI) runGeneratedOp(
	cmd *cobra.Command,
	apiName, opPath, method string,
	required, optional []*paramInfo,
	args []string,
	operationBase string,
) error {
	// Substitute required params into the path, query string, and headers.
	path := opPath
	q := url.Values{}
	var extraHeaders []string
	bodyArgStart := len(required)

	for i, p := range required {
		val := args[i]
		switch p.in {
		case "path":
			path = strings.ReplaceAll(path, "{"+p.name+"}", url.PathEscape(val))
		case "query":
			q.Set(p.name, val)
		case "header":
			extraHeaders = append(extraHeaders, p.name+": "+val)
		case "cookie":
			extraHeaders = append(extraHeaders, "Cookie: "+p.name+"="+url.PathEscape(val))
		}
	}

	// Collect optional param flags.
	for _, p := range optional {
		val, err := cmd.Flags().GetString(p.flagName)
		if err != nil || val == "" {
			continue
		}
		switch p.in {
		case "query":
			q.Set(p.name, val)
		case "header":
			extraHeaders = append(extraHeaders, p.name+": "+val)
		case "cookie":
			extraHeaders = append(extraHeaders, "Cookie: "+p.name+"="+url.PathEscape(val))
		}
	}

	// Build the raw URL. When operation_base is set, use it as the full URL
	// prefix instead of the short-name shorthand so that generated commands
	// hit a different host/path than the API root (auth is still applied via
	// the operation_base prefix match in applyAPIProfile).
	var rawURL string
	if operationBase != "" {
		rawURL = strings.TrimRight(operationBase, "/") + path
	} else {
		rawURL = apiName + path
	}
	if qs := q.Encode(); qs != "" {
		rawURL += "?" + qs
	}

	bodyArgs := args[bodyArgStart:]
	return c.runHTTPInternal(cmd, method, append([]string{rawURL}, bodyArgs...), false, extraHeaders, false, "")
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
	// Replace underscores and spaces with dashes, collapse multiple dashes.
	result := strings.ReplaceAll(b.String(), "_", "-")
	result = strings.ReplaceAll(result, " ", "-")
	return result
}

// deprecatedNotice returns the cobra Deprecated string when flagged, else "".
func deprecatedNotice(deprecated bool) string {
	if deprecated {
		return "this operation is deprecated"
	}
	return ""
}
