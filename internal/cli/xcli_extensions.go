package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/rest-sh/restish/v2/internal/spec"
)

func (c *CLI) printXCLIExtensionSummary(report spec.XCLIExtensionReport) {
	summary := report.Summary()
	if len(summary) == 0 {
		return
	}
	style := humanTextStyleFor(c.Stdout)
	fmt.Fprintf(c.Stdout, "%s x-cli extensions: %s\n", style.info("OpenAPI"), strings.Join(summary, ", "))
}

func printXCLIExtensionDoctorDetails(out io.Writer, style humanTextStyle, report spec.XCLIExtensionReport) {
	summary := report.Summary()
	if len(summary) == 0 {
		return
	}
	fmt.Fprintf(out, "OpenAPI x-cli extensions: %s\n", strings.Join(summary, ", "))
	for _, detail := range report.Details {
		fmt.Fprintf(out, "  %s %s: %s", style.key(detail.Extension+":"), detail.Location, detail.Effect)
		if detail.Value != "" {
			fmt.Fprintf(out, " (%s)", detail.Value)
		}
		if detail.Name != "" {
			fmt.Fprintf(out, " [%s]", detail.Name)
		}
		fmt.Fprintln(out)
	}
}
