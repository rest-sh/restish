---
title: Restish vs curl and HTTPie
linkTitle: Restish vs curl and HTTPie
weight: 5
description: Decide when Restish, curl, or HTTPie is the best tool for an API job.
---

Restish is not a replacement for every HTTP tool. It is strongest when repeated
API work benefits from generated commands, profiles, auth, output shaping,
pagination, and plugins.

## Use curl When

- you need the lowest-level HTTP behavior possible
- a tiny portable script must run on machines without Restish
- you are reproducing vendor support instructions exactly

```bash
curl -H 'Accept: application/json' 'https://api.rest.sh/images?format=jpeg'
```

## Use HTTPie When

- you want a friendly generic HTTP client
- you do not need generated API commands
- you like HTTPie's request syntax for ad hoc work

```bash
http https://api.rest.sh/images Accept:application/json
```

## Use Restish When

- the API has OpenAPI and you want generated commands
- you repeat profiles, auth, headers, TLS, filters, or pagination
- you need normalized links and body filtering
- plugins should extend the workflow

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish example list-images -f body.self -o lines
```

## Side-By-Side

```bash
curl -H 'Accept: application/json' 'https://api.rest.sh/images?format=jpeg'
restish 'https://api.rest.sh/images?format=jpeg'
restish example list-images -o table --rsh-columns name,format,self
```

The first two are generic HTTP. Restish sends a useful `Accept` header by
default, so the generic Restish command stays shorter. The last command uses
the API description to expose a named command.

## Related Pages

- [Quickstart](/docs/getting-started/quickstart/)
- [Requests](../requests/)
- [API Commands](/docs/concepts/api-commands/)
