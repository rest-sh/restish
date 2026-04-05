# Content Types And Encodings

## Summary

Restish v2 uses a registry-driven model for request and response body handling.
Content types describe how to marshal and unmarshal structured values, while
encodings describe how to transparently decompress compressed responses.

This keeps body handling extensible without scattering format-specific logic
throughout the CLI pipeline.

## Problem

Restish needs to send and receive more than just JSON. It also needs to deal
with HTTP content negotiation and compressed responses in a way that stays
predictable for both interactive use and scripting.

The design needed to:

- support multiple structured body formats
- negotiate useful `Accept` and `Accept-Encoding` headers automatically
- decode responses into a stable internal value model
- preserve the separation between body construction and wire encoding
- leave room for additional formats later

## Design

The core model is two registries:

- content types for marshaling and unmarshaling body values
- encodings for transparent response decompression

A content type entry defines:

- a short CLI-facing name like `json` or `yaml`
- one or more MIME types
- a quality value for `Accept` header generation
- marshal and unmarshal behavior

An encoding entry defines:

- the encoding token like `gzip` or `br`
- a quality value for `Accept-Encoding`
- a decompressor

This allows Restish to treat the request and response pipeline generically:

- request bodies are built as Go values first
- the selected content type encodes those values onto the wire
- response bodies are decompressed first if needed
- the selected content type decodes the bytes back into a Go value

The built-in registry currently includes:

- `json`
- `yaml`
- `cbor`
- `msgpack`
- `form`
- `multipart`
- `text`

and built-in encodings:

- `br`
- `gzip`
- `deflate`

Some design choices worth preserving:

- unknown response content types fall back to plain string data
- decoded values are normalized into JSON-safe structures so downstream
  formatting and filtering stay stable
- multipart can return a concrete runtime `Content-Type` with a boundary rather
  than pretending all content types are static strings
- wildcard MIME types like `text/*` are supported for low-priority fallback

## Examples

The generated `Accept` header conceptually prefers richer machine formats first:

```text
application/cbor;q=0.9, application/msgpack;q=0.8, application/json;q=0.5, application/yaml;q=0.5, text/*;q=0.2
```

and `Accept-Encoding` similarly advertises supported compression:

```text
br, gzip, deflate
```

A call like:

```bash
restish post https://api.example.com/items -c yaml name: Alice
```

builds a structured body value first:

```json
{
  "name": "Alice"
}
```

and then encodes it as YAML before sending it with an appropriate
`Content-Type`.

A compressed JSON response like:

```http
Content-Type: application/json
Content-Encoding: gzip
```

is decompressed before JSON decoding, so downstream code receives the same kind
of decoded structured value it would have received from an uncompressed JSON
response.

For `multipart/form-data`, the content type is concrete at runtime, for example:

```text
multipart/form-data; boundary=----restish123
```

## Alternatives Considered

### Hard-code JSON everywhere

This would simplify early implementation, but it would cut off too many of the
wire formats Restish is designed to interoperate with.

### Mix encoding and content-type handling together

Compression and body serialization are related at the transport layer, but they
solve different problems. Keeping separate registries makes the model clearer.

### Return decoder-native map types unchanged

Some formats decode into map types that do not serialize cleanly to JSON. It is
better to normalize decoded data once than to force every downstream consumer to
handle format-specific map behavior.

## Notes

The current implementation reflects this design directly:

- `internal/content/registry.go` defines the registry model and negotiation
  behavior
- `internal/content/defaults.go` registers the built-in content types and
  encodings
- `internal/output/response.go` uses the content registry when normalizing
  responses
- `internal/cli/http.go` uses the content registry when encoding request bodies

One detail worth preserving is the distinction between `Marshal` and
`MarshalContentType`. That allows formats like multipart to compute the real
wire content type dynamically instead of squeezing everything into a static MIME
string model.
