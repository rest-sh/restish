// Package plugin provides the public contract for writing Restish
// out-of-process plugins.
//
// Restish supports three plugin styles:
//
//   - hook plugins for one-shot auth, middleware, loader, and formatter hooks
//   - command plugins for top-level workflow commands
//   - TLS signer plugins for external mTLS signing
//
// Startup contract
//
// Plugin binaries are discovered as executables named "restish-<name>".
// Restish starts them with one of these startup flags:
//
//   - --rsh-plugin-manifest: write an unframed CBOR Manifest
//   - --rsh-plugin-commands: write an unframed CBOR command list for command plugins
//
// Most plugins should use HandleStartupFlags or Run instead of manually
// decoding startup flags.
//
// Runtime transport
//
// After startup, runtime protocol messages use length-prefixed CBOR framing:
//
//	[ 4-byte big-endian uint32 length ][ CBOR payload ]
//
// Use WriteMessage and ReadMessage for framed runtime messages.
// Use WriteManifest and WriteCommands for unframed startup responses.
//
// Command-plugin messages
//
// Command plugins should prefer the typed message structs in messages.go:
//
//   - InitMsg for the initial host -> plugin command selection
//   - HTTPRequestMsg / HTTPResponseMsg for delegated HTTP
//   - APISpecMsg / APISpecResponseMsg for fetching registered API specs
//   - ListAPIsMsg / ListProfilesMsg for config discovery
//   - ConfigReadMsg for effective API/profile/plugin config
//   - PromptMsg / ConfirmMsg for user interaction
//   - ResponseMsg, StdoutDataMsg, StderrDataMsg, WarnMsg, and DoneMsg for output
//
// For simple command plugins, plugin.Run and CommandClient are usually enough.
//
// Hook-plugin payload shapes
//
// Hook plugins currently receive one framed CBOR map and, except for formatter
// hooks, return one framed CBOR reply map.
//
//   - auth:
//     request contains api_name, profile_name, params, and request metadata
//     reply typically returns request.header updates
//   - request-middleware:
//     request contains the outbound request metadata
//     reply can return updated request headers
//   - response-middleware:
//     request contains original request metadata plus normalized response fields
//     reply may set drop, follow, or response
//   - loader:
//     request contains content_type and raw body bytes
//     reply returns an OpenAPI document in body plus optional content_type
//   - formatter:
//     request contains format, color, and normalized response fields
//     plugin writes final formatted bytes directly to stdout
//
// See docs/plugin-quickstart.md for the fastest practical path to a working
// plugin, and docs/design/019-hook-plugins.md plus docs/design/020-command-plugins.md
// for the full protocol details.
package plugin
