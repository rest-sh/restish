---
title: Edit Workflow
linkTitle: Edit Workflow
weight: 100
description: Fetch a resource, edit it locally, and submit changes back safely with Restish.
---

Restish includes an edit workflow for resource-oriented APIs.

The `edit` command turns the common fetch-edit-update cycle into one command:

1. fetch the current resource
2. open it in your editor, or apply shorthand patch args
3. show a diff
4. send the update back

## Basic Interactive Edit

```bash
restish edit https://api.rest.sh/types
```

Restish fetches the current representation, opens it in your editor, shows a
diff, and asks for confirmation before sending the update.

That makes it a safer replacement for the manual sequence of:

1. GET the current resource
2. save it locally
3. edit it
4. send it back with PATCH or PUT

## Choose The Edit Format

The editable representation can be JSON or YAML:

```bash
restish edit --edit-format json https://api.rest.sh/types
restish edit --edit-format yaml https://api.rest.sh/types
```

JSON is the default.

## Quick Patch Mode

If you already know the fields to change, pass shorthand patch args instead of
opening an editor:

```bash
restish edit https://api.rest.sh/types string: changed
```

This is the fastest path for small updates.

Conceptually, this is best when you know the exact fields you want to change
and do not need to review a full document in an editor.

## Dry Run And Confirmation

Use `--dry-run` when you want to preview the diff without sending anything:

```bash
restish edit --dry-run https://api.rest.sh/types string: changed
```

Use `-y` or `--rsh-yes` to skip the confirmation prompt in automation:

```bash
restish edit -y https://api.rest.sh/types string: changed
```

## How Updates Are Sent

Restish chooses the update method pragmatically:

- if merge patch is supported, it can send `PATCH` with
  `application/merge-patch+json`
- otherwise it falls back to `PUT` with the edited full representation

You do not need to manage that decision manually in common cases.

## Concurrency Protection

When the original response includes `Etag` or `Last-Modified`, Restish reuses
that metadata on update:

- `Etag` becomes `If-Match`
- `Last-Modified` becomes `If-Unmodified-Since`

That helps prevent accidental overwrites when the server supports conditional
requests.

This is one of the main reasons `edit` is safer than manually copy-pasting a
stale representation back to the API.

## Related Guides

- [Requests](../requests/)
- [Input and Shorthand](../input/)
- [Edit Command](/docs/reference/edit-command/)
