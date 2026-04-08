---
title: Filter Response Fields
linkTitle: Filter Response Fields
weight: 30
description: Extract the fields you need from a Restish response.
---

# Filter Response Fields

When a response is larger than you need, use a filter to select the part that
matters.

Use shorthand for direct paths:

```bash
restish get https://api.example.com/items -f body.items[0].name
restish get https://api.example.com/items -f headers.Content-Type
restish get https://api.example.com/items -f links.next
```

Use jq when you need selection or reshaping:

```bash
restish get https://api.example.com/items -f '.body.items[] | select(.active) | .name'
restish get https://api.example.com/items -f '.body.items | length'
```

Add `-r` for shell-friendly output:

```bash
restish get https://api.example.com/items -f '.body.items[] | .name' -r
```

That strips quotes from strings and prints one scalar per line.
