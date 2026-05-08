---
title: Example API
linkTitle: Example API
weight: 15
description: Canonical api.rest.sh endpoints and commands used throughout the Restish docs.
---

The docs use `api.rest.sh` whenever a live endpoint makes a Restish workflow
clearer. Restish infers `https://` for host-like request targets, while config
values and OpenAPI metadata still use full URLs. The API is intentionally broad:
it has OpenAPI discovery, request echoing, auth fixtures, forms, uploads,
streaming, pagination, retries, content negotiation, binary responses,
redirects, and safe CRUD examples.

Configure it once when you want short API-aware commands:

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

Use a host-like URL when you want to stay in generic HTTP mode:

{{< restish-example >}}
restish api.rest.sh/
{{< /restish-example >}}

{{< restish-example >}}
restish api.rest.sh/images -o table --rsh-columns name,format,self
{{< /restish-example >}}

## First Requests And Inspection

| Endpoint | Use it for |
| --- | --- |
| `/` | Echo a basic request and show headers Restish sends. |
| `/headers` | Return request headers only. |
| `/user-agent` | Inspect the `User-Agent` header. |
| `/response-headers?Name=value` | Make the server set response headers. |
| `/anything` and `/anything/{path}` | Echo method, URL, path, query, headers, and body. |
| `/get`, `/post`, `/put`, `/patch`, `/delete`, `/head`, `/options` | Focused generic HTTP verb examples. |

{{< restish-example >}}
restish api.rest.sh/
{{< /restish-example >}}

{{< restish-example >}}
restish -H 'X-Demo: docs' 'api.rest.sh/anything/docs?active=true'
{{< /restish-example >}}

## API-Aware Commands

The API publishes an OpenAPI document at `/openapi.json`. After configuration,
Restish generates commands such as:

{{< restish-example >}}
restish example list-images
{{< /restish-example >}}

{{< restish-example >}}
restish example get-image jpeg
{{< /restish-example >}}

```bash
restish example get-types-example
restish example get-status 404 --rsh-ignore-status-code
```

Use API-aware commands for repeated work, generated help, shell completion, and
profile-aware auth. Use generic URLs for quick exploration.

## Request Bodies

| Endpoint | Use it for |
| --- | --- |
| `/post`, `/put`, `/patch` | Echo JSON, YAML, CBOR, or stdin request bodies. |
| `/types` | Schema-oriented examples and edit workflow. |
| `/login` | URL-encoded form login examples. |
| `/uploads` | Multipart form echo examples, including file metadata when the client sends file parts. |

{{< restish-example >}}
restish post api.rest.sh/post 'name: Alice, enabled: true'
{{< /restish-example >}}

{{< restish-example >}}
restish post -c form api.rest.sh/login 'username: alice, password: secret'
{{< /restish-example >}}

## Auth Sandbox

The auth endpoints require credentials but return only safe summaries.

| Endpoint | Required auth |
| --- | --- |
| `/auth/basic` | HTTP Basic auth |
| `/auth/bearer` | `Authorization: Bearer ...` |
| `/auth/api-key-header` | `X-API-Key` header |
| `/auth/api-key-query` | `api_key` query parameter |

{{< restish-example >}}
restish -H 'Authorization: Bearer docs-token' api.rest.sh/auth/bearer
{{< /restish-example >}}

{{< restish-example >}}
restish -H 'X-API-Key: docs-key' api.rest.sh/auth/api-key-header
{{< /restish-example >}}

## Collections, Links, And CRUD

| Endpoint | Use it for |
| --- | --- |
| `/images` | Pagination, links, table output, filtering, and image lists. |
| `/images/{type}` | Raw image downloads and terminal image rendering. |
| `/items` and `/items/{item-id}` | Safe generic CRUD examples. |
| `/books` and `/books/{book-id}` | Bulk-management workflows. |
| `/example` | Nested data for filtering and projection examples. |

{{< restish-example >}}
restish api.rest.sh/images -f links.next
{{< /restish-example >}}

{{< restish-example >}}
restish api.rest.sh/example -f body.basics.profiles
{{< /restish-example >}}

```bash
restish post api.rest.sh/items 'id: docs-demo, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z'
```

## Streaming

| Endpoint | Use it for |
| --- | --- |
| `/events` | Server-Sent Events with docs-shaped JSON event data. |
| `/logs` | NDJSON log records. |
| `/sse/metrics` | Metrics-shaped SSE events. |

Always bound copy-paste stream examples:

{{< restish-example >}}
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
{{< /restish-example >}}

```bash
restish api.rest.sh/events --rsh-max-items 3 -f data.user.id -o lines
```

## Resilience And HTTP Behavior

| Endpoint | Use it for |
| --- | --- |
| `/flaky?failures=2&key=docs` | Retry recovery examples. |
| `/slow?delay=2s` | Timeout examples. |
| `/status/{code}` | Exit status and error-body examples. |
| `/cache`, `/cached/{seconds}`, `/etag/{etag}` | HTTP cache and conditional requests. |
| `/redirect/{n}`, `/relative-redirect/{n}`, `/absolute-redirect/{n}`, `/redirect-to` | Redirect behavior and verbose transcripts. |

{{< restish-example >}}
restish 'api.rest.sh/flaky?failures=1&key=docs' --rsh-retry 2
{{< /restish-example >}}

{{< restish-example >}}
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
{{< /restish-example >}}

## Content, Binary, And Utilities

| Endpoint | Use it for |
| --- | --- |
| `/formats/{format}` | JSON, YAML, CBOR, and vendor JSON decoding examples. |
| `/problem` | `application/problem+json` error payloads. |
| `/gzip`, `/deflate`, `/brotli` | Response decompression examples. |
| `/image` | Accept-driven image negotiation. |
| `/bytes/{n}`, `/stream-bytes/{n}`, `/range/{n}`, `/drip` | Raw bytes, ranges, and slow byte streams. |
| `/xml`, `/html`, `/uuid`, `/ip`, `/base64/encode/{value}`, `/base64/decode/{value}` | Small utility examples. |
| `/cookies`, `/cookies/set`, `/cookies/delete` | Cookie behavior examples. |

{{< restish-example >}}
restish -H 'Accept: application/json' api.rest.sh/formats/json
{{< /restish-example >}}

```bash
restish api.rest.sh/images/jpeg > dragonfly.jpg
```

## Related Pages

- [Tour of Restish](/docs/getting-started/quickstart/)
- [Requests](/docs/guides/requests/)
- [Authentication](/docs/guides/authentication/)
- [Content Types](/docs/reference/content-types/)
- [HTTP Commands](/docs/reference/http-commands/)
