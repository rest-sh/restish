---
title: Doctor Command
linkTitle: Doctor
weight: 16
description: Reference for diagnosing Restish configuration, registered APIs, plugins, and runtime state.
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
```

Prefer JSON output when attaching diagnostics to an issue or comparing results
across machines.

TTY human output colorizes status words when terminal color is enabled. JSON
output is stable and uncolored.

`doctor api <name>` and `doctor plugin <name>` exit non-zero when the named API
or plugin does not exist. They still print the diagnostic report they can
produce so the output is useful in bug reports.

Permission diagnostics are strongest on Unix-like systems, where Restish checks
config and token-cache mode bits. On Windows, existing config and token-cache
files report permission status as `unknown` because ACL inspection is not yet
implemented; this avoids reporting `ok` for a file Restish did not actually
verify.

The root report also lists installed plugins and registered content type
aliases. Use `-o json` for plugin paths, detailed plugin capabilities, MIME
types, structured suffixes, and quality values.

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
<!-- END GENERATED -->

## Related Pages

- [Troubleshooting](/docs/guides/troubleshooting/)
- [Config](../config/)
- [API Management](../api-management/)
- [Plugin Command](../plugin-command/)
