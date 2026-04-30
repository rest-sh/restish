---
title: Config Theme Command
linkTitle: Theme
weight: 40
description: Manage readable output highlighting themes.
---

`config theme` controls the colors used by the readable formatter. It is for
people who spend time inspecting responses in a terminal and want output that
works better with their terminal background, contrast needs, or personal
preferences.

## Examples

```bash
restish config theme set https://example.com/theme.json
restish config theme set user/repo dark
```

`config theme set` fetches a theme JSON file and saves it in the active config.
GitHub shorthand resolves `user/repo` to a raw `theme.json`, or to
`<name>.json` when you pass the optional name.

## Notes

Theme support affects human-readable terminal output, not JSON, NDJSON, raw
bytes, or other machine-oriented formats. If a script depends on output shape,
choose a format such as `-o json`; themes are intentionally cosmetic.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Output](/docs/guides/output/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
