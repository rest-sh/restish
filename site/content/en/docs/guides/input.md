---
title: Input and Shorthand
linkTitle: Input and Shorthand
weight: 30
description: Build structured request bodies with shorthand, stdin, forms, multipart uploads, and files.
aliases:
  - /docs/recipes/patch-piped-json-with-shorthand/
  - /docs/recipes/send-a-form-login-request/
---

Restish shorthand lets you create structured request bodies directly on the
command line. JSON is the default request encoding, but the same body model can
be encoded as YAML, form data, multipart, CBOR, and other registered types.

## Object Input

{{< restish-example >}}
restish post api.rest.sh/post 'name: Alice, enabled: true' count: 3
{{< /restish-example >}}

The `/post` endpoint echoes the parsed body so you can confirm the result.

## Nested Objects And Arrays

{{< restish-example >}}
restish post api.rest.sh/post \
  user.name: Alice \
  user.roles[]: admin \
  user.roles[]: editor \
  active: true
{{< /restish-example >}}

Use quotes when your shell would otherwise treat brackets or spaces specially:

{{< restish-example >}}
restish post api.rest.sh/post 'tags[]: docs' 'tags[]: cli'
{{< /restish-example >}}

## Strings, Nulls, And Empty Values

Shorthand coerces common scalar values. Force strings with quotes when the exact
text matters:

{{< restish-example >}}
restish post api.rest.sh/post 'enabled: "true", missing: "null", blank: ""'
{{< /restish-example >}}

## Stdin And Patches

Use stdin for larger payloads:

```bash
echo '{"name":"Alice","role":"user"}' | restish post api.rest.sh/post
```

When stdin is not structured JSON or YAML, Restish preserves it as a plain text
request body:

```bash
printf 'hello from stdin' | restish post -c text api.rest.sh/post
```

Add shorthand arguments to patch structured stdin before sending:

```bash
echo '{"name":"Alice","role":"user"}' | restish post api.rest.sh/post role: admin
```

## Form Bodies

Use `-c form` for URL-encoded request bodies:

{{< restish-example >}}
restish post -c form api.rest.sh/login 'username: alice, password: secret'
{{< /restish-example >}}

Representative output:

```json
{
  "token": "docs-token-alice",
  "token_type": "Bearer",
  "user": "alice"
}
```

## Multipart Uploads

Use `-c multipart` for form-style uploads. The example API echoes normal
fields and reports file metadata when the request contains real file parts:

```bash
restish post -c multipart api.rest.sh/uploads \
  description: docs, \
  file: @README.md
```

The response echoes multipart field values. When a client sends real file parts, `/uploads` also reports file metadata such as field name, filename, content type, and size.

## File Loading

```bash
restish post api.rest.sh/post payload: @payload.json
restish post api.rest.sh/post note: @message.txt
```

Structured files are parsed when possible. Quote or force string behavior when a
literal `@` should be sent as text.

## Related Pages

- [Shorthand Reference](/docs/reference/shorthand/)
- [Content Types](/docs/reference/content-types/)
- [Requests](../requests/)
