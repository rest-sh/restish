---
title: Install and Use Plugins
linkTitle: Install and Use
weight: 10
description: Install, list, remove, configure, verify, and debug Restish plugins.
aliases:
  - /docs/recipes/convert-array-output-to-csv/
---

This is the operator path for using plugins that already exist.

Plugins are executable programs on your machine. Install and run them at your
own risk, from sources you trust. Restish checks the plugin manifest and
capabilities, but it does not sandbox plugin code or verify publisher identity.

## List Plugins

```bash
restish plugin list
```

Run this first when an output format, command, auth method, or TLS signer is
missing.

## Install A Plugin

Install from the official Restish GitHub releases:

```bash
restish plugin install rest-sh/restish:csv
```

The part after `:` is expanded to a plugin binary name, so `:csv` means
`restish-csv`. Restish downloads the latest release asset for your OS and CPU,
extracts it, verifies the plugin manifest, and copies the binary into your
plugin directory.

`plugin install` prints the source, resolved file or download URL, manifest
name/version, and declared capabilities before installing. In a terminal,
confirm the trust prompt to continue. In automation, pass `--yes` after pinning
the source you intend to trust:

```bash
restish plugin install rest-sh/restish:csv --yes
```

Install a system-installed plugin from `PATH`:

```bash
brew install rest-sh/tap/restish-csv
restish plugin install restish-csv
```

Use the same pattern for `restish-bulk`, `restish-mcp`, and
`restish-pkcs11`.

Install directly from a URL when your team publishes archives somewhere else:

```bash
restish plugin install https://downloads.example.com/restish-csv_darwin_arm64.tar.gz
```

For local development builds, install the binary directly:

```bash
restish plugin install ./restish-csv
```

The plugin must be executable and compatible with the v2 plugin protocol. Every
install path verifies the plugin manifest before keeping the binary, but
manifest verification is not a security review. Restish shows declared
capability families in `plugin list`, and it only enables capabilities that the
manifest explicitly declares. Restish loads plugins only after they are
installed into Restish's configured plugin directory; it does not scan every
executable on `PATH`.

## Remove A Plugin

```bash
restish plugin remove restish-csv
```

## Verify Behavior

Formatter example:

```bash
restish https://api.rest.sh/images -o csv
```

Command plugin example:

```bash
restish bulk --help
restish mcp serve example
```

## Debug A Plugin

```bash
restish plugin debug ./restish-csv
```

Debug mode prints decoded protocol messages to stderr. Use it when a plugin is
discovered but does not behave as expected.

## Related Pages

- [Plugin Command](/docs/reference/plugin-command/)
- [Example Plugins](../example-plugins/)
- [Troubleshooting](/docs/guides/troubleshooting/)
