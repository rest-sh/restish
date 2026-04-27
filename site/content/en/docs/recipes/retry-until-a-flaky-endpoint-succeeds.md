---
title: Retry Until a Flaky Endpoint Succeeds
linkTitle: Retry Flaky
weight: 61
description: Demonstrate retry recovery with a fixture that fails before succeeding.
---

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-recipe' --rsh-retry 2
```

Set retries to zero to confirm the first failure path:

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-once' --rsh-retry 0
```

Related: [Retries and Caching](/docs/guides/retries-and-caching/).
