---
title: Send an API Key in a Query Parameter
linkTitle: API Key Query
weight: 73
description: Call an endpoint that requires an api_key query parameter.
---

Some APIs require keys in query parameters, especially older services or simple
webhook-style endpoints. Prefer headers when the API supports them, because
query strings are more likely to appear in logs. Use this form when the API
contract requires it.

{{< restish-example >}}
restish -q api_key=docs-key https://api.rest.sh/auth/api-key-query
{{< /restish-example >}}

Quoted URL form:

{{< restish-example >}}
restish 'https://api.rest.sh/auth/api-key-query?api_key=docs-key'
{{< /restish-example >}}

The `-q` form is easier to compose with generated commands and avoids shell
quoting issues. If you write the query string directly, quote the URL so your
shell does not treat `?` or `&` specially. The [Requests guide](/docs/guides/requests/)
covers both styles.

Related: [Authentication](/docs/guides/authentication/), [Requests](/docs/guides/requests/).
