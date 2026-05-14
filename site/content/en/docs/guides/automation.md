---
title: Scripting and Automation
linkTitle: Automation
weight: 88
description: Write scripts around Restish using stable stdout, stderr, exit-code, format, retry, timeout, and pagination behavior.
aliases:
  - /docs/reference/automation/
---

Restish is scriptable when response data stays on stdout, diagnostics stay on
stderr, and long-running work is bounded. Use this guide for CI, cron jobs,
shell loops, and small automation around APIs.

## Output Streams

Selected output is written to stdout. Progress, verbose request/response
details, plugin diagnostics, migration notices, and warnings are written to
stderr. Redirecting or piping stdout changes `--rsh-print=auto` from an
interactive response transcript to raw body bytes for unfiltered responses, or
to pretty rendered output when a filter, metadata shortcut, collection, or
explicit output format is selected.

Use explicit output controls in scripts. `json` is best for one complete
document, `ndjson` for record streams, and `lines` for scalar values. Add
`--rsh-print=b` when compact JSON is useful:

```bash
restish api.rest.sh/images -o json
restish api.rest.sh/images --rsh-print=b -o json
restish api.rest.sh/images -o ndjson
restish api.rest.sh/images -f body.self -o lines
```

## Exit Codes

Restish exits `0` for success, `1` for runtime failures or non-2xx HTTP
statuses, `2` for usage errors such as missing arguments, and `130` for SIGINT.
HTTP error statuses still write the response body before returning non-zero.

When a script intentionally handles HTTP status itself, keep the body and force
a zero exit code:

```bash
restish api.rest.sh/status/404 --rsh-ignore-status-code -o json
```

Structured problem responses decode like JSON-family responses:

{{< restish-example >}}
restish api.rest.sh/problem --rsh-ignore-status-code
{{< /restish-example >}}

## Quiet And Bounded Runs

Use `-S` when only the exit code matters:

```bash
restish -S api.rest.sh/status/204
```

Bound pagination and streaming explicitly in automation:

```bash
restish api.rest.sh/images --rsh-no-paginate
restish api.rest.sh/images --rsh-max-pages 3
restish api.rest.sh/images --rsh-max-items 100
restish api.rest.sh/events --rsh-max-items 10 -o ndjson
```

Collect before filtering when the script needs the whole logical collection,
such as a count, sort, or unique operation:

{{< restish-example >}}
restish api.rest.sh/images --rsh-collect -f '.body | length'
{{< /restish-example >}}

## Retries And Timeouts

Retries are useful for transient failures, but every script should still have
a clear bound. Give slow services enough time when the delay is normal, and use
shorter timeouts when testing failure handling:

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 3s
restish 'api.rest.sh/flaky?failures=1&key=script' --rsh-retry 2
```

Automatic retries apply to `GET` and `HEAD` by default. Add
`--rsh-retry-unsafe` only when a non-idempotent method can tolerate replay.

## Stable Request Flags

These flags are the usual script building blocks:

- `--rsh-ignore-status-code` keeps HTTP error bodies usable.
- `-S` suppresses output when the exit code is enough.
- `-o json`, `-o ndjson`, and `-o lines` avoid terminal-oriented formatting.
- `--rsh-print=b` requests compact rendered output.
- plain redirection writes unfiltered response body bytes unchanged.
- `--rsh-no-paginate`, `--rsh-max-pages`, and `--rsh-max-items` bound
  collection and stream work.
- `--rsh-timeout`, `--rsh-retry`, and `--rsh-retry-max-wait` keep network work
  predictable.

## Related Pages

- [Global Flags](/docs/reference/global-flags/)
- [Command Behavior](/docs/guides/command-behavior/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [Requests](/docs/guides/requests/)
- [Troubleshooting](/docs/guides/troubleshooting/)
