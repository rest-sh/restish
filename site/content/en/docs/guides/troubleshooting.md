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
- the binary is not on `PATH`
- the manifest is invalid
- `allowed_plugins` is set and the plugin name is not allowlisted

## TLS or mTLS Requests Fail

Work through this in order:

1. verify the server cert with `restish cert <uri>`
2. add `--rsh-ca-cert` if you need a custom CA
3. confirm whether you should be using `--rsh-client-key` or a TLS signer
4. retry with `-vv` when you need more TLS detail

Use `--rsh-insecure` only as a temporary debugging step.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Authentication](/docs/guides/authentication/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [TLS](/docs/guides/tls/)
- [Pagination and Links](/docs/guides/pagination/)
