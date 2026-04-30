# Config And Profiles

## Summary

Restish v2 uses a typed JSONC configuration model centered on one primary user
config file, `restish.json`. That file stores global settings, registered APIs,
profiles, and request-affecting defaults.

Profiles are Restish's main "environment" abstraction. They let one API
registration express per-environment differences without duplicating the entire
API definition.

This document defines both the data model and the persistence guarantees around
it.

## Goals

- one obvious place for users to inspect and edit configuration
- strict validation so typos fail early
- comments and formatting preserved when Restish edits the file
- deterministic layering between config, environment, and CLI flags
- safe writes under concurrent processes
- a real migration path from v1 locations and filenames

## Non-Goals

- supporting arbitrary config sources or many file formats
- creating a fully general environment-inheritance system shared across APIs
- silently accepting malformed or unknown fields for flexibility

## Persistent Layout

The primary persistent surfaces are:

- config file: `restish.json`
- token cache
- spec cache
- response cache
- plugin directory

Design 001 defines that the `CLI` runtime owns the resolved paths. This
document defines what those paths are for.

## Path Resolution

Restish should resolve config and cache locations in a developer-friendly,
predictable way:

- XDG-style locations on macOS, Linux, and other Unix-like systems
- native user config and cache locations on Windows
- explicit overrides when tests, scripts, projects, or embedders supply custom
  paths

The design intent is consistency and discoverability. Restish is a shell-native
developer tool, so Unix-like users should be able to find, edit, back up, and
sync configuration from the same `~/.config` tree they use for other CLI tools.
This is more useful for Restish than macOS's native
`~/Library/Application Support` default, which is appropriate for app bundles
but surprising for a terminal-first workflow. Config, token cache, spec cache,
and response cache should all come from one coherent path-resolution strategy
rather than several unrelated helper functions.

The plugin directory is part of this same path model. It should resolve beneath
the configured Restish config directory, so tests, XDG overrides, and embedded
CLIs do not accidentally use a different plugin trust root from the main
configuration.

User-facing config file selection follows this precedence:

1. `--rsh-config <file>`
2. `RSH_CONFIG=<file>`
3. `RSH_CONFIG_DIR=<dir>/restish.json`
4. `XDG_CONFIG_HOME/restish/restish.json`
5. the default config directory:
   - macOS, Linux, and other Unix-like systems: `~/.config/restish/restish.json`
   - Windows: `%APPDATA%\restish\restish.json`

`--rsh-config` and `RSH_CONFIG` select a complete config file, not an overlay.
When either is present, Restish reads and writes only that file. A missing
explicit config file is an error so operators do not accidentally run with the
global default after mistyping a project path. The ordinary platform-default
config path keeps the v1 migration and auto-create behavior described below.

Sidecar state that belongs to the selected config trust root, such as OAuth
token caches and external-tool approval records, lives next to the explicit
config file. Response and spec caches remain under the cache root, but explicit
configs get a cache namespace derived from the config path so two project
configs do not reuse each other's cached HTTP responses or discovered specs.

Cache directory selection mirrors config selection with `RSH_CACHE_DIR`,
`XDG_CACHE_HOME/restish`, `~/.cache/restish` on Unix-like systems, and the
Windows user cache directory on Windows.

## Primary Config Shape

The primary top-level keys are:

- `apis`
- global output or behavior defaults as the product grows

Each entry under `apis` is keyed by a short API name and contains the stable
registration for that API:

- `base_url`
- `spec_url`
- `spec_files`
- pagination configuration
- profile map
- other API-wide metadata such as operation-base or plugin allowlists

The exact field list may evolve, but the structural rule should remain:

- API-level keys describe the API as a whole
- profile-level keys describe environment-specific request behavior

## Profile Model

Profiles live under each API, not globally.

This is deliberate because a profile name such as `prod`, `staging`, or
`enterprise` only makes sense relative to one API registration.

Profiles may set or override:

- `base_url`
- persistent headers
- persistent query parameters
- auth configuration
- TLS signer selection and parameters
- other request-affecting defaults added in future designs

The default selected profile name is `default` unless the user specifies
another profile through env vars or flags.

## Layering And Precedence

When Restish resolves a request through a registered API, precedence is:

1. built-in defaults
2. API registration defaults
3. selected profile values
4. environment-derived overrides
5. explicit CLI flags and command arguments

Two rules matter here:

- profiles provide durable defaults, not hard locks
- invalid explicit selection should error rather than silently falling back

That means a misspelled `--rsh-profile` should be treated as a real error.

## Header And Query Semantics

Persistent headers and query parameters are merged in a way that still lets the
user override them per invocation.

The desired behavior is:

- config contributes defaults
- command flags contribute explicit invocation values
- explicit invocation values win

The implementation detail may be prepend/append plus later overwrite, but the
user-visible contract is override-friendly layering.

## Strict Validation

The config format is intentionally strict:

- JSONC comments are allowed in source text
- after comment stripping or CST parsing, data is decoded into typed structs
- unknown fields are rejected
- invalid enums or malformed values should fail early with location-aware
  diagnostics

Strictness is a feature here. It keeps config drift from becoming runtime
mystery behavior.

Validation diagnostics should list the legal fields for the object that failed
when doing so is practical. A strict parser that says only "unsupported field"
without the valid alternatives makes config migration much harder to debug.

Size and duration values should surface parse errors. Falling back to a default
after a malformed cache size, timeout, or similar operator setting hides the
configuration mistake and makes behavior depend on accident.

## Example

```jsonc
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com",
      "spec_url": "https://api.github.com/openapi.json",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"],
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice",
              "password": "",
            },
          },
        },
        "enterprise": {
          "base_url": "https://github.example.com/api/v3",
          "headers": ["Accept: application/json"],
          "query": ["per_page=100"],
        },
      },
    },
  },
}
```

For:

```bash
restish --rsh-profile enterprise get github/repos/octo/example -q per_page=50
```

the effective request should use:

- the `enterprise` profile's `base_url`
- the profile's persistent headers
- the CLI-supplied `per_page=50` value

## Write Guarantees

Config writes are safety-sensitive because the file is user-authored and often
manually curated.

The design requires:

- cross-process file locking during mutation
- temp-file write, fsync, then rename
- preserving existing line endings where possible
- preserving comments and nearby formatting when commands edit object paths
- refusing writes that would corrupt structure or silently change unrelated
  content

Design 027 covers the patching behavior in more detail.

## Migration From v1

Restish v2 needs an explicit migration path from known v1 config locations and
filenames.

Migration expectations:

- detect legacy locations on supported platforms
- migrate or import on first use when safe
- preserve API registrations, profiles, and auth config
- preserve comments when practical
- emit a clear hint when automatic migration is not possible

Users should not experience "my APIs disappeared" simply because the config path
changed between major versions.

## Credentials

Today, some auth material may still live in the main config file. That is
acceptable for an initial implementation, but the long-term design direction is
to separate credentials from ordinary config more cleanly, either through:

- a dedicated credentials file with restrictive permissions, or
- OS keyring integration

That refactor is compatible with this design as long as profile auth references
remain stable from the operator's point of view.

## Concurrency And Multi-Process Use

Restish should assume users may run multiple instances concurrently:

- two shells editing config
- one command refreshing tokens while another reads them
- automation plus interactive sessions

That means in-memory mutexes are not sufficient for persistence safety.
Cross-process locking is part of the design, not an optimization.

## Alternatives Considered

### Separate Files For APIs And Global Config

Possible, but it makes discovery and editing harder for most users.

### Global Profiles Shared Across All APIs

This weakens the connection between profile names and the API-specific behavior
they control.

### Loose Schema With Best-Effort Field Preservation

This improves short-term flexibility but hurts confidence and debuggability.

## Relationship To Other Designs

- Design 004 consumes profile auth configuration.
- Design 006 consumes spec-related config.
- Design 017 defines how env vars and flags override config.
- Design 027 defines comment-preserving edit behavior.
- Design 031 defines migration expectations from v1.
