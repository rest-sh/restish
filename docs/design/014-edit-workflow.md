# Edit Workflow

## Summary

Restish v2 includes an `edit` command that fetches a resource, lets the user
modify it locally or via shorthand patch arguments, shows a diff, and then
writes the change back to the server.

This turns common fetch-edit-update API workflows into a single CLI action.

## Problem

Many APIs expose resources that are easier to update by editing the current
representation than by constructing a full replacement body from scratch.

The design needed to:

- fetch the current resource representation
- let users edit that representation in a familiar format
- support both full interactive editing and quick patch-style updates
- provide a chance to review the change before sending it
- use concurrency safeguards like ETag or Last-Modified when possible

## Design

The `edit` workflow is:

1. GET the resource
2. normalize and decode the body
3. render it to an editor-friendly format
4. let the user edit it locally, or apply shorthand patch args instead
5. show a diff
6. optionally confirm
7. send the update back

The editable representation can currently be JSON or YAML via `--edit-format`.

The update method is chosen pragmatically:

- use `PATCH` with `application/merge-patch+json` when merge patch is supported
- otherwise use `PUT` with the edited full representation

The workflow also reuses response metadata when available:

- `Etag` becomes `If-Match`
- `Last-Modified` becomes `If-Unmodified-Since`

That helps avoid overwriting changes blindly when the server supports
conditional updates.

Some design choices worth preserving:

- patch args and full editor mode are part of the same command model
- unchanged content exits cleanly without sending an update
- `--dry-run` stops after diff generation
- `-y` or `--rsh-yes` skips the confirmation prompt for automation

## Examples

Interactive edit:

```bash
restish edit https://api.example.com/items/123
```

Edit as YAML instead of JSON:

```bash
restish edit --edit-format yaml https://api.example.com/items/123
```

Patch with shorthand instead of opening an editor:

```bash
restish edit https://api.example.com/items/123 name: Alice status: active
```

Preview the diff without sending the update:

```bash
restish edit --dry-run https://api.example.com/items/123 name: Alice
```

## Alternatives Considered

### Require users to GET and PUT manually

That works, but it pushes a lot of repetitive glue work onto users for a very
common API interaction pattern.

### Make edit purely editor-driven

That would miss a useful fast path. Shorthand patch args make quick one-line
updates much more convenient.

### Always send full replacements

Full replacement is straightforward, but merge patch support is a better fit
when the server advertises it and the client only changed part of the document.

## Notes

The current implementation reflects this design directly:

- `internal/cli/edit.go` implements the full fetch-edit-diff-update loop
- shorthand patch args are applied against the fetched decoded body
- concurrency headers are reused when the original response provides them

One detail worth preserving is that `edit` operates on decoded structured data,
not opaque response bytes. That keeps the editing experience aligned with the
same content-type and normalization model used elsewhere in the CLI.
