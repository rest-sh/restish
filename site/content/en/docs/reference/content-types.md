---
title: Content Types
linkTitle: Content Types
weight: 28
description: Reference for built-in content types, request encoding, response decoding, and compression.
---

Restish uses a content registry for request bodies, response decoding,
compression, filters, output formats, and plugins.

Content type handling is what lets one command send JSON, receive CBOR, filter
the decoded body, and then render readable output. Request encoding and response
decoding are separate decisions: `-c` controls what Restish sends, while the
response `Content-Type` controls how Restish decodes what came back.

## Built-In Types

Run the live list from your binary:

```bash
restish content-types
```

Typical built-ins include `json`, `yaml`, `cbor`, `msgpack`, `ion`, `form`,
`multipart`, `text`, and `binary`.

## Request Encoding

```bash
restish post -c json https://api.rest.sh/post string: hello
restish post -c yaml https://api.rest.sh/post string: hello
restish post -c form https://api.rest.sh/login 'username: alice, password: secret'
restish post -c multipart https://api.rest.sh/uploads description: docs, file: @README.md
```

Use `json` for most API bodies, `form` for older login/token endpoints, and
`multipart` when a request includes file parts. The [Input guide](/docs/guides/input/)
shows how shorthand, stdin, and files become request bodies.

Use `binary` or an explicit binary media type when the request should preserve
raw bytes from stdin or a file instead of encoding a structured value.

## Response Decoding

Restish matches response `Content-Type` against registered MIME types, wildcard
fallbacks such as `text/*`, and structured suffixes such as `+json`.

```bash
restish https://api.rest.sh/formats/json
restish https://api.rest.sh/formats/yaml -o yaml
restish https://api.rest.sh/formats/cbor
restish https://api.rest.sh/problem --rsh-ignore-status-code
```

Structured suffix support is why `application/problem+json` and vendor JSON
types can still be filtered like normal JSON.

## Accept Header

Restish sends an `Accept` header built from the registry. Override it when the
server should prefer a specific representation:

```bash
restish -H 'Accept: application/json' https://api.rest.sh/formats/json
restish -H 'Accept: image/png' https://api.rest.sh/image -o image
```

Changing `Accept` asks the server for a representation. Changing `-o` only
changes local rendering after Restish has decoded the response.

## Compression

```bash
restish https://api.rest.sh/gzip
restish https://api.rest.sh/deflate
restish https://api.rest.sh/brotli
```

Restish advertises and decodes supported response encodings through
`Accept-Encoding` and `Content-Encoding`. Built-in decoding covers gzip,
deflate, and Brotli before response content-type decoding, filtering, or output
formatting runs.

## Plugins

Loader plugins can add spec content types. Formatter plugins can add output
formats such as `csv` without changing response decoding.

That separation is important for plugin authors: a formatter plugin can render
an already-decoded response without teaching Restish a new wire format.

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Plugin Manifest](../plugin-manifest/)
