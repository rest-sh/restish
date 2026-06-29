# Example CLI

This example is a small branded CLI built by embedding Restish. It uses
`https://api.rest.sh` as the bundled API, fetches that API's OpenAPI document
from `https://api.rest.sh/openapi.json`, and promotes generated operations to
the root command.

Build it from the repository root:

```bash
go build -o /tmp/example-cli ./examples/example-cli
```

Run it with an isolated temporary config file:

```bash
tmp_config="$(mktemp)"
printf '{}' > "$tmp_config"
chmod 600 "$tmp_config"
RSH_CONFIG="$tmp_config" /tmp/example-cli --help
RSH_CONFIG="$tmp_config" /tmp/example-cli list-items -o json
```

The important embedding pieces are in [main.go](./main.go):

```go
cli.SetCommandName("example")
cli.SetCommandDescription("Example CLI", "Example CLI for api.rest.sh.")
cli.SetDefaultConfig(&restish.Config{APIs: map[string]*restish.APIConfig{
	"api": {
		BaseURL: "https://api.rest.sh",
		SpecURL: "https://api.rest.sh/openapi.json",
	},
}})
cli.SetCommandSurface(restish.CommandSurface{
	PromotedAPI: "api",
})
```

Use this as a starting point by changing the command name, description, API
base URL, OpenAPI spec URL, default profiles, and auth configuration. Keep the
API key `"api"` unless you have a reason to expose more than one API; it keeps
the public command shape focused on generated operations such as
`example list-items`.
