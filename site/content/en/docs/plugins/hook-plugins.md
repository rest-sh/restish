---
title: Hook Plugins
linkTitle: Hook Plugins
weight: 20
description: Learn how auth, middleware, loader, and formatter hook plugins fit into the Restish request lifecycle.
---

Hook plugins are short-lived extensions that handle one focused piece of
Restish behavior per invocation.

Typical uses:

- auth
- request middleware
- response middleware
- spec loading
- output formatting

If your feature needs its own command tree or multiple round trips, use a
[command plugin](../command-plugins/) instead.

## Lifecycle

Most hook plugins use a one-shot request/reply pattern:

1. Restish starts the plugin process
2. Restish writes one CBOR message to stdin
3. the plugin writes one reply to stdout
4. the plugin exits

Formatter plugins are the exception. They receive a short session of
`formatter` messages so they can keep state across paginated or streamed
output.

## Common Hook Types

### Auth

Auth plugins receive request metadata and return updates such as new headers.

Input shape:

```json
{
  "api": "myapi",
  "profile": "default",
  "params": {
    "token_file": "/tmp/token"
  },
  "request": {
    "method": "GET",
    "uri": "https://api.example.com/items",
    "headers": {
      "Accept": ["application/json"]
    }
  }
}
```

Typical output:

```json
{
  "headers": {
    "Authorization": ["Bearer abc123"]
  }
}
```

### Request Middleware

Request-middleware plugins run after Restish has prepared the outbound request.
They are best used for header-level adjustments that still belong in the normal
request pipeline.

Typical use:

- add a tracing header
- normalize a custom header format
- attach a request ID from local context

### Response Middleware

Response-middleware plugins receive the normalized response and can:

- replace or merge response fields
- drop output entirely
- ask Restish to follow another URI

Typical follow response:

```json
{
  "follow": {
    "method": "GET",
    "uri": "https://api.example.com/next-page"
  }
}
```

Typical use:

- follow a server-directed next step
- strip or replace noisy fields before output
- normalize a custom API envelope

### Loader

Loader plugins convert non-built-in API description formats into an OpenAPI
document that Restish can load normally.

Typical use:

- translate a custom service description format into OpenAPI
- normalize a variant MIME type before the host spec loader runs

### Formatter

Formatter plugins render output for `-o <name>`. They receive `start`, `item`,
and `end` events and write raw bytes to stdout.

Typical use:

- CSV output
- a domain-specific pretty printer
- record-oriented output for paginated or streamed responses

## Minimal Go Example

This formatter plugin handles one custom output format named `hello`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/plugin"
)

func main() {
	manifest := plugin.Manifest{
		Name:              "hello-format",
		Version:           "0.1.0",
		Description:       "Example formatter plugin",
		RestishAPIVersion: 2,
		Hooks:             []string{"formatter"},
		FormatterNames:    []string{"hello"},
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	var req plugin.FormatterRequest
	if err := dec.ReadMessage(&req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if req.Event == "start" {
		fmt.Fprintln(os.Stdout, "hello from plugin")
	}
}
```

## Pitfalls

- Hook plugins should do one focused job and exit. If your design needs a
  long-lived back-and-forth exchange, it is probably a command plugin.
- Formatter plugins must tolerate `start`, zero or more `item`, then `end`.
- Response-middleware follow requests only carry `method` and `uri`. They are
  not appropriate for workflows that need extra headers or a request body.
- Auth plugins only receive secret auth params when the manifest explicitly
  opts into `needs_auth_secrets`.

## Related Pages

- [Command Plugins](../command-plugins/)
- [Plugin Manifest](../reference/plugin-manifest/)
- [Plugin Message Reference](../reference/plugin-messages/)
- [Built-In Example Plugins](../example-plugins/)
- [Plugin Quickstart](/docs/plugins/quickstart/)
- [Design Record 019](/docs/contributing/design-records/)
