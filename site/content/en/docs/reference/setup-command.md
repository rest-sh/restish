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
restish setup fish
```

## Supported Shells

- `zsh`
- `bash`
- `fish`

## What It Changes

`setup` prints shell-specific configuration that helps your shell stop
interpreting Restish shorthand, brackets, and wildcard-like input before the
CLI receives it.

That matters most for:

- shorthand patches such as `tags[0]: blue`
- filter expressions containing brackets or punctuation
- commands users would otherwise need to quote more aggressively

## When To Re-Run It

Run `setup` again when:

- you switch shells
- you set up Restish on another machine
- your shell config was reset or replaced

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Shell Completion and Setup](/docs/guides/completions/)
