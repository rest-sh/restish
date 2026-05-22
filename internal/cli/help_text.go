package cli

const rootLongDefault = "**Restish** is a CLI for interacting with REST-ish HTTP APIs.\n\n" +
	"Every API deserves a CLI. Restish gives you:\n\n" +
	"- Generic HTTP commands for quick one-off requests.\n" +
	"- OpenAPI-backed commands for registered APIs.\n" +
	"- Output, filtering, auth, pagination, retries, caching, and plugins that stay friendly to the shell.\n\n" +
	"Run `restish api connect` when an API should become a named command surface. Run `restish get`, `restish post`, and the other HTTP commands when a direct URL is enough."

const versionLong = "Print the Restish version and exit.\n\n" +
	"Use this in bug reports, release checks, and scripts that need to confirm which Restish binary is running."

const apiLong = "Manage APIs registered in the local Restish config.\n\n" +
	"Registered APIs turn OpenAPI descriptions into generated commands with shell completion, persistent profiles, and auth-aware requests. Use `api connect` to add an API, `api sync` after its OpenAPI document changes, and `api set` for local profile edits."

const apiListLong = "List every API registered in the active Restish config.\n\n" +
	"Use `-o json` when scripts need stable fields such as API names, base URLs, operation counts, and profile counts. Human output is a compact inventory for deciding what to inspect, sync, or remove next."

const apiRemoveLong = "Remove a registered API from the local config.\n\n" +
	"This deletes the saved API definition, generated-command source, HTTP response cache entries, and API-scoped auth token cache entries for that name. Unreferenced shared auth-profile tokens used only by the removed API are cleared too. It does not contact the remote API, delete server-side resources, or remove unrelated cache entries."

const apiSyncLong = "Force re-fetch of the cached OpenAPI spec for a named API.\n\n" +
	"Use this after the API publishes new operations, updates parameter schemas, moves the discovered spec URL, or adds operation servers that generated commands should know about. Sync refreshes spec-derived API metadata, but preserves local profiles because they may contain credentials.\n\n" +
	"By default, sync follows the same-origin spec source already recorded for the API. Use `--allow-cross-origin-spec` only when you trust a `Link` header or saved spec source that points to an OpenAPI document on another host. Private, loopback, link-local, and unspecified follow targets are still rejected unless the original API is already private/local. Use `api set` to update `spec_url`, or reconnect with `--spec`, when you need to name a private spec URL directly."

const apiConnectLong = "Connect Restish to an API, discover its OpenAPI description, and save a named API profile.\n\n" +
	"Use this when repeated work against an API deserves generated commands, shell completion, auth setup, and profile-aware defaults.\n\n" +
	"Common choices:\n\n" +
	"- Use `--spec` when discovery is blocked, the API does not advertise its spec, or you want to pin setup to a known OpenAPI URL or local file.\n" +
	"- Use `--allow-cross-origin-spec` only when you trust a `Link` header that points to an OpenAPI document on another host. Private, loopback, link-local, and unspecified follow targets are still rejected unless the original API is already private/local; use `--spec` when you need to name a private spec URL directly.\n" +
	"- Use `--no-discover` to save a base URL without fetching a spec.\n" +
	"- Use `--replace` when reconnecting should replace existing profiles with generated OpenAPI or `x-cli-config` profile defaults. Without it, existing profiles are preserved, while API-level discovery fields are refreshed from the new connect run.\n" +
	"- Use `--yes` only for safe connect prompts you have already decided to accept in automation."

const apiInspectLong = "Print the saved config for one registered API as JSON.\n\n" +
	"Use this when you need the exact merged API entry, including profiles, headers, query defaults, auth config, spec URLs, and generated-command settings. Sensitive values may still be present if they are stored directly in config."

const apiSetLong = "Patch one registered API using Restish shorthand syntax.\n\n" +
	"Use this for durable local overrides such as profile URLs, default headers, query parameters, auth settings, and server variables. Patches are applied to the saved config file; they do not update the remote API or the cached OpenAPI document.\n\n" +
	"Run `restish api inspect <name>` first when you want to confirm the current config shape."

const apiAuthLong = "Manage auth material for a registered API profile.\n\n" +
	"Use these commands when a generated OpenAPI command reports missing auth, when you want to see which credentials satisfy secured operations, or when cached OAuth tokens need to be cleared.\n\n" +
	"Most commands honor `--rsh-profile` so you can inspect or update a non-default API profile."

const apiAuthAddLong = "Add or initialize a credential binding for an API profile.\n\n" +
	"Use this after `api auth inspect` reports a missing credential ID. When cached OpenAPI auth metadata is available, Restish can prefill auth settings and prompt for the parameters needed by that credential."

const apiAuthRemoveLong = "Remove one credential binding from an API profile.\n\n" +
	"This edits local Restish config only. It does not revoke remote tokens or delete cached OAuth tokens; run `api auth logout` when cached tokens should be cleared too."

const apiAuthLogoutLong = "Delete cached API auth tokens.\n\n" +
	"Use this when credentials changed, an OAuth grant should be refreshed, or a shared auth profile should forget cached tokens.\n\n" +
	"- Pass an API name to clear the current `--rsh-profile` token cache entry.\n" +
	"- Add `--all-profiles` to clear every profile for that API.\n" +
	"- Use `--auth-profile` to clear a shared auth profile cache without naming an API."

const apiAuthHeaderLong = "Print one auth header value that Restish would apply for an API profile.\n\n" +
	"Use this for debugging generated-command auth without sending a request. Pass `--operation` to inspect operation-specific security requirements, or `--credential` to inspect a named credential binding directly."

const apiAuthInspectLong = "Inspect auth readiness and material for an API profile.\n\n" +
	"By default this shows configured credentials, generated-operation coverage, and the auth values Restish would apply. Use `--operation` for operation-specific OpenAPI security requirements or `--credential` for one credential binding. Add `--redact` before sharing output so sensitive header, token, and credential values are masked."

const configLong = "Manage local Restish configuration.\n\n" +
	"The config stores registered APIs, profiles, auth settings, plugin settings, cache preferences, and output theme choices. Use `config show` for a redacted summary, `config path` to locate the file, and `config set` for scripted changes."

const configPathLong = "Print the active Restish config file path.\n\n" +
	"This honors `--rsh-config` and `RSH_CONFIG`, so it is the safest way to confirm which config a command will read or write."

const configShowLong = "Print the active config summary, or redacted JSON with `-o json`.\n\n" +
	"Human output shows counts and the config file path. JSON output is intended for inspection and support; sensitive auth values, credential-like headers, and credential-like query parameters are redacted where Restish recognizes them."

const configEditLong = "Open the active Restish config file in `$VISUAL` or `$EDITOR`.\n\n" +
	"Use this for manual config edits that are easier in an editor than with `config set`. Restish creates the config file if needed and preserves the platform-specific config path unless `--rsh-config` or `RSH_CONFIG` selects another file."

const configSetLong = "Patch the active Restish config using shorthand syntax.\n\n" +
	"Use this for durable scripted changes, for example cache settings, theme colors, API profile defaults, or auth profile entries. Prefer `api set` when the change only belongs to one registered API."

const configThemeLong = "Manage the terminal output highlighting theme.\n\n" +
	"Restish can use bundled themes, local JSON or JSONC files, direct URLs, or GitHub `user/repo` shorthand. Use `config theme list` to see bundled choices and `config theme reset` to return to the built-in default."

const configThemeSetLong = "Install a theme JSON or JSONC file and save it in config.\n\n" +
	"Sources may be bundled theme names, local files, HTTPS URLs, or GitHub `user/repo` shorthand. Remote sources are executable in the sense that they affect terminal rendering, so Restish asks for confirmation unless the source is already trusted or you pass `--yes`."

const configThemeListLong = "List bundled theme names.\n\n" +
	"The current configured bundled theme is marked with `*`. Custom local, URL, or GitHub themes are not expanded into this bundled list."

const configThemeResetLong = "Reset terminal output highlighting to the built-in theme.\n\n" +
	"This removes the saved theme override from config. It does not delete local theme files or remote sources."

const cacheLong = "Manage Restish's HTTP response cache.\n\n" +
	"The HTTP cache stores reusable responses for requests that are safe to cache. It is separate from the OpenAPI spec cache and OAuth token cache. Use `cache info` to inspect size, location, largest cached hosts, and API/profile usage, and `cache clear` when cached responses should no longer be reused."

const cacheInfoLong = "Print the HTTP response cache directory, size, entry count, oldest entry, and largest hosts.\n\n" +
	"TTY output includes a compact API/profile usage map. Human output also shows the largest cached hosts and API/profile namespaces with size percentages so you can see where disk space is going. Unregistered namespaces, such as old cache entries from a previous Restish version or manual cache files, are marked clearly and can be cleared by their namespace prefix. Use `-o json` for stable fields including host and API/profile breakdowns. This command does not inspect the OpenAPI spec cache or auth token cache."

const cacheClearLong = "Delete cached HTTP responses.\n\n" +
	"Omit the API name to clear every HTTP response cache entry. Pass an API name to clear entries for that registered API. Use `--direct` to clear direct URL requests that are not associated with a registered API. If an unregistered namespace remains from an older Restish version or manual cache files, pass the namespace prefix shown by `cache info` to clear it. OAuth tokens and cached OpenAPI documents are not removed."

const pluginLong = "Manage Restish plugins.\n\n" +
	"Plugins are executable programs that can add commands, content loaders, output formatters, hooks, or TLS signing behavior. Use `plugin list` to see discovered plugins, `plugin install` for trusted plugin binaries, and `plugin debug` when a plugin is discovered but not behaving correctly."

const pluginListLong = "List all discovered plugins and their capabilities.\n\n" +
	"Human output shows plugin names, versions, capabilities, command names, formatter names, loader content types, and descriptions when available. Use `-o json` for automation or diagnostics."

const pluginInstallLong = "Install a trusted plugin into Restish's plugin directory.\n\n" +
	"Plugins are executable programs. Restish reads the plugin manifest before installing, checks declared capabilities, and verifies protocol compatibility, but it does not sandbox plugin code or verify publisher identity. Install plugins only from sources you trust.\n\n" +
	"Sources can be local executable paths, commands on `PATH`, direct archive URLs, or GitHub release shorthand such as `rest-sh/restish mcp`.\n\n" +
	"Use `--yes` only after choosing a source you intend to trust. It skips the interactive confirmation prompt; it does not make the plugin safer."

const pluginRemoveLong = "Remove an installed plugin from the Restish plugin directory.\n\n" +
	"You can pass either the installed file name or the plugin manifest name. Restish refuses ambiguous manifest-name matches so you can delete the intended executable explicitly."

const pluginDebugLong = "Spawn a plugin and print decoded protocol messages to stderr.\n\n" +
	"Use this when a plugin is discovered but does not behave as expected. It shows the manifest/startup exchange and runtime messages so you can see whether the plugin, host, or protocol payload is failing.\n\n" +
	"Pass plugin arguments after the plugin name. Use `--` before arguments that could otherwise be interpreted by Restish."

const editLong = "Fetch a resource, edit it locally, then send the changed representation back.\n\n" +
	"Restish first sends `GET`, opens the response body as JSON or YAML, then sends either `PATCH` with JSON Merge Patch when supported or `PUT` otherwise. Use shorthand patch arguments for non-interactive edits.\n\n" +
	"Safety controls:\n\n" +
	"- Use `--dry-run` to print the diff without sending an update.\n" +
	"- Use `--no-editor` to print or patch the editable body without launching `$VISUAL` or `$EDITOR`.\n" +
	"- Use `--yes` only after reviewing the diff in automation."

const certLong = "Show the TLS certificate chain for an HTTPS server.\n\n" +
	"Use this to inspect certificate subjects, issuers, DNS names, validity windows, and expiry timing with the same TLS-related flags Restish uses for requests. `--warn-days` exits non-zero when the leaf certificate expires soon, which is useful in monitoring scripts."

const linksLong = "Perform a `GET` request and print hypermedia links found in the response.\n\n" +
	"Restish extracts links from `Link` headers, HAL `_links`, JSON:API links, Siren links, and JSON-LD `@id` fields. Pass relation names after the URI to filter the output to specific rels."

const doctorLong = "Diagnose Restish configuration and runtime paths.\n\n" +
	"Use this when Restish is reading the wrong config, permissions look suspicious, shell setup is incomplete, caches are in unexpected locations, or plugin discovery is confusing. Pass `-o json` for structured diagnostics."

const doctorAPILong = "Diagnose one registered API.\n\n" +
	"The report checks registration, spec cache freshness, generated operation availability, shallow auth readiness, and optional network reachability. Use `--check-network` when you want Restish to make a bounded request to the API base URL. For detailed credential coverage and auth material, run `restish api auth inspect <api>`."

const doctorPluginLong = "Diagnose one Restish plugin executable.\n\n" +
	"The report checks plugin discovery, executable status, manifest loading, declared capabilities, and Restish plugin protocol compatibility."

const shellLong = "Configure shell integration for Restish.\n\n" +
	"Shell setup prevents common glob-expansion issues and can install shell completion where supported. Use `shell setup <shell>` for the managed setup flow, or `shell completion` when you only need completion scripts."

const shellSetupLong = "Append a `noglob` wrapper or function for Restish to your shell startup file and install completion for supported shells.\n\n" +
	"Supported shells: zsh, bash, fish.\n\n" +
	"Use `--dry-run` to preview file changes, `--no-completion` to skip completion installation, and `--yes` only after you are comfortable with the startup-file change."

const completionLong = "Generate or install shell completion scripts.\n\n" +
	"Script generation writes to stdout for package managers and manual setup. `completion install` writes a generated script under Restish's config directory or a shell-native user completion directory, then updates shell startup files only when the shell requires it."

const completionInstallLong = "Install shell completion for your user account.\n\n" +
	"Supported shells: zsh and fish. The zsh installer writes the generated script under Restish's config directory and adds a managed source block to `~/.zshrc`. The fish installer writes to the shell's user completions directory."
