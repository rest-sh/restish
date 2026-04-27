---
title: Edit Command
linkTitle: Edit
weight: 40
description: Fetch a resource, edit it locally, and send it back.
---

`edit` is for APIs where a resource is easier to review in an editor than to
rebuild from command-line arguments. Restish fetches the resource, opens it in
your editor, then sends the changed document back with the appropriate update
request.

## Examples

```bash
restish edit https://api.rest.sh/types
restish edit --edit-format yaml https://api.rest.sh/types
restish edit --dry-run https://api.rest.sh/types string: changed
restish edit -y https://api.rest.sh/types string: changed
```

Use `--edit-format yaml` when YAML is easier to read than JSON. Use `--dry-run`
to see the outgoing update without sending it. Use `-y` only when you already
reviewed the change and want to skip confirmation.

## Notes

The edit workflow depends on the API supporting a writable representation of the
resource. Start with a safe or resettable endpoint while learning. The
[Edit Workflow guide](/docs/guides/edit-workflow/) covers editor selection,
merge behavior, and safety habits.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Edit Workflow](/docs/guides/edit-workflow/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
