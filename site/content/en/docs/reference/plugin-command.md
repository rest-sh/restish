---
title: Plugin Command
linkTitle: Plugin
weight: 40
description: Install, list, remove, and debug Restish plugins.
---

Plugins extend Restish without making every feature part of the core binary.
Operators use the `plugin` command to install, list, remove, and debug those
extensions. Authors use the plugin reference pages to understand the protocols.

Installed plugins are executable code and run at your own risk. Restish checks
manifests and declared capabilities; it does not sandbox plugins or verify
publisher identity.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen plugin-command -->
Generated from the current Cobra command tree.

### `restish plugin`

Manage restish plugins

Manage Restish plugins.

Plugins are executable programs that can add commands, content loaders, output formatters, hooks, or TLS signing behavior. Use `plugin list` to see discovered plugins, `plugin install` for trusted plugin binaries, and `plugin debug` when a plugin is discovered but not behaving correctly.

Usage:

```text
restish plugin
```

Examples:

```bash
  restish plugin list
  restish plugin install ./restish-example
  restish plugin install rest-sh/restish mcp
```

Subcommands:

**`restish plugin debug`**: Spawn a plugin and print decoded CBOR messages to stderr

**`restish plugin install`**: Install a plugin from a path, URL, PATH command, or GitHub release

**`restish plugin list`**: List all discovered plugins

**`restish plugin remove`**: Remove an installed plugin


### `restish plugin list`

List all discovered plugins

List all discovered plugins and their capabilities.

Human output shows plugin names, versions, capabilities, command names, formatter names, loader content types, and descriptions when available. Use `-o json` for automation or diagnostics.

Usage:

```text
restish plugin list
```

Examples:

```bash
  restish plugin list
  restish plugin list -o json
```


### `restish plugin install`

Install a plugin from a path, URL, PATH command, or GitHub release

Install a trusted plugin into Restish's plugin directory.

Plugins are executable programs. Restish reads the plugin manifest before installing, checks declared capabilities, and verifies protocol compatibility, but it does not sandbox plugin code or verify publisher identity. Install plugins only from sources you trust.

Sources can be local executable paths, commands on `PATH`, direct archive URLs, or GitHub release shorthand such as `rest-sh/restish mcp`.

Use `--yes` only after choosing a source you intend to trust. It skips the interactive confirmation prompt; it does not make the plugin safer.

Usage:

```text
restish plugin install <source> [name] [flags]
```

Examples:

```bash
  restish plugin install ./restish-example
  restish plugin install restish-example
  restish plugin install rest-sh/restish mcp
```

Flags:

**`--yes`**

Type: `bool`; default: `false`

Trust and install without an interactive confirmation



### `restish plugin remove`

Remove an installed plugin

Remove an installed plugin from the Restish plugin directory.

You can pass either the installed file name or the plugin manifest name. Restish refuses ambiguous manifest-name matches so you can delete the intended executable explicitly.

Usage:

```text
restish plugin remove <name>
```

Examples:

```bash
  restish plugin remove example
```


### `restish plugin debug`

Spawn a plugin and print decoded CBOR messages to stderr

Spawn a plugin and print decoded protocol messages to stderr.

Use this when a plugin is discovered but does not behave as expected. It shows the manifest/startup exchange and runtime messages so you can see whether the plugin, host, or protocol payload is failing.

Pass plugin arguments after the plugin name. Use `--` before arguments that could otherwise be interpreted by Restish.

Usage:

```text
restish plugin debug <name> [args...]
```

Examples:

```bash
  restish plugin debug example -- --help
```
<!-- END GENERATED -->

## Examples

```bash
restish plugin list
restish plugin install ./restish-csv
restish plugin install restish-csv
restish plugin install rest-sh/restish csv
restish plugin install https://downloads.example.com/restish-csv_darwin_arm64.tar.gz
restish plugin remove restish-csv
restish plugin debug ./restish-csv
```

If plugin stdout contains malformed CBOR after valid messages, `plugin debug`
keeps draining stdout to protect the terminal and then exits with the decode
error. That makes it safer to debug broken plugins without dumping binary
protocol data into an interactive terminal.

## Notes

Keep operator tasks separate from author tasks. Installing and verifying a
plugin should not require reading wire protocol details. Start with
[Install and Use Plugins](/docs/plugins/install-and-use/) unless you are
building a plugin yourself.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Plugins](/docs/plugins/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
