# CLI Architecture

## Summary

Restish v2 centers the application around a concrete `CLI` runtime object. That
runtime owns configuration, I/O handles, registries, plugin discovery results,
and root command tree assembly.

The aim is not just to reduce global state. The aim is to make the CLI
predictable enough that:

- tests can stand up isolated instances
- embedders can drive Restish without relying on process globals
- commands share one lifecycle and cancellation model
- extension points stay explicit and inspectable

## Goals

- one top-level runtime object per logical CLI instance
- no package-level mutable singleton required for normal operation
- all user-visible I/O owned by the `CLI`
- command registration built from runtime state rather than import side effects
- startup order that is deterministic and easy to reason about

## Non-Goals

- making every internal package depend directly on `CLI`
- forbidding helper packages from using their own small structs or pure
  functions
- freezing the exact current field layout forever

The design cares about boundaries and invariants more than exact struct layout.

## Core Runtime Model

Conceptually, `CLI` should own five groups of responsibility.

### 1. I/O

The runtime owns:

- stdin
- stdout
- stderr
- terminal capability knowledge derived from those streams
- prompting helpers
- editor/browser launch hooks where relevant

The invariant is simple: if a command needs user interaction or output, it
should go through the `CLI` runtime rather than reaching directly for
`os.Stdin`, `os.Stdout`, or `os.Stderr`.

`CLI.Run` mutates runtime state for the duration of an invocation: it loads
configuration, discovers plugins, registers generated commands, tracks request
closers, and may temporarily wrap stdout for buffered non-TTY output. External
embedders should create a fresh `CLI` per invocation, set streams and hooks
before calling `Run`, and avoid sharing one `CLI` concurrently across commands.
The test hooks exposed by `Hooks()` are intentionally narrow and are not a
general extension API.

### 2. Paths And Persistence

The runtime knows where user state lives:

- config path
- plugin directory
- cache directories
- token cache path

It does not need to implement every storage detail itself, but it owns the
resolved path configuration so tests and embedders can override it cleanly.

### 3. Registries

The runtime owns the active registries for:

- content types
- encodings
- output formatters
- hypermedia parsers
- auth handlers
- spec loaders

These registries are part of the runtime because they can be extended by:

- built-in initialization
- tests
- embedders
- plugins discovered at startup

### 4. Loaded State

The runtime owns loaded configuration and startup-discovered plugin metadata.
This is the stable, inspectable state that command registration depends on.

### 5. Execution Services

The runtime owns the services needed at command execution time:

- request planning/execution entry points
- context and signal handling
- formatter selection
- plugin session launching
- diagnostics and logging helpers

## Recommended Subsystem Split

The current implementation still has a fairly wide `CLI` struct. The intended
architecture is a `CLI` with named subsystem groupings such as:

- `IO`
- `Paths`
- `Registry`
- `Runtime`
- `TestHooks`

This is an architectural direction, not a requirement to land one giant
refactor immediately. The important design rule is that responsibilities should
be grouped cleanly enough that:

- tests can stub one concern without accidentally stubbing five others
- fields that exist only for tests do not become part of the public API surface
- runtime behavior is easier to document and inspect

## Lifecycle

Every `CLI` instance goes through the same lifecycle.

### 1. Construction

Construction should create a runtime with:

- injected or default I/O handles
- injected or default paths
- empty registries ready for built-ins and plugins
- no implicit network activity

Construction alone should be cheap and side-effect-light.

### 2. Built-In Registration

Built-in formatters, loaders, parsers, and commands are registered.

This stage is deterministic and local-only.

### 3. Config Load

The runtime loads persistent config from disk. Unknown fields and malformed
values are errors, not warnings.

### 4. Plugin Discovery

Plugins are discovered once during startup and registered into the appropriate
runtime registries and command catalog. Discovery should remain local-only and
must not execute arbitrary plugin behavior beyond manifest/command metadata
queries defined by the plugin design docs.

### 5. Command Tree Assembly

The root Cobra tree is assembled from:

- built-in commands
- generated API command groups based on cached specs
- command-plugin entry points

Network discovery should not be required at this phase. Startup command
construction must be offline-safe.

### 6. Dispatch

Command execution begins with a root context derived from signal-aware
cancellation. Every later context should derive from that root.

### 7. Teardown

On command completion or cancellation, the runtime ensures subprocesses,
formatter sessions, and other request-scoped resources are closed.

## Architectural Invariants

These rules should remain true even if the implementation evolves.

### Multiple Instances Are Supported

Two `CLI` instances in the same process should not share mutable operational
state by accident.

### No Direct Process I/O From Commands

Commands must not bypass the runtime's streams. This is required for:

- tests using buffers
- embedders using custom streams
- command plugins and prompts behaving consistently

### No Root-Time Network Dependency

Building the command tree must not require live network access. Cached specs are
acceptable; live discovery belongs to explicit execution paths such as
`api configure` or `api sync`.

### Context Propagation Is End-To-End

Everything long-lived should derive from the root command context:

- HTTP requests
- OAuth waits
- pagination loops
- plugin processes
- TLS signer sessions

Using `context.Background()` inside the runtime should be treated as a design
bug unless there is a very strong documented reason.

### Startup Discovery Is Additive

Plugin discovery and generated command registration extend the runtime; they do
not replace built-in behavior or bypass the core request pipeline.

## Command Registration Model

The root command tree has four sources:

1. built-in administrative and workflow commands
2. generic HTTP verb commands
3. generated API command groups
4. plugin-contributed command roots

Built-ins must take precedence over generated or plugin-provided names. This
avoids shadowing of core commands such as `api`, `cache`, or `setup`.

The command tree should remain inspectable from the runtime state. Restish
should not depend on hidden late-bound command injection that only happens after
the user starts typing.

## Embedding And Testing

The architecture is intentionally embedding-friendly.

Tests and embedders should be able to override:

- streams
- paths
- registries
- config source
- browser/editor openers
- plugin discovery paths

This is why central ownership matters: if a command bypasses the runtime, it
becomes much harder to test and much harder to trust as part of a shared
pipeline.

## Relationship To Other Designs

- Design 002 defines the persistent config model loaded into the runtime.
- Design 017 defines operator-facing command behavior and diagnostics.
- Design 018 defines plugin discovery and lifecycle rules.
- Design 029 defines the request execution pipeline used once a command is
  running.

## Refactor Direction

The review surfaced a few architectural pressures that this design now adopts as
future direction:

- split prompt handling behind a `Prompter` interface
- move global-flag parsing into one structured runtime step
- reduce public "testing only" runtime fields by hiding them behind test hooks
- replace ad-hoc boolean parameter lists with options structs in execution code

Those changes are encouraged because they move the implementation closer to the
runtime shape described here.
