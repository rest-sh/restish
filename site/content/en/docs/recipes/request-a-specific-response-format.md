---
title: Request a Specific Response Format
linkTitle: Response Format
weight: 45
description: Send an Accept header and choose an output format.
---

There are two different format decisions in an HTTP request. `Accept` tells the
server which representation you prefer. `-o` tells Restish how to render the
decoded response for your terminal, file, or pipeline.

```bash
restish -H 'Accept: application/json' https://api.rest.sh/formats/json
restish -H 'Accept: application/yaml' https://api.rest.sh/formats/yaml -o yaml
```

Use this when a server negotiates aggressively or when you need to prove that a
specific response type decodes correctly. The [Content Types reference](/docs/reference/content-types/)
lists the built-in decoders.

Related: [Content Types](/docs/reference/content-types/), [Output](/docs/guides/output/).
