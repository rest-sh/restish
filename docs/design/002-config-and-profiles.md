# Config And Profiles

## Summary

Restish v2 uses a single `restish.json` file as the main source of persistent
user configuration. That file holds registered APIs, optional profiles for each
API, and a small set of global settings.

Profiles are the main mechanism for expressing per-environment differences such
as base URLs, persistent headers, query parameters, auth configuration, and TLS
signer settings.

## Problem

Restish needs persistent configuration, but the configuration model has to stay
easy to understand from the command line.

The main pressures on the design were:

- users should have one obvious place to look for configuration
- API-specific behavior should be explicit and inspectable
- switching between environments should not require duplicating an entire API
  definition
- command-line flags should still be able to override persistent defaults for a
  single invocation
- the config format should be strict enough to catch mistakes but friendly
  enough to annotate

## Design

The chosen model is a single JSONC-backed config file, normally named
`restish.json`.

Within that file, the most important top-level key is `apis`. Each API is
registered under a short name and can define:

- `base_url`
- `spec_url`
- `profiles`
- pagination settings

Profiles live under an API rather than at the global level. That keeps the
configuration aligned with how Restish is used: a profile is meaningful in the
context of a specific API, not as a universal environment abstraction shared by
all APIs.

Each profile can override or add request-affecting settings such as:

- `base_url`
- persistent headers
- persistent query parameters
- auth configuration
- TLS signer configuration and parameters

The active profile defaults to `default`, with `RSH_PROFILE` and
`--rsh-profile` selecting a different one.

When Restish resolves a request through a registered API, configuration is
applied in layers:

1. API registration establishes the base identity of the target.
2. The selected profile overrides API-level defaults such as `base_url`.
3. Persistent profile headers and query parameters are prepended to request
   options.
4. Command-line flags still win for that invocation because they are applied
   later.

That precedence rule is important. Profiles provide durable defaults, but they
do not trap users into them.

The file format is intentionally strict. Restish strips JSONC comments first,
then decodes into typed Go structs with unknown fields rejected. This keeps the
file human-editable while still catching typos early.

## Example

```jsonc
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com",
      "spec_url": "https://api.github.com/openapi.json",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"],
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice",
              "password": "",
            },
          },
        },
        "enterprise": {
          "base_url": "https://github.example.com/api/v3",
          "headers": ["Accept: application/json"],
          "query": ["per_page=100"],
        },
      },
    },
  },
}
```

For a call like:

```bash
restish --rsh-profile enterprise get github/repos/octo/example -q per_page=50
```

the effective request uses:

- the `enterprise` profile's `base_url`
- the profile's persistent headers
- the command-line `-q per_page=50` value instead of the profile default

## Alternatives Considered

### Separate files for APIs and global config

This can work, but it makes discovery worse. One file is easier to explain,
easier to inspect, and easier to manage in both documentation and tooling.

### Global profiles shared across all APIs

That sounds reusable, but in practice it weakens the connection between a
profile and the API-specific settings it actually controls. Nesting profiles
under APIs makes intent clearer.

### Unstructured or loosely validated config

This would make experimentation easier in the short term, but it tends to shift
mistakes into runtime behavior. Restish benefits from rejecting invalid config
early and explicitly.

## Notes

The current implementation reflects this design directly:

- `internal/config/config.go` defines the single-file typed config model and
  JSONC parsing behavior
- `internal/config/jsonc_patch.go` preserves comments for targeted object-path
  edits made by config-management commands
- `internal/cli/http.go` applies API and profile settings to outgoing requests
- `internal/cli/api.go` exposes management commands like `api configure`,
  `api show`, `api set`, and `api edit`

One useful detail to preserve is that profile headers and query parameters are
prepended rather than appended when building request options. That allows
command-line values to override persistent defaults cleanly for one-off calls.
