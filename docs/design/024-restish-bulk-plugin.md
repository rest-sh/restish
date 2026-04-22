# `restish-bulk` Plugin

## Summary

`restish-bulk` brings the v1 bulk resource management workflow into v2 as a
first-party command plugin instead of a built-in command. It keeps the familiar
checkout model and Git-like verbs:

- `bulk init`
- `bulk list`
- `bulk status`
- `bulk diff`
- `bulk reset`
- `bulk pull`
- `bulk push`

The plugin owns the local checkout state on disk while delegating all HTTP back
to Restish through the command-plugin protocol.

## Goals

- preserve the v1 bulk workflow in v2 without bloating the core binary
- keep a local working-tree model for many remote resources
- let users inspect, edit, diff, reset, pull, and push resource state
- reuse Restish auth/TLS/retry/cache behavior instead of re-implementing it

## Non-Goals

- becoming a generic distributed version-control system
- supporting every possible remote collection shape without some operator help
- making the host CLI understand every bulk-specific subcommand and flag

## Why It Is A Command Plugin

Bulk resource management is intentionally larger than a single request:

- it needs a local working tree and metadata cache
- it coordinates many HTTP requests over time
- it compares local and remote state
- it wants its own subcommands and workflow-oriented help text

That makes it a poor fit for core generic HTTP commands and an awkward fit for
one-shot hook plugins.

## Manifest And Command Shape

The plugin advertises:

- `name: bulk`
- `hooks: ["command"]`

and contributes a single top-level command declaration:

- `bulk`

Restish does not parse flags beneath that command. Instead, it forwards the raw
argument vector to the plugin, and the plugin runs its own Cobra command tree.
That lets `restish-bulk` preserve the v1 UX shape without teaching the host
about every bulk-specific flag.

## Local State Model

The plugin keeps the v1 on-disk layout:

- working files live in the current directory using URL-derived paths such as
  `users/alice.json`
- metadata lives in `.rshbulk/meta`
- cached remote copies live under `.rshbulk/<path>`

The checkout is therefore a hybrid:

- user-visible working files
- hidden plugin-owned metadata and cache state

## Per-Resource Metadata

Each tracked file stores enough metadata to reconcile local and remote state:

- the remote URL
- the last seen remote version
- the local version synced from the server
- conditional request metadata such as `ETag` and `Last-Modified`
- a hash of the normalized local JSON body

That metadata is enough to detect:

- local edits
- remote updates
- remote deletions
- new local files ready to be created remotely

## HTTP Delegation

The plugin never constructs its own HTTP client. It emits command-plugin
`http-request` messages and waits for normalized `http-response` replies.

That means bulk operations automatically inherit normal Restish behavior:

- API/profile resolution
- auth handlers
- request middleware
- retries
- caching
- TLS signer support

`bulk push` and `bulk pull` therefore behave like ordinary Restish traffic
instead of a parallel implementation.

## Collection Discovery

`bulk init` and index refreshes expect a list response where each item exposes a
resource URL and version. Like v1, the plugin recognizes several common field
names:

- URL: `url`, `uri`, `self`, `link`
- version: `version`, `etag`, `last_modified`, `lastModified`, `modified`

If the list response does not include a direct URL, `--url-template` can build
one from item fields.

An optional `-f` filter runs before extraction so non-standard list responses
can be reshaped into the expected tuple.

This is the plugin's main extensibility seam for weird collection responses
without making bulk initialization entirely bespoke per API.

## Command Semantics

### `init`

- record the collection source and config
- fetch the initial remote index
- materialize the working tree
- save metadata for future status/pull/push operations

### `list`

- enumerate tracked files
- optionally project each file through a filter

### `status`

- compare working files against cached remote state
- refresh remote index when needed
- classify resources as modified, added, deleted, or out-of-date

### `diff`

- show unified diffs for local changes
- optionally compare against refreshed remote state with `--remote`

### `reset`

- restore working files from the cached remote copy
- discard local unsynced edits for the selected target

### `pull`

- refresh remote state
- update local cache and working files
- avoid overwriting local edits silently

### `push`

- detect local adds, edits, and deletes
- issue conditional updates when metadata allows
- update cache and metadata on success

## Conflict And Safety Model

Bulk is fundamentally a reconciliation workflow, so conflict handling matters.

The plugin should detect and surface at least:

- local edit versus remote update
- local delete versus remote update
- remote delete of a locally edited file
- conditional request failure due to stale metadata

The design bias is toward surfacing a conflict instead of overwriting one side
silently.

## Output Ownership

Bulk commands mostly own their own human-oriented output:

- status summaries
- file lists
- diffs
- workflow help text

When a delegated HTTP call fails and the server response is worth showing, the
plugin can still emit a command-plugin `response` message so Restish formats the
error using the active output settings.

## Resource Creation Model

The plugin assumes client-generated identifiers plus `PUT` semantics for new
resources, the same tradeoff as v1.

That is not universally correct for all APIs, but it keeps the workflow
practical for APIs whose resource identity can be derived deterministically from
the local checkout path or data model.

## Command-Plugin Impact

`restish-bulk` drove one concrete host behavior change: command-plugin commands
must bypass host-side flag parsing.

Without that, `restish bulk init --url-template=...` would fail before the
plugin ever saw the option. With host-side flag parsing disabled, the plugin can
own:

- nested subcommands
- workflow-specific flags
- `--help` output beneath the plugin command

This keeps the generic host protocol small while still enabling rich command
plugins.

## Alternatives Considered

### Keep Bulk Built Into The Core Binary

Would enlarge the core and make the plugin architecture less proven.

### Rebuild Bulk Around A Different Local-State Model

Possible, but preserving v1's operator mental model is a major reason this
plugin exists.

### Let Bulk Make HTTP Requests Directly

Would duplicate too much Restish behavior and break consistency.

## Relationship To Other Designs

- Design 020 defines the command-plugin session model this plugin uses.
- Design 029 defines the delegated HTTP pipeline bulk operations rely on.
- Design 031 treats restoring the v1 bulk workflow as part of the compatibility
  story for v2.
