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

Existing v1 users who need the legacy binary can install the keg-only v1 formula
from the same tap:

```bash
brew install rest-sh/tap/restish@1
```

## GitHub Release Archives

Download the archive for your OS and CPU from GitHub Releases, unpack it, and
put the `restish` binary on your `PATH`.

Release archives are published for macOS, Linux, and Windows on `amd64` and
`arm64`.

## OCI Image

```bash
docker run --rm ghcr.io/rest-sh/restish:latest https://api.rest.sh/
```

## Build From Source

From this repository:

```bash
go build ./cmd/restish
./restish --help
./restish https://api.rest.sh/
```

To install the binary into your Go bin directory:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

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
restish shell setup zsh
```

Use `bash` or `fish` instead of `zsh` when that is your shell.

## Existing v1 Users

Restish v2 can migrate default-location v1 config on first run. Read
[Upgrade From v1](../upgrade-from-v1/) before editing config or replacing
plugins.

## Next Step

Follow the [Tour of Restish](../quickstart/) to try the major workflows in your
browser, then run the same examples locally.

## Related Pages

- [Tour of Restish](../quickstart/)
- [Shell Setup](../shell-setup/)
- [Development Setup](/docs/contributing/development-setup/)
- [Upgrade From v1](../upgrade-from-v1/)
