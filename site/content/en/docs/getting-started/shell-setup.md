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
restish shell setup zsh
restish shell setup bash
restish shell setup fish
```

For zsh and fish, setup installs completion by default. Opt out when you only
want the wrapper:

```bash
restish shell setup zsh --no-completion
```

Preview the change first when you want to inspect it:

```bash
restish shell setup zsh --dry-run
```

Apply without prompting in automation:

```bash
restish shell setup zsh --yes
```

For zsh and bash this writes a `noglob` alias. Fish gets the equivalent wrapper
function.

## Why It Matters

Without setup, these commands may fail before Restish starts:

```bash
restish 'api.rest.sh/images?format=jpeg&limit=1'
restish api.rest.sh/images --rsh-no-paginate -f 'body[0].self'
restish post api.rest.sh/post 'tags[]: docs' 'tags[]: cli'
```

Quoting still works, and it is the most portable habit for shared scripts. The
setup command makes day-to-day interactive use less brittle.

## Completion Scripts

Restish also supports shell completion through Cobra:

```bash
restish shell completion zsh
restish shell completion bash
restish shell completion fish
restish shell completion powershell
```

For a normal zsh or fish user install, let Restish write the completion script:

```bash
restish shell setup zsh
restish shell setup fish
```

For zsh, the generated script is stored under Restish's config directory and a
managed source block is added to `~/.zshrc`. For fish, the generated script is
written to fish's user completions directory.

Generated API commands participate in completion after an API is configured:

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish example <TAB>
restish example/<TAB>
```

Use the command form when you want generated help and flags. Use the URL form
when you want to explore operation paths with generic HTTP requests.

## Recommended Habit

- Quote URLs that contain `?` or `&` in scripts.
- Quote filters that contain brackets, spaces, pipes, or comparisons.
- Use `restish shell setup <shell>` for interactive use.
- Prefer generated commands when an API has a spec; completion becomes much
  more useful.

## Next Step

[Connect to an API](../connect-to-an-api/) when you want generated commands,
completion, profiles, and API-specific help.

## Related Pages

- [Tour of Restish](../quickstart/)
- [Completions](/docs/guides/completions/)
- [Shorthand](/docs/reference/shorthand/)
- [Query Syntax](/docs/reference/query-syntax/)
