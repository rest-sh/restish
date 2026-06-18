# AGENTS.md

## Commands

```bash
# Build the main CLI
go build ./cmd/restish

# Build plugins
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11

# Regenerate docs-site regions derived from CLI help and flags.
# Run --write after changing CLI surface (commands, flags, help text);
# CI fails on stale regions via --check.
go run ./cmd/restish-docgen --write
go run ./cmd/restish-docgen --check

# Validate the docs site after changes under site/ (quick check)
hugo --source site --quiet

# Full CI parity for the docs job (npm ci needed once; social-images parses
# content front matter and can fail independently of Hugo)
npm --prefix site ci
npm --prefix site run social-images
hugo --source site --quiet --gc --minify --cacheDir /tmp/hugo_cache

# Validate docs examples and links (CI runs both)
scripts/check-doc-examples.rb
scripts/check-doc-links.rb
```

### Verification Ladder

Run the narrowest meaningful test first, then widen:

```bash
go test ./internal/cli/...       # or another touched package
go test ./...                    # fast suite; default development loop
go test -tags=integration ./...  # full suite, including slow plugin and CLI
                                 # integration tests; run before commits that
                                 # touch CLI/plugin behavior and final commits
go test -race ./...              # when touching concurrency, streaming,
                                 # subprocess lifecycle, or shared buffers
```

CI requires all of: `go test ./...`, `go test -tags=integration ./...`, building all five binaries above, `go run ./cmd/restish-docgen --check`, `scripts/check-doc-links.rb`, `scripts/check-doc-examples.rb`, and the docs-site build (`npm --prefix site ci`, `npm --prefix site run social-images`, then Hugo). A weekly job also executes every docs `restish-example` live against `api.rest.sh`.

### Manual Smoke Test

```bash
go build -o /tmp/restish ./cmd/restish
/tmp/restish api.rest.sh/types   # safe public test API
```

## Architecture

Restish is a CLI for interacting with REST-ish HTTP APIs. It generates commands from OpenAPI 3.x specs and supports generic HTTP verbs, content negotiation, authentication, pagination, caching, filtering, and plugins.

### Central `CLI` Struct

The core design is a `CLI` struct in `internal/cli/cli.go` that owns all state — I/O handles, config, content registry, spec loaders, link parsers, formatters, and plugins. Tests instantiate `CLI` directly with `bytes.Buffer` for I/O and `httptest.Server` for HTTP.

**Entry point**: `cmd/restish/main.go` creates a `CLI` and calls `Run(os.Args)`.

### Package Map

- `internal/cli` — command tree, flags, generated commands, pagination, help; the `CLI` struct lives here
- `internal/request` — HTTP execution: retries, redirects, TLS, redaction
- `internal/spec`, `internal/openapi` — spec discovery/loading/caching and OpenAPI → command generation
- `internal/output` — response formatters (JSON, YAML, tables, TOON, …); regression fixtures in `testdata/`
- `auth`, `config` — public Go API for Restish config structs/loading and OAuth token cache sharing
- `internal/auth`, `internal/secrets` — bundled auth handler implementations; credential-recognition allow-lists
- `internal/config` — v1 migration, embedder read helpers, comment-preserving JSONC edits
- `internal/fileutil` — internal file locking and atomic-write helpers
- `internal/content`, `internal/input`, `internal/filter`, `internal/hypermedia` — content negotiation, shorthand request bodies, shorthand/jq filtering, link parsing
- `internal/cache` — size-bounded disk cache for specs and responses
- `internal/plugin`, `internal/procutil` — plugin discovery/manifests; subprocess lifecycle
- `plugin/` (top level) — public plugin API contract; wire compatibility is a public promise

## Design Documentation

`docs/design/` contains architectural design documents covering each subsystem in detail. Read these before making changes to core systems. Before writing a significant new feature or change, write a design doc and get feedback. See `docs/design/README.md` for a list of design docs and what they cover.

## Documentation Site

`site/` contains the source for the documentation site at https://rest.sh/. This is user-facing documentation and should be updated with new features and changes.

## Local Planning and Review Notes

`TODO.md` and `review.md` are local working files for planning, code reviews, and in-progress implementation notes only. They must never be committed to git history. Keep them ignored/untracked, and if they are accidentally staged, unstage them before committing.

## PR Review and Cleanup Discipline

Keep review and fix loops scoped to the current PR's behavioral surface unless the user explicitly asks for broader cleanup. After substantial review loops, do a simplification pass before final handoff: reduce repeated scaffolding, table-drive genuinely similar cases, and consolidate helpers where that preserves readability. Do not shrink a PR by deleting important coverage or making security/auth/cache behavior harder to audit.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) for all commits:

```text
<type>(optional-scope): <description>
```

Prefer these types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`, and `security`. Use a concise imperative description, such as `fix: preserve JSONC edit directory modes` or `docs: update plugin quickstart`.

## Skills

Invoke these skills when relevant:

- `rsh-product` for shaping features, CLI UX, naming, scope, and compatibility decisions before or during implementation
- `rsh-review` for code review feedback on code changes
- `rsh-test` for writing concise, behavior-focused Restish tests
- `rsh-docs` for writing and maintaining documentation, blog posts, and design docs
- `rsh-simplify` for approved simplification and dead-code cleanup work
- `rsh-release-qa` for release readiness QA from `main`

## Maintenance

If you find a stale fact in this file or a skill — a command that fails, a path or package that no longer exists — fix it as part of your change.
