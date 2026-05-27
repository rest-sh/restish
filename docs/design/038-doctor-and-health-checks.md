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
- keep machine-readable JSON available explicitly without making redirected
  human reports disappear from pipelines

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
```

`doctor` is a diagnostic command. When stdout is a terminal, it writes the
human report to stderr to match the rest of Restish's diagnostic channel
policy. When stdout is redirected, it writes the human report to stdout and
prints a one-line stderr hint pointing to `-o json` for machine-readable output.
This makes `restish doctor > report.txt` capture the report users intended to
share while preserving stderr for diagnostics about doctor itself.
Human status words may be colorized when terminal color is enabled: okay states
use the success/status theme color, warnings and unknown states use the warning
diagnostic color, failures use the error diagnostic color, and remediation hints
use the hint diagnostic color. Redirected output remains plain by default and
JSON output is never colorized.

`-o json` is the explicit machine mode for the entire command family. In JSON
mode, `doctor`, `doctor api`, and `doctor plugin` write one structured JSON
document to stdout and keep normal human diagnostics off stderr unless a
lower-level plugin manifest probe writes its own stderr.

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
- installed plugin names, versions, and capability summaries in human output,
  with paths and capability details in JSON output
- registered content-type names in human output, with names, MIME types,
  suffixes, and quality values in JSON output
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
- OpenAPI metadata issues that make generated operations hard to configure,
  such as operation security requirements that reference undeclared security
  schemes
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
can run `api sync`, `api auth inspect`, or real requests when they need those
more specific checks. Doctor should point to `api auth inspect` for detailed
credential coverage, readiness, and auth material instead of duplicating that
deeper auth-debugging surface.

## Plugin Doctor

`restish doctor plugin <name>` inspects a plugin executable.

Resolution rules:

- an absolute or path-like argument is used directly
- a bare plugin name is resolved under the configured plugin directory
- bare names without the `restish-` executable prefix are resolved as
  `restish-<name>`, so `restish doctor plugin csv` inspects `restish-csv`

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

## Failure And Exit Behavior

Doctor findings are not normal request failures. The command should usually
exit zero after printing a complete report, even when the report contains words
such as "invalid", "missing", or "insecure".

Non-zero exit is reserved for cases where doctor cannot perform the requested
diagnostic action, such as an internal I/O failure while reading diagnostic
inputs, an invalid command invocation, or a named target that does not exist
(`doctor api <missing>` / `doctor plugin <missing>`). Doctor should still print
the diagnostic report it can produce before returning non-zero. This keeps
doctor usable in support sessions: users can paste the report without first
deciding which findings were fatal.

When stdout is a terminal, the absence of stdout is not a bug: stderr carries
the human report. When stdout is redirected, stdout carries the human report so
redirection captures it naturally.

## Relationship To Other Designs

- Design 002 defines config paths, migration, strict schema, and permissions.
- Design 017 defines bootstrap-safe command behavior and stderr diagnostics.
- Design 018 defines plugin discovery and manifest trust boundaries.
- Design 030 defines sensitive-data handling and persistence safety.
- Design 037 records why `doctor` is top-level instead of living under `api`.
