---
title: Example API
linkTitle: Example API
weight: 15
description: Canonical api.rest.sh endpoints and commands used throughout the Restish docs.
---

The docs use `https://api.rest.sh` whenever a live endpoint makes a Restish
workflow clearer. The API is intentionally broad: it has OpenAPI discovery,
request echoing, auth fixtures, forms, uploads, streaming, pagination, retries,
content negotiation, binary responses, redirects, and safe CRUD examples.

Configure it once when you want short API-aware commands:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

Use full URLs when you want to stay in generic HTTP mode:

```bash
restish https://api.rest.sh/
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
```

## First Requests And Inspection

| Endpoint | Use it for |
| --- | --- |
| `/` | Echo a basic request and show headers Restish sends. |
| `/headers` | Return request headers only. |
| `/user-agent` | Inspect the `User-Agent` header. |
| `/response-headers?Name=value` | Make the server set response headers. |
| `/anything` and `/anything/{path}` | Echo method, URL, path, query, headers, and body. |
| `/get`, `/post`, `/put`, `/patch`, `/delete`, `/head`, `/options` | Focused generic HTTP verb examples. |

```bash
restish https://api.rest.sh/
restish -H 'X-Demo: docs' 'https://api.rest.sh/anything/docs?active=true'
```

## API-Aware Commands

The API publishes an OpenAPI document at `/openapi.json`. After configuration,
Restish generates commands such as:

```bash
restish example list-images
restish example get-image jpeg
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

```bash
restish post https://api.rest.sh/post 'name: Alice, enabled: true'
restish post -c form https://api.rest.sh/login 'username: alice, password: secret'
```

## Auth Sandbox

The auth endpoints require credentials but return only safe summaries.

| Endpoint | Required auth |
| --- | --- |
| `/auth/basic` | HTTP Basic auth |
| `/auth/bearer` | `Authorization: Bearer ...` |
| `/auth/api-key-header` | `X-API-Key` header |
| `/auth/api-key-query` | `api_key` query parameter |

```bash
restish -H 'Authorization: Bearer docs-token' https://api.rest.sh/auth/bearer
restish -H 'X-API-Key: docs-key' https://api.rest.sh/auth/api-key-header
```

## Collections, Links, And CRUD

| Endpoint | Use it for |
| --- | --- |
| `/images` | Pagination, links, table output, filtering, and image lists. |
| `/images/{type}` | Raw image downloads and terminal image rendering. |
| `/items` and `/items/{item-id}` | Safe generic CRUD examples. |
| `/books` and `/books/{book-id}` | Bulk-management workflows. |
| `/example` | Nested data for filtering and projection examples. |

```bash
restish https://api.rest.sh/images -f links.next -r
restish https://api.rest.sh/example -f body.basics.profiles
restish post https://api.rest.sh/items 'id: docs-demo, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z'
```

## Streaming

| Endpoint | Use it for |
| --- | --- |
| `/events` | Server-Sent Events with docs-shaped JSON event data. |
| `/logs` | NDJSON log records. |
| `/sse/metrics` | Metrics-shaped SSE events. |

Always bound copy-paste stream examples:

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
restish https://api.rest.sh/events --rsh-max-events 3 -f data.user.id -r
```

## Resilience And HTTP Behavior

| Endpoint | Use it for |
| --- | --- |
| `/flaky?failures=2&key=docs` | Retry recovery examples. |
| `/slow?delay=2s` | Timeout examples. |
| `/status/{code}` | Exit status and error-body examples. |
| `/cache`, `/cached/{seconds}`, `/etag/{etag}` | HTTP cache and conditional requests. |
| `/redirect/{n}`, `/relative-redirect/{n}`, `/absolute-redirect/{n}`, `/redirect-to` | Redirect behavior and verbose transcripts. |

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs' --rsh-retry 2
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

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

```bash
restish -H 'Accept: application/json' https://api.rest.sh/formats/json
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

## Related Pages

- [Quickstart](/docs/getting-started/quickstart/)
- [Requests](/docs/guides/requests/)
- [Authentication](/docs/guides/authentication/)
- [Content Types](/docs/reference/content-types/)
- [HTTP Commands](/docs/reference/http-commands/)
