---
title: Environment Variables
linkTitle: Environment Variables
weight: 22
description: Reference for environment variables that affect config, cache, profiles, editors, terminal behavior, and HTTP transport.
---

Environment variables are useful for local defaults and automation, but command
line flags should be used when one invocation must be explicit.

<!-- BEGIN GENERATED: restish-docgen environment-variables -->
Generated from production source environment-variable usage plus Go's standard proxy environment contract.

### Config And Profiles

| Variable | Purpose | Source |
| --- | --- | --- |
| `RSH_CONFIG` | Explicit config file path. It selects one config file for the invocation. | config paths |
| `RSH_CONFIG_DIR` | Config directory override; Restish uses `restish.json` inside this directory. | config paths |
| `RSH_CACHE_DIR` | HTTP/spec cache directory override. | config paths |
| `XDG_CONFIG_HOME` | Base config directory; Restish uses `$XDG_CONFIG_HOME/restish/restish.json`. | config paths |
| `XDG_CACHE_HOME` | Base cache directory; Restish uses `$XDG_CACHE_HOME/restish`. | config paths |
| `RSH_PROFILE` | Default API profile name. The `--rsh-profile` flag wins for one command. | global flags |
| `RSH_AUTH` | Default generated-operation auth override, such as `PartnerKey` or `UserOAuth+PartnerKey`. | global flags |

### Request Defaults

| Variable | Purpose | Source |
| --- | --- | --- |
| `RSH_HEADER` | Comma-separated default request headers in `Name: Value` form; escape literal commas as `\,`. | global flags |
| `RSH_QUERY` | Comma-separated default query parameters in `key=value` form; escape literal commas as `\,`. | global flags |
| `RSH_TIMEOUT` | Default request timeout such as `15s`. | global flags |
| `RSH_FILTER` | Default response filter expression. | global flags |
| `RSH_NO_CACHE` | Bypass HTTP cache where supported. | global flags |
| `RSH_INSECURE` | Disable TLS certificate verification when truthy. | global flags |
| `RSH_RETRY` | Default retry count where supported. | global flags |
| `RSH_RETRY_UNSAFE` | Allow retry replay for POST, PUT, PATCH, and DELETE when truthy. | global flags |
| `RSH_RETRY_MAX_WAIT` | Default cap for `Retry-After` / `X-Retry-In`, such as `30s`. | global flags |

### Editor And Terminal

| Variable | Purpose | Source |
| --- | --- | --- |
| `RSH_OUTPUT_FORMAT` | Default rendered body format for `-o` / `--rsh-output-format`. | global flags |
| `RSH_PRINT` | Default `--rsh-print` output parts, such as `b` for compact rendered output in scripts. | global flags |
| `VISUAL` | Preferred editor for `config edit` and `edit`. | editor |
| `EDITOR` | Fallback editor for `config edit` and `edit`. | editor |
| `BROWSER` | Browser binary used for OAuth authorization-code flows. When set, Restish invokes it directly instead of `xdg-open` (Linux), `open` (macOS), or `cmd /c start` (Windows). | oauth browser |
| `GLAMOUR_STYLE` | Markdown rendering style for markdown-formatted terminal output. | output |
| `RSH_IMAGE_PROTOCOL` | Force terminal image rendering protocol: `kitty`, `iterm2`, or `halfblock`. | output |
| `KITTY_WINDOW_ID` | Used to auto-detect Kitty image support. | output |
| `TERM` | Used to auto-detect Kitty terminal support. | output |
| `TERM_PROGRAM` | Used to auto-detect iTerm2-style image support. | output |
| `COLUMNS` | Terminal width hint for half-block image rendering. | output |
| `SHELL` | Used for first-run shell setup hints. | shell setup |
| `NO_COLOR` | Disable color where respected. | output |
| `NOCOLOR` | Disable color; supported as an older spelling alongside `NO_COLOR`. | output |
| `COLOR` | Force color where respected. | output |

### Plugin Runtime

| Variable | Purpose | Source |
| --- | --- | --- |
| `RSH_COMMAND_PLUGIN_DISCOVERY_TIMEOUT` | Override command-plugin startup discovery timeout. | plugin runtime |
| `RSH_COMMAND_PLUGIN_SHUTDOWN_GRACE` | Override command-plugin shutdown grace period. | plugin runtime |

### Plugin Installation

| Variable | Purpose | Source |
| --- | --- | --- |
| `GITHUB_TOKEN` | Bearer token used for GitHub release API requests during `restish plugin install owner/repo plugin`. | plugin install |

### Proxies

| Variable | Purpose | Source |
| --- | --- | --- |
| `HTTPS_PROXY` | Standard Go HTTPS proxy setting used by Restish HTTP transports. | Go HTTP transport |
| `HTTP_PROXY` | Standard Go HTTP proxy setting used by Restish HTTP transports. | Go HTTP transport |
| `NO_PROXY` | Standard Go proxy bypass list used by Restish HTTP transports. | Go HTTP transport |
<!-- END GENERATED -->

Flags override environment defaults for one command. Restish also uses Go's
standard proxy environment behavior:

```bash
export HTTPS_PROXY=https://proxy.corp.test:8080
export HTTP_PROXY=http://proxy.corp.test:8080
export NO_PROXY=localhost,127.0.0.1,.corp.test
```

## Related Pages

- [Global Flags](../global-flags/)
- [Config](../config/)
- [Profiles](../profiles/)
