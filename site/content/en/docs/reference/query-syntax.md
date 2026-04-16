---
title: Query Syntax
linkTitle: Query Syntax
weight: 31
description: Reference for the two query/filter languages Restish supports when projecting response data.
---

Restish supports two filter languages:

- shorthand path syntax for direct field access
- `jq` for richer selection and transformation

## Auto Detection

If the filter starts with `body`, `headers`, `links`, `status`, `proto`, or
`@`, Restish treats it as shorthand.

Otherwise, Restish treats it as `jq`.

## Shorthand Examples

```bash
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
```

## jq Examples

```bash
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name'
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

## Related Pages

- [Filtering](/docs/guides/filtering/)
- [Shorthand Syntax](../shorthand/)
