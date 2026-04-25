---
title: Global Flags
linkTitle: Global Flags
weight: 15
description: Reference for the flags shared across most Restish commands.
---

Most Restish commands share the same global flags. That is one reason the tool
feels consistent whether you are making a generic request, calling a generated
API command, or using a plugin-backed workflow.

Command-line flags override environment variables and config-file defaults.

## Request Construction

- `-H`, `--rsh-header`: add a request header in `Name: Value` format; manually setting `Accept` overrides Restish's generated content-negotiation header
- `-q`, `--rsh-query`: add a request query parameter in `key=value` format
- `-c`, `--rsh-content-type`: choose the request body content type
- `-s`, `--rsh-server`: override the scheme and host for a request; when the override includes a path, that path is prefixed to outgoing request paths
- `-p`, `--rsh-profile`: choose the active API profile
- `--rsh-timeout`: set the request timeout, for example `30s`

Related env vars:

- `RSH_PROFILE`
- `RSH_TIMEOUT`

Examples:

```bash
restish -H 'Accept: application/json' https://api.rest.sh/
restish post -c yaml https://api.rest.sh/types string: hello
restish --rsh-server https://staging.example.com myapi users list
restish --rsh-server https://staging.example.com/v2 myapi users list
```

## Output And Filtering

- `-o`, `--rsh-output-format`: choose a formatter such as `json`, `ndjson`, `yaml`, or `table`
- `-f`, `--rsh-filter`: filter the normalized response with shorthand or jq
- `--rsh-filter-lang`: force `shorthand` or `jq`
- `-r`, `--rsh-raw`: with no filter, write the original response body bytes; with a filter, make scalar output shell-friendly
- `--rsh-columns`: pick columns for `-o table`
- `--rsh-sort-by`: sort table rows by a column
- `--rsh-headers`: shorthand for `-f headers`
- `-S`, `--rsh-silent`: suppress output entirely
- `-v`, `--rsh-verbose`: print request and response diagnostics to stderr

Default behavior worth remembering:

- TTY structured output defaults to `readable`
- non-TTY structured output defaults to JSON
- `--rsh-headers` is shorthand for `-f headers`

Examples:

```bash
restish https://api.rest.sh/images -f '.body[] | .name' -r
restish https://api.rest.sh/images -o ndjson -f 'body.self'
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
restish https://api.rest.sh/images -v
```

## Pagination And Streaming

- `--rsh-no-paginate`: disable automatic pagination
- `--rsh-max-pages`: cap the number of pages fetched
- `--rsh-max-items`: cap the number of collected items
- `--rsh-collect`: collect all pages before filtering and formatting
- `--rsh-max-events`: cap SSE events or NDJSON lines processed

These matter most for collection endpoints, SSE streams, and NDJSON streams.

Examples:

```bash
restish https://api.rest.sh/images --rsh-max-pages 3
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
restish https://api.rest.sh/images -o ndjson --rsh-max-items 100
restish https://your-api.example.com/events --rsh-max-events 10 -o ndjson
```

## Resilience And Status Handling

- `--rsh-retry`: set retry count, `0` to disable; when omitted Restish retries twice
- `--rsh-no-cache`: bypass cache reads and writes
- `--rsh-ignore-status-code`: always exit `0` regardless of HTTP status
- `--rsh-max-body-size`: cap response body size in MiB

Related env vars:

- `RSH_RETRY`
- `RSH_COMMAND_PLUGIN_SHUTDOWN_GRACE`: plugin shutdown grace period, such as `250ms` or `2s`

`RSH_HOOK_RM_BEHAVIOR` is used only by Restish's hook-plugin test fixture; it is not a supported user-facing setting.

Examples:

```bash
restish https://api.rest.sh/images --rsh-retry 5
restish https://api.rest.sh/images --rsh-no-cache
restish https://api.rest.sh/missing --rsh-ignore-status-code
```

## TLS And mTLS

- `--rsh-insecure`: disable certificate verification
- `--rsh-ca-cert`: trust an additional PEM CA bundle
- `--rsh-client-cert`: path to a PEM client certificate
- `--rsh-client-key`: path to a PEM private key
- `--rsh-tls-min-version`: require `TLS1.2` or `TLS1.3`
- `--rsh-tls-signer`: choose a TLS signer plugin
- `--rsh-tls-signer-param`: pass plugin-specific `key=value` parameters

Example:

```bash
restish \
  --rsh-ca-cert ./corp-ca.pem \
  --rsh-client-cert ./client.pem \
  --rsh-client-key ./client.key \
  https://internal.example.com/items
```

## Default Behavior Worth Remembering

- bare URLs are treated as `GET`
- TTY output defaults to `readable`
- non-TTY structured output defaults to JSON
- `2xx` exits with `0`, `3xx` with `3`, `4xx` with `4`, and `5xx` with `5`

See also:

- [Commands](../commands/)
- [Environment Variables](../environment-variables/)
- [Output Defaults](../output-defaults/)
- [Output Formats](../output-formats/)
- [Command Behavior Guide](/docs/guides/command-behavior/)
