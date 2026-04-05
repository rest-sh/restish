# Shorthand Input

## Summary

Restish v2 can build request bodies from positional CLI arguments using
shorthand syntax, optionally merging those arguments with structured input read
from stdin.

This gives the CLI a compact way to express request payloads without forcing
users to write full JSON for common cases.

## Problem

Typing JSON directly in a shell is noisy and error-prone, especially for common
API interactions where the body is small and mostly object-shaped.

At the same time, Restish still needs to work well with piped input and with
structured formats coming from other tools. The request body design therefore
needed to:

- keep small ad hoc payloads fast to type
- support nested object construction from the command line
- preserve compatibility with piped structured input
- allow stdin data to be refined or patched by command-line arguments
- leave final encoding to the content-type layer rather than baking it into the
  parser

## Design

Restish treats shorthand as a body-construction language, not a wire format.

The input stage produces a Go value representing the request body. That value is
then handed to the content registry, which is responsible for encoding it into
JSON, YAML, or another configured media type.

The main rules are:

1. no args and TTY stdin means no body
2. stdin alone is parsed as structured input when possible
3. args alone are joined back into a single shorthand expression and parsed
4. stdin plus args treats stdin as the base document and applies args as a
   shorthand patch

That patching behavior is a key part of the design. It lets users combine
generated or piped structured data with quick command-line overrides without
building an entirely new document from scratch.

One important constraint is that file-reference shorthand is content-type aware.
For form-style submissions, Restish keeps values like `@upload.txt` literal
instead of eagerly interpreting them as shorthand file input.

The shorthand syntax is mainly used to construct nested object and array values
compactly from shell arguments. Typical forms include:

- `name: Alice`
- `user.address.city: NYC`
- `tags[]: red, tags[]: blue`
- `enabled: true`

## Examples

Simple object body:

```bash
restish post https://api.example.com/users name: Alice age: 30
```

which builds a structured value equivalent to:

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

which produces a body equivalent to:

```json
{
  "name": "Alice",
  "age": 25
}
```

## Alternatives Considered

### Require full JSON or YAML bodies

This is explicit, but too verbose for the kind of quick exploratory and
iterative API usage Restish is designed to support.

### Treat shorthand as a transport-specific encoding

That would blur the boundary between body construction and content encoding. It
is cleaner to produce a structured value first, then let the content layer
decide how to serialize it.

### Ignore stdin once shorthand args are present

This would simplify implementation, but it would make command-line patching of
piped data much less useful. Supporting a base document plus patch preserves a
nice compositional workflow.

## Notes

The current implementation reflects this design directly:

- `internal/input/body.go` defines the body-construction rules for args and
  stdin
- `internal/cli/http.go` calls body construction before delegating final
  encoding to the content registry
- `internal/input/body_test.go` covers nested shorthand, stdin passthrough, and
  stdin-plus-args patching behavior

One detail worth preserving is that shorthand parsing reconstructs the shell
expression by joining already-split CLI args with spaces. That keeps the command
line ergonomic while still using the shorthand parser as the source of truth for
the resulting structured value.
