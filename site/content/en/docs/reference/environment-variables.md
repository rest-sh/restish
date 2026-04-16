---
title: Environment Variables
linkTitle: Environment Variables
weight: 17
description: Reference for the environment variables Restish uses for config, cache, profiles, and request defaults.
---

Restish supports a small set of environment variables for choosing defaults
without editing every command or config file.

## Config And Cache Locations

- `RSH_CONFIG_DIR`: override the config directory
- `RSH_CACHE_DIR`: override the HTTP response cache directory

These affect where Restish reads persistent config and cached HTTP responses.

## Request Defaults

- `RSH_PROFILE`: default active profile
- `RSH_TIMEOUT`: default request timeout
- `RSH_RETRY`: default retry count

These are useful when you want a shell-wide default instead of repeating flags
on each command.

## Color And Terminal Behavior

- `NOCOLOR=1`: disable colorized output
- `COLOR=1`: force color where supported

These are mainly useful when terminal auto-detection is not doing what you
want.

## Choosing Between Env Vars, Flags, And Config

The practical precedence order is:

1. command-line flags
2. environment variables
3. config file defaults

Use environment variables when:

- you want a session-level default
- the value changes by shell or machine
- you do not want to edit persistent config for a temporary need

## Related Pages

- [Config](../config/)
- [Global Flags](../global-flags/)
- [Profiles](../profiles/)
