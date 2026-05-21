---
title: Cache Command
linkTitle: Cache
weight: 15
description: Reference for inspecting and clearing the Restish HTTP response cache.
---

Use `restish cache` to inspect or clear cached HTTP responses. This cache is
separate from the OpenAPI spec cache and OAuth/auth token cache.

## Common Examples

```bash
restish cache info
restish cache info -o json
restish cache clear
restish cache clear example
```

Use `api sync` when you need to refresh a cached OpenAPI document. Use
`api auth logout` when you need to clear cached auth tokens.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen cache-command -->
Generated from the current Cobra command tree.

### `restish cache`

Manage the HTTP response cache

Manage Restish's HTTP response cache.

The HTTP cache stores reusable responses for requests that are safe to cache. It is separate from the OpenAPI spec cache and OAuth token cache. Use `cache info` to inspect size and location, and `cache clear` when cached responses should no longer be reused.

Usage:

```text
restish cache
```

Examples:

```bash
  restish cache info
  restish cache clear
  restish cache clear demo
```

Subcommands:

**`restish cache clear`**: Delete cached HTTP responses, not OAuth tokens (omit API to clear all)

**`restish cache info`**: Print cache directory, size, entry count, and oldest entry


### `restish cache info`

Print cache directory, size, entry count, and oldest entry

Print the HTTP response cache directory, size, entry count, and oldest entry.

Use `-o json` for scripts that need stable fields. This command does not inspect the OpenAPI spec cache or auth token cache.

Usage:

```text
restish cache info
```

Examples:

```bash
  restish cache info
  restish cache info -o json
```


### `restish cache clear`

Delete cached HTTP responses, not OAuth tokens (omit API to clear all)

Delete cached HTTP responses.

Omit the API name to clear every HTTP response cache entry. Pass an API name to clear only entries for that registered API. OAuth tokens and cached OpenAPI documents are not removed.

Usage:

```text
restish cache clear [api]
```

Examples:

```bash
  restish cache clear
  restish cache clear demo
```
<!-- END GENERATED -->

## Related Pages

- [Config Command](../config-command/)
- [API Management](../api-management/)
- [Retries And Caching](/docs/guides/retries-and-caching/)
- [Global Flags](../global-flags/)
