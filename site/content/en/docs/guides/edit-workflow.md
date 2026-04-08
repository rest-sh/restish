---
title: Edit Workflow
linkTitle: Edit Workflow
weight: 100
description: Fetch a resource, edit it locally, and submit changes back safely with Restish.
---

# Edit Workflow

Restish includes an edit workflow for resource-oriented APIs.

The `edit` command turns the common fetch-edit-update cycle into one command:

1. fetch the current resource
2. open it in your editor, or apply shorthand patch args
3. show a diff
4. send the update back

## Basic Interactive Edit

```bash
restish edit https://api.example.com/items/123
```

Restish fetches the current representation, opens it in your editor, shows a
diff, and asks for confirmation before sending the update.

## Choose The Edit Format

The editable representation can be JSON or YAML:

```bash
restish edit --edit-format json https://api.example.com/items/123
restish edit --edit-format yaml https://api.example.com/items/123
```

JSON is the default.

## Quick Patch Mode

If you already know the fields to change, pass shorthand patch args instead of
opening an editor:

```bash
restish edit https://api.example.com/items/123 name: Alice status: active
```

This is the fastest path for small updates.

## Dry Run And Confirmation

Use `--dry-run` when you want to preview the diff without sending anything:

```bash
restish edit --dry-run https://api.example.com/items/123 name: Alice
```

Use `-y` or `--rsh-yes` to skip the confirmation prompt in automation:

```bash
restish edit -y https://api.example.com/items/123 status: active
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

## Related Guides

- [Requests](../requests/)
- [Input and Shorthand](../input/)

Source material:

- [`docs/design/014-edit-workflow.md`](/Users/daniel/src/restish2/docs/design/014-edit-workflow.md)
