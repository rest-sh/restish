---
title: Troubleshooting
linkTitle: Troubleshooting
weight: 110
description: Diagnose common Restish setup, request, auth, output, pagination, spec, TLS, and plugin problems.
---

Use this page when the command ran but the result was surprising, or when the
command did not run because shell, config, auth, TLS, or plugin setup got in the
way.

## The Shell Rewrites My URL Or Filter

**Symptom:** A command with `?`, `&`, `[0]`, `[]`, or `*` fails before Restish
sends a request.

**Likely cause:** Your shell expanded the characters.

**How to confirm:** Quote the URL or filter and retry.

**Fix:**

```bash
restish shell setup zsh
restish 'api.rest.sh/images?format=jpeg&limit=1'
restish api.rest.sh/images --rsh-no-paginate -f 'body[0].self'
```

**Prevention:** Quote complex arguments in scripts and run `restish shell setup <shell>` for interactive use.

**Related docs:** [Shell Setup](/docs/getting-started/shell-setup/), [Query Syntax](/docs/reference/query-syntax/).

## I Expected JSON But Got Readable Output

**Symptom:** Output is human-readable instead of JSON.

**Likely cause:** TTY output defaults to `readable`.

**How to confirm:** Run with an explicit format.

**Fix:**

```bash
restish api.rest.sh/images -o json
```

**Prevention:** Use `-o json` in scripts and redirects when the format matters.

**Related docs:** [Output](/docs/guides/output/), [Output Defaults](/docs/reference/output-defaults/).

## I Redirected Output And Got The Server's Format

**Symptom:** A redirected response is CBOR, YAML, text, or another server format
instead of JSON.

**Likely cause:** Redirected unfiltered output saves response body bytes.

**How to confirm:** Run with an explicit format.

**Fix:**

```bash
restish api.rest.sh/content/cbor -o json > response.json
```

**Prevention:** Use `-o json` when a file or script needs JSON. Omit `-o` when
you want to save the response body unchanged.

**Related docs:** [Output Defaults](/docs/reference/output-defaults/), [Save a Response Unchanged](/docs/recipes/save-a-response-unchanged/).

## Config Directory Cannot Be Determined

**Symptom:** Restish says it cannot determine the config directory.

**Likely cause:** The environment has no explicit config path and no usable
home or platform config directory.

**Fix:** Set one of these before running Restish:

```bash
export RSH_CONFIG_DIR="$HOME/.config/restish"
export RSH_CACHE_DIR="$HOME/.cache/restish"
```

For project-local config, use `RSH_CONFIG=/absolute/path/restish.json` or
`--rsh-config /absolute/path/restish.json`.

**Related docs:** [Config](/docs/reference/config/), [Environment Variables](/docs/reference/environment-variables/).

## v1 Migration Backup Already Exists

**Symptom:** A v1-to-v2 migration mentions a `.bak.v1` backup, or you are
recovering after an interrupted migration.

**Likely cause:** Restish found a prior v1 backup directory.

**Fix:** Run Restish again. Matching backups are reused; different existing
backups are preserved and a numbered backup such as `.bak.v1.2` is created.
After migration, the old `apis.json` and `config.json` are removed from the
legacy location so they are not imported again.

**Related docs:** [Upgrade From v1](/docs/getting-started/upgrade-from-v1/).

## Content Negotiation Returned A Different Format

**Symptom:** The server returns CBOR, YAML, or another structured media type
instead of JSON.

**Likely cause:** Restish sends an `Accept` header listing all registered
content types with quality values.

**How to confirm:** Inspect the request with `/headers` or verbose mode.

**Fix:**

```bash
restish -H 'Accept: application/json' api.rest.sh/formats/json
restish -v api.rest.sh/headers
```

**Prevention:** Set an `Accept` header in a profile for APIs that negotiate aggressively.

**Related docs:** [Content Types](/docs/reference/content-types/), [Profiles](/docs/reference/profiles/).

## Auth Fails

**Symptom:** The API returns `401` or `403`.

**Likely cause:** Missing credential, wrong profile, expired token, or auth on a
public operation where it should have been suppressed.

**How to confirm:** Test against the safe auth fixtures.

**Fix:**

```bash
restish -H 'Authorization: Bearer docs-token' api.rest.sh/auth/bearer
restish api auth inspect example --raw-header Authorization
restish api auth list example
restish -v -p token api.rest.sh/auth/bearer
```

For generated OpenAPI commands, errors mentioning missing credential bindings
mean the selected profile does not have a `credentials.<id>` entry for that
operation. Errors mentioning missing requirement values mean the binding's
`satisfies` list does not include the required scope or role.

**Prevention:** Put auth in profiles and keep public/private operation security accurate in OpenAPI.

**Related docs:** [Authentication](/docs/guides/authentication/), [Profiles](/docs/reference/profiles/).

## Generated Commands Are Missing

**Symptom:** `restish myapi --help` does not show expected operations.

**Likely cause:** Spec discovery failed, the cached spec is stale, operation
names changed, or operations are hidden/ignored.

**How to confirm:**

```bash
restish api inspect myapi
restish api sync myapi
restish myapi --help
```

**Fix:** Set `spec_url`, sync the API, or update the OpenAPI document.

**Prevention:** Publish `/openapi.json` or service-description links and keep operation IDs stable.

**Related docs:** [API Setup and Discovery](/docs/guides/api-setup-and-discovery/), [OpenAPI Reference](/docs/reference/openapi-cli-integration/).

## Config Permissions Are Rejected

**Symptom:** Restish exits with a message that `restish.json` is
group/world-readable.

**Likely cause:** The config file permissions allow other local users to read
profiles, headers, or auth settings.

**How to confirm:**

```bash
ls -l ~/.config/restish/restish.json
```

**Fix:** Restrict the file to your user.

```bash
chmod 600 ~/.config/restish/restish.json
```

**Prevention:** Create config through Restish commands when possible; they write
private files by default.

**Related docs:** [Config](/docs/reference/config/), [Authentication](/docs/guides/authentication/).

## Pagination Changed My Output Shape

**Symptom:** A paginated endpoint behaves differently from a one-page endpoint.

**Likely cause:** Restish follows `next` links and may stream items unless a
document format or collect mode requires buffering.

**How to confirm:**

```bash
restish api.rest.sh/images --rsh-no-paginate -f links.next
restish api.rest.sh/images --rsh-collect -f '.body | length'
```

**Fix:** Use `--rsh-no-paginate`, `--rsh-max-pages`, `--rsh-max-items`, or `--rsh-collect` explicitly.

**Prevention:** Choose document output for whole results and record output for item streams.

**Related docs:** [Pagination](/docs/guides/pagination/), [Output](/docs/guides/output/).

## Live Stream Output Never Finishes

**Symptom:** A stream keeps running or `-o json` is not useful.

**Likely cause:** SSE and NDJSON streams may be unbounded, while JSON is a
document format.

**How to confirm:** Add an item cap or a timeout.

**Fix:**

```bash
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
restish api.rest.sh/events --rsh-max-items 3 -f data.message -o lines
```

**Prevention:** Use `--rsh-max-items` for fixed samples and `--rsh-timeout` for
time-bounded stream checks.

**Related docs:** [Streaming](/docs/guides/streaming/), [Output Formats](/docs/reference/output-formats/).

## The Body Is Hidden Because Status Failed

**Symptom:** A non-2xx response fails the command and interrupts a script.

**Likely cause:** HTTP status families map to exit codes.

**How to confirm:** Use a status fixture.

**Fix:**

```bash
restish api.rest.sh/status/404 --rsh-ignore-status-code
restish api.rest.sh/problem --rsh-ignore-status-code
```

**Prevention:** Use `--rsh-ignore-status-code` when error bodies are expected data.

**Related docs:** [Command Behavior](/docs/guides/command-behavior/).

## Cache Looks Stale

**Symptom:** A response does not reflect the latest server state.

**Likely cause:** A cacheable response was served from the local HTTP cache.

**How to confirm:** Run with verbose mode or bypass cache.

**Fix:**

```bash
restish api.rest.sh/cache --rsh-no-cache
restish cache info
restish cache clear
```

**Prevention:** Use `--rsh-no-cache` while debugging server state.

**Related docs:** [Retries and Caching](/docs/guides/retries-and-caching/), [Commands](/docs/reference/commands/).

## TLS Or mTLS Fails

**Symptom:** Certificate verification, custom CA, client certificate, or TLS
signer setup fails.

**Likely cause:** Unknown CA, wrong client cert/key pair, missing hardware token,
or plugin configuration error.

**How to confirm:**

```bash
restish cert api.rest.sh
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

**Fix:** Use the correct CA, client certificate, key, or TLS signer parameters.
Avoid `--rsh-insecure` except for short debugging.

**Prevention:** Store TLS settings in profiles for repeated use.

**Related docs:** [TLS](/docs/guides/tls/), [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/).

## Plugins Are Not Discovered Or Fail

**Symptom:** A plugin command or output format is unavailable, or a plugin exits with protocol errors.

**Likely cause:** The plugin is not installed, not executable, not in a discovered
location, or is using an incompatible protocol.

**How to confirm:**

```bash
restish plugin list
restish plugin debug ./path/to/plugin
```

**Fix:** Install the plugin, fix permissions, rebuild for v2, or inspect decoded messages with `plugin debug`.

**Prevention:** Keep operator docs separate from author protocol docs and verify plugin discovery after installation.

**Related docs:** [Install and Use Plugins](/docs/plugins/install-and-use/), [Plugin Messages](/docs/reference/plugin-messages/).

## A CRUD Example Changed Shared State

**Symptom:** `/items` examples produce data different from the docs.

**Likely cause:** The CRUD fixture is resettable but stateful enough for examples
to interact.

**How to confirm:**

```bash
restish api.rest.sh/items
```

**Fix:** Use unique IDs for create/update/delete examples, such as `docs-$USER` or a timestamp.

**Prevention:** Recipes that mutate `/items` should create their own item and clean it up.

**Related docs:** [Create, Patch, and Delete an Item](/docs/recipes/create-patch-and-delete-an-item-safely/).
