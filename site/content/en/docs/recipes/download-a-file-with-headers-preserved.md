---
title: Download a File With Headers Preserved
linkTitle: Download With Headers
weight: 42
description: Save raw bytes while capturing verbose headers separately.
---

When you save a response to a file, stdout should contain only the body bytes.
Verbose diagnostics are written to stderr, so you can redirect them to a
separate file without corrupting the download.

```bash
restish -v https://api.rest.sh/images/jpeg > dragonfly.jpg 2> dragonfly.headers.txt
```

The body goes to stdout and verbose metadata goes to stderr, so the saved file
stays clean. Unfiltered responses redirect as body bytes by default; see
[Output Defaults](/docs/reference/output-defaults/) for how redirects choose
between body bytes and formatted values.

Related: [Command Behavior](/docs/guides/command-behavior/), [Output](/docs/guides/output/).
