+++
title = "Restish v2"
linkTitle = "Docs"
+++

{{< blocks/cover title="Restish v2" image_anchor="top" color="dark" >}}
<div class="mx-auto">
  <p class="lead">A modern CLI for REST-ish HTTP APIs, with docs built for real workflows.</p>
  <p>Install Restish, make a first successful request, connect to OpenAPI-described APIs, and extend the CLI with plugins.</p>
  <div class="restish-home-actions">
  <a class="btn btn-lg btn-primary me-3 mb-4" href="/docs/getting-started/install/">
    Get Started
  </a>
  <a class="btn btn-lg btn-outline-light mb-4" href="/docs/plugins/quickstart/">
    Plugin Quickstart
  </a>
  </div>
  <p class="restish-home-snippet"><code>restish get https://httpbin.org/json</code></p>
</div>
{{< /blocks/cover >}}

{{< blocks/section color="white" >}}
{{% blocks/feature icon="fa-solid fa-magnifying-glass" title="Searchable static docs" %}}
Static HTML with stable URLs and header anchors works better for users and
search engines than the old client-rendered site.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-list-check" title="Task-first structure" %}}
Guides, recipes, reference, and plugin docs are separated so people can find
the right level of detail quickly.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-puzzle-piece" title="Plugin-first product story" %}}
Plugins are a core part of Restish v2, and the docs treat them like a
first-class capability instead of a buried appendix.
{{% /blocks/feature %}}
{{< /blocks/section >}}

{{< blocks/section color="light" >}}
<div class="restish-card-grid">
  <a class="restish-card" href="/docs/getting-started/first-request/">
    <h2>First Request in Minutes</h2>
    <p>Install the CLI, configure your shell, and make a successful HTTP request without learning the whole product first.</p>
  </a>
  <a class="restish-card" href="/docs/guides/requests/">
    <h2>Daily Request Workflows</h2>
    <p>Learn generic verbs, API-aware commands, request bodies, filtering, pagination, retries, caching, and streaming.</p>
  </a>
  <a class="restish-card" href="/docs/plugins/quickstart/">
    <h2>Build Plugins</h2>
    <p>Start with the smallest working plugin, then choose between hook, command, and TLS signer plugin types.</p>
  </a>
</div>
{{< /blocks/section >}}
