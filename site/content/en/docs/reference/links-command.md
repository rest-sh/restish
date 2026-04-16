---
title: Links Command
linkTitle: Links Command
weight: 13
description: Reference for restish links, which fetches a resource and prints normalized hypermedia links.
---

`restish links <uri> [rel...]` performs a GET and prints the discovered links.

## Examples

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next
restish links https://api.rest.sh/images self next prev
```

If you only need one relation in a shell-friendly form, you can also filter the
normalized response directly:

```bash
restish https://api.rest.sh/images -f links.next -r
```

## Related Pages

- [Links and Hypermedia](/docs/guides/links-and-hypermedia/)
- [Pagination and Links](/docs/guides/pagination/)
