package plugin

import (
	"io"
	"time"
)

const (
	// StartupFlagManifest asks a plugin to write its Manifest and exit.
	StartupFlagManifest = "--rsh-plugin-manifest"
	// StartupFlagCommands asks a command plugin to write its command list and exit.
	StartupFlagCommands = "--rsh-plugin-commands"
	// StartupFlagColor tells a command plugin whether host terminal color is enabled.
	StartupFlagColor = "--rsh-color"
	// StartupFlagStdoutTTY tells a command plugin whether host stdout is a TTY.
	StartupFlagStdoutTTY = "--rsh-stdout-tty"
	// StartupFlagStderrTTY tells a command plugin whether host stderr is a TTY.
	StartupFlagStderrTTY = "--rsh-stderr-tty"
)

// Manifest is the metadata a plugin reports when called with
// --rsh-plugin-manifest. Plugin authors populate and write this with
// WriteManifest instead of manually marshalling CBOR.
type Manifest struct {
	// Name is the stable plugin identifier, without the "restish-" executable prefix.
	Name string `cbor:"name" json:"name"`
	// Version is a plugin-defined version string shown in plugin listings.
	Version string `cbor:"version,omitempty" json:"version,omitempty"`
	// Description is a short human-readable summary of the plugin.
	Description string `cbor:"description,omitempty" json:"description,omitempty"`
	// RestishAPIVersion is the plugin protocol version the plugin expects.
	RestishAPIVersion int `cbor:"restish_api_version" json:"restish_api_version"`
	// Hooks lists plugin capabilities such as "command", "formatter", or "auth".
	Hooks []string `cbor:"hooks,omitempty" json:"hooks,omitempty"`
	// FormatterNames lists the output format names this plugin registers when
	// the "formatter" hook is declared.
	FormatterNames []string `cbor:"formatter_names,omitempty" json:"formatter_names,omitempty"`
	// LoaderContentTypes lists the MIME types this plugin handles when the
	// "loader" hook is declared.
	LoaderContentTypes []string `cbor:"loader_content_types,omitempty" json:"loader_content_types,omitempty"`
	// AuthAPINames, when non-empty, restricts the "auth" hook to the listed
	// API names so the plugin is not invoked for every unrelated API.
	AuthAPINames []string `cbor:"auth_api_names,omitempty" json:"auth_api_names,omitempty"`
	// NeedsAuthSecrets, when true, tells Restish to forward secret auth
	// params (passwords, client secrets) and credential-bearing request headers
	// to this plugin. When false (the default), secret params are omitted and
	// Authorization, Cookie, and Proxy-Authorization request headers are sent
	// as "<redacted>" in auth and middleware hook payloads.
	NeedsAuthSecrets bool `cbor:"needs_auth_secrets,omitempty" json:"needs_auth_secrets,omitempty"`
	// HookTimeouts overrides the per-hook subprocess deadline. Keys are hook
	// names (e.g. "auth", "request-middleware"). The default is 30 s for all
	// hooks except "auth", which defaults to 5 minutes.
	HookTimeouts map[string]time.Duration `cbor:"hook_timeouts,omitempty" json:"hook_timeouts,omitempty"`
}

// CommandDecl describes one command that a command-plugin exposes.
// It is used in the response to --rsh-plugin-commands.
type CommandDecl struct {
	// Name is the top-level command name contributed by the plugin.
	Name string `cbor:"name" json:"name"`
	// Short is the one-line help text shown in command listings.
	Short string `cbor:"short,omitempty" json:"short,omitempty"`
	// Long is optional extended help text.
	Long string `cbor:"long,omitempty" json:"long,omitempty"`
	// PassthroughStdio asks the host to forward stdin frames to the plugin.
	PassthroughStdio bool `cbor:"passthrough_stdio,omitempty" json:"passthrough_stdio,omitempty"`
}

// WriteManifest serialises m as a CBOR data item and writes it to w.
// It is the canonical way to respond to --rsh-plugin-manifest.
//
//	case plugin.StartupFlagManifest:
//	    return plugin.WriteManifest(os.Stdout, m)
func WriteManifest(w io.Writer, m Manifest) error {
	return WriteMessage(w, m)
}

// WriteCommands serialises cmds as a CBOR map with a "commands" array and
// writes it to w. It is the canonical way to respond to --rsh-plugin-commands.
//
//	case plugin.StartupFlagCommands:
//	    return plugin.WriteCommands(os.Stdout, cmds)
func WriteCommands(w io.Writer, cmds []CommandDecl) error {
	type wrapper struct {
		Commands []CommandDecl `cbor:"commands"`
	}
	return WriteMessage(w, wrapper{Commands: cmds})
}
