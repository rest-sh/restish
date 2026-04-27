---
title: Create, Patch, and Delete an Item Safely
linkTitle: CRUD Item Safely
weight: 23
description: Use the resettable /items fixture with a unique ID.
---

This recipe practices a full create-update-delete loop against the public
example API. Because `/items` is shared and resettable, use a unique ID so your
example does not collide with someone else's state:

```bash
ITEM_ID="docs-$(date +%s)"
restish post https://api.rest.sh/items "id: $ITEM_ID, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z"
restish patch "https://api.rest.sh/items/$ITEM_ID" 'enabled: false, tags[]: docs'
restish delete "https://api.rest.sh/items/$ITEM_ID" --rsh-ignore-status-code
```

The first command creates an item from shorthand fields. The patch command sends
only the fields you want to change. The delete command uses
`--rsh-ignore-status-code` so cleanup remains calm if the item is already gone;
see [Command Behavior](/docs/guides/command-behavior/) for how HTTP statuses
map to exit codes.

Check the collection:

```bash
restish https://api.rest.sh/items
```

Related: [HTTP Commands](/docs/reference/http-commands/), [Requests](/docs/guides/requests/).
