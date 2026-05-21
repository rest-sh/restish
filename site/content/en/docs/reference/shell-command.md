---
title: Shell Command
linkTitle: Shell
weight: 17
description: Reference for Restish shell setup and completion commands.
aliases:
  - /docs/reference/setup-command/
---

Use `restish shell` to configure interactive shell behavior and generate or
install completion scripts. Shell setup prevents URL, query, filter, and
shorthand characters from being rewritten before Restish sees them.

For the workflow explanation, see [Shell Setup](/docs/getting-started/shell-setup/).

## Common Examples

```bash
restish shell setup zsh --dry-run
restish shell setup zsh
restish shell setup zsh --no-completion
restish shell completion zsh
restish shell completion bash
restish shell completion install fish
```

Quote URLs, filters, and shorthand in scripts even after shell setup. The
wrapper is for day-to-day interactive use.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen shell-command -->
Generated from the current Cobra command tree.

### `restish shell`

Configure shell integration for restish

Configure shell integration for Restish.

Shell setup prevents common glob-expansion issues and can install shell completion where supported. Use `shell setup <shell>` for the managed setup flow, or `shell completion` when you only need completion scripts.

Usage:

```text
restish shell
```

Subcommands:

**`restish shell completion`**: Generate shell completion scripts

**`restish shell setup`**: Configure your shell for restish


### `restish shell setup`

Configure your shell for restish

Append a `noglob` wrapper or function for Restish to your shell startup file and install completion for supported shells.

Supported shells: zsh, bash, fish.

Use `--dry-run` to preview file changes, `--no-completion` to skip completion installation, and `--yes` only after you are comfortable with the startup-file change.

Usage:

```text
restish shell setup <shell> [flags]
```

Flags:

**`--dry-run`**

Type: `bool`; default: `false`

Show what would be written without modifying files

**`--no-completion`**

Type: `bool`; default: `false`

Do not install shell completion

**`-y`, `--yes`**

Type: `bool`; default: `false`

Apply changes without confirmation prompt



### `restish shell completion`

Generate shell completion scripts

Generate or install shell completion scripts.

Script generation writes to stdout for package managers and manual setup. `completion install` writes a generated script under Restish's config directory or a shell-native user completion directory, then updates shell startup files only when the shell requires it.

Usage:

```text
restish shell completion
```

Subcommands:

**`restish shell completion bash`**: Generate the autocompletion script for bash

**`restish shell completion fish`**: Generate the autocompletion script for fish

**`restish shell completion install`**: Install shell completion for your user account

**`restish shell completion powershell`**: Generate the autocompletion script for powershell

**`restish shell completion zsh`**: Generate the autocompletion script for zsh


### `restish shell completion bash`

Generate the autocompletion script for bash

Generate the autocompletion script for `bash`.

This writes the script to stdout for package managers and manual shell setup. For user-level installation, use:

```bash
restish shell completion bash > /etc/bash_completion.d/restish
```

Usage:

```text
restish shell completion bash
```

Flags:

**`--no-descriptions`**

Type: `bool`; default: `false`

disable completion descriptions



### `restish shell completion zsh`

Generate the autocompletion script for zsh

Generate the autocompletion script for `zsh`.

This writes the script to stdout for package managers and manual shell setup. For user-level installation, use:

```bash
restish shell completion install zsh
```

Usage:

```text
restish shell completion zsh [flags]
```

Flags:

**`--no-descriptions`**

Type: `bool`; default: `false`

disable completion descriptions



### `restish shell completion fish`

Generate the autocompletion script for fish

Generate the autocompletion script for `fish`.

This writes the script to stdout for package managers and manual shell setup. For user-level installation, use:

```bash
restish shell completion install fish
```

Usage:

```text
restish shell completion fish [flags]
```

Flags:

**`--no-descriptions`**

Type: `bool`; default: `false`

disable completion descriptions



### `restish shell completion powershell`

Generate the autocompletion script for powershell

Generate the autocompletion script for `powershell`.

This writes the script to stdout for package managers and manual shell setup. For user-level installation, use:

```bash
restish shell completion powershell | Out-String | Invoke-Expression
```

Usage:

```text
restish shell completion powershell [flags]
```

Flags:

**`--no-descriptions`**

Type: `bool`; default: `false`

disable completion descriptions



### `restish shell completion install`

Install shell completion for your user account

Install shell completion for your user account.

Supported shells: zsh and fish. The zsh installer writes the generated script under Restish's config directory and adds a managed source block to `~/.zshrc`. The fish installer writes to the shell's user completions directory.

Usage:

```text
restish shell completion install <shell> [flags]
```

Flags:

**`--dry-run`**

Type: `bool`; default: `false`

Show what would be written without modifying files

**`--no-descriptions`**

Type: `bool`; default: `false`

disable completion descriptions

**`-y`, `--yes`**

Type: `bool`; default: `false`

Apply changes without confirmation prompt
<!-- END GENERATED -->

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Commands](../commands/)
- [Shorthand](../shorthand/)
- [Query Syntax](../query-syntax/)
