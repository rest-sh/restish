---
title: Inspect Response Links
linkTitle: Inspect Response Links
weight: 50
description: Inspect hypermedia links exposed by an API response with Restish.
---

Use the `links` command to fetch a resource and print its discovered links:

```bash
restish links https://api.rest.sh/images
```

To limit output to a few relations:

```bash
restish links https://api.rest.sh/images next self
```

If you want one relation in a shell-friendly form, filter the normalized
response instead:

```bash
restish https://api.rest.sh/images -f links.next -r
```
