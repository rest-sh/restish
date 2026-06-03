# Historical Finding Classes

This reference distills prior Restish release findings into reusable QA
heuristics. Use it to choose targeted probes without depending on local
planning files.

## Triage Rules

- Credential leakage, data loss, corrupted config/cache state, or common
  generated-command breakage is release-blocking until verified fixed or
  explicitly deferred by the user.
- Medium issues are blockers when they affect common workflows, credentials,
  generated commands, config/cache integrity, release packaging, or code changed
  since the last release.
- Low issues become targeted smoke checks when a nearby subsystem changed.
- Help/example bugs are release risks when they teach users to send the wrong
  value, leak secret-looking values, or make common copy/paste examples fail.
- Rejected historical findings are still useful for documentation and
  "working as intended" checks.

## Historical Blocker And Near-Blocker Classes

### Auth And Redaction

- Optional anonymous OpenAPI security alternatives (`{}` plus a credential)
  must remain callable when optional env-backed credentials are unresolved.
  Ready credentials should still be preferred; required-auth operations should
  still fail on missing env.
- OpenAPI `mutualTLS` should be satisfied by resolved TLS client cert/key or TLS
  signer transport settings, then fail later with normal TLS diagnostics if the
  files/plugins are bad.
- Noninteractive `api connect` auth setup should reject unknown credential IDs,
  wrong-case credential IDs, unknown fields, and unused credential-scoped
  prompts.
- Verbose body redaction must catch `password`, `password2`,
  `confirm_password`, `client_secret`, `secret`, `token`, and similar fields,
  while preserving ordinary numeric fields such as `max_tokens` and
  `token_budget`.
- Network-error diagnostics must redact URL userinfo, credential-looking query
  params from the original URL, secrets added by `--rsh-query`/env/defaults,
  and generated query apiKey credentials in both the top-level error prefix and
  nested transport URL.
- Verbose request traces should redact configured credentials in headers,
  query, cookies, JSON bodies, form bodies, response headers, plugin stderr, and
  redirect diagnostics.

### Generated OpenAPI Commands

- Duplicate path/operation header parameters with the same HTTP header name,
  including case variants, should merge before generated flag creation and
  should not skip the whole operation.
- Generated fallback command names should remain short, stable, and traceable
  when `operationId` is absent or when `operation_base` is configured.
- Specs with `description` fields that look like `$ref` objects must not be
  treated as schema refs.
- OpenAPI extension effects (`x-cli-*`) should be visible and should not report
  nested effects for ignored entries.
- Stale local spec files should refresh raw spec and generated operation caches
  before generated command registration or execution.
- Concurrent `api connect` runs should preserve config entries and generated
  spec/operation caches for every successful connect.

### Schema Help And Examples

- OpenAPI 3.1 conditionals inside `allOf` should not hide direct object
  properties in generated help. Conditional required branches should render as
  useful constraints.
- Schema help should keep effective flag types consistent after invalid
  enum/type fallback, such as string enum values declared under an integer
  schema.
- Generated body examples should normalize string-valued boolean/integer/number
  defaults/examples to real JSON scalar values when unambiguous.
- JSON nulls in `enum`, `default`, `example`, and `const` should render as
  `null`, not Go-ish `<nil>`, and generated examples should avoid contradicting
  null-only constraints.
- Mixed-type root `enum`/`const` values should not make generated body examples
  contradict an object schema.
- Generated examples should redact real secret-like explicit examples but avoid
  over-redacting ordinary non-secret token counters or logprob fields.
- Generated shorthand examples should not turn URI/URL/URN strings into `@file`
  control syntax.
- Generated shorthand examples for arrays of objects should be shell-safe and
  structurally copyable; otherwise prefer JSON file input.
- Generated root help should handle absent, short, multiline Markdown, and
  README-length `info.description` values without overwhelming command help.

### Query, Path, And Pagination

- `deepObject` and `form` object query help should show syntax Restish accepts,
  and runtime serialization should match each OpenAPI style.
- JSON-content query parameters should encode one JSON query value matching the
  schema. Whole-array parent values should work; repeated array-of-object flags
  must not silently become arrays of JSON strings.
- String-or-array query flags should support multiple values or clearly point
  users to an accepted `--rsh-query` workaround.
- Placeholder/free-form query parameter names should either be modeled
  deliberately or documented as requiring `--rsh-query`.
- Path parameters with slashes, commas, percent signs, spaces, question marks,
  and already-escaped input should encode consistently. Pathful
  `--rsh-server` overrides are a known place to check for double encoding.
- Page-param pagination should honor the effective query parameter, preserve
  strict `items_path` behavior, respect max-page/max-item limits, and handle
  metadata filters.
- Link-header pagination should handle relative next URLs, quoted parameters
  with commas, malformed targets, unsupported schemes, and standalone `links`
  output.

### Media Types And Bodies

- Generated XML request-body `@file` input should send raw XML bytes with the
  declared XML media type.
- Generated NDJSON request-body `@file` input should send raw newline-delimited
  bytes with the declared NDJSON media type, not a JSON-encoded string.
- Wildcard or protocol-specific raw binary media types such as `*/*`,
  `application/*`, and `application/offset+octet-stream` should send raw bytes
  for `string format: binary` bodies while preserving the declared
  `Content-Type`.
- Multipart examples should be usable or should explain `@file` syntax clearly.
  Check missing files, unreadable files, literal `@`, repeated file parts,
  binary parts, and per-part encoding metadata.
- Operations advertising multiple request or response `content` entries should
  choose sensible defaults and respect `--rsh-content-type`, explicit `Accept`,
  vendor `+json`, XML/text alternatives, and strict provider expectations.

### Built-Ins, Plugins, And Docs

- Built-in commands should reject unsupported output formats clearly and should
  avoid advertising inherited output/filter flags they do not support.
- Unknown subcommands under command groups such as `cache` and `plugin` should
  fail, not silently exit 0.
- Explicit `--rsh-header` and `--rsh-query` should have documented precedence
  against env/default values and should not unexpectedly drop unrelated entries
  unless that is intentional.
- First-party command plugins should expose long help consistently for root
  `--help`, `-h`, `help`, and subcommand `--help` forms.
- Plugin protocol changes should remain additive or bump/check compatibility.
  Subprocesses that start must be waited on; timeout/error paths must close
  pipes and avoid goroutine/process leaks.
- Generated docs regions, README install guidance, plugin docs, release
  packaging docs, and Hugo site build should stay in sync with command behavior.

## Safe Repro Patterns

Use these shapes when turning a historical class into a release probe.

- Local fixture: write a small OpenAPI file under `t.TempDir()` in tests or
  under `${TMPDIR:-/tmp}` for manual QA; connect it with isolated config/cache.
- Public spec: connect into isolated config/cache with fake credentials, then
  inspect help, `doctor api`, `api auth inspect`, `--rsh-generate-body`, or
  request construction only.
- Request construction: use `--rsh-print H`, `--rsh-print B`, verbose output,
  `--rsh-server https://httpbin.org/anything`, or a localhost failure target.
- Redaction: intentionally use fake secrets with unique strings and grep the
  resulting stderr/stdout for leaks.
- Network diagnostics: use `127.0.0.1:9`, `--rsh-timeout 200ms`, and
  `--rsh-retry 0` so failures are quick and local.
- Provider drift: compare generated command output against generic requests or
  curl only with safe read-only endpoints.

## Issue Report Shape

For every new problem, capture:

- Candidate commit and platform.
- Exact commands and environment variables, with fake secrets only.
- Expected behavior.
- Actual behavior, including relevant stderr/stdout excerpts.
- Whether the issue is a release blocker.
- Severity and impact.
- Related historical class from this reference.
- Suggested fix direction, without patching code during QA.

## Residual Risk Notes

Call out:

- Checks skipped because a tool was unavailable, such as local GoReleaser.
- Network-dependent probes that were not run or were inconclusive.
- Public APIs that failed for provider/network reasons rather than Restish
  behavior.
- Dirty worktrees, non-candidate branches, or tests run against a commit that
  differs from the requested release candidate.
