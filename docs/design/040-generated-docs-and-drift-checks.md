# Generated Docs And Drift Checks

Status: Accepted

## Problem

Restish reference pages describe command flags, subcommands, plugin manifest
fields, and host/plugin protocol messages that are already defined in code.
Manual tables are useful to read, but they drift when command help, Cobra
flags, plugin messages, or manifest structs change.

The docs site should keep narrative pages human-written while making factual
reference material mechanically refreshed from the codebase.

## Goals

- Generate factual reference fragments from the current Restish source.
- Keep existing guide, recipe, and explanatory reference prose editable by
  humans.
- Fail CI when checked-in generated fragments are stale.
- Use the public command/plugin boundaries as much as practical, especially
  for command plugins.
- Keep generation deterministic and independent of a developer's local config
  or installed plugins.

## Non-Goals

- Generate full guides, recipes, troubleshooting pages, or product narrative.
- Replace command help as the primary in-terminal reference.
- Validate every docs example in the first implementation.
- Generate registered API command docs from a live user config.

## Proposed Design

Add a checked-in maintainer binary at `cmd/restish-docgen`. It supports:

- `--write`: update generated regions in checked-in Markdown files.
- `--check`: render the same regions and fail if any target file would change.

Generated regions live inline in the existing docs pages using markers:

```html
<!-- BEGIN GENERATED: restish-docgen <region> -->
...
<!-- END GENERATED -->
```

Inline regions keep the Hugo setup simple and make generated/reference context
visible in normal Markdown review. If inline regions become too noisy, a later
change can move generated fragments behind Hugo includes without changing the
source-of-truth model.

## Command Reference Generation

The generator constructs the real Cobra command tree in-process through a
docs-only CLI helper. It loads an empty config and isolates plugin discovery so
local user state cannot affect generated docs.

For built-in command plugins, generation uses compiled plugin binaries. The
generator builds known command plugins into a temporary plugin directory, then
discovers them through the same manifest and command-discovery startup flags as
the host. This treats the executable and plugin protocol as the source of truth
instead of private implementation constructors.

Generated command fragments include:

- command path and usage
- short and long help
- aliases
- examples
- subcommands
- local flags
- global persistent flags

Full Cobra `Long` text is included in generated regions so the published site
is useful to search engines and AI agents that browse the docs.

## Plugin Protocol Schema Generation

The generator parses `plugin/manifest.go` and `plugin/messages.go` with Go's
AST packages. Generated fragments include:

- manifest and command-discovery structs
- message type constants
- command plugin message structs
- hook plugin message structs
- TLS signer message structs
- field names, CBOR/JSON tags, Go types, required/optional status, and field
  comments

Comments in these source files therefore become part of the plugin author
contract. Implementation-only details should stay out of those comments or be
phrased so they are safe to publish.

## Run Points

Local developer loop after changing command, flag, plugin protocol, config, or
environment behavior:

```bash
go run ./cmd/restish-docgen --write
hugo --source site --quiet
scripts/check-doc-links.rb
```

PR CI:

```bash
go run ./cmd/restish-docgen --check
hugo --source site --quiet
scripts/check-doc-links.rb
```

Docs deploy and release packaging should also run `--check` so a release tag
cannot publish stale generated reference material.

## Failure Modes

- If plugin binaries fail to build, doc generation fails.
- If a plugin manifest or command discovery response is invalid, doc generation
  fails.
- If a generated region marker is missing, `--write` and `--check` fail rather
  than appending unknown content.
- If checked-in generated docs drift, `--check` prints the stale file paths and
  exits non-zero.

## Documentation Impact

Update contributor docs to mention `restish-docgen` in the docs maintenance and
development setup workflows. Reference pages keep curated explanations and
examples around generated sections.
