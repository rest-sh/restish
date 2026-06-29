---
title: "Scripting REST APIs Without Fragile curl Loops"
linkTitle: "Scripting REST APIs Without Fragile curl Loops"
date: 2026-06-30
author: "Daniel Taylor"
description: API shell scripts fail in predictable ways — silent HTTP errors, hand-rolled pagination, no retries, unparseable output. Restish bakes the boring reliability into the CLI with stdout and stderr discipline, HTTP-aware exit codes, bounded pagination, and retry and timeout flags.
canonical_url: "https://rest.sh/blog/scripting-rest-apis-without-fragile-curl-loops/"
categories:
  - Automation
tags:
  - automation
  - cli
  - api
  - devops
---

Somewhere in your infrastructure there is a shell script with a loop like
this:

```bash
url="https://api.example.com/items?page=1"
while [ -n "$url" ]; do
  resp=$(curl -s "$url")
  echo "$resp" | jq -r '.items[].id'
  url=$(echo "$resp" | jq -r '.links.next // empty')
done
```

It works, mostly. It has also quietly accumulated a list of failure modes that
nobody will notice until a bad day:

- A `500` response exits with status `0`, so the error page flows into `jq`,
  which prints nothing, and the script "succeeds" with empty output.
- There is no retry, so one transient network blip fails the whole job — or
  worse, half of it.
- There is no timeout, so a hung connection hangs the cron job behind it.
- There is no page bound, so a pagination bug upstream turns the loop into an
  accidental load test.
- If `next` ever points somewhere unexpected — another host, an attacker-
  influenced URL in a response body — the loop follows it without a thought.

None of these are exotic. They are the standard tax on hand-rolling HTTP
plumbing in shell, paid one incident at a time. This post is about paying it
once, in the tool, instead.

Restish is a CLI for REST-ish HTTP APIs, and one of its design goals is being
boring in scripts: response data on stdout, diagnostics on stderr, exit codes
that mean something, and bounded loops by default. Here is what that looks
like for each failure mode above.

<div class="restish-blog-callout">
  <strong>Try it as you read.</strong> Runnable examples below use the browser
  preview against the public <code>api.rest.sh</code> API. Multi-command
  pipelines and failure demos are shown as fenced shell snippets.
</div>

## The Loop, Replaced

The whole script above is one command:

{{< restish-example >}}
restish api.rest.sh/images -f body.self -o lines
{{< /restish-example >}}

Restish recognizes the collection's `next` links and follows them
automatically, the `-f` filter selects one field from each item, and
`-o lines` prints one scalar per line for the next program in the pipe. The
pagination is bounded (25 pages by default, configurable with
`--rsh-max-pages`), and next-page URLs must stay on the same origin — scheme,
hostname, and effective port. A link that wanders off-origin stops the loop
with a warning instead of being followed.

When a script needs the whole logical collection at once — to count it, sort
it, or deduplicate it — collect first, then filter:

{{< restish-example >}}
restish api.rest.sh/images --rsh-collect -f '.body | length'
{{< /restish-example >}}

And when you want to bound the work explicitly, say so:

```bash
restish api.rest.sh/images --rsh-no-paginate     # exactly one page
restish api.rest.sh/images --rsh-max-pages 3     # at most three pages
restish api.rest.sh/images --rsh-max-items 100   # at most 100 items
```

## Exit Codes That Mean Something

A script's first question about an API call is "did it work?", and the answer
should not require parsing anything. Restish
[maps outcomes to exit codes](/docs/guides/automation/):

| Exit code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime failure (network, TLS, …) |
| 2 | Usage error (bad arguments) |
| 3 | Final HTTP `3xx` response |
| 4 | Final HTTP `4xx` response |
| 5 | Final HTTP `5xx` response |
| 130 | Interrupted (SIGINT) |

So ordinary shell control flow just works:

```bash
if ! restish -S api.rest.sh/status/204; then
  echo "health check failed" >&2
  exit 1
fi
```

`-S` suppresses output for the cases where the exit code is the whole answer.

And when the reaction depends on whose fault it was, the `4` versus `5` split
is already there — no body parsing required:

```bash
restish -S api.rest.sh/status/204
case $? in
  0) ;;                                            # healthy
  4) echo "client bug: fix the request" >&2 ;;
  5) echo "server error: retry later" >&2 ;;
  *) echo "transport or usage failure" >&2 ;;
esac
```

HTTP error statuses still write the response body to stdout before exiting
non-zero, so you can log what the API actually said. And when the script
handles HTTP status itself and wants the error body as data — a structured
problem response, say — keep the body and force a zero exit:

{{< restish-example >}}
restish api.rest.sh/problem --rsh-ignore-status-code
{{< /restish-example >}}

## stdout Is for Data, stderr Is for Commentary

Restish keeps the streams disciplined: selected response data goes to stdout;
progress, warnings, verbose request traces, and pagination notices go to
stderr. A pipeline never has to strain diagnostics out of its data, and `-v`
debugging does not corrupt the output a downstream step consumes.

Output formats make the data side explicit instead of terminal-shaped:

```bash
restish api.rest.sh/images -o json            # one complete JSON document
restish api.rest.sh/images -o ndjson          # one JSON record per line
restish api.rest.sh/images -f body.self -o lines   # one scalar per line
```

`json` suits a single document handed to one consumer, `ndjson` suits record
streams processed line by line, and `lines` suits scalar values feeding
`xargs`, `sort`, or a `while read` loop. There is no guessing about
prettification either: redirecting an unfiltered response writes the body
bytes unchanged, and any `-f` filter or explicit `-o` format renders
structured output for the next program.

## Retries and Timeouts Without a Wrapper

The retry-with-backoff wrapper function pasted between shell scripts can
retire. Bound the time, state the retries:

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 3s
restish 'api.rest.sh/flaky?failures=1&key=my-job' --rsh-retry 2
```

One detail matters more than it looks: automatic retries apply to `GET` and
`HEAD` by default, not to writes. Replaying a `POST` because the first attempt
timed out is how scripts double-charge customers. When a non-idempotent
endpoint genuinely tolerates replay, opting in is explicit:
`--rsh-retry-unsafe`.

## Putting It Together

A realistic CI step — check that every image resource an API lists is
actually reachable:

```bash
#!/usr/bin/env bash
set -euo pipefail

restish api.rest.sh/images -f body.self -o lines --rsh-max-items 50 |
while read -r path; do
  restish -S "api.rest.sh$path" --rsh-timeout 10s --rsh-retry 2 ||
    { echo "unreachable: $path" >&2; exit 1; }
done
```

Every fragile part of the opening loop is now someone else's tested code:
pagination is automatic and bounded, transient failures retry, hangs time
out, HTTP errors become exit codes, and stdout carries nothing but data.

This works the same against your own APIs — and if an API publishes OpenAPI,
you can [connect it](/docs/getting-started/connect-to-an-api/) and write the
script against generated commands with profiles and auth handled:

```bash
restish api connect example api.rest.sh
restish example list-images -f body.self -o lines
```

## Try It

Install Restish:

```bash
brew install restish
restish --version
```

Or with Go:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

Then run the replacement for the opening loop:

```bash
restish api.rest.sh/images -f body.self -o lines
```

Useful next stops:

- [Scripting and Automation](/docs/guides/automation/) is the durable
  reference for exit codes, streams, and the stable script flags.
- [Pagination and Links](/docs/guides/pagination/) covers limits, collect
  mode, and APIs that paginate without `next` links.
- [Retries and Caching](/docs/guides/retries-and-caching/) goes deeper on
  retry behavior.
- [Output](/docs/guides/output/) explains the format model and redirect
  semantics.

The fragile parts of API scripts were never the interesting parts. Move them
into the tool, and the script that is left is the part you actually meant to
write.
