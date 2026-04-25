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

## What Each Subcommand Does

- `plugin list`: show discovered and installed plugins
- `plugin install <source>`: add a plugin from a local path or supported source
- `plugin remove <name>`: remove an installed plugin
- `plugin debug <name> [args...]`: run a plugin in debug mode to inspect its behavior

## Typical Workflow

1. install or build the plugin
2. run `restish plugin list` to confirm discovery
3. use the plugin through its command surface or hook integration
4. run `restish plugin debug ...` if discovery or runtime behavior looks wrong

Plugin discovery only loads binaries from the Restish plugin directory. Use
`plugin install` to copy a local build there.

## Related Pages

- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Plugins Reference](../plugins/)
- [Config](../config/)
