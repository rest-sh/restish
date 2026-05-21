---
title: Config Command
linkTitle: Config Command
weight: 14
description: Reference for inspecting, editing, patching, and theming Restish configuration.
aliases:
  - /docs/reference/theme-command/
---

Use `restish config` when you need to inspect or change local Restish state:
registered APIs, profiles, auth settings, cache preferences, plugin config,
and terminal themes.

For field-level config structure and precedence, see [Config](../config/).

## Common Examples

```bash
restish config path
restish config show
restish config show -o json
restish config set 'cache.max_size: 500MB'
restish config theme list
restish config theme set one-dark-pro
restish config theme reset
```

Use `api set` for edits that belong to one registered API. Use `config set`
when the patch targets top-level config, shared auth profiles, cache settings,
plugin settings, or themes.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen config-command -->
Generated from the current Cobra command tree.

### `restish config`

Manage local Restish configuration

Manage local Restish configuration.

The config stores registered APIs, profiles, auth settings, plugin settings, cache preferences, and output theme choices. Use `config show` for a redacted summary, `config path` to locate the file, and `config set` for scripted changes.

Usage:

```text
restish config
```

Examples:

```bash
  restish config show
  restish config path
  restish config set 'cache.max_size: 500MB'
```

Subcommands:

**`restish config edit`**: Open the restish config file in $VISUAL or $EDITOR

**`restish config path`**: Print the active config file path

**`restish config set`**: Patch config using shorthand syntax

**`restish config show`**: Print the active config summary or redacted JSON

**`restish config theme`**: Manage terminal output highlighting theme


### `restish config path`

Print the active config file path

Print the active Restish config file path.

This honors `--rsh-config` and `RSH_CONFIG`, so it is the safest way to confirm which config a command will read or write.

Usage:

```text
restish config path
```

Examples:

```bash
  restish config path
```


### `restish config show`

Print the active config summary or redacted JSON

Print the active config summary, or redacted JSON with `-o json`.

Human output shows counts and the config file path. JSON output is intended for inspection and support; sensitive auth values, credential-like headers, and credential-like query parameters are redacted where Restish recognizes them.

Usage:

```text
restish config show
```

Examples:

```bash
  restish config show
  restish config show -o json
```


### `restish config edit`

Open the restish config file in $VISUAL or $EDITOR

Open the active Restish config file in `$VISUAL` or `$EDITOR`.

Use this for manual config edits that are easier in an editor than with `config set`. Restish creates the config file if needed and preserves the platform-specific config path unless `--rsh-config` or `RSH_CONFIG` selects another file.

Usage:

```text
restish config edit
```

Examples:

```bash
  restish config edit
```


### `restish config set`

Patch config using shorthand syntax

Patch the active Restish config using shorthand syntax.

Use this for durable scripted changes, for example cache settings, theme colors, API profile defaults, or auth profile entries. Prefer `api set` when the change only belongs to one registered API.

Usage:

```text
restish config set <patch> [patch...]
```

Examples:

```bash
  restish config set 'cache.max_size: 500MB'
  restish config set 'theme.key: #afd787'
```


### `restish config theme`

Manage terminal output highlighting theme

Manage the terminal output highlighting theme.

Restish can use bundled themes, local JSON or JSONC files, direct URLs, or GitHub `user/repo` shorthand. Use `config theme list` to see bundled choices and `config theme reset` to return to the built-in default.

Usage:

```text
restish config theme
```

Examples:

```bash
  restish config theme list
  restish config theme set one-dark-pro
  restish config theme reset
```

Subcommands:

**`restish config theme list`**: List official theme names

**`restish config theme reset`**: Reset auto output highlighting to the built-in theme

**`restish config theme set`**: Install a theme JSON or JSONC file and save it in config


### `restish config theme list`

List official theme names

List bundled theme names.

The current configured bundled theme is marked with `*`. Custom local, URL, or GitHub themes are not expanded into this bundled list.

Usage:

```text
restish config theme list
```

Examples:

```bash
  restish config theme list
```


### `restish config theme set`

Install a theme JSON or JSONC file and save it in config

Install a theme JSON or JSONC file and save it in config.

Sources may be bundled theme names, local files, HTTPS URLs, or GitHub `user/repo` shorthand. Remote sources are executable in the sense that they affect terminal rendering, so Restish asks for confirmation unless the source is already trusted or you pass `--yes`.

Usage:

```text
restish config theme set <theme|path-or-url-or-user/repo> [name] [flags]
```

Examples:

```bash
  restish config theme set one-dark-pro
  restish config theme set ./theme.json
  restish config theme set user/repo dark
```

Flags:

**`--yes`**

Type: `bool`; default: `false`

Install without confirmation prompt



### `restish config theme reset`

Reset auto output highlighting to the built-in theme

Reset terminal output highlighting to the built-in theme.

This removes the saved theme override from config. It does not delete local theme files or remote sources.

Usage:

```text
restish config theme reset
```

Aliases: `unset`

Examples:

```bash
  restish config theme reset
```
<!-- END GENERATED -->

## Related Pages

- [Config](../config/)
- [Profiles](../profiles/)
- [Auth](../auth/)
- [Environment Variables](../environment-variables/)
- [Commands](../commands/)
