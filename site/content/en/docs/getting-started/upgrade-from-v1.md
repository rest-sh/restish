---
title: Upgrade From v1
linkTitle: Upgrade From v1
weight: 40
description: Move from Restish v1 to v2 with the config migration behavior, command changes, and the main workflow differences in one place.
---

This page is the shortest path for existing Restish v1 users who want to know
what changed, what is preserved automatically, and what needs a manual update.

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
- the plugin protocol changed, so v1 plugins do not load in v2

If Restish migrates your config, it prints a one-time notice like:

```text
Migrated config from v1 at /path/to/restish; kept backup at /path/to/restish.bak.v1
```

That backup directory contains copies of your original v1 files.

## Config Migration

When v2 does not find `restish.json`, it checks the legacy v1 config locations:

- macOS: `~/Library/Application Support/restish/`
- Linux: `~/.config/restish/`

It reads `apis.json` and `config.json`, converts supported API/profile settings
into the v2 shape, writes `restish.json`, and keeps a `.bak.v1` backup of the
original directory.

The migration carries over the main API-specific settings:

- API base URLs
- `spec_files`
- `operation_base`
- profiles
- persistent headers and query params
- auth type and auth params
- PKCS#11 TLS signer settings

Comments from the v1 files are preserved in the generated `restish.json` as
commented reference blocks at the top of the file.

## Deliberate Behavior Changes

These changes are intentional in v2 and are not treated as regressions:

- v0-style slug aliases are gone and are not auto-created in v2
- config is centered on `restish.json` instead of separate global and API files
- generated command names are derived directly from the current spec instead of
  preserving older alias behavior
- plugin executables must speak the v2 plugin protocol

## Accidental Regressions Already Fixed

Several upgrade pain points from early v2 builds have already been restored or
fixed:

- missing `operationId` values now fall back to generated command names again
- path-level parameters are merged correctly into generated commands
- required headers are marked required instead of silently optional
- `servers[]` base-path handling works again
- structured `+json` and related content types decode correctly
- invalid profile names error again instead of silently falling back

If you hit one of those in an older v2 build, upgrade before debugging further.

## Command And Flag Mapping

Use this as the fast lookup table when muscle memory collides with v2.

| v1 | v2 | Notes |
| --- | --- | --- |
| `apis.json` | `restish.json` | API config now lives under top-level `apis` |
| `config.json` | `restish.json` | v2 uses one config file |
| `restish api edit` | `restish api edit` | Same command, now opens `restish.json` |
| `restish api configure <name>` | `restish api configure <name> <url>` | v2 expects the base URL explicitly |
| n/a | `restish api add <name> <url> 'path:value'` | fast one-shot registration with shorthand expressions |
| n/a | `restish api set <name> 'path:value'` | shorthand updates support set/append/delete |
| `auth.name` | `auth.type` | Profile auth config field renamed |
| profile `base` | profile `base_url` | API/profile base field renamed |
| API `base` | API `base_url` | API base field renamed |
| `-p`, `--rsh-profile` | `-p`, `--rsh-profile` | Same flag, but invalid profile names now error |
| v1 plugin binaries | v2 plugin binaries | Rebuild or replace plugins for the v2 protocol |

## Plugin Changes

Restish v2 still supports plugins, but the wire protocol and manifest model are
different enough that v1 plugins will not load unchanged.

Plan on one of these paths:

- install an updated v2-compatible plugin binary
- rebuild your own plugin against the v2 protocol
- replace old plugin behavior with built-in v2 features where possible

For current plugin setup and authoring docs, start with:

- [Install And Use Plugins](/docs/plugins/install-and-use/)
- [Plugin Quickstart](/docs/plugins/quickstart/)

## Check Your Upgrade Quickly

After the first run, verify the migrated config and one known request:

```bash
restish api list
restish api show your-api
restish -p default your-api --help
```

If that looks correct, make one real request:

```bash
restish your-api some-operation
```

Or use a generic URL request first if you want a smaller smoke test:

```bash
restish https://api.rest.sh/
```

## When To Edit By Hand

Open `restish.json` directly when:

- you want to review the migrated comment blocks
- you need to clean up old values after the first run
- you had v1 global config defaults that do not map cleanly into the v2 model

Use:

```bash
restish api edit
```

## Related Pages

- [Install](../install/)
- [First Request](../first-request/)
- [Your First API Config](../your-first-config/)
- [Config](/docs/reference/config/)
- [Install And Use Plugins](/docs/plugins/install-and-use/)
