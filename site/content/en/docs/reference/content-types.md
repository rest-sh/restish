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
restish post -c json https://api.example.com/items name: Alice
restish post -c yaml https://api.example.com/items name: Alice
restish post -c form https://api.example.com/login username: alice password: secret
```

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

## Compression Encodings

The built-in registry also knows how to decompress these response encodings:

- `br`
- `gzip`
- `deflate`

## Plugins

Loader plugins can extend Restish with additional API description content
types. Formatter plugins can add new output names on top of the same decoded
response model.

See:

- [Input Guide](/docs/guides/input/)
- [Output Guide](/docs/guides/output/)
- [Plugin Manifest Reference](../plugin-manifest/)
