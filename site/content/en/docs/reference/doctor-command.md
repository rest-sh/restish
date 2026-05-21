---
title: Doctor Command
linkTitle: Doctor
weight: 16
description: Reference for diagnosing Restish configuration, registered APIs, plugins, and v1 migration.
---

Use `restish doctor` when behavior looks environment-specific: the wrong config
file is being read, permissions are suspicious, shell setup is incomplete,
plugins are missing, or a registered API needs a health check.

## Common Examples

```bash
restish doctor
restish doctor -o json
restish doctor api example
restish doctor api example --check-network
restish doctor plugin mcp
restish doctor migrate-v1
```

Prefer JSON output when attaching diagnostics to an issue or comparing results
across machines.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen doctor-command -->
Generated from the current Cobra command tree.

### `restish doctor`

Diagnose Restish configuration and runtime paths

Diagnose Restish configuration and runtime paths.

Use this when Restish is reading the wrong config, permissions look suspicious, shell setup is incomplete, caches are in unexpected locations, or plugin discovery is confusing. Pass `-o json` for structured diagnostics.

Usage:

```text
restish doctor
```

Examples:

```bash
  restish doctor
  restish doctor -o json
  restish doctor api demo --check-network
```

Subcommands:

**`restish doctor api`**: Diagnose a registered API

**`restish doctor migrate-v1`**: Run default-location v1 config migration if eligible

**`restish doctor plugin`**: Diagnose a Restish plugin executable


### `restish doctor api`

Diagnose a registered API

Diagnose one registered API.

The report checks registration, spec cache freshness, generated operation availability, shallow auth readiness, and optional network reachability. Use `--check-network` when you want Restish to make a bounded request to the API base URL. For detailed credential coverage and auth material, run `restish api auth inspect <api>`.

Usage:

```text
restish doctor api <name> [flags]
```

Examples:

```bash
  restish doctor api demo
  restish doctor api demo --check-network
```

Flags:

**`--check-network`**

Type: `bool`; default: `false`

Make a bounded network request to check API reachability



### `restish doctor plugin`

Diagnose a Restish plugin executable

Diagnose one Restish plugin executable.

The report checks plugin discovery, executable status, manifest loading, declared capabilities, and Restish plugin protocol compatibility.

Usage:

```text
restish doctor plugin <name>
```

Examples:

```bash
  restish doctor plugin mcp
```


### `restish doctor migrate-v1`

Run default-location v1 config migration if eligible

Run default-location Restish v1 config migration if eligible.

Use this when upgrading a machine that still has a v1 config in the default location. The command reports whether migration is possible, where files would be written, and why migration may be skipped.

Usage:

```text
restish doctor migrate-v1
```

Examples:

```bash
  restish doctor migrate-v1
  restish doctor migrate-v1 --to ./restish.json
```
<!-- END GENERATED -->

## Related Pages

- [Troubleshooting](/docs/guides/troubleshooting/)
- [Config](../config/)
- [API Management](../api-management/)
- [Plugin Command](../plugin-command/)
