---
title: Filter Response Fields
linkTitle: Filter Response Fields
weight: 30
description: Extract the fields you need from a Restish response.
---

When a response is larger than you need, use a filter to select the part that
matters.

Use shorthand for direct paths:

```bash
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
```

Example output:

```json
[
  {
    "network": "Github",
    "url": "https://github.com/danielgtaylor"
  },
  {
    "network": "Dev Blog",
    "url": "https://dev.to/danielgtaylor"
  },
  {
    "network": "LinkedIn",
    "url": "https://www.linkedin.com/in/danielgtaylor"
  }
]
```

```text
application/cbor
```

```text
https://api.rest.sh/images?cursor=abc123
```

Use jq when you need selection or reshaping:

```bash
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name'
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

Example output:

```text
Dragonfly macro
```

```text
5
```

Add `-r` for shell-friendly output:

```bash
restish https://api.rest.sh/images -f '.body[] | .name' -r
```

That strips quotes from strings and prints one scalar per line.

Example output:

```text
Dragonfly macro
Origami under blacklight
Andy Warhol mural in Miami
Station in Prague
Chihuly glass in boats
```
