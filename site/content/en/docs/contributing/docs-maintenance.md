---
title: Docs Maintenance
linkTitle: Docs Maintenance
weight: 30
description: Contributor checklist for user-facing documentation, validation steps, live examples, and migration audits.
---

Use this page when you are changing user-facing behavior and need to make the
docs stay honest.

This page is a maintenance workflow, not the source of truth for product
behavior. Current behavior comes from the CLI, tests, design records, and live
example API. When they disagree, update the docs and capture any remaining
follow-up in the project issue tracker or an active design record.

## Before You Edit

Check the nearby docs before changing one page in isolation:

- the section landing page
- related getting-started pages, guides, recipes, and reference pages
- the Tour of Restish when the change affects a major workflow, first-click
  onboarding, or browser-runnable examples
- plugin operator or author docs if the behavior touches plugins
- design records under `docs/design/` when architecture or invariants matter
- the v1 tag or source archive when older example coverage may have been
  better there
- the `rsh-docs` skill reference notes for documentation-site patterns and
  style inspiration

## Page Checklist

When a feature changes, check these documentation surfaces:

- getting-started impact
- guide impact
- recipe impact
- reference impact
- plugin/operator impact
- plugin/author impact
- troubleshooting impact
- design-record impact

Do not assume one page update is enough. Restish docs are intentionally layered.

For each user-facing page, verify:

- the page has one clear mode: tutorial, guide, recipe, reference, or
  troubleshooting
- the opening paragraph says what the page helps the reader do
- operational pages include at least one copyable command
- commands are paired with representative output when practical
- interactive examples use the default auto output unless the page is
  specifically teaching JSON, scripting, redirects, or exact machine-readable
  response shape
- the Tour of Restish keeps browser examples runnable where practical and
  clearly labels local-only setup, config, plugin, shell, and credential
  workflows
- examples omit flags that Restish handles by default; for example, image
  responses redirect as body bytes without an output flag
- prerequisites, auth, config, and private-infrastructure assumptions are
  explicit
- generic URL requests and API-aware generated commands are distinguished when
  that choice matters
- common failure notes sit near examples users are likely to break
- related pages send readers to the next useful place

## Validation Steps

Before sending a docs change:

1. build the site locally
2. check changed links with `scripts/check-doc-links.rb`
3. confirm examples are internally consistent
4. refresh or check generated reference regions with `restish-docgen`
5. compare any remaining hand-written command examples with current
   `restish --help` output
6. prefer `api.rest.sh` examples when a live endpoint makes the explanation
   stronger
7. grep changed docs for stale placeholders and stale source notes

Local build:

```bash
go run ./cmd/restish-docgen --check
hugo --source site --quiet
scripts/check-doc-links.rb
scripts/check-doc-examples.rb
```

Generated reference refresh:

```bash
go run ./cmd/restish-docgen --write
```

`restish-docgen` updates marked regions in reference pages from Cobra command
metadata, compiled built-in command plugin binaries, and the public plugin
protocol structs, config structs, and source-scanned environment variables.
Keep the prose around those generated regions curated by hand.

Example smoke tests:

```bash
scripts/check-doc-examples.rb
scripts/check-doc-examples.rb --mode live
```

The default example check is local and no-network. It validates every
`restish-example` shortcode as one shell-parseable Restish command. Live mode
builds a temporary CLI, connects the public example API in an isolated config,
installs the CSV formatter plugin into that temp config's plugin directory, and
runs the browser examples against `api.rest.sh`. Live mode skips intentionally
interactive edit examples and treats documented timeout/failure examples as
expected failures.

Useful stale-text checks:

```bash
rg 'api[.]example[.]com|your-api[.]example[.]com|auth[.]example[.]com|upload[.]example[.]com|Source material[:]' site/content/en/docs
```

## Example Validation Guidance

Prefer examples that can be:

- run against `api.rest.sh`
- exercised in local manual verification
- reflected in future CI or golden tests when the workflow is stable enough

Not every example can be fully live. When it cannot, keep placeholders clearly
generic and explain why.

Good live example candidates include:

- `/` and `/headers` for first requests and request inspection
- `/anything`, `/get`, `/post`, `/put`, `/patch`, `/delete`, `/head`, and
  `/options` for generic HTTP behavior
- `/auth/basic`, `/auth/bearer`, `/auth/api-key-header`, and
  `/auth/api-key-query` for safe auth examples
- `/login` and `/uploads` for form and multipart examples
- `/items` and `/items/{item-id}` for generic CRUD examples
- `/images`, `/images/{type}`, `/example`, `/types`, and `/books` for the core
  repeated docs workflows
- `/events` and `/logs` for streaming examples, with bounded commands such as
  `--rsh-max-items`
- `/flaky`, `/slow`, `/status/{code}`, `/cache`, `/cached/{seconds}`, and
  `/etag/{etag}` for retry, timeout, cache, and status examples
- `/formats/{format}`, `/problem`, `/gzip`, `/deflate`, `/brotli`, and
  `/image` for content negotiation and decoding examples

Use placeholders for:

- private hosts
- OAuth providers
- corporate proxies
- custom CA and mTLS infrastructure
- destructive workflows that cannot be safely reset

## Migration Audit

The old v1 docs archive is still useful. It contains strong example commands
and explanations that may be missing from the v2 site. When refreshing a page,
check the matching old topic from the v1 tag/archive and decide whether each
piece is:

- migrated into the current page
- superseded by v2 behavior
- intentionally retired
- still missing and tracked in the project issue tracker or an active design
  record

High-value old topics to keep auditing:

- bulk workflows
- OpenAPI CLI integration and extensions
- shorthand input and query syntax
- retries, timeouts, caching, and exit statuses
- pagination and hypermedia links
- output defaults, redirected body bytes, gron, images, and file downloads
- configuration and v1-to-v2 migration behavior

## Related Pages

- [Development Setup](../development-setup/)
- [Design Records](../design-records/)
- [Example API](/docs/reference/example-api/)
