---
title: Bulk Command
linkTitle: Bulk
weight: 40
description: Reference for the restish-bulk command plugin.
---

`restish-bulk` is a command plugin for workflows where a collection needs to be
pulled down, edited locally, and pushed back in a controlled way. It is useful
for repeatable content or data maintenance tasks where one request at a time is
too slow or too error-prone.

## Examples

```bash
restish bulk init https://api.rest.sh/books
restish bulk status
restish bulk pull
restish bulk diff
restish bulk push
restish bulk reset
```

`init` starts a bulk workspace for a collection. `status` shows local and remote
state. `pull` refreshes local data. `diff` previews local edits. `push` sends
local changes only when it has a safe precondition such as an ETag,
Last-Modified value, or matching local/remote version metadata. `reset` returns
the workspace to a clean state.

Use `bulk push --force` only for an intentional overwrite when the API does not
provide validators or when you have separately resolved the conflict. Push
output summarizes created, updated, deleted, skipped, and refused resources.

## Notes

Bulk is provided by a command plugin. Verify plugin discovery with
`restish plugin list` before using it. Operator setup is covered in
[Install and Use Plugins](/docs/plugins/install-and-use/); plugin protocol
details live in the author docs.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Bulk Management](/docs/guides/bulk-management/)
- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
