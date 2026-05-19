---
title: Embedding Restish in Go
linkTitle: Embedding
weight: 115
description: Build a custom Go CLI on top of Restish with custom auth, formats, and bundled API defaults.
aliases:
  - /docs/reference/embedding/
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

The handler implements `restish.AuthHandler`. Use the root package aliases,
such as `restish.AuthParam` and `restish.AuthContext`, rather than importing an
auth subpackage:

```go
type corpTokenHandler struct{}

func (corpTokenHandler) Parameters() []restish.AuthParam {
	return []restish.AuthParam{{Name: "token", Required: true, Secret: true}}
}

func (corpTokenHandler) Authenticate(ctx context.Context, req *http.Request, ac restish.AuthContext) error {
	req.Header.Set("Authorization", "Bearer "+ac.Params["token"])
	return nil
}
```

The handler receives the request context, profile name, params, token store,
and HTTP client used by Restish auth flows.

## Response Cache Safety

The stock CLI partitions cached API responses by API and profile. For example,
requests made through the `billing` API's `prod` profile use a separate cache
namespace from the same API's `default` profile, so credentialed responses are
not reused across profiles.

Embedders get that behavior when they call `Run` or `FetchResponse`, because
Restish applies the configured API/profile before building the request
transport. If you build lower-level `request.Options` yourself and set
`CacheDir`, also set `CacheNamespace` for any request context that may carry
credentials:

```go
opts := request.Options{
	CacheDir:       cacheDir,
	CacheNamespace: apiName + ":" + profileName,
}
```

Use a stable namespace such as `<api>:<profile>`. Disable caching with
`NoCache` for ad hoc credentialed raw requests that do not have a stable
profile boundary.

## Custom Content Or Output

Embedder-facing registration methods let custom CLIs add content types,
encodings, link parsers, loaders, and formatters before running the CLI:

```go
cli := restish.New()
cli.AddContentType(&restish.ContentType{
	Name:      "ion",
	MIMETypes: []string{"application/ion"},
	Marshal:   marshalIon,
	Unmarshal: unmarshalIon,
})
cli.AddFormatter("corp-table", corpFormatter)
```

Custom spec loaders implement `restish.Loader`, whose parse method receives
`restish.LoadOptions` so local paths, source URLs, request context, and
cross-origin reference policy stay available during loading:

```go
func (corpLoader) LoadWithOptions(body []byte, opts restish.LoadOptions) (*restish.APISpec, error) {
	// Parse body and use opts.SourceURL or opts.LocalPath for diagnostics/ref resolution.
}
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
