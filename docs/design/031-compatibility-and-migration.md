# Compatibility And Migration

## Summary

Restish v2 is a redesign, but it is still a successor to v1 rather than a brand
new product. That means the project needs an explicit compatibility and
migration stance.

This document defines:

- which kinds of v1 behavior should be preserved
- which breaks are intentional
- how configuration and operator workflows migrate
- how to judge whether v2 is ready to release

## Compatibility Philosophy

The goal is not byte-for-byte behavioral identity with v1. The goal is:

- preserve user intent where possible
- preserve muscle memory for common workflows
- restore accidental regressions before release
- document intentional breaks clearly when the new design is better

The project should treat "pre-release" as a chance to fix compatibility gaps,
not as permission to leave them undocumented.

## Compatibility Tiers

### Tier 1: Must Preserve Or Restore

These are high-value behaviors that should match v1 unless a new design record
explicitly says otherwise:

- suffixed structured content types such as `application/problem+json`
- generated-command fallback naming when `operationId` is absent
- correct handling of path-level parameters and `servers[]`
- HTML-unescaped JSON output
- printable-text rendering for text bodies
- query/header/profile ergonomics users already depend on
- manual header override semantics, especially for `Accept`
- documented environment input shapes such as comma-separated `RSH_HEADER`
- headless-friendly auth paths for remote or SSH usage
- interactive edit workflows, including v1-compatible shorthand flags where
  the command still exists
- response metadata filtering such as `headers.Date`
- explicit full-response filters such as `-f @`
- shorthand filter examples from the v1 docs when the syntax is clearly
  shorthand rather than jq
- raw and redirected binary output fidelity
- v1-style aliases where they materially reduce migration pain

### Tier 2: May Change With Documentation

These can change if the new design is better and the migration is documented:

- plugin architecture and plugin packaging
- config file layout
- setup/completion commands
- output defaults where v1 had ambiguous behavior
- auth handler internals and storage details

### Tier 3: Internal Freedom

These can change freely as long as public behavior stays stable:

- package layout
- internal type names
- cache serialization details
- loader implementation strategy

## Intentional v2 Changes

The current design set intentionally changes v1 in at least these areas:

- central `CLI` runtime instead of wider global state
- out-of-process plugin architecture instead of only built-ins/in-process hooks
- stronger separation between document and record output
- JSONC-backed typed config model
- more explicit pipeline planning for pagination and streaming
- retirement of the v1 interactive `api connect <name>` prompt flow in favor
  of `restish.json`, `api connect`, `api set`, and `config edit`
- removal of the v1/v2-draft API-or-URI, Authorization-header-only auth inspect
  behavior in favor of `restish api auth inspect <api> --raw-header
  Authorization`, because v2 auth can be credential-specific and may not use
  the `Authorization` header

Those are acceptable breaks, but they require migration documentation and
operator guidance.

## Migration Surfaces

### Configuration Files

Restish must support a migration path from v1 config locations and filenames.

The implemented v2 behavior is:

- when the default v2 `restish.json` is missing, detect known legacy locations
  on startup or first write
- automatically migrate v1 `apis.json` and `config.json` into `restish.json`
  when safe
- copy legacy files into an atomic backup directory before writing v2 config;
  reuse a matching `.bak.v1` backup during recovery, or create a numbered
  `.bak.v1.N` backup when the existing backup has different contents
- remove legacy `apis.json` and `config.json` after the new `restish.json` has
  been written and parsed successfully, so deleting `restish.json` later does
  not silently re-import stale v1 state
- treat `RSH_CONFIG_DIR` as a clean v2 config root; it does not scan or mutate
  platform legacy locations
- preserve comments where possible
- emit a clear hint when migration cannot be automatic

Migration should not be macOS-only, Linux-only, or implicit based on whichever
path happened to work on the developer's machine.

Automatic v1 migration is limited to the default platform config path when no
v2 config exists. `restish doctor migrate-v1` is the explicit operator command
for running or inspecting default-location v1 migration when the normal
first-run path is not enough.

Explicit config file selection is intentionally stricter. If `--rsh-config` or
`RSH_CONFIG` names a file that does not exist, Restish errors instead of falling
back to global config or running default-location migration. That makes project
configs predictable and prevents accidental writes to the wrong config file.

If no config root can be resolved from explicit config, `RSH_CONFIG_DIR`,
`XDG_CONFIG_HOME`, platform user directories, or `HOME`, Restish should fail
with a setup error instead of creating relative config state in the current
working directory. Cache-only state can use a temporary fallback, but persistent
configuration cannot.

### API Registrations

Migrated API registrations should preserve:

- short names
- base URLs
- spec URLs or files
- profile names
- auth settings
- pagination settings

If v2 cannot preserve a field, the migration path must report that explicitly.

### Command Names And Aliases

Generated commands should preserve stable names where possible and provide
aliases for common v1 spellings when the new canonical naming changed only for
implementation reasons.

### Auth Workflows

Users upgrading from v1 should not lose access to common environments such as:

- browser-capable local machines
- SSH sessions
- CI/service-account flows

If a v1 auth flow is intentionally removed, the replacement path must be
documented before release.

## Release-Readiness Checklist

Before v2 release, the design expects explicit sign-off on:

1. configuration migration works on supported platforms
2. v1 accidental regressions have been fixed or consciously retired
3. the docs site has a migration guide, not just scattered notes
4. module/install instructions point at the canonical v2 module path
5. core commands present in v1 and v2 are all documented
6. plugin differences from v1 are explained to both operators and authors

## Documentation Requirements

The user-facing docs should include a dedicated migration guide with:

- where config moved
- how profiles map
- renamed or removed commands
- changed defaults
- plugin model changes
- known non-goals and removals

Design records alone are not sufficient for this. The migration guide belongs
in the site docs as well.

## Regression Classification

When reviewing a v2 behavior difference, classify it as one of:

- intentional improvement
- acceptable break needing documentation
- accidental regression that must be fixed
- unclear; requires product decision

That classification should appear in design review or issue discussion so the
project does not normalize accidental regressions as "just different now."

## v1 Documentation Examples As Tests

Commands from the v1 documentation are regression inputs for v2 when the same
command shape is still accepted. The important compatibility target is user
intent, not incidental formatting.

The following examples represent specific classes that should be covered by
tests or migration notes:

- `-H 'Accept: application/json'` narrows the accepted response types instead
  of appending to the generated Restish accept string
- `RSH_HEADER=header1:value1,header2:value2` produces multiple headers unless
  a future design replaces that input shape with a documented alternative
- `restish edit -i ...` enters the supported interactive edit path or has a
  documented replacement before release
- `-f headers.Date`, `-f headers`, `-f status`, and `-f @` operate on the
  normalized response envelope rather than the body alone
- shorthand filters used by v1 examples continue to parse as shorthand in auto
  mode when they use bare normalized-response roots or shorthand recursive
  descent such as `..url`
- pagination progress never appears in stdout, and metadata filters do not
  fetch extra pages merely because a body collection has a next link
- `-r` and redirected unfiltered downloads preserve the original response body
  bytes

Any future v1-docs example that does not work in v2 should be classified before
release as restored, intentionally changed with documentation, or unsupported.

## Decision Rule

If a v1 behavior was:

- widely visible to users
- low-cost to preserve
- not in tension with safety or architecture

then v2 should usually preserve or restore it.

If a v1 behavior was:

- confusing
- unsafe
- tightly coupled to architecture being removed

then v2 may break it, but the break should be explicit in both design docs and
user-facing migration material.
