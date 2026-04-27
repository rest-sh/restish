---
title: Convert Array Output to CSV
linkTitle: Convert to CSV
weight: 82
description: Use the csv formatter plugin for array-shaped responses.
---

Verify the formatter plugin is installed:

```bash
restish plugin list
```

Render images as CSV:

```bash
restish https://api.rest.sh/images -o csv
```

This works best for arrays of similarly shaped objects.

Related: [Install and Use Plugins](/docs/plugins/install-and-use/), [Output Formats](/docs/reference/output-formats/).
