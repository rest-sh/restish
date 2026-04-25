---
title: Bulk Management
linkTitle: Bulk Management
weight: 85
description: Check out API resources to disk, review local and remote changes, and push updates in bulk.
---

`restish bulk` gives you a Git-like workflow for APIs that expose versioned
resources.

It is best for collections where you want to:

- pull many resources to disk
- review and edit them locally
- compare local and remote changes
- push updates back in batches

## When To Use It

Bulk mode fits APIs that look roughly like this:

```text
GET    /books           -> list items with a URL and version
GET    /books/{id}      -> fetch one item
PUT    /books/{id}      -> create or replace one item
DELETE /books/{id}      -> delete one item
```

The easiest path is when the list response already exposes a resource URL and a
version field such as `url` and `etag`.

## First Checkout

The docs example API includes a books collection designed for this workflow.

```bash
mkdir books
cd books
restish bulk init https://api.rest.sh/books
```

That creates a working checkout in the current directory and stores metadata
under `.rshbulk/`.

## See What Was Checked Out

List all tracked files:

```bash
restish bulk list
```

Filter the list with a match expression:

```bash
restish bulk list --match 'rating_average >= 4.8'
restish bulk list --match 'author.lower contains brian'
```

You can also project each matched file with a shorthand filter:

```bash
restish bulk list \
  --match 'rating_average > 4.7' \
  -f 'recent_ratings[0].rating'
```

## Review Local And Remote Changes

Check the current status:

```bash
restish bulk status
```

This compares:

- local edits in your checkout
- remote changes since the last sync

See diffs for local changes:

```bash
restish bulk diff
restish bulk diff the-book.json
```

See diffs for remote updates before pulling them:

```bash
restish bulk diff --remote
```

## Edit, Reset, Pull, Push

A typical flow looks like:

```bash
# edit or add files in the checkout
restish bulk status
restish bulk diff
restish bulk push
```

If you want to undo a local change:

```bash
restish bulk reset the-book.json
```

If the server changed while you were working:

```bash
restish bulk pull
```

`pull` updates remote state without overwriting local edits. `push` applies your
local adds, edits, and deletions back to the server.

For large checkouts, `init`, `pull`, and `push` fetch or update resources with
four workers by default. Use `--jobs` to tune concurrency for slower servers or
larger collections:

```bash
restish bulk pull --jobs 8
restish bulk push --jobs 2
```

## Non-Standard List Responses

If the list response does not already expose the fields Restish expects, shape
it during `init`.

The plugin recognizes these field names automatically:

- resource URL: `url`, `uri`, `self`, `link`
- resource version: `version`, `etag`, `last_modified`, `lastModified`,
  `modified`

If your API uses different names, reshape the response with `-f` and build the
URL with `--url-template`:

```bash
restish bulk init https://api.example.com/items \
  -f 'body.items.{owner, id, version: unique_hash}' \
  --url-template '/items/{owner}/{id}'
```

## Important Limits

- New resources are expected to use client-generated identifiers and `PUT`
  semantics.
- Bulk mode is best for document-like resources you are comfortable storing on
  disk.
- The plugin owns checkout state in the current directory, so it is best used
  in a dedicated folder per collection.

## Related Pages

- [Example API](/docs/reference/example-api/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [Requests](/docs/guides/requests/)
