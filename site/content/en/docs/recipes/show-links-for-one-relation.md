---
title: Show Links for One Relation
linkTitle: One Link Relation
weight: 59
description: Print only one normalized link relation.
---

When a response has many links, you usually only need one relation for the next
step in a script. Ask the `links` command for that relation directly, or use a
filter when the link is part of a larger request pipeline.

```bash
restish links https://api.rest.sh/images next
```

Equivalent filter form:

```bash
restish https://api.rest.sh/images -f links.next -r
```

Both commands read the same normalized `links.next` value. The command form is
nice for inspection; the filter form is nice when combined with other output
settings. See [Links and Hypermedia](/docs/guides/links-and-hypermedia/) for
how Restish finds links.

Related: [Links Command](/docs/reference/links-command/).
