---
title: Getting Started
linkTitle: Getting Started
weight: 10
description: Install Restish, make a request, register the example API, and learn the shortest path to daily use.
---

This section gets you from an installed binary to one useful request, then shows
why API-aware commands and profiles make repeated work calmer.

## Start Here

1. [Install](./install/) the current binary or build from source.
2. Follow the [Quickstart](./quickstart/) for the shortest complete path.
3. Make a slower [First Request](./first-request/) if you want the pieces named.
4. Run [Shell Setup](./shell-setup/) before using filters, query strings, or shorthand heavily.
5. [Connect to an API](./connect-to-an-api/) when generated commands are useful.
6. [Set Up Profiles](./set-up-profiles/) for environments, auth, and defaults.

## Common First Wins

- See what Restish sends: `restish https://api.rest.sh/`
- Register the docs API: `restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'`
- Use a generated command: `restish example list-images`
- Filter a response: `restish https://api.rest.sh/images -f body.self -r`

## Existing v1 Users

If you already have Restish v1 config or plugins, read [Upgrade From v1](./upgrade-from-v1/) before editing config. The migration docs are kept out of the new-user happy path, but they are still important for existing setups.

## Related Pages

- [Requests](/docs/guides/requests/)
- [Authentication](/docs/guides/authentication/)
- [Output](/docs/guides/output/)
- [Example API](/docs/reference/example-api/)
