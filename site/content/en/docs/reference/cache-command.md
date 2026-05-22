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
restish cache clear --direct
```

Use `api sync` when you need to refresh a cached OpenAPI document. Use
`api auth logout` when you need to clear cached auth tokens.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen cache-command -->
Generated from the current Cobra command tree.

### `restish cache`

Manage the HTTP response cache

Manage Restish's HTTP response cache.

The HTTP cache stores reusable responses for requests that are safe to cache. It is separate from the OpenAPI spec cache and OAuth token cache. Use `cache info` to inspect size, location, largest cached hosts, and API/profile usage, and `cache clear` when cached responses should no longer be reused.

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

**`restish cache info`**: Print cache directory, size, entry count, oldest entry, and largest hosts


### `restish cache info`

Print cache directory, size, entry count, oldest entry, and largest hosts

Print the HTTP response cache directory, size, entry count, oldest entry, and largest hosts.

TTY output includes a compact API/profile usage map. Human output also shows the largest cached hosts and API/profile namespaces with size percentages so you can see where disk space is going. Unregistered namespaces, such as old cache entries from a previous Restish version or manual cache files, are marked clearly and can be cleared by their namespace prefix. Use `-o json` for stable fields including host and API/profile breakdowns. This command does not inspect the OpenAPI spec cache or auth token cache.

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

Omit the API name to clear every HTTP response cache entry. Pass an API name to clear entries for that registered API. Use `--direct` to clear direct URL requests that are not associated with a registered API. If an unregistered namespace remains from an older Restish version or manual cache files, pass the namespace prefix shown by `cache info` to clear it. OAuth tokens and cached OpenAPI documents are not removed.

Usage:

```text
restish cache clear [api-or-namespace] [flags]
```

Examples:

```bash
  restish cache clear
  restish cache clear demo
  restish cache clear --direct
```

Flags:

**`--direct`**

Type: `bool`; default: `false`

Clear cached responses for direct URL requests that are not associated with a registered API
<!-- END GENERATED -->

## Related Pages

- [Config Command](../config-command/)
- [API Management](../api-management/)
- [Retries And Caching](/docs/guides/retries-and-caching/)
- [Global Flags](../global-flags/)
