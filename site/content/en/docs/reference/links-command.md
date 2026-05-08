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
restish links api.rest.sh/images
restish links api.rest.sh/images next
restish api.rest.sh/images -f links.next
```

The first command prints all discovered relations. The second prints only the
`next` relation. The third shows the equivalent filter form, which is useful in
scripts that already make a normal request.

## Notes

Use `links` when you only need relations such as `self`, `next`, or `prev`.
Use normal requests plus filters when you need body data and link data together.
Restish extracts links from HTTP `Link` headers and supported body formats such
as HAL, JSON:API, Siren, JSON-LD/TSJ, and simple `self` fields. The command
prints Restish's normalized relation map, so repeated raw links with the same
relation name are collapsed to one URI.
The discovery rules are explained in [Links and Hypermedia](/docs/guides/links-and-hypermedia/).

## Related Pages

- [Commands](/docs/reference/commands/)
- [Links and Hypermedia](/docs/guides/links-and-hypermedia/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
