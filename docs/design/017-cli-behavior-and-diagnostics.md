# CLI Behavior And Diagnostics

## Summary

Restish v2 uses a few consistent command-line behavior rules across commands:

- HTTP status classes map to CLI exit codes
- verbose request/response diagnostics go to stderr
- normal response output goes to stdout
- flags can explicitly suppress output or ignore HTTP-derived exit codes

These rules keep Restish usable both interactively and in scripts.

## Problem

CLI tools are often used in pipelines and automation where output channels and
exit codes matter as much as the data itself.

The design needed to:

- preserve machine-readable stdout
- make diagnostics visible without corrupting stdout
- signal HTTP failures in a predictable way
- give users opt-outs when they care about output more than status

## Design

Restish maps HTTP status classes to exit codes:

- `2xx -> 0`
- `3xx -> 3`
- `4xx -> 4`
- `5xx -> 5`

This makes status-family failures visible to scripts without inventing a large
custom exit code taxonomy.

Diagnostic output follows a channel split:

- stdout is for the selected command result
- stderr is for prompts, warnings, progress, and verbose request/response logs

Verbose mode currently emphasizes request and response visibility with `-v`.
This includes lines like:

- request method and URL
- request headers
- response protocol and status
- response headers

Two flags intentionally modify the normal contract:

- `--rsh-ignore-status-code` forces a zero exit code even for failing HTTP
  responses
- `--rsh-silent` suppresses output entirely so only the exit code matters

The root command also supports a convenience behavior: a bare URL or API short
name is treated as a GET request even without an explicit verb command.

## Examples

Normal GET with body on stdout and exit code based on HTTP status:

```bash
restish https://api.example.com/items
```

Verbose diagnostics to stderr:

```bash
restish get https://api.example.com/items -v
```

Ignore HTTP-derived exit codes:

```bash
restish get https://api.example.com/items --rsh-ignore-status-code
```

Suppress output entirely:

```bash
restish get https://api.example.com/items --rsh-silent
```

## Alternatives Considered

### Use one generic non-zero exit code for all HTTP failures

That would be simpler, but it throws away information that is often useful in
automation.

### Send verbose logs to stdout

That would make interactive use look fine at times, but it would break piping
and machine-readable output.

### Require explicit verbs for all requests

That would be more rigid, but it would make quick one-off usage less pleasant.
Treating bare URLs as GET requests is a helpful shortcut.

## Notes

The current implementation reflects this design directly:

- `internal/output/response.go` defines HTTP status to exit-code mapping
- `internal/cli/http.go` applies output selection, status handling, and verbose
  logging
- the root command dispatches bare URLs as GET requests

One detail worth preserving is that Restish still writes the response body to
stdout even when the eventual return value is a non-zero HTTP-derived exit code.
That keeps failure responses inspectable without forcing users to choose between
body visibility and correct script signaling.
