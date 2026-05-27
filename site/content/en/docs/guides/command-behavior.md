---
title: Command Behavior
linkTitle: Command Behavior
weight: 85
description: Understand exit codes, redirects, diagnostics, stdout, stderr, and script-friendly behavior.
aliases:
  - /docs/recipes/follow-or-inspect-redirects/
  - /docs/recipes/decode-an-api-problem-response/
  - /docs/recipes/ignore-a-404-but-keep-the-body/
---

Restish is designed for terminals and scripts. Output channels, exit codes, and
verbose diagnostics are part of the interface.

## Exit Codes

Restish uses a compact exit-code policy:

| Result | Exit code | Notes |
| --- | --- | --- |
| Successful command, including final HTTP `2xx` responses | `0` | Redirects are followed before the final status is evaluated. |
| Final HTTP `3xx` response | `3` | Redirects are followed before the final status is evaluated. |
| Final HTTP `4xx` response | `4` | Restish still writes the response body before exiting non-zero. |
| Final HTTP `5xx` response | `5` | Restish still writes the response body before exiting non-zero. |
| Runtime failure | `1` | Network errors, TLS failures, config problems, auth failures, parse errors, formatter errors, and most plugin failures. |
| Usage error | `2` | Missing arguments, unknown commands, unknown flags, or invalid flag values before the request runs. |
| Interrupted with `Ctrl-C` / SIGINT | `130` | Matches the usual shell convention for interrupted processes. |
| Command plugin exit code | plugin-defined | Command plugins may return their own `0`-`255` exit code. |

Inspect an error body without failing the shell command:

{{< restish-example >}}
restish api.rest.sh/status/404 --rsh-ignore-status-code
{{< /restish-example >}}

Use the same flag for structured problem responses when the error document is
the data you need to inspect:

{{< restish-example >}}
restish api.rest.sh/problem --rsh-ignore-status-code
{{< /restish-example >}}

Add `-o json` when a script needs the error body as one JSON document.

## Stdout And Stderr

Response output goes to stdout. Diagnostics, verbose request/response metadata,
progress, and warnings go to stderr.

```bash
restish -v api.rest.sh/images/jpeg > dragonfly.jpg 2> headers.txt
```

## Verbose Mode

```bash
restish -v api.rest.sh/headers
restish -vv api.rest.sh/headers
```

`-v` shows request and response headers plus the resolved config path, profile,
auth state, input source, request body media type, response decode media type,
filter language, output format, plugin invocations, and a compact pipeline
summary when those details apply. `-vv` adds more TLS detail.

## Redirects

Use redirect fixtures to inspect behavior:

```bash
restish api.rest.sh/redirect/2 -v
restish 'api.rest.sh/redirect-to?url=/get&status_code=307' -v
```

When auth or custom headers are involved, use verbose mode to confirm what is
sent after redirects.

## Timeouts

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

Timeouts are useful in scripts where a hanging request is worse than a clear
failure.

## Silent Mode

Use `-S` when only the exit code matters:

```bash
restish -S api.rest.sh/status/204
```

## Related Pages

- [Global Flags](/docs/reference/global-flags/)
- [Retries and Caching](../retries-and-caching/)
- [Troubleshooting](../troubleshooting/)
- [Output Defaults](/docs/reference/output-defaults/)
