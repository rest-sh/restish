# TOON Output Format

## Summary

Add a `toon` output formatter that renders a normalized response body as
[Token-Oriented Object Notation (TOON)](https://github.com/toon-format/spec), a
compact, indentation-based, schema-aware encoding of the JSON data model
designed to reduce token usage when structured data is fed to large language
models.

The motivating workflow is using Restish as the tool surface for an LLM agent
(in place of, or alongside, an MCP server — see design 023). In that setup every
response the agent reads, and every list it pages through, costs input tokens.
TOON's tabular encoding of uniform arrays of flat, primitive-valued objects
declares field names once and streams rows, which materially cuts token count
for the list/collection responses that dominate REST APIs.

This is an **output-only, opt-in, additive** change: a new value for
`-o/--rsh-output-format`. It introduces no new default behavior and no
compatibility risk to existing formats.

## Product Frame

**Problem:**
Agents that consume Restish output pay for every token of every response. JSON
repeats every object key on every array element, so collection responses are
token-expensive. There is no way to ask Restish for a more token-dense
representation of the same data.

**Goals:**

- Provide `-o toon` to render response bodies as TOON.
- Make the largest savings reachable where they exist: uniform arrays of flat,
  primitive-valued objects (list endpoints, or `-f` projections that produce
  such lists).
- Compose with existing filtering, pagination, and streaming, since the agent
  use case almost always projects before encoding.
- Track a pinned TOON spec version with official encode fixtures and focused
  formatter tests so output is a stable, auditable contract.

**Non-goals:**

- Accepting TOON as a **request body** content type (agent → API). That belongs
  to the content-type/encoding subsystem (design 003), is a larger lift, and has
  low marginal value because agents emit JSON request bodies well. Out of scope
  for v1.
- Making TOON a default. JSON remains the machine default.
- Guaranteeing token savings for arbitrary shapes (see Tradeoffs).
- Round-trip decoding of TOON inside Restish.

**Primary workflow:**

1. `restish api list-things -o toon` — render a collection response as TOON.
2. `restish api get-thing -f body.items -o toon` — project to the token-dense
   shape first, then encode. This is the recommended, highest-savings pattern.

**CLI surface:**

- Command/flag/config: new `toon` value for `-o/--rsh-output-format` and
  `RSH_OUTPUT_FORMAT`. Registered in `output.DefaultFormatters()`, so it appears
  automatically in `root.go` flag help and in error messages via
  `FormatterNames`.
- Defaults: unchanged. `auto` in a TTY, `json` when piped.
- TTY behavior: optional syntax highlighting via the existing `highlight` path;
  otherwise plain.
- Non-TTY behavior: deterministic, color-free TOON to stdout.
- Output: encodes `resp.Body` only, matching `-o json`/`gron` (status and
  headers are not part of structured body output).
- Errors: encoder failures return through the existing `Formatter` error path;
  the command surfaces them like any other format error.

**Compatibility:**

- Existing behavior preserved: no change to any existing formatter, flag
  default, or output contract.
- Intentional break: none.

**Validation:**

- Tests: formatter tests plus official TOON encode fixtures under
  `internal/output/testdata` covering objects, flat primitive-valued object
  arrays (tabular form), non-uniform arrays (expanded list form), nested
  structures, primitive arrays, empty arrays/objects, null, booleans, numeric
  normalization, and strings requiring quoting. Behavior tests verify that
  `-o toon` is selectable and composes with `-f`.
- Docs/help: `site/` output docs gain a TOON section; flag help updates
  automatically.
- Manual checks: confirm token reduction on a representative list response vs
  minified JSON, and confirm parse-back fidelity against the reference decoder.

## Architecture Fit

Output formatting is a clean, first-class extension point. A formatter
implements `output.Formatter` (`internal/output/format.go`) and is registered in
`DefaultFormatters()`. Library-backed formatters are already house style — the
CBOR formatter wraps `github.com/fxamacker/cbor/v2`.

The new `TOONFormatter` will:

- Implement `Formatter.Format` to encode `resp.Body`.
- Implement `ValueFormatter.FormatValue` so it works for filtered values and
  paginated/streamed item values — the cases where TOON's tabular form is most
  valuable.

It will **not** implement `ValueStream`/`FramedValueStreamFormatter` in v1.
True tabular TOON needs the full array in hand to emit one header and a known
row count (`key[N]{...}`), which is incompatible with incremental row-at-a-time
streaming. Streamed output falls back to per-value encoding, which is documented
rather than silently degraded.

## Key Decision: Hand-Roll the Encoder (No New Dependency)

**Decision: implement a self-contained, output-only TOON encoder in
`internal/output`, pinned to spec v3.3, locked with official encode fixtures
and focused formatter tests.**

Rationale:

- The only official Go implementation (`github.com/toon-format/toon-go`) is
  MIT-licensed but very early: no tagged release, single author, ~9 commits, a
  module pseudo-version only. That is a different maturity tier from the CBOR
  precedent (`github.com/fxamacker/cbor`, mature and widely used) and a
  realistic merge blocker for a security-sensitive CLI's output surface.
- The encoder is output-only and bounded: objects, inline primitive arrays,
  tabular flat primitive-valued object arrays, expanded-list fallback, plus the
  quoting and number-canonicalization rules. The spec is precise enough to
  implement directly.
- Owning the code keeps the dependency footprint flat and makes the output
  contract fully auditable in-repo.

Cost and mitigation:

- We own spec-conformance and any future spec drift. Mitigation: pin to spec
  v3.3 explicitly in code comments and the design doc, and lock observable
  output with official encode fixtures and focused formatter tests so behavior
  changes are deliberate and reviewable.

Alternative considered — vendor `toon-format/toon-go`: less code to own and
tracks the spec, but the dependency's immaturity outweighs that for an upstream
contribution. Revisit if/when the library reaches a stable tagged release.

## Token Benchmark

Tokens counted with `tiktoken` `o200k_base` (GPT-4o-class), rendering each
fixture through the real Restish formatters. The four fixtures:

- **Uniform 5** — a real `api.rest.sh/images` response: 5 records, each a flat
  object with three short string fields (`format`, `name`, `self`). The smallest
  realistic list; shows that declaring the header once pays off even at a few
  rows.
- **Uniform 100** — 100 synthetic user records, each a flat object with mixed
  scalar types: `id` (int), `name`/`email`/`role` (strings), `active` (bool),
  `age` (int). The canonical list-endpoint shape at scale: TOON names the six
  fields once, then streams 100 comma-separated rows.
- **Nested 40** — 40 synthetic objects that are deliberately *not* tabular: each
  has a nested `meta` object (a `created` timestamp, a variable-length `tags`
  array, an `owner` sub-object) and a `settings` object with a nested
  `thresholds` map. Non-uniform shape plus nested values force TOON's
  expanded-list form, where per-line indentation is the dominant cost.
- **String-heavy** — the real `sedric-labs deployments-list-for-model` response:
  3 deployment objects, each dominated by a ~1.5 KB embedded LLM `prompt` string
  plus nested per-version `configuration` maps. Represents payloads where one
  large leaf string dwarfs the structure, so format choice barely matters.

| Format | Uniform 5 | Uniform 100 | Nested 40 | String-heavy |
| --- | --: | --: | --: | --: |
| **toon** | **71** | **1,689** | **3,343** | **2,048** |
| json (compact) | 96 | 2,903 | 2,762 | 1,978 |
| json (pretty, `-o json`) | 155 | 5,002 | 4,842 | 2,184 |
| yaml | 108 | 3,700 | 3,360 | 2,009 |
| ndjson | 98 | 3,000 | 2,800 | 2,191 |
| gron | 196 | 6,403 | 6,783 | 2,560 |
| table ¹ | 141 | 2,743 | 1,556 | 686 |
| lines ² | n/a | n/a | n/a | n/a |
| cbor ³ | 288 B | 8,311 B | 6,359 B | 8,121 B |

TOON token savings vs each text format (negative = TOON uses more):

| vs | Uniform 5 | Uniform 100 | Nested 40 | String-heavy |
| --- | --: | --: | --: | --: |
| json (compact) | +26% | +42% | −21% | −4% |
| json (pretty) | +54% | +66% | +31% | +6% |
| yaml | +34% | +54% | +1% | −2% |
| ndjson | +28% | +44% | −19% | +7% |
| gron | +64% | +74% | +51% | +20% |
| table ¹ | +50% | +38% | −115% | −199% |

¹ `table` truncates long cell values (~40 chars) and flattens nested objects, so
its low counts on the nested and string-heavy shapes reflect **dropped data**,
not efficiency. It is also human-only ("not stable machine parsing") and loses
type fidelity (`null` vs `"null"`, `3` vs `"3"` render identically).
² `lines` only renders arrays of scalars; it rejects record/object arrays.
³ `cbor` is binary (bytes shown, not tokens) and is not consumable as LLM text.

`cl100k_base` (GPT-4) agrees within ~1 point on the uniform cases. Takeaways:

- On uniform record collections — the shape of REST list endpoints — TOON beats
  every other text format, and the win **grows with row count** (+26% → +42% vs
  compact JSON from 5 to 100 rows).
- On nested/irregular or string-dominated payloads, compact JSON and ndjson are
  slightly smaller than TOON (its per-line indentation outweighs the brace/quote
  savings). This is why the format is opt-in and the docs steer users to project
  to a uniform list first.
- TOON beats pretty JSON, YAML, and gron on every shape tested.

## Tradeoffs / Honest Limitations

- **Savings are shape-dependent.** TOON wins big on uniform arrays of flat,
  primitive-valued objects, scaling with row count. For deeply nested or
  irregular JSON, compact JSON can be *smaller* than TOON (its indentation
  overhead exceeds the brace/quote savings). Docs state this and steer users to
  `-f` projection first.
- **Filtering is the bigger lever.** `-f` (jq/shorthand) drops unneeded fields
  entirely and usually saves more tokens than re-encoding. TOON is complementary,
  not a replacement; the docs lead with `-f ... -o toon`.
- **Model comprehension is the real open risk.** Token savings only pay off if
  the agent parses TOON as reliably as JSON. The format's own benchmarks claim
  parity-or-better on tabular retrieval; we present this as a measured claim, not
  a guarantee, and recommend users validate on their own model/data.

## Future Work

- **Tabular output for paginated collections.** Streamed items encode one
  document each, so a large paginated list never becomes a single table. Verified
  that `--rsh-collect -o toon` materializes the full array first and produces one
  table; documented as the recommended path for large list endpoints. True
  per-row streaming is spec-constrained (the `[N]` header needs the row count
  upfront).
- **Delimiter selection.** Tab/pipe delimiters can avoid quoting comma-bearing
  values; revisit only if users hit it.

### Rejected: narrower indentation

Measured 1-space indentation: it saves ~6% tokens on tabular output (uniform-100:
1689 → 1589 with `o200k_base`) but **breaks the expanded-list form**. The dash
list-item marker `- ` is two characters wide and is deliberately coupled to the
two-space indent so a list item's continuation fields align under its first
field. At one-space indent that alignment is off by a column, producing
structurally ambiguous output. The tabular gain is not worth correctness loss on
the non-uniform path, so indentation stays at the spec default of two.

## Open Questions

- Should `-o toon` emit a trailing newline (json/gron do) — yes, for consistency
  with other structured formatters, even though the TOON document itself ends
  without one.

## References

- TOON spec: https://github.com/toon-format/spec
- Official Go implementation: https://github.com/toon-format/toon-go
- Related: design 003 (content types), 009 (response normalization and output),
  010 (filtering and projection), 023 (restish-mcp plugin).
