---
title: Troubleshooting
linkTitle: Troubleshooting
weight: 98
description: Diagnose the most common Restish setup, request, auth, discovery, and plugin problems.
---

This page collects the failure modes people hit most often when first adopting
Restish.

## The Shell Is Rewriting My Input

Symptoms:

- `?`, `[]`, or `*` behave strangely
- shorthand arguments disappear or expand
- filter expressions are altered before Restish sees them

Fix:

```bash
restish setup zsh
source ~/.zshrc
```

Or quote the affected arguments manually.

Related page:

- [Shell Setup](/docs/getting-started/shell-setup/)

## API Registration Worked, But No Generated Commands Appeared

Symptoms:

- `restish api configure myapi ...` succeeds
- `restish myapi --help` does not show operations

Check:

```bash
restish api show myapi
restish api sync myapi
```

Common causes:

- the API does not advertise its spec
- the spec is not at a common fallback path
- the spec is invalid or not OpenAPI-compatible
- `spec_url` was never set explicitly for a non-standard API

Fix:

Set `spec_url` in config if you know exactly where the spec lives.

## I Expected JSON But Got Readable Output

On a terminal, structured output defaults to the human-oriented `readable`
format.

If you want JSON:

```bash
restish https://api.rest.sh/images -o json
```

If you redirect to a file or pipe, structured output defaults to JSON already.

## I Need To Compare A Request With Curl

Restish does not currently print a full curl reproduction command because
profile auth, hook plugins, TLS signer plugins, retries, and generated-command
defaults can change the request after CLI parsing. Use `-v` to inspect the
method, final URL, and redacted headers, then copy the non-secret parts into
curl manually:

```bash
restish https://api.rest.sh/images -v
```

If you need byte-for-byte wire debugging, use curl directly for that check and
then bring the confirmed headers or body back into Restish.

## A Server Requires Exact Header Casing

HTTP header names are case-insensitive. Restish uses Go's standard HTTP
transport, which canonicalizes header names and may use HTTP/2 where header
names are lowercase on the wire.

If a broken HTTP/1.1 server requires exact outbound header casing, Restish
cannot currently guarantee that casing without replacing the transport. Use
curl or a small purpose-built helper for that endpoint, or fix the server/proxy
to treat header names case-insensitively.

## The Command Failed Because of HTTP Status, But I Still Need the Body

Use:

```bash
restish https://api.rest.sh/images --rsh-ignore-status-code
```

That preserves the output but forces a zero exit code.

## My Response Looks Stale

Restish uses a local HTTP cache when the response is cacheable.

Force a fresh request:

```bash
restish https://api.rest.sh/images --rsh-no-cache
```

Inspect or clear the cache:

```bash
restish cache info
restish cache clear
restish cache clear myapi
```

## OAuth or Prompted Auth Seems Wrong

Useful checks:

```bash
restish auth-header myapi
restish api clear-auth-cache myapi
restish -p debug myapi/items -v
```

Those help answer:

- which profile is active
- whether a token is cached
- whether the final `Authorization` header is what you expect

## Pagination Changed the Shape of My Output

If you are counting, sorting, aggregating, or filtering across the whole
result, collect first:

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

If you want item-by-item processing, prefer:

```bash
restish https://api.rest.sh/images -o ndjson
```

## Plugins Are Not Being Discovered

Checks:

```bash
restish plugin list
restish plugin debug restish-my-plugin
```

Common causes:

- the executable is not named `restish-<name>`
- the binary was built but not installed into the Restish plugin directory
- the manifest is invalid

## TLS or mTLS Requests Fail

Work through this in order:

1. verify the server cert with `restish cert <uri>`
2. add `--rsh-ca-cert` if you need a custom CA
3. confirm whether you should be using `--rsh-client-key` or a TLS signer
4. retry with `-vv` when you need more TLS detail

Use `--rsh-insecure` only as a temporary debugging step.

## Two Operations Produce the Same Command Name

Symptoms:

- `restish myapi --help` shows a command name that collides with another operation
- one operation's help shows the wrong description

Cause:

Two operations share the same `operationId`, or their names generate the same
kebab-case command slug.

Fix:

Use `x-cli-name` on one or both operations to assign distinct names:

```yaml
paths:
  /items:
    get:
      operationId: listItems
      x-cli-name: list-all-items
  /items/active:
    get:
      operationId: listItems   # duplicate
      x-cli-name: list-active-items
```

## Enum Values Do Not Appear in Shell Completion

Symptoms:

- tab-completing a flag shows no values even though the OpenAPI spec defines an enum

Common causes:

- the spec uses `type: array` with `items.enum` instead of a top-level `enum`
- the parameter schema is nested under `$ref` and not fully resolved
- the enum is on a request body field, not a query or path parameter

Fix:

Check that the parameter schema has a direct `enum` array at the top level.
Run `restish api sync myapi` to refresh the cached spec after updating it.

## Corporate HTTP Proxy

If your network requires an HTTP proxy, set the standard environment variables
before running Restish:

```bash
export HTTPS_PROXY=https://proxy.corp.example.com:8080
export HTTP_PROXY=http://proxy.corp.example.com:8080
export NO_PROXY=localhost,127.0.0.1,.corp.example.com
```

Restish uses Go's standard `net/http` transport, which respects these variables
automatically. If your proxy requires mTLS or a custom CA, combine the proxy
env var with `--rsh-ca-cert`.

## A New Operation I Added Does Not Show Up

Symptoms:

- you updated an OpenAPI spec that Restish caches
- the new operation's command is missing from `restish myapi --help`

Fix:

```bash
restish api sync myapi
```

Restish builds generated commands from the cached spec at startup. `api sync`
re-fetches and re-caches the spec so the new operation appears on the next run.

## Spec Changes Do Not Take Effect After a Local Edit

If you are using `spec_files` to load a local spec file and your edits are not
showing up:

```bash
restish api sync myapi
```

The same cache applies to local file specs. Sync forces a reload and cache
refresh.

## Config Migrated from v1 — Where Did My APIs Go?

Restish v2 reads config from a new path (`restish.json` in
`$RSH_CONFIG_DIR` or the platform default config directory). It does not
automatically migrate v1 config on first run.

To locate your v1 config:

```bash
cat ~/.restish/apis.json 2>/dev/null || echo "not found"
```

Migration steps:

1. export each API name and base URL from the v1 config
2. run `restish api configure <name> <base_url>` for each
3. re-run `restish api sync <name>` if the spec is not auto-discovered

See the [Upgrade from v1](/docs/getting-started/upgrade-from-v1/) guide for a
complete list of breaking changes.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Authentication](/docs/guides/authentication/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [TLS](/docs/guides/tls/)
- [Pagination and Links](/docs/guides/pagination/)
