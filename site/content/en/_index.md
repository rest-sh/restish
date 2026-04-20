+++
title = "Restish v2"
linkTitle = "Docs"
+++

{{< blocks/cover title="Restish v2" image_anchor="top" color="dark" >}}
<div class="mx-auto">
  <p class="lead">A CLI for REST-ish HTTP APIs that starts as a better HTTP client and grows into an API-specific command line.</p>
  <p>Use generic HTTP commands for one-off calls, then connect to OpenAPI-described APIs for generated commands, shell completion, profiles, filtering, pagination, and plugins.</p>
  <div class="restish-home-actions">
  <a class="btn btn-lg btn-primary me-3 mb-4" href="/docs/getting-started/install/">
    Get Started
  </a>
  <a class="btn btn-lg btn-outline-light me-3 mb-4" href="/docs/getting-started/quickstart/">
    Quickstart
  </a>
  <a class="btn btn-lg btn-outline-light me-3 mb-4" href="/docs/getting-started/first-request/">
    First Request
  </a>
  <a class="btn btn-lg btn-outline-light mb-4" href="/docs/plugins/quickstart/">
    Plugin Quickstart
  </a>
  <a class="btn btn-lg btn-outline-light mb-4" href="/docs/getting-started/upgrade-from-v1/">
    Upgrade From v1
  </a>
  </div>
  <p class="restish-home-snippet"><code>restish https://api.rest.sh/</code></p>
</div>
{{< /blocks/cover >}}

{{< blocks/section color="white" >}}
<div class="row">
{{% blocks/feature icon="fa-solid fa-bolt" title="Fast for one-off calls" %}}
Make a request immediately with `get`, `post`, `put`, `patch`, and `delete`,
without registering an API first.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-diagram-project" title="Generated API commands" %}}
Point Restish at an OpenAPI-described API and it can generate discoverable,
shell-completed commands from the spec.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-wand-magic-sparkles" title="Built for daily workflows" %}}
Profiles, auth, shorthand input, filtering, pagination, retries, caching, and
plugins all fit one consistent request model.
{{% /blocks/feature %}}
</div>
{{< /blocks/section >}}

{{< blocks/section color="light" >}}
<div class="restish-card-grid">
  <a class="restish-card" href="/docs/getting-started/install/">
    <h2>Install in One Command</h2>
    <p>Install with Homebrew, mise, Nix, GitHub releases, or Go, then verify the binary in one minute.</p>
  </a>
  <a class="restish-card" href="/docs/getting-started/quickstart/">
    <h2>Ten-Minute Quickstart</h2>
    <p>Go from install to a generated API command, profile, and filtered output with one realistic flow.</p>
  </a>
  <a class="restish-card" href="/docs/getting-started/first-request/">
    <h2>First Request in Minutes</h2>
    <p>Use a generic HTTP command first, learn the bare minimum, then layer in headers, bodies, and output control.</p>
  </a>
  <a class="restish-card" href="/docs/getting-started/upgrade-from-v1/">
    <h2>Upgrade From v1</h2>
    <p>See the config migration path, deliberate behavior changes, fixed regressions, and the main command mapping in one place.</p>
  </a>
  <a class="restish-card" href="/docs/getting-started/connect-to-an-api/">
    <h2>Turn an API Into a CLI</h2>
    <p>Register an API, let Restish discover its OpenAPI document, and work from generated commands instead of full URLs.</p>
  </a>
  <a class="restish-card" href="/docs/guides/requests/">
    <h2>Daily Request Workflows</h2>
    <p>Learn generic verbs, API-aware commands, request bodies, filtering, pagination, retries, caching, and streaming.</p>
  </a>
  <a class="restish-card" href="/docs/guides/comparison/">
    <h2>Why Not Just curl?</h2>
    <p>See where Restish fits compared with curl and HTTPie, and why generated API commands change the daily workflow.</p>
  </a>
  <a class="restish-card" href="/docs/plugins/quickstart/">
    <h2>Build Plugins</h2>
    <p>Start with the smallest working plugin, then choose between hook, command, and TLS signer plugin types.</p>
  </a>
</div>
{{< /blocks/section >}}

{{% blocks/section color="white" %}}
## Start Here

1. [Install Restish](/docs/getting-started/install/)
2. [Run the Quickstart](/docs/getting-started/quickstart/)
3. [Make your first request](/docs/getting-started/first-request/)
4. [Set up your shell](/docs/getting-started/shell-setup/)
5. [Connect to an API](/docs/getting-started/connect-to-an-api/)
6. [Upgrade from v1](/docs/getting-started/upgrade-from-v1/)

## Why People Use Restish

- faster than hand-building every request with a lower-level HTTP client
- better day-to-day ergonomics once an API description is available
- one tool for ad hoc calls, repeatable profiles, and generated API commands
- plugin hooks for auth, formatting, loaders, workflow commands, and TLS signing

[Compare Restish with curl and HTTPie](/docs/guides/comparison/)

## Choose a Path

- New user:
  [Quickstart](/docs/getting-started/quickstart/)
- Daily API operator:
  [Requests](/docs/guides/requests/)
- API author:
  [Connect to an API](/docs/getting-started/connect-to-an-api/)
- Plugin author:
  [Plugin Quickstart](/docs/plugins/quickstart/)
- Existing v1 user:
  [Upgrade From v1](/docs/getting-started/upgrade-from-v1/)
{{% /blocks/section %}}
