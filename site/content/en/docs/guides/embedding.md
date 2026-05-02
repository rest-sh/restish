---
title: Embedding Restish in Go
linkTitle: Embedding
weight: 115
description: Build a custom Go CLI on top of Restish with custom auth, formats, and bundled API defaults.
---

Restish can be used as a Go library when an organization wants to ship a
branded CLI while keeping Restish's request pipeline, OpenAPI command
generation, auth, output, and plugin behavior.

Use out-of-process plugins for the stock `restish` binary. Use embedding when
you own the binary and need in-process defaults or extensions.

## Minimal Custom CLI

```go
package main

import (
	"os"

	restish "github.com/rest-sh/restish/v2"
)

func main() {
	cli := restish.New()
	if err := cli.Run(os.Args); err != nil {
		cli.HandleError(err)
	}
}
```

`Run` installs SIGINT/SIGTERM handling by default so Ctrl-C cancels in-flight
requests in the stock CLI. If your application already owns process signal
handling, disable Restish's handler before running:

```go
cli := restish.New()
cli.SetSignalHandling(false)
```

## Custom Auth

Register an auth handler under the `auth.type` name your config will use:

```go
cli := restish.New()
cli.AddAuthHandler("corp-token", corpTokenHandler{})
```

The handler implements the auth interfaces from
`github.com/rest-sh/restish/v2/auth`. It receives the request context, profile
name, params, token store, and HTTP client used by Restish auth flows.

## Custom Content Or Output

Embedder-facing registration methods let custom CLIs add content types,
encodings, link parsers, loaders, and formatters before running the CLI:

```go
cli := restish.New()
cli.AddContentType("ion", []string{"application/ion"}, ionCodec)
cli.AddFormatter("corp-table", corpFormatter)
```

Prefer plugins when the extension should also work with the stock `restish`
binary. Prefer in-process registration when the custom CLI must ship a
company-specific default without an external executable.

## Bundled Defaults

A custom CLI can install in-memory bundled config before calling `Run`:

```go
cli := restish.New()
cli.SetDefaultConfig(defaultConfig)
```

User config overrides bundled defaults with the same API or auth-profile name.
Config-writing commands such as `api connect` keep those bundled defaults in
the running `CLI` after the write completes. A custom CLI can also load or write
a Restish config before calling `Run`, or ship project templates that users
connect with:

```bash
mycorp api connect billing https://billing.example.com --spec ./openapi.yaml
```

Keep config explicit. `RSH_CONFIG_DIR`, `RSH_CONFIG`, and `--rsh-config` keep
the same precedence rules as the stock binary.

## Related Pages

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Authentication](/docs/guides/authentication/)
- [Plugins](/docs/plugins/)
- [Plugin Quickstart](/docs/plugins/quickstart/)
