# API Command Generation

## Summary

Restish v2 treats registered APIs as first-class CLI surfaces. Users configure
an API once, Restish discovers or loads its spec, and operations from that spec
become ordinary commands with generated help, arguments, and flags.

## Problem

A major part of Restish's value is that it should feel better than a generic
HTTP client once an API is known. Users should not need to memorize full URLs,
operation-specific parameters, or the shape of every request by hand.

At the same time, the generated CLI cannot be so magical that it becomes
unpredictable or impossible to debug. The generated command model needed to be:

- config-backed rather than hidden in local state
- predictable from the source OpenAPI document
- close enough to ordinary Cobra commands that help text and completions remain
  understandable
- flexible enough to honor API-specific CLI hints

## Design

The chosen model has three layers.

First, APIs are registered in `restish.json` under short names. A registration
provides the base URL and may provide an explicit spec URL plus per-profile
overrides like headers, query parameters, auth, and pagination settings.

Second, Restish resolves an API name to both a request target and a spec
identity. The short name becomes part of the command tree, while the
configuration remains the source of truth for base URL and profile behavior.

Third, operations from the API's OpenAPI document are turned into Cobra
commands. This lets generated operations behave like built-in commands instead
of a separate subsystem.

Some specific choices are worth preserving:

- generated commands are grouped under the API short name
- command names come from `operationId`, converted to kebab-case by default
- OpenAPI extensions such as `x-cli-name`, `x-cli-description`,
  `x-cli-aliases`, `x-cli-hidden`, and `x-cli-ignore` can shape the generated
  CLI
- required path and query parameters become positional arguments
- optional parameters become flags
- request bodies remain compatible with the same shorthand input model used by
  generic HTTP commands

The overall intent is that generated commands should feel native, not like a
thin codegen artifact pasted on top of the CLI.

## Example

Given this API registration:

```json
{
  "apis": {
    "petstore": {
      "base_url": "https://api.example.com"
    }
  }
}
```

and this OpenAPI operation:

```yaml
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      summary: Get a pet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
        - name: include
          in: query
          schema:
            type: string
      x-cli-name: pet
```

Restish generates a command that behaves roughly like:

```bash
restish petstore pet <pet-id> --include owner
```

which resolves to a request like:

```text
GET https://api.example.com/pets/<pet-id>?include=owner
```

## Alternatives Considered

### Keep everything as generic HTTP commands

This is simpler internally, but it gives up one of Restish's biggest strengths:
turning API descriptions into a usable command-line interface.

### Generate static code ahead of time

That would work for some APIs, but it adds an extra build step and makes the
user experience heavier. Restish is meant to adapt from local configuration and
cached specs, not require a compile phase for normal use.

### Treat generated operations as a separate mode

We could have made generated operations behave differently from built-in Cobra
commands, but that would make help, completions, and discoverability less
consistent. Folding them into the normal command tree keeps the UX simpler.

## Notes

The current implementation reflects this design in a few places:

- `internal/config/config.go` defines the config-backed API registration model
- `internal/cli/generated.go` builds API command groups and per-operation Cobra
  commands from OpenAPI data
- `internal/cli/http.go` resolves API short names and profile settings before
  issuing requests

One implementation detail that evolved from the early proposal is when command
generation happens. The original v2 notes discussed lazy stub commands that
would load specs on demand. The current implementation instead registers
generated commands from cached specs at startup and avoids network discovery
during command tree construction.
