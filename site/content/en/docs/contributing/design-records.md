---
title: Design Records
linkTitle: Design Records
weight: 20
description: Maintainer-oriented records that preserve Restish v2 design intent, invariants, and user-doc triggers.
---

Design records live in `docs/design/`. They explain why Restish is shaped the
way it is: runtime boundaries, config, auth, TLS, OpenAPI command generation,
request execution, response normalization, plugins, security, and migration.

## When To Read Them

Read design records when you are changing:

- persistent config or profile behavior
- auth, token storage, external tools, or OAuth flows
- TLS, mTLS, or signer plugins
- OpenAPI discovery or generated commands
- request execution order
- pagination, streaming, filtering, or output contracts
- plugin protocol or trust boundaries
- v1 compatibility and migration behavior

## User Docs Trigger

If a design record changes user-visible behavior, update the user docs in the
same work. Design records preserve intent; they do not replace guides,
recipes, or reference pages.

## Reading Order

Start with `docs/design/README.md`. It groups records into foundations,
request/API model, response/data flow, workflows/UX, and extensibility.

## Related Pages

- [Docs Maintenance](../docs-maintenance/)
- [Development Setup](../development-setup/)
- [How Restish Works](/docs/concepts/how-restish-works/)
