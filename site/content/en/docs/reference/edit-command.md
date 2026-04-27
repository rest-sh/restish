---
title: Edit Command
linkTitle: Edit
weight: 40
description: Fetch a resource, edit it locally, and send it back.
---

Fetch a resource, edit it locally, and send it back.

## Examples

```bash
restish edit https://api.rest.sh/types
restish edit --edit-format yaml https://api.rest.sh/types
restish edit --dry-run https://api.rest.sh/types string: changed
restish edit -y https://api.rest.sh/types string: changed
```

## Notes

Use `--dry-run` before writing and `-y` only when the update is already reviewed.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
