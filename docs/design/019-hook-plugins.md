# Hook Plugins

## Summary

Hook plugins are usually short-lived subprocesses that handle a single
extension point per invocation. Restish writes one CBOR data item to the
plugin's stdin, reads either one CBOR reply or raw formatter output, and then
the plugin exits.

Formatter plugins are the one exception: they run as a short formatter session
so they can maintain output state across paginated and event-stream renders.

This model is used for:

- `auth`
- `request-middleware`
- `response-middleware`
- `loader`
- `formatter`

## Problem

Some extensions need access to Restish's request and response model but do not
need to own an entire command lifecycle. Typical examples include:

- adding auth headers from external credentials
- injecting request headers
- mutating or suppressing normalized responses
- teaching Restish about a new API description content type
- adding a new output format

Those use cases should stay simple to implement and safe to reason about. Most
of them do not need a persistent session or multi-message protocol.

Formatter plugins turned out to be the boundary case. One-shot formatter hooks
work for ordinary responses, but they break down for outputs such as CSV when
Restish is streaming items from pagination or NDJSON/SSE:

- the formatter may need to emit a header once, then many records
- the host should not have to buffer the entire stream just to satisfy a plugin
- one-shot formatter invocations per item create inconsistent behavior between
  collected and streamed output

## Design

The default hook-plugin contract is intentionally one-shot:

1. Restish selects discovered plugins whose manifest declares a hook.
2. It builds a hook-specific input message.
3. It starts the plugin executable.
4. It writes one CBOR data item to stdin.
5. It reads one result from stdout.
6. The plugin exits.

Most hooks use the generic request/reply helper in
[`internal/plugin/hook.go`](../../internal/plugin/hook.go),
which enforces a 30 second timeout and requires exit status 0.

### Auth Hook

The `auth` hook runs during request preparation. Restish sends:

- API name
- profile name
- auth params from config
- the outbound request method, URI, and headers

The plugin replies with request updates, typically header changes. The reply is
merged into the outbound request before it is sent.

This keeps external auth providers inside the same request-hook stage as
built-in auth handlers.

### Request Middleware Hook

The `request-middleware` hook receives the prepared request and can return
header updates before the request goes out. The implementation currently only
applies header changes, even if a plugin also returns method or URI fields.

That is an intentional practical boundary: middleware runs after Restish has
already prepared the request object and transport options.

### Response Middleware Hook

The `response-middleware` hook receives:

- the original request metadata
- the normalized response status, headers, and body

The plugin may:

- return `{"drop": true}` to suppress output entirely
- return `{"follow": {...}}` to tell Restish to make a follow-up request
- return `{"response": {...}}` to replace body fields or merge headers

The follow-up path is especially important: the plugin asks Restish to issue
the request, so auth, retries, TLS, and other core behaviors still apply.

**Known limitation:** the `follow` message only carries `method` and `uri`.
There is no way to attach a request body or additional headers to the
follow-up request. Follow is therefore only appropriate for bodyless
redirects (e.g. redirecting a GET to a different endpoint). If a plugin
needs to issue a follow-up request with a body, it should use the command
plugin protocol instead.

### Loader Hook

Loader plugins let Restish recognize non-built-in spec formats. A loader plugin
declares `loader_content_types` in its manifest, which Restish turns into
registered `spec.Loader` instances at startup.

When a matching content type is detected, Restish sends the raw body to the
plugin and expects back:

- `body`: an OpenAPI document as bytes or a string
- optional `content_type`

The plugin does not return Restish's internal API model directly. Instead, it
returns an OpenAPI document, and Restish parses that through the normal
OpenAPI-loading path. That keeps generated commands aligned with the rest of
the system.

### Formatter Hook

Formatter plugins declare `formatter_names` in the manifest. Each declared name
becomes available through `-o <name>`.

Formatter hooks are slightly different from the other hook types because stdout
is treated as raw formatted bytes rather than a CBOR reply.

### Formatter Hook Session

Formatter plugins receive a stream of `formatter` messages on stdin. The host
starts one plugin process, then sends:

1. `formatter` with `event: "start"`
2. zero or more `formatter` messages with `event: "item"`
3. one final `formatter` message with `event: "end"`

The `start` message carries response metadata (`proto`, `status`, `headers`,
`links`) and may also include a full `body` for ordinary non-streaming
responses.

Each `item` message carries one body/sub-value to render. This is what lets the
same formatter handle:

- a normal one-shot body by reading `start.response.body`
- paginated output by rendering each `item`
- NDJSON/SSE output by rendering each `item`

The plugin writes raw formatted bytes to stdout as values arrive and exits after
the `end` message or EOF on stdin.

This model is intentionally narrow:

- only formatter hooks get the long-lived stream session
- the host still owns pagination, filtering, and event parsing
- plugins do not get to drive the HTTP loop or ask for additional pages

The practical goal is to support stateful output modes such as CSV without
forcing Restish to invent formatter-specific buffering behavior in the core.

## Alternatives Considered

### Keep these extension points in-process only

That is simpler for the core binary, but it forces every custom formatter,
loader, or auth hook to ship as a custom Restish build.

### Let loader plugins return an internal command model directly

That would expose too much of Restish's internal representation. Requiring
loader plugins to hand back OpenAPI keeps the seam smaller and more stable.

### Keep formatter hooks one-shot forever

That keeps the protocol simpler, but it forces the host to choose between:

- buffering entire paginated or streaming result sets before formatting, or
- calling formatter plugins once per item and accepting inconsistent behavior

Neither is a good fit for stateful formats such as CSV.

### Use raw JSON everywhere instead of CBOR

JSON would be easier to inspect manually, but CBOR is more efficient for binary
payloads, maps naturally to byte-oriented plugin messages, and is self-delimiting
over a stream.

## Notes

The current implementation is centered in:

- `internal/cli/hooks.go` for auth and middleware integration
- `internal/spec/plugin_loader.go` for loader-backed spec loading
- `internal/output/plugin_formatter.go` for formatter-backed output
- `internal/cli/plugin_hook_test.go` for end-to-end examples of every hook type

One detail worth preserving is that hook plugins are called from well-defined
seams in the existing pipeline instead of becoming alternate pipelines. Even
with streaming formatter sessions, the goal is extension, not bypass.
