---
title: Bulk Management
linkTitle: Bulk Management
weight: 100
description: Manage API collections as local files with the restish-bulk command plugin.
---

`restish bulk` is a command-plugin workflow for checking out many API resources
to disk, editing them locally, and pushing changes back through Restish.

## Prerequisites

- The `restish-bulk` plugin binary is installed and discoverable.
- The target API exposes collection and item URLs.
- You understand the API's update semantics before pushing changes.

Verify discovery:

```bash
restish plugin list
restish bulk --help
```

## Initialize A Checkout

The example API has a books collection used for bulk examples:

```bash
restish bulk init https://api.rest.sh/books
```

The plugin fetches the collection through Restish, writes resources to disk, and
keeps metadata needed for later status, pull, reset, and push operations.
Collection entries must include valid resource URLs. Relative URLs are resolved
against the collection URL; malformed URLs stop the checkout with a clear error
instead of creating partial local paths.

## Daily Workflow

```bash
restish bulk status
restish bulk pull
restish bulk diff
restish bulk push
```

Use `status` before `push` so you know which local files changed and whether
remote updates exist.

Bulk metadata is saved atomically after completed file updates, so interrupted
long pulls or pushes keep the last completed checkout state.

`push` is conservative. Updates and deletes require an HTTP validator
(`ETag`/`Last-Modified`) or matching local/remote version metadata. When that
evidence is missing or stale, the resource is refused instead of overwritten.
The push summary reports created, updated, deleted, skipped, and refused counts.

Use force only after reviewing the diff and accepting the overwrite risk:

```bash
restish bulk push --force
```

## Reset Local Changes

```bash
restish bulk reset
restish bulk reset path/to/item.json
```

Reset discards local changes. Use it intentionally.

## Matching Resources

Bulk operations can select resources with match expressions when the plugin
supports the workflow:

```bash
restish bulk status --match 'rating_average >= 4.8'
```

## Shape Mismatches

If your API uses different collection fields than the plugin expects, reshape
responses with filters or configure the plugin according to its help output.
Keep the HTTP work delegated to Restish so profiles, auth, retries, cache, and
TLS behavior stay consistent.

## Related Pages

- [Bulk Command](/docs/reference/bulk-command/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Example API](/docs/reference/example-api/)
