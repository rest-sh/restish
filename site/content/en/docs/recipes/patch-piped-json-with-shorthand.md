---
title: Patch Piped JSON With Shorthand
linkTitle: Patch Piped JSON
weight: 25
description: Use stdin as a base document and override fields with shorthand.
---

This pattern is useful when another command already produced most of the JSON
you need, but one or two fields must change before sending it. Restish reads the
stdin document, applies shorthand arguments as patches, then encodes the final
request body.

```bash
echo '{"name":"Alice","role":"user"}' | restish post https://api.rest.sh/post role: admin
```

The sent body has `role` changed to `admin` before encoding.

Use this when generated data needs a small command-line override. For larger or
repeatable edits, prefer a checked-in input file or the workflow in
[Input and Shorthand](/docs/guides/input/).

Related: [Input and Shorthand](/docs/guides/input/), [Shorthand](/docs/reference/shorthand/).
