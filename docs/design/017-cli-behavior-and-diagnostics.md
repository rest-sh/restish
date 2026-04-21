# CLI Behavior And Diagnostics

## Summary

Restish v2 must behave like a serious CLI tool in both interactive and scripted
contexts. That means command resolution, prompts, stdout/stderr use, verbose
logging, cancellation, and exit codes all need explicit rules.

This document defines those operator-facing rules.

## Goals

- machine-readable stdout stays clean
- stderr carries diagnostics, prompts, warnings, and progress
- exit codes are predictable
- explicit user choices beat implicit heuristics
- cancellation behaves consistently across commands

## Command Resolution Rules

The root command resolves in this order:

1. built-in commands
2. generated API command groups
3. plugin-contributed commands
4. bare-URL or bare-API GET convenience behavior where applicable

Built-ins must win over generated or plugin names. An API named `cache` should
not accidentally shadow `restish cache`.

Bare-target GET is a convenience, not a replacement for command parsing. It
must not bypass the normal subcommand tree in cases where the input is actually
meant to be a built-in command.

## Global Flags And Environment

Global flags should have one consistent precedence model:

1. built-in defaults
2. environment variables
3. explicit CLI flags

Help text must only claim env-var support that actually exists.

Global flags should eventually be parsed into one structured runtime object so
every command sees the same resolved values instead of re-reading ad-hoc state.

## Stdout And Stderr Contract

The channel split is strict:

- stdout is only for command result output
- stderr is for diagnostics, prompts, warnings, progress, and verbose logs

This is one of the most important scriptability guarantees in the product.

## Verbose Logging

Verbose mode should expose enough information to debug request flow without
breaking stdout contracts.

At `-v`, users should see at least:

- request method and URL
- selected headers with sensitive values redacted
- response protocol and status
- response headers
- pagination or retry progress when relevant

At higher verbosity, Restish may add:

- TLS/certificate details
- cache hit/miss detail
- plugin lifecycle traces

If help text advertises a verbosity level, the behavior should exist.

## Redaction

Verbose logs must redact:

- sensitive headers
- sensitive query parameters
- token-like values returned in structured remote error bodies

Redaction rules are defined in design 030 and apply here.

## Exit Codes

Restish uses both HTTP-derived and local-process-derived exit semantics.

Recommended mapping:

- `0` for success
- `3` for HTTP 3xx when surfaced as final status
- `4` for HTTP 4xx
- `5` for HTTP 5xx
- `1` for generic local runtime failure
- `2` for local usage or validation failure where a distinct code is useful
- `130` for SIGINT / canceled interactive execution

The exact non-HTTP local mapping may evolve, but Restish should clearly
distinguish "the server returned an error" from "the CLI failed locally."

## Output Versus Exit Status

Restish may still write the response body to stdout even if the final exit code
is HTTP-derived non-zero. This is a useful contract for inspecting failure
responses.

Two explicit flags modify the normal behavior:

- `--rsh-ignore-status-code` forces HTTP-derived exit status to zero
- `--rsh-silent` suppresses normal output

Silence affects output channels, not internal success/failure semantics.

## Prompting

Prompts should be coordinated through the runtime and behave consistently across:

- auth prompts
- edit confirmations
- plugin prompts

Prompting rules:

- prefer terminal devices over stdin when stdin is occupied by piped data
- do not treat EOF as implicit confirmation for destructive actions
- defaults should be explicit in both UX text and behavior

## Cancellation

The root command should derive its context from `signal.NotifyContext`.

Every long-running command and subprocess should honor that context, including:

- HTTP requests
- OAuth waits
- pagination loops
- plugin sessions
- certificate inspection

On cancellation, Restish should avoid noisy redundant error output and should
exit like a normal Unix CLI.

## Shell Setup And Truth In Help

User-facing command help and docs must match actual supported behavior.

That means:

- only supported shells should be advertised by `setup`
- platform-specific shell startup files should be handled honestly
- completion claims should match actual registered completion behavior

Design docs and help text should not promise capabilities that the command
rejects.

## Examples

Normal GET with status-derived exit behavior:

```bash
restish https://api.example.com/items
```

Verbose diagnostics on stderr:

```bash
restish get https://api.example.com/items -v
```

Ignore HTTP-derived exit status:

```bash
restish get https://api.example.com/items --rsh-ignore-status-code
```

## Relationship To Other Designs

- Design 001 defines runtime I/O ownership.
- Design 016 covers setup and completion behavior in more detail.
- Design 028 defines output-family behavior.
- Design 029 defines execution and cancellation flow.
- Design 030 defines redaction and safety rules.
