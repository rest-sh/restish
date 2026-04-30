---
title: Completions
linkTitle: Completions
weight: 35
description: Generate shell completions and use generated API command completion effectively.
---

Completion helps most after Restish knows about your APIs. It can complete
built-in commands, global flags, generated API commands, and many generated
operation parameters.

## Generate Completion Scripts

```bash
restish completion zsh
restish completion bash
restish completion fish
restish completion powershell
```

Install the output according to your shell's completion mechanism.

## Install Completion

For normal user-managed zsh and fish setups, Restish can install completion for
you:

```bash
restish completion install zsh
restish completion install fish
```

For zsh, this writes the generated script under Restish's config directory and
adds a managed source block to `~/.zshrc`. For fish, it writes to the shell's
user completions directory. Preview the files first:

```bash
restish completion install zsh --dry-run
restish completion install fish --dry-run
```

## Configure Shell Safety Too

Completion does not prevent glob expansion. Run setup for interactive use:

```bash
restish setup zsh
```

Or install completion while configuring the shell wrapper:

```bash
restish setup zsh --completion
restish setup fish --completion
```

## Generated API Completion

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish example <TAB>
restish example get-image <TAB>
```

Good OpenAPI schemas improve completion for enum values, path parameters, and
flags.

## Refresh After Spec Changes

```bash
restish api sync example
```

Then start a new shell or refresh completion if your shell caches command trees.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [OpenAPI and CLI Integration](../openapi-cli-integration/)
- [Setup Command](/docs/reference/setup-command/)
