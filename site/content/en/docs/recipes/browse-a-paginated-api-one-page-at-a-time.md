---
title: Browse a Paginated API One Page at a Time
linkTitle: One Page at a Time
weight: 56
description: Disable automatic pagination to inspect the first page and its next link.
---

Restish follows pagination links for you by default. That is convenient once
you trust an API, but the first page is often where you learn the server's
shape: what one page contains, what link relation names it uses, and whether
the `next` link is present. Use `--rsh-no-paginate` when you want to slow down
and inspect one response at a time.

```bash
restish https://api.rest.sh/images --rsh-no-paginate
```

Show only the next link:

```bash
restish https://api.rest.sh/images --rsh-no-paginate -f links.next
```

The first command shows the page body plus normalized links. The second command
prints only the `next` relation as plain text, using the same filter roots
explained in [Links and Hypermedia](/docs/guides/links-and-hypermedia/). Use
this before tuning `--rsh-max-pages`, `--rsh-max-items`, or collect mode.

Related: [Pagination](/docs/guides/pagination/).
