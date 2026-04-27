---
title: Install
linkTitle: Install
weight: 20
description: Install or build Restish and verify that the binary can make a request.
---

Before the first public v2 release, source builds are the reliable path. Package
manager and container examples describe the intended release channels and should
be verified when release artifacts are published.

## Build From Source

From this repository:

```bash
go build ./cmd/restish
./restish --help
./restish https://api.rest.sh/
```

To install the binary into your Go bin directory:

```bash
go install ./cmd/restish
restish --help
```

## Planned Release Channels

When v2 artifacts are published, these channels should be available or updated
before the docs are treated as release-ready.

Homebrew:

```bash
brew install restish
```

mise:

```bash
mise use -g restish@latest
```

Nixpkgs:

```bash
nix-env --install --attr nixpkgs.restish
```

OCI image:

```bash
docker run --rm ghcr.io/rest-sh/restish:latest https://api.rest.sh/
```

GitHub release binaries should be copied to a directory on your `PATH`.

## Verify The Install

```bash
restish --help
restish https://api.rest.sh/
```

The request should return an echo-shaped JSON document containing at least
`method`, `host`, `path`, and `url`.

## Set Up Your Shell

After the binary works, configure your shell so query strings, brackets, and
shorthand arrays are not rewritten before Restish sees them:

```bash
restish setup zsh
```

Use `bash` or `fish` instead of `zsh` when that is your shell.

## Existing v1 Users

Restish v2 can migrate default-location v1 config on first run. Read
[Upgrade From v1](../upgrade-from-v1/) before editing config or replacing
plugins.

## Next Step

Follow the [Quickstart](../quickstart/) to make your first request and register
the live example API.

## Related Pages

- [Quickstart](../quickstart/)
- [Shell Setup](../shell-setup/)
- [Development Setup](/docs/contributing/development-setup/)
- [Upgrade From v1](../upgrade-from-v1/)
