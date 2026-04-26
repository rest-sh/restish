---
title: Input and Shorthand
linkTitle: Input and Shorthand
weight: 40
description: Build structured request bodies in Restish using shorthand syntax and stdin.
---

Restish can turn CLI arguments into structured request bodies, which makes it
much faster to work with JSON-like input from the terminal.

## Example

```bash
restish post https://api.rest.sh name: restish tags[]: cli tags[]: http enabled: true
```

That command builds a structured value and then lets the content layer decide
how to encode it for the request.

## The Main Rules

Restish builds request bodies using four core rules:

1. no args and TTY stdin means no body
2. stdin alone is parsed as structured input when possible
3. args alone are joined back into one shorthand expression and parsed
4. stdin plus args treats stdin as the base document and applies args as a
   patch

That last behavior is especially useful because it lets you combine generated
or piped structured data with quick command-line overrides.

## Simple Object Input

```bash
restish post https://api.rest.sh name: Alice age: 30
```

Equivalent body:

```json
{
  "name": "Alice",
  "age": 30
}
```

## Nested Objects And Arrays

```bash
restish post https://api.rest.sh \
  user.address.city: NYC \
  user.address.country: USA \
  tags[]: red \
  tags[]: blue
```

Equivalent body:

```json
{
  "user": {
    "address": {
      "city": "NYC",
      "country": "USA"
    }
  },
  "tags": ["red", "blue"]
}
```

## Strings, Nulls, And Empty Values

Restish coerces common literal-looking values automatically:

- `true` and `false`
- `null`
- numbers such as `123` and `1.5`

Quote the value when you want the literal string instead:

```bash
restish post https://api.rest.sh enabled: "true" missing: "null"
```

Generated OpenAPI commands can use request schema information to preserve
string fields automatically. If the spec says `id` is a string, then:

```bash
restish myapi create-item id: 123
```

sends `"id": "123"` for that generated command. Generic HTTP commands keep the
normal shorthand coercion rules.

Use `""` or a blank value for an empty string:

```bash
restish post https://api.rest.sh blank1: blank2: ""
```

## Patch Piped Input

```bash
echo '{"name":"Bob","age":25}' | \
  restish post https://api.rest.sh name: Alice
```

Equivalent body:

```json
{
  "name": "Alice",
  "age": 25
}
```

## When Stdin Is Not Structured

If stdin is not parseable as structured shorthand, JSON, or YAML, Restish falls
back to treating it as a raw string body when no shorthand args are present.

That means simple pass-through workflows still work well:

```bash
cat payload.txt | restish post https://api.rest.sh
```

## File Input And Forms

One subtle but important rule is that file-reference shorthand is
content-type-aware.

For form-style content types such as `multipart/form-data`, Restish does not
eagerly interpret values like `@upload.txt` as shorthand file input. It keeps
them literal so form submissions behave predictably.

## Shell Quoting

If your shell expands `[]` or `?` before Restish sees the input, either quote
the arguments or install shell setup:

```bash
restish setup zsh
restish post https://api.rest.sh 'tags[]: red' 'tags[]: blue'
```

## When To Use Shorthand

Shorthand is best when:

- the body is small
- the shape is mostly object- or array-like
- you want quick exploratory requests
- you are patching structured stdin

Prefer files or piped input when:

- the document is large
- you want to preserve exact formatting
- the payload is already being produced by another tool

## Learn More

- [Requests](../requests/)
- [Shorthand Syntax](/docs/reference/shorthand/)
- [Query Syntax](/docs/reference/query-syntax/)
- [Design Records](/docs/contributing/design-records/)
