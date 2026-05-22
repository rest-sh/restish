# V2 Command Surface Decision

Status: accepted for the first Restish v2 release.

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
  flags on generated API commands and generic HTTP commands.
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
restish edit <uri> [patch ...] [--no-editor]
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
   `doctor`, `version`, `cert`, `edit`, and `links` are not API registrations,
   so they should not be hidden under `api`. Rarely used runtime inventory,
   such as the content-type registry, belongs in `doctor` rather than owning a
   top-level command word.

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
  set <name> <shorthand-patch> [patch...]
  sync <name>
  remove <name>
  auth
    list <api>
    add <api> <credential-id>
    remove <api> <credential-id>
    inspect <api>
    logout [api]

restish config
  path
  show [-o json]
  edit
  set <shorthand-patch> [patch...]
  theme set <source> [name]

restish cache
  info
  clear [api]

restish plugin
  list
  install <source>
  remove <name>
  debug <name> [args...]

restish mcp
  serve <api...>

restish bulk ...
restish shell completion bash|zsh|fish|powershell
restish shell setup <shell>
restish doctor [api|plugin]
restish cert <uri>
restish links <uri> [rel...]
restish edit <uri> [patch ...] [--no-editor]
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
rerun to replace existing profiles with generated or default profile material.
Without `--replace`, existing profiles are preserved because they may contain
credential material that cannot be recovered from the API later. API-level
fields such as `base_url`, `spec_url`, `allow_cross_origin_spec`,
`operation_base`, pagination, server variables, retry settings, and
`allowed_operation_origins` are refreshed from the new connect run or left to
explicit `api set`/`config edit` changes; they are not covered by the profile
preservation rule.

`api sync <name>` refreshes the cached OpenAPI source and may persist
spec-derived API metadata that changed after registration, such as a newly
discovered `spec_url` or operation-server origins. It does not overwrite
profiles or apply new `x-cli-config` profile defaults; profiles only change via
explicit profile/auth commands, `api set`, config editing, or reconnecting with
`--replace`.

V2 does not keep older API setup aliases. Multiple setup verbs would make
scripts, docs, help output, and support answers less clear before the first
stable v2 release.

### Configuration Is A First-Class Object

V2 adds `restish config` for configuration work:

```text
restish config path
restish config show [-o json]
restish config edit
restish config set <shorthand-patch> [patch...]
restish config theme set <source> [name]
```

This replaces the v1 habit of putting whole-config work under `api`. The
distinction matters because the v2 config file contains APIs, profiles,
plugins, output defaults, theme settings, and other local state. Users should
not have to learn that `api edit` opens the entire Restish config.

`api set` remains for API-scoped settings and accepts the same shorthand patch
language rooted at `apis.<name>`. `config set` is for arbitrary local
configuration. The unreleased pre-v2 `set key value` form is not part of the
stable v2 contract. `config show -o json` redacts sensitive values so it is safer
to use in bug reports and support conversations.

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
restish api auth logout <api>
restish api auth logout <api> --all-profiles
restish api auth logout --auth-profile <name>
```

OAuth token cache state is authentication state. Keeping cache recovery beside
`api auth inspect`, `api auth add`, and `api auth remove` makes the workflow
findable from `restish api auth --help` and avoids overloading the top-level
`api` object with credential internals. `api auth inspect` is the canonical
readiness and inspection surface; v2 should not keep a separate `api auth list`
command or alias.

The API argument is omitted only for `--auth-profile`, because shared auth
profiles are not owned by a single API. `--all-profiles` still requires an API
name and clears the API-prefixed token entries plus shared auth-profile entries
referenced by that API's profiles.

### Global Flag Discovery Lives In Help

V2 uses focused command help by default and expands the full global flag
surface with:

```text
restish <command> --help-all
```

Global Restish flags are powerful, but generated operation help should stay
focused on API parameters. Ordinary help shows common global flags and points
to `--help-all` for the grouped request, output, auth, TLS, pagination, cache,
and general reference.

There is no public `restish flags` command in v2. `--help-all` is the canonical
in-CLI global flag reference because it stays attached to the command context
where those flags apply. If a development build still contains a `flags`
command, it is not part of the stable v2 surface and should be removed before
release rather than documented as an alias.

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

`restish shell completion <shell>` is the canonical v2 command for printing
Cobra shell completion scripts. The historical top-level `completion` command
may remain as a hidden compatibility alias, but it is not part of the published
v2 command surface.

### Content Types Are Doctor Inventory

V2 uses:

```text
restish doctor
restish doctor -o json
```

The content-type registry describes what the current Restish runtime can encode
or decode. It is not a property of a single API registration, and day-to-day
users rarely need a standalone command for it. The root doctor report includes
a human `Content types:` line, and JSON doctor output includes the detailed
names, MIME types, suffixes, and quality values for support/debugging use.

## Compatibility And Migration

Because these decisions land before the first stable v2 release, v2 does not
need to ship public aliases for command names that were not selected for the
stable surface. Hidden bootstrap or compatibility aliases are allowed only when
another design record calls them out explicitly. The user-facing migration story
should compare v1 and v2 directly.

Important v1-to-v2 command moves:

| V1 command or habit | V2 command |
| --- | --- |
| `api show <name>` | `api inspect <name>` |
| `api edit` | `config edit` |
| `api clear-auth-cache <name>` | `api auth logout <name>` |
| `api content-types` | `doctor` / `doctor -o json` content-type diagnostics |
| top-level shell/setup guidance | `shell setup <shell>` plus `shell completion ...` |
| direct plugin service command | explicit service verb such as `mcp serve` |

The request path is intentionally stable:

- bare URLs without body input still perform GET
- bare URLs with shorthand or stdin body input infer POST
- HTTP verb commands remain at the root
- generated API commands remain at the root under the API name
- generated API commands use `--rsh-*` for Restish-owned flags so OpenAPI
  parameter flags can stay unprefixed without collisions
- generic HTTP commands use the same `--rsh-*` request, auth, output, TLS,
  pagination, cache, retry, and streaming controls for consistency with
  generated commands
- command-local workflow flags outside those request surfaces stay unprefixed,
  such as `api connect --spec`, `api auth inspect --operation`, `edit --yes`,
  `cache clear --direct`, and `doctor api --check-network`

## Rejected Alternatives

Do not move generated API calls under `restish api call`.
Generated commands are the product's differentiator. Connected APIs should feel
like native CLIs, not like records inside an API-management subsystem.

Do not keep every old command name as a documented alias.
Aliases are valuable after a stable release, but the first v2 surface should be
smaller, clearer, and easier to teach. Compatibility docs can map v1 habits to
the v2 names instead.

Do not keep a separate `flags` command.
The full global flag surface changes with the binary, but a separate command
creates a second place users and tests have to check. `--help-all` keeps the
reference close to the command whose flags are being inspected and avoids
another top-level noun before the first stable v2 release.

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
- Help tests should cover root, `api`, `api auth`, `config`, `mcp`, a generated
  API operation, and `--help-all` expansion.
- Full CLI/plugin changes should pass `go test -tags=integration ./...` before
  release.

## Documentation Impact

User docs should present v2 commands directly, then include a migration guide
for v1 users. Pages that commonly mention these workflows include:

- quickstart and API connection guides
- command reference
- config and profile docs
- auth troubleshooting
- `--help-all` and global flag reference
- shell setup and completion docs
- MCP/plugin operator docs
- upgrade-from-v1 guide

Docs should explain the chosen v2 model directly. Upgrading users need a
concise v1-to-v2 map.

## Outcome

The implemented v2 surface should follow this decision. `api` owns API
registration and API-auth configuration, `config` owns local configuration,
`--help-all` exposes global Restish controls in command context,
content-type registry diagnostics live in `doctor`, MCP uses `serve`, and shell
integration lives under `shell setup` and `shell completion`.

The resulting command tree keeps the v1 request experience familiar while
making the control plane more explicit and easier to document before v2 becomes
stable.
