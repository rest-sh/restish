---
title: Environment Variables
linkTitle: Environment Variables
weight: 22
description: Reference for environment variables that affect config, cache, profiles, editors, terminal behavior, and HTTP transport.
---

Environment variables are useful for local defaults and automation, but command
line flags should be used when one invocation must be explicit.

## Config And Profiles

| Variable | Purpose |
| --- | --- |
| `RSH_CONFIG` | Explicit config file path. |
| `RSH_CONFIG_DIR` | Config directory override where supported. |
| `RSH_CACHE_DIR` | HTTP/spec cache directory override. |
| `XDG_CONFIG_HOME` | Base config directory; Restish uses `$XDG_CONFIG_HOME/restish/restish.json`. |
| `XDG_CACHE_HOME` | Base cache directory; Restish uses `$XDG_CACHE_HOME/restish`. |
| `RSH_PROFILE` | Default profile name. |

## Request Defaults

| Variable | Purpose |
| --- | --- |
| `RSH_TIMEOUT` | Default request timeout such as `15s`. |
| `RSH_NO_CACHE` | Bypass HTTP cache where supported. |
| `RSH_RETRY` | Default retry count where supported. |
| `RSH_RETRY_MAX_WAIT` | Default cap for `Retry-After`/`X-Retry-In`, such as `30s`. |

Flags override environment defaults for one command.

## Editor And Terminal

| Variable | Purpose |
| --- | --- |
| `VISUAL` | Preferred editor for `config edit` and `edit`. |
| `EDITOR` | Fallback editor. |
| `NO_COLOR` | Disable color where respected. |
| `CLICOLOR_FORCE` or `FORCE_COLOR` | Force color where respected. |

## Proxies

Restish uses Go's standard proxy environment behavior:

```bash
export HTTPS_PROXY=https://proxy.corp.test:8080
export HTTP_PROXY=http://proxy.corp.test:8080
export NO_PROXY=localhost,127.0.0.1,.corp.test
```

## Related Pages

- [Global Flags](../global-flags/)
- [Config](../config/)
- [Profiles](../profiles/)
