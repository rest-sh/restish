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

Restish should resolve config and cache locations in a platform-correct way:

- XDG locations on Linux and other Unix-like systems where applicable
- native config locations on macOS and Windows where appropriate
- explicit overrides when tests or embedders supply custom paths

The design intent is consistency. Config, token cache, spec cache, and response
cache should all come from one coherent path-resolution strategy rather than
several unrelated helper functions.

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
