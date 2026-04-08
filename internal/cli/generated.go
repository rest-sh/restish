package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/spf13/cobra"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/spec"
)

// buildAPICommand constructs a Cobra command group for a registered API and
// populates it with one subcommand per OpenAPI operation found in s.
// Returns nil when the spec cannot be built into a v3 model.
func (c *CLI) buildAPICommand(apiName string, apiCfg *config.APIConfig, s *spec.APISpec) *cobra.Command {
	model, err := s.V3Model()
	if err != nil || model == nil || model.Model.Paths == nil {
		return nil
	}

	apiCmd := &cobra.Command{
		Use:   apiName,
		Short: fmt.Sprintf("Commands generated from the %s API spec", apiName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.Name())
			}
			return cmd.Help()
		},
	}

	// Collect unique tags so we can create command groups for help display.
	tagsSeen := map[string]bool{}

	for path, pathItem := range model.Model.Paths.PathItems.FromOldest() {
		for _, mo := range spec.PathItemMethods(pathItem) {
			if mo.Op == nil || mo.Op.OperationId == "" {
				continue
			}
			cmd := c.buildOperationCommand(apiName, path, mo.Method, mo.Op, apiCfg.OperationBase)
			if cmd == nil {
				continue
			}
			// Register first tag as the group ID.
			if len(mo.Op.Tags) > 0 {
				tag := mo.Op.Tags[0]
				if !tagsSeen[tag] {
					tagsSeen[tag] = true
					apiCmd.AddGroup(&cobra.Group{
						ID:    tag,
						Title: tag,
					})
				}
				cmd.GroupID = tag
			}
			apiCmd.AddCommand(cmd)
		}
	}

	return apiCmd
}

// paramInfo holds the information we need about a single parameter.
type paramInfo struct {
	name     string // original API parameter name
	flagName string // kebab-case flag name
	in       string // "path", "query", "header", "cookie"
	required bool
	desc     string
	enum     []string // allowed values from OpenAPI schema enum, if present
}

// buildOperationCommand creates a Cobra command for one OpenAPI operation.
// Returns nil when the operation is excluded via x-cli-ignore.
// operationBase, when non-empty, replaces the apiName short-name prefix in
// generated URLs so operations are served from a different host/path than
// the API root.
func (c *CLI) buildOperationCommand(apiName, opPath, method string, op *v3high.Operation, operationBase string) *cobra.Command {
	// x-cli-ignore: exclude this operation from the CLI entirely.
	if spec.OpExtBool(op, "x-cli-ignore") {
		return nil
	}

	// Derive command name from operationId, with x-cli-name override.
	cmdName := toKebabCase(op.OperationId)
	if name := spec.OpExtString(op, "x-cli-name"); name != "" {
		cmdName = name
	}

	// Build param lists, honouring per-parameter x-cli-name / x-cli-description.
	pathParamOrder := extractPathParamNames(opPath)

	allParams := make(map[string]*paramInfo)
	for _, p := range op.Parameters {
		req := p.Required != nil && *p.Required
		flagName := toKebabCase(p.Name)
		if n := spec.ParamExtString(p, "x-cli-name"); n != "" {
			flagName = n
		}
		desc := p.Description
		if d := spec.ParamExtString(p, "x-cli-description"); d != "" {
			desc = d
		}
		var enumVals []string
		if p.Schema != nil {
			if schema := p.Schema.Schema(); schema != nil {
				for _, node := range schema.Enum {
					if node != nil {
						enumVals = append(enumVals, node.Value)
					}
				}
			}
		}
		allParams[p.Name] = &paramInfo{
			name:     p.Name,
			flagName: flagName,
			in:       p.In,
			required: req,
			desc:     desc,
			enum:     enumVals,
		}
	}

	// Required: path params (in path order) then required query params.
	var required []*paramInfo
	for _, name := range pathParamOrder {
		if pi, ok := allParams[name]; ok {
			required = append(required, pi)
		}
	}
	for _, p := range op.Parameters {
		if p.In == "query" && p.Required != nil && *p.Required {
			required = append(required, allParams[p.Name])
		}
	}

	// Optional: non-required, non-path params.
	var optional []*paramInfo
	for _, p := range op.Parameters {
		if p.In == "path" {
			continue
		}
		if req := p.Required != nil && *p.Required; !req {
			optional = append(optional, allParams[p.Name])
		}
	}

	// Build Use string.
	use := cmdName
	for _, p := range required {
		use += " <" + p.flagName + ">"
	}
	if op.RequestBody != nil {
		use += " [body...]"
	}

	// Short and long descriptions, with x-cli-description override.
	short := op.Summary
	if desc := spec.OpExtString(op, "x-cli-description"); desc != "" {
		short = desc
	}
	if short == "" {
		short = fmt.Sprintf("%s %s", method, opPath)
	}

	long := op.Description
	if desc := spec.OpExtString(op, "x-cli-description"); desc != "" {
		long = desc
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

	deprecated := op.Deprecated != nil && *op.Deprecated
	hidden := spec.OpExtBool(op, "x-cli-hidden")
	aliases := spec.OpExtStrings(op, "x-cli-aliases")

	cmd := &cobra.Command{
		Use:        use,
		Short:      short,
		Long:       long,
		Aliases:    aliases,
		Args:       cobra.MinimumNArgs(len(required)),
		Hidden:     hidden,
		Deprecated: deprecatedNotice(deprecated),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runGeneratedOp(cmd, apiName, opPath, method, required, optional, args, operationBase)
		},
	}

	for _, p := range optional {
		cmd.Flags().String(p.flagName, "", p.desc)
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

	return cmd
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
	return c.runHTTPInternal(cmd, method, append([]string{rawURL}, bodyArgs...), false, extraHeaders, false)
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
