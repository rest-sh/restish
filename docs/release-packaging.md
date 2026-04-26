# Release Packaging

This document records the v2 release packaging decisions that are not tied to
one package manager.

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
