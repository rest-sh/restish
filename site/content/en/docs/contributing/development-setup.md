---
title: Development Setup
linkTitle: Development Setup
weight: 10
description: Build, test, run, and validate Restish and the documentation site from a local checkout.
---

Use this page when changing code, plugins, or docs in the Restish repository.

## Build The CLI

```bash
go build ./cmd/restish
```

## Build Plugin Binaries

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

## Run Tests

```bash
go test ./...
go test ./internal/cli/...
```

Use a temporary `GOCACHE` when your environment needs an isolated cache:

```bash
GOCACHE=/tmp/restish-gocache go test ./...
```

## Update Golden Files

```bash
go test -update ./internal/output/...
```

## Build The Docs Site

```bash
hugo --source site --quiet
```

For docs changes, also check stale placeholders:

```bash
rg 'api[.]example[.]com|your-api[.]example[.]com|auth[.]example[.]com|upload[.]example[.]com|Source material[:]' site/content/en/docs
```

## Typical Workflow

1. Read the relevant design record before changing core behavior.
2. Make the code or docs change.
3. Update guides, recipes, reference, plugins, and troubleshooting where user behavior changed.
4. Run focused tests or `hugo` validation.
5. Record follow-ups in `TODO.md` only when they are real remaining work.

## Related Pages

- [Docs Maintenance](../docs-maintenance/)
- [Design Records](../design-records/)
