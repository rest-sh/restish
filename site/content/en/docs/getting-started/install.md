---
title: Install
linkTitle: Install
weight: 20
description: Install or build Restish and verify that the binary can make a request.
---

Use Homebrew when you want the managed install path. Use GitHub release
archives, the OCI image, or a source build when those fit your environment
better.

## Homebrew

```bash
brew install rest-sh/tap/restish
restish --version
```

## mise

Use mise when you already manage developer tools with it:

```bash
mise use -g restish@latest
restish --version
```

After v2 is released, `latest` installs the latest stable v2 release. Pin a v1
version when you need to stay on v1:

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

```bash
docker run --rm ghcr.io/rest-sh/restish:latest api.rest.sh/
```

## Build From Source

From this repository:

```bash
go build ./cmd/restish
./restish --help
./restish api.rest.sh/
```

To install the binary into your Go bin directory:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
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
plugins.

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
