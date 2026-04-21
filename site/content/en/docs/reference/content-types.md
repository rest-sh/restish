---
title: Content Types
linkTitle: Content Types
weight: 28
description: Reference for the built-in content types and encodings supported by Restish.
---

Restish uses a content registry for request encoding, response decoding, and
compression handling. That registry is shared across the CLI, generated API
commands, and plugins.

## Built-In Content Types

The current built-in set is:

- `json`: `application/json`
- `yaml`: `application/yaml`, `application/x-yaml`, `text/yaml`, `text/x-yaml`
- `cbor`: `application/cbor`
- `msgpack`: `application/msgpack`, `application/x-msgpack`, `application/vnd.msgpack`
- `ion`: `application/ion`, `text/ion`
- `form`: `application/x-www-form-urlencoded`
- `multipart`: `multipart/form-data`
- `text`: `text/event-stream`, `text/*`

You can see the live list from your installed binary with:

```bash
restish api content-types
```

## Choosing A Request Content Type

Use `-c` or `--rsh-content-type` to choose how Restish encodes a request body.

```bash
restish post -c json https://api.rest.sh/types string: hello
restish post -c yaml https://api.rest.sh/types string: hello
restish post -c form https://api.example.com/login username: alice password: secret
```

Common cases:

- `json` for most APIs
- `yaml` when the API or workflow prefers YAML documents
- `form` for URL-encoded login or token endpoints
- `multipart` for form-style uploads

## Response Decoding

Restish chooses a decoder by matching the response `Content-Type` header
against the registry.

That is why the same response can later be:

- filtered with shorthand or jq
- rendered as JSON, YAML, table, or another formatter
- paginated based on decoded body structure

## Forms And Multipart

`form` and `multipart` both expect object-like input. Restish does not
aggressively reinterpret file-like values for form-style content types, because
preserving literal values is usually the safer default.

Examples:

```bash
restish post -c form https://api.example.com/login username: alice password: secret
restish post -c multipart https://api.example.com/upload name: example
```

The JSON and YAML examples use the public example API. The form and multipart
examples stay generic because they illustrate shapes more than a specific live
endpoint.

## Accept Header And Quality Ordering

When Restish sends requests it builds an `Accept` header from all registered
content types. Entries are sorted by quality value (`q`) descending, so higher
quality types appear first. Content types with quality `1.0` are written
without a `q` parameter.

For example, a registry with JSON at `q=1.0` and CBOR at `q=0.9` produces:

```
Accept: application/json, application/cbor;q=0.9
```

You can observe the full list your binary sends by running a request with
`--rsh-verbose` and reading the `Accept` header line.

## Compression Encodings

The built-in registry also knows how to decompress these response encodings:

- `br`
- `gzip`
- `deflate`

## Plugins

Loader plugins can extend Restish with additional API description content
types. Formatter plugins can add new output names on top of the same decoded
response model.

For example, a formatter plugin can add `-o csv` without changing how the
incoming response body was decoded.

See:

- [Input Guide](/docs/guides/input/)
- [Output Guide](/docs/guides/output/)
- [API Management](/docs/reference/api-management/)
- [Plugin Manifest Reference](../plugin-manifest/)
