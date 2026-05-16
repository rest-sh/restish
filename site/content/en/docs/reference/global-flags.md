---
title: Global Flags
linkTitle: Global Flags
weight: 12
description: Reference for flags shared across generic requests, generated API commands, utilities, and plugin-delegated requests.
---

Global flags apply across generic requests, generated API commands, utilities,
and many plugin-delegated requests. Ordinary command help shows common flags;
`--help-all` shows the full grouped reference:

```bash
restish get --help
restish get --help-all
```

## Request Construction

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `-H`, `--rsh-header` | repeatable `Name: Value` | none | Add request headers. Sensitive values are redacted in diagnostics. |
| `-q`, `--rsh-query` | repeatable `key=value` | none | Add query params without hand-editing the URL. |
| `-c`, `--rsh-content-type` | content alias or MIME | `json` | Request body encoder, such as `json`, `yaml`, `form`, or `multipart`. |
| `-s`, `--rsh-server` | URL | config/spec server | Override scheme and host for one request. |
| `-t`, `--rsh-timeout` | duration | transport default | Bound ordinary request lifetime. For SSE/NDJSON streams, bound the wait for response headers before stream rules take over. |
| `--rsh-max-body-size` | MiB | `100` when `0` | Maximum response body size. |
| `--rsh-ignore-status-code` | boolean | false | Exit zero even for HTTP error statuses. |

```bash
restish -H 'Accept: application/json' api.rest.sh/headers
restish -q api_key=docs-key api.rest.sh/auth/api-key-query
restish post -c form api.rest.sh/login 'username: alice, password: secret'
restish --rsh-server https://staging.example.com example list-images
```

`RSH_HEADER` and `RSH_QUERY` use comma-separated entries, matching the
repeatable `-H` and `-q` flags. Escape a literal comma in an environment value
as `\,`:

```bash
RSH_HEADER='X-List: a\,b' restish api.rest.sh/headers
RSH_QUERY='list=a\,b' restish api.rest.sh/get
```

Generated operation commands may also expose `--rsh-generate-body` when the
OpenAPI operation has a request body schema. That flag prints an example body
for the generated operation instead of sending the request.

## Output And Filtering

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `-f`, `--rsh-filter` | expression | none | Filter with shorthand or jq. |
| `--rsh-filter-lang` | `shorthand` or `jq` | auto | Force one parser. |
| `-o`, `--rsh-output-format` | format | `auto` | `auto`, `json`, `yaml`, `cbor`, `table`, `ndjson`, `lines`, `gron`, `image`, or plugin formats. |
| `--rsh-print` | `auto` or letters | `auto` | Print parts: `H` request headers, `B` request body, `h` response status and headers, `b` rendered body, `p` pretty, `c` color. |
| `--rsh-columns` | comma list | formatter default | Columns for `-o table`. |
| `--rsh-sort-by` | column name | none | Sort table rows by a column. |
| `--rsh-headers` | boolean | false | Shortcut for `-f headers`; selects raw response headers. |
| `--rsh-status` | boolean | false | Shortcut for `-f status`. |
| `-S`, `--rsh-silent` | boolean | false | Suppress request output, diagnostics, and request errors; use only exit status. |

```bash
restish api.rest.sh/images -f body.self -o lines
restish api.rest.sh/images -o table --rsh-columns name,format,self
restish api.rest.sh/types --rsh-print=b > types.json
restish api.rest.sh/ --rsh-headers
restish -S api.rest.sh/status/204
```

## Auth And Profiles

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `-p`, `--rsh-profile` | profile name | `default` or `RSH_PROFILE` | Select API profile. Command line wins over env. |
| `--rsh-auth` | auth requirement | operation default | Generated operation auth override, such as `PartnerKey` or `UserOAuth+PartnerKey`. |
| `--rsh-no-browser` | boolean | false | Disable automatic browser launch for interactive auth. |

```bash
restish -p json example list-images
restish myapi signed-report --rsh-auth UserOAuth+PartnerKey
restish --rsh-no-browser api connect example api.rest.sh 'prompt.api_key: docs-key'
```

## TLS

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--rsh-ca-cert` | path | system trust | Additional PEM CA certificate. |
| `--rsh-client-cert` | path | none | PEM client certificate for mTLS. |
| `--rsh-client-key` | path | none | PEM private key for mTLS. |
| `--rsh-insecure` | boolean | false | Disable certificate verification for debugging only. |
| `--rsh-tls-min-version` | `TLS1.2` or `TLS1.3` | Go default | Minimum TLS version. |
| `--rsh-tls-signer` | plugin name | none | TLS signer plugin for external key signing. |
| `--rsh-tls-signer-param` | repeatable `key=value` | none | Parameters for the signer plugin. |

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
restish --rsh-tls-min-version TLS1.3 api.rest.sh
```

## Pagination And Streaming

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--rsh-no-paginate` | boolean | false | Return only the first page. |
| `--rsh-max-pages` | integer | `25` | Maximum pages, `0` means unlimited. |
| `--rsh-max-items` | integer | `0` | Maximum paginated items or streamed records, `0` means unlimited. |
| `--rsh-collect` | boolean | false | Collect all pages before filtering. |

```bash
restish api.rest.sh/images --rsh-no-paginate
restish api.rest.sh/images --rsh-max-pages 3
restish api.rest.sh/images --rsh-collect -f '.body | length'
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
```

## Cache And Retry

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--rsh-no-cache` | boolean | false | Bypass response cache reads and writes. |
| `--rsh-retry` | integer | `2` | Retry attempts for network errors and transient HTTP responses; `0` disables. |
| `--rsh-retry-max-wait` | duration | `5m` | Cap server-provided retry delays. |
| `--rsh-retry-unsafe` | boolean | false | Permit retries for POST, PUT, PATCH, and DELETE. |

```bash
restish 'api.rest.sh/flaky?failures=1&key=flags' --rsh-retry 2
restish post https://api.vendor.test/jobs name:demo --rsh-retry 2 --rsh-retry-unsafe
restish api.rest.sh/cache --rsh-no-cache
```

## General

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--rsh-config` | path | platform default | Active config file. Overrides `RSH_CONFIG` and default discovery. |
| `-v`, `--rsh-verbose` | count | `0` | `-v` shows request/response headers; `-vv` adds TLS details. |
| `--help-all` | boolean | false | Show all inherited Restish flags in help. |
| `--version` | boolean | false | Print version from the root command. |

```bash
restish -v api.rest.sh/headers
restish -vv api.rest.sh/headers
restish --rsh-config ./restish.json api list
```

## Precedence

For request behavior, command-line flags win over environment variables, which
win over profile/API config defaults. Use explicit flags for one command; use
profiles when a choice should become routine.

## Related Pages

- [Requests](/docs/guides/requests/)
- [Output](/docs/guides/output/)
- [Config](../config/)
