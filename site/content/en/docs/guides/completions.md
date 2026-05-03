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
restish shell setup zsh
```

Or install completion while configuring the shell wrapper:

```bash
restish shell setup zsh --completion
restish shell setup fish --completion
```

## Generated API Completion

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish example <TAB>
restish example get-image <TAB>
```

Good OpenAPI schemas improve completion for enum values, path parameters, and
flags.

## URL Completion

Generated operations also complete as URL paths when you prefer the generic
request form:

```bash
restish example/<TAB>
restish get example/<TAB>
restish get https://api.rest.sh/ima<TAB>
```

Restish keeps the form you started with. API short-name paths stay short,
full URLs stay full, and scheme-less URLs stay scheme-less. Verb commands only
show operations for that method, so `restish delete example/items/abc/<TAB>`
suggests DELETE endpoints instead of GET endpoints.

Completion fills path segments you have already typed:

```bash
restish example/items/docs-demo<TAB>
```

can complete to operation paths under `items/docs-demo`, while leaving any
remaining path variables visible.

## Refresh After Spec Changes

```bash
restish api sync example
```

Then start a new shell or refresh completion if your shell caches command
trees. URL completion uses cached operation metadata and does not discover
remote specs during shell completion.

## Related Pages

- [Shell Setup](/docs/getting-started/shell-setup/)
- [OpenAPI and CLI Integration](../openapi-cli-integration/)
- [Setup Command](/docs/reference/setup-command/)
