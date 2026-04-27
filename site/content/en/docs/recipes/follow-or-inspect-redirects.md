---
title: Follow or Inspect Redirects
linkTitle: Redirects
weight: 63
description: Use verbose output to inspect redirect behavior.
---

Redirects are easy to miss because the final response may look successful even
when Restish followed one or more intermediate locations. Verbose mode shows
the request and response trail on stderr, while stdout remains available for
the final response body.

```bash
restish https://api.rest.sh/redirect/2 -v
```

Choose a redirect status and target:

```bash
restish 'https://api.rest.sh/redirect-to?url=/get&status_code=307' -v
```

Use the second form when you want to test a specific redirect status. Codes such
as `307` and `308` preserve the method and body, while `301`, `302`, and `303`
are often treated differently by clients. The broader behavior is covered in
[Command Behavior](/docs/guides/command-behavior/).

Related: [Command Behavior](/docs/guides/command-behavior/).
