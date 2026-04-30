# V2 Command Surface Review

Status: accepted and implemented for the first Restish v2 release.

## Product Frame

Problem:
Restish v2 has a strong request surface, but the command/control surface still
mixes user jobs with internal concepts. Because v2 has not shipped, this is the
right moment to make breaking command-shape changes that reduce support burden
and make the tool easier to explain.

Users:

- first-time users making one request without config
- daily users who want API-specific commands generated from OpenAPI
- API integrators managing profiles, auth, specs, and output defaults
- plugin operators installing trusted workflow extensions
- maintainers who need a command tree that is easy to document and test

Job:
Users should be able to make a request, connect an API, call generated
operations, manage configuration/auth/cache/plugins, and diagnose problems
without learning implementation vocabulary.

Previous workaround:
Users had to infer that `api edit` edited the whole config file, that
`api clear-auth-cache` was auth management rather than API registration
management, that global flags were fully discoverable through `--help-all`, and
that plugin commands such as `mcp` could use their own action vocabulary.

Pain:
The pain is discoverability and vocabulary more than missing capability.
Commands that work correctly can still be hard to remember, hard to document,
or misleading when their object/action shape does not match the user job.

Outcome:
The v2 surface should be explainable as:

1. generic HTTP requests work with no setup
2. connected APIs become top-level native commands
3. `api` manages API registrations and generated-command inputs
4. `config` manages local Restish configuration
5. `api auth` manages API credentials and auth cache
6. workflow extensions use explicit verbs such as `serve`

Evidence:
The current command tree is assembled in `internal/cli/root.go`,
`internal/cli/api.go`, `internal/cli/generated.go`, plugin command files, and
the shipped plugin entry points. `internal/cli/command_surface_test.go` captures
the current map so intentional moves are visible in tests instead of hidden in
help-output drift.

## Previous Command Surface

The previous built-in surface was:

```text
restish
  <url-or-api-short-name> [...]
  get|GET <url>
  head|HEAD <url>
  options|OPTIONS <url>
  post|POST <url> [body...]
  put|PUT <url> [body...]
  patch|PATCH <url> [body...]
  delete|DELETE <url>

  api
    connect <name> <url> [setup-expression ...]
    list
    show <name>
    set <name> <key> <value> | <name> <path:value>
    edit
    sync <name>
    remove <name>
    clear-auth-cache <name>
    content-types
    auth
      list <api>
      add <api> <credential-id>
      remove <api> <credential-id>
      inspect <api>

  cache
    info
    clear [api]

  plugin
    list
    install <source>
    remove <name>
    debug <name> [args...]

  completion
    bash
    zsh
    fish
    powershell
    install <shell>

  setup <shell>
  theme
    set <url-or-user/repo> [name]

  cert <uri>
  doctor
    api <name>
    plugin <name>
    migrate-v1
  edit <uri> [patch ...]
  links <uri> [rel...]
  version
  help
```

Generated APIs add:

```text
restish <api>
  <operation> [required args...] [body...] [operation flags]
  <tag> <operation> ...  # when command_layout: tags
```

Shipped command plugins add optional top-level commands:

```text
restish bulk
  init URL
  list|ls
  status|st
  diff|di [file...]
  reset|re [file...]
  pull|pl
  push|ps

restish mcp <api...>
```

## Product Decision

Keep Restish's core request model:

- bare URL means GET
- HTTP verbs remain top-level
- registered API names remain top-level command groups
- generated operation flags use ordinary API parameter names
- Restish-owned request/global flags keep the `--rsh-*` namespace

The control plane changed before v2 ships:

1. Add `restish config` and move whole-config work there.
2. Move auth cache clearing under `api auth`.
3. Add a first-class global flag reference command.
4. Make the MCP command action explicit with `mcp serve`.
5. Rehome shell setup under a clearer shell/completion workflow.

These are intentional breaking changes. v2 should document the new commands
directly instead of preserving aliases that make the first stable surface
larger and less clear.

## Implemented Target Surface

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
  show
  edit
  set <path:value>
  theme set <source> [name]

restish cache
  info
  clear [api]

restish content-types

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
restish flags
restish version
```

## Exact Changes To Make

### 1. Add `config` As The Configuration Object

Add:

```text
restish config path
restish config show [--json]
restish config edit
restish config set <path:value>
restish config theme set <source> [name]
```

Changed:

- `api edit` becomes `config edit`
- `api set` remains only for API-scoped settings
- `theme set` becomes `config theme set`
- `api show <name>` becomes `api inspect <name>`

Rationale:
`api edit` currently edits the whole config file, not one API. A top-level
`config` object gives users a predictable place for local state and lets `api`
stay focused on registrations, specs, profiles, and generated commands.

Tradeoff:
This adds a new top-level command, but it removes conceptual overload from
`api` and `theme`.

### 2. Move Auth Cache Under `api auth`

Add:

```text
restish api auth clear-cache <api> [--all-profiles]
restish api auth clear-cache --auth-profile <name>
```

Remove:

```text
restish api clear-auth-cache <name>
```

Rationale:
OAuth token cache state is auth state. Keeping it beside `api auth list`,
`api auth add`, and `api auth inspect` makes the recovery workflow easier to
find.

Tradeoff:
The new command is longer, but it is semantically placed and easier to
discover from `restish api auth --help`.

### 3. Add `flags`

Add:

```text
restish flags
restish flags request
restish flags output
restish flags auth
restish flags tls
restish flags pagination
restish flags cache
```

Rationale:
The current `--help-all` mechanism is useful but hidden. A command gives users
a memorable way to discover the full global surface while allowing root and
generated operation help to stay focused.

Tradeoff:
This duplicates some docs content, but it puts the reference exactly where CLI
users look first.

### 4. Change MCP To `mcp serve`

Add:

```text
restish mcp serve <api...>
```

Reserve:

```text
restish mcp inspect <api...>
```

Remove:

```text
restish mcp <api...>
```

Rationale:
MCP starts a long-running protocol server. Naming the action `serve` is clearer
than making the object command do work directly, and it leaves room for MCP
inspection/debugging commands.

Tradeoff:
One extra word in MCP client config, in exchange for a more extensible command
family.

### 5. Rehome Shell Setup

Implemented target:

```text
restish shell setup <shell> [--completion]
```

Removed before release:

```text
restish setup <shell>
```

Rationale:
`setup` sounds like whole-tool setup or API setup. The actual job is shell
integration: noglob alias plus optional completion.

Tradeoff:
`restish setup zsh` was short and quickstart-friendly, but the clearer
object/action shape is worth the extra word before v2 ships.

## Rejected Alternatives

Do not move generated API calls under `restish api call`.
Generated commands are the product's differentiator: connected APIs should feel
like native CLIs.

Do not remove `--rsh-*` globally.
The prefix protects Restish request controls from generated operation parameter
flags. Short aliases such as `-H`, `-f`, `-o`, `-p`, and `-t` already cover the
daily path.

Do not keep old names as documented aliases for v2's first stable release.
Aliases are useful after a release, but v2 is still unreleased and should avoid
shipping avoidable surface area.

## Validation Plan

Tests:

- keep `internal/cli/command_surface_test.go` updated with every intentional
  command move
- add focused tests for each new command family before removing old commands
- assert old command names fail once the final v2 breaking change lands
- cover help output for root, `api`, `api auth`, `config`, `flags`, `mcp`, and
  generated operation help

Docs/help:

- update `site/content/en/docs/reference/commands.md`
- update API management, global flags, shell setup, completion, MCP, and config
  docs
- ensure quickstart uses `api connect`, `config edit`, `api auth clear-cache`,
  `mcp serve`, and the chosen shell setup command consistently

Manual checks:

- `restish --help`
- `restish flags`
- `restish api --help`
- `restish api auth --help`
- `restish config --help`
- `restish mcp serve example`
- one generated API command help page with and without `--help-all`

## Outcome

The implemented v2 surface uses `api inspect`, `config edit`, `config theme
set`, `api auth clear-cache`, `flags`, `mcp serve`, and `shell setup`.
`content-types` moved to a top-level utility command because it describes the
runtime registry rather than API registration state.

`config set` accepts the same key/value and shorthand `path:value` syntax used
by `api set`, but it does not support append expressions for arbitrary paths.
`flags` prints plain text grouped like command help; JSON output can be added
later if a docs-generation or shell-tooling use case appears.
