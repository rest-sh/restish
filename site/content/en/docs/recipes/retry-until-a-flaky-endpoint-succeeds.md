---
title: Retry Until a Flaky Endpoint Succeeds
linkTitle: Retry Flaky
weight: 61
description: Demonstrate retry recovery with a fixture that fails before succeeding.
---

Retries are for transient failures: temporary `5xx` responses, flaky networks,
or services that need a moment to recover. The example API includes a fixture
that fails a configurable number of times for a key, which makes retry behavior
easy to see without waiting for a real outage.

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-recipe' --rsh-retry 2
```

Set retries to zero to confirm the first failure path:

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-once' --rsh-retry 0
```

The first command has enough retries to recover. The second disables retries so
you can see the initial failure. Use unique keys while experimenting so your
results are not affected by a previous run. For policy details, read
[Retries and Caching](/docs/guides/retries-and-caching/).

By default, automatic retries apply to `GET` and `HEAD`. `--rsh-retry` controls
the attempt count. Add `--rsh-retry-unsafe` only for POST, PUT, PATCH, or
DELETE requests that can tolerate replay, since replay can repeat side effects
if a server processed an earlier attempt.

Related: [Retries and Caching](/docs/guides/retries-and-caching/).
