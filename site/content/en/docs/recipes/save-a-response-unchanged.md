---
title: Save a Response Unchanged
linkTitle: Save Unchanged
weight: 40
description: Save response body bytes instead of formatted output.
---

Most structured responses are decoded and then formatted for humans or tools.
When you need the response body bytes instead, choose a raw path. That matters
for binary files, fixtures, and anything another program will parse directly.

```bash
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

For an image:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

Image responses redirect as body bytes by default. Use `--rsh-raw` for byte
streams that would otherwise be decoded or formatted. Raw output still uses the
body that Go's HTTP client exposes after any HTTP content-encoding decompression;
it is not a packet capture of the exact wire transfer. The distinction is part
of Restish's [output defaults](/docs/reference/output-defaults/).

Related: [Output](/docs/guides/output/).
