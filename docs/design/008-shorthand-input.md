# Shorthand Input

## Summary

Restish v2 can build request bodies from positional CLI arguments using
shorthand syntax, optionally merging those arguments with structured input read
from stdin.

Shorthand is a body-construction language, not a wire format. Its output is a
logical value that later flows through the content-type registry for final
serialization.

## Goals

- make small structured request bodies fast to type
- support nested objects and arrays without forcing users to write full JSON
- compose cleanly with stdin-provided documents
- provide patch-like refinement of piped input
- keep the shorthand language reusable across other structured-input surfaces

## Non-Goals

- replacing JSON or YAML as full document authoring formats
- coupling shorthand semantics to one transport media type
- forcing users to learn different mini-languages for bodies, config edits, and
  simple path selection where one shared language can work

## Design Position

Shorthand exists at the **value construction** layer of the request pipeline.

That means:

1. Restish decides whether there is a body and where the base value comes from.
2. Shorthand constructs or patches a logical Go value.
3. The content registry serializes that value as JSON, YAML, form data, or
   another selected media type.

This separation is important. It keeps shorthand focused on document shape
instead of transport encoding details.

## Body Source Resolution

Request body construction follows this decision order:

1. no positional body args and TTY stdin -> no body
2. stdin only -> parse stdin as the base body
3. shorthand args only -> parse shorthand as the full body
4. stdin plus shorthand args -> parse stdin as the base body, then apply the
   shorthand patch

For generic bare-target invocations such as `restish https://api.example.com`
or `restish my-api`, this body decision also controls method inference: no body
sends `GET`, while a body from shorthand or stdin sends `POST`. Explicit verb
commands and generated OpenAPI commands keep their resolved method.

This patching behavior is one of the defining Restish workflows. It lets users
generate or fetch a document elsewhere, then refine it at the command line
without rebuilding it from scratch.

## Base Input From Stdin

When stdin provides the base body, Restish should:

- treat structured input as structured data when it can be decoded
- preserve plain text as plain text when structured decoding does not apply
- allow shorthand patching only when the base value can be represented as a
  mutable structured value
- cap stdin body reads at 16 MiB and fail clearly when the cap is exceeded

With stdin only and no shorthand arguments, non-structured text is still a
valid request body. It should be sent as a plain string/text value instead of
failing because no JSON or YAML parser accepted it.

If stdin is binary and the selected content-type path does not support a safe
structured patch workflow, Restish should not pretend shorthand patching is
possible.

## Shorthand Parsing Model

Shorthand arguments arrive from the shell already split. Restish reconstructs
the expression by joining the already-split args with spaces and then feeds that
expression into the shorthand parser.

That design preserves shell ergonomics while keeping the parser itself as the
single source of truth for the resulting value.

## Core Constructs

Typical forms include:

- `name: Alice`
- `user.address.city: NYC`
- `tags[]: red`
- `tags[0]: first`
- `enabled: true`

These expressions construct nested objects and arrays using a compact path-like
syntax.

## Type Coercion

Unquoted scalar values are coerced by the shorthand parser into a logical type
such as:

- string
- integer
- float
- boolean
- null

Quoting forces string semantics where needed. Restish should preserve the parser
library's type-coercion contract rather than reinterpreting those values later
in the CLI layer.

Generated-command request bodies follow the same rule. OpenAPI schemas can make
the expected type visible in help and generated body examples, but they do not
silently rewrite shorthand values. If an API expects a string that looks like a
number, the user must quote it, for example `id: "123"`. Keeping generated and
generic request bodies schema-agnostic avoids command-specific surprises and
preserves the content registry's final serialization behavior.

## Array And Object Semantics

Shorthand must support:

- object field assignment
- array append via `[]`
- array index assignment via `[n]`
- nested object creation under arrays

When shorthand patches an existing base document, array and object behavior
should follow the shorthand library's normal structural rules rather than a
separate Restish-specific patch dialect.

## Patch Semantics

When stdin and shorthand args are both present, stdin is the base document and
shorthand applies as a structural patch.

That means:

- fields named in shorthand are added or replaced
- unspecified fields remain from the base document
- patch directives such as delete or move operate relative to the base document

This is intentionally closer to "document refinement" than to raw text
substitution.

## Special Forms

The shorthand language includes several special forms that matter to Restish's
overall design.

### `undefined`

`undefined` removes a field from the resulting document when patching an
existing structure.

This is the main deletion primitive for:

- request-body patching
- config-edit shortcuts that reuse shorthand semantics

### `^`

The move operator reassigns a value from one path to another and removes the
source path.

This is useful for structural reshaping of a piped base document without
dropping into jq.

### `@`

File reference loads a file and uses its content as the value. For structured
files, the value may be parsed and inlined as a structured value.

### `%`

Base64 file load reads file bytes and inserts a base64-encoded string value.
This is the right tool for embedding binary content into JSON-like bodies.

### `j`

The JSON-literal helper allows an inline JSON object or array to be inserted as
an exact structured value inside a larger shorthand expression.

### `//`

Comments are ignored by the parser. This matters when shorthand is embedded in
files or generated workflows rather than typed only as one shell command.

## Content-Type Interaction

Shorthand is content-type aware in one important way: not every request-body
mode should reinterpret `@something` or other special values the same way.

For form-style submissions, Restish may need to preserve literal values rather
than eagerly converting them as shorthand file references. The body-construction
layer therefore owns the decision of when shorthand special forms are active and
when a content-type mode should keep values literal.

This is particularly important for:

- `application/x-www-form-urlencoded`
- `multipart/form-data`
- `application/octet-stream` and other raw binary request media

Generated OpenAPI commands use the same rule. Shorthand builds a logical value;
the selected media encoder decides whether that value becomes JSON, URL-encoded
fields, multipart fields and files, or raw bytes. For multipart bodies, a scalar
string beginning with `@` is interpreted by the multipart encoder as a file part
reference and fails locally if the path cannot be read. A scalar string
beginning with `@@` escapes this multipart-only rule and sends a literal text
value beginning with `@`.

## Reuse Outside Request Bodies

Shorthand is not only for request bodies.

The same language or a deliberately compatible subset is reused for:

- config patch surfaces such as `api set`
- simple path-oriented filtering and projection

This reuse is intentional because it lowers the number of distinct mini-languages
users need to carry around. The same mental model for "nested paths and
structural updates" should work across several Restish workflows.

## Error Handling

Shorthand parse errors should be surfaced as local CLI errors, not deferred into
later encoding or request execution where the user loses context.

Errors should identify:

- the failing expression
- structural issues such as invalid array syntax where possible
- whether the failure happened while parsing a full shorthand body or while
  patching a base document

## Examples

Simple object body:

```bash
restish post https://api.example.com/users name: Alice age: 30
```

which builds a logical value equivalent to:

```json
{
  "name": "Alice",
  "age": 30
}
```

Nested object body:

```bash
restish post https://api.example.com/users user.address.city: NYC
```

which becomes:

```json
{
  "user": {
    "address": {
      "city": "NYC"
    }
  }
}
```

Piped input patched by shorthand args:

```bash
echo '{"name":"Bob","age":25}' | restish post https://api.example.com/users name: Alice
```

which produces:

```json
{
  "name": "Alice",
  "age": 25
}
```

Delete and move during patching:

```bash
echo '{"old":"value","role":"user"}' | \
  restish post https://api.example.com/users new: ^old role: undefined
```

## Alternatives Considered

### Require Full JSON Or YAML Bodies

Explicit, but too verbose for exploratory CLI usage.

### Treat Shorthand As A Transport Encoding

Rejected because it would blur body construction and wire encoding.

### Ignore Stdin Once Shorthand Args Are Present

Simpler, but it would remove one of Restish's nicest composition patterns.

## Relationship To Other Designs

- Design 003 defines how the resulting logical value is encoded.
- Design 007 relies on shorthand for generated-command request bodies.
- Design 010 reuses compatible shorthand path semantics for simple filtering.
- Design 014 uses shorthand patching inside the edit workflow.
