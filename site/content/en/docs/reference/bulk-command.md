---
title: Bulk Command
linkTitle: Bulk Command
weight: 34
description: Reference for the restish bulk plugin command set.
---

`restish bulk` manages a local checkout of API resources and pushes changes
back to the API.

## Commands

- `bulk init <collection-url>`: create a checkout in the current directory
- `bulk list [path...]`: list tracked resources
- `bulk status`: show local and remote change status
- `bulk diff [path...]`: show local diffs
- `bulk diff --remote`: show remote diffs
- `bulk reset [path...]`: discard local changes
- `bulk pull`: fetch remote changes without overwriting local edits
- `bulk push`: apply local adds, edits, and deletes to the API

## Common Flags

- `--jobs <n>`: concurrency for `init`, `pull`, and `push`
- `--match <expr>`: filter resources for list/status-style operations
- `-f`, `--rsh-filter <expr>`: project list output through Restish filtering
- `--url-template <template>`: build resource URLs during `init`

## Example

```bash
mkdir books
cd books
restish bulk init https://api.rest.sh/books
restish bulk status
restish bulk diff
restish bulk push
```

## Related Pages

- [Bulk Management Guide](/docs/guides/bulk-management/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [Example API](../example-api/)
