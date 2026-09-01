package cli

import (
	"fmt"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

const (
	printRequestHeaders  rune = 'H'
	printRequestBody     rune = 'B'
	printResponseHeaders rune = 'h'
	printRenderedBody    rune = 'b'
	printRawBody         rune = 0
	printPretty          rune = 'p'
	printColor           rune = 'c'
)

type printSpec struct {
	order  []rune
	pretty bool
	color  bool
}

type printResponseKind int

const (
	printBoundedResponse printResponseKind = iota
	printStreamResponse
	printValueResponse
)

func (s printSpec) has(part rune) bool {
	for _, p := range s.order {
		if p == part {
			return true
		}
	}
	return false
}

func (s printSpec) rawBodyOnly() bool {
	return len(s.order) == 1 && s.order[0] == printRawBody
}

func (s printSpec) includesResponseBody() bool {
	return s.has(printRenderedBody) || s.has(printRawBody)
}

func (c *CLI) resolvePrintSpec(gf GlobalFlags, tty bool, kind printResponseKind) (printSpec, error) {
	if gf.Raw {
		return printSpec{order: []rune{printRawBody}}, nil
	}
	value := strings.TrimSpace(gf.Print)
	if value == "" {
		value = "auto"
	}
	if strings.EqualFold(value, "auto") {
		return c.autoPrintSpec(gf, tty, kind), nil
	}
	return parsePrintSpec(value)
}

func validateRawOutputFlags(cmd *cobra.Command, gf GlobalFlags) error {
	if !gf.Raw {
		return nil
	}
	var conflicts []string
	if gf.OutputFormatSet {
		conflicts = append(conflicts, "-o/--rsh-output-format")
	}
	if gf.PrintSet || cmd.Flags().Changed("rsh-print") {
		conflicts = append(conflicts, "--rsh-print")
	}
	if gf.Filter != "" {
		conflicts = append(conflicts, "-f/--rsh-filter")
	}
	if gf.HeadersShorthand {
		conflicts = append(conflicts, "--rsh-headers")
	}
	if gf.StatusShorthand {
		conflicts = append(conflicts, "--rsh-status")
	}
	if gf.Collect {
		conflicts = append(conflicts, "--rsh-collect")
	}
	if gf.MaxItems != 0 {
		conflicts = append(conflicts, "--rsh-max-items")
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("--rsh-raw cannot be combined with %s", strings.Join(conflicts, ", "))
	}
	return nil
}

func (c *CLI) autoPrintSpec(gf GlobalFlags, tty bool, kind printResponseKind) printSpec {
	if explicitOutputFilter(gf) {
		return printSpec{
			order:  []rune{printRenderedBody},
			pretty: true,
			color:  tty && output.ColorEnabled(c.Stdout),
		}
	}
	if tty {
		return printSpec{
			order:  []rune{printResponseHeaders, printRenderedBody},
			pretty: true,
			color:  output.ColorEnabled(c.Stdout),
		}
	}
	if kind != printValueResponse && untransformedRedirectOutput(gf) {
		return printSpec{order: []rune{printRawBody}}
	}
	return printSpec{order: []rune{printRenderedBody}, pretty: true}
}

func untransformedRedirectOutput(gf GlobalFlags) bool {
	return gf.OutputFormat == "" &&
		!gf.OutputFormatSet &&
		!gf.Collect &&
		gf.MaxItems == 0 &&
		!explicitOutputFilter(gf)
}

func parsePrintSpec(value string) (printSpec, error) {
	var spec printSpec
	seenParts := map[rune]bool{}
	for _, ch := range value {
		switch ch {
		case printRequestHeaders, printRequestBody, printResponseHeaders, printRenderedBody:
			if !seenParts[ch] {
				spec.order = append(spec.order, ch)
				seenParts[ch] = true
			}
		case printPretty:
			spec.pretty = true
		case printColor:
			spec.color = true
		default:
			return printSpec{}, fmt.Errorf("invalid --rsh-print value %q: unknown part %q (use auto or any of H, B, h, b, p, c)", value, string(ch))
		}
	}
	if len(spec.order) == 0 {
		return printSpec{}, fmt.Errorf("invalid --rsh-print value %q: include at least one output part (H, B, h, or b)", value)
	}
	return spec, nil
}
