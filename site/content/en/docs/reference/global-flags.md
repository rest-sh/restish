---
title: Global Flags
linkTitle: Global Flags
weight: 12
description: Reference for flags shared across most Restish commands.
---

Global flags apply across generic requests, generated API commands, utilities,
and many plugin-delegated requests.

Use the CLI reference command to see the same groups from your installed
binary:

```bash
restish flags
restish flags output
```

## Request Construction

```bash
restish -H 'Accept: application/json' https://api.rest.sh/headers
restish -q api_key=docs-key https://api.rest.sh/auth/api-key-query
restish post -c form https://api.rest.sh/login 'username: alice, password: secret'
restish --rsh-server https://api.rest.sh example list-images
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

Flags: `-H/--rsh-header`, `-q/--rsh-query`, `-c/--rsh-content-type`,
`-s/--rsh-server`, `-t/--rsh-timeout`, `--rsh-max-body-size`,
`--rsh-ignore-status-code`.

## Output And Filtering

```bash
restish https://api.rest.sh/images -f body.self -o lines
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
restish https://api.rest.sh/images -o ndjson -f body.self
restish https://api.rest.sh/images --rsh-sort-by name -o table
restish https://api.rest.sh/ --rsh-headers
restish -S https://api.rest.sh/status/204
```

Flags: `-f/--rsh-filter`, `--rsh-filter-lang`, `-o/--rsh-output-format`,
`-r/--rsh-raw`, `--rsh-columns`, `--rsh-sort-by`, `--rsh-headers`,
`-S/--rsh-silent`.

## Auth And Profiles

```bash
restish -p json example list-images
restish --rsh-no-browser api connect example https://api.rest.sh 'prompt.api_key: docs-key'
```

Flags: `-p/--rsh-profile`, `--rsh-no-browser`.

## TLS

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
restish --rsh-client-cert ./client.pem --rsh-client-key ./client-key.pem https://mtls.internal.test/items
restish --rsh-insecure https://service.internal.test/items
restish --rsh-tls-min-version TLS1.3 https://api.rest.sh
```

Flags: `--rsh-ca-cert`, `--rsh-client-cert`, `--rsh-client-key`,
`--rsh-insecure`, `--rsh-tls-min-version`, `--rsh-tls-signer`,
`--rsh-tls-signer-param`.

## Pagination And Streaming

```bash
restish https://api.rest.sh/images --rsh-no-paginate
restish https://api.rest.sh/images --rsh-max-pages 3
restish https://api.rest.sh/images --rsh-max-items 100
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

## Cache And Retry

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=flags' --rsh-retry 2
restish post https://api.example.com/jobs name:demo --rsh-retry 2 --rsh-retry-unsafe
restish https://api.rest.sh/status/429 --rsh-retry-max-wait 30s
restish https://api.rest.sh/cache --rsh-no-cache
```

Default retry behavior is conservative for `GET` and `HEAD`. `--rsh-retry`
sets the retry count. Add `--rsh-retry-unsafe` only when the current POST, PUT,
PATCH, or DELETE request can safely be replayed. `--rsh-retry-max-wait` caps
server-provided `Retry-After` and `X-Retry-In` delays; the default cap is 5
minutes.

## General

```bash
restish -v https://api.rest.sh/headers
restish -vv https://api.rest.sh/headers
restish --rsh-config ./restish.json api list
```

## Related Pages

- [Requests](/docs/guides/requests/)
- [Output](/docs/guides/output/)
- [Config](../config/)
