---
title: Hook Plugins
linkTitle: Hook Plugins
weight: 20
description: Learn what hook plugins can do and where they fit in the Restish request lifecycle.
---

Hook plugins are short-lived extensions that receive a request payload and
return a response payload.

Typical uses:

- auth
- request middleware
- response middleware
- spec loading
- output formatting

## Lifecycle

Hook plugins are designed for one-shot work:

1. Restish starts the plugin
2. Restish writes one CBOR request message to stdin
3. the plugin writes one reply or formatter output
4. the plugin exits

## When To Choose A Hook Plugin

Choose a hook plugin when the job can be expressed as one focused request-time
operation, such as:

- adding auth headers
- mutating request headers
- rewriting or suppressing a normalized response
- teaching Restish about another spec format
- adding a new output formatter

If you need a persistent workflow, multiple HTTP calls, or your own command
tree, use a [command plugin](../command-plugins/) instead.

Primary source:

- [`docs/design/019-hook-plugins.md`](/docs/contributing/design-records/)
