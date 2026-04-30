# Comment-Preserving Config Edits

## Summary

Restish keeps `restish.json` as a strict, typed JSONC configuration file.
Command-driven config edits should preserve user-authored comments and nearby
formatting whenever possible instead of rewriting the entire file as plain JSON.

This design is intentionally narrow. It targets the `api connect`, `api set`,
and `api remove` workflows that modify object-shaped paths inside the config.

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

Restish uses a comment-preserving structural edit path for config mutations.
The exact implementation may be:

- a narrowly scoped patcher, or
- a CST-preserving library such as `hujson`

The important thing is the behavior contract, not which parsing library
provides it.

The editor must:

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

Two additional guarantees are now part of the design:

- line endings should be preserved when editing existing files
- edits must not coerce existing non-object values into objects silently just to
  satisfy a nested set operation

If an edit would require destructive shape repair, Restish should fail with a
clear error instead.

## Concurrency

Comment preservation is not enough by itself. Config edits must also be safe
under concurrent processes.

That means:

- cross-process file locking around read-modify-write
- reloading from disk while the lock is held
- atomic replace with fsync-before-rename discipline

Without that, a perfect patcher can still lose user edits.

## Notes

The command behavior is now:

- `api connect` preserves comments in an existing JSONC config when adding or
  replacing an API entry, including when setup expressions are supplied on the
  command line
- `api set` preserves comments when updating a supported config path
- `api remove` preserves comments around unaffected entries

The normal `Load` path remains the source of truth for validation and unknown
field rejection.

If the current bespoke patcher continues to accumulate correctness edge cases,
switching to a better CST-preserving implementation is an acceptable v2 change.
