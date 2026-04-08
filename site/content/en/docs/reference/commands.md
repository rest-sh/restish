---
title: Commands
linkTitle: Commands
weight: 10
description: High-level reference for Restish command families, common workflows, and navigation.
---

# Commands Reference

Restish has two major command styles:

- generic HTTP commands such as `get`, `post`, `put`, `patch`, and `delete`
- API-aware commands that appear under a configured API name

## Core Command Families

### Generic HTTP Commands

Use these when you want to make a direct request quickly:

```bash
restish get https://httpbin.org/json
restish post https://httpbin.org/anything name=daniel active:=true
```

These commands work without any API registration step.

### API Management Commands

Use `api` commands to register, inspect, and manage APIs described by OpenAPI
or other supported loaders.

Common workflow:

```bash
restish api configure petstore https://api.example.com/openapi.json
restish api list
restish petstore --help
```

After configuration, Restish generates subcommands under the API name.

### Plugin Commands

Plugins can contribute new top-level command surfaces. For example, the MCP
plugin adds `restish mcp ...`.

See the [plugin quickstart](/docs/plugins/quickstart/) and
[plugin reference](/docs/reference/plugins/) for the extension model.

## Common Global Behavior

Most commands participate in the same shared runtime:

- profiles can inject base URLs, auth settings, TLS options, and defaults
- output can be reformatted with `-o`
- filters can project or transform structured responses
- retries, caching, and pagination apply where relevant
- plugins can intercept requests, responses, auth, loading, and formatting

## Finding Detailed Help

- Run `restish --help` for top-level command discovery.
- Run `restish <command> --help` for command-specific flags and examples.
- Run `restish <api-name> --help` after configuring an API to discover
  generated operations.
- Use the [guides](/docs/guides/) for workflows and the
  [reference section](/docs/reference/) for factual details.
