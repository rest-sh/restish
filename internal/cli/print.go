package cli

import (
	"fmt"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
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
	value := strings.TrimSpace(gf.Print)
	if value == "" {
		value = "auto"
	}
	if strings.EqualFold(value, "auto") {
		return c.autoPrintSpec(gf, tty, kind), nil
	}
	return parsePrintSpec(value)
}

func (c *CLI) autoPrintSpec(gf GlobalFlags, tty bool, kind printResponseKind) printSpec {
	if explicitOutputFilter(gf) {
		return printSpec{order: []rune{printRenderedBody}, pretty: true}
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
