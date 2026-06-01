---
title: "Restish v2: A CLI for REST APIs, Rebuilt"
linkTitle: "Restish v2: A CLI for REST APIs, Rebuilt"
date: 2026-05-30
author: "Daniel Taylor"
description: Restish v2 keeps the fast one-off HTTP workflow, then turns OpenAPI-described APIs into shell-native commands with profiles, auth, output filtering, pagination, plugins, and MCP.
canonical_url: "https://rest.sh/blog/restish-v2-a-cli-for-rest-apis-rebuilt/"
categories:
  - Releases
tags:
  - cli
  - openapi
  - rest
  - devtools
---

Most APIs already know more about themselves than the command line does. They
have schemas, auth rules, pagination links, content types, examples, and often a
full OpenAPI description. Then we copy a `curl` command into a terminal and
teach all of that context back to the shell one flag at a time.

`curl` is still the universal primitive. Restish is for the moment after that:
when a URL becomes a workflow, and the API should start feeling like a small
native CLI instead of a string you keep rebuilding.

Restish v1 proved the shape of the product: start with direct HTTP requests,
then let OpenAPI turn repeated work into an API-specific CLI. V2 keeps that
idea, but rebuilds the foundation around it: one config model, clearer command
families, stricter output contracts, safer auth behavior, and a real plugin
boundary.

<div class="restish-blog-callout">
  <strong>Try it as you read.</strong> The runnable examples below use the same
  browser preview as the docs. They make live requests to the public
  <code>api.rest.sh</code> API, and the same commands work locally after you
  install Restish.
</div>

## Why a V2?

V2 exists because the original design worked well enough to outgrow itself.
Restish had picked up direct requests, generated OpenAPI commands, shorthand
input, auth helpers, pagination, output formatting, config editing, and plugin
workflows over time. The result was useful, but some of the product vocabulary
and internal boundaries were carrying too much history.

Streaming was one pressure point. Server-Sent Events, NDJSON, paginated
collections, filters, and ordinary response bodies all want different output
shapes, but they still have to compose with the same `stdout`, `stderr`,
redirect, timeout, and formatting rules. V2 gave Restish a stricter output
model so live streams can emit records incrementally while scripts still get
predictable output.

Configuration was another. V1 split API registrations and global settings
across multiple files, and the old interactive API setup tried to discover too
much through a prompt flow that was hard to maintain and easy to get stuck in.
V2 centers persistent state in one JSONC `restish.json`, makes config editing
explicit with `config edit`, and makes API setup a repeatable command:
`restish api connect <name> <url>`.

OpenAPI auth also needed a more honest model. Many APIs do not have one global
credential. One operation may be public, another may require an API key, and a
third may accept either OAuth or a partner key. V2 lets a profile hold multiple
credential bindings, then selects credentials per operation from the OpenAPI
security policy instead of pasting one profile-level auth value onto every
generated request.

Finally, v2 needed a real extension boundary. V1 had an external auth command
hook, but not a general plugin system. V2 adds plugin discovery, manifests,
hook protocols, command plugins, formatter plugins, TLS signer plugins, and
host-delegated request execution around one shared request pipeline. A plugin
can extend Restish without reimplementing auth, TLS, retries, pagination,
streaming, filtering, or output rules on its own.

Those changes are why v2 has a few intentional breaks. Config moved from
`apis.json` and `config.json` into `restish.json`; `api edit` became
`config edit`; `api clear-auth-cache` became `api auth logout`; auth inspection
became API/profile-aware; and some output defaults became stricter so redirects,
streaming, and scripts behave predictably. The
[upgrade guide](/docs/getting-started/upgrade-from-v1/) covers the command
mapping, but the theme is simple: preserve the daily request workflow, and
clean up the foundation underneath it.

So v2 is not a pivot away from v1. It is the same bet with sharper edges:
direct requests stay fast, connected APIs feel native, and the Restish-owned
control plane becomes easier to explain, test, and extend.

## Start With a URL

Restish does not require setup for the first request. Give it a URL and it sends
a request, decodes the response, and renders something useful for the terminal.

{{< restish-example >}}
restish api.rest.sh/types
{{< /restish-example >}}

That fast path matters. A tool that only works after setup is not the tool you
reach for while debugging a response, checking a new endpoint, or pasting one
command into a chat thread.

Restish keeps the obvious HTTP verbs too:

{{< restish-example >}}
restish post api.rest.sh/body 'name: Ada, tags: [cli, openapi], active: true'
{{< /restish-example >}}

The body there is Restish shorthand: a shell-friendly way to send structured
data without stopping to quote and escape JSON by hand. Raw JSON, forms,
multipart uploads, files, and stdin are still there when you need them.

## Then Let the API Become a CLI

The bigger idea is that a useful API already describes itself. If an API
publishes OpenAPI, Restish can turn that description into commands at runtime.

Locally, that starts like this:

```bash
restish api connect example api.rest.sh
restish example --help
```

After that, operations from the API become commands under the API name. The
browser preview uses a built-in `example` API mapping so you can try the same
shape without writing local config:

{{< restish-example >}}
restish example list-images -o table
{{< /restish-example >}}

That command shape is intentional. Direct HTTP requests stay direct. Connected
APIs become native command groups. A generated API call does not live under
`restish api call ...` because calling the API is the daily workflow. The `api`
command is for managing registrations, not for hiding the API behind one more
noun.

## Output Is a Product Feature

API tools tend to fail in two different ways. Some are friendly in a terminal
but awkward in scripts. Others are scriptable, but make humans stare at raw
payloads until the coffee wears off.

Restish treats output as part of the contract:

- stdout carries the selected response data.
- stderr carries diagnostics, warnings, progress, and verbose traces.
- interactive terminals get readable output by default.
- redirected unfiltered responses preserve body bytes.
- filters and explicit formats produce structured output for the next command.

For example, if you want one field from every item, ask for one field from every
item:

{{< restish-example >}}
restish api.rest.sh/books -f body.url -o lines
{{< /restish-example >}}

The v2 rule for files is intentionally boring: if stdout is redirected and you
did not ask Restish to filter or format the response, Restish writes the body
bytes. That keeps image downloads, archives, CBOR, YAML, and other payloads from
being "helpfully" rewritten when what you needed was the file. When you do ask
for `-o json`, `-o table`, `-o ndjson`, `-o lines`, or another format, Restish
renders the selected value for the next program.

## Pagination Shouldn't Be Copy-Pasted

A lot of API scripts grow the same little loop: fetch a page, extract `next`,
fetch another page, stop eventually, and hope the next link does not wander
somewhere surprising.

Restish follows recognized `next` links for collection responses by default,
with limits. It also keeps same-origin pagination conservative unless an API has
explicitly opted into a broader discovery path.

{{< restish-example >}}
restish example list-images --per-page=1 --rsh-max-items 3 -f body.self -o lines
{{< /restish-example >}}

The goal is not to hide HTTP. The goal is to remove the repeated glue code while
still making limits, warnings, and failure cases visible.

## Auth Belongs in Profiles, Not Shell History

Real APIs need credentials, environments, and sometimes several auth schemes for
different operations. Repeating those details in every command is both tedious
and risky.

Restish v2 keeps auth in profiles. A profile can hold headers, query params,
base URLs, API keys, bearer tokens, OAuth settings, mTLS settings, external tool
auth, and OpenAPI-derived credential bindings.

For quick debugging, you can still send a header directly:

{{< restish-example >}}
restish -H 'Authorization: Bearer docs-token' api.rest.sh/auth/bearer
{{< /restish-example >}}

For real work, put the value behind an environment reference or an OAuth flow.
Restish can inspect computed auth material, redact sensitive values for support
logs, and clear cached OAuth tokens with `restish api auth logout`.

OpenAPI operation security also matters. Public operations should not inherit
credentials by accident. Operations with several allowed auth alternatives need
a way to choose which credential to use. V2 makes that part of the generated
command behavior instead of treating auth as one global header pasted onto every
request.

## V2 Cleaned Up the Restish Side

Restish v1 grew organically, which meant several local management tasks ended
up under `api`. V2 keeps the request path familiar and gives the control plane
clearer nouns:

```text
restish api      # connect, inspect, sync, remove, and configure APIs
restish config   # show, edit, and update local Restish config
restish cache    # inspect and clear response cache state
restish plugin   # install, list, remove, and debug plugins
restish shell    # setup and completion
restish doctor   # local diagnostics
```

That sounds small, but command vocabulary is product design. If `config edit`
opens the whole Restish config, the command should say `config`. If an OAuth
token cache needs to be cleared, that belongs with `api auth`. If MCP starts a
long-running server, the command should say `serve`.

## Plugins Keep the Core Focused

Restish v2 has a plugin system because not every useful API workflow belongs in
the main binary.

Hooks can add output formats, auth behavior, loaders, and request or response
middleware. Command plugins can provide larger workflows while delegating HTTP
execution back to Restish, so they still get the normal profiles, auth, TLS,
retries, output formatting, and diagnostics. TLS signer plugins can handle
client-certificate signing when private keys must stay in hardware or another
local system.

The first-party plugins are examples of that split:

- `restish-csv` adds CSV output.
- `restish-bulk` keeps a multi-step resource checkout workflow out of core.
- `restish-pkcs11` signs mTLS handshakes with PKCS#11-backed keys.
- `restish-mcp` exposes registered APIs as MCP tools.

The `plugin debug` command is there because plugin protocols should not require
guessing what happened on stdio.

## OpenAPI to MCP Is Now One Command

MCP is a natural extension of the same idea behind Restish: an API description
can become an interface for another kind of user.

For humans, Restish turns OpenAPI operations into CLI commands. For agents,
`restish-mcp` can expose those operations as MCP tools:

```bash
restish mcp serve example
```

The important part is not only "OpenAPI becomes tools." The important part is
that those tools run through the same Restish request pipeline: profiles, auth,
TLS, retries, timeouts, and output normalization.

V2 also keeps the default conservative. MCP exposes read-oriented tools by
default. Write operations require an explicit opt-in, and you can allowlist
operations when a client should only see a subset of the API.

## Where Restish Fits

Restish is not trying to replace every API tool.

Use `curl` when you need the universal primitive. Use Postman when you want a
collaborative GUI workspace. Use generated SDKs inside applications. Use
Swagger UI when a browser is the right place to explore a spec.

Restish is for the space between those:

- you live in the terminal
- you work with REST-ish APIs repeatedly
- your API publishes OpenAPI, or at least behaves consistently
- you want commands that compose with shell tools
- you want auth, profiles, output, pagination, retries, and plugins to be
  remembered by the tool instead of pasted into every command

That space is bigger than it looks. It includes local development, API support,
CI jobs, internal platform tools, incident debugging, docs examples, and now
agent tool servers.

## Try It Locally

Install the v2 release line with Homebrew:

```bash
brew install restish
restish --version
```

Or with Go:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

Then make one request:

```bash
restish api.rest.sh/types
```

If the first request works, connect the example API and look at the generated
commands:

```bash
restish api connect example api.rest.sh
restish example --help
restish example list-images -o table --rsh-columns name,format,self
```

Useful next stops:

- [Tour of Restish](/docs/getting-started/tour/) runs more examples in the browser.
- [Install](/docs/getting-started/install/) covers package managers and setup.
- [Connect to an API](/docs/getting-started/connect-to-an-api/) turns an OpenAPI API into commands.
- [Scripting and Automation](/docs/guides/automation/) covers stdout, stderr, exit codes, retries, and pagination.
- [Serve APIs Over MCP](/docs/plugins/mcp/) explains the MCP plugin.

Restish v2 is a redesign, but not a reinvention. The core promise is the same:
start with a URL, then let the API teach your terminal how to work with it.
