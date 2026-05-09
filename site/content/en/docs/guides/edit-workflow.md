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
restish edit api.rest.sh/types
```

Restish uses `$VISUAL`, then `$EDITOR`, and falls back to the platform default
where possible.

After the editor exits, Restish parses the file and compares the normalized
resource value. If the editor only changed whitespace, indentation, key spacing,
or trailing newlines, Restish reports no changes and does not send an update.

When you pass shorthand patch arguments, Restish stays in patch-only mode and
does not open an editor.

## Choose The Edit Format

```bash
restish edit --edit-format json api.rest.sh/types
restish edit --edit-format yaml api.rest.sh/types
```

## Apply A Shorthand Patch

```bash
restish edit --dry-run api.rest.sh/types string: changed
restish edit -y api.rest.sh/types string: changed
```

Use `--dry-run` to inspect what would be sent. Use `-y` when you want to skip
interactive confirmation.

Use `--no-editor` without patch args when you only want to review the editable
representation:

```bash
restish edit --no-editor api.rest.sh/types
```

## Conditional Requests

When the initial GET returns validators such as `ETag` or `Last-Modified`,
Restish uses conditional update headers where possible so it does not overwrite
a resource that changed while you were editing.
If the response has neither validator, Restish warns before confirmation that
the update cannot be guarded against concurrent edits.

Use the ETag fixture to understand conditional behavior separately:

```bash
restish api.rest.sh/etag/docs -v
```

## Related Pages

- [Edit Command](/docs/reference/edit-command/)
- [Input and Shorthand](../input/)
- [Retries and Caching](../retries-and-caching/)
- [Command Behavior](../command-behavior/)
