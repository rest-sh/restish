# V2 Command Surface Decision

Status: accepted and implemented for the first Restish v2 release.

## Problem

Restish v2 is a successor to v1, not a new tool with no history. The command
surface should preserve the parts of v1 that made Restish useful: direct HTTP
requests, API-aware generated commands, shorthand input, good output defaults,
and shell-friendly diagnostics.

At the same time, v1 mixed several control-plane jobs under `api` because the
original tool grew organically. API registration, whole-config editing,
credential inspection, auth cache cleanup, content-type introspection, shell
integration, and plugin commands were all available, but the vocabulary did not
always match the user's task.

V2 should use the redesign window to keep the request model familiar while
making the control surface easier to explain, document, test, and extend.

## Goals

- Preserve the fast path: `restish https://example.com` and HTTP verb commands
  still work without setup.
- Keep registered APIs as top-level command groups so generated OpenAPI
  operations feel like native CLIs.
- Separate API registration, local configuration, auth state, cache state,
  shell integration, and plugins into predictable command families.
- Make long-running services and workflow extensions use explicit verbs.
- Keep global Restish request controls discoverable without overwhelming every
  generated command help page.
- Treat this document as the stable v2 design decision, not as a changelog.

## Non-Goals

- Do not force generated API calls under `restish api call`.
- Do not remove the `--rsh-*` namespace for Restish-owned request and output
  flags.
- Do not preserve every v1 command name when a new object/action shape is
  clearer before the first v2 release.
- Do not make this document a full command reference. User-facing reference
  pages own exact examples and help text.

## V1 Baseline

V1 exposed three important command categories.

Generic HTTP requests were immediate:

```text
restish <url>
restish get|head|options|post|put|patch|delete <url> [...]
```

API-aware commands were generated from registered OpenAPI descriptions:

```text
restish <api-name> <operation> [required args...] [body...] [operation flags]
restish <api-name> <tag> <operation> ...  # when operations are grouped
```

The control plane lived mostly under `api`, plus several top-level utilities:

```text
restish api show <name>
restish api edit
restish api sync <name>
restish api clear-auth-cache <name>
restish api content-types
restish api auth inspect <api-or-uri>
restish cert <uri>
restish edit <uri> [patch ...]
restish links <uri> [rel...]
restish completion <shell>
restish bulk ...
```

That baseline proved the product model: Restish is both a generic HTTP client
and an API-specific CLI generator. The weak point was vocabulary. Whole-config
editing looked like an API operation. Auth cache cleanup looked like API
registration cleanup. Runtime content-type introspection was nested below API
management. Shell integration and plugin processes did not have a consistent
object/action home.

## Product Principles

The v2 command tree follows these rules.

1. Request execution stays direct.
   Bare URLs and HTTP verbs are first-class commands because they are the
   daily path and the easiest way to try Restish.

2. Connected APIs become native command groups.
   A configured API name at the root should feel like installing a small
   purpose-built CLI for that API.

3. `api` manages API registrations and generated-command inputs.
   It owns connecting, listing, inspecting, syncing, removing, API-scoped
   settings, and API auth configuration.

4. `config` manages local Restish configuration.
   Whole-file editing, config path discovery, redacted config display,
   arbitrary config setting, and theme configuration belong here.

5. Auth state lives under `api auth`.
   Credentials and token cache recovery are part of API authentication, not
   general API registration management.

6. Runtime utilities are top-level when they describe Restish itself.
   `content-types`, `flags`, `doctor`, `version`, `cert`, `edit`, and `links`
   are not API registrations, so they should not be hidden under `api`.

7. Long-running plugin actions use explicit verbs.
   A command such as `mcp` should expose `serve` rather than doing long-running
   work from the object command itself.

## V2 Command Surface

The accepted v2 surface is:

```text
restish <url-or-api-short-name> [...]
restish get|head|options|post|put|patch|delete <url> [...]
restish <api> <operation> [...]

restish api
  connect <name> <url> [setup-expression ...]
  list
  inspect <name>
  set <name> <path:value>
  sync <name>
  remove <name>
  auth
    list <api>
    add <api> <credential-id>
    remove <api> <credential-id>
    inspect <api>
    clear-cache <api>

restish config
  path
  show [--json]
  edit
  set <path:value>
  theme set <source> [name]

restish cache
  info
  clear [api]

restish content-types

restish flags
  request
  output
  auth
  tls
  pagination
  cache
  general

restish plugin
  list
  install <source>
  remove <name>
  debug <name> [args...]

restish mcp
  serve <api...>

restish bulk ...
restish completion bash|zsh|fish|powershell|install
restish shell setup <shell>
restish doctor [api|plugin|migrate-v1]
restish cert <uri>
restish links <uri> [rel...]
restish edit <uri> [patch ...]
restish version
```

Generated APIs continue to add:

```text
restish <api>
  <operation> [required args...] [body...] [operation flags]
  <tag> <operation> ...  # when command_layout: tags
```

Shipped command plugins may add their own top-level commands, but their
subcommands should follow the same object/action rule. For example, `bulk`
keeps its established verbs because they describe a resource checkout workflow.

## Decisions

### API Connection Is The Setup Path

V2 uses:

```text
restish api connect <name> <url> [setup-expression ...]
```

This is the single API registration path. It normalizes and saves the base URL,
attempts spec discovery by default, applies OpenAPI-derived setup hints when a
spec is found, and still registers the API when discovery does not find a spec.
`--no-discover` performs minimal local registration with no spec probes.
`--spec <url-or-file>` uses an explicit OpenAPI source. `--replace` allows a
rerun to replace generated or default material that would otherwise be
preserved as user-owned local config.

V2 does not keep older API setup aliases. Multiple setup verbs would make
scripts, docs, help output, and support answers less clear before the first
stable v2 release.

### Configuration Is A First-Class Object

V2 adds `restish config` for configuration work:

```text
restish config path
restish config show [--json]
restish config edit
restish config set <path:value>
restish config theme set <source> [name]
```

This replaces the v1 habit of putting whole-config work under `api`. The
distinction matters because the v2 config file contains APIs, profiles,
plugins, output defaults, theme settings, and other local state. Users should
not have to learn that `api edit` opens the entire Restish config.

`api set` remains for API-scoped settings. `config set` is for arbitrary local
configuration. `config show --json` redacts sensitive values so it is safer to
use in bug reports and support conversations.

### API Inspection Uses `inspect`

V2 uses:

```text
restish api inspect <name>
```

The word `inspect` is already used for credential and diagnostic workflows. It
signals "show me the effective details for this object" without implying that
the output is a raw serialization of the stored config file.

### Auth Cache Recovery Lives With Auth

V2 uses:

```text
restish api auth clear-cache <api>
restish api auth clear-cache <api> --all-profiles
restish api auth clear-cache <api> --auth-profile <name>
```

OAuth token cache state is authentication state. Keeping cache recovery beside
`api auth list`, `api auth add`, and `api auth inspect` makes the workflow
findable from `restish api auth --help` and avoids overloading the top-level
`api` object with credential internals.

### Global Flag Discovery Gets Its Own Command

V2 adds:

```text
restish flags
restish flags request
restish flags output
restish flags auth
restish flags tls
restish flags pagination
restish flags cache
restish flags general
```

Global Restish flags are powerful, but generated operation help should stay
focused on API parameters. `restish flags` gives users a memorable in-CLI
reference for the full `--rsh-*` surface while allowing ordinary help output to
remain readable.

### MCP Uses An Explicit Service Verb

V2 uses:

```text
restish mcp serve <api...>
```

MCP starts a long-running protocol server. `serve` says that directly and
leaves room for future MCP-specific inspection or debugging commands without
changing the family shape.

### Shell Setup Belongs Under `shell`

V2 uses:

```text
restish shell setup <shell>
```

Shell setup installs integration such as the noglob alias and optional
completion. The top-level word `setup` sounds like whole-tool setup or API
setup, so v2 gives the workflow an explicit object.

`completion` remains the low-level command family for printing or installing
Cobra shell completions.

### Content Types Are A Runtime Utility

V2 uses:

```text
restish content-types
```

The content-type registry describes what the current Restish runtime can encode
or decode. It is not a property of a single API registration, so it is a
top-level utility command.

## Compatibility And Migration

Because these decisions land before the first stable v2 release, v2 does not
need to ship aliases for command names that were not selected for the stable
surface. The user-facing migration story should compare v1 and v2 directly.

Important v1-to-v2 command moves:

| V1 command or habit | V2 command |
| --- | --- |
| `api show <name>` | `api inspect <name>` |
| `api edit` | `config edit` |
| `api clear-auth-cache <name>` | `api auth clear-cache <name>` |
| `api content-types` | `content-types` |
| top-level shell/setup guidance | `shell setup <shell>` plus `completion ...` |
| direct plugin service command | explicit service verb such as `mcp serve` |

The request path is intentionally stable:

- bare URLs still perform GET
- HTTP verb commands remain at the root
- generated API commands remain at the root under the API name
- Restish-owned flags retain the `--rsh-*` namespace

## Rejected Alternatives

Do not move generated API calls under `restish api call`.
Generated commands are the product's differentiator. Connected APIs should feel
like native CLIs, not like records inside an API-management subsystem.

Do not keep every old command name as a documented alias.
Aliases are valuable after a stable release, but the first v2 surface should be
smaller, clearer, and easier to teach. Compatibility docs can map v1 habits to
the v2 names instead.

Do not make `flags` a docs-only page.
The full global flag surface changes with the binary. An in-CLI reference keeps
the source of truth close to the implementation and helps users who are working
offline or inside terminals.

Do not hide plugin services behind object commands.
Long-running processes should be obvious in scripts, editor configs, and MCP
client settings.

## Testing Plan

The command surface is product behavior and should be protected by tests.

- `internal/cli/command_surface_test.go` maps built-in command families and
  verifies removed or non-canonical names do not remain accidentally available.
- Config command tests cover `config path`, `config show`, JSON redaction, and
  `config set`.
- MCP tests cover `mcp serve` and reject running the service directly from
  `mcp <api...>`.
- Help tests should cover root, `api`, `api auth`, `config`, `flags`, `mcp`,
  and a generated API operation.
- Full CLI/plugin changes should pass `go test -tags=integration ./...` before
  release.

## Documentation Impact

User docs should present v2 commands directly, then include a migration guide
for v1 users. Pages that commonly mention these workflows include:

- quickstart and API connection guides
- command reference
- config and profile docs
- auth troubleshooting
- global flags reference
- shell setup and completion docs
- MCP/plugin operator docs
- upgrade-from-v1 guide

Docs should explain the chosen v2 model directly. Upgrading users need a
concise v1-to-v2 map.

## Outcome

The implemented v2 surface follows this decision. `api` owns API registration
and API-auth configuration, `config` owns local configuration, `flags` exposes
global Restish controls, `content-types` is a top-level runtime utility, MCP
uses `serve`, and shell integration lives under `shell setup`.

The resulting command tree keeps the v1 request experience familiar while
making the control plane more explicit and easier to document before v2 becomes
stable.
