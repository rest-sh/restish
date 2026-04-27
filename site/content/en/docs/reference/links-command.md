---
title: Links Command
linkTitle: Links
weight: 40
description: GET a resource and print normalized hypermedia links.
---

GET a resource and print normalized hypermedia links.

## Examples

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next
restish https://api.rest.sh/images -f links.next -r
```

## Notes

Use `links` when you only need relations such as `self`, `next`, or `prev`.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
