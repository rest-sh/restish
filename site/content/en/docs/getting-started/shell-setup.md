---
title: Shell Setup
linkTitle: Shell Setup
weight: 35
description: Configure your shell so URLs, filters, and shorthand reach Restish unchanged.
---

Restish uses characters that shells also care about: `?`, `&`, `[`, `]`, `*`,
quotes, and spaces. Shell setup prevents the shell from rewriting request URLs,
query strings, filters, and shorthand before Restish can parse them.

## Configure The Wrapper

Run the setup command for your shell:

```bash
restish setup zsh
restish setup bash
restish setup fish
```

Preview the change first when you want to inspect it:

```bash
restish setup zsh --dry-run
```

Apply without prompting in automation:

```bash
restish setup zsh --yes
```

For zsh and bash this writes a `noglob` alias. Fish gets the equivalent wrapper
function.

## Why It Matters

Without setup, these commands may fail before Restish starts:

```bash
restish 'https://api.rest.sh/images?format=jpeg&limit=1'
restish https://api.rest.sh/images -f 'body[0].self'
restish post https://api.rest.sh/post 'tags[]: docs' 'tags[]: cli'
```

Quoting still works, and it is the most portable habit for shared scripts. The
setup command makes day-to-day interactive use less brittle.

## Completion Scripts

Restish also supports shell completion through Cobra:

```bash
restish completion zsh
restish completion bash
restish completion fish
restish completion powershell
```

Generated API commands participate in completion after an API is configured:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
restish example <TAB>
```

## Recommended Habit

- Quote URLs that contain `?` or `&` in scripts.
- Quote filters that contain brackets, spaces, pipes, or comparisons.
- Use `restish setup <shell>` for interactive use.
- Prefer generated commands when an API has a spec; completion becomes much
  more useful.

## Next Step

[Connect to an API](../connect-to-an-api/) when you want generated commands,
completion, profiles, and API-specific help.

## Related Pages

- [Quickstart](../quickstart/)
- [Completions](/docs/guides/completions/)
- [Shorthand](/docs/reference/shorthand/)
- [Query Syntax](/docs/reference/query-syntax/)
