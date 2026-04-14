---
title: How Restish Works
linkTitle: How Restish Works
weight: 10
description: Understand the core model behind generic requests, API commands, profiles, and normalized responses.
---

Restish combines two modes:

- generic HTTP commands for immediate access
- API-aware commands generated from API descriptions

Under the hood, Restish v2 centers around a single CLI object that owns config,
I/O, registries, spec loading, output formatting, and plugins.

## Why This Matters To Users

That architecture gives users a consistent mental model:

- profiles decide how requests are configured
- APIs can add generated commands
- responses go through a normalized output path
- plugins extend behavior without changing the core workflow

## The Basic Flow

From a user's perspective, most work follows the same pattern:

1. choose a target, either a full URL or a configured API name
2. apply profile settings such as base URL, auth, headers, and TLS
3. make the request
4. normalize the response
5. optionally paginate, filter, or stream it
6. render the final output

That is why the same flags and habits carry across so many parts of the CLI.

## Generic Requests vs API Commands

Generic requests are best when you want to move fast:

```bash
restish https://api.rest.sh/images
```

API commands are better once Restish knows the API description:

```bash
restish myapi users list
```

The underlying request pipeline is still the same. The difference is how much
help Restish can provide before the request is sent.

## Profiles Tie The Experience Together

Profiles are the main place where repeated behavior lives:

- base URL overrides
- default headers and query parameters
- auth configuration
- TLS signer or certificate choices

That means switching environments is usually a profile choice, not a complete
rewrite of every command.

## Normalized Responses Make Output Predictable

Restish does not send raw HTTP responses directly to each formatter. It
normalizes them first into a stable response shape that includes protocol
details, headers, links, and body data.

That is why filtering, output formats, pagination, and links all feel like
parts of one system instead of unrelated features.

## Plugins Extend Specific Seams

Plugins do not replace the CLI model. They plug into specific parts of it:

- auth
- request middleware
- response middleware
- spec loading
- formatting
- custom top-level commands
- TLS signing

That lets Restish grow without making every feature a core built-in behavior.

## Learn More

- [API Commands](../api-commands/)
- [Profiles](../profiles/)
- [Plugins](../plugins/)

Contributor-oriented design details live in the
[design records](/docs/contributing/design-records/).
