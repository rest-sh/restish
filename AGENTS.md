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

# Run fast tests (default development loop; excludes slow integration tests)
go test ./...

# Run tests for a specific package
go test ./internal/cli/...

# Run the full suite, including slow plugin and CLI integration tests.
# Run this before commits that touch CLI/plugin behavior, and before final commits.
go test -tags=integration ./...

# Update golden files for output formatter regression tests
go test -update ./internal/output/...
```

## Architecture

Restish is a CLI for interacting with REST-ish HTTP APIs. It generates commands from OpenAPI 3.x specs and supports generic HTTP verbs, content negotiation, authentication, pagination, caching, filtering, and plugins.

### Central `CLI` Struct

The core design is a `CLI` struct in `internal/cli/cli.go` that owns all state — I/O handles, config, content registry, spec loaders, link parsers, formatters, and plugins. Tests instantiate `CLI` directly with `bytes.Buffer` for I/O and `httptest.Server` for HTTP.

**Entry point**: `cmd/restish/main.go` creates a `CLI` and calls `Run(os.Args)`.

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

- `rsh-review` for code review feedback on code changes
- `rsh-test` for writing concise, behavior-focused Restish tests
- `rsh-docs` for writing and maintaining documentation
- `rsh-simplify` for approved simplification and dead-code cleanup work
