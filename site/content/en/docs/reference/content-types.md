---
title: Content Types
linkTitle: Content Types
weight: 28
description: Reference for built-in content types, request encoding, response decoding, compression, and content plugins.
aliases:
  - /docs/recipes/request-a-specific-response-format/
---

Restish separates two decisions:

- Request encoding: `-c` or `--rsh-content-type` controls how a request body is sent.
- Response rendering: `-o` controls how a decoded response is printed.

HTTP `Accept` headers ask the server for a representation. Output formats do
not change what the server sends.

## Built-In Types

`restish content-types` lists the registered content aliases and MIME types:

```bash
restish content-types
restish content-types -o json
```

| Alias | MIME types |
| --- | --- |
| `json` | `application/json` |
| `ndjson` | `application/x-ndjson`, `application/ndjson`, `application/jsonl`, `application/jsonlines` |
| `yaml` | `application/yaml`, `application/x-yaml`, `text/yaml`, `text/x-yaml` |
| `cbor` | `application/cbor` |
| `msgpack` | `application/msgpack`, `application/x-msgpack`, `application/vnd.msgpack` |
| `binary` | `application/octet-stream` |
| `ion` | `application/ion`, `text/ion` |
| `form` | `application/x-www-form-urlencoded` |
| `multipart` | `multipart/form-data` |
| `sse` | `text/event-stream` |
| `text` | `text/plain`, `text/*` |

JSON-family structured types with `+json`, such as
`application/problem+json`, decode as JSON. Structured suffixes win before broad
wildcards, so a response labeled `text/example+json` is treated as JSON rather
than plain `text/*` unless an exact handler is registered for that MIME type.

## Request Encoding

JSON is the default request body encoding:

```bash
restish post -c json api.rest.sh/post string: hello
restish post -c yaml api.rest.sh/post string: hello
restish post -c form api.rest.sh/login 'username: alice, password: secret'
restish post -c multipart api.rest.sh/uploads description: docs, file: @README.md
```

Shorthand builds a logical value. The selected encoder turns that value into
JSON, YAML, CBOR, form data, multipart parts, text, or raw bytes.
Use `-c text` for plain text request bodies; SSE responses still decode from
`text/event-stream`.

## Response Decoding

Restish decodes supported structured responses before filtering and formatting:

```bash
restish api.rest.sh/formats/json
restish api.rest.sh/formats/yaml -o yaml
restish api.rest.sh/formats/cbor
restish api.rest.sh/problem --rsh-ignore-status-code
```

If stdout is redirected and no filter or output format is selected, Restish
writes the response body bytes instead of reformatting them.

## Accept Header

Use `Accept` when you need to influence server-side content negotiation:

```bash
restish -H 'Accept: application/json' api.rest.sh/formats/json
restish -H 'Accept: image/png' api.rest.sh/images/png
```

Restish generates an `Accept` header from registered content types, ordered by
quality and deduplicated by canonical MIME type. If a plugin registers the same
MIME type later, that later registration is the effective one.

Use `-o` when you want Restish to transform a decoded response after it arrives:

```bash
restish api.rest.sh/content/cbor -o json > response.json
```

## Compression

Restish handles common HTTP content encodings before response decoding:

```bash
restish api.rest.sh/gzip
restish api.rest.sh/deflate
restish api.rest.sh/brotli
```

Raw output uses the body exposed after HTTP content-encoding decompression. It
is not a capture of compressed wire bytes.

## Plugins

Content plugins can add request encoders, response decoders, and output
formatters. Use `restish content-types` and `restish plugin list` to confirm
what your current binary can handle.

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Plugins](/docs/plugins/install-and-use/)
