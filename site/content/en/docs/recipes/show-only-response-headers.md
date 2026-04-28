---
title: Show Only Response Headers
linkTitle: Show Headers
weight: 38
description: Print response headers without the body.
---

Headers explain how the server wants the response handled: content type,
caching, pagination links, rate limits, and more. Use `--rsh-headers` when you
care about that metadata and do not need the body.

{{< restish-example >}}
restish https://api.rest.sh/ --rsh-headers
{{< /restish-example >}}

Filter one header as raw text:

{{< restish-example >}}
restish https://api.rest.sh/ -f headers.Content-Type -r
{{< /restish-example >}}

Use `/headers` when you want the request headers the server received:

{{< restish-example >}}
restish https://api.rest.sh/headers
{{< /restish-example >}}

The first two commands inspect response headers. The `/headers` fixture is
different: it echoes the request headers the server received, which is useful
for debugging profiles, auth, and custom `Accept` headers.

Related: [Output](/docs/guides/output/), [Global Flags](/docs/reference/global-flags/).
