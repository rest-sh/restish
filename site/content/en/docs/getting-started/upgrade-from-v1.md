---
title: Upgrade From v1
linkTitle: Upgrade From v1
weight: 40
description: Move from Restish v1 to v2 with the config migration behavior, command changes, and the main workflow differences in one place.
aliases:
  - /docs/getting-started/changelog/
---

This page is the shortest path for existing Restish v1 users who want to know
what changed, what is preserved automatically, and what needs a manual update.
The archived v1 documentation remains available at [rest.sh/v1/](https://rest.sh/v1/) for
old links, internal runbooks, and side-by-side migration checks.

## What To Expect

Restish v2 keeps the same basic idea: start with generic HTTP requests, then
register APIs for generated commands, profiles, auth, filtering, pagination,
and plugins.

The main upgrade differences are:

- v2 stores persistent config in one `restish.json` file instead of separate
  `apis.json` and `config.json`
- the first v2 run auto-migrates legacy v1 config files when no v2 config
  exists yet
- generated commands are stricter and more explicit in a few places
- v2 adds a general plugin system; v1 only had the external auth command hook

If Restish migrates your config, it prints a one-time notice like:

```text
Migrated config from v1 at /path/to/restish; kept backup at /path/to/restish.bak.v1
```

That backup directory contains copies of your original v1 files. After the new
`restish.json` is safely written, Restish removes the migrated `apis.json` and
`config.json` so stale v1 state is not imported again later.

Migration warnings are printed after the notice when v1 values cannot be
carried over safely. For example, v2 only accepts `operation_base` as an
absolute path such as `/v1`; legacy full-URL `operation_base` values are
dropped with a warning instead of being rewritten to a different host.

## Config Migration

Restish v2 uses `~/.config/restish/restish.json` on macOS, Linux, and other
Unix-like systems by default. Windows uses `%APPDATA%\restish\restish.json`.

When v2 does not find `restish.json` in the default location, it checks the
legacy v1 config locations:

- macOS: `~/Library/Application Support/restish/`
- Linux: `~/.config/restish/`

It reads `apis.json` and `config.json`, converts supported API/profile settings
into the v2 shape, writes the new `restish.json`, and keeps a `.bak.v1` backup
of the original files. If `.bak.v1` already exists and contains matching files,
Restish reuses it to recover an interrupted migration. If it exists with
different content, Restish writes a numbered backup such as `.bak.v1.2`. On
macOS this means v1 files can be found in the old `Application Support` backup
while the v2 config is written to the developer-friendly `~/.config/restish/`
location.

Explicit config selection is different. If you pass `--rsh-config ./restish.json`
or set `RSH_CONFIG`, that exact file must already exist. Restish does not use
an explicit missing file as a signal to search other config directories or run
default-location v1 migration.

The migration carries over the main API-specific settings:

- API base URLs
- `spec_files`
- `operation_base`
- profiles
- persistent headers and query params
- auth type and auth params
- PKCS#11 TLS signer settings

Comments from the v1 files remain in the backup copies. The generated
`restish.json` is JSONC: it includes a short migration header, converted v2
config, and comments that Restish preserves during later config edits where
possible.

## Deliberate Behavior Changes

These changes are intentional in v2 and are not treated as regressions:

- v0-style slug aliases are gone and are not auto-created in v2
- config is centered on `~/.config/restish/restish.json` instead of separate
  global and API files; on macOS this moves v2 config out of
  `~/Library/Application Support`
- `--rsh-config` and `RSH_CONFIG` select one exact config file and no longer
  overlay or fall back to the default config when that file is missing
- generated command names are derived directly from the current spec instead of
  preserving older alias behavior
- redirected non-TTY output preserves response body bytes when no filter,
  collection, metadata shortcut, or output format is set; use `-o json` when a
  script needs Restish to render decoded structured data as JSON
- `-r` is removed. Redirect stdout without a filter, metadata shortcut,
  collection, or output format when you want to save response body bytes.
- interactive HTTP requests now print status, response headers, and formatted
  body to stdout when no explicit filter is set. Use `--rsh-print` to choose
  request/response parts, or `-f @` for the normalized response envelope.
- filter language is auto-detected between shorthand and jq; use
  `--rsh-filter-lang` only when a filter is ambiguous
- automatic pagination follows `next` links only on the same origin. Cross-host
  pagination stops with a warning unless a command has an explicit opt-in for
  that discovery path.

## Command And Flag Mapping

Use this as the fast lookup table when muscle memory collides with v2.

| v1                                    | v2                                                          | Status                                                |
| ------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------- |
| `apis.json`                           | `restish.json`                                              | API config now lives under top-level `apis`           |
| `config.json`                         | `restish.json`                                              | v2 uses one config file                               |
| `restish api configure <name> <url>`  | `restish api connect <name> <url>`                          | v2 prints a migration hint for `api configure`        |
| `restish api edit`                    | `restish config edit`                                       | Config editing moved under `config`                   |
| old interactive API setup             | `restish api connect <name> <url>`                          | v2 expects the base URL explicitly                    |
| n/a                                   | `restish api connect <name> <url> 'path:value'`             | fast one-shot registration with shorthand expressions |
| n/a                                   | `restish api set <name> 'path:value'`                       | shorthand updates support set/append/delete           |
| `restish api clear-auth-cache <name>` | `restish api auth logout <name>`                            | Token cache state lives under `api auth`              |
| `restish api auth inspect <uri>`      | `restish api auth get <api>`                                | URL form was replaced by API/profile-aware inspection |
| `auth.name`                           | `auth.type`                                                 | Profile auth config field renamed                     |
| profile `base`                        | profile `base_url`                                          | API/profile base field renamed                        |
| API `base`                            | API `base_url`                                              | API base field renamed                                |
| `-p`, `--rsh-profile`                 | `-p`, `--rsh-profile`                                       | Same flag, but invalid profile names now error        |
| external auth command hook            | `external-tool` auth                                        | Same local-helper idea; v2 plugin system is separate  |

## Extension Changes

Restish v1 did not have general plugin binaries. It had an external auth
command hook for local helpers that could provide or mutate auth material. V2
keeps that local-helper path as `external-tool` auth, and adds a separate plugin
system for broader extensions.

Use the v2 plugin system when you want to install or build extensions for:

- command workflows
- output formatters
- API loaders
- request or response middleware
- auth hooks
- TLS signing

For current plugin setup and authoring docs, start with:

- [Install And Use Plugins](/docs/plugins/install-and-use/)
- [Plugin Quickstart](/docs/plugins/quickstart/)

## Check Your Upgrade Quickly

After the first run, verify the migrated config and one known request:

```bash
restish api list
restish api inspect your-api
restish -p default your-api --help
```

If that looks correct, make one real request:

```bash
restish your-api some-operation
```

Or use a generic URL request first if you want a smaller smoke test:

```bash
restish api.rest.sh/
```

## When To Edit By Hand

Open `restish.json` directly when:

- you want to review the migrated comment blocks
- you need to clean up old values after the first run
- you had v1 global config defaults that do not map cleanly into the v2 model

Use:

```bash
restish config edit
```

## Related Pages

- [Install](../install/)
- [Tour of Restish](../tour/)
- [Config](/docs/reference/config/)
- [Install And Use Plugins](/docs/plugins/install-and-use/)
- [Archived v1 Docs](https://rest.sh/v1/)
