# Restish — Design Document

> **Status:** Draft — feature inventory of v1 as the baseline for v2 design.
>
> This document catalogs every feature of Restish v1 (`rest-sh/restish`) as a
> factual baseline. No v2 decisions are recorded here yet; those will be a
> separate iteration once the inventory is agreed on.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [CLI Structure & Commands](#3-cli-structure--commands)
4. [API Discovery & Spec Loading](#4-api-discovery--spec-loading)
5. [OpenAPI Extensions](#5-openapi-extensions)
6. [Authentication](#6-authentication)
7. [API Configuration](#7-api-configuration)
8. [Global Configuration](#8-global-configuration)
9. [Request Handling](#9-request-handling)
10. [Content Types & Encodings](#10-content-types--encodings)
11. [Output Formatting](#11-output-formatting)
12. [Response Structure](#12-response-structure)
13. [Filtering & Projection](#13-filtering--projection)
14. [CLI Shorthand (Input Language)](#14-cli-shorthand-input-language)
15. [Hypermedia](#15-hypermedia)
16. [Caching](#16-caching)
17. [Retries & Timeouts](#17-retries--timeouts)
18. [Edit Command](#18-edit-command)
19. [Bulk Resource Management](#19-bulk-resource-management)
20. [Shell Completion](#20-shell-completion)
21. [TLS / Certificate Handling](#21-tls--certificate-handling)
22. [Exit Codes](#22-exit-codes)
23. [Extensibility (Library API)](#23-extensibility-library-api)
24. [Miscellaneous Utilities](#24-miscellaneous-utilities)
25. [Installation & Distribution](#25-installation--distribution)
26. [Dependencies (Key Libraries)](#26-dependencies-key-libraries)

---

## 1. Overview

Restish is a CLI for interacting with REST-ish HTTP APIs. Its central
philosophy is that every API deserves a CLI and that the interface should be
defined by the server — so a registered API always reflects the latest
operations and schemas without requiring a client update.

**Core value propositions:**

- Generic HTTP verbs for quick one-off requests (like curl / HTTPie).
- Generated, documented, shell-completed commands for registered APIs (via
  OpenAPI 3).
- Always up-to-date: API descriptions are fetched and cached automatically.
- First-class hypermedia support for navigating link-driven APIs.
- Designed to be embeddable as a Go library so organizations can ship custom
  CLIs.

## How To Use This Baseline

This document is intentionally inventory-oriented. It exists to answer "what did
v1 do?" rather than "what should v2 do?"

Use it in combination with the v2 records as follows:

- consult this file when preserving or intentionally breaking a v1 behavior
- consult the v2 records when you need the normative design for the new runtime
- record intentional v1 incompatibilities in design 031 rather than editing this
  baseline to match v2

For a reimplementation effort, this document is most useful as a coverage
checklist: it helps confirm that the v2 design corpus either preserves,
replaces, or intentionally retires each major v1 capability.

## Baseline To v2 Crosswalk

The mapping below helps contributors jump from a v1 feature area into the
record that now defines the v2 design:

| v1 Area | Primary v2 design record |
| ------- | ------------------------ |
| CLI runtime and lifecycle | `001-cli-architecture.md` |
| Config, profiles, and persistence | `002-config-and-profiles.md` |
| Content types and encodings | `003-content-types-and-encodings.md` |
| Authentication | `004-authentication.md` |
| TLS, mTLS, and cert inspection | `005-tls-and-cert-handling.md` |
| Spec discovery and loading | `006-spec-discovery-and-loading.md` |
| Generated API commands | `007-api-command-generation.md` |
| Shorthand request input | `008-shorthand-input.md` |
| Response model and output | `009-response-normalization-and-output.md` |
| Filtering and projection | `010-filtering-and-projection.md` |
| Hypermedia and pagination | `011-pagination-and-hypermedia.md` |
| Streaming | `012-streaming.md` |
| Caching and retries | `013-caching-and-retries.md` |
| Edit workflow | `014-edit-workflow.md` |
| `links` command | `015-links-command.md` |
| Shell setup and completions | `016-setup-and-completions.md` |
| CLI behavior, diagnostics, exit rules | `017-cli-behavior-and-diagnostics.md` |
| Plugin architecture and concrete plugins | `018` through `027` |
| Output framing | `028-document-and-record-output.md` |
| End-to-end request pipeline | `029-request-execution-pipeline.md` |
| Security model | `030-security-model-and-trust-boundaries.md` |
| Compatibility and migration | `031-compatibility-and-migration.md` |

Where the v2 design intentionally changes behavior, the crosswalk should be
read together with design 031 rather than assuming direct one-to-one parity.

---

## 2. Architecture

### Package layout (v1)

| Package   | Role                                                                                                                           |
| --------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `main`    | Wires up the binary: calls `cli.Init`, `cli.Defaults`, registers loaders and auth handlers, calls `cli.Run`.                   |
| `cli`     | Core framework: command tree, request/response pipeline, formatters, config, caching, shorthand, content types, auth registry. |
| `openapi` | OpenAPI 3.0/3.1 loader: detects, fetches, parses specs; maps paths/operations/parameters to `cli.Operation` structs.           |
| `oauth`   | OAuth 2.0 auth handlers (client credentials, authorization code + PKCE).                                                       |
| `bulk`    | Git-like bulk resource management subcommand.                                                                                  |

### Runtime flow

```
Args → GlobalFlags pre-parse → API config lookup → lazy API spec load
     → cobra command dispatch → request build → auth → HTTP transport
     → cache → retry → response decode → filter → format → output
```

### Extensibility points

The `cli` package exposes a registry-based plugin model. Callers (including
third-party tool authors) can register:

- `AddLoader` — custom API spec loaders
- `AddAuth` — custom authentication handlers
- `AddContentType` — custom request/response body marshallers
- `AddEncoding` — custom content-transfer-encodings (compression)
- `AddLinkParser` — custom hypermedia link parsers
- `AddGlobalFlag` — additional persistent flags

---

## 3. CLI Structure & Commands

### Generic HTTP commands

Available without any API registration. Default verb is GET; `https://` is
assumed if no scheme is given.

| Command     | Aliases   | Description                           |
| ----------- | --------- | ------------------------------------- |
| `<default>` | —         | HTTP GET (bare URL as first argument) |
| `get`       | `GET`     | Explicit GET                          |
| `head`      | `HEAD`    | HTTP HEAD                             |
| `options`   | `OPTIONS` | HTTP OPTIONS                          |
| `post`      | `POST`    | HTTP POST (accepts body)              |
| `put`       | `PUT`     | HTTP PUT (accepts body)               |
| `patch`     | `PATCH`   | HTTP PATCH (accepts body)             |
| `delete`    | `DELETE`  | HTTP DELETE (accepts optional body)   |

### `edit` command

Convenience command combining GET + edit + PUT.

```
restish edit <uri> [-i] [-y] [-e json|yaml] [shorthand-patches...]
```

- `-i` / `--rsh-interactive` — opens resource in `$VISUAL` / `$EDITOR`
- `-y` / `--rsh-yes` — skip confirmation prompt
- `-e` / `--rsh-edit-format` — format to use when editing (`json` or `yaml`,
  default `json`)
- Applies conditional request headers (`If-Match` from `ETag`, etc.)
- If the resource carries a `$schema`, the editor gets schema-based completion
  and hover docs

### `links` command

```
restish links <uri> [rel1 rel2...]
```

Performs a GET, normalizes all hypermedia links, and prints them. Optional
additional arguments filter to specific link relation names.

### `auth-header` command

```
restish auth-header <api-name-or-uri>
```

Returns an `Authorization` header value (e.g. `Bearer <token>`) for an
API, using cached tokens and refreshing as needed. Intended for use in shell
pipelines with other tools (e.g. curl).

### `cert` command

```
restish cert <uri>
```

Connects via TLS and displays certificate details: issuer, subject, signature
algorithm, validity window (including relative "expires in N days"), and DNS
names.

### `api` management commands

| Subcommand                    | Description                                                                                      |
| ----------------------------- | ------------------------------------------------------------------------------------------------ |
| `api configure <name> [url]`  | Interactive TUI to create or update an API registration (profiles, headers, query params, auth). |
| `api show <name>`             | Print the API config as JSON or YAML.                                                            |
| `api edit`                    | Open `apis.json` in `$VISUAL` / `$EDITOR`.                                                       |
| `api sync <name>`             | Force-refresh the cached API spec.                                                               |
| `api clear-auth-cache <name>` | Clear the cached OAuth token for the current profile.                                            |
| `api content-types`           | List registered content types and output formats.                                                |

### Generated API commands

When an API is registered and its spec is loaded, each OpenAPI operation
becomes a subcommand:

```
restish <api-name> <operation-id-in-kebab-case> [path-params...] [--option=val...] [body-shorthand...]
```

- Required parameters → positional arguments
- Optional parameters → `--flag` options
- Request body → remaining args (shorthand) or stdin
- Operations are grouped if they have OpenAPI tags
- Deprecated operations show a deprecation notice in help
- Hidden operations (`x-cli-hidden`) omitted from standard help list but
  accessible directly
- API short name can be used in place of the domain in bare URLs:
  `restish myapi/resources/123`

---

## 4. API Discovery & Spec Loading

### Auto-discovery sequence

When a registered API is loaded, Restish:

1. Checks for a valid (non-expired) CBOR cache file.
2. If `spec_files` is configured, loads from those local files or URLs
   (skips HTTP discovery).
3. Otherwise, makes a GET to the API base URL and inspects link relations:
   - `service-desc` (RFC 8631)
   - `describedby` (RFC 5988)
4. Falls back to well-known paths: `/openapi.json`, `/openapi.yaml`.
5. Tries the base URL itself.
6. For each candidate URL, fetches and runs each registered loader's
   `Detect()` method.

### Supported spec formats

| Format  | Version | Status          |
| ------- | ------- | --------------- |
| Swagger | 2.0     | Not supported   |
| OpenAPI | 3.0     | Fully supported |
| OpenAPI | 3.1     | Fully supported |

Parser: `pb33f/libopenapi`

### Spec caching

- Parsed API is serialized to CBOR and stored in
  `<cache-dir>/<api-name>.cbor`.
- Default TTL: 24 hours (applied if the spec response has no cache headers).
- Cache invalidated when Restish version changes (`restish_version` field
  compared).
- `api sync <name>` force-invalidates.

### Multi-file merging

`spec_files` accepts multiple paths/URLs. Operations are merged in order;
the first non-empty title/description wins.

---

## 5. OpenAPI Extensions

Extensions are `x-` properties that customize CLI behavior.

| Extension           | Applies to                 | Description                                                  |
| ------------------- | -------------------------- | ------------------------------------------------------------ |
| `x-cli-name`        | info, path, parameter      | Override the CLI name (used for command or flag name).       |
| `x-cli-aliases`     | operation                  | Additional command aliases.                                  |
| `x-cli-description` | path, operation, parameter | Override description shown in CLI help.                      |
| `x-cli-ignore`      | path, operation, parameter | Exclude from the generated CLI entirely.                     |
| `x-cli-hidden`      | path, operation            | Hide from `--help` list but still accessible.                |
| `x-cli-config`      | top-level document         | AutoConfiguration: tell clients how to set up auth profiles. |

### `x-cli-config` (AutoConfiguration)

Enables the API to self-describe its CLI configuration:

```yaml
x-cli-config:
  security: <scheme-name-or-type> # references securitySchemes or uses built-in type name
  headers:
    key: value # persistent headers to pre-configure
  prompt:
    <var-name>:
      description: ...
      example: ...
      exclude: true # if true, used only for template expansion, not sent to server
  params:
    <param-name>: "literal or {template}" # values built from prompted vars
```

Supported built-in security type values: `http-basic`,
`oauth-client-credentials`, `oauth-authorization-code`.

When a user runs `restish api configure`, the API's `x-cli-config` drives
the interactive prompts, reducing setup friction.

### Operation name derivation

- `operationId` is converted to `kebab-case`.
- `x-cli-name` overrides the result.
- Parameter names are similarly kebab-cased for flag/arg names.

### Grouping

OpenAPI tags become command groups in the help output.

---

## 6. Authentication

All auth handlers implement the `AuthHandler` interface:

```go
type AuthHandler interface {
    Parameters() []AuthParam
    OnRequest(req *http.Request, key string, params map[string]string) error
}
```

### HTTP Basic (`http-basic`)

Sends `Authorization: Basic <base64(user:pass)>`. If `password` is omitted
from config, the user is prompted at runtime (terminal prompt, no echo).

Parameters: `username` (required), `password` (optional).

### API Key

Not a separate auth type. API keys are handled by setting a persistent header
or query parameter in the profile config. Example:
`"Authorization": "Bearer <token>"`.

### OAuth 2.0 Client Credentials (`oauth-client-credentials`)

Machine-to-machine flow (RFC 6749). Fetches a bearer token from `token_url`
using `client_id` + `client_secret`. Token is cached; re-fetched on expiry.

Parameters: `client_id`, `client_secret`, `token_url`, `scopes` (optional),
`audience` (optional). Any additional params are forwarded to the token
endpoint.

### OAuth 2.0 Authorization Code + PKCE (`oauth-authorization-code`)

User-login flow (RFC 6749 + RFC 7636). Starts a local HTTP server on
port `8484` to receive the redirect. Opens the browser for login. Supports
refresh tokens (if `offline_access` scope or equivalent is requested).

Parameters: `client_id`, `authorize_url`, `token_url`, `scopes` (optional),
`audience` (optional), `client_secret` (optional).

If the machine cannot open a browser, the user can authenticate on another
machine and paste back the resulting token.

### External Tool (`external-tool`)

Delegates auth to an arbitrary shell command. Restish serializes the
outgoing request as JSON on stdin; the tool writes modified headers (and
optionally a replacement URI) as JSON on stdout.

Input schema:

```json
{
  "method": "GET",
  "uri": "https://...",
  "headers": {"name": ["value"]},
  "body": "..."
}
```

Output schema (only `headers` and `uri` are processed):

```json
{"headers": {"authorization": ["Bearer ..."]}, "uri": "https://..."}
```

Parameters: `commandline` (required), `omitbody` (optional — omits `body`
from stdin payload).

### PKCS#11 Hardware Certificates

Per-profile TLS config supports hardware certificate retrieval via PKCS#11
(`ThalesIgnite/crypto11`). Configured under `tls.pkcs11`:
`path` (optional shared object path), `label` (required certificate label).

---

## 7. API Configuration

### File locations

| OS      | Path                                              |
| ------- | ------------------------------------------------- |
| macOS   | `~/Library/Application Support/restish/apis.json` |
| Windows | `%AppData%\restish\apis.json`                     |
| Linux   | `~/.config/restish/apis.json`                     |

Override with `RESTISH_CONFIG_DIR` environment variable.

### `apis.json` structure

```json
{
  "$schema": "https://rest.sh/schemas/apis.json",
  "<api-name>": {
    "base": "https://api.example.com",
    "operation_base": "/",
    "spec_files": ["/path/to/spec.yaml", "https://example.com/openapi.json"],
    "tls": {
      "insecure": false,
      "cert": "/path/to/cert.pem",
      "key": "/path/to/key.pem",
      "ca_cert": "/path/to/ca.pem",
      "pkcs11": { "path": "/lib/pkcs11.so", "label": "my-cert" }
    },
    "profiles": {
      "default": {
        "base": "https://staging.example.com",
        "headers": { "X-API-Version": "2024-01" },
        "query": { "format": "json" },
        "auth": {
          "name": "oauth-authorization-code",
          "params": { "client_id": "...", "authorize_url": "...", "token_url": "..." }
        }
      },
      "prod": { "..." }
    }
  }
}
```

### Named profiles

Each API can have multiple named profiles (default is `default`). Selected
with `-p <name>` or `RSH_PROFILE`. Profiles can have their own:

- `base` URL override
- `headers` (persistent, sent on every request)
- `query` (persistent query params)
- `auth` configuration

### `operation_base`

Normally operations are relative to `base`. `operation_base` overrides the
path component, useful when an API is served at a sub-path but spec paths are
absolute.

---

## 8. Global Configuration

### Precedence (high → low)

1. CLI flags
2. Environment variables (`RSH_*`)
3. Config file

### Config file locations

| OS      | Path                                                |
| ------- | --------------------------------------------------- |
| macOS   | `~/Library/Application Support/restish/config.json` |
| Windows | `%AppData%\restish\config.json`                     |
| Linux   | `~/.config/restish/config.json`                     |

Also checks `/etc/restish/config.json` and `~/.restish/config.json` (legacy).

### All global flags

| Flag                        | Env var                  | Default   | Description                             |
| --------------------------- | ------------------------ | --------- | --------------------------------------- |
| `-v`, `--rsh-verbose`       | `RSH_VERBOSE`            | false     | Verbose debug output                    |
| `-o`, `--rsh-output-format` | `RSH_OUTPUT_FORMAT`      | `auto`    | Output format                           |
| `-f`, `--rsh-filter`        | `RSH_FILTER`             | —         | Shorthand query filter/projection       |
| `-r`, `--rsh-raw`           | `RSH_RAW`                | false     | Raw string output (strip JSON quotes)   |
| `-s`, `--rsh-server`        | `RSH_SERVER`             | —         | Override scheme://host for requests     |
| `-H`, `--rsh-header`        | `RSH_HEADER`             | —         | Set a request header (repeatable)       |
| `-q`, `--rsh-query`         | `RSH_QUERY`              | —         | Set a query parameter (repeatable)      |
| `-p`, `--rsh-profile`       | `RSH_PROFILE`            | `default` | Auth profile                            |
| `-t`, `--rsh-timeout`       | `RSH_TIMEOUT`            | 0 (none)  | Request timeout (duration, e.g. `30s`)  |
| `--rsh-no-paginate`         | `RSH_NO_PAGINATE`        | false     | Disable auto-pagination                 |
| `--rsh-no-cache`            | `RSH_NO_CACHE`           | false     | Disable cache read (still writes)       |
| `--rsh-insecure`            | `RSH_INSECURE`           | false     | Disable TLS certificate verification    |
| `--rsh-client-cert`         | `RSH_CLIENT_CERT`        | —         | PEM client certificate path             |
| `--rsh-client-key`          | `RSH_CLIENT_KEY`         | —         | PEM private key path                    |
| `--rsh-ca-cert`             | `RSH_CA_CERT`            | —         | PEM CA certificate path                 |
| `--rsh-retry`               | `RSH_RETRY`              | 2         | Number of retries for retriable errors  |
| `--rsh-ignore-status-code`  | `RSH_IGNORE_STATUS_CODE` | false     | Always exit 0 regardless of HTTP status |

Color control: `COLOR=1` forces color on; `NOCOLOR=1` forces color off.

---

## 9. Request Handling

### Protocol

- HTTP/2 with TLS by default; falls back to HTTP/1.1.
- `https://` is assumed when no scheme is provided.
- Bare `:8080/path` syntax is expanded to `http://localhost:8080/path`.

### Headers & query params

- Per-request: `-H Name:Value` and `-q key=value` flags (repeatable).
- Persistent: configured in API profile `headers` / `query` maps.
- Environment: `RSH_HEADER=name1:val1,name2:val2`, `RSH_QUERY=k=v`.

### Request body

Three input modes, combinable:

1. **Stdin redirect** — any data piped/redirected to stdin is sent as-is.
2. **CLI Shorthand** — positional arguments after the URL are parsed as
   shorthand and merged into a JSON body.
3. **Combined** — stdin provides a template; shorthand args override/patch
   specific fields.

Default `Content-Type` when body is present: `application/json`.

### Content negotiation

Restish sends an `Accept` header listing all registered content types ordered
by quality factor (`q` value). This lets servers choose the best supported
format automatically.

---

## 10. Content Types & Encodings

### Request/response body formats

| Short name | MIME type             | q (priority) | Notes                            |
| ---------- | --------------------- | ------------ | -------------------------------- |
| `cbor`     | `application/cbor`    | 0.9          | Binary; preferred for efficiency |
| `msgpack`  | `application/msgpack` | 0.8          | Binary                           |
| `ion`      | `application/ion`     | 0.6          | Amazon Ion                       |
| `json`     | `application/json`    | 0.5          |                                  |
| `yaml`     | `application/yaml`    | 0.5          |                                  |
| `text`     | `text/*`              | 0.2          | Plain text (passthrough)         |

Output-only formats (not in `Accept`):

| Short name | Description                                        |
| ---------- | -------------------------------------------------- |
| `readable` | Human-readable custom format (default interactive) |
| `table`    | Tabular (for arrays of objects)                    |
| `gron`     | Grep-friendly path=value format                    |

### Content-transfer encodings (compression)

| Name      | RFC / standard    |
| --------- | ----------------- |
| `gzip`    | RFC 1952          |
| `deflate` | RFC 1951          |
| `br`      | RFC 7932 (Brotli) |

Decompressed transparently on response; Restish sets `Accept-Encoding`
accordingly.

---

## 11. Output Formatting

### Auto-detection

```
Is stdout a TTY?
  Yes → interactive defaults: color on, full response, readable format
  No  → pipe/file defaults: color off, body only, JSON format
```

Overrides: `COLOR=1` / `NOCOLOR=1`; `-o <format>`; `-f <filter>`.

### `readable` format

Custom human-oriented format resembling YAML+JSON. Features:

- No quotes on object keys.
- No trailing commas.
- Special rendering for: `null`, booleans, numbers (scientific notation),
  strings, ISO 8601 dates and datetimes, binary data as `0xdeadbeef...` hex.
- Special string coloring for URLs and dates.
- Full HTTP response printed first (status line, headers), then body.
- Syntax highlighted with a custom `cli-dark` chroma theme including
  bracket-depth colorization.

### `table` format

Renders arrays of objects as a Unicode box-drawing table. Column headers are
derived from object keys. Requires the response (or filtered result) to be
an array of objects.

### `gron` format

Each leaf value is printed as `path = value;` on its own line, JavaScript
property-access style. Enables precise `grep` over large responses and
easy discovery of paths to use with `-f`.

### `json` / `yaml` output

Marshals the full normalized response structure (proto, status, headers,
links, body) — or the filter result — as JSON or YAML. Used by default in
non-interactive mode.

### `cbor` output

Writes CBOR-encoded body to stdout; useful for saving binary-efficient
representations.

### Image preview

If the response body is an image (JPEG, PNG, GIF, WebP, HEIC, etc.), it is
rendered in the terminal as Unicode half-block characters in true color mode
via `pixterm` / `ansimage`.

### Markdown in help

Operation and parameter descriptions from OpenAPI are rendered as styled
Markdown in help output via `charmbracelet/glamour` (only when stdout is a
TTY; word-wrapped to terminal width).

---

## 12. Response Structure

All responses are normalized to the following internal structure before
formatting/filtering:

```json
{
  "proto": "HTTP/2.0",
  "status": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "links": {
    "next": [{ "rel": "next", "uri": "https://api.example.com/items?cursor=abc" }]
  },
  "body": { ... }
}
```

- `headers` — canonicalized (title-cased).
- `links` — normalized from all registered hypermedia parsers; URIs fully
  resolved to absolute form.
- `body` — parsed from the wire format into a Go value (map/slice/scalar).

This structure is the input to filter expressions and non-readable formatters.

In interactive mode, the full structure is displayed. In non-interactive
(redirected) mode, only `body` is output by default.

---

## 13. Filtering & Projection

### Flag

`-f <query>` / `--rsh-filter <query>` (also `RSH_FILTER`).

Applied after the response is decoded. The full response structure
(including `proto`, `status`, `headers`, `links`, `body`) is the root.

### Query language

Uses `danielgtaylor/shorthand` v2 query syntax (similar to JMESPath / jq):

| Feature                     | Syntax example                 |
| --------------------------- | ------------------------------ |
| Property access             | `body.user.name`               |
| Array index                 | `body.items[0]`                |
| Negative index              | `body.items[-1]`               |
| Array slicing               | `body.items[1:3]`              |
| Wildcard property           | `body.*.id`                    |
| Filter/predicate            | `body.items[status == active]` |
| Object projection           | `body.{id, name: user.name}`   |
| Recursive descent           | `body..url`                    |
| Pipe (stop further descent) | `body.items\|[0]`              |
| Array flatten               | `body.items[].tags[]`          |

Filter expressions (inside `[...]`) use `danielgtaylor/mexpr` — a small
expression language supporting comparisons, `contains`, `startsWith`,
`where`, `after`, etc.

### Raw mode

`-r` / `--rsh-raw` strips JSON string quotes from the filter result when the
result is:

- A string → printed without quotes.
- An array of scalars → one value per line, no quotes.

Useful for shell scripting where you want to loop over IDs without parsing.

---

## 14. CLI Shorthand (Input Language)

Restish uses `danielgtaylor/shorthand` v2 as a JSON superset for constructing
request bodies from CLI arguments.

### Core features

| Feature                   | Syntax                                          |
| ------------------------- | ----------------------------------------------- |
| Key-value pair            | `name: Alice`                                   |
| Nested object (dot)       | `user.address.city: NYC`                        |
| Nested object (braces)    | `user{name: Alice, age: 30}`                    |
| Array literal             | `[1, 2, 3]`                                     |
| Array append              | `tags[]: admin`                                 |
| Array index set           | `items[0]: foo`                                 |
| Array insert before       | `items[^0]: first`                              |
| Remove field              | `id: undefined`                                 |
| Move / swap               | `id ^ name` (rename), `arr[0] ^ arr[-1]` (swap) |
| Load from file            | `data: @file.json`, `icon: @logo.png`           |
| Base64 bytes              | `payload: %wg==`                                |
| ISO 8601 datetime         | `created: 2024-01-01T00:00:00Z`                 |
| Quoted string (no coerce) | `count: "42"`                                   |
| Multiple pairs            | `a: 1, b: 2` (comma-separated)                  |
| Comments                  | `// this is a comment`                          |

### Type coercion

Unquoted values are coerced automatically: `null`, `true`, `false` → their
types; integers and floats → numbers; ISO 8601 dates → time; `%...` →
bytes; everything else → string.

### Combined stdin + args

When stdin is provided along with shorthand args, the stdin JSON is parsed
first, then the args are applied as a patch on top. This enables templating:
multiple near-identical requests from one base template.

### Patch operations

Supports JSON Merge Patch semantics plus JSON Patch–style moves, making it
useful as a request body for HTTP PATCH endpoints. Suggested MIME type:
`application/merge-patch+shorthand`.

---

## 15. Hypermedia

### Link normalization

All link parsers produce a common internal link type:

```json
{"rel": "next", "uri": "https://api.example.com/items?cursor=abc"}
```

URIs are always resolved to absolute form against the request base.

### Registered parsers

| Parser                         | Standard / format                        |
| ------------------------------ | ---------------------------------------- |
| `LinkHeaderParser`             | HTTP `Link` header (RFC 5988)            |
| `HALParser`                    | HAL (`_links`)                           |
| `TerrificallySimpleJSONParser` | TSJ (`@id`, `@context`)                  |
| `JSONAPIParser`                | JSON:API (`links`, `data.relationships`) |

Note: Siren is listed in the README feature list but its parser was not
found in the registered defaults in `cli.Defaults()` — may be an
omission or may have been removed. **Needs verification.**

### Automatic pagination

When a response contains a `next` link relation, Restish automatically fetches
subsequent pages and concatenates their body arrays into a single result.
Disabled with `--rsh-no-paginate`.

### `links` command

Fetches a URL and displays all normalized links, optionally filtered by
relation name. Useful for exploring hypermedia APIs.

---

## 16. Caching

### Response cache

- HTTP-level disk cache respecting RFC 7234 (`Cache-Control`, `Expires`).
- Implemented via `gbl08ma/httpcache` + `diskcache`.
- Location: `<cache-dir>/responses/`.
- `--rsh-no-cache` skips reading the cache (but still writes).

### API spec cache

- After loading and parsing an API spec, the result is serialized to CBOR
  and written to `<cache-dir>/<api-name>.cbor`.
- 24-hour TTL stored in `cache.json`; applied even when the server sends no
  cache headers.
- Invalidated on Restish version change.
- `api sync <name>` force-invalidates.

### Auth token cache

- OAuth tokens (access + optional refresh) stored in `cache.json` under
  `<api-name>:<profile>` key.
- `api clear-auth-cache <name>` clears the entry.

### Cache file locations

| OS      | Path                        |
| ------- | --------------------------- |
| macOS   | `~/Library/Caches/restish/` |
| Windows | `%LocalAppData%\restish\`   |
| Linux   | `~/.cache/restish/`         |

Override with `RESTISH_CACHE_DIR`.

### MinCachedTransport

A special transport variant that adds a minimum `max-age` to responses that
would otherwise not be cached (no `Cache-Control` or `Expires`). Used when
loading API spec documents.

---

## 17. Retries & Timeouts

### Automatic retries

Default retry count: **2**. Configurable via `--rsh-retry` / `RSH_RETRY`
(set to `0` to disable).

Retriable HTTP status codes:

| Code | Reason                |
| ---- | --------------------- |
| 408  | Request Timeout       |
| 425  | Too Early             |
| 429  | Too Many Requests     |
| 500  | Internal Server Error |
| 502  | Bad Gateway           |
| 503  | Service Unavailable   |
| 504  | Gateway Timeout       |

Client-side timeouts (when `--rsh-timeout` is set) are also retried.

### Retry delay

Default: 1 second between retries. Overridden by server-supplied headers:

- `Retry-After` (RFC 7231) — date or seconds
- `X-Retry-In` — seconds (used by Traefik rate limiting middleware)

### Request timeouts

`--rsh-timeout <duration>` / `RSH_TIMEOUT`. Duration format: Go syntax
(`30s`, `500ms`, `1m`). Default `0` = no timeout.

---

## 18. Edit Command

```
restish edit <uri> [-i] [-y] [-e json|yaml] [shorthand-patches...]
```

Workflow:

1. GET the resource.
2. Apply any shorthand patch arguments to the in-memory representation.
3. If `-i`: write to a temp file and open in `$VISUAL` / `$EDITOR`; wait
   for editor to close; read result back.
4. If not `-y`: show a diff and ask for confirmation.
5. PUT the modified resource back.

### Conditional requests

If the GET response includes any of the following headers, the corresponding
conditional header is included in the PUT:

| GET response header | PUT request header    |
| ------------------- | --------------------- |
| `ETag`              | `If-Match`            |
| `Last-Modified`     | `If-Unmodified-Since` |

Prevents overwriting concurrent changes (optimistic concurrency).

### Schema-driven editor experience

If the resource body includes a `$schema` property (a URL to a JSON Schema),
editors that support JSON Schema (e.g. VS Code) will provide hover docs,
completion, and lint while the user edits.

---

## 19. Bulk Resource Management

A git-like interface for managing collections of API resources as local
files. Package `bulk`, exposed under `restish bulk` (aliased `rb` in docs).

### Prerequisites

The API must support:

- A list endpoint returning items with a URL/link and a version/etag field.
- GET + PUT + DELETE on individual resources.
- Conditional updates (ETag or Last-Modified).

### Auto-recognized list item fields

| Purpose        | Recognized field names                                         |
| -------------- | -------------------------------------------------------------- |
| Resource URL   | `url`, `uri`, `self`, `link`                                   |
| Version / etag | `version`, `etag`, `last_modified`, `lastModified`, `modified` |

### Commands

| Command           | Alias | Description                                                                                                                                                                                                                     |
| ----------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bulk init <url>` | `i`   | Fetch all resources from list URL and save locally as JSON files. Options: `-f` (shorthand filter to extract url/version from non-standard responses), `--url-template` (template to build resource URL from list item fields). |
| `bulk list`       | `ls`  | List checked-out resource files. Options: `-m` / `--match` (mexpr filter), `-f` (shorthand query applied to each matched file).                                                                                                 |
| `bulk status`     | `st`  | Show local changes and remote changes since last pull.                                                                                                                                                                          |
| `bulk diff`       | `di`  | Show unified diff of local changes (or `--remote` for remote). Options: `-m` (match filter).                                                                                                                                    |
| `bulk reset`      | `re`  | Undo local changes to files. Options: `-m` (match filter).                                                                                                                                                                      |
| `bulk pull`       | `pl`  | Fetch and apply remote changes. Does not overwrite local changes.                                                                                                                                                               |
| `bulk push`       | `ps`  | Upload local changes (new, modified, deleted) to the server sequentially.                                                                                                                                                       |

### Schema-aware filtering

If resources advertise a JSON Schema (via `describedby` link or `$schema`
field), `bulk list --match` can emit warnings when expressions reference
incompatible field types.

### Resource creation

Resources are created by adding a new `.json` file to the checkout directory
and running `bulk push`. Requires client-generated IDs and PUT semantics
(not POST).

---

## 20. Shell Completion

Built on Cobra's completion framework. Supported shells:

- Bash (`restish completion bash`)
- Zsh (`restish completion zsh`)
- Fish (`restish completion fish`)
- PowerShell (`restish completion powershell`)

### Dynamic completions

- API names appear as completions for the root command.
- Once an API is selected, registered operations appear as completions.
- URL templates with partial matches are suggested with operation descriptions.
- Short-name URLs (`myapi/resources`) are completed with full template paths.
- `--rsh-output-format` suggests: `auto`, `json`, `yaml`.
- `--rsh-profile` suggests profile names from the current API config.

---

## 21. TLS / Certificate Handling

### Global flags

`--rsh-insecure`, `--rsh-client-cert`, `--rsh-client-key`, `--rsh-ca-cert`
(see §8).

### Per-API TLS config (`tls` object in `apis.json`)

```json
{
  "tls": {
    "insecure": false,
    "cert": "/path/to/cert.pem",
    "key": "/path/to/key.pem",
    "ca_cert": "/path/to/ca.pem",
    "pkcs11": {
      "path": "/usr/lib/opensc-pkcs11.so",
      "label": "my-yubikey-cert"
    }
  }
}
```

PKCS#11 support via `ThalesIgnite/crypto11` enables hardware security keys
and smart cards.

---

## 22. Exit Codes

| Code | Meaning                                               |
| ---- | ----------------------------------------------------- |
| `0`  | Success (2xx)                                         |
| `1`  | Unrecoverable error (panic, connection failure, etc.) |
| `3`  | 3xx HTTP response                                     |
| `4`  | 4xx HTTP response                                     |
| `5`  | 5xx HTTP response                                     |

`--rsh-ignore-status-code` / `RSH_IGNORE_STATUS_CODE=1` — always exit 0
for HTTP responses (errors still exit 1).

When a command makes multiple requests (e.g. paginated), the exit code
reflects the most recent HTTP status.

---

## 23. Extensibility (Library API)

Restish is designed to be imported as a Go library, allowing organizations
to build custom CLIs for their APIs.

### Registration functions

```go
cli.Init(name, version string)
cli.Defaults()                          // registers built-in encodings, types, parsers, auth
cli.AddLoader(loader Loader)            // custom API spec loaders
cli.AddAuth(name string, h AuthHandler) // custom auth schemes
cli.AddContentType(name, mime string, q float64, m Marshaller)
cli.AddEncoding(name string, e ContentEncoding)
cli.AddLinkParser(p LinkParser)
cli.AddGlobalFlag(name, short, desc string, def any, array bool)
```

### Interfaces

**`Loader`**

```go
type Loader interface {
    LocationHints() []string
    Detect(resp *http.Response) bool
    Load(entrypoint, spec url.URL, resp *http.Response) (API, error)
}
```

**`AuthHandler`**

```go
type AuthHandler interface {
    Parameters() []AuthParam
    OnRequest(req *http.Request, key string, params map[string]string) error
}
```

**`LinkParser`** — parses an HTTP response and returns normalized `[]*Link`.

**`ContentEncoding`** — wraps/unwraps encoded bytes.

**`Marshaller`** — marshal/unmarshal to/from `[]byte`.

### `ResponseFormatter`

The formatter is also replaceable via `cli.Formatter = NewDefaultFormatter(...)`.

### `cli.Root`

The root `*cobra.Command` is exported, so embedders can add their own
subcommands before calling `cli.Run()`.

---

## 24. Miscellaneous Utilities

### `cert` command

Directly inspects TLS certificate chain. Output includes: issuer, subject,
signature algorithm, `NotBefore`, `NotAfter`, relative expiry ("in N.N days"
/ "N.N days ago"), and SAN DNS names.

### `auth-header` command

Emits a ready-to-use `Authorization` header value. Integrates with cached
OAuth tokens. Useful for scripting:

```bash
curl https://api.example.com/ -H "Authorization: $(restish auth-header my-api)"
```

### `api content-types`

Lists all registered content types (by priority) and output formats.

### Operation deprecation

OpenAPI `deprecated: true` is reflected in help text (`[DEPRECATED]` notice).

### Operation grouping

OpenAPI tags are mapped to named `cobra.Group` instances, organizing
operations under headings in `--help` output.

### Config directory migration

On first run with a new config dir location, Restish migrates
`config.json`, `apis.json`, and `cache.json` from `~/.restish/` to the
platform-standard location. Caches are not migrated (regenerable).

### Custom app name

`cli.Init("myapp", version)` sets the application name, which controls:

- The name shown in help text and errors.
- Config/cache directory names (`~/.config/myapp/`, etc.).
- Environment variable prefix (`MYAPP_CONFIG_DIR`, etc.).

---

## 25. Installation & Distribution

| Method         | Command                                        |
| -------------- | ---------------------------------------------- |
| Homebrew       | `brew install rest-sh/tap/restish`             |
| mise           | `mise use -g restish@latest`                   |
| Nixpkgs        | `nix-env --install --attr nixpkgs.restish`     |
| GitHub release | Download binary from releases page             |
| Go install     | `go install github.com/rest-sh/restish@latest` |

Built with `goreleaser`. CI via GitHub Actions.

### `zsh` note

When using `zsh` (default on macOS), `?` and `[]` in arguments may be
glob-expanded by the shell. Recommendation: `alias restish="noglob restish"`
in `~/.zshrc`, or quote arguments.

---

## 26. Dependencies (Key Libraries)

| Library                      | Role                                            |
| ---------------------------- | ----------------------------------------------- |
| `spf13/cobra`                | CLI framework (commands, flags, completions)    |
| `spf13/viper`                | Configuration (file, env, flags)                |
| `spf13/pflag`                | POSIX flag parsing                              |
| `pb33f/libopenapi`           | OpenAPI 3.0/3.1 parsing                         |
| `danielgtaylor/shorthand/v2` | CLI shorthand language + query language         |
| `danielgtaylor/mexpr`        | Expression language for bulk list filtering     |
| `danielgtaylor/casing`       | Name case conversion                            |
| `gbl08ma/httpcache`          | RFC 7234 disk-based HTTP cache                  |
| `fxamacker/cbor/v2`          | CBOR encode/decode                              |
| `shamaton/msgpack/v2`        | MessagePack encode/decode                       |
| `amzn/ion-go`                | Amazon Ion encode/decode                        |
| `andybalholm/brotli`         | Brotli decompression                            |
| `alecthomas/chroma`          | Syntax highlighting                             |
| `charmbracelet/glamour`      | Markdown rendering in terminal                  |
| `eliukblau/pixterm`          | Image rendering in terminal                     |
| `AlecAivazis/survey/v2`      | Interactive terminal UI (API configure prompts) |
| `ThalesIgnite/crypto11`      | PKCS#11 hardware certificate support            |
| `golang.org/x/oauth2`        | OAuth 2.0 token exchange                        |
| `tent/http-link-go`          | HTTP Link header parsing (RFC 5988)             |
| `hexops/gotextdiff`          | Unified diff for `edit` and `bulk diff`         |
| `schollz/progressbar/v3`     | Progress bar for `bulk push/pull`               |
| `alexeyco/simpletable`       | Table rendering                                 |
| `gosimple/slug`              | Operation name slugification                    |
| `lucasjones/reggen`          | Regex-based example generation                  |
| `logrusorgru/aurora`         | Terminal color helpers                          |
| `mattn/go-colorable`         | Cross-platform color output (Windows)           |
| `mattn/go-isatty`            | TTY detection                                   |
