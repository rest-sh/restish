---
title: "Turn an OpenAPI Spec Into a CLI Without Generating Code"
linkTitle: "Turn an OpenAPI Spec Into a CLI Without Generating Code"
date: 2026-06-08
author: "Daniel Taylor"
description: OpenAPI can be more than SDK input or Swagger UI data. Restish loads API descriptions at runtime and turns them into shell-native commands with profiles, auth, output, pagination, and MCP-ready extension points.
canonical_url: "https://rest.sh/blog/turn-an-openapi-spec-into-a-cli-without-generating-code/"
categories:
  - OpenAPI
tags:
  - openapi
  - cli
  - rest
  - devtools
---

OpenAPI is usually treated as an input to something else. Generate an SDK. Render
Swagger UI. Publish reference docs. Feed a contract test. All of those are good
jobs for a spec.

But there is another useful job hiding in the same document: teach the terminal
how to work with the API.

That does not have to mean generating a new repository, choosing a language
runtime, publishing a package, and keeping a client binary in sync with every
operation change. A CLI can load an OpenAPI description at runtime, cache what it
needs, and turn repeated API work into commands without making users rebuild a
client.

That is the core bet behind Restish. It can still make a one-off HTTP request,
but once an API publishes OpenAPI, Restish can turn that API into a small
shell-native command group.

<div class="restish-blog-callout">
  <strong>Try it as you read.</strong> Runnable examples below use the browser
  preview against the public <code>api.rest.sh</code> API. Local setup commands
  are shown as fenced shell snippets because they write Restish config on your
  machine.
</div>

## Start With Plain HTTP

A good API CLI still needs the universal escape hatch: send a request to a URL.
Restish does not require setup before it is useful.

{{< restish-example >}}
restish api.rest.sh/auth/bearer -H 'Authorization: Bearer docs-token'
{{< /restish-example >}}

That path matters because API work often starts messy. You paste a URL from a
bug report, check a response header, inspect an error body, or try one endpoint
before you know whether the API deserves local setup.

That is useful, but it is not the whole job. The moment you call the same API
repeatedly, the URL, auth, parameters, output shape, and pagination rules start
becoming a little client you keep reconstructing by hand.

OpenAPI already knows a lot of that.

## Register The API Once

When an API publishes an OpenAPI document, connect it to Restish:

```bash
restish api connect example api.rest.sh
restish example --help
```

Restish discovers the spec, stores the API registration, and caches the
description. The API name becomes a command group. After that, generated
operations are available under `restish example ...`.

The browser preview used in these docs has a built-in `example` mapping, so you
can try the generated-command shape without writing local config:

{{< restish-example >}}
restish example get-auth-bearer
{{< /restish-example >}}

That command is not a wrapper around a handwritten `curl` string. It comes from
the API description. Restish knows the operation, parameters, path, response
shape, and profile context, then runs the request through the same pipeline used
by ordinary URL requests.

## What The Spec Becomes

OpenAPI has many fields that look like documentation metadata until a CLI starts
using them as interface design.

`operationId` becomes a command name:

```yaml
paths:
  /items/{item-id}:
    get:
      operationId: getItem
      summary: Get one item
```

Restish turns that into:

```bash
restish myapi get-item alpha
```

Path parameters become positional arguments. Optional query, header, and cookie
parameters become flags. Summaries and descriptions become help text. Schemas
describe request bodies and parameter types. Servers influence where operations
are sent. Security requirements tell Restish which credentials an operation can
use.

Here is the anatomy of a generated command:

<div class="restish-anatomy">
  <pre class="restish-anatomy__command"><code><span class="anatomy-bin">restish</span> <span class="anatomy-api">inventory</span> <span class="anatomy-op">update-item</span> <span class="anatomy-option">--dry-run</span> <span class="anatomy-arg">item-123</span> <span class="anatomy-body">'name: Travel mug, enabled: true'</span></code></pre>
  <div class="restish-anatomy__grid">
    <div class="restish-anatomy__item anatomy-bin">
      <span class="restish-anatomy__label">Binary</span>
      <code>restish</code>
      <span class="restish-anatomy__from">Restish itself</span>
      <span class="restish-anatomy__note">The universal entry point.</span>
    </div>
    <div class="restish-anatomy__item anatomy-api">
      <span class="restish-anatomy__label">API</span>
      <code>inventory</code>
      <span class="restish-anatomy__from">Chosen during <code>api connect</code></span>
      <span class="restish-anatomy__note">Local operator vocabulary, not an OpenAPI field.</span>
    </div>
    <div class="restish-anatomy__item anatomy-op">
      <span class="restish-anatomy__label">Operation</span>
      <code>update-item</code>
      <span class="restish-anatomy__from"><code>operationId: updateItem</code></span>
      <span class="restish-anatomy__note">The operation ID becomes the command name.</span>
    </div>
    <div class="restish-anatomy__item anatomy-option">
      <span class="restish-anatomy__label">Option</span>
      <code>--dry-run</code>
      <span class="restish-anatomy__from">Optional query parameter <code>dry_run</code></span>
      <span class="restish-anatomy__note">Optional parameters become flags.</span>
    </div>
    <div class="restish-anatomy__item anatomy-arg">
      <span class="restish-anatomy__label">Argument</span>
      <code>item-123</code>
      <span class="restish-anatomy__from">Required path parameter <code>item-id</code></span>
      <span class="restish-anatomy__note">Required path parameters become positional arguments.</span>
    </div>
    <div class="restish-anatomy__item anatomy-body">
      <span class="restish-anatomy__label">Body</span>
      <code>'name: Travel mug, enabled: true'</code>
      <span class="restish-anatomy__from">JSON request body schema</span>
      <span class="restish-anatomy__note">Body input can come from shorthand, stdin, or a file.</span>
    </div>
  </div>
</div>

The corresponding OpenAPI operation might look like this:

```yaml
paths:
  /items/{item-id}:
    patch:
      operationId: updateItem
      summary: Update one item
      parameters:
        - name: item-id
          in: path
          required: true
          schema:
            type: string
        - name: dry_run
          in: query
          schema:
            type: boolean
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                enabled:
                  type: boolean
```

That means spec quality becomes CLI quality. Stable operation IDs, clear
parameter names, accurate schemas, and honest auth metadata are not only good
for docs. They make the generated command easier to discover, complete, and run.

When a spec needs CLI-specific polish, Restish supports targeted extensions:

```yaml
x-cli-name: list-items
x-cli-aliases: [items]
x-cli-description: List items with optional filtering.
```

Those extensions should stay small. OpenAPI remains the source of truth for the
API. Restish-specific hints are for command vocabulary, hiding confusing
operations, auth setup hints, and other terminal UX details that the base spec
cannot express cleanly.

## Why Runtime Helps

Generated SDKs make sense inside applications. A service written in Go,
TypeScript, Python, or Java usually wants typed calls, versioned dependencies,
and application-local error handling. That is a different job.

A terminal workflow has different constraints:

- users want the newest operation without waiting for a released client
- commands need to compose with files, pipes, `jq`, `grep`, and CI scripts
- auth and environment selection belong in local profiles
- output has to be helpful for humans and boring for scripts
- debugging often starts before anyone knows which language a real integration
  will use

Runtime generation fits that shape. You connect an API once, then sync the spec
when it changes:

```bash
restish api sync example
restish example --help
```

There is still caching. Restish should not fetch and parse a large spec for
every shell completion or every request. But the cached artifact is local
operational state, not a generated client project that must be reviewed,
published, installed, and eventually forgotten.

This also keeps API examples honest. Documentation can say:

```bash
restish example get-image jpeg > dragonfly.jpg
```

That example is both readable to a person and connected to the live API shape
after sync. If the API publishes a better command name, updated schema, or new
auth metadata, the CLI can pick it up from the spec instead of waiting for a new
handwritten wrapper.

## Output Is A Contract

Turning OpenAPI into commands only helps if the result behaves like a good CLI.
The generated command should not dump surprising diagnostics into a script, add
color to piped output, or corrupt a downloaded file because the tool tried to be
helpful.

Restish v2 treats output as a product contract:

- `stdout` carries the selected response data
- `stderr` carries diagnostics, warnings, progress, and verbose traces
- terminal output can be readable by default
- redirected unfiltered responses preserve body bytes
- explicit filters and formats produce structured output for the next program

For example, ask for one field from every paginated item:

{{< restish-example >}}
restish example list-images -f body.self
{{< /restish-example >}}

The same rule applies to generated OpenAPI commands and plain URL requests. The
API-aware layer should add names, help, auth, and request shaping. It should not
break the shell contract underneath.

## Auth Belongs With Profiles

OpenAPI command generation becomes much more useful when credentials are not
pasted into every command.

Restish keeps repeated auth in profiles. A profile can hold API keys, bearer
tokens, OAuth configuration, mTLS settings, external tool auth, base URLs,
headers, query defaults, and OpenAPI credential bindings.

The practical difference is that this:

```bash
restish github list-issues --state open
```

can carry the right base URL, token source, TLS config, and operation-specific
credential choice without turning every docs example into a wall of headers.

OpenAPI security matters here. Some operations are public. Some allow several
auth alternatives. Some require a credential that should only be used for a
subset of operations. Restish v2 treats operation security as request execution
metadata, not as one global header pasted onto every generated call.

That is where a runtime CLI can feel more like a local tool than a copied HTTP
example. Profiles remember the environment. The spec explains the operation.
The command stays small enough to type.

## The Same Source Can Feed Agents

OpenAPI-to-CLI is also a useful bridge to agent tools.

For humans, Restish turns OpenAPI operations into terminal commands. For MCP
clients, the `restish-mcp` plugin can expose registered operations as tools:

```bash
restish mcp serve example
```

That does not mean every endpoint should be exposed to every model, user, or
profile. Restish keeps MCP conservative by default: read-oriented tools are
shown first, write tools require explicit opt-in, and operations can be
allowlisted or hidden.

The important part is that both interfaces use the same source of truth. The
OpenAPI document describes the API. Restish profiles carry local auth and
environment choices. The request pipeline owns TLS, retries, timeouts,
normalization, filtering, and output behavior.

Humans use the CLI. Agents use MCP. The API should not need a separate pile of
handwritten glue for each one.

There is also a lighter-weight path when an agent is calling Restish through an
ordinary command instead of MCP: render the response as TOON.
[TOON](https://github.com/toon-format/spec) is a token-dense text encoding for
JSON-shaped data, and Restish now supports it with `-o toon`:

{{< restish-example >}}
restish api.rest.sh/images -f 'body.{name, format}' -o toon
{{< /restish-example >}}

That is not a replacement for filtering. The biggest savings still come from
projecting the response down to the records and fields the agent needs, then
using TOON when the remaining shape is a uniform list. Restish treats TOON as
output-only, so JSON remains the format to use for request bodies and for
workflows where the consumer expects standard JSON. The
[output formats reference](/docs/reference/output-formats/#toon-for-agents)
covers the tradeoffs.

## CLI-Friendly Specs

If you publish an API and want it to work well in tools like Restish, start with
the ordinary OpenAPI basics:

- publish the spec at a predictable URL such as `/openapi.json`
- use stable, human-readable `operationId` values
- write summaries that make sense as command help
- give parameters clear names and accurate schemas
- describe request and response bodies honestly
- model auth per operation, including public operations
- provide examples for tricky request shapes
- expose pagination links or enough metadata for clients to proceed safely

Then test the terminal shape:

```bash
restish api connect myapi https://api.example.test
restish myapi --help
restish doctor api myapi
```

If the generated commands are awkward, the fix is often useful beyond Restish.
Better operation IDs improve docs anchors. Better schemas improve SDKs. Better
security metadata improves Swagger UI, contract tests, and agent tools.

That is the quiet advantage of OpenAPI as a shared interface contract. A spec
that can teach a terminal usually teaches other tools better too.

## Try It Locally

Install Restish:

```bash
brew install restish
restish --version
```

Or with Go:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

Connect the public example API:

```bash
restish api connect example api.rest.sh
restish example --help
restish example list-images
```

Useful next stops:

- [Connect to an API](/docs/getting-started/connect-to-an-api/) covers setup, explicit spec URLs, sync, and project config.
- [OpenAPI Reference](/docs/reference/openapi-cli-integration/) explains command naming, parameters, extensions, and generated help.
- [Output Guide](/docs/guides/output/) covers formats, filters, redirects, and script-friendly output.
- [Authentication Guide](/docs/guides/authentication/) covers profiles and repeated credentials.
- [Serve APIs Over MCP](/docs/plugins/mcp/) shows how registered APIs become MCP tools.

OpenAPI is not only for generated SDKs and browser docs. If the spec already
knows the API, the terminal should be able to learn from it too.
