# Setup And Completions

## Summary

Restish v2 supports two shell-facing behaviors that make the CLI easier to use
interactively:

- shell completion scripts
- shell setup that installs a `noglob` alias or equivalent

These are separate concerns on purpose: completion teaches the shell what
arguments exist, while setup teaches the shell not to eagerly expand Restish
input that is meant for Restish itself.

## Problem

Restish uses shorthand syntax, filters, and generated commands that often
contain characters shells like to interpret. That creates two common problems:

- users want completions for commands, flags, and generated values
- users do not want the shell to mangle expressions before Restish sees them

The setup/completion design therefore needed to:

- leverage standard shell completion support instead of inventing a custom
  mechanism
- preserve raw CLI input for shorthand and filter expressions
- support the major interactive shells without a large custom integration layer

## Design

Completion generation is delegated to Cobra's built-in shell completion support.
That keeps Restish aligned with standard Go CLI behavior and automatically
covers both built-in commands and generated commands that are part of the Cobra
tree.

Setup is intentionally narrower. The `setup <shell>` command appends a shell
snippet to the relevant rc file that configures `restish` to run under
`noglob`-style behavior where the shell has a correct equivalent.

The built-in setup currently supports:

- `zsh`
- `bash`

Some design choices worth preserving:

- setup is idempotent and does not append the same alias repeatedly
- setup and completion stay separate so users can opt into one without the
  other
- completion candidates come from the real command tree, which includes
  generated commands and enum-backed flag values when they are registered

## Examples

Generate a completion script:

```bash
restish completion zsh
restish completion bash
restish completion fish
```

Configure the shell alias:

```bash
restish setup zsh
```

which appends a line like:

```text
alias restish="noglob restish"
```

to the shell rc file.

Generated command completions also benefit from OpenAPI-derived enum values. For
example, an operation with a query parameter enum of `active`, `inactive`, and
`pending` can surface those as shell completion candidates for the generated
flag.

## Alternatives Considered

### Rely on users to disable globbing manually each time

This is possible, but it makes day-to-day usage more error-prone and harder to
teach.

### Write custom completion systems per shell

That would add a lot of maintenance burden for little gain. Cobra already
provides the standard integration point Restish needs.

### Fold setup and completion into one command

These solve related but distinct problems. Keeping them separate makes the
behavior clearer and easier to evolve.

## Notes

The current implementation reflects this design directly:

- `internal/cli/setup.go` implements shell setup and alias installation
- Cobra's built-in completion support provides the `completion` command
- generated command completion hooks are registered on generated flags and
  positional arguments where enum values are available

One detail worth preserving is that the `setup` command exists mainly to protect
Restish's own input syntax from shell expansion. It is not just a convenience
wrapper around completion installation.
