---
title: Create, Patch, and Delete an Item Safely
linkTitle: CRUD Item Safely
weight: 23
description: Use the resettable /items fixture with a unique ID.
---

Use a unique ID so your example does not collide with someone else's state:

```bash
ITEM_ID="docs-$(date +%s)"
restish post https://api.rest.sh/items "id: $ITEM_ID, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z"
restish patch "https://api.rest.sh/items/$ITEM_ID" 'enabled: false, tags[]: docs'
restish delete "https://api.rest.sh/items/$ITEM_ID" --rsh-ignore-status-code
```

Check the collection:

```bash
restish https://api.rest.sh/items
```

Related: [HTTP Commands](/docs/reference/http-commands/), [Requests](/docs/guides/requests/).
