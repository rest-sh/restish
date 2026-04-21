# Setup And Completions

## Summary

Restish v2 supports two shell-facing behaviors that make the CLI easier to use
interactively:

- shell completion scripts
- shell setup that installs `noglob`-style protection where the shell has a
  correct equivalent

These are separate concerns on purpose: completion teaches the shell what
arguments exist, while setup teaches the shell not to eagerly expand Restish
input that is meant for Restish itself.

## Goals

- leverage standard shell completion support instead of inventing a custom
  mechanism
- preserve raw CLI input for shorthand and filter expressions
- support common interactive shells with minimal custom integration
- keep setup behavior inspectable, idempotent, and safe

## Non-Goals

- implementing one giant shell-integration system that mixes completion,
  aliases, and environment setup into one opaque step
- promising shell support where Restish cannot actually provide correct
  `noglob`-style protection

## Two Different Features

### Completion

Completion is about discovery:

- command names
- flags
- enum-backed values
- generated API commands

### Setup

Setup is about argument preservation:

- shorthand expressions
- bracket-heavy filters
- wildcard-like syntax that shells would otherwise expand first

Treating these as separate features lets users opt into one without assuming
they want the other.

## Completion Design

Completion generation is delegated to Cobra's built-in shell completion support.
That keeps Restish aligned with standard Go CLI behavior and automatically
covers both built-in commands and generated commands that are part of the Cobra
tree.

Completion should reflect the real command tree, which means it can include:

- built-in commands
- generated API command groups
- plugin-contributed command roots when they are registered at startup

Where Restish has enough metadata, completion should also surface:

- enum-backed parameter values
- known output-format names
- profile names and similar finite sets

## Setup Design

`setup <shell>` appends a shell snippet to the relevant rc file that configures
`restish` to run under `noglob`-style behavior where the shell has a correct
equivalent.

The built-in setup currently supports:

- `zsh`
- `bash`

Shells without a compatible equivalent, such as fish, should not be advertised
as supported by `setup` just because they support completion.

## Rc File Selection

Rc file selection is part of the design, not an incidental implementation
detail.

Expected behavior:

- `zsh` -> `~/.zshrc`
- `bash` on macOS login-shell workflows -> `~/.bash_profile`
- `bash` on other Unix-like systems -> `~/.bashrc`

This keeps the setup command aligned with where those shells actually load user
startup config in common environments.

## Idempotence And Confirmation

Setup should be idempotent:

- do not append the same alias repeatedly
- detect when the configured line already exists

Setup should also be operator-friendly:

- support dry-run behavior
- support explicit confirmation bypass for automation
- write helpful output telling the user what changed

## Shell Detection And Hints

Restish may offer first-run hints based on shell detection, but hints are not
the same as setup support.

The design rule is:

- hints may suggest a supported shell setup command
- unsupported shells may still get completion guidance
- detection heuristics such as `$SHELL` should be treated as best-effort hints,
  not as proof of the currently running shell

## Why `noglob` Protection Matters

Restish syntax often includes characters shells like to interpret:

- `[]`
- `*`
- `?`
- `{}` in some shell contexts

Without setup or careful quoting, the shell may transform those expressions
before Restish ever sees them. Setup exists to reduce that day-to-day friction.

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

Generated command completions can also benefit from OpenAPI-derived enum values.

## Alternatives Considered

### Rely On Users To Disable Globbing Manually Every Time

Possible, but too error-prone for day-to-day use.

### Write Custom Completion Systems Per Shell

Too much maintenance burden for too little benefit.

### Fold Setup And Completion Into One Command

These solve related but distinct problems and should stay separate.

## Relationship To Other Designs

- Design 007 explains where generated commands come from.
- Design 017 defines truth-in-help and shell-facing CLI behavior.
- Design 001 defines runtime-owned prompting and file-write behavior.
