---
title: Get One Field From Every Item
linkTitle: Get One Field
weight: 35
description: Print one field per collection item.
---

This is the most common shell pattern: call a list endpoint, select one field
from each item, and print the result without JSON quotes. It is useful for
loops, copy-paste, and quick checks.

{{< restish-example >}}
restish https://api.rest.sh/images -f body.self -r
{{< /restish-example >}}

Example output:

```text
/images/jpeg
/images/webp
/images/gif
/images/png
/images/heic
```

`body.self` selects the `self` field from each item in the response body. `-r`
turns the selected strings into plain text. For more complex selection, use the
jq-style filter examples in [Filtering](/docs/guides/filtering/).

Related: [Filtering](/docs/guides/filtering/).
