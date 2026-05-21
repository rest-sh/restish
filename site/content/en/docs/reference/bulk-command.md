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
restish bulk init api.rest.sh/books
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

## Generated Plugin Help

<!-- BEGIN GENERATED: restish-docgen bulk-help -->
Generated from the compiled `restish-bulk` plugin binary.

### `restish bulk --help`

```text
Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.

Use `bulk init` on a list endpoint that returns resource URLs and versions. Then use `bulk status`, `bulk diff`, `bulk pull`, and `bulk push` in the checkout directory.

Usage:
  bulk [flags]
  bulk [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  diff        Show local or remote diffs
  help        Help about any command
  init        Initialize a new bulk checkout
  list        List checked out files
  pull        Pull remote updates without overwriting local changes
  push        Upload local changes to the remote server
  reset       Undo local changes to files
  status      Show local and remote added/changed/removed files

Flags:
  -h, --help   help for bulk

Use "bulk [command] --help" for more information about a command.
```

### `restish bulk init --help`

```text
Initialize a bulk checkout from a list endpoint that returns each resource URL and version.

Use `-f` to project or filter the list response before URL extraction. Use `--url-template` when the list items contain IDs or fields that need to be turned into resource URLs.

Usage:
  bulk init URL [flags]

Aliases:
  init, i

Flags:
  -f, --filter string         Filter/project the list response before extracting url/version
  -h, --help                  help for init
  -j, --jobs int              Maximum concurrent resource requests (default 4)
      --url-template string   URL template to build resource links, e.g. /users/{id}
```

### `restish bulk list --help`

```text
List files tracked by the current bulk checkout.

Use `--match` to restrict files by expression and `-f` to print projected content from each matching JSON file.

Usage:
  bulk list [flags]

Aliases:
  list, ls

Flags:
  -f, --filter string   Show projected content for each matched file
  -h, --help            help for list
  -m, --match string    Expression to match
```

### `restish bulk status --help`

```text
Show local and remote added, changed, and removed resources for the current checkout.

Use this before `bulk pull` or `bulk push` to see whether the remote API or local files have changed since the last recorded version.

Usage:
  bulk status [flags]

Aliases:
  status, st

Flags:
  -h, --help   help for status
```

### `restish bulk diff --help`

```text
Show local diffs for tracked files, or remote diffs with `--remote`.

Pass file names to focus the diff. Use `--match` to select files by expression when file paths are inconvenient.

Usage:
  bulk diff [file...] [flags]

Aliases:
  diff, di

Flags:
  -h, --help           help for diff
  -m, --match string   Expression to match
      --remote         Show remote diffs instead of local
```

### `restish bulk pull --help`

```text
Fetch remote changes for the current checkout without overwriting local edits.

Use this after `bulk status` reports remote changes. `--jobs` controls how many resource requests run concurrently.

Usage:
  bulk pull [flags]

Aliases:
  pull, pl

Flags:
  -h, --help       help for pull
  -j, --jobs int   Maximum concurrent resource requests (default 4)
```

### `restish bulk push --help`

```text
Upload local changes from the current checkout to the remote API.

By default, bulk uses recorded `ETag`, `Last-Modified`, or version preconditions when available so remote changes are not silently overwritten. Use `--force` only when you intentionally want to push without those guards.

Usage:
  bulk push [flags]

Aliases:
  push, ps

Flags:
      --force      Push without ETag/Last-Modified or matching version preconditions
  -h, --help       help for push
  -j, --jobs int   Maximum concurrent resource requests (default 4)
```

### `restish bulk reset --help`

```text
Undo local changes in the current checkout by restoring tracked files to their last recorded version.

Pass file names or use `--match` to limit what is reset. This changes local files only; it does not send requests to the remote API.

Usage:
  bulk reset [file...] [flags]

Aliases:
  reset, re

Flags:
  -h, --help           help for reset
  -m, --match string   Expression to match
```
<!-- END GENERATED -->

## Related Pages

- [Commands](/docs/reference/commands/)
- [Bulk Management](/docs/plugins/bulk-management/)
- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
