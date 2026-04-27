---
title: Download a File With Headers Preserved
linkTitle: Download With Headers
weight: 42
description: Save raw bytes while capturing verbose headers separately.
---

```bash
restish -v https://api.rest.sh/images/jpeg > dragonfly.jpg 2> dragonfly.headers.txt
```

The body goes to stdout and verbose metadata goes to stderr, so the saved file
stays clean.

Related: [Command Behavior](/docs/guides/command-behavior/), [Output](/docs/guides/output/).
