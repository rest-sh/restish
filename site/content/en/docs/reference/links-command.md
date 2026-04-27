---
title: Links Command
linkTitle: Links
weight: 40
description: GET a resource and print normalized hypermedia links.
---

The `links` command performs a `GET`, extracts links from headers or supported
body formats, and prints normalized link relations. It is a shortcut for the
common question: "where can this response take me next?"

## Examples

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next
restish https://api.rest.sh/images -f links.next -r
```

The first command prints all discovered relations. The second prints only the
`next` relation. The third shows the equivalent filter form, which is useful in
scripts that already make a normal request.

## Notes

Use `links` when you only need relations such as `self`, `next`, or `prev`.
Use normal requests plus filters when you need body data and link data together.
The discovery rules are explained in [Links and Hypermedia](/docs/guides/links-and-hypermedia/).

## Related Pages

- [Commands](/docs/reference/commands/)
- [Links and Hypermedia](/docs/guides/links-and-hypermedia/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
