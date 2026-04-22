---
title: Environment Variables
linkTitle: Environment Variables
weight: 17
description: Reference for the environment variables Restish uses for config, cache, profiles, and request defaults.
---

Restish supports a small set of environment variables for choosing defaults
without editing every command or config file.

## Color And Terminal Behavior

- `NO_COLOR`: when set to any non-empty value, disables colorized output.
  This is the standard convention from https://no-color.org/ and takes
  precedence over `COLOR`.
- `NOCOLOR`: legacy alias for `NO_COLOR`.
- `COLOR=1`: force color where supported (overrides TTY auto-detection when
  not already disabled by `NO_COLOR`).
- `COLUMNS`: terminal width used when wrapping output; falls back to the
  detected terminal width or 80 if unset.
- `TERM`, `TERM_PROGRAM`, `KITTY_WINDOW_ID`: used to detect the active
  terminal type for image rendering protocol selection.

## Image Rendering

- `RSH_IMAGE_PROTOCOL`: explicitly set the image rendering protocol.
  Values: `kitty`, `iterm2`, `halfblock`.
  When unset, Restish detects the protocol from `KITTY_WINDOW_ID`, `TERM`, and
  `TERM_PROGRAM`.

## Editor

- `VISUAL`: editor command used by `restish edit` and `restish api edit`.
  Checked first; `EDITOR` is the fallback.
- `EDITOR`: fallback editor command when `VISUAL` is unset.
- `SHELL`: used by the setup hint to suggest which shell to configure.

## Config And Cache Locations

- `RSH_CONFIG_DIR`: override the config directory.
- `RSH_CACHE_DIR`: override the HTTP response cache directory.

These affect where Restish reads persistent config and cached HTTP responses.

## Request Defaults

- `RSH_PROFILE`: default active profile.
- `RSH_TIMEOUT`: default request timeout.
- `RSH_RETRY`: default retry count.
- `RSH_HEADER`: default header added as if passed with `-H`.
- `RSH_QUERY`: default query string parameter added as if passed with `-q`.
- `RSH_OUTPUT_FORMAT`: default output format.
- `RSH_FILTER`: default filter expression.
- `RSH_INSECURE`: when truthy (`1`, `true`, or `yes`), act like
  `--rsh-insecure`.
- `RSH_NO_CACHE`: when truthy (`1`, `true`, or `yes`), act like
  `--rsh-no-cache`.

These are useful when you want a shell-wide default instead of repeating flags
on each command.

`RSH_HEADER` and `RSH_QUERY` are especially useful for session-wide defaults
such as an `Accept` header or a shared API version query parameter.

Example:

```bash
export RSH_HEADER='Accept: application/json'
export RSH_OUTPUT_FORMAT=json
```

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
- [Images in the Terminal](/docs/guides/images-in-the-terminal/)
