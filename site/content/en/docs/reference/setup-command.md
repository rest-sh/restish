---
title: Shell Setup Command
linkTitle: Setup
weight: 40
description: Configure shell wrappers so Restish arguments reach the CLI unchanged.
---

Restish commands often contain characters that shells also interpret: `?`, `&`,
`[`, `]`, `*`, quotes, and spaces. The `shell setup` command installs small
shell wrappers so interactive commands reach Restish unchanged.

## Examples

```bash
restish shell setup zsh --dry-run
restish shell setup zsh --yes
restish shell setup zsh --no-completion
restish shell setup bash
restish shell setup fish
```

Use `--dry-run` to inspect what would be added before changing shell startup
files. Use `--yes` when you are applying the change intentionally and do not
want a prompt.

For zsh and fish, setup installs completion by default. For zsh, it installs
the generated completion script under Restish's config directory and adds a
managed completion block to `~/.zshrc`. For fish, it writes the generated
script to the shell's user completions directory. Use `--no-completion` when
you only want the shell wrapper.

## Notes

Use this for interactive shells. Still quote complex URLs and filters in
scripts, because scripts should be portable and explicit. The first-user flow
explains the practical impact in [Shell Setup](/docs/getting-started/shell-setup/).

## Related Pages

- [Commands](/docs/reference/commands/)
- [Shell Setup](/docs/getting-started/shell-setup/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
