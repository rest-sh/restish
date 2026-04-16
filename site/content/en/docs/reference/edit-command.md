---
title: Edit Command
linkTitle: Edit Command
weight: 12
description: Reference for the fetch-edit-update workflow exposed by restish edit.
---

`restish edit <uri> [patch ...]` turns the fetch-edit-update cycle into one
command.

## Common Forms

```bash
restish edit https://api.rest.sh/types
restish edit --edit-format yaml https://api.rest.sh/types
restish edit --dry-run https://api.rest.sh/types string: changed
restish edit -y https://api.rest.sh/types string: changed
```

## Important Flags

- `--edit-format`: `json` or `yaml`
- `--dry-run`: show the diff without sending the update
- `-y`, `--rsh-yes`: skip the confirmation prompt

## Related Pages

- [Edit Workflow](/docs/guides/edit-workflow/)
- [Input and Shorthand](/docs/guides/input/)
