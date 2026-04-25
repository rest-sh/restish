# Edit Workflow

## Summary

Restish v2 includes an `edit` command that fetches a resource, lets the user
modify it locally or via shorthand patch arguments, shows a diff, and then
writes the change back to the server.

This turns common fetch-edit-update API workflows into one supported CLI flow
instead of making users stitch together GET, temp files, diff tools, and PUT or
PATCH requests manually.

## Goals

- fetch the current resource representation
- let users edit that representation in a familiar format
- support both full interactive editing and quick patch-style updates
- provide an explicit review step before sending changes
- preserve concurrency safeguards like ETag or Last-Modified when possible
- behave safely in non-interactive or scripted contexts

## Non-Goals

- becoming a generic merge tool for arbitrary remote documents
- preserving raw response bytes exactly through the editing loop
- guessing a safe update method without using server signals or explicit rules

## High-Level Workflow

The edit flow is:

1. GET the resource
2. decode and normalize the body into structured data
3. choose an editable representation
4. either open the editor or apply shorthand patch args
5. compare the edited value with the original
6. show a diff
7. optionally confirm
8. send the update request

If the content is unchanged, the workflow exits cleanly without sending an
update.

Both the initial GET and the final PUT/PATCH are normal Restish requests. They
must use the shared request pipeline, including profile headers, query defaults,
auth callbacks, TLS options, retry policy, middleware, and prepared request
overrides.

## Editable Representation

The editable representation is derived from decoded structured data, not from
the raw response bytes.

This is intentional because it keeps the edit experience aligned with the same
content-type and normalization model Restish uses elsewhere. It also avoids
needing a byte-preserving edit mode for every possible wire format.

The editable representation can currently be JSON or YAML via `--edit-format`.

## Editor Selection

Interactive editing should choose the editor from:

1. `$VISUAL`
2. `$EDITOR`

If neither is available, Restish should fail clearly rather than attempting an
implicit default that may surprise the user.

Editor selection is part of the runtime I/O model from design 001 and should not
bypass it.

Editor command parsing should follow shell-field rules for `$VISUAL` and
`$EDITOR` rather than splitting on whitespace by hand.

## Patch-Only Mode

Shorthand patch args are part of the same command model as full editor mode.

This means users can do:

```bash
restish edit https://api.example.com/items/123 name: Alice status: active
```

without opening the editor at all.

This is a fast path, not a separate command family.

## Diff And Review

After the edited value is produced, Restish should compute a diff between the
original logical value and the edited logical value.

The diff exists for two reasons:

- it gives the user a clear review step
- it makes dry-run mode meaningful

`--dry-run` should stop after diff generation without sending an update.

## Confirmation Semantics

By default, interactive edit mode should ask for confirmation before sending a
destructive update unless the workflow or options make that clearly redundant.

The v1-compatible `-i` flag selects the interactive edit path. It should remain
available while `edit` keeps the same command shape, even if editor mode is also
the default for some invocations.

`-y` / `--rsh-yes` skips the confirmation prompt for automation.

EOF must not be treated as implicit "yes". Non-interactive confirmation defaults
should bias toward safety.

## Update Method Selection

The update method is chosen pragmatically:

- use `PATCH` with `application/merge-patch+json` when merge patch is supported
- otherwise use `PUT` with the edited full representation

Support signals may come from response headers such as `Accept-Patch` or from
other documented API behavior.

The important design point is that Restish should not pretend merge patch is
available when the server did not signal it.

## Concurrency Protection

The edit workflow should preserve concurrency metadata from the original
response when possible:

- `ETag` -> `If-Match`
- `Last-Modified` -> `If-Unmodified-Since`

This helps prevent blind overwrites when the server supports conditional
updates.

These concurrency headers are part of the design, not an incidental convenience.

## Unsupported Content

The edit workflow assumes the resource can be represented as structured data.
If the original resource is:

- binary
- opaque text that the update path cannot round-trip meaningfully
- otherwise not suitable for structured edit mode

Restish should fail clearly instead of pretending it can offer a safe edit
experience.

## Non-Interactive Safety

For scripted or piped usage:

- shorthand patch mode should still work
- editor mode requires an explicit editor environment
- confirmation skipping must be explicit
- dry-run remains safe and useful

The design should not rely on a TTY existing for correctness.

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
restish edit https://api.example.com/items/123 name: Alice
```

Preview the diff without sending the update:

```bash
restish edit --dry-run https://api.example.com/items/123 name: Alice
```

## Alternatives Considered

### Require Users To GET And PUT Manually

Too much repetitive glue for a common task.

### Make Edit Purely Editor-Driven

Would miss a valuable fast path for one-line updates.

### Always Send Full Replacements

Too blunt when merge-patch support is available.

## Relationship To Other Designs

- Design 003 and 009 define the decode/normalize model the edit command works
  from.
- Design 008 defines shorthand patch semantics reused here.
- Design 017 defines prompting behavior.
- Design 029 places edit inside the shared request execution pipeline.
