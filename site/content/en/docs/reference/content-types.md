---
title: Content Types
linkTitle: Content Types
weight: 28
description: Reference for built-in content types, request encoding, response decoding, and compression.
---

Restish uses a content registry for request bodies, response decoding,
compression, filters, output formats, and plugins.

## Built-In Types

Run the live list from your binary:

```bash
restish api content-types
```

Typical built-ins include `json`, `yaml`, `cbor`, `msgpack`, `ion`, `form`,
`multipart`, and `text`.

## Request Encoding

```bash
restish post -c json https://api.rest.sh/post string: hello
restish post -c yaml https://api.rest.sh/post string: hello
restish post -c form https://api.rest.sh/login 'username: alice, password: secret'
restish post -c multipart https://api.rest.sh/uploads description: docs, file: @README.md
```

## Response Decoding

Restish matches response `Content-Type` against registered MIME types, wildcard
fallbacks such as `text/*`, and structured suffixes such as `+json`.

```bash
restish https://api.rest.sh/formats/json
restish https://api.rest.sh/formats/yaml -o yaml
restish https://api.rest.sh/formats/cbor
restish https://api.rest.sh/problem --rsh-ignore-status-code
```

## Accept Header

Restish sends an `Accept` header built from the registry. Override it when the
server should prefer a specific representation:

```bash
restish -H 'Accept: application/json' https://api.rest.sh/formats/json
restish -H 'Accept: image/png' https://api.rest.sh/image -o image
```

## Compression

```bash
restish https://api.rest.sh/gzip
restish https://api.rest.sh/deflate
restish https://api.rest.sh/brotli
```

## Plugins

Loader plugins can add spec content types. Formatter plugins can add output
formats such as `csv` without changing response decoding.

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Plugin Manifest](../plugin-manifest/)
