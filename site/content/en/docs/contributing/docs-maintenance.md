---
title: Docs Maintenance
linkTitle: Docs Maintenance
weight: 30
description: Contributor checklist for user-facing documentation, validation steps, and migration status.
---

Use this page when you are changing user-facing behavior and need to make the
docs stay honest.

## Docs Checklist

When a feature changes, check these documentation surfaces:

- getting-started impact
- guide impact
- recipe impact
- reference impact
- plugin/operator impact
- design-record impact

Do not assume one page update is enough. Restish docs are intentionally layered.

## Validation Steps

Before sending a docs change:

1. build the site locally
2. click through the new links
3. confirm examples are still internally consistent
4. prefer `api.rest.sh` examples when a live endpoint makes the explanation
   stronger

Local build:

```bash
hugo --source site --quiet
```

## Example Validation Guidance

Prefer examples that can be:

- run against `api.rest.sh`
- exercised in local manual verification
- reflected in future CI or golden tests when the workflow is stable enough

Not every example can be fully live. When it cannot, keep placeholders clearly
generic and explain why.

## Migration Status

The v2 docs site is no longer a thin skeleton. Most high-value content from the
older docs and design records has been migrated into:

- getting-started pages
- workflow guides
- recipes
- command/reference pages
- plugin/operator docs

Remaining work is mostly refinement:

- more example standardization
- more cross-linking
- diagrams where they materially help
- periodic audits against the CLI surface and design docs

## Old Docs Topic Tracking

Topics intentionally carried forward into the new site include:

- bulk workflows
- OpenAPI CLI integration
- shorthand input
- retries, caching, pagination, links, and output behavior

Topics still tracked as ongoing refinement rather than one-time migration:

- command example consistency
- broader live-example coverage
- contributor validation workflow

## Related Pages

- [Development Setup](./development-setup/)
- [Design Records](./design-records/)
