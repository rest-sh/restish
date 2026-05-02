---
title: Convert Array Output to CSV
linkTitle: Convert to CSV
weight: 82
description: Use the csv formatter plugin for array-shaped responses.
---

CSV is useful when the next tool is a spreadsheet, a reporting job, or a
command-line pipeline that expects rows. Restish does not assume every user
wants CSV built in; the formatter can come from a plugin, while the response
decoding and filtering still happen in the main CLI.

Verify the formatter plugin is installed:

```bash
restish plugin list
```

Render images as CSV:

```bash
restish https://api.rest.sh/images -o csv
```

This works best for arrays of similarly shaped objects. If the response is
nested, filter it first so the formatter sees only the fields you want, as shown
in the [Filtering guide](/docs/guides/filtering/).
For paginated or streamed data, the CSV header is fixed from the first batch.
Missing fields become empty cells; newly appearing fields are ignored with a
warning.

Related: [Install and Use Plugins](/docs/plugins/install-and-use/), [Output Formats](/docs/reference/output-formats/).
