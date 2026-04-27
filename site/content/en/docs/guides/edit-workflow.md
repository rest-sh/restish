---
title: Edit Workflow
linkTitle: Edit Workflow
weight: 65
description: Fetch a resource, edit it locally, review the diff, and send it back safely.
---

`restish edit` is for fetch-edit-update workflows. It gets the resource,
opens an editor or applies shorthand changes, then writes the result back.

## Edit In Your Editor

```bash
restish edit https://api.rest.sh/types
```

Restish uses `$VISUAL`, then `$EDITOR`, and falls back to the platform default
where possible.

## Choose The Edit Format

```bash
restish edit --edit-format json https://api.rest.sh/types
restish edit --edit-format yaml https://api.rest.sh/types
```

## Apply A Shorthand Patch

```bash
restish edit --dry-run https://api.rest.sh/types string: changed
restish edit -y https://api.rest.sh/types string: changed
```

Use `--dry-run` to inspect what would be sent. Use `-y` when you want to skip
interactive confirmation.

## Conditional Requests

When the initial GET returns validators such as `ETag` or `Last-Modified`,
Restish uses conditional update headers where possible so it does not overwrite
a resource that changed while you were editing.

Use the ETag fixture to understand conditional behavior separately:

```bash
restish https://api.rest.sh/etag/docs -v
```

## Related Pages

- [Edit Command](/docs/reference/edit-command/)
- [Input and Shorthand](../input/)
- [Retries and Caching](../retries-and-caching/)
- [Command Behavior](../command-behavior/)
