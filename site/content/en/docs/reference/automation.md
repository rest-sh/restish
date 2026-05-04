---
title: Automation Contract
linkTitle: Automation
weight: 24
description: stdout, stderr, exit-code, and flag contracts for scripts that call Restish.
---

Restish is scriptable when you keep primary data on stdout and diagnostics on
stderr. This page describes the contracts to rely on in CI, cron jobs, and
shell pipelines.

## Output Streams

Response data is written to stdout. Progress, verbose request/response details,
plugin diagnostics, migration notices, and warnings are written to stderr.

Use a machine-oriented format in scripts:

```bash
restish https://api.rest.sh/images -o json
restish https://api.rest.sh/images -o ndjson
restish https://api.rest.sh/images -f body.self -o lines
```

## Exit Codes

Restish exits `0` for success, `1` for runtime failures or non-2xx HTTP
statuses, `2` for usage errors such as missing arguments, and `130` for SIGINT.
HTTP error statuses still write the response body before returning non-zero.

When a script intentionally handles HTTP status itself, keep the body and force
a zero exit code:

```bash
restish https://api.rest.sh/status/404 --rsh-ignore-status-code -o json
```

## Quiet And Bounded Runs

Use `-S` when only the exit code matters:

```bash
restish -S https://api.rest.sh/status/204
```

Bound pagination and streaming explicitly in automation:

```bash
restish https://api.rest.sh/images --rsh-no-paginate
restish https://api.rest.sh/images --rsh-max-pages 3
restish https://api.rest.sh/images --rsh-max-items 100
restish https://api.rest.sh/events --rsh-max-items 10 -o ndjson
```

## Stable Request Flags

These flags are the usual script building blocks:

- `--rsh-ignore-status-code` keeps HTTP error bodies usable.
- `-S` suppresses output when the exit code is enough.
- `-o json`, `-o ndjson`, and `-o lines` avoid terminal-oriented formatting.
- `-r` writes raw response body bytes.
- `--rsh-no-paginate`, `--rsh-max-pages`, and `--rsh-max-items` bound
  collection and stream work.

## Related Pages

- [Global Flags](/docs/reference/global-flags/)
- [Command Behavior](/docs/reference/command-behavior/)
- [Requests](/docs/guides/requests/)
- [Troubleshooting](/docs/guides/troubleshooting/)
