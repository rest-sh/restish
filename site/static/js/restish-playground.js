(function () {
  const supportedVerbs = new Set(["get", "head", "options", "post", "put", "patch", "delete"]);
  const valueFlags = new Map([
    ["-H", "header"],
    ["--rsh-header", "header"],
    ["-q", "query"],
    ["--rsh-query", "query"],
    ["-c", "contentType"],
    ["--rsh-content-type", "contentType"],
    ["-o", "outputFormat"],
    ["--rsh-output-format", "outputFormat"],
    ["--rsh-print", "print"],
    ["-f", "filter"],
    ["--rsh-filter", "filter"],
    ["--rsh-filter-lang", "filterLang"],
    ["--rsh-columns", "columns"],
    ["--rsh-sort-by", "sortBy"],
    ["--rsh-timeout", "timeout"],
    ["-t", "timeout"],
    ["--rsh-max-pages", "maxPages"],
    ["--rsh-max-items", "maxItems"],
    ["--rsh-retry", "retry"],
    ["--rsh-retry-max-wait", "retryMaxWait"]
  ]);
  const supportedOutputFormats = new Set(["auto", "json", "yaml", "ndjson", "lines", "table", "image", "gron", "csv", "toon"]);
  const boolFlags = new Map([
    ["-v", "verbose"],
    ["-vv", "verbose"],
    ["--verbose", "verbose"],
    ["--rsh-headers", "headersOnly"],
    ["--rsh-no-cache", "noCache"],
    ["--rsh-no-paginate", "noPaginate"],
    ["--rsh-ignore-status-code", "ignoreStatus"],
    ["--rsh-collect", "collect"],
    ["--rsh-retry-unsafe", "retryUnsafe"]
  ]);
  const generatedValueFlags = new Map([
    ["--api-key", "api_key"],
    ["--api_key", "api_key"],
    ["--chunk-size", "chunk_size"],
    ["--chunk_size", "chunk_size"],
    ["--count", "count"],
    ["--cursor", "cursor"],
    ["--delay", "delay"],
    ["--duration", "duration"],
    ["--failures", "failures"],
    ["--format", "format"],
    ["--key", "key"],
    ["--limit", "limit"],
    ["--numbytes", "numbytes"],
    ["--per-page", "per_page"],
    ["--per_page", "per_page"],
    ["--retry-after", "retry-after"],
    ["--search", "search"],
    ["--seed", "seed"],
    ["--status", "status"],
    ["--status-code", "status_code"],
    ["--status_code", "status_code"],
    ["--x-retry-in", "x-retry-in"]
  ]);
  const generatedBoolFlags = new Map([
    ["--private", "private"]
  ]);
  const exampleOperations = new Map(Object.entries({
    "create-item": { method: "POST", path: "/items" },
    "delete-book": { method: "DELETE", path: "/books/{book-id}", pathParams: ["book-id"] },
    "delete-echo": { method: "DELETE", path: "/" },
    "delete-item": { method: "DELETE", path: "/items/{item-id}", pathParams: ["item-id"] },
    "delete-method": { method: "DELETE", path: "/delete" },
    "get-absolute-redirect": { method: "GET", path: "/absolute-redirect/{n}", pathParams: ["n"] },
    "get-accept-image": { method: "GET", path: "/image" },
    "get-anything": { method: "GET", path: "/anything" },
    "get-anything-path": { method: "GET", path: "/anything/{path}", pathParams: ["path"] },
    "get-auth-api-key-header": { method: "GET", path: "/auth/api-key-header", headers: [["X-API-Key", "docs-key"]] },
    "get-auth-api-key-query": { method: "GET", path: "/auth/api-key-query", query: [["api_key", "docs-query-key"]] },
    "get-auth-basic": { method: "GET", path: "/auth/basic", headers: [["Authorization", "Basic ZG9jczpkb2Nz"]] },
    "get-auth-bearer": { method: "GET", path: "/auth/bearer", headers: [["Authorization", "Bearer docs-token"]] },
    "get-base64-decode": { method: "GET", path: "/base64/decode/{value}", pathParams: ["value"] },
    "get-base64-encode": { method: "GET", path: "/base64/encode/{value}", pathParams: ["value"] },
    "get-book": { method: "GET", path: "/books/{book-id}", pathParams: ["book-id"] },
    "get-brotli": { method: "GET", path: "/brotli" },
    "get-bytes": { method: "GET", path: "/bytes/{n}", pathParams: ["n"] },
    "get-cache": { method: "GET", path: "/cache" },
    "get-cached": { method: "GET", path: "/cached/{seconds}", pathParams: ["seconds"] },
    "get-cookies": { method: "GET", path: "/cookies" },
    "get-cookies-delete": { method: "GET", path: "/cookies/delete" },
    "get-cookies-set": { method: "GET", path: "/cookies/set" },
    "get-deflate": { method: "GET", path: "/deflate" },
    "get-drip": { method: "GET", path: "/drip" },
    "get-echo": { method: "GET", path: "/" },
    "get-etag": { method: "GET", path: "/etag/{etag}", pathParams: ["etag"] },
    "get-events": { method: "GET", path: "/events" },
    "get-example": { method: "GET", path: "/example" },
    "get-flaky": { method: "GET", path: "/flaky" },
    "get-format": { method: "GET", path: "/formats/{format}", pathParams: ["format"] },
    "get-gzip": { method: "GET", path: "/gzip" },
    "get-headers": { method: "GET", path: "/headers" },
    "get-html": { method: "GET", path: "/html" },
    "get-image": { method: "GET", path: "/images/{type}", pathParams: ["type"], defaults: { type: "jpeg" } },
    "get-ip": { method: "GET", path: "/ip" },
    "get-item": { method: "GET", path: "/items/{item-id}", pathParams: ["item-id"] },
    "get-logs": { method: "GET", path: "/logs" },
    "get-method": { method: "GET", path: "/get" },
    "get-problem": { method: "GET", path: "/problem" },
    "get-range": { method: "GET", path: "/range/{n}", pathParams: ["n"] },
    "get-redirect": { method: "GET", path: "/redirect/{n}", pathParams: ["n"] },
    "get-redirect-to": { method: "GET", path: "/redirect-to", requiredQuery: ["url"] },
    "get-relative-redirect": { method: "GET", path: "/relative-redirect/{n}", pathParams: ["n"] },
    "get-response-headers": { method: "GET", path: "/response-headers" },
    "get-slow": { method: "GET", path: "/slow" },
    "get-sse-metrics": { method: "GET", path: "/sse/metrics" },
    "get-status": { method: "GET", path: "/status/{code}", pathParams: ["code"], defaults: { code: "200" } },
    "get-stream-bytes": { method: "GET", path: "/stream-bytes/{n}", pathParams: ["n"] },
    "get-types-example": { method: "GET", path: "/types" },
    "get-user-agent": { method: "GET", path: "/user-agent" },
    "get-uuid": { method: "GET", path: "/uuid" },
    "get-xml": { method: "GET", path: "/xml" },
    "head-method": { method: "HEAD", path: "/head" },
    "list-books": { method: "GET", path: "/books" },
    "list-images": { method: "GET", path: "/images" },
    "list-items": { method: "GET", path: "/items" },
    "options-method": { method: "OPTIONS", path: "/options" },
    "patch-book": { method: "PATCH", path: "/books/{book-id}", pathParams: ["book-id"] },
    "patch-echo": { method: "PATCH", path: "/" },
    "patch-item": { method: "PATCH", path: "/items/{item-id}", pathParams: ["item-id"] },
    "patch-method": { method: "PATCH", path: "/patch" },
    "post-echo": { method: "POST", path: "/" },
    "post-login": { method: "POST", path: "/login" },
    "post-method": { method: "POST", path: "/post" },
    "put-book": { method: "PUT", path: "/books/{book-id}", pathParams: ["book-id"] },
    "put-echo": { method: "PUT", path: "/" },
    "put-method": { method: "PUT", path: "/put" },
    "put-types-example": { method: "PUT", path: "/types" }
  }));
  const rootCompletions = [
    "api",
    "cache",
    "config",
    "delete",
    "edit",
    "example",
    "get",
    "head",
    "links",
    "options",
    "patch",
    "plugin",
    "post",
    "put",
    "shell"
  ];
  const shellCompletions = ["completion", "setup"];
  const shellCompletionCompletions = ["bash", "fish", "install", "powershell", "zsh"];
  const apiCompletions = ["connect", "info", "list", "profiles", "set", "sync"];
  const configCompletions = ["edit", "get", "path", "set"];
  const cacheCompletions = ["clear", "dir", "list"];
  const pluginCompletions = ["debug", "install", "list", "remove"];
  const contentTypeCompletions = ["json", "form"];
  const printCompletions = ["hbp", "hbpc", "h", "b", "Hhbp"];
  const filterLangCompletions = ["restish", "jq"];
  const imageFormatCompletions = ["jpeg", "png", "webp", "gif", "heic"];
  const documentFormatCompletions = ["json", "yaml", "xml", "html", "text", "csv"];
  const statusCodeCompletions = ["200", "201", "204", "301", "302", "400", "401", "403", "404", "409", "422", "429", "500", "503"];
  const examplePathCompletions = [
    "api.rest.sh/",
    "api.rest.sh/auth/api-key-header",
    "api.rest.sh/auth/api-key-query",
    "api.rest.sh/books",
    "api.rest.sh/books/restish-tour",
    "api.rest.sh/cache",
    "api.rest.sh/events",
    "api.rest.sh/example",
    "api.rest.sh/flaky",
    "api.rest.sh/formats/json",
    "api.rest.sh/headers",
    "api.rest.sh/images",
    "api.rest.sh/images/jpeg",
    "api.rest.sh/items",
    "api.rest.sh/items/docs-demo",
    "api.rest.sh/logs",
    "api.rest.sh/problem",
    "api.rest.sh/status/200",
    "api.rest.sh/types"
  ];
  const exampleAliasPathCompletions = examplePathCompletions
    .filter((item) => item !== "api.rest.sh/")
    .map((item) => `example/${item.slice("api.rest.sh/".length)}`);

  function shellWords(input) {
    const words = [];
    let current = "";
    let quote = "";
    let escaped = false;
    const text = input.replace(/\\\r?\n/g, " ");

    for (const ch of text) {
      if (escaped) {
        current += ch;
        escaped = false;
        continue;
      }
      if (ch === "\\") {
        escaped = true;
        continue;
      }
      if (quote) {
        if (ch === quote) {
          quote = "";
        } else {
          current += ch;
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (/\s/.test(ch)) {
        if (current) {
          words.push(current);
          current = "";
        }
        continue;
      }
      current += ch;
    }

    if (quote) {
      throw new Error("Unclosed quote. The browser preview supports shell-style single and double quotes.");
    }
    if (escaped) {
      current += "\\";
    }
    if (current) {
      words.push(current);
    }
    return words;
  }

  function shellWordsPartial(input) {
    const words = [];
    let current = "";
    let quote = "";
    let escaped = false;
    const text = input.replace(/\\\r?\n/g, " ");

    for (const ch of text) {
      if (escaped) {
        current += ch;
        escaped = false;
        continue;
      }
      if (ch === "\\") {
        escaped = true;
        continue;
      }
      if (quote) {
        if (ch === quote) {
          quote = "";
        } else {
          current += ch;
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (/\s/.test(ch)) {
        if (current) {
          words.push(current);
          current = "";
        }
        continue;
      }
      current += ch;
    }

    if (escaped) {
      current += "\\";
    }
    if (current || !/\s$/.test(text)) {
      words.push(current);
    }
    return words;
  }

  function completeCommand(input, cursor) {
    const position = typeof cursor === "number" ? cursor : input.length;
    const range = currentTokenRange(input, position);
    if (/[`'"$\\]/.test(range.tokenBeforeCursor)) {
      return { value: input, cursor: position, matches: [], applied: false };
    }

    const candidates = completionCandidates(input, position, range.tokenBeforeCursor);
    const matches = uniqueSorted(candidates).filter((item) => item.toLowerCase().startsWith(range.tokenBeforeCursor.toLowerCase()));
    if (!matches.length) {
      return { value: input, cursor: position, matches: [], applied: false };
    }

    let replacement = "";
    let applied = false;
    if (matches.length === 1) {
      replacement = completionInsertText(matches[0], true);
      applied = replacement !== range.fullToken;
    } else {
      replacement = commonPrefix(matches);
      applied = replacement.length > range.tokenBeforeCursor.length;
    }

    if (!applied) {
      return { value: input, cursor: position, matches, applied: false };
    }

    const value = input.slice(0, range.start) + replacement + input.slice(range.end);
    return {
      value,
      cursor: range.start + replacement.length,
      matches,
      applied: true
    };
  }

  function currentTokenRange(input, cursor) {
    let start = cursor;
    while (start > 0 && !/\s/.test(input[start - 1])) {
      start -= 1;
    }
    let end = cursor;
    while (end < input.length && !/\s/.test(input[end])) {
      end += 1;
    }
    return {
      start,
      end,
      tokenBeforeCursor: input.slice(start, cursor),
      fullToken: input.slice(start, end)
    };
  }

  function completionCandidates(input, cursor, currentPrefix) {
    const before = input.slice(0, cursor);
    const endsAtBoundary = /\s$/.test(before);
    const partialWords = shellWordsPartial(before);
    const completedWords = endsAtBoundary ? partialWords : partialWords.slice(0, -1);
    const tokenIndex = endsAtBoundary ? partialWords.length : Math.max(0, partialWords.length - 1);

    if (tokenIndex === 0) {
      return ["restish"];
    }
    if (completedWords[0] !== "restish") {
      return [];
    }

    const inlineValue = inlineFlagValueCandidates(currentPrefix);
    if (inlineValue) {
      return inlineValue;
    }

    const previous = completedWords[completedWords.length - 1] || "";
    const flagValues = valueCandidatesForFlag(previous);
    if (flagValues) {
      return flagValues;
    }

    if (currentPrefix.startsWith("-")) {
      return flagCompletions(completedWords);
    }

    if (tokenIndex === 1) {
      return rootCompletions.concat(requestTargetCompletions(completedWords, currentPrefix));
    }

    const first = completedWords[1];
    if (first === "example") {
      return exampleCommandCompletions(completedWords);
    }
    if (first === "shell") {
      return nestedCompletions(completedWords, [shellCompletions, shellCompletionCompletions]);
    }
    if (first === "api") {
      return nestedCompletions(completedWords, [apiCompletions]);
    }
    if (first === "config") {
      return nestedCompletions(completedWords, [configCompletions]);
    }
    if (first === "cache") {
      return nestedCompletions(completedWords, [cacheCompletions]);
    }
    if (first === "plugin") {
      return nestedCompletions(completedWords, [pluginCompletions]);
    }
    if (first === "links") {
      return linksCompletions(completedWords);
    }
    if (supportedVerbs.has(first) || first === "edit") {
      return requestTargetCompletions(completedWords, currentPrefix);
    }
    if (looksLikeURL(first) || first.startsWith("example/")) {
      return bodyOrFlagHintCompletions(currentPrefix);
    }
    return requestTargetCompletions(completedWords, currentPrefix);
  }

  function inlineFlagValueCandidates(currentPrefix) {
    const match = currentPrefix.match(/^(--[^=]+=)(.*)$/);
    if (!match) {
      return null;
    }
    const flag = match[1].slice(0, -1);
    const values = valueCandidatesForFlag(flag);
    return values ? values.map((value) => match[1] + value) : null;
  }

  function valueCandidatesForFlag(flag) {
    if (flag === "-o" || flag === "--rsh-output-format") {
      return Array.from(supportedOutputFormats);
    }
    if (flag === "-c" || flag === "--rsh-content-type") {
      return contentTypeCompletions;
    }
    if (flag === "--rsh-print") {
      return printCompletions;
    }
    if (flag === "--rsh-filter-lang") {
      return filterLangCompletions;
    }
    if (flag === "--format") {
      return documentFormatCompletions.concat(imageFormatCompletions);
    }
    if (flag === "--status" || flag === "--status-code" || flag === "--status_code") {
      return statusCodeCompletions;
    }
    if (flag === "--private") {
      return ["true", "false"];
    }
    return null;
  }

  function flagCompletions(completedWords) {
    const flags = Array.from(valueFlags.keys())
      .concat(Array.from(boolFlags.keys()))
      .concat(Array.from(generatedValueFlags.keys()))
      .concat(Array.from(generatedBoolFlags.keys()));
    if (completedWords[1] !== "example") {
      return flags.filter((item) => !generatedValueFlags.has(item) && !generatedBoolFlags.has(item));
    }
    return flags;
  }

  function exampleCommandCompletions(completedWords) {
    if (completedWords.length <= 2) {
      return Array.from(exampleOperations.keys());
    }
    const op = completedWords[2];
    if (op === "get-image") {
      return imageFormatCompletions;
    }
    if (op === "get-format") {
      return documentFormatCompletions.concat(imageFormatCompletions);
    }
    if (op === "get-status") {
      return statusCodeCompletions;
    }
    if (op === "get-item" || op === "patch-item" || op === "delete-item") {
      return ["docs-demo", "docs-second"];
    }
    if (op === "get-book" || op === "put-book" || op === "patch-book" || op === "delete-book") {
      return ["restish-tour", "api-field-guide"];
    }
    return bodyOrFlagHintCompletions("");
  }

  function nestedCompletions(completedWords, levels) {
    const depth = completedWords.length - 2;
    return levels[depth] || [];
  }

  function linksCompletions(completedWords) {
    if (completedWords.length <= 2) {
      return requestTargetCompletions(completedWords, "");
    }
    return ["self", "next", "prev", "first", "last", "item"];
  }

  function requestTargetCompletions(completedWords, currentPrefix) {
    if (currentPrefix.startsWith("example/")) {
      return exampleAliasPathCompletions;
    }
    if (currentPrefix.startsWith("https://api.rest.sh/")) {
      return examplePathCompletions.map((item) => item.replace(/^api\.rest\.sh/, "https://api.rest.sh"));
    }
    return examplePathCompletions;
  }

  function bodyOrFlagHintCompletions(currentPrefix) {
    if (currentPrefix.startsWith("-")) {
      return flagCompletions(["restish"]);
    }
    return ["name:", "title:", "tags[]:", "enabled:", "count:"];
  }

  function completionInsertText(value, singleMatch) {
    if (!singleMatch) {
      return value;
    }
    if (value.endsWith(":") || value.endsWith("=") || value.endsWith("/")) {
      return value;
    }
    return value + " ";
  }

  function uniqueSorted(values) {
    return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
  }

  function commonPrefix(values) {
    if (!values.length) {
      return "";
    }
    let prefix = values[0];
    for (const value of values.slice(1)) {
      let index = 0;
      while (index < prefix.length && index < value.length && prefix[index].toLowerCase() === value[index].toLowerCase()) {
        index += 1;
      }
      prefix = prefix.slice(0, index);
      if (!prefix) {
        break;
      }
    }
    return prefix;
  }

  function parseCommand(command) {
    const cleanCommand = command.trim().replace(/^\$\s*/, "");
    if (hasUnsupportedShellSyntax(cleanCommand)) {
      throw new Error("Shell pipes, redirects, substitutions, and environment expansion require the real Restish CLI.");
    }

    const tokens = shellWords(cleanCommand);
    if (tokens.includes("|")) {
      throw new Error("Shell pipes, redirects, substitutions, and environment expansion require the real Restish CLI.");
    }
    if (tokens[0] !== "restish") {
      throw new Error("This preview only runs commands that start with `restish`.");
    }

    const flags = {
      headers: [],
      query: [],
      contentType: "",
      outputFormat: "",
      filter: "",
      filterLang: "",
      columns: "",
      sortBy: "",
      timeout: "",
      maxPages: "",
      maxItems: "",
      retry: "",
      retryMaxWait: "",
      generatedParams: [],
      verbose: false,
      print: "",
      headersOnly: false,
      noCache: false,
      noPaginate: false,
      collect: false,
      retryUnsafe: false
    };
    const args = [];

    for (let i = 1; i < tokens.length; i += 1) {
      let token = tokens[i];
      let inlineValue = "";
      if (token.startsWith("--") && token.includes("=")) {
        const parts = token.split(/=(.*)/s);
        token = parts[0];
        inlineValue = parts[1] || "";
      }
      if (valueFlags.has(token)) {
        const name = valueFlags.get(token);
        const value = inlineValue || tokens[i + 1];
        if (!value) {
          throw new Error(`${token} needs a value.`);
        }
        if (!inlineValue) {
          i += 1;
        }
        if (name === "header") {
          flags.headers.push(value);
        } else if (name === "query") {
          flags.query.push(value);
        } else {
          flags[name] = value;
        }
        continue;
      }
      if (boolFlags.has(token)) {
        flags[boolFlags.get(token)] = true;
        continue;
      }
      if (generatedBoolFlags.has(token)) {
        const value = inlineValue ? parseBoolFlagValue(token, inlineValue) : true;
        flags.generatedParams.push([generatedBoolFlags.get(token), String(value)]);
        continue;
      }
      if (generatedValueFlags.has(token)) {
        const name = generatedValueFlags.get(token);
        const value = inlineValue || tokens[i + 1];
        if (!value) {
          throw new Error(`${token} needs a value.`);
        }
        if (!inlineValue) {
          i += 1;
        }
        flags.generatedParams.push([name, value]);
        continue;
      }
      if (token.startsWith("-")) {
        throw new Error(`${token} is not supported in the browser preview. Use the real Restish CLI for this feature.`);
      }
      args.push(token);
    }

    if (!args.length) {
      throw new Error("Add a URL or supported docs API command.");
    }
    if (flags.outputFormat && !supportedOutputFormats.has(flags.outputFormat)) {
      throw new Error(`Output format \`${flags.outputFormat}\` is not supported in the browser preview.`);
    }
    let method = "GET";
    let url = "";
    let bodyArgs = [];
    let mode = "http";
    let linkRels = [];

    if (supportedVerbs.has(args[0].toLowerCase())) {
      method = args[0].toUpperCase();
      url = args[1] || "";
      bodyArgs = args.slice(2);
    } else if (args[0] === "links") {
      mode = "links";
      url = args[1] || "";
      linkRels = args.slice(2);
    } else if (args[0] === "edit") {
      mode = "edit";
      method = "GET";
      url = args[1] || "";
      bodyArgs = args.slice(2);
    } else if (looksLikeURL(args[0])) {
      if (flags.generatedParams.length) {
        throw new Error("Generated operation parameter flags such as `--format` or `--limit` only work with `restish example ...` in the browser preview.");
      }
      url = args[0];
      bodyArgs = args.slice(1);
      if (bodyArgs.length) {
        method = "POST";
      }
    } else if (args[0] === "example") {
      const mapped = mapExampleCommand(args.slice(1), flags.generatedParams);
      method = mapped.method;
      url = mapped.url;
      bodyArgs = mapped.bodyArgs;
      for (const header of mapped.headers || []) {
        flags.headers.push(`${header[0]}: ${header[1]}`);
      }
      for (const query of mapped.query || []) {
        flags.query.push(`${query[0]}=${query[1]}`);
      }
    } else if (/^example\//.test(args[0])) {
      if (flags.generatedParams.length) {
        throw new Error("Generated operation parameter flags such as `--format` or `--limit` only work with `restish example <operation>` in the browser preview.");
      }
      url = `https://api.rest.sh/${args[0].slice("example/".length)}`;
      bodyArgs = args.slice(1);
      if (bodyArgs.length) {
        method = "POST";
      }
    } else if (args[0] === "api" || args[0] === "plugin" || args[0] === "shell" || args[0] === "config" || args[0] === "cache") {
      throw new Error(`\`restish ${args[0]}\` changes local CLI state or setup. Run it in a terminal.`);
    } else {
      throw new Error(`\`${args[0]}\` is not in the browser preview command map.`);
    }

    if (!url) {
      throw new Error("Missing request URL.");
    }
    url = normalizeURL(url);
    if (!url) {
      throw new Error("Use an absolute URL or a host-like URL such as `api.rest.sh/images`.");
    }

    return { method, url, bodyArgs, flags, mode, linkRels };
  }

  function parseBoolFlagValue(name, value) {
    if (/^(true|1|yes|on)$/i.test(value)) {
      return true;
    }
    if (/^(false|0|no|off)$/i.test(value)) {
      return false;
    }
    throw new Error(`${name} needs a boolean value when using ${name}=value.`);
  }

  function hasUnsupportedShellSyntax(input) {
    let quote = "";
    let escaped = false;
    for (const ch of input) {
      if (escaped) {
        escaped = false;
        continue;
      }
      if (ch === "\\") {
        escaped = true;
        continue;
      }
      if (quote) {
        if (ch === quote) {
          quote = "";
        } else if (quote === "\"" && (ch === "$" || ch === "`")) {
          return true;
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (/[|;&<>`$]/.test(ch)) {
        return true;
      }
    }
    return false;
  }

  function normalizeURL(value) {
    if (/^https?:\/\//i.test(value)) {
      return value;
    }
    if (looksLikeBareURL(value)) {
      return `https://${value}`;
    }
    return "";
  }

  function looksLikeURL(value) {
    return /^https?:\/\//i.test(value) || looksLikeBareURL(value);
  }

  function looksLikeBareURL(value) {
    return /^(([a-z0-9-]+\.)+[a-z0-9-]+|localhost|127\.0\.0\.1|\[?::1\]?)(:\d+)?([/?#].*)?$/i.test(value);
  }

  function mapExampleCommand(args, generatedParams) {
    const op = args[0];
    if (op === "post-upload") {
      throw new Error("`restish example post-upload` needs multipart file handling. Run it in the real Restish CLI.");
    }
    const operation = exampleOperations.get(op);
    if (!operation) {
      throw new Error(`\`example ${op || ""}\` is not mapped in the browser preview yet.`);
    }
    const pathParams = operation.pathParams || [];
    const queryParams = (generatedParams || []).slice();
    let index = 1;
    let path = operation.path;
    for (const name of pathParams) {
      const fallback = operation.defaults && operation.defaults[name];
      const value = args[index] || fallback;
      if (!value) {
        throw new Error(`\`restish example ${op}\` needs a \`${name}\` value.`);
      }
      path = path.replace(`{${name}}`, encodeURIComponent(value));
      index += args[index] ? 1 : 0;
    }
    const url = new URL(path, "https://api.rest.sh");
    for (const name of operation.requiredQuery || []) {
      const value = args[index];
      if (!value) {
        throw new Error(`\`restish example ${op}\` needs a \`${toKebab(name)}\` value.`);
      }
      url.searchParams.append(name, value);
      index += 1;
    }
    for (const [key, value] of queryParams) {
      url.searchParams.append(key, value);
    }
    return {
      method: operation.method,
      url: url.toString(),
      bodyArgs: args.slice(index),
      headers: operation.headers || [],
      query: operation.query || []
    };
  }

  function toKebab(value) {
    return String(value).replace(/_/g, "-");
  }

  function buildRequest(plan) {
    const url = new URL(plan.url);
    for (const item of plan.flags.query) {
      const idx = item.indexOf("=");
      if (idx < 1) {
        throw new Error(`Invalid query parameter \`${item}\`; expected key=value.`);
      }
      url.searchParams.append(item.slice(0, idx), item.slice(idx + 1));
    }

    const headers = new Headers();
    headers.set("Accept", "application/json, application/yaml;q=0.5, text/*;q=0.2");
    if (plan.flags.noCache) {
      headers.set("Cache-Control", "no-cache");
    }
    for (const item of plan.flags.headers) {
      const idx = item.indexOf(":");
      if (idx < 1) {
        throw new Error(`Invalid header \`${item}\`; expected Name: Value.`);
      }
      headers.set(item.slice(0, idx).trim(), item.slice(idx + 1).trim());
    }
    if (url.hostname === "api.rest.sh" && url.pathname === "/flaky" && url.searchParams.get("key") === "tour") {
      url.searchParams.set("key", `tour-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`);
    }

    const init = { method: plan.method, mode: "cors", headers };
    if (plan.flags.timeout) {
      init.signal = timeoutSignal(plan.flags.timeout);
    }
    if (["POST", "PUT", "PATCH"].includes(plan.method) && plan.bodyArgs.length) {
      const contentType = plan.flags.contentType || "json";
      const body = parseBodyArgs(plan.bodyArgs);
      if (contentType === "json") {
        headers.set("Content-Type", "application/json");
        init.body = JSON.stringify(body);
      } else if (contentType === "form") {
        headers.set("Content-Type", "application/x-www-form-urlencoded");
        init.body = new URLSearchParams(flattenForm(body)).toString();
      } else {
        throw new Error(`Content type \`${contentType}\` requires the real Restish CLI in this preview.`);
      }
    } else if (plan.bodyArgs.length && plan.mode !== "edit") {
      throw new Error("Request bodies are only supported for POST, PUT, and PATCH in the browser preview.");
    }

    return { url, init };
  }

  function parseBodyArgs(parts) {
    if (parts.length === 1) {
      const trimmed = parts[0].trim();
      if (/^[\[{]/.test(trimmed)) {
        try {
          return JSON.parse(trimmed);
        } catch (error) {
          throw new Error(`Could not parse JSON body: ${error.message}`);
        }
      }
    }
    return parseShorthand(parts);
  }

  function parseShorthand(parts) {
    const out = {};
    for (const part of shorthandFields(parts)) {
      const idx = findFieldSeparator(part);
      if (idx < 1) {
        throw new Error(`Could not parse shorthand field \`${part.trim()}\`; expected key: value.`);
      }
      const key = part.slice(0, idx).trim();
      assignShorthandPath(out, keyPath(key), parseScalar(part.slice(idx + 1).trim()));
    }
    return out;
  }

  function shorthandFields(parts) {
    const fields = [];
    for (const part of Array.isArray(parts) ? parts : [parts]) {
      for (const field of splitCommas(part)) {
        if (field.trim()) {
          fields.push(field.trim());
        }
      }
    }
    return fields;
  }

  function findFieldSeparator(field) {
    let quote = "";
    for (let i = 0; i < field.length; i += 1) {
      const ch = field[i];
      if (quote) {
        if (ch === quote) {
          quote = "";
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (ch === ":") {
        return i;
      }
    }
    return -1;
  }

  function keyPath(key) {
    const path = [];
    for (const segment of key.split(".").filter(Boolean)) {
      const re = /([^\[\]]+)|\[(.*?)\]/g;
      let match;
      while ((match = re.exec(segment))) {
        if (match[1]) {
          path.push(match[1]);
        } else if (match[2] === "") {
          path.push("[]");
        } else if (/^\d+$/.test(match[2])) {
          path.push(Number(match[2]));
        } else {
          path.push(match[2]);
        }
      }
    }
    return path;
  }

  function assignShorthandPath(root, path, value) {
    if (!path.length) {
      return;
    }
    let current = root;
    for (let i = 0; i < path.length; i += 1) {
      const part = path[i];
      const last = i === path.length - 1;
      const next = path[i + 1];

      if (last) {
        if (part === "[]") {
          current.push(value);
        } else {
          current[part] = value;
        }
        return;
      }

      if (part === "[]") {
        let child = current[current.length - 1];
        if (!child || typeof child !== "object") {
          child = nextContainer(next);
          current.push(child);
        }
        current = child;
        continue;
      }

      if (!current[part] || typeof current[part] !== "object") {
        current[part] = nextContainer(next);
      }
      current = current[part];
    }
  }

  function nextContainer(next) {
    return next === "[]" || typeof next === "number" ? [] : {};
  }

  function flattenForm(value, prefix, out) {
    const fields = out || {};
    if (!value || typeof value !== "object" || Array.isArray(value)) {
      fields[prefix || "value"] = value == null ? "" : String(value);
      return fields;
    }
    for (const [key, child] of Object.entries(value)) {
      const name = prefix ? `${prefix}.${key}` : key;
      if (Array.isArray(child)) {
        fields[name] = child.map((item) => item == null ? "" : String(item)).join(",");
      } else if (child && typeof child === "object") {
        flattenForm(child, name, fields);
      } else {
        fields[name] = child == null ? "" : String(child);
      }
    }
    return fields;
  }

  function splitCommas(text) {
    const parts = [];
    let current = "";
    let quote = "";
    let depth = 0;
    for (const ch of text) {
      if (quote) {
        if (ch === quote) {
          quote = "";
        }
        current += ch;
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        current += ch;
        continue;
      }
      if (ch === "[" || ch === "{" || ch === "(") {
        depth += 1;
        current += ch;
        continue;
      }
      if ((ch === "]" || ch === "}" || ch === ")") && depth > 0) {
        depth -= 1;
        current += ch;
        continue;
      }
      if (ch === "," && depth === 0) {
        parts.push(current);
        current = "";
        continue;
      }
      current += ch;
    }
    if (current.trim()) {
      parts.push(current);
    }
    return parts;
  }

  function parseScalar(value) {
    if ((value.startsWith("\"") && value.endsWith("\"")) || (value.startsWith("'") && value.endsWith("'"))) {
      return value.slice(1, -1);
    }
    if (value.startsWith("[") && value.endsWith("]")) {
      const inner = value.slice(1, -1).trim();
      return inner ? splitCommas(inner).map((item) => parseScalar(item.trim())) : [];
    }
    if (value === "true") return true;
    if (value === "false") return false;
    if (value === "null") return null;
    if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
    return value;
  }

  async function run(command, options) {
    const callbacks = options || {};
    const plan = parseCommand(command);
    const request = buildRequest(plan);
    if (plan.mode === "edit") {
      return renderWithVerbose(editPreviewResponse(plan, request), plan, request);
    }
    const preview = docsFixtureResponse(plan, request);
    if (preview && preview.previewOnly) {
      return renderWithVerbose(preview, plan, request);
    }
    let response;
    let raw;
    try {
      response = await fetchWithRetries(request, plan);
    } catch (error) {
      if (isAbortError(error) || /timed out after/i.test(error.message || "")) {
        throw error;
      }
      const fixture = docsFixtureResponse(plan, request);
      if (fixture) {
        return renderWithVerbose(fixture, plan, request);
      }
      throw error;
    }
    const headers = responseHeadersObject(response.headers);

    if (isStreamingContentType(headers["Content-Type"] || "")) {
      return renderWithVerbose(await readStreamingResponse(response, headers, plan, request, callbacks.onStream), plan, request);
    }

    raw = await response.text();
    const body = decodeBody(raw, headers["Content-Type"] || "");
    let normalized = {
      proto: "HTTP/2.0",
      status: response.status,
      headers,
      links: parseLinkHeader(headers.Link || "", request.url),
      body,
      raw,
      sourceURL: request.url.toString()
    };
    normalized = await maybePaginate(normalized, plan, request);

    return renderWithVerbose(normalized, plan, request);
  }

  async function fetchWithRetries(request, plan) {
    const maxRetries = parseNonNegativeInteger(plan.flags.retry);
    const canRetry = plan.method === "GET" || plan.method === "HEAD" || plan.flags.retryUnsafe;
    let attempt = 0;
    let lastError;

    while (attempt <= maxRetries) {
      try {
        const response = await fetch(request.url, request.init);
        if (!canRetry || attempt >= maxRetries || !isRetryableStatus(response.status)) {
          return response;
        }
        await discardResponse(response);
        await sleep(retryDelay(response, attempt, plan));
      } catch (error) {
        if (isAbortError(error) && plan.flags.timeout) {
          throw new Error(`Request timed out after ${plan.flags.timeout}.`);
        }
        lastError = error;
        if (!canRetry || attempt >= maxRetries) {
          throw error;
        }
        await sleep(retryDelay(null, attempt, plan));
      }
      attempt += 1;
    }
    throw lastError || new Error("Request failed after retry attempts.");
  }

  function parseNonNegativeInteger(value) {
    if (value === "" || value === undefined || value === null) {
      return 0;
    }
    const parsed = Number(value);
    if (!Number.isFinite(parsed) || parsed < 0) {
      throw new Error("Retry count must be a non-negative number.");
    }
    return Math.floor(parsed);
  }

  function isRetryableStatus(status) {
    return status === 408 ||
      status === 425 ||
      status === 429 ||
      status === 500 ||
      status === 502 ||
      status === 503 ||
      status === 504;
  }

  function retryDelay(response, attempt, plan) {
    const headerDelay = response ? parseRetryHeader(response.headers.get("Retry-After") || response.headers.get("X-Retry-In")) : 0;
    const fallbackDelay = Math.min(1000, 100 * Math.pow(2, attempt));
    const maxWait = plan.flags.retryMaxWait ? parseDuration(plan.flags.retryMaxWait) : 0;
    const delay = headerDelay || fallbackDelay;
    return maxWait > 0 ? Math.min(delay, maxWait) : delay;
  }

  function parseRetryHeader(value) {
    if (!value) {
      return 0;
    }
    if (/^\d+(\.\d+)?$/.test(value)) {
      return Number(value) * 1000;
    }
    const duration = parseDuration(value);
    if (duration) {
      return duration;
    }
    const date = Date.parse(value);
    return Number.isFinite(date) ? Math.max(0, date - Date.now()) : 0;
  }

  async function discardResponse(response) {
    try {
      await response.text();
    } catch (_) {
      if (response.body && response.body.cancel) {
        try {
          await response.body.cancel();
        } catch (_) {
          // Ignore cleanup failures while preparing for a retry.
        }
      }
    }
  }

  function sleep(ms) {
    return new Promise(function (resolve) {
      window.setTimeout(resolve, Math.max(0, ms || 0));
    });
  }

  function isAbortError(error) {
    return error && (error.name === "AbortError" || /aborted/i.test(error.message || ""));
  }

  async function readStreamingResponse(response, headers, plan, request, onStream) {
    const contentType = headers["Content-Type"] || "";
    const maxItems = plan.flags.maxItems ? Number(plan.flags.maxItems) : 25;
    const records = [];
    let raw = "";
    const emit = function () {
      if (typeof onStream !== "function") {
        return;
      }
      onStream(renderWithVerbose({
        proto: "HTTP/2.0",
        status: response.status,
        headers,
        links: {},
        body: records.slice(0, maxItems),
        raw,
        sourceURL: request.url.toString()
      }, plan, request));
    };

    if (!response.body || !response.body.getReader) {
      raw = await response.text();
      records.push(...decodeStreamRecords(raw, contentType));
      emit();
    } else {
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      try {
        emit();
        while (records.length < maxItems) {
          const chunk = await reader.read();
          if (chunk.done) {
            break;
          }
          const previousCount = records.length;
          const text = decoder.decode(chunk.value, { stream: true });
          raw += text;
          buffer += text;
          buffer = drainStreamBuffer(buffer, contentType, records, maxItems);
          if (records.length > previousCount) {
            emit();
          }
        }
      } finally {
        if (records.length >= maxItems) {
          await reader.cancel();
        }
      }
      const tail = decoder.decode();
      if (tail) {
        raw += tail;
        buffer += tail;
      }
      if (buffer.trim() && records.length < maxItems) {
        const previousCount = records.length;
        records.push(...decodeStreamRecords(buffer, contentType).slice(0, maxItems - records.length));
        if (records.length > previousCount) {
          emit();
        }
      }
    }

    return {
      proto: "HTTP/2.0",
      status: response.status,
      headers,
      links: {},
      body: records.slice(0, maxItems),
      raw,
      sourceURL: request.url.toString()
    };
  }

  function isStreamingContentType(contentType) {
    const value = contentType.toLowerCase();
    return value.includes("text/event-stream") ||
      value.includes("application/x-ndjson") ||
      value.includes("application/ndjson") ||
      value.includes("application/jsonl") ||
      value.includes("application/json-seq");
  }

  function drainStreamBuffer(buffer, contentType, records, maxItems) {
    if (contentType.toLowerCase().includes("text/event-stream")) {
      let boundary = eventBoundary(buffer);
      while (boundary >= 0 && records.length < maxItems) {
        const block = buffer.slice(0, boundary);
        const event = parseSSEEvent(block);
        if (event) {
          records.push(event);
        }
        buffer = buffer.slice(buffer.charAt(boundary) === "\r" ? boundary + 4 : boundary + 2);
        boundary = eventBoundary(buffer);
      }
      return buffer;
    }

    const lines = buffer.split(/\r?\n/);
    const tail = lines.pop() || "";
    for (const line of lines) {
      if (records.length >= maxItems) {
        return tail;
      }
      const record = parseNDJSONLine(line);
      if (record !== undefined) {
        records.push(record);
      }
    }
    return tail;
  }

  function eventBoundary(buffer) {
    const lf = buffer.indexOf("\n\n");
    const crlf = buffer.indexOf("\r\n\r\n");
    if (lf < 0) return crlf;
    if (crlf < 0) return lf;
    return Math.min(lf, crlf);
  }

  function decodeStreamRecords(text, contentType) {
    if (contentType.toLowerCase().includes("text/event-stream")) {
      return text.split(/\r?\n\r?\n/).map(parseSSEEvent).filter(Boolean);
    }
    return text.split(/\r?\n/).map(parseNDJSONLine).filter((item) => item !== undefined);
  }

  function parseNDJSONLine(line) {
    const text = line.trim();
    if (!text) {
      return undefined;
    }
    try {
      return JSON.parse(text);
    } catch (_) {
      return text;
    }
  }

  function parseSSEEvent(block) {
    const lines = block.split(/\r?\n/);
    const event = {};
    const data = [];
    for (const line of lines) {
      if (!line || line.startsWith(":")) {
        continue;
      }
      const idx = line.indexOf(":");
      const field = idx >= 0 ? line.slice(0, idx) : line;
      let value = idx >= 0 ? line.slice(idx + 1) : "";
      if (value.startsWith(" ")) {
        value = value.slice(1);
      }
      if (field === "data") {
        data.push(value);
      } else if (field) {
        event[field] = value;
      }
    }
    if (!data.length && !Object.keys(event).length) {
      return null;
    }
    if (data.length) {
      const payload = data.join("\n");
      event.data = parseNDJSONLine(payload);
    }
    return event;
  }

  function editPreviewResponse(plan, request) {
    const current = fixtureBodyForPath(request.url.pathname) || {};
    const changes = plan.bodyArgs.length ? parseBodyArgs(plan.bodyArgs) : {};
    return jsonResponse(200, {
      resource: request.url.toString(),
      workflow: "GET, edit locally, then PUT the changed representation",
      current,
      changes,
      updated: mergeObjects(current, changes),
      note: "Browser preview only. Run `restish edit` locally for editor integration, diffs, confirmations, and real PUT requests."
    }, {}, { previewOnly: true });
  }

  function mergeObjects(base, patch) {
    if (!base || typeof base !== "object" || Array.isArray(base)) {
      return patch;
    }
    const out = { ...base };
    for (const [key, value] of Object.entries(patch || {})) {
      if (value && typeof value === "object" && !Array.isArray(value) && out[key] && typeof out[key] === "object" && !Array.isArray(out[key])) {
        out[key] = mergeObjects(out[key], value);
      } else {
        out[key] = value;
      }
    }
    return out;
  }

  async function maybePaginate(first, plan, request) {
    if (plan.mode !== "http" || plan.method !== "GET" || plan.flags.noPaginate || !Array.isArray(first.body)) {
      return first;
    }
    const maxPages = plan.flags.maxPages ? Number(plan.flags.maxPages) : 25;
    const maxItems = plan.flags.maxItems ? Number(plan.flags.maxItems) : 0;
    const collected = first.body.slice();
    let current = first;
    let pages = 1;

    while (current.links && current.links.next && (maxPages === 0 || pages < maxPages)) {
      if (maxItems > 0 && collected.length >= maxItems) {
        break;
      }
      const nextURL = new URL(current.links.next, request.url);
      const nextInit = {
        method: "GET",
        mode: "cors",
        headers: request.init.headers
      };
      const nextResponse = await fetch(nextURL, nextInit);
      const nextRaw = await nextResponse.text();
      const nextHeaders = responseHeadersObject(nextResponse.headers);
      const nextBody = decodeBody(nextRaw, nextHeaders["Content-Type"] || "");
      current = {
        proto: "HTTP/2.0",
        status: nextResponse.status,
        headers: nextHeaders,
        links: parseLinkHeader(nextHeaders.Link || "", nextURL),
        body: nextBody
      };
      if (!Array.isArray(nextBody)) {
        break;
      }
      collected.push(...nextBody);
      pages += 1;
    }

    if (maxItems > 0) {
      first.body = collected.slice(0, maxItems);
    } else {
      first.body = collected;
    }
    first.links = current.links || first.links;
    return first;
  }

  function docsFixtureResponse(plan, request) {
    if (request.url.hostname !== "api.rest.sh") {
      return null;
    }

    const path = request.url.pathname;
    const requestHeaders = requestHeadersObject(request.init.headers);
    const baseBody = {
      method: plan.method,
      headers: requestHeaders,
      host: request.url.host,
      url: request.url.toString(),
      path
    };
    if (request.url.search) {
      baseBody.query = Object.fromEntries(request.url.searchParams.entries());
    }

    if (path === "/" || path === "/get" || path === "/head" || path === "/options" || path === "/headers" || path.startsWith("/anything")) {
      return jsonResponse(200, baseBody);
    }

    if (path === "/types") {
      return jsonResponse(200, fixtureBodyForPath(path));
    }

    if (path === "/example") {
      return jsonResponse(200, fixtureBodyForPath(path));
    }

    if (["/post", "/put", "/patch", "/delete"].includes(path)) {
      return jsonResponse(200, {
        ...baseBody,
        body: decodeRequestBody(request.init.body, requestHeaders["Content-Type"] || "")
      });
    }

    if (path === "/login") {
      const body = decodeRequestBody(request.init.body, requestHeaders["Content-Type"] || "") || {};
      return jsonResponse(200, {
        token: `docs-token-${body.username || "demo"}`,
        token_type: "Bearer",
        user: body.username || "demo"
      });
    }

    if (path === "/auth/api-key-header") {
      return jsonResponse(200, {
        authenticated: requestHeaders["X-Api-Key"] === "docs-key",
        scheme: "api-key-header"
      });
    }

    if (path === "/auth/api-key-query") {
      return jsonResponse(200, {
        authenticated: ["docs-key", "docs-query-key"].includes(request.url.searchParams.get("api_key") || ""),
        scheme: "api-key-query"
      });
    }

    if (path === "/auth/bearer") {
      return jsonResponse(200, {
        authenticated: /^Bearer\s+\S+/.test(requestHeaders.Authorization || ""),
        scheme: "bearer",
        subject: "docs-token"
      });
    }

    if (path === "/auth/basic") {
      return jsonResponse(200, {
        authenticated: /^Basic\s+\S+/.test(requestHeaders.Authorization || ""),
        scheme: "http-basic",
        subject: "docs-user"
      });
    }

    if (path === "/problem") {
      return jsonResponse(400, {
        type: "https://api.rest.sh/problems/example",
        title: "Example problem",
        status: 400,
        detail: "Fixture problem response rendered by the docs browser preview."
      });
    }

    if (path === "/slow") {
      return jsonResponse(200, {
        delay: request.url.searchParams.get("delay") || "0s",
        ok: true
      });
    }

    if (path === "/flaky") {
      return jsonResponse(200, {
        attempt: 2,
        failures: Number(request.url.searchParams.get("failures") || "1"),
        key: request.url.searchParams.get("key") || "docs",
        ok: true
      });
    }

    if (path === "/cache") {
      return jsonResponse(200, {
        generated: "2026-04-27T00:00:00Z",
        until: "2026-04-27T00:05:00Z"
      }, {
        "Cache-Control": "public, max-age=300",
        ETag: "\"docs-cache\""
      });
    }

    const cachedMatch = path.match(/^\/cached\/(\d+)$/);
    if (cachedMatch) {
      return jsonResponse(200, {
        generated: "2026-04-27T00:00:00Z",
        until: `2026-04-27T00:00:${String(Math.min(Number(cachedMatch[1]), 59)).padStart(2, "0")}Z`
      }, {
        "Cache-Control": `${request.url.searchParams.get("private") === "true" ? "private" : "public"}, max-age=${cachedMatch[1]}`
      });
    }

    const etagMatch = path.match(/^\/etag\/([^/]+)$/);
    if (etagMatch) {
      return jsonResponse(200, {
        etag: decodeURIComponent(etagMatch[1]),
        ok: true
      }, {
        ETag: `"${decodeURIComponent(etagMatch[1])}"`
      });
    }

    if (path.startsWith("/formats/")) {
      return jsonResponse(200, {
        format: path.slice("/formats/".length),
        negotiated: requestHeaders.Accept || "application/json"
      });
    }

    if (path === "/images") {
      return jsonResponse(200, [
        { name: "Dragonfly JPEG", format: "jpeg", self: "/images/jpeg" },
        { name: "Dragonfly WebP", format: "webp", self: "/images/webp" },
        { name: "Dragonfly GIF", format: "gif", self: "/images/gif" },
        { name: "Dragonfly PNG", format: "png", self: "/images/png" },
        { name: "Dragonfly HEIC", format: "heic", self: "/images/heic" }
      ], {
        Link: "<https://api.rest.sh/images?page=2>; rel=\"next\""
      });
    }

    if (path === "/items") {
      if (plan.method === "POST") {
        return jsonResponse(200, decodeRequestBody(request.init.body, requestHeaders["Content-Type"] || "") || fixtureBodyForPath("/items/docs-demo"));
      }
      return jsonResponse(200, [
        fixtureBodyForPath("/items/docs-demo"),
        fixtureBodyForPath("/items/docs-second")
      ]);
    }

    const itemMatch = path.match(/^\/items\/([^/]+)$/);
    if (itemMatch) {
      const id = decodeURIComponent(itemMatch[1]);
      if (plan.method === "DELETE") {
        return jsonResponse(200, { deleted: id });
      }
      if (plan.method === "PATCH") {
        return jsonResponse(200, mergeObjects(fixtureBodyForPath(`/items/${id}`), decodeRequestBody(request.init.body, requestHeaders["Content-Type"] || "") || {}));
      }
      return jsonResponse(200, fixtureBodyForPath(`/items/${id}`));
    }

    if (path === "/books") {
      return jsonResponse(200, [
        { url: "/books/restish-tour", version: "1", modified: "2026-04-27T00:00:00Z" },
        { url: "/books/api-field-guide", version: "1", modified: "2026-04-27T00:00:00Z" }
      ]);
    }

    const bookMatch = path.match(/^\/books\/([^/]+)$/);
    if (bookMatch) {
      const id = decodeURIComponent(bookMatch[1]);
      const body = decodeRequestBody(request.init.body, requestHeaders["Content-Type"] || "") || {};
      if (plan.method === "DELETE") {
        return jsonResponse(200, { deleted: id });
      }
      if (["PUT", "PATCH"].includes(plan.method)) {
        return jsonResponse(200, mergeObjects(fixtureBodyForPath(`/books/${id}`), body));
      }
      return jsonResponse(200, fixtureBodyForPath(`/books/${id}`));
    }

    if (path === "/logs") {
      return jsonResponse(200, [
        { type: "info", message: "worker started", user: { id: "docs-1" } },
        { type: "info", message: "image indexed", user: { id: "docs-2" } },
        { type: "warn", message: "retry scheduled", user: { id: "docs-1" } }
      ], {
        "Content-Type": "application/x-ndjson"
      });
    }

    if (path === "/events" || path === "/sse/metrics") {
      return jsonResponse(200, [
        { event: "metric", data: { type: "counter", message: "requests total", user: { id: "docs-1" }, value: 42 } },
        { event: "metric", data: { type: "gauge", message: "queue depth", user: { id: "docs-2" }, value: 7 } },
        { event: "metric", data: { type: "counter", message: "cache hits", user: { id: "docs-1" }, value: 19 } }
      ], {
        "Content-Type": "text/event-stream"
      });
    }

    const statusMatch = path.match(/^\/status\/(\d{3})$/);
    if (statusMatch) {
      const status = Number(statusMatch[1]);
      return jsonResponse(status, status >= 400 ? {
        title: status >= 500 ? "Server Error" : "Client Error",
        status,
        detail: "Fixture response rendered by the docs browser preview."
      } : null);
    }

    if (path.startsWith("/images/")) {
      return {
        proto: "HTTP",
        status: 200,
        headers: {
          "Content-Type": `image/${path.slice("/images/".length) || "jpeg"}`,
          "Content-Length": "12345"
        },
        links: {},
        body: "(binary image data)",
        raw: "(binary image data)",
        sourceURL: request.url.toString(),
        previewOnly: true
      };
    }

    if (path.startsWith("/bytes/")) {
      throw new Error("Binary byte responses require the real Restish CLI in this preview.");
    }

    return null;
  }

  function fixtureBodyForPath(path) {
    if (path === "/types") {
      return {
        boolean: true,
        number: 123.45,
        string: "Hello from api.rest.sh",
        array: ["one", "two", "three"],
        object: {
          nested: "value",
          url: "https://rest.sh/"
        }
      };
    }
    if (path === "/example") {
      return {
        basics: {
          name: "Restish Docs",
          url: "https://rest.sh/",
          profiles: ["github", "docs", "api"]
        },
        volunteer: [
          {
            organization: "Open Source Collective",
            summary: "Maintains API tooling and documentation examples."
          },
          {
            organization: "Local Library",
            summary: "Builds small services for catalog and event data."
          }
        ],
        skills: [
          {
            name: "API Technologies",
            keywords: ["OpenAPI", "HTTP", "JSON", "SSE"]
          },
          {
            name: "Developer Tools",
            keywords: ["CLI", "Testing", "Documentation"]
          }
        ]
      };
    }
    if (path.startsWith("/items/")) {
      const id = decodeURIComponent(path.slice("/items/".length)) || "docs-demo";
      return {
        id,
        name: id === "docs-second" ? "Second docs item" : "Docs demo item",
        enabled: true,
        tags: ["docs", "tour"],
        updated: "2026-04-27T00:00:00Z"
      };
    }
    if (path.startsWith("/books/")) {
      const id = decodeURIComponent(path.slice("/books/".length)) || "restish-tour";
      return {
        title: id === "api-field-guide" ? "API Field Guide" : "Tour of Restish",
        author: "Restish Docs",
        published: "2026-04-27T00:00:00Z",
        rating_average: 4.8,
        ratings: 42,
        recent_ratings: [
          { date: "2026-04-27T00:00:00Z", rating: 5 },
          { date: "2026-04-26T00:00:00Z", rating: 4.5 }
        ]
      };
    }
    return null;
  }

  function jsonResponse(status, body, extraHeaders, options) {
    return {
      proto: "HTTP",
      status,
      headers: {
        "Content-Type": "application/json",
        ...(extraHeaders || {})
      },
      links: parseLinkHeader((extraHeaders && extraHeaders.Link) || "", new URL("https://api.rest.sh/")),
      body,
      raw: JSON.stringify(body),
      ...(options || {})
    };
  }

  function requestHeadersObject(headers) {
    const out = {};
    headers.forEach((value, key) => {
      out[key.replace(/(^|-)([a-z])/g, function (_, prefix, letter) {
        return prefix + letter.toUpperCase();
      })] = value;
    });
    return out;
  }

  function responseHeadersObject(headers) {
    const out = {};
    headers.forEach((value, key) => {
      out[key.replace(/(^|-)([a-z])/g, function (_, prefix, letter) {
        return prefix + letter.toUpperCase();
      })] = value;
    });
    return out;
  }

  function decodeRequestBody(body, contentType) {
    if (!body) {
      return null;
    }
    if (contentType.includes("application/json")) {
      try {
        return JSON.parse(body);
      } catch (_) {
        return body;
      }
    }
    if (contentType.includes("application/x-www-form-urlencoded")) {
      return Object.fromEntries(new URLSearchParams(body).entries());
    }
    return body;
  }

  function decodeBody(raw, contentType) {
    if (!raw) {
      return null;
    }
    if (contentType.includes("json") || /^[\[{]/.test(raw.trim())) {
      try {
        return JSON.parse(raw);
      } catch (_) {
        return raw;
      }
    }
    return raw;
  }

  function demoHeaders(headers) {
    const out = {};
    for (const key of Object.keys(headers || {})) {
      if (!isDemoHiddenHeader(key)) {
        out[key] = headers[key];
      }
    }
    return out;
  }

  function isDemoHiddenHeader(name) {
    const key = name.toLowerCase();
    return key.startsWith("access-control-") ||
      key.startsWith("cf-") ||
      key.startsWith("x-do-") ||
      key === "alt-svc" ||
      key === "content-encoding" ||
      key === "nel" ||
      key === "report-to" ||
      key === "server" ||
      key === "server-timing" ||
      key === "vary" ||
      key === "via" ||
      key === "x-request-id";
  }

  function renderWithVerbose(doc, plan, request) {
    const output = render(doc, plan, request);
    if (!plan.flags.verbose) {
      return output;
    }
    const trace = verboseTrace(doc, plan, request);
    if (output && typeof output === "object" && output.kind === "image") {
      return {
        ...output,
        text: trace + output.text
      };
    }
    return trace + output;
  }

  function verboseTrace(doc, plan, request) {
    const flags = plan.flags;
    const lines = [
      "verbose preview",
      "config: browser preview",
      "profile: default",
      `auth: ${requestHasAuth(request) ? "configured" : "none"}`,
      `input: ${request.init.body ? "body arguments" : "none"}`,
      `filter: ${flags.headersOnly ? "headers" : flags.filter || "none"}`,
      `output: ${verboseOutputFormat(doc, plan)}`,
      `> ${plan.method} ${request.url.toString()}`
    ];

    const requestHeaders = demoRequestHeaders(requestHeadersObject(request.init.headers));
    for (const key of Object.keys(requestHeaders).sort()) {
      lines.push(`> ${key}: ${requestHeaders[key]}`);
    }
    if (request.init.body) {
      const length = typeof request.init.body === "string" ? request.init.body.length : 0;
      lines.push(`> Content-Length: ${length}`);
    }

    lines.push(`< ${doc.proto || "HTTP/2.0"} ${doc.status} ${statusText(doc.status)}`);
    const responseHeaders = demoHeaders(doc.headers);
    for (const key of Object.keys(responseHeaders).sort()) {
      lines.push(`< ${key}: ${responseHeaders[key]}`);
    }
    lines.push("");
    return lines.join("\n") + "\n";
  }

  function requestHasAuth(request) {
    const headers = requestHeadersObject(request.init.headers);
    if (Object.keys(headers).some((name) => isSensitiveRequestHeader(name))) {
      return true;
    }
    return request.url.searchParams.has("api_key") || request.url.searchParams.has("access_token");
  }

  function demoRequestHeaders(headers) {
    const out = {};
    for (const [key, value] of Object.entries(headers || {})) {
      out[key] = isSensitiveRequestHeader(key) ? "<redacted>" : value;
    }
    return out;
  }

  function isSensitiveRequestHeader(name) {
    const key = name.toLowerCase();
    return key === "authorization" ||
      key === "cookie" ||
      key === "proxy-authorization" ||
      key === "x-api-key" ||
      key.includes("token") ||
      key.includes("secret");
  }

  function verboseOutputFormat(doc, plan) {
    const flags = plan.flags;
    if (flags.headersOnly) return "headers";
    if (flags.print && flags.print !== "auto") {
      const parts = printParts(flags.print);
      if ((parts.requestHeaders || parts.responseHeaders) && !parts.body) return "headers";
      if (parts.body && !parts.requestHeaders && !parts.requestBody && !parts.responseHeaders) return "body";
    }
    if (flags.outputFormat) return flags.outputFormat;
    if (!flags.filter && isImageResponse(doc)) return "image";
    if (flags.filter) return "filtered";
    return "auto";
  }

  function render(doc, plan, request) {
    const flags = plan.flags;
    if (plan.mode === "links") {
      return renderValue(selectLinks(doc.links || {}, plan.linkRels), flags);
    }

    // Parse --rsh-print spec. No flag or "auto" defaults to headers+body+pretty (playground default).
    const printStr = flags.print && flags.print !== "auto" ? flags.print : "";
    const parts = printParts(printStr || "hbp");

    let value = doc.body;
    // --rsh-headers or --rsh-print=h-without-b: filter to headers.
    let filter = flags.headersOnly || (printStr && parts.responseHeaders && !parts.body && !parts.requestHeaders && !parts.requestBody) ? "headers" : flags.filter;
    const explicitFilter = Boolean(filter || flags.headersOnly);

    if (filter === "headers") {
      value = demoHeaders(doc.headers);
    } else if (filter) {
      value = applyFilter(doc, filter);
    }

    if (printStr) {
      return renderPrintParts(doc, plan, request, value, explicitFilter, parts);
    }

    if (flags.outputFormat === "json") {
      return JSON.stringify(value, null, 2) + "\n";
    }
    if (flags.outputFormat === "yaml") {
      return yamlOutput(value);
    }
    if (flags.outputFormat === "ndjson") {
      return ndjsonOutput(value);
    }
    if (flags.outputFormat === "lines") {
      return linesOutput(value);
    }
    if (flags.outputFormat === "table") {
      return tableOutput(value, flags.columns, flags.sortBy);
    }
    if ((flags.outputFormat === "" || flags.outputFormat === "image") && !explicitFilter && isImageResponse(doc)) {
      return imageOutput(doc);
    }
    if (flags.outputFormat === "gron") {
      return gronOutput(value);
    }
    if (flags.outputFormat === "csv") {
      return csvOutput(value, flags.columns);
    }
    if (flags.outputFormat === "toon") {
      return toonOutput(value);
    }
    if (explicitFilter) {
      return renderValue(value, flags);
    }
    return readableResponse(doc);
  }

  function shouldHighlight(plan) {
    const printStr = plan.flags.print && plan.flags.print !== "auto" ? plan.flags.print : "";
    return !printStr || printStr.includes("c");
  }

  function highlightLanguage(plan) {
    const printStr = plan.flags.print && plan.flags.print !== "auto" ? plan.flags.print : "";
    const parts = printStr ? printParts(printStr) : null;
    if (parts && parts.color && !parts.pretty) {
      return "language-json";
    }
    return "language-readable";
  }

  function printParts(spec) {
    const parts = {
      requestHeaders: false,
      requestBody: false,
      responseHeaders: false,
      body: false,
      pretty: false,
      color: false
    };
    for (const ch of spec || "") {
      if (ch === "H") parts.requestHeaders = true;
      else if (ch === "B") parts.requestBody = true;
      else if (ch === "h") parts.responseHeaders = true;
      else if (ch === "b") parts.body = true;
      else if (ch === "p") parts.pretty = true;
      else if (ch === "c") parts.color = true;
      else throw new Error(`Invalid --rsh-print value \`${spec}\`: unknown part \`${ch}\`.`);
    }
    return parts;
  }

  function renderPrintParts(doc, plan, request, value, explicitFilter, parts) {
    const chunks = [];
    const spec = plan.flags.print || "";
    for (const ch of spec) {
      if (ch === "H") {
        chunks.push(requestPreamble(request, plan));
      } else if (ch === "B") {
        const body = requestBodyOutput(request, parts.pretty);
        if (body) chunks.push(body);
      } else if (ch === "h") {
        chunks.push(responsePreamble(doc));
      } else if (ch === "b") {
        const body = renderBodyPart(doc, value, plan.flags, explicitFilter, parts.pretty);
        if (typeof body === "object") {
          if (chunks.length === 0) return body;
          chunks.push(body.text || "");
        } else {
          chunks.push(body);
        }
      }
    }
    return chunks.join("").replace(/\n?$/, "\n");
  }

  function requestPreamble(request, plan) {
    if (!request) return "";
    const lines = [`${plan.method} ${request.url.pathname}${request.url.search || ""} HTTP/1.1`];
    lines.push(`Host: ${request.url.host}`);
    const headers = demoRequestHeaders(requestHeadersObject(request.init.headers));
    for (const key of Object.keys(headers).sort()) {
      lines.push(`${key}: ${headers[key]}`);
    }
    lines.push("");
    return lines.join("\n") + "\n";
  }

  function requestBodyOutput(request, pretty) {
    if (!request || !request.init.body) return "";
    const body = String(request.init.body);
    const contentType = request.init.headers.get("Content-Type") || "";
    if (pretty && contentType.includes("application/json")) {
      try {
        return JSON.stringify(JSON.parse(body), null, 2) + "\n";
      } catch (_) {
        return body.replace(/\n?$/, "\n");
      }
    }
    return body.replace(/\n?$/, "\n");
  }

  function responsePreamble(doc) {
    const lines = [`${doc.proto || "HTTP/2.0"} ${doc.status} ${statusText(doc.status)}`];
    const headers = demoHeaders(doc.headers);
    for (const key of Object.keys(headers).sort()) {
      lines.push(`${key}: ${headers[key]}`);
    }
    lines.push("");
    return lines.join("\n") + "\n";
  }

  function renderBodyPart(doc, value, flags, explicitFilter, pretty) {
    if (flags.outputFormat === "json") {
      return JSON.stringify(value, null, pretty ? 2 : 0) + "\n";
    }
    if (flags.outputFormat) {
      return renderValue(value, flags);
    }
    if (!explicitFilter && isImageResponse(doc)) {
      return imageOutput(doc);
    }
    if (!pretty) {
      if (value === null || value === undefined) return "null\n";
      if (typeof value !== "object") return String(value) + "\n";
      return JSON.stringify(value) + "\n";
    }
    return readableFilteredOutput(value);
  }

  function isImageResponse(doc) {
    return contentType(doc).startsWith("image/");
  }

  function contentType(doc) {
    const headers = doc.headers || {};
    const key = Object.keys(headers).find((name) => name.toLowerCase() === "content-type");
    return key ? String(headers[key]).toLowerCase() : "";
  }

  function imageOutput(doc) {
    const format = contentType(doc).split(";")[0] || "image/*";
    const lines = [`${doc.proto || "HTTP/2.0"} ${doc.status} ${statusText(doc.status)}`];
    const headers = demoHeaders(doc.headers);
    for (const key of Object.keys(headers).sort()) {
      lines.push(`${key}: ${headers[key]}`);
    }
    lines.push("");
    return {
      kind: "image",
      text: lines.join("\n").replace(/\n?$/, "\n"),
      url: doc.sourceURL || "",
      alt: `Response image (${format})`
    };
  }

  function applyFilter(doc, filter) {
    const expr = filter.trim().replace(/^\./, "");
    if (expr === "@") {
      return doc;
    }
    const firstItemMatch = expr.match(/^(.*?)\|\[0\]\.(.+)$/);
    if (firstItemMatch) {
      const selected = walk(doc, pathParts(firstItemMatch[1]));
      const first = Array.isArray(selected) ? selected[0] : undefined;
      return walk(first, pathParts(firstItemMatch[2]));
    }
    if (expr.startsWith("body |") || expr.startsWith("body[]") || expr.startsWith(".body")) {
      return applyJQishFilter(doc, expr);
    }
    return walk(doc, pathParts(expr));
  }

  function pathParts(expr) {
    const parts = [];
    for (const part of expr.split(".").filter(Boolean)) {
      const match = part.match(/^([^\[]+)(?:\[(.+)\])?$/);
      if (!match) {
        parts.push(part);
        continue;
      }
      parts.push(match[1]);
      if (match[2] !== undefined) {
        parts.push(`[${match[2]}]`);
      }
    }
    return parts;
  }

  function walk(value, parts) {
    if (!parts.length) {
      return value;
    }
    const [first, ...rest] = parts;
    if (first.startsWith("{") && first.endsWith("}")) {
      return projectValue(value, first.slice(1, -1), rest);
    }
    if (first.startsWith("[") && first.endsWith("]")) {
      return applyBracket(value, first.slice(1, -1), rest);
    }
    if (Array.isArray(value)) {
      return value.map((item) => walk(item, parts)).filter((item) => item !== undefined);
    }
    if (value && typeof value === "object") {
      const key = Object.keys(value).find((candidate) => candidate.toLowerCase() === first.toLowerCase());
      return key ? walk(value[key], rest) : undefined;
    }
    return undefined;
  }

  function projectValue(value, rawKeys, rest) {
    const keys = rawKeys.split(",").map((key) => key.trim()).filter(Boolean);
    if (Array.isArray(value)) {
      return value.map((item) => projectValue(item, rawKeys, rest));
    }
    if (!value || typeof value !== "object") {
      return undefined;
    }
    const out = {};
    for (const wanted of keys) {
      const key = Object.keys(value).find((candidate) => candidate.toLowerCase() === wanted.toLowerCase());
      if (key) {
        out[key] = rest.length ? walk(value[key], rest) : value[key];
      }
    }
    return out;
  }

  function applyBracket(value, expr, rest) {
    if (!Array.isArray(value)) {
      return undefined;
    }
    if (/^-?\d+$/.test(expr)) {
      let index = Number(expr);
      if (index < 0) {
        index = value.length + index;
      }
      return walk(value[index], rest);
    }
    const match = expr.match(/^([A-Za-z0-9_-]+)\s*={1,2}\s*['"]?([^'"]+)['"]?$/);
    if (match) {
      const selected = [];
      for (const item of value) {
        if (!item || item[match[1]] !== match[2]) {
          continue;
        }
        const result = walk(item, rest);
        if (Array.isArray(result)) {
          selected.push(...result);
        } else if (result !== undefined) {
          selected.push(result);
        }
      }
      return selected;
    }
    return undefined;
  }

  function applyJQishFilter(doc, expr) {
    const normalized = expr.replace(/^\./, "").trim();
    if (normalized === "body | length") {
      return Array.isArray(doc.body) || typeof doc.body === "string" ? doc.body.length : Object.keys(doc.body || {}).length;
    }
    const mapMatch = normalized.match(/^body \| map\(\.([A-Za-z0-9_-]+)\)(?: \| unique)?$/);
    if (mapMatch && Array.isArray(doc.body)) {
      const values = doc.body.map((item) => item && item[mapMatch[1]]).filter((item) => item !== undefined);
      return normalized.endsWith("| unique") ? Array.from(new Set(values)) : values;
    }
    const eachMatch = normalized.match(/^body\[\] \| \.([A-Za-z0-9_-]+)$/);
    if (eachMatch && Array.isArray(doc.body)) {
      return doc.body.map((item) => item && item[eachMatch[1]]).filter((item) => item !== undefined);
    }
    const selectMatch = normalized.match(/^body\[\] \| select\(\.([A-Za-z0-9_-]+) == "([^"]+)"\) \| \.([A-Za-z0-9_-]+)$/);
    if (selectMatch && Array.isArray(doc.body)) {
      return doc.body
        .filter((item) => item && item[selectMatch[1]] === selectMatch[2])
        .map((item) => item[selectMatch[3]])
        .filter((item) => item !== undefined);
    }
    return walk(doc, pathParts(normalized));
  }

  function linesOutput(value) {
    if (Array.isArray(value)) {
      return value.map(lineScalar).join("\n") + "\n";
    }
    return lineScalar(value) + "\n";
  }

  function lineScalar(value) {
    if (value === undefined || value === null) {
      return "null";
    }
    if (typeof value === "object") {
      throw new Error("lines: line output requires scalar values; use -o json for structured data.");
    }
    return String(value);
  }

  function renderValue(value, flags) {
    if (flags.outputFormat === "json") {
      return JSON.stringify(value, null, 2) + "\n";
    }
    if (flags.outputFormat === "yaml") {
      return yamlOutput(value);
    }
    if (flags.outputFormat === "gron") {
      return gronOutput(value);
    }
    if (flags.outputFormat === "csv") {
      return csvOutput(value, flags.columns);
    }
    if (flags.outputFormat === "toon") {
      return toonOutput(value);
    }
    return readableFilteredOutput(value);
  }

  function readableFilteredOutput(value) {
    if (value === undefined) {
      return "null\n";
    }
    if (value === null || typeof value !== "object") {
      return String(value) + "\n";
    }
    return JSON.stringify(value, null, 2) + "\n";
  }

  function readableResponse(doc) {
    const lines = [`${doc.proto || "HTTP/2.0"} ${doc.status} ${statusText(doc.status)}`];
    const headers = demoHeaders(doc.headers);
    for (const key of Object.keys(headers).sort()) {
      lines.push(`${key}: ${headers[key]}`);
    }
    lines.push("");
    if (doc.body !== null && doc.body !== undefined) {
      lines.push(JSON.stringify(doc.body, null, 2));
    }
    return lines.join("\n").replace(/\n?$/, "\n");
  }

  function readableOutput(value, indent) {
    const depth = indent || 0;
    const pad = "  ".repeat(depth);
    if (Array.isArray(value)) {
      return value.map((item) => `${pad}- ${readableInline(item, depth)}`).join("\n") + "\n";
    }
    if (value && typeof value === "object") {
      return Object.keys(value).sort().map((key) => {
        const child = value[key];
        if (child && typeof child === "object") {
          return `${pad}${key}:\n${readableOutput(child, depth + 1).replace(/\n$/, "")}`;
        }
        return `${pad}${key}: ${readableInline(child, depth)}`;
      }).join("\n") + "\n";
    }
    return linesOutput(value);
  }

  function readableInline(value) {
    if (typeof value === "string") {
      return JSON.stringify(value);
    }
    if (value === null || value === undefined) {
      return "null";
    }
    if (typeof value === "object") {
      return "\n" + readableOutput(value, 1).replace(/\n$/, "");
    }
    return String(value);
  }

  function tableOutput(value, columns, sortBy) {
    const rows = tableRows(value);
    if (!rows) {
      return readableOutput(value);
    }
    if (!rows.length) {
      return "(empty)\n";
    }
    if (sortBy) {
      rows.sort((a, b) => compareTableCells(a && a[sortBy], b && b[sortBy]));
    }
    const cols = columns ? columns.split(",").map((item) => item.trim()).filter(Boolean) : inferColumns(rows);
    if (!cols.length) {
      return "";
    }
    const body = rows.map((row) => cols.map((col) => truncateTableCell(tableScalar(row && row[col]), 40)));
    const widths = cols.map((col, index) => Math.max(
      displayWidth(col),
      ...body.map((row) => displayWidth(row[index]))
    ));

    const sep = (left, mid, right) => left + widths.map((width) => "─".repeat(width + 2)).join(mid) + right;
    const renderRow = (row) => "│" + row.map((cell, index) => ` ${padTableCell(cell, widths[index])} `).join("│") + "│";
    return [
      sep("┌", "┬", "┐"),
      renderRow(cols),
      sep("├", "┼", "┤"),
      ...body.map(renderRow),
      sep("└", "┴", "┘")
    ].join("\n") + "\n";
  }

  function ndjsonOutput(value) {
    const rows = Array.isArray(value) ? value : [value];
    return rows.map((row) => JSON.stringify(row)).join("\n") + "\n";
  }

  function csvOutput(value, columns) {
    const rows = Array.isArray(value) ? value : [value];
    const objects = rows.filter((row) => row && typeof row === "object" && !Array.isArray(row));
    if (!objects.length) {
      throw new Error("csv: CSV output requires object rows.");
    }
    const cols = columns ? columns.split(",").map((item) => item.trim()).filter(Boolean) : inferColumns(objects);
    const lines = [cols.map(csvCell).join(",")];
    for (const row of objects) {
      lines.push(cols.map((col) => csvCell(row[col])).join(","));
    }
    return lines.join("\n") + "\n";
  }

  function csvCell(value) {
    const text = value === undefined || value === null
      ? ""
      : typeof value === "object"
        ? JSON.stringify(value)
        : String(value);
    if (/[",\n\r]/.test(text)) {
      return `"${text.replace(/"/g, "\"\"")}"`;
    }
    return text;
  }

  function toonOutput(value) {
    return encodeTOONDocument(value) + "\n";
  }

  function encodeTOONDocument(value) {
    return encodeTOONRoot(value).replace(/\n+$/, "");
  }

  function encodeTOONRoot(value) {
    if (isTOONObject(value)) {
      return encodeTOONObject(0, value);
    }
    if (Array.isArray(value)) {
      if (!value.length) {
        return "[]\n";
      }
      return encodeTOONArray(0, "", value, { allowTabular: true });
    }
    return encodeTOONScalar(value) + "\n";
  }

  function encodeTOONObject(depth, obj) {
    return toonObjectFields(obj).map((field) => encodeTOONField(depth, field.key, field.value)).join("");
  }

  function encodeTOONField(depth, key, value) {
    const keyToken = encodeTOONKey(key);
    if (isTOONObject(value)) {
      return `${toonIndent(depth)}${keyToken}:\n${encodeTOONObject(depth + 1, value)}`;
    }
    if (Array.isArray(value)) {
      return encodeTOONArray(depth, keyToken, value, { allowTabular: true });
    }
    return `${toonIndent(depth)}${keyToken}: ${encodeTOONScalar(value)}\n`;
  }

  function encodeTOONArray(depth, keyToken, arr, context) {
    const indent = toonIndent(depth);
    if (!arr.length) {
      if (keyToken) {
        return `${indent}${keyToken}: []\n`;
      }
      return context.emptyArrayHeader ? `${indent}[0]:\n` : `${indent}[]\n`;
    }

    if (arr.every(isTOONPrimitive)) {
      return `${indent}${keyToken}[${arr.length}]: ${arr.map(encodeTOONScalar).join(",")}\n`;
    }

    if (context.allowTabular) {
      const fields = toonTabularFields(arr);
      if (fields) {
        let out = `${indent}${keyToken}[${arr.length}]{${fields.map(encodeTOONKey).join(",")}}:\n`;
        for (const item of arr) {
          out += `${toonIndent(depth + 1)}${fields.map((field) => encodeTOONScalar(item[field])).join(",")}\n`;
        }
        return out;
      }
    }

    let out = `${indent}${keyToken}[${arr.length}]:\n`;
    for (const item of arr) {
      out += encodeTOONListItem(depth + 1, item);
    }
    return out;
  }

  function encodeTOONListItem(depth, item) {
    if (isTOONObject(item)) {
      const fields = toonObjectFields(item);
      if (!fields.length) {
        return `${toonIndent(depth)}-\n`;
      }
      return toonDashItem(depth, encodeTOONObject(depth + 1, item));
    }
    if (Array.isArray(item)) {
      return toonDashArrayItem(depth, encodeTOONArray(depth + 1, "", item, {
        allowTabular: false,
        emptyArrayHeader: true
      }));
    }
    return `${toonIndent(depth)}- ${encodeTOONScalar(item)}\n`;
  }

  function toonTabularFields(arr) {
    let fields = null;
    for (const item of arr) {
      if (!isTOONObject(item)) {
        return null;
      }
      const itemFields = toonObjectFields(item);
      if (!itemFields.length || itemFields.some((field) => !isTOONPrimitive(field.value))) {
        return null;
      }
      if (!fields) {
        fields = itemFields.map((field) => field.key);
        continue;
      }
      if (itemFields.length !== fields.length || fields.some((field) => !Object.prototype.hasOwnProperty.call(item, field))) {
        return null;
      }
    }
    return fields;
  }

  function isTOONObject(value) {
    return value && typeof value === "object" && !Array.isArray(value);
  }

  function isTOONPrimitive(value) {
    return !isTOONObject(value) && !Array.isArray(value);
  }

  function toonObjectFields(obj) {
    return Object.keys(obj).map((key) => ({ key, value: obj[key] }));
  }

  function encodeTOONScalar(value) {
    if (value === null || value === undefined) {
      return "null";
    }
    if (typeof value === "boolean") {
      return value ? "true" : "false";
    }
    if (typeof value === "number") {
      return encodeTOONNumber(value);
    }
    return encodeTOONString(String(value));
  }

  function encodeTOONNumber(value) {
    if (!Number.isFinite(value)) {
      return "null";
    }
    if (Object.is(value, -0)) {
      return "0";
    }
    return String(value);
  }

  function encodeTOONString(value) {
    return toonStringNeedsQuote(value) ? quoteTOONString(value) : value;
  }

  function encodeTOONKey(key) {
    return /^[A-Za-z_][A-Za-z0-9_.]*$/.test(key) ? key : quoteTOONString(key);
  }

  function toonStringNeedsQuote(value) {
    if (value === "" || value === "true" || value === "false" || value === "null") {
      return true;
    }
    if (value[0] === "-") {
      return true;
    }
    if (value.trim() !== value) {
      return true;
    }
    if (/^-?\d+(\.\d+)?([eE][+-]?\d+)?$/.test(value)) {
      return true;
    }
    if (/[:"\\[\]{},]/.test(value)) {
      return true;
    }
    for (const ch of value) {
      if (ch.codePointAt(0) <= 0x1f) {
        return true;
      }
    }
    return false;
  }

  function quoteTOONString(value) {
    let out = "\"";
    for (const ch of value) {
      const code = ch.codePointAt(0);
      if (ch === "\\") {
        out += "\\\\";
      } else if (ch === "\"") {
        out += "\\\"";
      } else if (ch === "\n") {
        out += "\\n";
      } else if (ch === "\r") {
        out += "\\r";
      } else if (ch === "\t") {
        out += "\\t";
      } else if (code <= 0x1f) {
        out += `\\u${code.toString(16).padStart(4, "0")}`;
      } else {
        out += ch;
      }
    }
    return out + "\"";
  }

  function toonDashItem(depth, content) {
    const strip = toonIndentUnit.length * (depth + 1);
    return `${toonIndent(depth)}- ${content.slice(strip)}`;
  }

  function toonDashArrayItem(depth, content) {
    const firstLineStrip = toonIndentUnit.length * (depth + 1);
    const lines = toonLines(content);
    let out = `${toonIndent(depth)}- `;
    lines.forEach((line, index) => {
      if (index === 0) {
        out += line.slice(firstLineStrip);
      } else if (line.startsWith(toonIndentUnit)) {
        out += line.slice(toonIndentUnit.length);
      } else {
        out += line;
      }
    });
    return out;
  }

  function toonLines(content) {
    const lines = content.split("\n");
    const out = [];
    lines.forEach((line, index) => {
      if (index < lines.length - 1) {
        out.push(line + "\n");
      } else if (line) {
        out.push(line);
      }
    });
    return out;
  }

  function toonIndent(depth) {
    return toonIndentUnit.repeat(depth);
  }

  const toonIndentUnit = "  ";

  function gronOutput(value) {
    const lines = [];
    writeGron("json", value, lines);
    return lines.join("\n") + (lines.length ? "\n" : "");
  }

  function writeGron(path, value, lines) {
    if (Array.isArray(value)) {
      lines.push(`${path} = [];`);
      value.forEach((item, index) => writeGron(`${path}[${index}]`, item, lines));
      return;
    }
    if (value && typeof value === "object") {
      lines.push(`${path} = {};`);
      const keys = Object.keys(value);
      for (const key of keys) {
        writeGron(`${path}.${gronKey(key)}`, value[key], lines);
      }
      return;
    }
    lines.push(`${path} = ${JSON.stringify(value)};`);
  }

  function gronKey(key) {
    return /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(key) ? key : `[${JSON.stringify(key)}]`;
  }

  function tableScalar(value) {
    if (value === undefined || value === null) {
      return "";
    }
    if (typeof value === "object") {
      return JSON.stringify(value);
    }
    return String(value);
  }

  function tableRows(value) {
    if (value && typeof value === "object" && !Array.isArray(value)) {
      return [value];
    }
    if (!Array.isArray(value)) {
      return null;
    }
    const rows = [];
    for (const item of value) {
      if (!item || typeof item !== "object" || Array.isArray(item)) {
        return null;
      }
      rows.push(item);
    }
    return rows;
  }

  function compareTableCells(a, b) {
    const an = tableNumber(a);
    const bn = tableNumber(b);
    if (an !== null && bn !== null) {
      return an < bn ? -1 : an > bn ? 1 : 0;
    }
    const as = tableScalar(a);
    const bs = tableScalar(b);
    return as < bs ? -1 : as > bs ? 1 : 0;
  }

  function tableNumber(value) {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
    return null;
  }

  function truncateTableCell(value, maxChars) {
    const chars = Array.from(value);
    if (chars.length <= maxChars) {
      return value;
    }
    return chars.slice(0, Math.max(0, maxChars - 1)).join("") + "…";
  }

  function displayWidth(value) {
    return Array.from(value).length;
  }

  function padTableCell(value, width) {
    return value + " ".repeat(Math.max(0, width - displayWidth(value)));
  }

  function yamlOutput(value, depth) {
    const indent = "  ".repeat(depth || 0);
    if (Array.isArray(value)) {
      if (!value.length) {
        return "[]\n";
      }
      return value.map((item) => {
        if (item && typeof item === "object") {
          return `${indent}- ${yamlOutput(item, (depth || 0) + 1).trimStart()}`;
        }
        return `${indent}- ${yamlScalar(item)}\n`;
      }).join("");
    }
    if (value && typeof value === "object") {
      const keys = Object.keys(value).sort();
      if (!keys.length) {
        return "{}\n";
      }
      return keys.map((key) => {
        const child = value[key];
        if (child && typeof child === "object") {
          return `${indent}${key}:\n${yamlOutput(child, (depth || 0) + 1)}`;
        }
        return `${indent}${key}: ${yamlScalar(child)}\n`;
      }).join("");
    }
    return `${indent}${yamlScalar(value)}\n`;
  }

  function yamlScalar(value) {
    if (value === null || value === undefined) {
      return "null";
    }
    if (typeof value === "number" || typeof value === "boolean") {
      return String(value);
    }
    if (value === "") {
      return "\"\"";
    }
    return JSON.stringify(String(value));
  }

  function selectLinks(links, rels) {
    if (!rels.length) {
      return links;
    }
    const out = {};
    for (const rel of rels) {
      if (links[rel] !== undefined) {
        out[rel] = links[rel];
      }
    }
    return out;
  }

  function parseLinkHeader(header, baseURL) {
    const links = {};
    if (!header) {
      return links;
    }
    for (const part of splitLinkHeader(header)) {
      const match = part.match(/<([^>]+)>\s*;(.*)$/);
      if (!match) {
        continue;
      }
      const relMatch = match[2].match(/(?:^|;)\s*rel="?([^";]+)"?/);
      if (!relMatch) {
        continue;
      }
      try {
        links[relMatch[1]] = new URL(match[1], baseURL).toString();
      } catch (_) {
        links[relMatch[1]] = match[1];
      }
    }
    return links;
  }

  function splitLinkHeader(header) {
    const parts = [];
    let current = "";
    let inAngle = false;
    for (const ch of header) {
      if (ch === "<") inAngle = true;
      if (ch === ">") inAngle = false;
      if (ch === "," && !inAngle) {
        parts.push(current.trim());
        current = "";
        continue;
      }
      current += ch;
    }
    if (current.trim()) {
      parts.push(current.trim());
    }
    return parts;
  }

  function timeoutSignal(raw) {
    const ms = parseDuration(raw);
    if (!ms) {
      return undefined;
    }
    const controller = new AbortController();
    window.setTimeout(function () {
      controller.abort();
    }, ms);
    return controller.signal;
  }

  function parseDuration(raw) {
    const match = String(raw).match(/^(\d+(?:\.\d+)?)(ms|s)?$/);
    if (!match) {
      return 0;
    }
    const value = Number(match[1]);
    return match[2] === "s" ? value * 1000 : value;
  }

  function statusText(status) {
    return {
      200: "OK",
      201: "Created",
      202: "Accepted",
      204: "No Content",
      301: "Moved Permanently",
      302: "Found",
      304: "Not Modified",
      400: "Bad Request",
      401: "Unauthorized",
      403: "Forbidden",
      404: "Not Found",
      405: "Method Not Allowed",
      409: "Conflict",
      422: "Unprocessable Entity",
      429: "Too Many Requests",
      500: "Internal Server Error",
      502: "Bad Gateway",
      503: "Service Unavailable",
      504: "Gateway Timeout"
    }[status] || "";
  }

  function inferColumns(rows) {
    if (!rows.length) {
      return [];
    }
    const seen = new Set();
    const cols = Object.keys(rows[0]).sort();
    cols.forEach((col) => seen.add(col));
    const extra = [];
    rows.slice(1).forEach((row) => {
      Object.keys(row).forEach((col) => {
        if (!seen.has(col)) {
          seen.add(col);
          extra.push(col);
        }
      });
    });
    return cols.concat(extra.sort());
  }

  function setOutput(node, output, isError, highlight = true, language = "language-readable") {
    highlight = highlight && !isError;
    node.replaceChildren();
    if (output && typeof output === "object" && output.kind === "image") {
      node.textContent = output.text || "";
      if (highlight && window.Prism) {
        node.className = language;
        Prism.highlightElement(node);
      }
      if (output.url) {
        const image = document.createElement("img");
        image.className = "restish-playground__terminal-image";
        image.src = output.url;
        image.alt = output.alt || "Response image";
        image.loading = "lazy";
        node.appendChild(image);
      }
    } else {
      node.textContent = output;
    }
    const pane = node.parentElement;
    pane.hidden = false;
    pane.classList.toggle("restish-playground__output--error", Boolean(isError));
    pane.classList.toggle("restish-playground__output--table", typeof output === "string" && output.startsWith("┌"));
    pane.scrollTop = pane.scrollHeight;
    if (highlight && window.Prism && typeof output === "string") {
      node.className = language;
      Prism.highlightElement(node);
    }
  }

  function hideOutput(node) {
    node.replaceChildren();
    const pane = node.parentElement;
    pane.hidden = true;
    pane.classList.remove("restish-playground__output--error");
    pane.classList.remove("restish-playground__output--table");
  }

  function setState(node, text, mode) {
    if (!node) {
      return;
    }
    node.textContent = text;
    node.dataset.mode = mode || "";
  }

  function initPlayground(root) {
    if (root.dataset.restishReady === "true") {
      return;
    }
    root.dataset.restishReady = "true";
    const command = root.querySelector("[data-restish-command]");
    const commandHighlight = root.querySelector("[data-restish-command-highlight]");
    const completions = root.querySelector("[data-restish-completions]");
    const output = root.querySelector("[data-restish-output]");
    const runButton = root.querySelector("[data-restish-run]");
    const copyButton = root.querySelector("[data-restish-copy]");
    const resetButton = root.querySelector("[data-restish-reset]");
    const state = root.querySelector("[data-restish-state]");
    const initial = command.value;
    let commandHighlightReady = false;
    let copyFeedbackTimer = 0;

    function disableInjectedCopyButtons() {
      root.querySelectorAll("button").forEach(function (button) {
        if (button === runButton || button === copyButton || button === resetButton) {
          return;
        }
        const label = [
          button.textContent,
          button.getAttribute("aria-label"),
          button.getAttribute("title"),
          button.className
        ].join(" ").toLowerCase();
        if (!label.includes("copy")) {
          return;
        }
        button.hidden = true;
        button.tabIndex = -1;
        button.setAttribute("aria-hidden", "true");
      });
    }

    function fallbackCopy(text) {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.setAttribute("readonly", "");
      textarea.style.position = "fixed";
      textarea.style.top = "-9999px";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      try {
        if (!document.execCommand("copy")) {
          throw new Error("Copy command failed");
        }
      } finally {
        textarea.remove();
      }
    }

    async function copyText(text) {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return;
      }
      fallbackCopy(text);
    }

    function flashCopyState(label, mode) {
      if (!state) {
        return;
      }
      const previousText = state.textContent;
      const previousMode = state.dataset.mode || "";
      window.clearTimeout(copyFeedbackTimer);
      setState(state, label, mode);
      copyFeedbackTimer = window.setTimeout(function () {
        if (state.textContent === label) {
          setState(state, previousText, previousMode);
        }
      }, 1200);
    }

    function resetCommandScroll() {
      command.scrollLeft = 0;
      command.scrollTop = 0;
      if (commandHighlight) {
        const pane = commandHighlight.parentElement;
        if (pane) {
          pane.scrollLeft = 0;
          pane.scrollTop = 0;
        }
      }
    }

    function renderCommandHighlight() {
      if (!commandHighlight || !window.Prism) {
        return;
      }
      commandHighlight.textContent = command.value || " ";
      Prism.highlightElement(commandHighlight);
    }

    function syncCommandScroll() {
      if (!commandHighlight) {
        return;
      }
      const pane = commandHighlight.parentElement;
      if (pane) {
        pane.scrollLeft = command.scrollLeft;
        pane.scrollTop = command.scrollTop;
      }
    }

    function hideCompletions() {
      if (!completions) {
        return;
      }
      completions.hidden = true;
      completions.replaceChildren();
    }

    function showCompletions(matches) {
      if (!completions) {
        return;
      }
      completions.replaceChildren();
      if (!matches || !matches.length) {
        completions.hidden = true;
        return;
      }
      const prompt = document.createElement("span");
      prompt.className = "restish-playground__completion-prompt";
      prompt.textContent = ">";
      prompt.setAttribute("aria-hidden", "true");
      completions.appendChild(prompt);
      const list = document.createElement("span");
      list.className = "restish-playground__completion-list";
      for (const match of matches.slice(0, 18)) {
        const item = document.createElement("span");
        item.className = "restish-playground__completion-item";
        item.textContent = match;
        list.appendChild(item);
      }
      if (matches.length > 18) {
        const more = document.createElement("span");
        more.className = "restish-playground__completion-more";
        more.textContent = `+${matches.length - 18}`;
        list.appendChild(more);
      }
      completions.appendChild(list);
      completions.hidden = false;
    }

    function applyCompletion() {
      const result = completeCommand(command.value, command.selectionStart || 0);
      if (result.applied) {
        command.value = result.value;
        command.setSelectionRange(result.cursor, result.cursor);
        resetCommandScroll();
        renderCommandHighlight();
      }
      showCompletions(result.matches);
      setState(state, result.matches.length ? "Complete" : "No match", result.matches.length ? "ok" : "error");
    }

    function enableCommandHighlight() {
      if (commandHighlightReady || !commandHighlight || !window.Prism) {
        return;
      }
      commandHighlightReady = true;
      root.classList.add("restish-playground--command-highlighted");
      resetCommandScroll();
      renderCommandHighlight();
      command.addEventListener("input", renderCommandHighlight);
      command.addEventListener("scroll", syncCommandScroll);
      window.requestAnimationFrame(function () {
        resetCommandScroll();
        renderCommandHighlight();
      });
    }

    enableCommandHighlight();
    window.addEventListener("load", enableCommandHighlight, { once: true });
    disableInjectedCopyButtons();
    new MutationObserver(disableInjectedCopyButtons).observe(root, {
      childList: true,
      subtree: true
    });

    async function runCommand() {
      if (runButton.disabled) {
        return;
      }
      hideCompletions();
      let highlight = true;
      let language = "language-readable";
      try {
        const plan = parseCommand(command.value);
        highlight = shouldHighlight(plan);
        language = highlightLanguage(plan);
      } catch (_) {
        // parseCommand errors will surface again inside run()
      }
      setOutput(output, "Running...\n", false);
      setState(state, "Running", "running");
      runButton.disabled = true;
      try {
        let streamed = false;
        const text = await run(command.value, {
          onStream: function (text) {
            streamed = true;
            setOutput(output, text, false, highlight, language);
            setState(state, "Streaming", "running");
          }
        });
        if (!streamed) {
          setOutput(output, text, false, highlight, language);
        } else if (output.textContent !== text) {
          setOutput(output, text, false, highlight, language);
        }
        setState(state, "Live", "ok");
      } catch (error) {
        const hint = error && error.message && error.message.includes("Failed to fetch")
          ? "Network request failed. The API may not allow browser CORS requests from this page."
          : error.message;
        setOutput(output, `Error: ${hint}\n`, true);
        setState(state, "Error", "error");
      } finally {
        runButton.disabled = false;
      }
    }

    runButton.addEventListener("click", runCommand);

    if (copyButton) {
      copyButton.addEventListener("click", async function () {
        const icon = copyButton.querySelector("i");
        copyButton.disabled = true;
        try {
          await copyText(command.value);
          flashCopyState("Copied", "ok");
          if (icon) {
            icon.className = "fas fa-check";
          }
        } catch (error) {
          flashCopyState("Copy failed", "error");
        } finally {
          window.setTimeout(function () {
            if (icon) {
              icon.className = "fas fa-copy";
            }
            copyButton.disabled = false;
          }, 1200);
        }
      });
    }

    command.addEventListener("keydown", function (event) {
      if (event.key === "Tab") {
        event.preventDefault();
        applyCompletion();
        resetCommandScroll();
        return;
      }
      if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
        return;
      }
      event.preventDefault();
      runCommand();
    });

    command.addEventListener("input", function () {
      hideCompletions();
      if (state && state.dataset.mode !== "running") {
        setState(state, "Ready", "");
      }
    });

    command.addEventListener("blur", function () {
      resetCommandScroll();
      window.setTimeout(hideCompletions, 120);
    });

    resetButton.addEventListener("click", function () {
      command.value = initial;
      resetCommandScroll();
      renderCommandHighlight();
      hideCompletions();
      hideOutput(output);
      setState(state, "Ready", "");
      command.focus();
    });
  }

  function initAllPlaygrounds() {
    document.querySelectorAll("[data-restish-playground]").forEach(initPlayground);
  }

  if (window.__RESTISH_PLAYGROUND_TEST__) {
    window.__restishPlaygroundTest = {
      completeCommand,
      encodeTOONDocument,
      parseCommand,
      render,
      toonOutput
    };
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initAllPlaygrounds);
  } else {
    initAllPlaygrounds();
  }
})();
