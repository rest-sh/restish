---
title: Shell Setup
linkTitle: Shell Setup
weight: 20
description: Configure shell aliases, completion, and quoting behavior for Restish.
---

Restish works best when shell completion and quoting behavior are configured
deliberately.

You can skip this page for the first five minutes if you just want one
successful request. Come back once you expect to use Restish regularly.

## Why This Matters

Generated API commands and shorthand expressions often include characters that
shells want to interpret. Completion and a small amount of shell setup make the
CLI much nicer to use day to day.

Without setup, shells often try to interpret input that should have been passed
through to Restish unchanged.

Common examples:

```bash
restish https://api.rest.sh/images?format=jpeg
restish post https://api.rest.sh tags[]: red tags[]: blue
restish https://api.rest.sh/images -f 'body[0].self'
```

Those commands are normal Restish input, but `?`, `[]`, and similar characters
are exactly the kind of syntax shells like to glob.

## Configure Shell Input Handling

Restish provides a `setup` command that appends a shell-specific alias or
abbreviation so your shell does not eagerly glob shorthand and filter
expressions.

Supported shells:

- `zsh`
- `bash`
- `fish`

Examples:

```bash
restish setup zsh
restish setup bash
restish setup fish
```

Today that writes one of these shell-specific lines:

- Zsh and Bash: `alias restish="noglob restish"`
- Fish: `abbr --add restish "noglob restish"`

The command is idempotent, so running it again does not keep appending the same
line repeatedly.

If you use Restish interactively, this step pays off quickly.

## Restart Or Reload Your Shell

After setup, either restart the shell or reload the rc file. For example:

```bash
source ~/.zshrc
source ~/.bashrc
source ~/.config/fish/config.fish
```

## Generate Completion Scripts

Restish also exposes Cobra's built-in completion support:

```bash
restish completion zsh
restish completion bash
restish completion fish
restish completion powershell
```

Use those commands with your shell's normal completion installation workflow.

Typical pattern:

- Zsh: write the script to a directory in your `fpath`
- Bash: source it from your shell startup files or install it into your
  completion directory
- Fish: write it into `~/.config/fish/completions/`
- PowerShell: load it through your PowerShell profile

If you installed Restish through Homebrew, you may also need to make sure your
shell is loading Homebrew's completion directory.

## Why `noglob`-Style Setup Matters

Completion and shell setup solve different problems:

- completion teaches the shell what commands and flags exist
- setup prevents the shell from rewriting input that Restish needs to parse

That is especially helpful for:

- shorthand input with brackets or punctuation
- filter expressions
- generated commands and flags that would otherwise benefit from completion

## Recommended Habit

For interactive use, set up both:

1. run `restish setup <shell>`
2. install the completion script for your shell

That combination gives you safer input handling plus tab completion for built-in
and generated commands.

Generated API commands benefit the most because completion can show discovered
operation names and, where available, enum-backed values from the API spec.

## Short Version

If you want the minimum useful setup:

```bash
restish setup zsh
source ~/.zshrc
```

Then add shell completion once you are ready.

## Related Guides

- [Install](../install/)
- [Quickstart](../quickstart/)
- [Connect to an API](../connect-to-an-api/)

## Source Material

Based on the setup and completions design work summarized in the
[design records](/docs/contributing/design-records/).
