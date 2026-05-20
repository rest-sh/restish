---
title: Save a Response Unchanged
linkTitle: Save Unchanged
weight: 40
description: Save response body bytes instead of formatted output.
---

When stdout is redirected and you do not choose a filter, collection, metadata
shortcut, or output format, Restish writes the response body bytes. That matters
for binary files, structured fixtures such as CBOR, and anything another
program will parse directly. Response middleware plugins are skipped for this
raw-download path, so installed plugins cannot silently alter saved files.

```bash
restish api.rest.sh/bytes/64 > sample.bin
restish api.rest.sh/formats/cbor > response.cbor
```

For an image:

```bash
restish api.rest.sh/images/jpeg > dragonfly.jpg
```

Use `-o json`, `-o yaml`, or another output format when you want Restish to
transform the decoded body or apply response middleware before rendering.
Redirected byte output still uses the body that Go's HTTP client exposes after
any HTTP content-encoding decompression; it is not a packet capture of the exact
wire transfer. The distinction is part of Restish's
[output defaults](/docs/reference/output-defaults/).
Restish's default `Accept` header still prefers JSON and other text-friendly
structured formats; set `Accept` yourself when you want the server to send a
binary structured format such as CBOR.

Related: [Output](/docs/guides/output/).
