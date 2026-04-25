package cli

import (
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const rshFlagGroupAnnotation = "restish-flag-group"

const (
	flagGroupRequest   = "Request Options"
	flagGroupOutput    = "Output Options"
	flagGroupAuth      = "Auth and Profile Options"
	flagGroupTLS       = "TLS Options"
	flagGroupPaging    = "Pagination and Streaming Options"
	flagGroupCache     = "Cache and Retry Options"
	flagGroupGeneral   = "General Options"
	flagGroupUngrouped = "Options"
)

var addHelpTemplateFuncsOnce sync.Once

var flagGroupOrder = []string{
	flagGroupRequest,
	flagGroupOutput,
	flagGroupAuth,
	flagGroupTLS,
	flagGroupPaging,
	flagGroupCache,
	flagGroupGeneral,
	flagGroupUngrouped,
}

var defaultFlagGroups = map[string]string{
	"rsh-header":             flagGroupRequest,
	"rsh-query":              flagGroupRequest,
	"rsh-server":             flagGroupRequest,
	"rsh-content-type":       flagGroupRequest,
	"rsh-timeout":            flagGroupRequest,
	"rsh-max-body-size":      flagGroupRequest,
	"rsh-ignore-status-code": flagGroupRequest,

	"rsh-output-format": flagGroupOutput,
	"rsh-filter":        flagGroupOutput,
	"rsh-filter-lang":   flagGroupOutput,
	"rsh-headers":       flagGroupOutput,
	"rsh-raw":           flagGroupOutput,
	"rsh-columns":       flagGroupOutput,
	"rsh-sort-by":       flagGroupOutput,
	"rsh-silent":        flagGroupOutput,

	"rsh-profile":    flagGroupAuth,
	"rsh-no-browser": flagGroupAuth,

	"rsh-insecure":         flagGroupTLS,
	"rsh-client-cert":      flagGroupTLS,
	"rsh-client-key":       flagGroupTLS,
	"rsh-tls-signer":       flagGroupTLS,
	"rsh-tls-signer-param": flagGroupTLS,
	"rsh-ca-cert":          flagGroupTLS,
	"rsh-tls-min-version":  flagGroupTLS,

	"rsh-no-paginate": flagGroupPaging,
	"rsh-collect":     flagGroupPaging,
	"rsh-max-pages":   flagGroupPaging,
	"rsh-max-items":   flagGroupPaging,
	"rsh-max-events":  flagGroupPaging,

	"rsh-no-cache": flagGroupCache,
	"rsh-retry":    flagGroupCache,

	"rsh-verbose": flagGroupGeneral,
	"help":        flagGroupGeneral,
	"version":     flagGroupGeneral,
}

const groupedUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}{{with rshFlagUsages .LocalFlags}}

Flags:
{{. | trimTrailingWhitespaces}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}{{with rshFlagUsages .InheritedFlags}}

Global Flags:
{{. | trimTrailingWhitespaces}}{{end}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func setupGroupedUsage(root *cobra.Command) {
	addHelpTemplateFuncsOnce.Do(func() {
		cobra.AddTemplateFunc("rshFlagUsages", groupedFlagUsages)
	})
	root.SetUsageTemplate(groupedUsageTemplate)
}

func groupedFlagUsages(flags *pflag.FlagSet) string {
	if flags == nil {
		return ""
	}

	groups := make(map[string]*pflag.FlagSet)
	hasGroups := false
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		group := flagGroupFor(flag)
		if group != flagGroupUngrouped {
			hasGroups = true
		}
		if groups[group] == nil {
			groups[group] = pflag.NewFlagSet("", pflag.ContinueOnError)
			groups[group].SortFlags = flags.SortFlags
		}
		groups[group].AddFlag(flag)
	})

	if !hasGroups {
		return flags.FlagUsages()
	}

	var out strings.Builder
	for _, group := range flagGroupOrder {
		groupFlags := groups[group]
		if groupFlags == nil || !groupFlags.HasAvailableFlags() {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString(group)
		out.WriteString("\n")
		out.WriteString(groupFlags.FlagUsages())
	}
	return out.String()
}

func flagGroupFor(flag *pflag.Flag) string {
	if flag == nil {
		return flagGroupUngrouped
	}
	if values := flag.Annotations[rshFlagGroupAnnotation]; len(values) > 0 && values[0] != "" {
		return values[0]
	}
	if group := defaultFlagGroups[flag.Name]; group != "" {
		return group
	}
	return flagGroupUngrouped
}
