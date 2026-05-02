# Doctor And Health Checks

Status: accepted for Restish v2 diagnostics.

## Summary

`restish doctor` is the operator recovery command for local Restish health. It
checks configuration, paths, caches, plugins, shell setup, and registered API
state without pretending to be a request command.

The command exists because strict startup is the right default for real work,
but users still need a reliable way to inspect a broken environment.

## Problem

Restish v2 deliberately validates config strictly, loads cached operation
metadata offline, and treats plugins as trusted executables with explicit
capability declarations. Those choices make normal command behavior predictable,
but they can also leave a user stuck when:

- the config file has a typo or obsolete v1 field
- file permissions make token caches unsafe
- a plugin executable cannot be found or queried
- generated operation metadata is missing
- shell globbing keeps rewriting shorthand arguments
- v1 config migration did or did not run

Users should not need to run a network request, edit config by hand, or read
source code to distinguish those cases.

## Goals

- stay available in bootstrap mode when full config parsing fails
- make local state and path decisions visible
- separate read-only diagnostics from mutating repair commands
- make network checks opt-in and bounded
- return success for diagnostic findings that were reported cleanly
- keep machine-readable stdout free for future structured modes

## Non-Goals

- automatic repair of every problem
- replacing strict config validation for normal commands
- validating remote API semantics beyond lightweight reachability
- becoming a full plugin sandbox or package verifier

## Command Surface

The accepted command family is:

```text
restish doctor
restish doctor api <name> [--check-network]
restish doctor plugin <name>
restish doctor migrate-v1
```

`doctor` is a diagnostic command, so it writes its report to stderr. That is
intentional: diagnostics, warnings, prompts, and progress belong on stderr
across Restish.

`--json` is the explicit machine mode for the entire command family. In JSON
mode, `doctor`, `doctor api`, `doctor plugin`, and `doctor migrate-v1` write
one structured JSON document to stdout and keep normal human diagnostics off
stderr unless a lower-level plugin manifest probe writes its own stderr.

## Root Doctor

`restish doctor` reports:

- active config file path
- config parse status and API count when parse succeeds
- unknown-field diagnostics with line/column, suggestions, and migration hints
  when parse fails in a way the diagnostic parser can explain
- config-file permission status
- HTTP response-cache path
- spec-cache path
- token-cache path and permission status
- plugin directory path
- shell setup hint for supported detected shells

The root command should not fail just because it found a problem it could
describe. A malformed config, insecure token cache, missing shell setup, or
unsupported shell is a diagnostic finding. The command should fail only when
the diagnostic machinery itself cannot continue in a meaningful way.

## API Doctor

`restish doctor api <name>` inspects one registered API from the selected
config and active profile.

It reports:

- whether the API is registered
- base URL
- explicit spec URL, if configured
- spec files, if configured
- whether a cached raw spec is present
- whether generated operation metadata is available and how many operations it
  contains
- whether profile auth or credential bindings are configured for the active
  profile
- reachability status

Reachability is skipped by default. `--check-network` performs a bounded `HEAD`
probe with a short timeout and reports the resulting HTTP status or connection
error. Any HTTP response from the server, including `405 Method Not Allowed`,
means the network path reached the API; 405 should be reported as reachable
with a note that `HEAD` is not supported. The probe is intentionally not part
of default doctor output because doctor should remain useful offline and should
not surprise users by contacting private APIs.

API doctor should not run generated operations, refresh specs, follow
pagination, or validate operation auth by sending application requests. Users
can run `api sync`, `api auth list`, or real requests when they need those more
specific checks.

## Plugin Doctor

`restish doctor plugin <name>` inspects a plugin executable.

Resolution rules:

- an absolute or path-like argument is used directly
- a bare plugin name is resolved under the configured plugin directory

The command reports:

- whether the path exists
- whether it is executable
- manifest parse and compatibility status
- manifest name and version
- declared capabilities
- startup protocol API version

Manifest loading necessarily executes the plugin's manifest mode. That is not
a sandbox: it is the same trust boundary used during plugin discovery and
installation. Doctor should make the path and manifest identity visible so the
operator can see exactly what was inspected.

## v1 Migration Doctor

`restish doctor migrate-v1` runs the default-location v1 migration path if and
only if that path is eligible:

- explicit config selection skips migration
- an existing v2 config skips migration
- inaccessible config paths are reported without crashing
- no eligible v1 config is reported as a clean diagnostic outcome
- successful migration reports the v1 source and backup path

This command exists because automatic migration only runs in the default config
path case. Users who are debugging a migration should have a named command that
explains whether migration is skipped, impossible, unnecessary, or completed.

## Failure And Exit Behavior

Doctor findings are not normal request failures. The command should usually
exit zero after printing a complete report, even when the report contains words
such as "invalid", "missing", or "insecure".

Non-zero exit is reserved for cases where doctor cannot perform the requested
diagnostic action, such as an internal I/O failure while reading diagnostic
inputs or an invalid command invocation. This keeps doctor usable in support
sessions: users can paste the report without first deciding which findings were
fatal.

The absence of stdout is not a bug. Stderr carries the human report.

## Relationship To Other Designs

- Design 002 defines config paths, migration, strict schema, and permissions.
- Design 017 defines bootstrap-safe command behavior and stderr diagnostics.
- Design 018 defines plugin discovery and manifest trust boundaries.
- Design 030 defines sensitive-data handling and persistence safety.
- Design 037 records why `doctor` is top-level instead of living under `api`.
