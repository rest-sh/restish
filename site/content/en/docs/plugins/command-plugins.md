---
title: Command Plugins
linkTitle: Command Plugins
weight: 30
description: Add new top-level Restish workflows with long-lived command plugins.
---

Command plugins add top-level commands and can delegate HTTP behavior back to
the host instead of rebuilding their own client stack.

## When To Choose A Command Plugin

Choose a command plugin when your feature needs:

- a top-level command such as `restish bulk`
- multiple HTTP requests in one workflow
- progress updates while work is running
- passthrough stdin/stdout for interactive sessions

If one request in and one reply out is enough, a [hook plugin](../hook-plugins/)
is usually the better fit.

## How They Integrate

Command plugins declare the `command` hook in their manifest and return command
declarations from `--rsh-plugin-commands`.

Each declaration becomes a root command in Restish. From there, the plugin can
ask Restish to make HTTP requests, print normalized responses, or emit raw
stdout and stderr data directly.

That lets the plugin reuse Restish's auth, TLS, retries, cache, and response
normalization behavior instead of rebuilding its own client stack.

Primary source:

- [`docs/design/020-command-plugins.md`](/docs/contributing/design-records/)
