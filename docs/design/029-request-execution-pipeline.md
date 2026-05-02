# Request Execution Pipeline

## Summary

Restish v2 should have one explicit request pipeline shared by:

- generic HTTP verb commands
- generated API commands
- `edit`, `links`, and similar workflow commands
- command plugins delegating HTTP back to the host

The point of this design is not just code reuse. It is to guarantee that auth,
TLS, retries, caching, pagination, filtering, and output selection compose in a
single predictable order.

## Goals

- one mental model for every request-producing command
- no hidden "special case" HTTP paths that bypass auth, config, or output rules
- cancellation and timeouts flow from one root context
- streaming and paginated responses diverge only after shared request planning
- plugins can reuse the same pipeline instead of re-implementing it

## Non-Goals

- making every command literally call the same function signature
- forcing bounded and unbounded responses through identical buffering logic
- allowing plugins to replace the host execution pipeline wholesale

## High-Level Phases

Every request goes through these phases:

1. command resolution
2. request planning
3. request preparation
4. transport construction
5. HTTP execution
6. response classification
7. response normalization or stream handling
8. pagination or follow-up execution
9. filtering
10. output planning and rendering
11. status mapping and teardown

Those phases are intentionally stable even if the implementation is split across
multiple packages.

## 1. Command Resolution

By the time the request pipeline begins, Restish has already resolved whether
the invocation came from:

- a built-in command such as `get` or `post`
- a bare URL shorthand
- a generated API command
- a workflow command such as `edit`
- a command plugin asking the host to execute a request

Built-in commands always win over API short names and plugin commands. This
avoids accidental shadowing and keeps the command tree deterministic.

For generated OpenAPI commands, command resolution also identifies the cached
operation model entry from design 034. The pipeline should not need parser
library objects at execution time; it receives an already-normalized operation
plan containing the method, operation-relative path, parameter bindings, request
body media choices, no-auth flag, and help-derived metadata needed for
diagnostics.

## 2. Request Planning

Planning produces a typed request plan that should contain at least:

- method
- target URL or API-relative path
- selected API name, if any
- selected profile name, if any
- path arguments
- query/header overrides from flags
- request body source
- requested output/filter options
- pagination options
- caching/retry/timeout options
- execution mode hints such as "stream expected" or "edit workflow"

For generated commands, planning also resolves OpenAPI-specific inputs into the
same request-plan fields used by generic commands:

- path parameters become substituted URL path segments;
- query parameters become query entries using the operation serialization rule;
- header parameters become request headers unless reserved by OpenAPI;
- cookie parameters become the outbound Cookie header;
- request-body arguments and stdin become the ordinary body source;
- operation `security: []` becomes a no-auth execution hint.

After this step, execution should not care whether the request came from a
generated command or a generic HTTP verb.

The plan is where Restish resolves precedence. The design rule is:

1. built-in defaults
2. API registration defaults
3. selected profile values
4. environment-derived overrides
5. CLI flags and explicit command arguments

Silent fallback is not acceptable at this stage. If the user names a missing
profile or malformed API, planning should return an error rather than quietly
using defaults.

## 3. Request Preparation

Preparation takes the plan and builds the concrete outbound request.

That includes:

- resolving API-relative paths against the effective base URL
- applying path substitutions
- appending persistent and invocation query parameters
- appending persistent and invocation headers
- constructing the request body from shorthand, stdin, files, or raw input
- deriving content negotiation headers from the content registry

If auth hooks, credential headers, credential-looking query parameters, or
request-middleware plugins may add credentials and the request has no
API/profile cache namespace, cache bypass is decided before transport
construction. That prevents a prebuilt cache transport from storing
unnamespaced credentialed responses.

OpenAPI server resolution and `operation_base` handling should already have
produced an API-relative or absolute-safe path before preparation. Preparation
still owns final URL joining and path cleaning so relative same-origin escapes
from generated operations cannot bypass profile selection or request-pipeline
policy.

Preparation is also where override semantics become concrete. Explicit CLI
headers and environment-derived headers are user intent, not additional hints.
For semantically singular headers such as `Accept` and `Accept-Encoding`, a
manual value replaces the generated value rather than appending a second one.
Header names are compared case-insensitively for this purpose.

If the effective API or server target includes a path prefix, relative
operation paths resolve beneath that prefix. A target like
`https://api.example.com/v2` followed by `users` therefore requests
`/v2/users`, not `/users`.

Request bodies should be encoded into reusable bytes at preparation time. That
lets retries, verbose logging, command plugins, and reproduction diagnostics
observe the same prepared body without re-reading stdin or re-encoding a
structured value differently.

Preparation also applies request-time extensions in order:

1. built-in auth resolution
2. auth hook plugins
3. request middleware hooks

Each stage may mutate headers and, where explicitly supported, other request
fields. The contract should stay typed. "Stringly typed map with magic keys" is
the legacy implementation detail, not the desired long-term design.

## 4. Transport Construction

Transport construction is responsible for everything below the HTTP request
object:

- base TLS options
- custom CA roots
- client certificate or TLS signer selection
- proxy handling
- response cache
- retry transport

The conceptual stack is:

```text
request plan
  -> base transport
  -> retry layer
  -> cache layer
  -> http.Client
```

Two rules matter:

- cache sits above retry so cache hits never trigger retry behavior
- streaming requests must not rely on `http.Client.Timeout` for whole-lifecycle
  enforcement

Instead, Restish should use a request context deadline for "time to first
headers" and then clear or replace that deadline after headers arrive for
long-lived streams.

## 5. HTTP Execution

Execution sends the prepared request with the command context. That context must
derive from the root CLI context, which in turn should derive from
`signal.NotifyContext` so Ctrl-C works consistently.

Execution must never switch to `context.Background()` for convenience. Doing so
breaks:

- cancellation
- timeout enforcement
- plugin-session teardown
- embedders that rely on context lifetimes

## 6. Response Classification

When headers arrive, Restish classifies the response before deciding how to
consume the body.

The classification step answers:

- is this a bounded response or a stream
- should the original raw bytes be preserved
- does the media type imply image rendering or another specialized formatter
- should pagination be considered
- should response middleware run before or after normalization

The classification output is a plan, not direct side effects. That keeps the
next steps easy to reason about and test.

## 7. Response Normalization Or Stream Handling

For bounded responses, Restish reads and normalizes the full body into the
normalized response model from design 009.

For stream responses, Restish switches to the streaming model from design 012:

- SSE and NDJSON items are parsed incrementally
- per-item filtering and record rendering can begin immediately
- document-only formats may reject the request or require bounded completion

Immediate rendering is a client-side contract once records or events arrive.
It does not imply Restish can bypass buffering by an origin server, CDN, reverse
proxy, compression layer, or any response path that withholds headers/body until
the stream-shaped response is complete.

Response middleware hooks conceptually operate on normalized responses. If the
middleware system ever needs to support true stream interception, that should be
designed as a separate stream middleware contract rather than hidden inside the
bounded-response middleware path.

## 8. Pagination And Follow-Up Execution

Bounded GET responses may trigger follow-up work:

- automatic pagination via discovered `next` links or configured paths
- middleware-requested follow-up requests
- workflow-driven follow-up requests such as edit confirmation or links lookup

Pagination must reuse the same request context and transport policy as the first
page. That means auth, retry, TLS, and cache semantics stay consistent.
Pagination follow-up URLs must remain same-origin with the first page using the
shared origin helper: scheme, hostname, and effective port all match.

Follow-up workflow requests, including `edit` PUT/PATCH requests and
command-plugin delegated HTTP, must also reuse the prepared request options
from this pipeline. They must not create ad-hoc clients or bypass profile
headers, auth callbacks, query parameters, TLS settings, retry policy, or
middleware.

The paginator must also maintain its own safety state:

- visited-URL set for cycle detection
- max pages
- max items
- context cancellation checks between pages

The "logical response shape" chosen by design 028 is preserved here. The
paginator does not get to change a document request into a record request.

## 9. Filtering

Filtering happens after the logical response shape is known.

That means:

- a record filter may run per page item or per stream event
- a whole-collection filter requires collection semantics
- document output sees one logical merged structure
- record output sees one logical item at a time

The planner should decide before rendering whether the selected filter can run
incrementally. If not, Restish should collect automatically or return a clear
error.

## 10. Output Planning And Rendering

Rendering is the last stage, not a transport concern.

The renderer receives:

- the selected output family
- TTY metadata
- normalized or streamed values
- filter results
- response metadata needed by readable or plugin formatters

The rendering contract is:

- stdout is for command output only
- stderr is for prompts, warnings, progress, and diagnostics
- explicit `-o` always wins
- TTY defaults are human-oriented
- non-TTY defaults are machine-oriented

The document/record distinction from design 028 is the central rule here.

## 11. Status Mapping And Teardown

After rendering, Restish maps the final outcome to an exit result.

This includes:

- HTTP-family exit codes
- local usage or runtime failures
- cancellation handling
- `--rsh-ignore-status-code`
- `--rsh-silent`

Teardown must close anything that was opened for the request:

- response bodies
- formatter plugin sessions
- command plugin subprocesses
- TLS signer subprocesses tied to the request or transport lifetime

## Invariants

The implementation may be refactored, but these invariants should remain true:

- all request execution originates from a `CLI` instance
- all interactive I/O flows through `CLI`-owned handles or a typed prompter
- all cancellation flows from the root context
- all HTTP-producing features share the same auth/TLS/retry/cache semantics
- streaming is an alternate body-consumption path, not an alternate transport
- filtering never mutates the original normalized response in place
- plugins extend named seams in the pipeline; they do not replace the pipeline

## Observability

Verbose output should map to phases in this design:

- request planning summary
- outbound request line and headers
- response status and headers
- pagination progress
- retry attempts
- cache hits/misses
- plugin hook invocations when relevant

Sensitive fields must be redacted per design 030.

## Implementation Guidance

The current codebase spreads this pipeline across several packages. That is
acceptable as long as the phase ordering above remains recoverable.

Useful implementation seams are:

- request plan type
- transport builder
- response classifier
- pagination controller
- output planner

Those seams are also the best places to add tests, because they let the design
be validated without depending on full end-to-end CLI snapshots for every case.
