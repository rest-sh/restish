---
title: Shell Completion and Setup
linkTitle: Completions
weight: 28
description: Install Restish shell completion and understand how completion differs from shell setup.
---

Restish uses two shell-facing features that solve different problems:

- `setup` protects Restish input from shell globbing
- `completion` teaches the shell what commands and flags exist

You usually want both.

## Generate Completion Scripts

Restish exposes Cobra's built-in completion support:

```bash
restish completion zsh
restish completion bash
restish completion fish
restish completion powershell
```

## Practical Installation Pattern

- Zsh: install the generated script in a directory on your `fpath`
- Bash: source the script from your shell startup files or completion
  directory
- Fish: write it into `~/.config/fish/completions/`
- PowerShell: load it from your PowerShell profile

If you installed Restish through Homebrew, make sure your shell is also loading
Homebrew's completion path.

## Why Completion Matters More After API Setup

Completion is most valuable once you register APIs:

```bash
restish api configure example https://api.rest.sh
restish example <TAB>
```

At that point completion can surface:

- generated operation names
- generated flags
- enum-backed values when the spec provides them

## Setup vs Completion

Use `setup` when the shell is rewriting your input:

```bash
restish setup zsh
```

Use `completion` when the shell does not know what commands and flags are
available:

```bash
restish completion zsh
```

## Recommended Habit

1. run `restish setup <shell>`
2. install the matching completion script

That gives you both safer input handling and better discovery.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [Connect to an API](/docs/getting-started/connect-to-an-api/)
