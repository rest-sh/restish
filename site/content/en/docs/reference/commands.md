---
title: Commands
linkTitle: Commands
weight: 10
description: Reference index for Restish built-in commands, generic HTTP verbs, generated API commands, and plugin commands.
---

Restish commands fall into a few groups.

## Generic HTTP Commands

```bash
restish https://api.rest.sh/
restish get https://api.rest.sh/get
restish post https://api.rest.sh/post name: Alice
restish put https://api.rest.sh/put name: Alice
restish patch https://api.rest.sh/patch enabled: false
restish delete https://api.rest.sh/delete --rsh-ignore-status-code
restish head https://api.rest.sh/head
restish options https://api.rest.sh/options
```

A bare URL is a generic GET.

## Configuration And Setup

- `api`: manage registered APIs and specs
- `cache`: inspect and clear HTTP response cache
- `setup`: write shell wrappers for safer interactive use
- `theme`: manage readable-output highlighting theme

## Utilities

- `cert`: inspect server TLS certificate chains
- `edit`: fetch, edit, and update a resource
- `links`: print normalized hypermedia links
- `completion`: generate shell completion scripts

Use `api auth inspect` to inspect configured API auth material.

## Generated API Commands

After configuration, an API name becomes a command group:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
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
- [Global Flags](../global-flags/)
- [Plugins](../plugins/)
