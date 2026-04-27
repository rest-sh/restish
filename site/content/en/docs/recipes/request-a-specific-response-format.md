---
title: Request a Specific Response Format
linkTitle: Response Format
weight: 45
description: Send an Accept header and choose an output format.
---

```bash
restish -H 'Accept: application/json' https://api.rest.sh/formats/json
restish -H 'Accept: application/yaml' https://api.rest.sh/formats/yaml -o yaml
```

`Accept` asks the server for a representation. `-o` chooses how Restish renders
the decoded response after it arrives.

Related: [Content Types](/docs/reference/content-types/), [Output](/docs/guides/output/).
