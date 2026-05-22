---
title: Utility Commands
linkTitle: Utilities
weight: 18
description: "Reference for smaller Restish utility commands: cert, links, and version."
aliases:
  - /docs/reference/cert-command/
  - /docs/reference/links-command/
---

Utility commands inspect supporting HTTP and runtime details without changing
remote resources. Use them to check TLS certificates, extract hypermedia links,
and report the Restish version.

## Common Examples

```bash
restish cert api.rest.sh
restish cert api.rest.sh --warn-days 30
restish links api.rest.sh/images next
restish version
```

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen utility-commands -->
Generated from the current Cobra command tree.

### `restish cert`

Show the TLS certificate chain for a server

Show the TLS certificate chain for an HTTPS server.

Use this to inspect certificate subjects, issuers, DNS names, validity windows, and expiry timing with the same TLS-related flags Restish uses for requests. `--warn-days` exits non-zero when the leaf certificate expires soon, which is useful in monitoring scripts.

Usage:

```text
restish cert <uri> [flags]
```

Examples:

```bash
  restish cert https://api.example.com
  restish cert api.example.com --warn-days 30
```

Flags:

**`--warn-days`**

Type: `int`; default: `0`

Exit non-zero if the leaf certificate expires within N days



### `restish links`

GET a URI and display its hypermedia links

Perform a `GET` request and print hypermedia links found in the response.

Restish extracts links from `Link` headers, HAL `_links`, JSON:API links, Siren links, and JSON-LD `@id` fields. Pass relation names after the URI to filter the output to specific rels.

Usage:

```text
restish links <uri> [rel...]
```

Examples:

```bash
  restish links https://api.example.com/items/123
  restish links https://api.example.com/items/123 self next
```


### `restish version`

Print the Restish version

Print the Restish version and exit.

Use this in bug reports, release checks, and scripts that need to confirm which Restish binary is running.

Usage:

```text
restish version
```
<!-- END GENERATED -->

## Related Pages

- [Content Types](../content-types/)
- [TLS](/docs/guides/tls/)
- [Commands](../commands/)
- [Global Flags](../global-flags/)
