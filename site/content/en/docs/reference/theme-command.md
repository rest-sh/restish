---
title: Theme Command
linkTitle: Theme Command
weight: 33
description: Reference for installing terminal readable-output themes.
---

The `theme` command installs style settings used by Restish's readable terminal
output.

## Syntax

```bash
restish theme set <url-or-user/repo> [name]
```

Use a full URL when a theme file is hosted directly. Use `user/repo` when the
theme lives in a GitHub repository supported by the theme loader.

The optional `name` selects a named theme from sources that contain more than
one theme.

## Config

Installed theme entries are stored in top-level `theme` in `restish.json`.

```jsonc
{
  "theme": {
    "string": "green",
    "number": "cyan"
  }
}
```

## Related Pages

- [Config](../config/)
- [Output Formats](../output-formats/)
