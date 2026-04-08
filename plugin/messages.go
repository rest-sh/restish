package plugin

// Message type constants for the command plugin protocol.
// Use these instead of bare strings to avoid typos; a mismatched type string
// causes the host or plugin to silently ignore the message.
const (
	// Plugin → host requests.
	MsgTypeInit         = "init"
	MsgTypeHTTPRequest  = "http-request"
	MsgTypeAPISpec      = "api-spec"
	MsgTypeListAPIs     = "list-apis"
	MsgTypeListProfiles = "list-profiles"
	MsgTypeConfigRead   = "config-read"
	MsgTypePrompt       = "prompt"
	MsgTypeConfirm      = "confirm"
	MsgTypeResponse     = "response"
	MsgTypeDone         = "done"
	MsgTypeStdoutData   = "stdout-data"
	MsgTypeStderrData   = "stderr-data"
	MsgTypeWarn         = "warn"

	// Host → plugin responses.
	MsgTypeHTTPResponse         = "http-response"
	MsgTypeAPISpecResponse      = "api-spec-response"
	MsgTypeListAPIsResponse     = "list-apis-response"
	MsgTypeListProfilesResponse = "list-profiles-response"
	MsgTypeConfigReadResponse   = "config-read-response"
	MsgTypePromptResponse       = "prompt-response"
	MsgTypeConfirmResponse      = "confirm-response"

	// Host → plugin passthrough-stdio data.
	MsgTypeStdinData  = "stdin-data"
	MsgTypeStdinClose = "stdin-close"
)

// InitMsg is the first message sent from the host to the plugin after startup.
// It carries the sub-command name and the raw CLI arguments.
type InitMsg struct {
	Type    string   `cbor:"type"`
	Command string   `cbor:"command"`
	Args    []string `cbor:"args"`
}

// HTTPRequestMsg asks the host to perform an HTTP request on behalf of the
// plugin and reply with an HTTPResponseMsg.
type HTTPRequestMsg struct {
	Type        string            `cbor:"type"`
	Method      string            `cbor:"method,omitempty"`
	URI         string            `cbor:"uri"`
	Headers     map[string]string `cbor:"headers,omitempty"`
	Body        any               `cbor:"body,omitempty"`
	ContentType string            `cbor:"content_type,omitempty"`
	// NoCache bypasses the response cache for this request.
	NoCache bool `cbor:"no_cache,omitempty"`
	// CacheTTL (seconds) injects Cache-Control: max-age=N on the request.
	// Only entries fresher than this TTL are served from the cache.
	CacheTTL int `cbor:"cache_ttl,omitempty"`
	// Timeout (seconds) sets a per-request deadline. 0 means no deadline.
	Timeout int `cbor:"timeout,omitempty"`
	// Filter is an optional shorthand or jq expression. When set, the host
	// applies it to the full response document before sending it back.
	Filter string `cbor:"filter,omitempty"`
}

// HTTPResponseMsg is the host reply to an HTTPRequestMsg.
type HTTPResponseMsg struct {
	Type    string            `cbor:"type"`
	Status  int               `cbor:"status"`
	Headers map[string]string `cbor:"headers,omitempty"`
	Body    any               `cbor:"body"`
	// Error is set when the HTTP request itself failed.
	Error string `cbor:"error,omitempty"`
}

// APISpecMsg asks the host to load the OpenAPI spec for a registered API.
type APISpecMsg struct {
	Type string `cbor:"type"`
	Name string `cbor:"name"`
}

// APISpecResponseMsg is the host reply to an APISpecMsg.
type APISpecResponseMsg struct {
	Type        string `cbor:"type"`
	Name        string `cbor:"name"`
	ContentType string `cbor:"content_type,omitempty"`
	Raw         []byte `cbor:"raw,omitempty"`
	Error       string `cbor:"error,omitempty"`
}

// ListAPIsMsg asks the host for the list of configured API names.
type ListAPIsMsg struct {
	Type string `cbor:"type"`
}

// ListAPIsResponseMsg is the host reply to a ListAPIsMsg.
type ListAPIsResponseMsg struct {
	Type  string   `cbor:"type"`
	APIs  []string `cbor:"apis"`
	Error string   `cbor:"error,omitempty"`
}

// ListProfilesMsg asks the host for the profile names of a specific API.
type ListProfilesMsg struct {
	Type string `cbor:"type"`
	API  string `cbor:"api"`
}

// ListProfilesResponseMsg is the host reply to a ListProfilesMsg.
type ListProfilesResponseMsg struct {
	Type     string   `cbor:"type"`
	API      string   `cbor:"api"`
	Profiles []string `cbor:"profiles"`
	Error    string   `cbor:"error,omitempty"`
}

// ConfigReadMsg asks the host for the effective configuration of an API
// profile (base URL, persistent headers and query params), and/or the
// plugin-specific config stored under plugins[Plugin] in restish.json.
type ConfigReadMsg struct {
	Type    string `cbor:"type"`
	API     string `cbor:"api,omitempty"`
	Profile string `cbor:"profile,omitempty"`
	// Plugin is the plugin's short name (without the "restish-" prefix).
	// When set, the response includes PluginConfig populated from
	// restish.json's plugins[Plugin] entry.
	Plugin string `cbor:"plugin,omitempty"`
}

// ConfigReadResponseMsg is the host reply to a ConfigReadMsg.
// Auth secrets are intentionally excluded.
type ConfigReadResponseMsg struct {
	Type    string   `cbor:"type"`
	BaseURL string   `cbor:"base_url,omitempty"`
	Headers []string `cbor:"headers,omitempty"`
	Query   []string `cbor:"query,omitempty"`
	Error   string   `cbor:"error,omitempty"`
	// PluginConfig holds the parsed plugins[name] entry from restish.json,
	// or nil when no config is stored for this plugin.
	PluginConfig any `cbor:"plugin_config,omitempty"`
}

// PromptMsg asks the host to display a message and read one line from the
// user. When Hidden is true and stdin is a TTY, echo is suppressed (suitable
// for password entry).
type PromptMsg struct {
	Type    string `cbor:"type"`
	Message string `cbor:"message"`
	Hidden  bool   `cbor:"hidden,omitempty"`
}

// PromptResponseMsg is the host reply to a PromptMsg.
type PromptResponseMsg struct {
	Type  string `cbor:"type"`
	Value string `cbor:"value,omitempty"`
	Error string `cbor:"error,omitempty"`
}

// ConfirmMsg asks the host to display a message and read a yes/no answer.
type ConfirmMsg struct {
	Type    string `cbor:"type"`
	Message string `cbor:"message"`
}

// ConfirmResponseMsg is the host reply to a ConfirmMsg.
type ConfirmResponseMsg struct {
	Type  string `cbor:"type"`
	Value bool   `cbor:"value"`
	Error string `cbor:"error,omitempty"`
}

// ResponseMsg asks the host to format and display a response using the
// configured output formatter (same as a regular API response).
type ResponseMsg struct {
	Type    string            `cbor:"type"`
	Status  int               `cbor:"status,omitempty"`
	Headers map[string]string `cbor:"headers,omitempty"`
	Body    any               `cbor:"body,omitempty"`
}

// DoneMsg signals that the plugin has finished. ExitCode 0 is success.
type DoneMsg struct {
	Type     string `cbor:"type"`
	ExitCode int    `cbor:"exit_code,omitempty"`
}

// StdoutDataMsg sends a chunk of raw bytes to the host's stdout.
type StdoutDataMsg struct {
	Type string `cbor:"type"`
	Data []byte `cbor:"data"`
}

// StderrDataMsg sends a chunk of raw bytes to the host's stderr.
type StderrDataMsg struct {
	Type string `cbor:"type"`
	Data []byte `cbor:"data"`
}

// WarnMsg prints a warning line (prefixed with "warning: ") on the host's
// stderr.
type WarnMsg struct {
	Type string `cbor:"type"`
	Text string `cbor:"text"`
}

// StdinDataMsg carries a chunk of stdin bytes from the host to the plugin
// (passthrough_stdio mode).
type StdinDataMsg struct {
	Type string `cbor:"type"`
	Data []byte `cbor:"data"`
}

// StdinCloseMsg signals that the host's stdin has reached EOF.
type StdinCloseMsg struct {
	Type string `cbor:"type"`
}
