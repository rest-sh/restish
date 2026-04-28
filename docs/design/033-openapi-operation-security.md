# OpenAPI Operation Security

## Status

Draft for v2 release-readiness. Current implementation only honors
operation-level `security: []` as an explicit no-auth marker; full
operation-specific scheme selection is intentionally deferred until this design
is implemented.

## Problem

OpenAPI security is operation-specific. A document can define several schemes,
set a default security policy at the document level, override it per operation,
offer alternative requirements, and express OAuth scopes. Restish profiles
currently describe how to attach credentials, but they do not explicitly declare
which OpenAPI security schemes they satisfy.

Without a clear contract, Restish can either attach too much auth to public
operations, fail to diagnose mismatched profiles, or give users a false sense
that an operation-specific OpenAPI policy is enforced.

## Goals

- Keep `security: []` as a reliable no-auth escape hatch.
- Let profiles declare which OpenAPI security schemes they satisfy.
- Select credentials per operation without prompting unexpectedly during command
  registration.
- Represent OpenAPI alternatives and combined requirements accurately enough to
  give users useful diagnostics.
- Surface OAuth scopes in help and errors even before Restish validates them
  against provider-specific tokens.

## Non-Goals

- Full OAuth token introspection across providers.
- Validating remote authorization server policy at command generation time.
- Inventing auth behavior that cannot be traced to config, profile selection, or
  the OpenAPI document.

## Current Behavior And Constraints

Restish request auth is profile-driven. Generic requests and generated commands
select a configured API/profile, then attach profile auth through the shared
request pipeline.

Generated operations carry `NoAuth` when the OpenAPI operation declares
`security: []`. In that case request execution suppresses profile auth for that
operation. Other OpenAPI `security` values, including alternatives, combined
requirements, and OAuth scopes, are preserved only as metadata or parsed
indirectly through setup/config prompts. They are not currently enforced per
operation, and they do not change which selected profile is used for the
request.

Startup command registration must stay offline. Any security matching must use
cached operation metadata and local config only.

## User-Facing Behavior

Profiles should eventually be able to declare scheme satisfaction explicitly:

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": "bearer",
          "security_schemes": ["BearerAuth"]
        },
        "admin": {
          "auth": "api_key",
          "security_schemes": ["AdminKey"]
        }
      }
    }
  }
}
```

For an operation with `security: [{BearerAuth: []}, {AdminKey: []}]`, either
profile can run the command. For `security: [{BearerAuth: [], AdminKey: []}]`,
one selected profile must satisfy both schemes or Restish should fail before
the request with a diagnostic naming the missing scheme.

For `security: []`, Restish sends no profile auth even when a profile is active.

Until the full policy below is implemented, every non-empty OpenAPI security
requirement behaves like an ordinary generated command: the selected
Restish profile supplies auth through the shared request pipeline.

## Proposed Design

Extend the neutral operation model with a compact security policy:

- `NoAuth` for explicit empty security arrays;
- `SecurityAlternatives []SecurityRequirement` for non-empty policy;
- each `SecurityRequirement` maps scheme name to required scopes.

Document-level security becomes the default for operations that omit
`security`. Operation-level security overrides the document default.

Extend profile config with a small declaration of scheme names the profile
satisfies. For common setup flows Restish can prefill this from the selected
OpenAPI scheme, but users can edit it by hand. Auth plugins can also declare
schemes if the plugin owns credential attachment.

At request time, generated commands choose the first satisfied alternative. If
none match, Restish returns an error that includes:

- the operation command;
- the selected profile;
- the alternatives from the spec;
- the scheme names and scopes missing from the selected profile.

OAuth scopes are compared as strings against profile declarations if present.
If a profile declares a scheme but omits scopes, Restish should warn or fail
according to a release decision; the safer default is to fail for operations
that require scopes and say how to declare them.

## Alternatives

Attaching the active profile auth to every operation except `security: []` is
simple but hides policy mismatches and is not enough for APIs with mixed auth
schemes.

Auto-selecting a different configured profile per operation could be convenient,
but it makes command behavior harder to reason about and can attach credentials
the user did not select. Restish should prefer explicit profile selection.

## Compatibility And Migration

The current `security: []` behavior is retained. Existing profiles without
scheme declarations continue to work until operation-specific enforcement is
enabled. Before enabling enforcement by default, Restish should provide a
compatibility period with warnings or a config migration that adds scheme names
for profiles created from OpenAPI setup prompts.

## Security And Failure Modes

No-auth operations must never run auth plugins or attach persistent profile
credentials.

Diagnostics must not print tokens or secret config values. They may print
scheme names and scope names from the OpenAPI document.

An untrusted spec must not be able to force Restish to send credentials to a
different origin. Server-origin checks and operation URL planning remain the
request pipeline's responsibility.

## Testing Plan

- document-level default security applies to operations that omit `security`;
- operation `security: []` suppresses auth;
- operation security overrides document defaults;
- alternative requirements accept any satisfied alternative;
- combined requirements require every scheme in the requirement;
- OAuth scopes appear in diagnostics and are checked against profile
  declarations;
- cached operation metadata preserves security policy for offline startup.

## Documentation Impact

The OpenAPI CLI integration guide should say that v2 currently supports
`security: []` as no-auth and that full per-operation scheme matching is a
planned v2 release-readiness item until this design is implemented.
Design 034 records this current narrow contract alongside the broader OpenAPI
implementation matrix.

## Open Questions

- Should profiles without declared scopes warn or fail when an operation
  requires OAuth scopes?
- Should auth plugins declare schemes statically in their manifest, dynamically
  during setup, or both?
- Should Restish offer a command that reports which generated operations are not
  satisfied by the selected profile?
