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

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Shell Completion and Setup](/docs/guides/completions/)
