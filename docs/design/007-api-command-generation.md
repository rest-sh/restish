# API Command Generation

## Summary

Restish v2 turns registered APIs into first-class CLI surfaces. A configured API
becomes a command group, and operations from its API description become normal
Cobra commands with predictable names, arguments, flags, and help output.

The key design point is that generated commands should feel native while still
being traceable back to the source API description.

## Goals

- let users work with APIs by name instead of memorizing raw URLs
- preserve a close mapping from spec to CLI behavior
- generate commands deterministically from cached or local specs
- support CLI-shaping extensions without turning generation into ad-hoc code
- keep generated commands compatible with the core request pipeline

## Non-Goals

- perfect code generation for every OpenAPI edge case
- making the CLI mirror every spec quirk literally when that hurts usability
- requiring ahead-of-time build steps

## Command Generation Inputs

Generation depends on:

- API registration from config
- canonical loaded API description from design 006
- profile and pagination metadata from config
- CLI-specific extensions such as `x-cli-*`

The generator should consume a stable operation model rather than reaching
deeply into parser-library internals wherever possible.

## Command Tree Shape

Each registered API contributes one top-level command group named after the API
short name:

```text
restish <api> <operation> ...
```

Under that group, each included operation becomes a child command.

Built-in commands still take precedence over API short names. The generator does
not get to shadow core commands such as `api`, `cache`, or `setup`.

The set of reserved built-in command names should come from the actual root
command tree or a guard test that proves the reserved list is in sync. A stale
hand-maintained list can let generated APIs shadow core behavior.

## Operation Inclusion

An operation is eligible for generation when:

- it is not explicitly ignored
- the spec parsed successfully
- all required path variables can be mapped to declared parameters

If an operation cannot be generated safely, Restish should surface a clear
diagnostic rather than silently dropping it.

Empty or nil path items in an OpenAPI document are ignored safely. They should
not panic command generation, MCP tool generation, or any plugin-facing
operation export path.

## Naming

### Default Name

The preferred source of the command name is:

1. `x-cli-name`
2. `operationId`
3. fallback derived from HTTP method plus path

The fallback is important for compatibility. Operations without `operationId`
must still produce commands.

### Fallback Naming Rule

When no explicit name is provided, Restish should derive a stable kebab-case
name from the method and path, for example:

- `GET /users/{id}` -> `get-users-id`
- `POST /v1/invoices` -> `post-v1-invoices`

The exact normalization may evolve, but it must be deterministic and collision
aware.

### Aliases

Generated commands may have aliases from:

- `x-cli-aliases`
- compatibility aliases retained from v1 where useful

Alias collisions should be diagnosed rather than silently overwriting another
command.

## Hiding And Ignoring

CLI-shaping extensions should apply consistently across:

- operations
- paths
- parameters where applicable

That means `x-cli-ignore` and `x-cli-hidden` are not operation-only concepts in
the design, even if the current implementation still needs to catch up.

## Parameter Mapping

Parameters come from multiple OpenAPI scopes:

- path-item parameters
- operation-level parameters

The generator must merge these scopes according to OpenAPI rules before building
the command interface.

### Positional Arguments

Required path parameters are positional arguments in path order.

### Flags

Query, header, and cookie parameters become flags unless there is a documented
reason to make them positional.

Required non-path parameters should still be represented as required flags,
rather than silently becoming optional.

### Missing Path Parameters

If the path template references `{petId}` but the operation does not declare a
matching path parameter after scope merge, generation should fail for that
operation with a diagnostic. Leaving the literal template token in the URL is
not acceptable.

## Request Body Mapping

If the operation supports a request body, the generated command uses the same
body-construction model as the generic HTTP commands:

- shorthand positional assignments
- stdin merge or replacement
- content-type-aware encoding

Generated commands should not invent a separate body grammar.

## Server Resolution

The operation URL is not just `api base URL + path`. The design must account
for OpenAPI `servers` definitions at:

- document level
- path level
- operation level

The generator or request planner must honor those server blocks and merge them
with API registration rules such as `operation_base` or profile base URL
overrides.

## Help And Discoverability

Generated commands should feel like ordinary Cobra commands:

- summary and description come from the spec or CLI extensions
- examples may be surfaced when available
- required args and flags are visible in help
- shell completion works for generated commands too

This is why generated commands are registered into the normal root tree instead
of living behind a special sub-interpreter.

## Name-Collision Policy

Collisions can happen between:

- two generated operations
- generated commands and built-ins
- generated commands and plugin commands

The design rule is:

- built-ins win over everything else
- duplicate generated names are reported
- skipped commands should produce warnings or errors during generation

Silent shadowing is not acceptable.

## Example

Given:

```jsonc
{
  "apis": {
    "petstore": {
      "base_url": "https://api.example.com"
    }
  }
}
```

and:

```yaml
paths:
  /pets/{petId}:
    parameters:
      - name: tenant
        in: header
        required: true
        schema:
          type: string
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

Restish should generate a command roughly shaped like:

```bash
restish petstore pet <pet-id> --tenant acme --include owner
```

which resolves through the normal request pipeline to:

```text
GET https://api.example.com/pets/<pet-id>?include=owner
tenant: acme
```

## Startup Versus Execution

Generated command registration at startup uses cached or local spec data only.
Live network fetching belongs to explicit management commands, not routine root
tree construction.

## Compatibility

Because generated commands are one of the most visible v1-to-v2 behaviors, the
generator should actively restore low-cost v1 compatibility where it does not
conflict with safety or architecture, including:

- fallback naming without `operationId`
- useful aliases
- honoring `servers[]`
- preserving required-parameter semantics

## Alternatives Considered

### Generic HTTP Commands Only

Too weak; it gives up a major product advantage.

### Ahead-Of-Time Code Generation

Too heavy for the normal operator workflow.

### Separate Generated-Command Mode

Would make help, completion, and discovery less coherent.

## Relationship To Other Designs

- Design 006 defines the source spec/loading model.
- Design 008 defines request-body shorthand used by generated commands.
- Design 017 defines command resolution and completion expectations.
- Design 029 defines how generated commands enter the shared request pipeline.
- Design 031 defines compatibility expectations for user-visible naming changes.
