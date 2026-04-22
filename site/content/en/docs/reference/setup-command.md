---
title: Setup Command
linkTitle: Setup Command
weight: 17
description: Reference for restish setup, which adds shell-specific noglob-style protection.
---

`restish setup <shell>` appends a shell-specific alias or abbreviation so the
shell does not glob Restish input before Restish sees it.

## Examples

```bash
restish setup zsh
restish setup bash
```

## Supported Shells

- `zsh`
- `bash`

## What It Changes

`setup` prints shell-specific configuration that helps your shell stop
interpreting Restish shorthand, brackets, and wildcard-like input before the
CLI receives it.

For `zsh` and `bash`, the alias wraps the command with `noglob` so the shell
does not expand glob characters before Restish sees them.

On macOS, `restish setup bash` writes to `~/.bash_profile` because login shells
typically read that file by default. On Linux and other Unix-like systems,
`bash` setup writes to `~/.bashrc`.

That matters most for:

- shorthand patches such as `tags[0]: blue`
- filter expressions containing brackets or punctuation
- commands users would otherwise need to quote more aggressively

## When To Re-Run It

Run `setup` again when:

- you switch shells
- you set up Restish on another machine
- your shell config was reset or replaced

If you use a shell Restish does not currently support directly, quote shorthand
and filter expressions explicitly instead of relying on `setup`.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Shell Completion and Setup](/docs/guides/completions/)
