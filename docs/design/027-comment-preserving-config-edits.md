# Comment-Preserving Config Edits

## Summary

Restish keeps `restish.json` as a strict, typed JSONC configuration file.
Command-driven config edits should preserve user-authored comments and nearby
formatting whenever possible instead of rewriting the entire file as plain JSON.

This design is intentionally narrow. It targets the `api configure`, `api set`,
and `api delete` workflows that modify object-shaped paths inside the config.

## Problem

JSONC support makes the config file much easier to explain and maintain, but the
original implementation only used JSONC on read. Any command that saved config
would re-marshal the whole file as JSON, which removed comments and often
surprised users who had annotated their config.

The biggest UX issues were:

- comments disappeared after routine commands like `api set`
- the warning did not help users who reasonably expected JSONC edits to round-trip
- users had to choose between scripted updates and preserving their notes

## Design

Restish uses a small JSONC-aware patcher in `internal/config` rather than a
general-purpose JSONC AST library.

The patcher:

- parses the existing file as JSONC while tracking byte ranges for object
  members and values
- preserves whitespace and comments outside the edited span
- supports replacing an existing value by path
- supports inserting missing object members, including creating intermediate
  objects for nested `api set` paths
- supports deleting object members cleanly
- validates the patched file by loading it through the normal strict config
  parser before writing it back

This keeps the implementation fast and small while covering the current Restish
config-management commands.

## Scope And Tradeoffs

The patcher is intentionally limited:

- it only supports object-member paths, not arbitrary array surgery
- it is optimized for Restish config mutations, not for general JSONC editing
- when there is no existing file, Restish still writes a normal formatted JSON
  config file

Those limits are acceptable because current config commands only need object
operations. Keeping the patcher narrow avoids introducing a larger dependency
or a slower full-fidelity syntax tree.

## Notes

The command behavior is now:

- `api configure` preserves comments in an existing JSONC config when adding or
  replacing an API entry
- `api set` preserves comments when updating a supported config path
- `api delete` preserves comments around unaffected entries

The normal `Load` path remains the source of truth for validation and unknown
field rejection.
