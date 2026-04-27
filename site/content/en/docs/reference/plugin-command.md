---
title: Plugin Command
linkTitle: Plugin
weight: 40
description: Install, list, remove, and debug Restish plugins.
---

Install, list, remove, and debug Restish plugins.

## Examples

```bash
restish plugin list
restish plugin install ./restish-csv
restish plugin remove restish-csv
restish plugin debug ./restish-csv
```

## Notes

Use `plugin debug` to inspect decoded protocol messages when a plugin fails to start or respond.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
