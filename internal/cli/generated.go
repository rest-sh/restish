package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/spf13/cobra"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/spec"
)

// buildAPICommand constructs a Cobra command group for a registered API and
// populates it with one subcommand per OpenAPI operation found in s.
// Returns nil when the spec cannot be built into a v3 model.
func (c *CLI) buildAPICommand(apiName string, apiCfg *config.APIConfig, s *spec.APISpec) *cobra.Command {
	model, err := s.Document.BuildV3Model()
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
		type methodOp struct {
			method string
			op     *v3high.Operation
		}
		ops := []methodOp{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"PATCH", pathItem.Patch},
			{"DELETE", pathItem.Delete},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}
		for _, mo := range ops {
			if mo.op == nil || mo.op.OperationId == "" {
				continue
			}
			cmd := c.buildOperationCommand(apiName, path, mo.method, mo.op)
			if cmd == nil {
				continue
			}
			// Register first tag as the group ID.
			if len(mo.op.Tags) > 0 {
				tag := mo.op.Tags[0]
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
}

// buildOperationCommand creates a Cobra command for one OpenAPI operation.
// Returns nil when the operation is excluded via x-cli-ignore.
func (c *CLI) buildOperationCommand(apiName, opPath, method string, op *v3high.Operation) *cobra.Command {
	// x-cli-ignore: exclude this operation from the CLI entirely.
	if opExtBool(op, "x-cli-ignore") {
		return nil
	}

	// Derive command name from operationId, with x-cli-name override.
	cmdName := toKebabCase(op.OperationId)
	if name := opExtString(op, "x-cli-name"); name != "" {
		cmdName = name
	}

	// Build param lists, honouring per-parameter x-cli-name / x-cli-description.
	pathParamOrder := extractPathParamNames(opPath)

	allParams := make(map[string]*paramInfo)
	for _, p := range op.Parameters {
		req := p.Required != nil && *p.Required
		flagName := toKebabCase(p.Name)
		if n := paramExtString(p, "x-cli-name"); n != "" {
			flagName = n
		}
		desc := p.Description
		if d := paramExtString(p, "x-cli-description"); d != "" {
			desc = d
		}
		allParams[p.Name] = &paramInfo{
			name:     p.Name,
			flagName: flagName,
			in:       p.In,
			required: req,
			desc:     desc,
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
	if desc := opExtString(op, "x-cli-description"); desc != "" {
		short = desc
	}
	if short == "" {
		short = fmt.Sprintf("%s %s", method, opPath)
	}

	long := op.Description
	if desc := opExtString(op, "x-cli-description"); desc != "" {
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
	hidden := opExtBool(op, "x-cli-hidden")
	aliases := opExtStrings(op, "x-cli-aliases")

	cmd := &cobra.Command{
		Use:        use,
		Short:      short,
		Long:       long,
		Aliases:    aliases,
		Args:       cobra.MinimumNArgs(len(required)),
		Hidden:     hidden,
		Deprecated: deprecatedNotice(deprecated),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runGeneratedOp(cmd, apiName, opPath, method, required, optional, args)
		},
	}

	for _, p := range optional {
		cmd.Flags().String(p.flagName, "", p.desc)
	}

	return cmd
}

// runGeneratedOp is the RunE handler for generated operation commands.
func (c *CLI) runGeneratedOp(
	cmd *cobra.Command,
	apiName, opPath, method string,
	required, optional []*paramInfo,
	args []string,
) error {
	// Substitute required params into the path and build query string.
	path := opPath
	q := url.Values{}
	bodyArgStart := len(required)

	for i, p := range required {
		val := args[i]
		if p.in == "path" {
			path = strings.ReplaceAll(path, "{"+p.name+"}", url.PathEscape(val))
		} else if p.in == "query" {
			q.Set(p.name, val)
		}
	}

	// Collect optional query param flags.
	for _, p := range optional {
		if p.in != "query" {
			continue
		}
		val, err := cmd.Flags().GetString(p.flagName)
		if err != nil || val == "" {
			continue
		}
		q.Set(p.name, val)
	}

	// Build the raw URL using the "apiname/path" shorthand.
	rawURL := apiName + path
	if qs := q.Encode(); qs != "" {
		rawURL += "?" + qs
	}

	bodyArgs := args[bodyArgStart:]
	return c.runHTTP(cmd, method, append([]string{rawURL}, bodyArgs...))
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
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			// Insert dash before an uppercase letter that follows a lowercase
			// letter, or before an uppercase that is followed by a lowercase
			// (handles acronyms: "getHTTPClient" → "get-http-client").
			prev := runes[i-1]
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
				b.WriteRune('-')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	// Replace underscores and spaces with dashes, collapse multiple dashes.
	result := strings.ReplaceAll(b.String(), "_", "-")
	result = strings.ReplaceAll(result, " ", "-")
	return result
}

// opExtBool reads a boolean extension from an operation.
func opExtBool(op *v3high.Operation, key string) bool {
	if op.Extensions == nil {
		return false
	}
	node := op.Extensions.GetOrZero(key)
	if node == nil {
		return false
	}
	var v bool
	_ = node.Decode(&v)
	return v
}

// opExtString reads a string extension from an operation.
func opExtString(op *v3high.Operation, key string) string {
	if op.Extensions == nil {
		return ""
	}
	node := op.Extensions.GetOrZero(key)
	if node == nil {
		return ""
	}
	var v string
	_ = node.Decode(&v)
	return v
}

// opExtStrings reads a string-slice extension from an operation.
func opExtStrings(op *v3high.Operation, key string) []string {
	if op.Extensions == nil {
		return nil
	}
	node := op.Extensions.GetOrZero(key)
	if node == nil {
		return nil
	}
	var v []string
	_ = node.Decode(&v)
	return v
}

// paramExtString reads a string extension from a parameter.
func paramExtString(p *v3high.Parameter, key string) string {
	if p.Extensions == nil {
		return ""
	}
	node := p.Extensions.GetOrZero(key)
	if node == nil {
		return ""
	}
	var v string
	_ = node.Decode(&v)
	return v
}

// deprecatedNotice returns the cobra Deprecated string when flagged, else "".
func deprecatedNotice(deprecated bool) string {
	if deprecated {
		return "this operation is deprecated"
	}
	return ""
}
