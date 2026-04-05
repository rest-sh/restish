# CLI Architecture

## Summary

Restish v2 uses a central `CLI` struct as the main composition point for the
application. The goal is to keep the binary easy to test, easy to embed, and
easy to extend without relying on package-level mutable state.

## Problem

Restish v1 accumulated several patterns that made the code harder to reason
about and harder to evolve:

- global mutable state for core runtime objects
- tight coupling between configuration, command registration, and request flow
- configuration tooling that carried more complexity than Restish needed
- brittle startup behavior driven by argument sniffing and other implicit state

For v2, we wanted a simpler foundation that still preserved the strengths of
Restish as both a CLI and an embeddable Go library.

## Design

The main design choice was to make `CLI` the central runtime object.

`CLI` owns the active configuration, standard I/O handles, content registry,
spec loaders, hypermedia parsers, output formatters, and discovered plugins. A
new instance can be created directly in tests or by the main binary entrypoint.

This shape supports a few important properties:

- multiple independent CLI instances can exist in the same process
- tests can inject in-memory stdin/stdout/stderr and temp paths
- extension points are explicit registration methods instead of implicit global
  hooks
- the root command tree is assembled from a concrete runtime object rather than
  shared singleton state

The configuration layer is also intentionally narrow. v2 uses a typed config
model loaded from `restish.json`, with JSONC support so users can keep comments
in the file while the implementation still parses into ordinary Go structs.

## Alternatives Considered

### Keep the v1-style global architecture

This would have reduced migration work in the short term, but it would keep the
same testing and maintainability problems we wanted to leave behind.

### Use a large general-purpose configuration framework

We discussed keeping a broader configuration abstraction, but the v2 goal was
clarity rather than flexibility for rarely used formats and sources. A smaller,
typed config layer fits the product better.

### Split the runtime across several top-level managers

That can work, but in practice it tends to move the composition problem around
instead of simplifying it. Keeping a single top-level `CLI` object makes the
runtime easier to construct and inspect.

## Notes

The current implementation reflects this design directly:

- `internal/cli/cli.go` defines the central `CLI` type and its registration
  methods
- `internal/config/config.go` provides the typed JSONC-backed config layer
- plugin-provided formatters and loaders are registered onto the `CLI` during
  startup

One detail that evolved from the original v2 proposal is generated command
registration. The proposal discussed lazy API command stubs; the current code
registers generated API commands from cached specs at startup and avoids network
discovery during command tree construction.
