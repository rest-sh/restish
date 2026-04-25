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

Resolution should be based on token classification, not vague best effort. A
practical command planner should:

1. parse global flags first
2. inspect the first remaining positional token
3. if it matches a built-in command, dispatch built-in handling
4. otherwise attempt generated API command-group resolution
5. otherwise attempt plugin command resolution
6. otherwise, if the token is a URL or configured API alias target, interpret it
   as the convenience GET form
7. otherwise emit a normal unknown-command or usage error

This ordering ensures user intent is explainable and stable.

## Resolution Edge Cases

The planner should explicitly define behavior for common ambiguous cases:

- a token that looks like both a URL scheme and a command name
- generated API operations whose names collide with plugin commands
- commands invoked after `--` where further parsing should stop
- built-ins that intentionally accept raw URL targets as later arguments

The default bias should favor explicit command names over convenience parsing.
Users can always write `get https://...` if they need to disambiguate.

## Global Flags And Environment

Global flags should have one consistent precedence model:

1. built-in defaults
2. environment variables
3. explicit CLI flags

Help text must only claim env-var support that actually exists.

Environment variables that accept repeated values should preserve their
documented input shape. For example, comma-separated `RSH_HEADER` entries from
v1 are parsed into multiple headers unless a future design deliberately replaces
that syntax and documents the migration.

Global flags should eventually be parsed into one structured runtime object so
every command sees the same resolved values instead of re-reading ad-hoc state.

That runtime object should be immutable from the point command execution begins.
Late mutation creates hard-to-debug differences between setup, request
execution, plugins, and output.

## Stdout And Stderr Contract

The channel split is strict:

- stdout is only for command result output
- stderr is for diagnostics, prompts, warnings, progress, and verbose logs

This is one of the most important scriptability guarantees in the product.

The same contract applies to helper commands and diagnostics-oriented commands.
Even a command like `cert` or `cache` should keep human-oriented warnings on
stderr and reserve stdout for the primary command result.

## Verbose Logging

Verbose mode should expose enough information to debug request flow without
breaking stdout contracts.

At `-v`, users should see at least:

- request method and URL
- selected headers with sensitive values redacted
- bounded redacted request and response bodies when available
- response protocol and status
- response headers
- pagination or retry progress when relevant

At higher verbosity, Restish may add:

- TLS/certificate details
- cache hit/miss detail
- plugin lifecycle traces

If help text advertises a verbosity level, the behavior should exist.

Verbose output should be additive. Higher verbosity may include more detail, but
it should not remove or reorder foundational information that users rely on
during troubleshooting.

## Redaction

Verbose logs must redact:

- sensitive headers
- sensitive query parameters
- token-like values returned in structured remote error bodies

Redaction rules are defined in design 030 and apply here.

## Exit Code Matrix

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

Recommended local categories are:

- usage/validation failure before execution
- local runtime/setup failure during execution
- cancellation/interruption

Whether usage failures collapse into `1` or use `2`, the mapping should remain
stable and documented once finalized.

## Output Versus Exit Status

Restish may still write the response body to stdout even if the final exit code
is HTTP-derived non-zero. This is a useful contract for inspecting failure
responses.

Two explicit flags modify the normal behavior:

- `--rsh-ignore-status-code` forces HTTP-derived exit status to zero
- `--rsh-silent` suppresses normal output

Silence affects output channels, not internal success/failure semantics.

Commands that naturally do not emit a body should still follow the same status
rules. "No stdout" does not imply "success" or "no diagnostics."

## Prompting

Prompts should be coordinated through the runtime and behave consistently across:

- auth prompts
- edit confirmations
- plugin prompts

Prompting rules:

- prefer terminal devices over stdin when stdin is occupied by piped data
- close terminal handles opened for a prompt, or centralize them in a runtime
  owner with a clear lifetime
- do not treat EOF as implicit confirmation for destructive actions
- defaults should be explicit in both UX text and behavior

When the session is non-interactive and a prompt would otherwise be required,
commands should fail with a clear message instead of hanging or guessing. This
is especially important for auth flows, edit confirmation, and plugin approval
prompts.

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

Long-running loops should check cancellation between units of work as well as at
the transport boundary. Waiting until the next network call to observe Ctrl-C is
not sufficient for pagination, streaming, or plugin-driven workflows.

## Diagnostic Categories

Diagnostics shown on stderr should conceptually fall into these categories:

- prompts
- warnings
- progress
- verbose transport detail
- local runtime failure explanation

Keeping these categories distinct helps both implementation and user
expectations.

In addition, diagnostics should preserve user-actionable context. Good stderr
messages explain:

- what failed
- at what stage it failed
- what target or subsystem was involved
- what the user can do next when there is an obvious action

This is especially valuable for config resolution, auth, TLS, and plugin
startup failures.

## Usage Error Model

Usage errors should be treated as a separate class from runtime failures.
Examples include:

- missing required arguments
- unknown flags
- invalid shorthand syntax detected before request execution
- unsupported shell names passed to `setup`

These should produce concise stderr output, reference help when useful, and
avoid verbose stack-like dumps in ordinary mode.

## Help And Completion Output

Help and shell-completion generation are also part of CLI behavior.

Design requirements:

- help text should be reproducible and not depend on ambient network state
- generated command help should reflect resolved API command structure for the
  current context
- completion output should be machine-oriented when emitted for shell
  integration, not decorated with human commentary

This keeps the CLI predictable both for people and for shell tooling.

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
