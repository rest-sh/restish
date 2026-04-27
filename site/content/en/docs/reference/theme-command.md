---
title: Theme Command
linkTitle: Theme
weight: 40
description: Manage readable output highlighting themes.
---

The `theme` command controls the colors used by the readable formatter. It is
for people who spend time inspecting responses in a terminal and want output
that works better with their terminal background, contrast needs, or personal
preferences.

## Examples

```bash
restish theme list
restish theme show
restish theme set
```

`theme list` shows the available themes. `theme show` previews the active
theme. `theme set` opens the interactive selection flow.

## Notes

Theme support affects human-readable terminal output, not JSON, NDJSON, raw
bytes, or other machine-oriented formats. If a script depends on output shape,
choose a format such as `-o json`; themes are intentionally cosmetic.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Output](/docs/guides/output/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
