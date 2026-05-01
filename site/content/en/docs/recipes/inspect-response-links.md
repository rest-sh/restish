---
title: Inspect Response Links
linkTitle: Inspect Links
weight: 58
description: Print normalized hypermedia links from a response.
---

APIs can publish navigation links in headers or in response bodies. Restish
normalizes those into the `links` root so you do not need to remember every
wire-level convention. Use the `links` command when the navigation metadata is
what you care about.

```bash
restish links https://api.rest.sh/images
```

Ask for specific relations:

```bash
restish links https://api.rest.sh/images next self
```

Filter links from a normal request:

```bash
restish https://api.rest.sh/images -f links.next
```

The command form is convenient for inspection. The filter form is convenient
inside scripts, because it composes with the rest of Restish output filtering.
The [Links and Hypermedia guide](/docs/guides/links-and-hypermedia/) explains
how relations such as `self`, `next`, and `prev` are discovered.

Related: [Links and Hypermedia](/docs/guides/links-and-hypermedia/).
