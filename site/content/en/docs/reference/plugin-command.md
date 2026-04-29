---
title: Plugin Command
linkTitle: Plugin
weight: 40
description: Install, list, remove, and debug Restish plugins.
---

Plugins extend Restish without making every feature part of the core binary.
Operators use the `plugin` command to install, list, remove, and debug those
extensions. Authors use the plugin reference pages to understand the protocols.

## Examples

```bash
restish plugin list
restish plugin install ./restish-csv
restish plugin install restish-csv
restish plugin install rest-sh/restish:csv
restish plugin install https://downloads.example.com/restish-csv_darwin_arm64.tar.gz
restish plugin remove restish-csv
restish plugin debug ./restish-csv
```

Use `plugin list` when a command or formatter is missing. Use `plugin debug`
when a plugin starts but does not behave as expected; it prints decoded protocol
messages so you can see where host and plugin disagree.

## Notes

Keep operator tasks separate from author tasks. Installing and verifying a
plugin should not require reading wire protocol details. Start with
[Install and Use Plugins](/docs/plugins/install-and-use/) unless you are
building a plugin yourself.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Plugins](/docs/plugins/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
