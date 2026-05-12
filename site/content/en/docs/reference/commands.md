---
title: Commands
linkTitle: Commands
weight: 10
description: Reference index for Restish built-in commands, generic HTTP verbs, generated API commands, and plugin commands.
aliases:
  - /docs/reference/auth-header-command/
  - /docs/reference/api-auth-inspect/
  - /docs/reference/cache-command/
  - /docs/reference/cert-command/
  - /docs/reference/links-command/
  - /docs/reference/setup-command/
  - /docs/reference/theme-command/
---

Restish commands fall into a few groups: generic HTTP requests, API management,
utility commands, generated API commands, and plugin commands. Run any command
with `--help` for exact flags and generated operation help.

## Generic HTTP Commands

```bash
restish api.rest.sh/
restish get api.rest.sh/get
restish post api.rest.sh/post name: Alice
restish put api.rest.sh/put name: Alice
restish patch api.rest.sh/patch enabled: false
restish delete api.rest.sh/delete --rsh-ignore-status-code
restish head api.rest.sh/head
restish options api.rest.sh/options
```

A bare URL without a body is a generic `GET`. A bare URL with shorthand or stdin
body input is a generic `POST`.

## Configuration And Setup

- `api`: manage registered APIs, specs, profiles, and API auth.
- `cache`: inspect and clear HTTP response cache.
- `config`: inspect and edit the active Restish config.
- `shell setup`: write shell wrappers for safer interactive use.
- `config theme`: manage readable-output highlighting theme.

### `api auth inspect`

Inspect configured API auth without sending the target request:

```bash
restish api auth inspect myapi
restish api auth inspect myapi --rsh-credential PartnerKey
restish api auth inspect myapi --rsh-credential UserBearer --raw-header Authorization
```

Use this before debugging a `401` or `403`. If the profile has exactly one
configured credential, `inspect` selects it by default. When a profile has
several credentials, pass `--rsh-credential`. Human output redacts sensitive
values; `--raw-header` is for scripts that need one computed header value.

### `cache`

Inspect and clear the HTTP response cache:

```bash
restish cache info
restish cache info -o json
restish cache clear
restish cache clear example
```

`cache info` prints the cache location, size, entry count, and oldest entry.
`cache clear` with no API name clears all HTTP response cache entries; there is
no special `all` alias. `cache clear example` clears only entries for the
configured API named `example`. Use
`restish api auth logout` for cached OAuth/auth tokens.

### `shell setup`

Install shell wrappers so URLs, filters, and shorthand reach Restish unchanged:

```bash
restish shell setup zsh --dry-run
restish shell setup zsh --yes
restish shell setup zsh --no-completion
restish shell setup bash
restish shell setup fish
```

For zsh and fish, setup installs completion by default. Use `--dry-run` to
preview startup-file changes and `--no-completion` when you only want the
argument wrapper.

### `config theme`

Install readable-output highlighting themes:

```bash
restish config theme set ./themes/one-dark-pro.json
restish config theme set https://example.com/theme.json
restish config theme set user/repo dark
restish config theme reset
```

Local paths are stored as absolute paths. GitHub shorthand resolves `user/repo`
to a raw `theme.json`, or to `<name>.json` when you pass the optional name.
Theme files and downloads are capped at 256 KiB. Themes affect human-readable
terminal output, not `json`, `ndjson`, raw bytes, or other machine-oriented
formats. Theme authors can use `header_key` to color HTTP response header names
differently from JSON/readable object keys. Use `reset` to remove `theme` and
`theme_source` from config and return to the built-in theme.

## Utilities

- `cert`: inspect server TLS certificate chains
- `content-types`: list registered content types and MIME types
- `doctor`: diagnose config, runtime paths, APIs, plugins, and v1 migration state
- `edit`: fetch, edit, and update a resource
- `links`: print normalized hypermedia links
- `shell completion`: generate shell completion scripts

Machine-readable JSON output is available for `api list`, `api auth list`,
`cache info`, `content-types`, and `links` with `-o json`.

### `cert`

Inspect a server TLS certificate chain:

```bash
restish cert api.rest.sh
restish cert --warn-days 14 api.rest.sh
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

The `service.internal.test` command is illustrative; replace it with a host
your network can reach.

Use `cert` before changing request TLS flags. It reports issuer, subject,
validity dates, and expiration warnings using the same custom CA file you would
use for a request.
The command is TLS-only: bare hosts default to `https://`, and non-TLS schemes
such as `http://` are usage errors.

### `links`

GET a resource and print normalized hypermedia links:

```bash
restish links api.rest.sh/images
restish links api.rest.sh/images next
restish api.rest.sh/images -f links.next
```

Use `links` when you only need relations such as `self`, `next`, or `prev`.
Use normal requests plus filters when you need body data and link data
together. Restish extracts links from HTTP `Link` headers and supported body
formats such as HAL, JSON:API, Siren, JSON-LD/TSJ, and simple `self` fields.

### `doctor`

`doctor` writes its human report to stderr on an interactive terminal. When
stdout is redirected, the human report goes to stdout and stderr prints a hint
to use `-o json` for machine-readable output:

```bash
restish doctor -o json
restish doctor api example --check-network -o json
restish doctor plugin ./restish-csv -o json
restish doctor migrate-v1 -o json
```

## Generated API Commands

After configuration, an API name becomes a command group:

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish example --help
restish example list-images
restish example get-image jpeg
```

## Plugin Commands

Installed command plugins can add root commands such as `bulk` and `mcp`.
Manage plugin installation and discovery with:

```bash
restish plugin list
restish plugin install ./restish-csv
restish plugin install rest-sh/restish:csv
restish plugin debug ./restish-csv
```

## Related Pages

- [HTTP Commands](../http-commands/)
- [API Management](../api-management/)
- [Auth](../auth/)
- [Global Flags](../global-flags/)
- [Plugins](../plugins/)
