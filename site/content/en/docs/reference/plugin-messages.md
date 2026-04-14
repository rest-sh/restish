---
title: Plugin Messages
linkTitle: Plugin Messages
weight: 44
description: Reference for the CBOR message model used by Restish plugins.
---

Restish plugins communicate over stdin/stdout using CBOR. Each message is one
self-delimiting CBOR data item. There is no extra length prefix or custom
frame wrapper.

## Hook Plugins

Most hook plugins are one-shot. Restish writes one request message and expects
one reply, except for formatter plugins, which receive a short formatter
session and write raw formatted bytes to stdout.

### Auth Hook

Input includes API name, profile name, auth params, and outbound request data.
Reply data typically updates request headers.

### Request Middleware Hook

Input includes the prepared request. Reply data can update outbound headers.

### Response Middleware Hook

Input includes request metadata plus normalized response status, headers, and
body. Reply data may drop the response, ask for a follow-up request, or replace
response fields.

### Loader Hook

Input includes the raw fetched spec body and content type. Reply data returns
OpenAPI bytes or text.

### Formatter Hook

Formatter plugins receive `formatter` messages with:

- `format`
- `color`
- `event`: `start`, `item`, or `end`
- normalized response metadata and/or body in `response`

For ordinary non-streaming responses, Restish usually sends the full body on
the `start` message and then `end`. For paginated or event-stream output,
Restish sends `start`, one or more `item` messages, then `end`.

Output is raw formatted bytes, not a CBOR reply envelope.

## Command Plugins

Command plugins are long-lived and exchange many messages in one session.

### Initial Host Message

Restish starts the command with an `init` message containing the declared
command name and raw plugin args.

### Messages A Plugin Can Send

- `http-request`
- `api-spec`
- `response`
- `stdout-data`
- `stderr-data`
- `progress`
- `spinner`
- `log`
- `warn`
- `done`

### Messages Restish Sends Back

- `http-response`
- `api-spec-response`
- `stdin-data`
- `stdin-close`

The key pattern is delegated HTTP: the plugin asks Restish to make the request,
and Restish applies auth, retries, TLS, cache, and normalization on its behalf.

## TLS Signer Plugins

TLS signer plugins are persistent signer processes used during mTLS setup.

Typical flow:

1. Restish sends `init` with plugin parameters.
2. Plugin replies with `ready` and the leaf certificate.
3. During TLS handshakes, Restish sends `sign` with a digest and hash id.
4. Plugin replies with a signature or an error.

## Go Helper Package

Go plugins should build against the public `plugin` package, which provides:

- `WriteMessage`
- `ReadMessage`
- `NewDecoder`
- `HandleStartupFlags`
- `Run`

See:

- [Plugin Quickstart](/docs/plugins/quickstart/)
- [Hook Plugins](/docs/plugins/hook-plugins/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/)
