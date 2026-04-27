---
title: Cache Commands
linkTitle: Cache
weight: 40
description: Inspect and clear the Restish HTTP response cache.
---

Restish can reuse HTTP responses when the server says they are cacheable. The
`cache` commands help you see where that cache lives, how large it is, and clear
it when you need to force fresh responses while debugging.

## Examples

```bash
restish cache info
restish cache clear
restish cache clear example
```

Use `cache info` first. It gives you the cache location, size, and entry count.
Use `cache clear` to clear everything, or pass an API name when you only want to
clear entries for one configured API.

## Notes

If a response looks stale, try the request once with `--rsh-no-cache` before
clearing everything. The [Retries and Caching guide](/docs/guides/retries-and-caching/)
explains when Restish stores responses and how request flags affect cache use.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
