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
- optional suffix matches such as `+json`
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
- `ion`
- `form`
- `multipart`
- `text`
- `binary`

Built-in encodings include:

- `br`
- `gzip`
- `deflate`

## Media-Type Matching Rules

Media-type matching must handle:

- exact MIME types
- wildcard fallbacks such as `text/*`
- structured suffixes such as `+json`, `+yaml`, `+cbor`, `+msgpack`, and `+ion`

Structured suffix support is essential for common API responses such as:

- `application/problem+json`
- `application/hal+json`
- `application/vnd.api+json`
- `application/ld+json`

These are part of normal API usage, not rare edge cases.

## Selection Algorithm

Response decoder selection should follow a stable order:

1. exact MIME-type match
2. structured-suffix match such as `+json`
3. wildcard fallback such as `text/*`
4. unknown-content fallback rules

The important design point is determinism. The selected decoder should not
depend on registration order in ways users cannot reason about.

If multiple entries could match within the same tier, the implementation should
apply a stable tie-break order:

1. higher explicit specificity over lower specificity
2. built-in exact registration order only when two handlers are intentionally
   aliases for the same wire format
3. otherwise treat ambiguous registration as a startup-time bug

This prevents "last plugin wins" behavior for core media-type resolution.

## Request Content-Type Selection

Request serialization needs an equally explicit selection model. The planner
must determine both the logical body family and the concrete wire header.

Conceptually, request body handling proceeds as follows:

1. determine whether the command invocation produced a body at all
2. determine whether the user explicitly selected a content type
3. if explicit, resolve that name or MIME type through the registry
4. otherwise infer a logical content type from the body source:
   - shorthand/object/array structured values default to a structured encoder
   - raw text input defaults to text when no structured parse occurred
   - file or stream passthrough may preserve raw bytes instead of re-encoding
5. ask the selected encoder for the concrete wire `Content-Type`
6. serialize the body and attach the final header

The key rule is that body shape and body source both matter. A pipeline that
received raw bytes from stdin should not silently reinterpret them as JSON just
because JSON is the most common format in the registry.

## Unknown Content Types

When Restish does not recognize a response content type, it must preserve the
payload safely.

The design rule is:

- printable text-like unknown payloads may be surfaced as text
- otherwise unknown payloads remain raw bytes

Unknown binary must not be coerced to a Go `string`, because that corrupts the
payload and creates misleading later output.

The printable-text decision should be based on body inspection after transport
decoding, not on optimistic assumptions from missing or incorrect server
metadata. If Restish cannot classify the body confidently as text, it should
prefer the raw-bytes path.

## Missing Or Invalid Content-Type Headers

Servers do not always send correct `Content-Type` headers. The decoding model
must therefore define behavior for:

- no `Content-Type` header at all
- malformed `Content-Type` values
- mismatched headers such as binary content labeled `application/json`

The preferred behavior is:

1. attempt header parsing
2. if parsing fails, record the parse issue for diagnostics
3. fall back to unknown-content rules rather than guessing a structured format
4. if parsing succeeds but the selected decoder fails, surface the decoder error
   rather than silently retrying unrelated formats

This keeps decoding failures explainable and avoids accidental format probing
that could hide server-side bugs.

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

The negotiation algorithm is:

1. gather all registered MIME types
2. include suffix-driven families only through their concrete MIME registrations
3. sort by descending quality
4. preserve stable ordering among equal-quality entries
5. emit the resulting header

Implementations should also deduplicate equivalent MIME types after
normalization. If two plugins register the same canonical media type, the
registry must resolve that conflict before header generation rather than
advertising duplicates.

A representative built-in header conceptually looks like:

```text
application/cbor;q=0.9, application/msgpack;q=0.8, application/json;q=0.5, application/yaml;q=0.5, text/*;q=0.2
```

Quality ordering should be stable and deliberate.

Restish should not advertise suffix forms like `application/*+json` unless the
runtime has an explicit reason to do so. Accept generation is based on concrete
formats the client is prepared to decode, not speculative wildcard families.

## Accept-Encoding Negotiation

`Accept-Encoding` is generated from the encoding registry and should only
advertise encodings Restish can actually decode.

The generation rule is parallel to content negotiation:

1. gather supported encodings
2. sort by descending quality
3. emit the resulting header

Example:

```text
br, gzip, deflate
```

If an operator explicitly overrides `Accept-Encoding`, Restish should treat that
as a complete override. The registry still governs what the client can decode,
but automatic header synthesis must not fight explicit user input.

## Multipart And Dynamic Content Types

Some request encoders need to compute the real wire content type at runtime.
`multipart/form-data` is the main example because the boundary is generated per
request.

This is why the design distinguishes:

- logical content-type family
- concrete wire `Content-Type` value

The request-construction layer should treat those as related but not identical
concepts.

The same rule applies to any encoder whose final wire header includes generated
parameters. The logical content family drives body construction, while the
concrete header is produced late enough to include runtime-generated values.

## Form And Multipart Request Semantics

OpenAPI-generated commands and generic HTTP commands share the same form
encoders.

For `application/x-www-form-urlencoded`, object values are encoded as field
names and scalar values. Nested object fields use stable bracketed keys such as
`metadata[color]`, and arrays are represented by repeated values such as
`tags[]=red&tags[]=blue` unless a more specific OpenAPI parameter serialization
rule owns the location.

For `multipart/form-data`, scalar fields become text parts and file or binary
values become file parts. Array file fields are represented by repeated parts
with the same field name, because that is what common multipart APIs expect for
multi-file uploads. Generated OpenAPI commands pass `encoding.contentType`
metadata into the multipart encoder so individual parts can carry the media
type declared by the spec.

For `application/octet-stream` and other binary request media, Restish preserves
raw bytes from files or stdin rather than re-encoding them as structured text.
Unknown binary responses follow the same preservation rule.

## Compression And Body Limits

Transport decompression happens before content decoding, which means the body
classification logic operates on the decompressed bytes. This implies:

- decompression errors are transport-level failures, not decoder failures
- body size limits, if configured, should specify whether they apply before or
  after decompression
- downstream decoders should not need to understand gzip, br, or deflate at all

That separation is important for correctness and for clean responsibility
boundaries in the request pipeline.

## Streaming Reuse

The same registry model should be reusable in streaming contexts, but with a
different execution shape:

- whole-document decoders consume a complete response body
- stream decoders consume framed items or incremental events
- unknown-content fallbacks still preserve bytes or text without inventing
  structure

This document therefore defines the registry contract, while design 012 defines
how stream-oriented consumers invoke that contract incrementally.

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
- Design 034 defines the OpenAPI media-type cases that generated commands map
  into this registry.
- Design 009 relies on this model for response normalization.
- Design 012 relies on this model for stream item decoding where applicable.
- Design 030 relies on this model not to corrupt binary payloads.
