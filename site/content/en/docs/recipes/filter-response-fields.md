---
title: Filter Response Fields
linkTitle: Filter Response Fields
weight: 30
description: Extract headers, links, or nested body fields from a response.
---

Restish normalizes every response into roots such as `headers`, `links`, and
`body`. Filters let you pick one part of that model instead of scanning the
whole response. Start with shorthand paths for simple selections, then switch
to jq expressions when you need conditionals or transforms.

```bash
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
```

Use jq for richer transforms:

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body[] | select(.format == "jpeg") | .name' -o lines
```

Explicit scalar filters print plain text by default. Use `-o lines` when the
selected value is an array or stream of strings and you want shell-friendly
line output. The [Normalized Responses concept](/docs/concepts/normalized-responses/)
shows why the same roots work across body data, headers, and links.

Related: [Filtering](/docs/guides/filtering/), [Query Syntax](/docs/reference/query-syntax/).
