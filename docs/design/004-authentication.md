# Authentication

## Summary

Restish v2 treats authentication as profile-driven request mutation. Each API
profile can declare an auth type plus parameters, and Restish resolves that
configuration into a handler that modifies outgoing requests immediately before
they are sent.

## Problem

Authentication needs to be persistent enough for real API use, but still easy
to understand from the command line and configuration file.

The design needed to support:

- simple built-in mechanisms like HTTP Basic
- token-based OAuth flows with caching
- per-profile auth differences for the same API
- interactive prompting for secrets when they are not stored
- future extension points without hard-coding every auth system into the core

## Design

The core model is:

- auth is configured under an API profile
- the profile declares a `type` and `params`
- Restish resolves that into an auth handler
- the handler mutates the outbound request in an `OnRequest` hook

This keeps auth aligned with the same profile layering model used for base
URLs, headers, and query defaults.

Built-in auth currently includes:

- `http-basic`
- `oauth-client-credentials`
- `oauth-authorization-code`

The request pipeline treats auth as a request-stage concern rather than a
transport concern. By the time the HTTP request is sent, the auth handler has
already added whatever credentials are needed.

There are a few specific choices worth preserving:

- profile selection determines the auth context
- token-oriented auth caches tokens using an `api:profile` cache key
- prompting is supported when secrets are omitted from config
- `auth-header <api>` reuses the same auth resolution path and prints the final
  `Authorization` header value for inspection or shell use

The OAuth model also includes OIDC discovery support so an issuer URL can be
resolved into concrete authorization and token endpoints.

## Examples

Basic auth in config:

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice",
              "password": "s3cr3t"
            }
          }
        }
      }
    }
  }
}
```

which causes:

```bash
restish get myapi/items
```

to send an `Authorization: Basic ...` header.

If the password is omitted:

```json
{
  "type": "http-basic",
  "params": {
    "username": "alice"
  }
}
```

Restish prompts for the secret at request time instead of requiring it to live
in the config file.

The same resolution path is used by:

```bash
restish auth-header myapi
```

which prints the computed `Authorization` header value.

## Alternatives Considered

### Store auth outside profiles

This would weaken the connection between environment selection and auth
selection. Profiles are the right place because auth almost always varies with
the same environment boundaries as base URLs and persistent headers.

### Treat auth as only static headers in config

That works for some APIs, but it is not expressive enough for OAuth flows,
prompted secrets, or token refresh behavior.

### Push all auth into plugins or external commands

That would keep the core smaller, but it would make common auth setups too
heavyweight. Built-in support for the most common mechanisms is worth it.

## Notes

The current implementation reflects this design directly:

- `internal/cli/auth.go` resolves profile auth config into request hooks
- `internal/auth/auth.go` defines the shared auth handler abstraction
- `internal/auth/basic.go` implements HTTP Basic authentication
- `internal/auth/oauth_common.go` provides shared OAuth and OIDC token helpers

One detail worth preserving is that auth is applied through the request hook
layer, not by teaching the HTTP transport about individual auth schemes. That
keeps request construction composable and makes auth behavior easier to inspect
and test.
