---
title: Plugin Messages
linkTitle: Plugin Messages
weight: 62
description: Reference for Restish host/plugin message families.
---

Restish v2 plugins communicate over structured messages. Operators usually need
`plugin debug`; authors need the message families and lifecycle rules.

## Startup Messages

`--rsh-plugin-manifest` writes one manifest map and exits.
`--rsh-plugin-commands` writes one command discovery map and exits. Command
discovery includes:

- `protocol_version`: command-plugin discovery protocol version
- `commands`: command declarations contributed by the plugin

Restish treats omitted `protocol_version` as the initial command-plugin
protocol, and rejects future versions that require a newer host.

## Command Plugin Messages

Plugin to host:

- `http-request`
- `api-spec`
- `list-apis`
- `list-profiles`
- `config-read`
- `prompt`
- `confirm`
- `response`
- `stdout-data`
- `stderr-data`
- `progress`
- `spinner`
- `log`
- `warn`
- `done`

Reply-bearing command messages may include `request_id`; hosts echo it in the
matching response. Go plugins should normally let `plugin.CommandClient`
generate and route those IDs.

Host to plugin:

- `http-response`
- `api-spec-response`
- `list-apis-response`
- `list-profiles-response`
- `config-read-response`
- `prompt-response`
- `confirm-response`
- `stdin-data`
- `stdin-close`

## Hook Plugins

Hook plugins are short-lived. They receive one focused request, response, auth,
loader, formatter, or TLS signer task and return one result or error.

## Debugging

```bash
restish plugin debug ./path/to/plugin
```

Use debug output to confirm message type, payload shape, and whether the plugin
sent `done` for command workflows.

## Related Pages

- [Command Plugins](/docs/plugins/command-plugins/)
- [Hook Plugins](/docs/plugins/hook-plugins/)
- [Plugin Command](../plugin-command/)
