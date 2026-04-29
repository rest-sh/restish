# Implementation Contract

## Summary

This record captures the current implementation-level contract that cuts across
the narrower subsystem records. A clean-room implementation should be able to
reconstruct global flags, config shape, command precedence, plugin protocol
families, and output ownership from this document plus the specialized records.

## Global Flags And Environment

Command-line flags override environment variables. Environment variables
override built-in defaults only when the matching flag was not set.

| Flag | Short | Type | Env | Default | Notes |
| --- | --- | --- | --- | --- | --- |
| `--rsh-header` | `-H` | repeat `Name: Value` | `RSH_HEADER` | empty | Env is comma-separated and prepended. |
| `--rsh-query` | `-q` | repeat `key=value` | `RSH_QUERY` | empty | Env is comma-separated and prepended. |
| `--rsh-server` | `-s` | string | | empty | Overrides scheme/host; path prefixes request path. |
| `--rsh-output-format` | `-o` | string | `RSH_OUTPUT_FORMAT` | auto | TTY readable; non-TTY JSON for structured output. |
| `--rsh-silent` | `-S` | bool | | false | Suppress output. |
| `--rsh-columns` | | string | | empty | Table columns. |
| `--rsh-sort-by` | | string | | empty | Table sort column. |
| `--rsh-content-type` | `-c` | string | | empty | Empty means JSON default for bodies unless operation media type applies. |
| `--rsh-filter` | `-f` | string | `RSH_FILTER` | empty | Shorthand/jq auto-detected. |
| `--rsh-filter-lang` | | string | | auto | `shorthand` or `jq`. |
| `--rsh-headers` | | bool | | false | Shorthand for `-f headers`. |
| `--rsh-raw` | `-r` | bool | | false | Raw original body or shell-friendly filtered scalars. |
| `--rsh-verbose` | `-v` | count | | 0 | `-v` headers, `-vv` TLS details. |
| `--rsh-insecure` | | bool | `RSH_INSECURE` | false | Warns, then disables TLS verification. |
| `--rsh-client-cert` | | string | | empty | mTLS cert. |
| `--rsh-client-key` | | string | | empty | mTLS key. |
| `--rsh-tls-signer` | | string | | empty | TLS signer plugin name/path. |
| `--rsh-tls-signer-param` | | repeat `key=value` | | empty | Plugin params. |
| `--rsh-ca-cert` | | string | | empty | Extra trusted CA. |
| `--rsh-tls-min-version` | | string | | empty | `TLS1.2` or `TLS1.3`. |
| `--rsh-ignore-status-code` | | bool | | false | Suppresses status-derived non-zero exit. |
| `--rsh-timeout` | `-t` | duration | `RSH_TIMEOUT` | none | Header wait timeout. |
| `--rsh-profile` | `-p` | string | `RSH_PROFILE` | `default` | Active API profile. |
| `--rsh-no-cache` | | bool | `RSH_NO_CACHE` | false | Bypass reads and writes. |
| `--rsh-no-browser` | | bool | | false | OAuth auth-code browser suppression. |
| `--rsh-retry` | | int | `RSH_RETRY` | 2 | `0` disables retries. |
| `--rsh-max-events` | | int | | 0 | Streaming event/line cap. |
| `--rsh-no-paginate` | | bool | | false | Disable automatic pagination. |
| `--rsh-collect` | | bool | | false | Collect pages before filtering. |
| `--rsh-max-pages` | | int | | 25 | `0` means unlimited. |
| `--rsh-max-items` | | int | | 0 | `0` means unlimited. |
| `--rsh-max-body-size` | | int MiB | | formatter default | Bounded response cap. |
| `--rsh-config` | | string path | `RSH_CONFIG` | platform default | Selects one complete config file. Missing explicit files error. |

Config file location precedence is `--rsh-config`, `RSH_CONFIG`,
`RSH_CONFIG_DIR/restish.json`, then the platform default config directory.
`--rsh-config` and `RSH_CONFIG` are source-of-truth selectors: Restish does not
merge them with the default config. Token and external-tool approval sidecars
live next to the selected explicit config. HTTP response and spec caches stay
under the cache root, with a namespace derived from the explicit config path.

## Config Schema

Top-level config is JSONC with strict decoded fields:

| Path | Type | Meaning |
| --- | --- | --- |
| `apis` | map | API registrations keyed by short name. |
| `cache.max_size` | string | Disk cache size such as `100MB`. |
| `theme` | map | Readable-output style entries. |
| `plugins` | map | Raw per-plugin JSON config. |

API fields:

| Field | Type | Meaning |
| --- | --- | --- |
| `base_url` | string | Default API URL prefix. |
| `spec_url` | string | Explicit spec URL. |
| `spec_files` | array | Ordered local/remote specs to merge. |
| `allow_cross_origin_spec` | bool | Permit safe cross-origin Link spec discovery. |
| `operation_base` | string | Absolute path prefix resolved against `base_url` for generated operations. |
| `server_variables` | map | Explicit OpenAPI server URL variable values used for generated operation paths. |
| `pagination.items_path` | string | Item extraction path. |
| `pagination.next_path` | string | Next URL extraction path. |
| `profiles` | map | Profile configs keyed by name. |

Profile fields are `base_url`, `headers`, `query`, `tls_signer`,
`tls_signer_params`, `server_variables`, and `auth`. Profile server variables
override API-level server variables for command generation. Auth fields are
`type` plus string `params`. Config files are written private and insecure
permissions are warning-only, not fatal.

## Command Surface And Precedence

Built-ins own: `get`, `head`, `options`, `post`, `put`, `patch`, `delete`,
`api`, `cache`, `cert`, `edit`, `links`, `plugin`, `setup`,
`theme`, `completion`, and `help`.

Generated API commands are registered under API short names when cached spec
metadata is available. Short-name GET fallback commands are registered for APIs
without generated command groups. Plugin commands are top-level commands but
must not collide with built-ins, generated APIs, configured API names, or other
plugin commands.

Bare URLs and registered API short names at root are treated as GET requests.
Generated command startup uses a fast path that skips value-taking global flags
but does not consume bool/count flags such as `-v` or `--rsh-insecure`.

## Plugin Wire Protocol Families

All plugin messages use CBOR. The stable message families are:

| Family | Direction | Purpose |
| --- | --- | --- |
| Manifest/startup flags | host -> plugin process | Discover hooks, loaders, formatters, and commands. |
| Hook messages | host <-> short-lived plugin | Auth, request middleware, response middleware, loader, formatter hooks. |
| Command messages | host <-> long-lived plugin | Init, stdin, HTTP delegation, formatting delegation, stderr, done. |
| Config messages | plugin -> host | Read/list config and prompt/confirm where allowed. |
| Formatter messages | host <-> formatter plugin | Normalize host response and stream or document formatting. |
| TLS signer messages | host <-> signer plugin | Certificate discovery and signing for mTLS. |

Protocol changes that alter message meaning require a plugin API version bump or
explicit compatibility handling.

## Output Ownership

Design 009 owns the normalized response schema and bounded response formatting
contract. Design 028 owns the planner that decides document vs record framing
across pagination, streaming, filters, and explicit formats.

| Concern | Owner |
| --- | --- |
| Decode body and preserve raw bytes | 009 |
| Normalize status, headers, links, body | 009 |
| Select default formatter for TTY/non-TTY | 009 and 028 |
| Decide document vs record execution | 028 |
| Paginated collection vs streaming behavior | 028 and 011 |
| SSE/NDJSON event rendering | 028 and 012 |

## Intentional v2 Breaks

The v1 interactive `api configure <name>` prompt flow is retired. v2 config is
edited through `restish.json`, `api add`, `api set`, and `api edit`.
Legacy `x-cli-config.prompt` is not retired: `api configure <name> <url>`
prompts for those values while writing local config, then normal requests use
the saved config without extension-driven prompting.

The `restish-mcp --http` flag is not part of v2; MCP currently uses stdio as a
command plugin.
