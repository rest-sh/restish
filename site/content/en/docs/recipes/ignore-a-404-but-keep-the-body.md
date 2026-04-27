---
title: Ignore a 404 but Keep the Body
linkTitle: Ignore 404
weight: 50
description: Inspect an error response without a failing exit code.
---

Restish treats HTTP error statuses as command failures because that is useful in
scripts and CI. Sometimes an error body is expected data, though: a `404` might
describe a missing optional resource, or a problem response might explain what
the user should fix.

```bash
restish https://api.rest.sh/status/404 --rsh-ignore-status-code
```

Use this when an error body is expected data for your script. The flag changes
the command exit behavior; it does not pretend the HTTP status was successful.

For problem details:

```bash
restish https://api.rest.sh/problem --rsh-ignore-status-code
```

Problem responses are structured, so you can filter them or send them to JSON
output just like other decoded bodies.

Related: [Command Behavior](/docs/guides/command-behavior/).
