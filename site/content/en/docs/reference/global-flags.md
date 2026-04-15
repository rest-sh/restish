---
title: Global Flags
linkTitle: Global Flags
weight: 15
description: Reference for the flags shared across most Restish commands.
---

Most Restish commands share the same global flags. That is one reason the tool
feels consistent whether you are making a generic request, calling a generated
API command, or using a plugin-backed workflow.

## Request Construction

- `-H`, `--rsh-header`: add a request header in `Name: Value` format
- `-q`, `--rsh-query`: add a request query parameter in `key=value` format
- `-c`, `--rsh-content-type`: choose the request body content type
- `-s`, `--rsh-server`: override the scheme and host for a request
- `-p`, `--rsh-profile`: choose the active API profile
- `--rsh-timeout`: set the request timeout, for example `30s`

Examples:

```bash
restish -H 'Accept: application/json' https://api.example.com/items
restish post -c yaml https://api.example.com/items name: Alice
restish --rsh-server https://staging.example.com myapi users list
```

## Output And Filtering

- `-o`, `--rsh-output-format`: choose a formatter such as `json`, `ndjson`, `yaml`, or `table`
- `-f`, `--rsh-filter`: filter the normalized response with shorthand or jq
- `--rsh-filter-lang`: force `shorthand` or `jq`
- `-r`, `--rsh-raw`: make filtered scalar output shell-friendly
- `--rsh-columns`: pick columns for `-o table`
- `--rsh-sort-by`: sort table rows by a column
- `--rsh-headers`: shorthand for `-f headers`
- `-S`, `--rsh-silent`: suppress output entirely
- `-v`, `--rsh-verbose`: print request and response diagnostics to stderr

Examples:

```bash
restish https://api.example.com/items -f '.body.items[] | .name' -r
restish https://api.example.com/items -o ndjson -f 'body.id'
restish https://api.example.com/items -o table --rsh-columns id,name,status
restish https://api.example.com/items -v
```

## Pagination And Streaming

- `--rsh-no-paginate`: disable automatic pagination
- `--rsh-max-pages`: cap the number of pages fetched
- `--rsh-max-items`: cap the number of collected items
- `--rsh-collect`: collect all pages before filtering and formatting
- `--rsh-max-events`: cap SSE events or NDJSON lines processed

Examples:

```bash
restish https://api.example.com/items --rsh-max-pages 3
restish https://api.example.com/items --rsh-collect -f '.body | length'
restish https://api.example.com/items -o ndjson --rsh-max-items 100
restish https://api.example.com/events --rsh-max-events 10 -o ndjson
```

## Resilience And Status Handling

- `--rsh-retry`: set retry count, `0` to disable, `-1` to use the default
- `--rsh-no-cache`: bypass cache reads and writes
- `--rsh-ignore-status-code`: always exit `0` regardless of HTTP status
- `--rsh-max-body-size`: cap response body size in MiB

Examples:

```bash
restish https://api.example.com/items --rsh-retry 5
restish https://api.example.com/items --rsh-no-cache
restish https://api.example.com/items --rsh-ignore-status-code
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
  https://api.example.com/items
```

## Default Behavior Worth Remembering

- bare URLs are treated as `GET`
- TTY output defaults to `readable`
- non-TTY structured output defaults to JSON
- `2xx` exits with `0`, `3xx` with `3`, `4xx` with `4`, and `5xx` with `5`

See also:

- [Commands](../commands/)
- [Output Formats](../output-formats/)
- [Command Behavior Guide](/docs/guides/command-behavior/)
