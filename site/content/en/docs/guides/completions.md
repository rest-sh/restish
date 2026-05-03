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
restish shell completion zsh
restish shell completion bash
restish shell completion fish
restish shell completion powershell
```

Install the output according to your shell's completion mechanism.

## Install Completion

For normal user-managed zsh and fish setups, Restish can install completion for
you:

```bash
restish shell setup zsh
restish shell setup fish
```

For zsh, this writes the generated script under Restish's config directory and
adds a managed source block to `~/.zshrc`. For fish, it writes to the shell's
user completions directory. Preview the files first:

Use `restish shell setup <shell> --dry-run` to preview the files first.

## Configure Shell Safety Too

Completion does not prevent glob expansion. Run setup for interactive use:

```bash
restish shell setup zsh
```

For zsh and fish, setup installs completion by default. Opt out when needed:

```bash
restish shell setup zsh --no-completion
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

can complete to operation paths under `items/docs-demo`. Restish does not
insert literal `{parameter}` placeholders into completed URLs. If the OpenAPI
schema gives enum values for a path parameter, Restish offers those values:

```bash
restish example/formats/<TAB>
```

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
