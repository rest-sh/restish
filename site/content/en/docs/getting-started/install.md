---
title: Install
linkTitle: Install
weight: 20
description: Install or build Restish and verify that the binary can make a request.
---

This page documents the Restish v2 install path. Homebrew is the recommended
managed install path for most macOS users. Build from source when you are
developing Restish itself or testing changes before they are released.

Always verify the major version after installation:

```bash
restish --version
```

## Homebrew

Use Homebrew for the easiest managed install on macOS:

```bash
brew install restish
restish --version
```

The `rest-sh/tap` tap keeps a legacy `restish@1` formula available for users
who intentionally stay on v1 during migration. It also carries first-party
plugin formulae.

## Build From Source

Use this path for development builds from the repository:

```bash
go build ./cmd/restish
./restish --help
./restish api.rest.sh/
```

You can also install the latest stable v2 binary into your Go bin directory:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

## mise

Use mise when you already manage developer tools with it:

```bash
mise use -g restish@latest
restish --version
```

`latest` installs the latest stable v2 release through the mise registry. Pin a
v1 version when you need to stay on v1:

```bash
mise use -g restish@0.21.2
```

## GitHub Release Archives

Download the archive for your OS and CPU from
[GitHub Releases](https://github.com/rest-sh/restish/releases), unpack it, and
put the `restish` binary on your `PATH`.

Release archives are published for macOS, Linux, and Windows on `amd64` and
`arm64`.

## OCI Image

Run the stable image from GitHub Container Registry:

```bash
docker run --rm ghcr.io/rest-sh/restish:latest api.rest.sh/
```

For local image changes, build a development image from this repository:

```bash
docker build --build-arg VERSION=dev -t restish:dev .
docker run --rm restish:dev --version
docker run --rm restish:dev api.rest.sh/
```

## Verify The Install

```bash
restish --version
restish api.rest.sh/
```

The request should return an echo-shaped JSON document containing at least
`method`, `host`, `path`, and `url`.

## Set Up Your Shell

After the binary works, configure your shell so query strings, brackets, and
shorthand arrays are not rewritten before Restish sees them:

```bash
restish shell setup zsh
```

Use `bash` or `fish` instead of `zsh` when that is your shell.

## Existing v1 Users

Restish v2 can migrate default-location v1 config on first run. Read
[Upgrade From v1](../upgrade-from-v1/) before editing config or replacing
plugins. The archived v1 documentation remains available at
[rest.sh/v1/](https://rest.sh/v1/) for teams that need to compare old commands during a
migration.

## Next Step

If you came here from the tour, continue with [Shell Setup](../shell-setup/) so
your local shell handles Restish filters, query strings, and shorthand input
cleanly. If you started with install, take the [Tour of Restish](../tour/)
next to see the major workflows before connecting your own APIs.

## Related Pages

- [Tour of Restish](../tour/)
- [Shell Setup](../shell-setup/)
- [Development Setup](/docs/contributing/development-setup/)
- [Upgrade From v1](../upgrade-from-v1/)
- [Archived v1 Docs](https://rest.sh/v1/)
