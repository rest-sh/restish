---
title: Hook Plugins
linkTitle: Hook Plugins
weight: 20
description: Learn what hook plugins can do and where they fit in the Restish request lifecycle.
---

# Hook Plugins

Hook plugins are short-lived extensions that receive a request payload and
return a response payload.

Typical uses:

- auth
- request middleware
- response middleware
- spec loading
- output formatting

Primary source:

- [`docs/design/019-hook-plugins.md`](/Users/daniel/src/restish2/docs/design/019-hook-plugins.md)
