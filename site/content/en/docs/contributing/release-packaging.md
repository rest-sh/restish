---
title: Release Packaging
linkTitle: Release Packaging
weight: 40
description: Maintainer checklist for Homebrew, GitHub releases, Go install, plugin binaries, versions, and changelog updates.
---

Use this page when preparing a Restish v2 release. It keeps the release
artifacts, package-manager metadata, plugin binaries, and docs in the same
checklist.

## Version And Changelog

1. Update the build version used by release builds.
2. Update the changelog with user-visible features, breaking changes, migration
   notes, and plugin protocol changes.
3. Check the upgrade guide when a v1 behavior changed or a v2 draft command was
   removed before release.

## GitHub Releases

Release archives should include the `restish` binary for supported macOS,
Linux, and Windows targets on `amd64` and `arm64`.

Before publishing:

```bash
go test ./...
go vet ./...
go build ./cmd/restish
```

The release notes should link to the install guide, changelog, and upgrade from
v1 guide.

## Homebrew

The Homebrew tap should install the current v2 `restish` binary as `restish`.
The legacy v1 formula stays keg-only as `restish@1` for users who need the old
binary while migrating.

After updating the formula, verify:

```bash
brew install rest-sh/tap/restish
restish --version
restish https://api.rest.sh/
```

## Go Install

The source install path should remain:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
```

Make sure the install docs do not point users at internal packages or plugin
commands as the main binary.

## Plugin Binaries

Build and publish first-party plugin binaries with the same OS/CPU coverage as
the main CLI when practical:

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

Each plugin archive should contain one executable plugin binary. Plugin docs
and release notes should remind users that installed plugins are trusted local
executables and that v1 plugins must be rebuilt for the v2 protocol.

Plugin protocol release checklist:

- Keep existing message fields additive and preserve field meanings.
- Add tests proving unknown optional fields are ignored.
- Add tests proving unsupported `required_features` fail clearly.
- Update plugin quickstart, reference, site, and design docs for new messages,
  hooks, or required features.
- Run `restish plugin debug` against command, formatter, loader, auth,
  middleware, and TLS signer fixtures before tagging.

## Related Pages

- [Install](../../getting-started/install/)
- [Upgrade From v1](../../getting-started/upgrade-from-v1/)
- [Install and Use Plugins](../../plugins/install-and-use/)
