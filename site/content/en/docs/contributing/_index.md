---
title: Contributing
linkTitle: Contributing
weight: 70
description: Contributor setup, architecture notes, and design records for Restish v2.
---

Contributor docs are separated from the end-user guides on purpose. Start here
if you want to build Restish itself, work on the docs site, or understand the
core architecture before changing behavior.

## Path

`Documentation -> Contributing`

## Start Here

- [Development Setup](./development-setup/) for the local toolchain and common
  commands.
- [Design Records](./design-records/) for the subsystem-level architecture
  documents in the repository's `docs/design/` directory.
- [Docs Maintenance](./docs-maintenance/) for doc checklists, validation, and
  migration status.

## Good Next Steps

- Read the CLI architecture record before changing shared command behavior.
- Check subsystem design docs before editing auth, spec loading, output, or
  plugin runtime code.
- Keep public docs focused on user workflows, then link deeper design notes
  when architecture context matters.
