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
    ["-f", "filter"],
    ["--rsh-filter", "filter"],
    ["--rsh-filter-lang", "filterLang"],
    ["--rsh-columns", "columns"],
    ["--rsh-sort-by", "sortBy"],
    ["--rsh-timeout", "timeout"],
    ["-t", "timeout"],
    ["--rsh-max-pages", "maxPages"],
    ["--rsh-max-items", "maxItems"],
    ["--rsh-max-events", "maxEvents"]
  ]);
  const boolFlags = new Map([
    ["-r", "raw"],
    ["--rsh-raw", "raw"],
    ["--rsh-headers", "headersOnly"],
    ["--rsh-no-paginate", "noPaginate"],
    ["--rsh-ignore-status-code", "ignoreStatus"],
    ["--rsh-collect", "collect"]
  ]);

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

  function parseCommand(command) {
    const cleanCommand = command.trim().replace(/^\$\s*/, "");
    if (/[|;&<>`$]/.test(cleanCommand)) {
      throw new Error("Shell pipes, redirects, substitutions, and environment expansion require the real Restish CLI.");
    }

    const tokens = shellWords(cleanCommand);
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
      raw: false,
      headersOnly: false
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
      if (token.startsWith("-")) {
        throw new Error(`${token} is not supported in the browser preview. Use the real Restish CLI for this feature.`);
      }
      args.push(token);
    }

    if (!args.length) {
      throw new Error("Add a URL or supported docs API command.");
    }
    if (flags.outputFormat === "raw") {
      throw new Error("Output format `raw` has been removed. Use `-r` or `--rsh-raw` for raw output.");
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
    } else if (looksLikeURL(args[0])) {
      url = args[0];
      bodyArgs = args.slice(1);
      if (bodyArgs.length) {
        method = "POST";
      }
    } else if (args[0] === "example") {
      const mapped = mapExampleCommand(args.slice(1));
      method = mapped.method;
      url = mapped.url;
      bodyArgs = mapped.bodyArgs;
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

  function mapExampleCommand(args) {
    const op = args[0];
    if (op === "list-images") {
      return { method: "GET", url: "https://api.rest.sh/images", bodyArgs: args.slice(1) };
    }
    if (op === "get-image") {
      const format = args[1] || "jpeg";
      return { method: "GET", url: `https://api.rest.sh/images/${encodeURIComponent(format)}`, bodyArgs: args.slice(2) };
    }
    if (op === "get-status") {
      const status = args[1] || "200";
      return { method: "GET", url: `https://api.rest.sh/status/${encodeURIComponent(status)}`, bodyArgs: args.slice(2) };
    }
    throw new Error(`\`example ${op || ""}\` is not mapped in the browser preview yet.`);
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
    for (const item of plan.flags.headers) {
      const idx = item.indexOf(":");
      if (idx < 1) {
        throw new Error(`Invalid header \`${item}\`; expected Name: Value.`);
      }
      headers.set(item.slice(0, idx).trim(), item.slice(idx + 1).trim());
    }

    const init = { method: plan.method, mode: "cors", headers };
    if (plan.flags.timeout) {
      init.signal = timeoutSignal(plan.flags.timeout);
    }
    if (["POST", "PUT", "PATCH"].includes(plan.method) && plan.bodyArgs.length) {
      const contentType = plan.flags.contentType || "json";
      if (contentType === "json") {
        headers.set("Content-Type", "application/json");
        init.body = JSON.stringify(parseShorthand(plan.bodyArgs));
      } else if (contentType === "form") {
        headers.set("Content-Type", "application/x-www-form-urlencoded");
        init.body = new URLSearchParams(flattenForm(parseShorthand(plan.bodyArgs))).toString();
      } else {
        throw new Error(`Content type \`${contentType}\` requires the real Restish CLI in this preview.`);
      }
    } else if (plan.bodyArgs.length) {
      throw new Error("Request bodies are only supported for POST, PUT, and PATCH in the browser preview.");
    }

    return { url, init };
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
      if (ch === ",") {
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
    if (value === "true") return true;
    if (value === "false") return false;
    if (value === "null") return null;
    if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
    return value;
  }

  async function run(command) {
    const plan = parseCommand(command);
    const request = buildRequest(plan);
    let response;
    let raw;
    try {
      response = await fetch(request.url, request.init);
      raw = await response.text();
    } catch (error) {
      const fixture = docsFixtureResponse(plan, request);
      if (fixture) {
        return render(fixture, plan);
      }
      throw error;
    }
    const headers = {};
    response.headers.forEach((value, key) => {
      headers[key.replace(/(^|-)([a-z])/g, function (_, prefix, letter) {
        return prefix + letter.toUpperCase();
      })] = value;
    });

    const body = decodeBody(raw, headers["Content-Type"] || "");
    let normalized = {
      proto: "HTTP/2.0",
      status: response.status,
      headers,
      links: parseLinkHeader(headers.Link || "", request.url),
      body,
      raw
    };
    normalized = await maybePaginate(normalized, plan, request);

    return render(normalized, plan);
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
      const nextHeaders = {};
      nextResponse.headers.forEach((value, key) => {
        nextHeaders[key.replace(/(^|-)([a-z])/g, function (_, prefix, letter) {
          return prefix + letter.toUpperCase();
        })] = value;
      });
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

    if (path === "/" || path === "/get" || path === "/headers" || path.startsWith("/anything")) {
      return jsonResponse(200, baseBody);
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
        authenticated: request.url.searchParams.get("api_key") === "docs-key",
        scheme: "api-key-query"
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

    const statusMatch = path.match(/^\/status\/(\d{3})$/);
    if (statusMatch) {
      const status = Number(statusMatch[1]);
      return jsonResponse(status, status >= 400 ? {
        title: status >= 500 ? "Server Error" : "Client Error",
        status,
        detail: "Fixture response rendered by the docs browser preview."
      } : null);
    }

    if (path.startsWith("/images/") || path.startsWith("/bytes/")) {
      throw new Error("Binary and image responses require the real Restish CLI in this preview.");
    }

    return null;
  }

  function jsonResponse(status, body, extraHeaders) {
    return {
      proto: "HTTP",
      status,
      headers: {
        "Content-Type": "application/json",
        ...(extraHeaders || {})
      },
      links: parseLinkHeader((extraHeaders && extraHeaders.Link) || "", new URL("https://api.rest.sh/")),
      body,
      raw: JSON.stringify(body)
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

  function render(doc, plan) {
    const flags = plan.flags;
    if (plan.mode === "links") {
      return renderValue(selectLinks(doc.links || {}, plan.linkRels), flags);
    }

    let value = doc.body;
    let filter = flags.headersOnly ? "headers" : flags.filter;
    const explicitFilter = Boolean(filter || flags.headersOnly);

    if (filter === "headers") {
      value = demoHeaders(doc.headers);
    } else if (filter) {
      value = applyFilter(doc, filter);
    }

    if (flags.raw) {
      if (explicitFilter) {
        throw new Error("--rsh-raw cannot be combined with --rsh-filter or --rsh-headers; use -o lines for shell-friendly filtered values.");
      }
      if (flags.outputFormat) {
        throw new Error("--rsh-raw cannot be combined with --rsh-output-format.");
      }
      return doc.raw !== undefined ? doc.raw : JSON.stringify(doc.body);
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
    if (explicitFilter) {
      return renderValue(value, flags);
    }
    return readableResponse(doc);
  }

  function applyFilter(doc, filter) {
    const expr = filter.trim().replace(/^\./, "");
    if (expr === "@") {
      return doc;
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
    const match = expr.match(/^([A-Za-z0-9_-]+)\s*=\s*['"]?([^'"]+)['"]?$/);
    if (match) {
      return value.map((item) => item && item[match[1]] === match[2] ? walk(item, rest) : undefined).filter((item) => item !== undefined);
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
    return readableOutput(value);
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
    if (!Array.isArray(value)) {
      return readableOutput(value);
    }
    const rows = value.slice();
    if (sortBy) {
      rows.sort((a, b) => String(a && a[sortBy] || "").localeCompare(String(b && b[sortBy] || "")));
    }
    const cols = columns ? columns.split(",").map((item) => item.trim()).filter(Boolean) : inferColumns(rows);
    if (!cols.length) {
      return "";
    }
    const tableRows = [cols].concat(rows.map((row) => cols.map((col) => tableScalar(row && row[col]))));
    const widths = cols.map((_, index) => Math.max(...tableRows.map((row) => row[index].length)));
    return tableRows.map((row, rowIndex) => {
      const line = row.map((cell, index) => cell.padEnd(widths[index])).join("  ").trimEnd();
      if (rowIndex === 0) {
        return line + "\n" + widths.map((width) => "-".repeat(width)).join("  ");
      }
      return line;
    }).join("\n") + "\n";
  }

  function ndjsonOutput(value) {
    const rows = Array.isArray(value) ? value : [value];
    return rows.map((row) => JSON.stringify(row)).join("\n") + "\n";
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
    const first = rows.find((row) => row && typeof row === "object" && !Array.isArray(row));
    return first ? Object.keys(first).slice(0, 4) : [];
  }

  function setOutput(node, text, isError) {
    node.textContent = text;
    const pane = node.parentElement;
    pane.hidden = false;
    pane.classList.toggle("restish-playground__output--error", Boolean(isError));
    if (window.Prism) {
      Prism.highlightElement(node);
    }
  }

  function hideOutput(node) {
    node.textContent = "";
    const pane = node.parentElement;
    pane.hidden = true;
    pane.classList.remove("restish-playground__output--error");
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
    const output = root.querySelector("[data-restish-output]");
    const runButton = root.querySelector("[data-restish-run]");
    const resetButton = root.querySelector("[data-restish-reset]");
    const state = root.querySelector("[data-restish-state]");
    const initial = command.value;

    async function runCommand() {
      if (runButton.disabled) {
        return;
      }
      setOutput(output, "Running...\n", false);
      setState(state, "Running", "running");
      runButton.disabled = true;
      try {
        const text = await run(command.value);
        setOutput(output, text, false);
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

    command.addEventListener("keydown", function (event) {
      if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
        return;
      }
      event.preventDefault();
      runCommand();
    });

    resetButton.addEventListener("click", function () {
      command.value = initial;
      hideOutput(output);
      setState(state, "Ready", "");
      command.focus();
    });
  }

  function initAllPlaygrounds() {
    document.querySelectorAll("[data-restish-playground]").forEach(initPlayground);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initAllPlaygrounds);
  } else {
    initAllPlaygrounds();
  }
})();
