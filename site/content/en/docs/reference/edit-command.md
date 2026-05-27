---
title: Edit Command
linkTitle: Edit
weight: 40
description: Fetch a resource, edit it locally, and send it back.
---

`edit` is for APIs where a resource is easier to review in an editor than to
rebuild from command-line arguments. Restish fetches the resource, opens it in
your editor, then sends the changed document back with the appropriate update
request.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen edit-command -->
Generated from the current Cobra command tree.

### `restish edit`

Fetch a resource, edit it locally, then send it back

Fetch a resource, edit it locally, then send the changed representation back.

Restish first sends `GET`, opens the response body as JSON or YAML, then sends either `PATCH` with JSON Merge Patch when supported or `PUT` otherwise. Use shorthand patch arguments for non-interactive edits.

Safety controls:

- Use `--dry-run` to print the diff without sending an update.
- Use `--no-editor` to print or patch the editable body without launching `$VISUAL` or `$EDITOR`.
- Use `--yes` only after reviewing the diff in automation.

Usage:

```text
restish edit <uri> [patch ...] [flags]
```

Examples:

```bash
  restish edit https://api.example.com/items/123
  restish edit https://api.example.com/items/123 'name: Ada' --dry-run
  restish edit demo/items/123 --no-editor
```

Flags:

**`--dry-run`**

Type: `bool`; default: `false`

Show the diff without sending the update

**`--no-editor`**

Type: `bool`; default: `false`

Do not open an editor; with no patch args, print the editable resource

**`-e`, `--edit-format`**

Type: `string`; default: `json`

Editor file format: json or yaml

**`-y`, `--yes`**

Type: `bool`; default: `false`

Skip the confirmation prompt
<!-- END GENERATED -->

## Examples

```bash
restish edit api.rest.sh/types
restish edit --edit-format yaml api.rest.sh/types
restish edit --no-editor api.rest.sh/types
restish edit --dry-run api.rest.sh/types 'string: changed'
restish edit -y api.rest.sh/types 'string: changed'
```

Use these examples against the public demo API while learning the workflow. The
generated reference above is the source of truth for exact flags and safety
behavior.

## Notes

Restish compares the normalized resource value after parsing the edited file.
Editor-only formatting changes do not produce a diff and do not send an update.

The edit workflow depends on the API supporting a writable representation of the
resource. Start with a safe or resettable endpoint while learning. The
[Edit Workflow guide](/docs/guides/edit-workflow/) covers editor selection,
merge behavior, and safety habits.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Edit Workflow](/docs/guides/edit-workflow/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
