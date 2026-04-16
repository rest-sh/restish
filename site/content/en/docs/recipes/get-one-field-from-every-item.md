---
title: Get One Field From Every Item
linkTitle: One Field Per Item
weight: 27
description: Extract a single field from each item in a response.
---

Use a jq filter plus raw output when you want one field per item:

```bash
restish https://api.rest.sh/images -f '.body[] | .name' -r
```

Example output:

```text
Dragonfly macro
Origami under blacklight
Andy Warhol mural in Miami
Station in Prague
Chihuly glass in boats
```
