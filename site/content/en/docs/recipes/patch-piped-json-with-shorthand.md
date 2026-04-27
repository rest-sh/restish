---
title: Patch Piped JSON With Shorthand
linkTitle: Patch Piped JSON
weight: 25
description: Use stdin as a base document and override fields with shorthand.
---

```bash
echo '{"name":"Alice","role":"user"}' | restish post https://api.rest.sh/post role: admin
```

The sent body has `role` changed to `admin` before encoding.

Use this when generated data needs a small command-line override.

Related: [Input and Shorthand](/docs/guides/input/), [Shorthand](/docs/reference/shorthand/).
