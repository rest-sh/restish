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

## Problem

Bulk resource management is intentionally larger than a single request:

- it needs a local working tree and metadata cache
- it coordinates many HTTP requests over time
- it compares local and remote state
- it wants its own subcommands and workflow-oriented help text

That makes it a poor fit for core generic HTTP commands and an awkward fit for
one-shot hook plugins.

## Design

The plugin advertises:

- `name: bulk`
- `hooks: ["command"]`

and contributes a single top-level command declaration:

- `bulk`

Restish does not parse flags beneath that command. Instead, it forwards the raw
argument vector to the plugin, and the plugin runs its own Cobra command tree.
That lets `restish-bulk` preserve the v1 UX shape without teaching the host
about every bulk-specific flag.

### Local Checkout Model

The plugin keeps the v1 on-disk layout:

- working files live in the current directory using URL-derived paths such as
  `users/alice.json`
- metadata lives in `.rshbulk/meta`
- cached remote copies live under `.rshbulk/<path>`

Each tracked file stores:

- the remote URL
- the last seen remote version
- the local version synced from the server
- conditional request metadata such as `Etag` and `Last-Modified`
- a hash of the normalized local JSON body

That metadata is enough to detect:

- local edits
- remote updates
- remote deletions
- new local files ready to be created remotely

### HTTP Delegation

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

### Output Ownership

Bulk commands mostly own their own human-oriented output:

- status summaries
- file lists
- diffs
- workflow help text

When a delegated HTTP call fails and the server response is worth showing, the
plugin can still emit a command-plugin `response` message so Restish formats the
error using the active output settings.

### Collection Discovery

`bulk init` and index refreshes expect a list response where each item exposes a
resource URL and version. Like v1, the plugin recognizes several common field
names:

- URL: `url`, `uri`, `self`, `link`
- version: `version`, `etag`, `last_modified`, `lastModified`, `modified`

If the list response does not include a direct URL, `--url-template` can build
one from item fields.

An optional `-f` filter runs before extraction so non-standard list responses
can be reshaped into the expected tuple.

## Workflow Semantics

The behavior intentionally tracks the v1 command set closely:

- `init` saves metadata and immediately performs the first pull
- `list` enumerates tracked files and can project each file through a shorthand
  filter
- `status` compares local edits with a freshly refreshed remote index
- `diff` shows unified diffs for local changes or `--remote` changes
- `reset` restores local files from the cached remote copy
- `pull` updates metadata and writes remote changes without overwriting local
  edits
- `push` applies local adds, edits, and deletes back to the server with
  conditional requests when possible

The plugin assumes client-generated identifiers plus `PUT` semantics for new
resources, the same tradeoff as v1.

## Why A Plugin Instead Of Core

Bulk is a strong fit for the command-plugin architecture because it needs all of
the following at once:

- a long-lived multi-step workflow
- dedicated subcommands
- local state on disk
- delegated HTTP through Restish

Shipping it as a plugin keeps the core CLI smaller while still validating that
the command-plugin transport is expressive enough for a real operator workflow.

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

## Notes

The implementation lives in
[`cmd/restish-bulk/main.go`](../../cmd/restish-bulk/main.go),
[`cmd/restish-bulk/client.go`](../../cmd/restish-bulk/client.go),
and
[`cmd/restish-bulk/bulk.go`](../../cmd/restish-bulk/bulk.go).

Integration coverage lives in
[`internal/cli/bulk_plugin_test.go`](../../internal/cli/bulk_plugin_test.go).
