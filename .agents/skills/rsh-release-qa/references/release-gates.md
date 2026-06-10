# Restish Release Gate Reference

These are the default checks for a full Restish release readiness pass. Run
from the exact release candidate checkout or a temporary worktree.

## Candidate Orientation

Create a fresh run directory first and reuse it for temporary build artifacts,
config files, caches, and smoke state. This keeps repeated QA runs independent
and makes cleanup simple.

```bash
TMP_ROOT="${TMPDIR:-/tmp}"
RUN_DIR="$(mktemp -d "$TMP_ROOT/restish-release-qa.XXXXXX")"
mkdir -p "$RUN_DIR/bin" "$RUN_DIR/go-cache" "$RUN_DIR/go-path" "$RUN_DIR/smoke-cache" "$RUN_DIR/npm-cache"
```

```bash
git status --short
git branch --show-current
git rev-parse HEAD
git fetch origin
git log --oneline --first-parent --max-count=20
git describe --tags --abbrev=0 --match 'v*'
```

If the current branch is not the candidate, prefer a detached temporary
worktree over switching the user's branch. Run the remaining checks from the
candidate checkout.

```bash
git worktree add --detach "$RUN_DIR/worktree" origin/main
cd "$RUN_DIR/worktree"
```

## Diff Surface

Replace `<baseline>` with the last release tag or the user-provided baseline.

```bash
git log --oneline --first-parent <baseline>..HEAD
git log --oneline --no-merges <baseline>..HEAD
git diff --stat <baseline>..HEAD
git diff --name-only <baseline>..HEAD
```

## Full Go Gate

Use writable caches when the sandbox cannot write the default Go build cache or
`GOPATH` module/checksum state.

```bash
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go test ./...
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go test -tags=integration ./...
```

Run targeted race coverage for the release-risk packages. Broaden the list when
the diff touches other concurrent or subprocess-heavy code.

```bash
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go test -race ./internal/cli ./internal/auth ./internal/request ./internal/plugin ./internal/spec ./cmd/restish-mcp
```

If the isolated `GOPATH` requires fresh toolchain or module downloads and the
network is unavailable, report that limitation instead of changing dependencies
during release QA.

## Release Binary Builds

Build the core CLI and first-party plugins into a temp directory.

```bash
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go build -o "$RUN_DIR/bin/restish" ./cmd/restish
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go build -o "$RUN_DIR/bin/restish-bulk" ./cmd/restish-bulk
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go build -o "$RUN_DIR/bin/restish-csv" ./cmd/restish-csv
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go build -o "$RUN_DIR/bin/restish-mcp" ./cmd/restish-mcp
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go build -o "$RUN_DIR/bin/restish-pkcs11" ./cmd/restish-pkcs11
```

Local `go build` binaries usually print the development version. Treat that as
expected unless testing GoReleaser injection specifically.

```bash
"$RUN_DIR/bin/restish" --version
printf '{}\n' > "$RUN_DIR/empty.json"
chmod 600 "$RUN_DIR/empty.json"
"$RUN_DIR/bin/restish" --rsh-config "$RUN_DIR/empty.json" help
```

## Generated Docs And Site

```bash
env GOCACHE="$RUN_DIR/go-cache" GOPATH="$RUN_DIR/go-path" go run ./cmd/restish-docgen --check
scripts/check-doc-links.rb
scripts/check-doc-examples.rb
```

The example check's `--mode live` variant executes every docs `restish-example`
against `api.rest.sh`; include it when the release touches command surface or
docs examples and the network allows it.

For the docs site, use a disposable npm cache if the user cache is not writable
or contains permission problems. These commands must run against `site/`, which
contains the docs site's `package.json` and lockfile.

```bash
npm --prefix site ci --cache "$RUN_DIR/npm-cache"
npm --prefix site run build
```

Hugo deprecation warnings are non-blocking unless they break the build or imply
near-term publish failure.

## Built-Binary Smokes

Use isolated config/cache files and harmless targets.

```bash
env RSH_CACHE_DIR="$RUN_DIR/smoke-cache" \
  "$RUN_DIR/bin/restish" \
  --rsh-config "$RUN_DIR/smoke.json" \
  api connect smokepet https://petstore3.swagger.io/api/v3 \
  --spec https://petstore3.swagger.io/api/v3/openapi.json \
  --yes prompt.credentials.api_key.value:fake-key
```

Then inspect generated command state without provider writes.

```bash
env RSH_CACHE_DIR="$RUN_DIR/smoke-cache" \
  "$RUN_DIR/bin/restish" \
  --rsh-config "$RUN_DIR/smoke.json" \
  doctor api smokepet

env RSH_CACHE_DIR="$RUN_DIR/smoke-cache" \
  "$RUN_DIR/bin/restish" \
  --rsh-config "$RUN_DIR/smoke.json" \
  smokepet get-pet-by-id 1 \
  --rsh-server https://httpbin.org/anything \
  --rsh-print H \
  --rsh-ignore-status-code
```

Check network-error redaction locally.

```bash
printf '{}\n' > "$RUN_DIR/redaction.json"
chmod 600 "$RUN_DIR/redaction.json"

env RSH_CACHE_DIR="$RUN_DIR/smoke-cache" \
  "$RUN_DIR/bin/restish" \
  --rsh-config "$RUN_DIR/redaction.json" \
  get \
  'http://alice:s3cr3t@127.0.0.1:9/anything?api_key=url-secret' \
  --rsh-query token=flag-secret \
  --rsh-retry 0 \
  --rsh-timeout 200ms
```

The command should fail, but the diagnostic must not expose `alice`,
`s3cr3t`, `url-secret`, or `flag-secret`.

## Optional Packaging Checks

If GoReleaser is installed:

```bash
goreleaser check
```

If it is not installed, report that limitation instead of installing new tools
during QA unless the user asks.
