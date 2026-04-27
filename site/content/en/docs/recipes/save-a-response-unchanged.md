---
title: Save a Response Unchanged
linkTitle: Save Unchanged
weight: 40
description: Save original response bytes instead of formatted output.
---

Most structured responses are decoded and then formatted for humans or tools.
When you need the exact bytes from the server, choose a raw path. That matters
for binary files, checksums, fixtures, and anything another program will parse
byte-for-byte.

```bash
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

For an image:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

Image responses redirect as original bytes by default. Use `--rsh-raw` for byte
streams that would otherwise be decoded or formatted. The distinction is part
of Restish's [output defaults](/docs/reference/output-defaults/).

Related: [Output](/docs/guides/output/).
