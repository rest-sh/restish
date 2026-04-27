---
title: Inspect Response Links
linkTitle: Inspect Links
weight: 58
description: Print normalized hypermedia links from a response.
---

```bash
restish links https://api.rest.sh/images
```

Ask for specific relations:

```bash
restish links https://api.rest.sh/images next self
```

Filter links from a normal request:

```bash
restish https://api.rest.sh/images -f links.next -r
```

Related: [Links and Hypermedia](/docs/guides/links-and-hypermedia/).
