# Comment-Preserving Config Edits

## Summary

Restish keeps `restish.json` as a strict, typed JSONC configuration file.
Command-driven config edits should preserve user-authored comments and nearby
formatting whenever possible instead of rewriting the entire file as plain JSON.

This design targets command-driven mutations inside the config: `api connect`,
`api set`, `api remove`, `api auth add/remove`, `config set`, and theme
updates. `config edit` is different: it lets the user's editor modify the whole
file, then reloads and validates the result before invalidating affected
caches.

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
- supports shorthand patch operations for `config set`, `api set`, and
  `api connect` setup expressions
- preserves recursive object merge, scalar replacement, whole-array set, array
  append with `[]`, array insertion with `[^index]`, field and item deletion
  with `undefined`, and shorthand `^` swap/move semantics
- roots `api set <name>` operations at `apis.<name>`, including both sides of a
  `^` operation
- validates the final patched config structurally and semantically before
  writing it back

This keeps command-line config edits aligned with request-body shorthand while
preserving the safety properties of the config file.

## Scope And Tradeoffs

The patcher is optimized for Restish config mutations, not for general JSONC
editing. When there is no existing file, Restish still starts from an empty
config object and writes valid JSONC-compatible JSON.

Two additional guarantees are now part of the design:

- line endings should be preserved when editing existing files
- edits must not be written when shorthand parsing, structural validation,
  typed decode, semantic validation, or runtime validation fails

No partial write is allowed. If validation fails, the original config bytes stay
on disk and the user receives the collected errors where practical.

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
  command line, then prints the absolute config file path it wrote
- `api set` preserves comments while applying API-scoped shorthand patches
- `api auth add/remove` preserves comments around profile and credential
  entries that it does not change
- `api remove` preserves comments around unaffected entries while also clearing
  local cache/auth state owned by the removed API
- `config set` preserves comments while applying full-config shorthand patches
- `config theme set` preserves comments while replacing the `theme` object
- `config edit` does not patch individual spans, but it validates the edited
  file through the strict config loader and invalidates spec caches when
  relevant API fields changed, then prints the absolute config file path it
  wrote

The normal `Load` and validation paths remain the source of truth for typed
decode, semantic validation, and unknown field rejection.

If the current bespoke patcher continues to accumulate correctness edge cases,
switching to a better CST-preserving implementation is an acceptable v2 change.
