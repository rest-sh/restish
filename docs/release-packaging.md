# Release Packaging

This document records the v2 release packaging decisions that are not tied to
one package manager.

The public install paths should point at the current v2 release. Keep source
builds documented as the development path, not the primary user install path.

## GitHub Release Artifacts

Tagged releases publish native archives with GoReleaser. The core CLI is built
as `restish` for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

Release archives use this shape:

```text
restish-X.Y.Z-darwin-arm64.tar.gz
restish-X.Y.Z-linux-amd64.tar.gz
restish-X.Y.Z-windows-amd64.zip
checksums.txt
```

The hyphenated archive names intentionally preserve the v1 GitHub Release
asset shape used by aqua and mise metadata. Stable v2 releases become the
`latest` version for floating installer configs; users who need v1 can pin an
older version such as `0.21.2`.

GoReleaser injects the tag version into
`github.com/rest-sh/restish/v2/internal/cli.Version`.

## Homebrew

The primary Homebrew install path for the current v2 CLI is Homebrew core:

```bash
brew install restish
```

The public tap still exists for legacy and plugin formulae:

```text
rest-sh/tap
```

The GitHub repository backing that tap must be named:

```text
rest-sh/homebrew-tap
```

Stable v2 releases should update the Homebrew core `restish` formula. After
publishing, verify the core formula installs the current v2 binary:

```bash
brew install restish
restish --version
restish api.rest.sh/
```

The tap keeps a separate `restish@1` formula for the last v1 release. That
formula is intentionally keg-only so it can coexist with the current core
`restish` formula without fighting for the same linked executable.

```bash
brew install rest-sh/tap/restish@1
```

The release workflow uses the existing Restish Releaser GitHub App secrets
(`RELEASER_APP_ID` and `RELEASER_APP_PRIVATE_KEY`) to mint a short-lived token
with access to `rest-sh/homebrew-tap`. The workflow seeds the v1 formula from
`packaging/homebrew/restish@1.rb` and updates first-party plugin formulae in the
tap. Do not advertise `rest-sh/tap/restish` as the normal v2 install path while
Homebrew core is current.

## mise

The mise registry shorthand includes a `restish` entry for the v2 line:

```bash
mise use -g restish@latest
```

Verify this after publishing because the shorthand follows published release
metadata. Users who need v1 should pin the last v1 version:

```bash
mise use -g restish@0.21.2
```

## First-Party Plugin Artifacts

GoReleaser builds pure-Go first-party plugins for the same OS/architecture set
as the core CLI:

- `restish-bulk`
- `restish-csv`
- `restish-mcp`

The PKCS#11 plugin depends on CGO-backed PKCS#11 bindings. The release workflow
uses `goreleaser-cross` to build it for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`

Windows arm64 is not built for `restish-pkcs11` until the release image includes
a Windows arm64 C toolchain.

Each plugin has its own GitHub Release archives and Homebrew formula:

```bash
brew install rest-sh/tap/restish-bulk
brew install rest-sh/tap/restish-csv
brew install rest-sh/tap/restish-mcp
brew install rest-sh/tap/restish-pkcs11
```

Restish does not scan `PATH` for plugins. Homebrew formula caveats tell users to
make the explicit trust decision with the existing local install path:

```bash
restish plugin install "$(brew --prefix restish-csv)/bin/restish-csv"
```

Plugin protocol release checklist:

- Keep existing message fields additive and preserve field meanings.
- Add tests proving unknown optional fields are ignored.
- Add tests proving unsupported `required_features` fail clearly.
- Update `docs/plugin-quickstart.md`, site plugin docs, and design docs for new
  messages, hooks, or required features.
- Run `restish plugin debug` against command, formatter, loader, auth,
  middleware, and TLS signer fixtures before tagging.

## OCI Image

The official image is published as:

```text
ghcr.io/rest-sh/restish
```

Stable release tags publish:

- `ghcr.io/rest-sh/restish:vX.Y.Z`
- `ghcr.io/rest-sh/restish:X.Y.Z`
- `ghcr.io/rest-sh/restish:X.Y`
- `ghcr.io/rest-sh/restish:latest`

Release candidates publish image tags for the exact candidate, such as
`vX.Y.Z-rc.N` and `X.Y.Z-rc.N`, but they do not update `latest` or the
minor-version floating tag.

The image is multi-arch for:

- `linux/amd64`
- `linux/arm64`

The default image contains the `restish` CLI and the normal CA certificate
bundle. It does not bundle command, formatter, or TLS-signer plugins. Plugin
workflows should either run on the host or build a derived image that copies
the required `restish-*` plugin binaries into the configured plugin directory.

## Local Image Build

Build the development image from the repo root:

```bash
docker build --build-arg VERSION=dev -t restish:dev .
```

Smoke checks:

```bash
docker run --rm restish:dev --version
docker run --rm restish:dev https://api.rest.sh/
```

## Publish Command

The GitHub Actions workflow publishes tagged releases. The equivalent manual
Buildx command is:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=vX.Y.Z \
  -t ghcr.io/rest-sh/restish:vX.Y.Z \
  -t ghcr.io/rest-sh/restish:X.Y.Z \
  -t ghcr.io/rest-sh/restish:X.Y \
  -t ghcr.io/rest-sh/restish:latest \
  --push .
```

For a release candidate, omit `latest` and the `X.Y` floating tag.
