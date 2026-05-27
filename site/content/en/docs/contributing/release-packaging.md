---
title: Release Packaging
linkTitle: Release Packaging
weight: 40
description: Maintainer checklist for Homebrew, GitHub releases, Go install, plugin binaries, versions, and changelog updates.
---

Use this page when preparing a Restish v2 release. It keeps the release
artifacts, package-manager metadata, plugin binaries, and docs in the same
checklist.

The public install guide assumes the v2 release artifacts are available:
Homebrew first for macOS users, then source, mise, GitHub archives, and OCI
image options. Before publishing docs, verify each public artifact below rather
than adding temporary caveats to the user-facing install path.

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
go test -tags=integration ./...
go vet ./...
go build ./cmd/restish
```

The release notes should link to the install guide, changelog, and upgrade from
v1 guide.

After publishing, verify the release page shows the v2 tag as the latest stable
release and includes `checksums.txt` plus archives named like:

```text
restish-X.Y.Z-darwin-arm64.tar.gz
restish-X.Y.Z-linux-amd64.tar.gz
restish-X.Y.Z-windows-amd64.zip
```

## Homebrew

The Homebrew tap should install the current v2 `restish` binary as `restish`.
The legacy v1 formula stays keg-only as `restish@1` for users who need the old
binary while migrating.

After updating the formula, verify:

```bash
brew install rest-sh/tap/restish
restish --version
restish api.rest.sh/
```

Check the tap formula directly and do not rely on an unqualified
`brew install restish` for the v2 release verification.

## mise

The mise registry shorthand currently resolves `restish` through the upstream
registry entry. After publishing v2 archives, verify the shorthand installs the
v2 release and that users can still pin the final v1 tag:

```bash
mise use -g restish@latest
restish --version
mise use -g restish@0.21.2
```

## Go Install

The source install path should remain:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
```

Confirm the installed binary reports the v2 version. Make sure the install docs
do not point users at internal packages or plugin commands as the main binary.

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

After publishing, verify GitHub release shorthand and Homebrew install both
work for first-party plugins:

```bash
restish plugin install rest-sh/restish csv --yes
brew install rest-sh/tap/restish-csv
restish plugin install restish-csv --yes
```

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
- [Docs Maintenance](../docs-maintenance/)
