# Content Types And Encodings

## Summary

Restish v2 uses a registry-driven model for request and response body handling.
Content types describe how to marshal and unmarshal structured values, while
encodings describe how to transparently decompress compressed responses.

This keeps body handling extensible without scattering format-specific logic
throughout the CLI pipeline.

## Goals

- support multiple structured body formats for both requests and responses
- generate predictable `Accept` and `Accept-Encoding` headers
- decode responses into a stable internal value model
- preserve raw bytes when decoding is not appropriate
- keep body serialization separate from wire-level content encoding

## Non-Goals

- assuming JSON is the only structured format worth negotiating
- coercing unknown binary payloads into lossy text
- treating content negotiation as an opaque implementation detail users cannot
  reason about

## Two Registries

The core model is two registries:

- content types for marshaling and unmarshaling body values
- encodings for transparent response decompression

A content-type entry defines:

- a short CLI-facing name like `json` or `yaml`
- one or more MIME types
- a quality value for `Accept` header generation
- marshal behavior
- unmarshal behavior

An encoding entry defines:

- the encoding token like `gzip` or `br`
- a quality value for `Accept-Encoding`
- a decompressor

## Request/Response Flow

The pipeline is:

1. build a logical request body value
2. serialize it using the selected content type
3. send it with the negotiated or explicit `Content-Type`
4. decompress the response if `Content-Encoding` applies
5. decode the response using the selected or detected content type
6. normalize the decoded value for filtering and formatting

Keeping decompression separate from content decoding is important because those
are different concerns with different registries and fallback rules.

## Built-In Content Types

The built-in registry currently includes at least:

- `json`
- `yaml`
- `cbor`
- `msgpack`
- `form`
- `multipart`
- `text`

Built-in encodings include:

- `br`
- `gzip`
- `deflate`

## Media-Type Matching Rules

Media-type matching must handle:

- exact MIME types
- wildcard fallbacks such as `text/*`
- structured suffixes such as `+json`, `+yaml`, and `+cbor`

Structured suffix support is essential for common API responses such as:

- `application/problem+json`
- `application/hal+json`
- `application/vnd.api+json`
- `application/ld+json`

These are part of normal API usage, not rare edge cases.

## Unknown Content Types

When Restish does not recognize a response content type, it must preserve the
payload safely.

The design rule is:

- printable text-like unknown payloads may be surfaced as text
- otherwise unknown payloads remain raw bytes

Unknown binary must not be coerced to a Go `string`, because that corrupts the
payload and creates misleading later output.

## Normalization Rules

Decoded structured values are normalized into JSON-safe structures so
downstream filtering and formatting stay stable across different wire formats.

The normalized representation should preserve:

- object and array shape
- numeric values as faithfully as practical
- raw bytes where structured decoding did not apply

Normalization is for interoperability, not for hiding the difference between
text, structured values, and raw binary.

## Accept Negotiation

Restish should generate a useful `Accept` header based on registered content
types and their quality values.

That ordering is user-visible and can affect what servers return. If CBOR is
preferred over JSON, users may observe CBOR responses from APIs that support
both.

The generated header conceptually looks like:

```text
application/cbor;q=0.9, application/msgpack;q=0.8, application/json;q=0.5, application/yaml;q=0.5, text/*;q=0.2
```

Quality ordering should be stable and deliberate.

## Accept-Encoding Negotiation

`Accept-Encoding` is generated from the encoding registry and should only
advertise encodings Restish can actually decode.

Example:

```text
br, gzip, deflate
```

## Multipart And Dynamic Content Types

Some request encoders need to compute the real wire content type at runtime.
`multipart/form-data` is the main example because the boundary is generated per
request.

This is why the design distinguishes:

- logical content-type family
- concrete wire `Content-Type` value

## Examples

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

and then encodes it as YAML before sending it with the correct `Content-Type`.

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

### Hard-Code JSON Everywhere

Too limiting for a tool intended to interoperate with real APIs.

### Mix Encoding And Content-Type Handling Together

Compression and body serialization are related on the wire but are distinct
design concerns.

### Return Decoder-Native Map Types Unchanged

Too much downstream complexity for filters and formatters.

## Relationship To Other Designs

- Design 008 relies on this model for request-body encoding.
- Design 009 relies on this model for response normalization.
- Design 012 relies on this model for stream item decoding where applicable.
- Design 030 relies on this model not to corrupt binary payloads.
