---
title: Getting Started
linkTitle: Getting Started
weight: 10
description: Install Restish, make a request, register the example API, and learn the shortest path to daily use.
---

This section starts with an interactive browser tour, then helps you install
Restish and turn one-off requests into a useful API-specific workflow. If you
do not know what to choose, follow the default path in order.

## Start Here

1. Follow the [Tour of Restish](./quickstart/) to try the major workflows in your browser.
2. Use [Install](./install/) when you are ready to run Restish locally.
3. Run [Shell Setup](./shell-setup/) before using filters, query strings, or shorthand heavily.
4. [Connect to an API](./connect-to-an-api/) when generated commands are useful.
5. [Set Up Profiles](./set-up-profiles/) for environments, auth, and defaults.

## Common First Wins

- See what Restish sends: `restish https://api.rest.sh/`
- Register the docs API: `restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'`
- Use a generated command: `restish example list-images`
- Filter a response: `restish https://api.rest.sh/images -f body.self -o lines`

## Existing v1 Users

If you already have Restish v1 config or plugins, read [Upgrade From v1](./upgrade-from-v1/) before editing config. The migration docs are kept out of the new-user happy path, but they are still important for existing setups.

## Related Pages

- [Requests](/docs/guides/requests/)
- [Authentication](/docs/guides/authentication/)
- [Output](/docs/guides/output/)
- [Example API](/docs/reference/example-api/)
