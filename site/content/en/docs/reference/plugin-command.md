---
title: Plugin Commands
linkTitle: Plugin Commands
weight: 18
description: Reference for listing, installing, removing, and debugging Restish plugins.
---

The `plugin` command group manages installed and discovered plugins.

## Subcommands

- `restish plugin list`
- `restish plugin install <source>`
- `restish plugin remove <name>`
- `restish plugin debug <name> [args...]`

## Examples

```bash
restish plugin list
restish plugin install ./restish-my-plugin
restish plugin remove restish-my-plugin
restish plugin debug restish-my-plugin
```

## Related Pages

- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Plugins Reference](../plugins/)
