(function () {
  "use strict";

  const outputActions = {
    auto: {
      command: "restish api.rest.sh/types",
      active: ["response", "decompress", "decode", "normalize", "format"],
      detail: "A normal terminal request receives bytes, decodes the body, normalizes the exchange, and chooses a human-friendly format. There is no paging or stream handling when the response is one document."
    },
    headers: {
      command: "restish api.rest.sh/ --rsh-headers",
      active: ["response", "normalize", "format"],
      detail: "Header output uses response metadata. Restish can skip body decompression and unmarshalling because the command only asks for normalized headers."
    },
    jsonfile: {
      command: "restish api.rest.sh/example -f body.skills > skills.json",
      active: ["response", "decompress", "decode", "normalize", "filter", "format"],
      detail: "For scripts, Restish can filter the normalized response and redirect the rendered JSON to a file. This path does not page or stream because the example endpoint returns one document."
    },
    pages: {
      command: "restish api.rest.sh/images",
      active: ["response", "decompress", "decode", "normalize", "paginate", "format"],
      detail: "Paginated collection endpoints add a page step after normalization. Restish follows pagination links and formats the resulting records for the terminal."
    },
    stream: {
      command: "restish api.rest.sh/events -o ndjson",
      active: ["response", "decompress", "decode", "normalize", "paginate", "format"],
      detail: "Streams reuse the same stage as pagination, but records arrive over time. NDJSON lets each event be decoded, normalized, and emitted without waiting for a final document."
    },
    redirect: {
      command: "restish api.rest.sh/images/jpeg > dragonfly.jpg",
      active: ["response", "decompress"],
      detail: "Unfiltered redirects preserve response body bytes after HTTP content-encoding decompression. Restish skips decoding, filtering, and formatting."
    }
  };

  const querySample = {
    proto: "HTTP/2.0",
    status: 200,
    headers: {
      "Content-Type": "application/json",
      "X-Request-ID": "req_7mQp42",
      "X-RateLimit-Remaining": "42"
    },
    headers_all: {
      "Content-Type": ["application/json"],
      "Set-Cookie": [
        "session=docs; Path=/; HttpOnly",
        "theme=dark; Path=/"
      ]
    },
    links: {
      self: "https://api.rest.sh/example",
      next: "https://api.rest.sh/example?page=2",
      help: "https://rest.sh/docs/guides/filtering/"
    },
    body: {
      account: {
        id: "acct_docs",
        name: "Restish Docs",
        active: true,
        created_at: "2026-06-08T09:30:00Z",
        tags: ["docs", "cli", "api"],
        plan: {
          name: "team",
          seats: 5,
          trial: false
        }
      },
      items: [
        {
          id: 101,
          name: "Dragonfly JPEG",
          format: "jpeg",
          size_bytes: 184523,
          public: true,
          created_at: "2026-06-01",
          labels: ["image", "demo", "public"],
          owner: {
            id: "usr_ada",
            name: "Ada"
          },
          links: {
            self: "https://api.rest.sh/images/jpeg",
            download: "https://api.rest.sh/images/jpeg?download=1"
          },
          metrics: {
            views: 1280,
            latency_ms: 24.6
          }
        },
        {
          id: 102,
          name: "OpenAPI Diagram",
          format: "svg",
          size_bytes: 9280,
          public: false,
          created_at: "2026-05-18",
          labels: ["diagram", "internal"],
          owner: {
            id: "usr_grace",
            name: "Grace"
          },
          links: {
            self: "https://api.rest.sh/images/svg",
            download: "https://api.rest.sh/images/svg?download=1"
          },
          metrics: {
            views: 312,
            latency_ms: 11.8
          }
        },
        {
          id: 103,
          name: "CLI Screenshot",
          format: "png",
          size_bytes: 64211,
          public: true,
          created_at: "2026-04-27",
          labels: ["screenshot", "docs"],
          owner: {
            id: "usr_ada",
            name: "Ada"
          },
          links: {
            self: "https://api.rest.sh/images/png",
            download: "https://api.rest.sh/images/png?download=1"
          },
          metrics: {
            views: 845,
            latency_ms: 18.2
          }
        }
      ],
      events: [
        {
          type: "deploy",
          at: "2026-06-08T16:45:00Z",
          actor: "daniel",
          success: true,
          changes: [
            { name: "output guide", kind: "docs" },
            { name: "TOON renderer", kind: "feature" }
          ]
        },
        {
          type: "audit",
          at: "2026-06-07T19:12:00Z",
          actor: "restish-bot",
          success: true,
          changes: [
            { name: "link check", kind: "qa" }
          ]
        }
      ],
      totals: {
        count: 3,
        public_count: 2,
        archived_count: 0,
        next_review_at: "2026-07-01"
      }
    }
  };

  function initOutputPipeline(root) {
    const command = root.querySelector("[data-output-command]");
    const detail = root.querySelector("[data-output-detail]");
    const actionButtons = Array.from(root.querySelectorAll("[data-output-action]"));
    const stepButtons = Array.from(root.querySelectorAll("[data-output-step]"));

    function activate(name) {
      const action = outputActions[name] || outputActions.auto;
      const active = new Set(action.active);
      if (command) {
        command.textContent = action.command;
      }
      if (detail) {
        detail.textContent = action.detail;
      }
      actionButtons.forEach(function (button) {
        button.toggleAttribute("data-active", button.dataset.outputAction === name);
      });
      stepButtons.forEach(function (button, index) {
        const isActive = active.has(button.dataset.outputStep);
        const next = stepButtons[index + 1];
        const hasActiveEdge = Boolean(isActive && next && active.has(next.dataset.outputStep));
        button.toggleAttribute("data-active", isActive);
        button.toggleAttribute("data-active-edge", hasActiveEdge);
        button.style.setProperty("--step-index", String(index));
      });
    }

    actionButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        activate(button.dataset.outputAction);
      });
    });
    activate("auto");
  }

  function initShorthandBody(root) {
    const input = root.querySelector("[data-shorthand-input]");
    const output = root.querySelector("[data-shorthand-output]");
    const status = root.querySelector("[data-shorthand-status]");
    const refreshHighlights = setupHighlightEditors(root);
    if (!input || !output) {
      return;
    }

    const shared = readSharedValue("shorthand-body");
    if (shared) {
      input.value = shared;
    }

    function render() {
      refreshHighlights();
      updateActiveSample(root, input.value);
      try {
        const value = parseShorthand(input.value);
        setHighlightedCode(output, JSON.stringify(value, null, 2), "json");
        setStatus(status, "Valid shorthand", "ok");
      } catch (error) {
        setHighlightedCode(output, error.message, "");
        setStatus(status, "Needs attention", "error");
      }
    }

    root.querySelectorAll("[data-sample]").forEach(function (button) {
      button.addEventListener("click", function () {
        input.value = button.dataset.sample || "";
        render();
        input.focus();
      });
    });
    const share = root.querySelector("[data-share-example]");
    if (share) {
      share.addEventListener("click", function () {
        copyShareLink("shorthand-body", input.value, share, status);
      });
    }
    input.addEventListener("input", render);
    render();
  }

  function initQueryRunner(root) {
    const input = root.querySelector("[data-query-input]");
    const source = root.querySelector("[data-query-source]");
    const output = root.querySelector("[data-query-output]");
    const status = root.querySelector("[data-query-status]");
    const traceRoot = root.querySelector("[data-filter-trace]");
    const traceToggle = root.querySelector("[data-toggle-trace]");
    const traceSummary = root.querySelector("[data-trace-summary]");
    const traceTokens = root.querySelector("[data-trace-tokens]");
    const traceBefore = root.querySelector("[data-trace-before]");
    const traceAfter = root.querySelector("[data-trace-after]");
    const traceDetail = root.querySelector("[data-trace-detail]");
    const tracePrev = root.querySelector("[data-trace-prev]");
    const traceNext = root.querySelector("[data-trace-next]");
    const tracePlay = root.querySelector("[data-trace-play]");
    const refreshHighlights = setupHighlightEditors(root);
    if (!input || !output) {
      return;
    }
    let traceSteps = [];
    let traceIndex = 0;
    let traceTimer = 0;

    const shared = readSharedValue("shorthand-filter");
    if (shared) {
      input.value = shared;
    }
    const sharedResponse = readSharedValue("response-shorthand");
    const sharedJSONResponse = readSharedValue("response-json");
    if (source) {
      source.value = sharedResponse
        ? JSON.stringify(parseShorthand(sharedResponse), null, 2)
        : (sharedJSONResponse || JSON.stringify(querySample, null, 2));
    }

    function render() {
      refreshHighlights();
      updateActiveSample(root, input.value);
      try {
        const sourceValue = parseQuerySource(source ? source.value : "");
        const value = applyShorthandFilter(sourceValue, input.value);
        setHighlightedCode(output, JSON.stringify(value, null, 2), "json");
        setStatus(status, "Filter matched", "ok");
        updateTrace(sourceValue);
      } catch (error) {
        setHighlightedCode(output, error.message, "");
        setStatus(status, "No match", "error");
        updateTrace(null, error);
      }
    }

    function updateTrace(sourceValue, error) {
      stopTracePlayback();
      if (!traceRoot || traceRoot.hidden) {
        return;
      }
      if (error || !sourceValue) {
        traceSteps = [];
        renderTraceError(error ? error.message : "Fix the response JSON before tracing.");
        return;
      }
      try {
        const trace = buildFilterTrace(sourceValue, input.value);
        traceSteps = trace.steps;
        traceIndex = Math.min(traceIndex, Math.max(0, traceSteps.length - 1));
        renderTraceStep();
      } catch (traceError) {
        traceSteps = [];
        renderTraceError(traceError.message);
      }
    }

    function renderTraceError(message) {
      if (traceSummary) {
        traceSummary.textContent = message;
      }
      if (traceTokens) {
        traceTokens.textContent = "";
      }
      setHighlightedCode(traceBefore, "", "");
      setHighlightedCode(traceAfter, "", "");
      if (traceDetail) {
        traceDetail.textContent = "";
      }
    }

    function renderTraceStep() {
      if (!traceSteps.length) {
        renderTraceError("This filter has no traceable steps yet.");
        return;
      }
      const step = traceSteps[traceIndex];
      if (traceSummary) {
        traceSummary.textContent = `Step ${traceIndex + 1} of ${traceSteps.length}: ${step.title}`;
      }
      if (traceTokens) {
        traceTokens.innerHTML = traceSteps.map(function (item, index) {
          const active = index === traceIndex ? " data-active" : "";
          return `<button type="button" data-trace-step="${index}"${active}>${escapeHTML(item.token)}</button>`;
        }).join("");
      }
      setHighlightedCode(traceBefore, JSON.stringify(step.before, null, 2), "json");
      setHighlightedCode(traceAfter, JSON.stringify(step.after, null, 2), "json");
      if (traceDetail) {
        traceDetail.textContent = step.detail;
      }
      if (tracePrev) {
        tracePrev.disabled = traceIndex === 0;
      }
      if (traceNext) {
        traceNext.disabled = traceIndex === traceSteps.length - 1;
      }
    }

    root.querySelectorAll("[data-sample]").forEach(function (button) {
      button.addEventListener("click", function () {
        input.value = button.dataset.sample || "";
        render();
        input.focus();
      });
    });
    const share = root.querySelector("[data-share-example]");
    if (share) {
      share.addEventListener("click", function () {
        try {
          copyShareLinkParams({
            "response-shorthand": source ? encodeShorthand(parseQuerySource(source.value)) : "",
            "shorthand-filter": input.value
          }, share, status);
        } catch (error) {
          setStatus(status, "Fix response JSON before sharing", "error");
        }
      });
    }
    if (traceToggle && traceRoot) {
      traceToggle.addEventListener("click", function () {
        traceRoot.hidden = !traceRoot.hidden;
        traceToggle.setAttribute("aria-expanded", String(!traceRoot.hidden));
        traceToggle.textContent = traceRoot.hidden ? "Trace filter" : "Hide trace";
        traceIndex = 0;
        try {
          updateTrace(parseQuerySource(source ? source.value : ""));
        } catch (error) {
          updateTrace(null, error);
        }
      });
    }
    if (traceTokens) {
      traceTokens.addEventListener("click", function (event) {
        const button = event.target.closest("[data-trace-step]");
        if (!button) {
          return;
        }
        stopTracePlayback();
        traceIndex = Number(button.dataset.traceStep || "0");
        renderTraceStep();
      });
    }
    if (tracePrev) {
      tracePrev.addEventListener("click", function () {
        stopTracePlayback();
        traceIndex = Math.max(0, traceIndex - 1);
        renderTraceStep();
      });
    }
    if (traceNext) {
      traceNext.addEventListener("click", function () {
        stopTracePlayback();
        traceIndex = Math.min(traceSteps.length - 1, traceIndex + 1);
        renderTraceStep();
      });
    }
    if (tracePlay) {
      tracePlay.addEventListener("click", function () {
        if (traceTimer) {
          stopTracePlayback();
          return;
        }
        if (!traceSteps.length) {
          return;
        }
        tracePlay.textContent = "Pause";
        traceTimer = window.setInterval(function () {
          if (traceIndex >= traceSteps.length - 1) {
            stopTracePlayback();
            return;
          }
          traceIndex += 1;
          renderTraceStep();
        }, 950);
      });
    }
    input.addEventListener("input", render);
    if (source) {
      source.addEventListener("input", render);
    }
    render();

    function stopTracePlayback() {
      if (traceTimer) {
        window.clearInterval(traceTimer);
        traceTimer = 0;
      }
      if (tracePlay) {
        tracePlay.textContent = "Play";
      }
    }
  }

  function setupHighlightEditors(root) {
    const editors = Array.from(root.querySelectorAll("[data-highlight-editor]"));
    const refreshers = editors.map(function (editor) {
      const field = editor.querySelector("textarea, input");
      const mirror = editor.querySelector("[data-highlight-mirror]");
      const mode = editor.dataset.highlightMode || "";
      if (!field || !mirror) {
        return function () {};
      }

      function refresh() {
        autoSizeField(field);
        mirror.innerHTML = highlightValue(field.value, mode);
        if (field.tagName === "TEXTAREA" && !field.value.endsWith("\n")) {
          mirror.insertAdjacentHTML("beforeend", "\n");
        }
      }

      field.addEventListener("input", refresh);
      field.addEventListener("scroll", function () {
        mirror.parentElement.scrollTop = field.scrollTop;
        mirror.parentElement.scrollLeft = field.scrollLeft;
      });
      refresh();
      return refresh;
    });
    return function () {
      refreshers.forEach(function (refresh) {
        refresh();
      });
    };
  }

  function autoSizeField(field) {
    if (field.tagName !== "TEXTAREA" || !field.hasAttribute("data-auto-size")) {
      return;
    }
    const min = Number(field.dataset.autoSizeMin || "2");
    const max = Number(field.dataset.autoSizeMax || "8");
    const lineHeight = parseFloat(window.getComputedStyle(field).lineHeight) || 20;
    const padding = 24;
    field.style.height = "auto";
    const maxHeight = (lineHeight * max) + padding;
    const minHeight = (lineHeight * min) + padding;
    const nextHeight = Math.max(minHeight, Math.min(field.scrollHeight, maxHeight));
    field.style.height = `${nextHeight}px`;
    field.style.overflowY = field.scrollHeight > maxHeight ? "auto" : "hidden";
  }

  function updateActiveSample(root, value) {
    const normalized = (value || "").trim();
    root.querySelectorAll("[data-sample]").forEach(function (button) {
      button.toggleAttribute("data-active", (button.dataset.sample || "").trim() === normalized);
    });
  }

  function setHighlightedCode(node, value, mode) {
    if (!node) {
      return;
    }
    if (mode) {
      node.innerHTML = highlightValue(value, mode);
    } else {
      node.textContent = value;
    }
  }

  function highlightValue(value, mode) {
    if (mode === "json") {
      return highlightJSON(value);
    }
    if (mode === "shorthand" || mode === "filter") {
      return highlightLooseSyntax(value, mode);
    }
    return escapeHTML(value);
  }

  function highlightJSON(value) {
    const input = String(value);
    const pattern = /("(?:\\.|[^"\\])*"(?=\s*:)|"(?:\\.|[^"\\])*"|true|false|null|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|[{}\[\],:])/g;
    let html = "";
    let cursor = 0;
    let match;
    while ((match = pattern.exec(input)) !== null) {
      html += escapeHTML(input.slice(cursor, match.index));
      const token = match[0];
      let cls = "restish-token-punctuation";
      if (token.startsWith("\"")) {
        cls = /^\s*:/.test(input.slice(pattern.lastIndex)) ? "restish-token-key" : jsonStringClass(token);
      } else if (token === "true" || token === "false") {
        cls = "restish-token-bool";
      } else if (token === "null") {
        cls = "restish-token-null";
      } else if (/^-?\d/.test(token)) {
        cls = "restish-token-number";
      }
      html += `<span class="${cls}">${escapeHTML(token)}</span>`;
      cursor = match.index + token.length;
    }
    html += escapeHTML(input.slice(cursor));
    return html;
  }

  function highlightLooseSyntax(value, mode) {
    let html = "";
    let index = 0;
    while (index < value.length) {
      const ch = value[index];
      if (/\s/.test(ch)) {
        html += escapeHTML(ch);
        index += 1;
        continue;
      }
      if (ch === "\"" || ch === "'") {
        const start = index;
        index += 1;
        while (index < value.length) {
          if (value[index] === ch && value[index - 1] !== "\\") {
            index += 1;
            break;
          }
          index += 1;
        }
        html += tokenSpan("string", value.slice(start, index));
        continue;
      }
      if (value.slice(index, index + 2) === "==") {
        html += tokenSpan("operator", "==");
        index += 2;
        continue;
      }
      if (mode === "filter" && value[index] === "@") {
        html += tokenSpan("operator", "@");
        index += 1;
        continue;
      }
      if (mode === "shorthand") {
        const keyPathEnd = shorthandKeyPathEnd(value, index);
        if (keyPathEnd > index) {
          html += highlightKeyPath(value.slice(index, keyPathEnd));
          index = keyPathEnd;
          continue;
        }
      }
      if ("{}[](),:|.".includes(ch)) {
        html += tokenSpan(ch === "|" ? "operator" : "punctuation", ch);
        index += 1;
        continue;
      }
      const start = index;
      while (index < value.length && !/[\s{}\[\](),:|.]/.test(value[index])) {
        index += 1;
      }
      const token = value.slice(start, index);
      const next = nextNonSpace(value, index);
      let cls = "string";
      if (next === ":" || (mode === "shorthand" && next === "{") || (mode === "filter" && /^[A-Za-z_][A-Za-z0-9_-]*$/.test(token))) {
        cls = "key";
      }
      if (token === "true" || token === "false") {
        cls = "bool";
      } else if (token === "null") {
        cls = "null";
      } else if (mode === "filter" && token === "contains") {
        cls = "operator";
      } else if (/^-?\d+(\.\d+)?$/.test(token)) {
        cls = "number";
      } else if (/^https?:\/\//.test(token)) {
        cls = "href";
      } else if (/^\d{4}-\d{2}-\d{2}(?:T[\d:.]+Z?)?$/.test(token)) {
        cls = "date";
      }
      html += tokenSpan(cls, token);
    }
    return html;
  }

  function shorthandKeyPathEnd(value, start) {
    let index = start;
    let quote = "";
    let bracketDepth = 0;
    while (index < value.length) {
      const ch = value[index];
      if (quote) {
        if (ch === quote && value[index - 1] !== "\\") {
          quote = "";
        }
        index += 1;
        continue;
      }
      if (ch === "\"" || ch === "'") {
        quote = ch;
        index += 1;
        continue;
      }
      if (ch === "[") {
        bracketDepth += 1;
      } else if (ch === "]") {
        bracketDepth = Math.max(0, bracketDepth - 1);
      }
      if (bracketDepth === 0 && (ch === ":" || ch === "{")) {
        return index;
      }
      if (bracketDepth === 0 && /[\s,}\])|]/.test(ch)) {
        return -1;
      }
      index += 1;
    }
    return -1;
  }

  function highlightKeyPath(path) {
    let html = "";
    let index = 0;
    while (index < path.length) {
      const ch = path[index];
      if (ch === "." || ch === "[" || ch === "]") {
        html += tokenSpan("punctuation", ch);
        index += 1;
        continue;
      }
      const start = index;
      while (index < path.length && path[index] !== "." && path[index] !== "[" && path[index] !== "]") {
        index += 1;
      }
      html += tokenSpan("key", path.slice(start, index));
    }
    return html;
  }

  function jsonStringClass(token) {
    try {
      const value = JSON.parse(token);
      if (/^https?:\/\//.test(value)) {
        return "restish-token-href";
      }
      if (/^\d{4}-\d{2}-\d{2}(?:T[\d:.]+Z?)?$/.test(value)) {
        return "restish-token-date";
      }
    } catch (error) {
      return "restish-token-string";
    }
    return "restish-token-string";
  }

  function nextNonSpace(value, index) {
    let cursor = index;
    while (/\s/.test(value[cursor] || "")) {
      cursor += 1;
    }
    return value[cursor] || "";
  }

  function tokenSpan(kind, value) {
    return `<span class="restish-token-${kind}">${escapeHTML(value)}</span>`;
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, function (ch) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;"
      }[ch];
    });
  }

  function parseQuerySource(input) {
    const parsed = JSON.parse(input || "null");
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed) && (
      Object.prototype.hasOwnProperty.call(parsed, "body") ||
      Object.prototype.hasOwnProperty.call(parsed, "headers") ||
      Object.prototype.hasOwnProperty.call(parsed, "headers_all") ||
      Object.prototype.hasOwnProperty.call(parsed, "links") ||
      Object.prototype.hasOwnProperty.call(parsed, "status") ||
      Object.prototype.hasOwnProperty.call(parsed, "proto")
    )) {
      return parsed;
    }
    return {
      body: parsed,
      headers: {},
      headers_all: {},
      links: {},
      proto: "",
      status: 0
    };
  }

  function parseShorthand(input) {
    const trimmed = input.trim();
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      return parseLiteral(trimmed);
    }
    const result = {};
    const fields = splitTopLevel(input, ",").map((item) => item.trim()).filter(Boolean);
    for (const field of fields) {
      const index = topLevelColonIndex(field);
      if (index <= 0) {
        const objectIndex = topLevelObjectFieldIndex(field);
        if (objectIndex > 0) {
          const path = field.slice(0, objectIndex).trim();
          setPath(result, path, parseLiteral(field.slice(objectIndex).trim()));
          continue;
        }
        throw new Error(`Could not parse \`${field}\`; expected key: value.`);
      }
      const path = field.slice(0, index).trim();
      const raw = field.slice(index + 1).trim();
      if (!path) {
        throw new Error("Shorthand fields need a key before `:`.");
      }
      setPath(result, path, parseScalar(raw));
    }
    return result;
  }

  function topLevelColonIndex(input) {
    let quote = "";
    let depth = 0;
    for (let index = 0; index < input.length; index += 1) {
      const ch = input[index];
      if (quote) {
        if (ch === quote && input[index - 1] !== "\\") {
          quote = "";
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (ch === "[" || ch === "{") {
        depth += 1;
      } else if (ch === "]" || ch === "}") {
        depth = Math.max(0, depth - 1);
      }
      if (ch === ":" && depth === 0) {
        return index;
      }
    }
    return -1;
  }

  function topLevelObjectFieldIndex(input) {
    let quote = "";
    let depth = 0;
    for (let index = 0; index < input.length; index += 1) {
      const ch = input[index];
      if (quote) {
        if (ch === quote && input[index - 1] !== "\\") {
          quote = "";
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        continue;
      }
      if (ch === "[" && depth === 0) {
        depth += 1;
        continue;
      }
      if (ch === "]" && depth > 0) {
        depth -= 1;
        continue;
      }
      if (ch === "{" && depth === 0) {
        const path = input.slice(0, index).trim();
        const literal = input.slice(index).trim();
        return path && literal.endsWith("}") ? index : -1;
      }
    }
    return -1;
  }

  function setPath(target, path, value) {
    const parts = assignmentPathSegments(path);
    let current = target;
    parts.forEach(function (part, index) {
      const isLast = index === parts.length - 1;
      const arrayMatch = part.match(/^(.+)\[(\d*)\]$/);
      const isArray = Boolean(arrayMatch);
      const key = arrayMatch ? arrayMatch[1] : part;
      const arrayIndex = arrayMatch && arrayMatch[2] !== "" ? Number(arrayMatch[2]) : -1;
      if (!key) {
        throw new Error(`Invalid path segment in \`${path}\`.`);
      }
      if (isLast) {
        if (isArray) {
          if (!Array.isArray(current[key])) {
            current[key] = [];
          }
          if (arrayIndex >= 0) {
            current[key][arrayIndex] = value;
          } else {
            current[key].push(value);
          }
        } else {
          current[key] = value;
        }
        return;
      }
      if (isArray) {
        if (!Array.isArray(current[key])) {
          current[key] = [];
        }
        const targetIndex = arrayIndex >= 0 ? arrayIndex : current[key].length - 1;
        if (arrayIndex >= 0 && !current[key][targetIndex]) {
          current[key][targetIndex] = {};
        } else if (arrayIndex < 0 && (!current[key].length || typeof current[key][current[key].length - 1] !== "object")) {
          current[key].push({});
        }
        current = current[key][arrayIndex >= 0 ? targetIndex : current[key].length - 1];
        return;
      }
      if (!current[key] || typeof current[key] !== "object" || Array.isArray(current[key])) {
        current[key] = {};
      }
      current = current[key];
    });
  }

  function assignmentPathSegments(path) {
    return splitPath(path).map(function (segment) {
      const match = segment.match(/^(.+)(\[(?:\d*)\])$/);
      if (!match) {
        return unquotePathSegment(segment);
      }
      return unquotePathSegment(match[1]) + match[2];
    }).filter(Boolean);
  }

  function parseScalar(raw) {
    if (raw === "") {
      return "";
    }
    if (raw.startsWith("[") || raw.startsWith("{")) {
      return parseLiteral(raw);
    }
    if (raw.startsWith("\"") && raw.endsWith("\"")) {
      return JSON.parse(raw);
    }
    if (raw.startsWith("'") && raw.endsWith("'")) {
      return raw.slice(1, -1);
    }
    if (raw === "true") {
      return true;
    }
    if (raw === "false") {
      return false;
    }
    if (raw === "null") {
      return null;
    }
    if (/^-?\d+(\.\d+)?$/.test(raw)) {
      return Number(raw);
    }
    return raw;
  }

  function encodeShorthand(value) {
    if (value && typeof value === "object") {
      return Object.keys(value).map(function (key) {
        return encodeField(encodePathSegment(key), value[key]);
      }).join(",");
    }
    return `body:${encodeLiteral(value)}`;
  }

  function encodeField(path, value) {
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const keys = Object.keys(value);
      if (keys.length === 1) {
        const key = keys[0];
        return encodeField(`${path}.${encodePathSegment(key)}`, value[key]);
      }
    }
    return `${path}:${encodeLiteral(value)}`;
  }

  function encodeLiteral(value) {
    if (Array.isArray(value)) {
      return `[${value.map(encodeLiteral).join(",")}]`;
    }
    if (value && typeof value === "object") {
      return `{${Object.keys(value).map(function (key) {
        return `${encodePathSegment(key)}:${encodeLiteral(value[key])}`;
      }).join(",")}}`;
    }
    return encodeScalar(value);
  }

  function encodeScalar(value) {
    if (value === null) {
      return "null";
    }
    if (typeof value === "boolean" || typeof value === "number") {
      return String(value);
    }
    const stringValue = String(value);
    if (canEncodeBareString(stringValue)) {
      return stringValue;
    }
    return JSON.stringify(stringValue);
  }

  function encodePathSegment(segment) {
    return /^[A-Za-z_][A-Za-z0-9_]*$/.test(segment) ? segment : JSON.stringify(segment);
  }

  function canEncodeBareString(value) {
    return value !== "" &&
      value === value.trim() &&
      value !== "true" &&
      value !== "false" &&
      value !== "null" &&
      !/^-?\d+(\.\d+)?$/.test(value) &&
      !/[,{}\[\]"']/.test(value);
  }

  function parseLiteral(input) {
    const parser = {
      index: 0,
      input
    };
    const value = parseLiteralValue(parser);
    skipLiteralSpace(parser);
    if (parser.index !== input.length) {
      throw new Error(`Unexpected shorthand literal near \`${input.slice(parser.index)}\`.`);
    }
    return value;
  }

  function parseLiteralValue(parser) {
    skipLiteralSpace(parser);
    const ch = parser.input[parser.index];
    if (ch === "{") {
      return parseLiteralObject(parser);
    }
    if (ch === "[") {
      return parseLiteralArray(parser);
    }
    if (ch === "\"" || ch === "'") {
      return parseLiteralQuoted(parser);
    }
    return parseLiteralBare(parser);
  }

  function parseLiteralObject(parser) {
    const result = {};
    parser.index += 1;
    skipLiteralSpace(parser);
    while (parser.index < parser.input.length && parser.input[parser.index] !== "}") {
      const key = parseLiteralKey(parser);
      skipLiteralSpace(parser);
      if (parser.input[parser.index] === "{") {
        result[key] = parseLiteralObject(parser);
        skipLiteralSpace(parser);
        if (parser.input[parser.index] === ",") {
          parser.index += 1;
          skipLiteralSpace(parser);
          if (parser.input[parser.index] === "}") {
            break;
          }
        } else if (parser.input[parser.index] !== "}") {
          throw new Error("Expected `,` or `}` in shorthand object literal.");
        }
        continue;
      }
      if (parser.input[parser.index] !== ":") {
        throw new Error("Expected `:` in shorthand object literal.");
      }
      parser.index += 1;
      result[key] = parseLiteralValue(parser);
      skipLiteralSpace(parser);
      if (parser.input[parser.index] === ",") {
        parser.index += 1;
        skipLiteralSpace(parser);
        if (parser.input[parser.index] === "}") {
          break;
        }
      } else if (parser.input[parser.index] !== "}") {
        throw new Error("Expected `,` or `}` in shorthand object literal.");
      }
    }
    if (parser.input[parser.index] !== "}") {
      throw new Error("Unclosed shorthand object literal.");
    }
    parser.index += 1;
    return result;
  }

  function parseLiteralArray(parser) {
    const result = [];
    parser.index += 1;
    skipLiteralSpace(parser);
    while (parser.index < parser.input.length && parser.input[parser.index] !== "]") {
      result.push(parseLiteralValue(parser));
      skipLiteralSpace(parser);
      if (parser.input[parser.index] === ",") {
        parser.index += 1;
        skipLiteralSpace(parser);
        if (parser.input[parser.index] === "]") {
          break;
        }
      } else if (parser.input[parser.index] !== "]") {
        throw new Error("Expected `,` or `]` in shorthand array literal.");
      }
    }
    if (parser.input[parser.index] !== "]") {
      throw new Error("Unclosed shorthand array literal.");
    }
    parser.index += 1;
    return result;
  }

  function parseLiteralKey(parser) {
    skipLiteralSpace(parser);
    const ch = parser.input[parser.index];
    if (ch === "\"" || ch === "'") {
      return parseLiteralQuoted(parser);
    }
    const start = parser.index;
    while (parser.index < parser.input.length && !/[\s:{}]/.test(parser.input[parser.index])) {
      parser.index += 1;
    }
    return parser.input.slice(start, parser.index);
  }

  function parseLiteralQuoted(parser) {
    const quote = parser.input[parser.index];
    if (quote === "\"") {
      let end = parser.index + 1;
      while (end < parser.input.length) {
        if (parser.input[end] === "\"" && parser.input[end - 1] !== "\\") {
          const raw = parser.input.slice(parser.index, end + 1);
          parser.index = end + 1;
          return JSON.parse(raw);
        }
        end += 1;
      }
      throw new Error("Unclosed quoted shorthand string.");
    }
    let value = "";
    parser.index += 1;
    while (parser.index < parser.input.length && parser.input[parser.index] !== "'") {
      value += parser.input[parser.index];
      parser.index += 1;
    }
    if (parser.input[parser.index] !== "'") {
      throw new Error("Unclosed quoted shorthand string.");
    }
    parser.index += 1;
    return value;
  }

  function parseLiteralBare(parser) {
    const start = parser.index;
    while (parser.index < parser.input.length && parser.input[parser.index] !== "," && parser.input[parser.index] !== "]" && parser.input[parser.index] !== "}") {
      parser.index += 1;
    }
    return parseScalar(parser.input.slice(start, parser.index).trim());
  }

  function skipLiteralSpace(parser) {
    while (/\s/.test(parser.input[parser.index] || "")) {
      parser.index += 1;
    }
  }

  function buildFilterTrace(doc, filter) {
    const parts = splitTopLevel(filter, "|").map((item) => item.trim()).filter(Boolean);
    const steps = [];
    let current = doc;
    if (!parts.length) {
      steps.push({
        token: "input",
        title: "Use the input value",
        detail: "An empty filter returns the normalized response unchanged.",
        before: doc,
        after: doc
      });
      return { steps, value: doc };
    }
    parts.forEach(function (part, index) {
      if (index > 0) {
        steps.push({
          token: "|",
          title: "Pipe the current result",
          detail: "A pipe stops path mapping, collects the current result, and runs the next expression against that value.",
          before: current,
          after: current
        });
      }
      const traced = traceExpression(current, part);
      steps.push(...traced.steps);
      current = traced.value;
    });
    return { steps, value: current };
  }

  function traceExpression(value, expr) {
    if (expr.startsWith("{") && expr.endsWith("}")) {
      const after = project(value, expr.slice(1, -1));
      return {
        value: after,
        steps: [traceStep(`{${expr.slice(1, -1)}}`, "Project selected fields", "Projection builds a new object from the current value.", value, after)]
      };
    }
    if (expr.startsWith("..")) {
      const field = expr.slice(2);
      const after = recursiveFind(value, field);
      return {
        value: after,
        steps: [traceStep(`..${field}`, `Find every ${field}`, "Recursive search walks below the current value and collects every matching field.", value, after)]
      };
    }

    const projection = expr.match(/^(.*)\.\{(.+)\}$/);
    if (projection) {
      const traced = tracePath(value, projection[1]);
      const before = traced.value;
      const after = project(before, projection[2]);
      traced.steps.push(traceStep(`{${projection[2]}}`, "Project selected fields", "Projection runs against the current value. If it is an array, each item gets the same projection.", before, after));
      traced.value = after;
      return traced;
    }

    const recursive = expr.match(/^(.*)\.\.([A-Za-z0-9_-]+)$/);
    if (recursive) {
      const traced = tracePath(value, recursive[1]);
      const before = traced.value;
      const after = recursiveFind(before, recursive[2]);
      traced.steps.push(traceStep(`..${recursive[2]}`, `Find every ${recursive[2]}`, "Recursive search walks below the current value and collects every matching field.", before, after));
      traced.value = after;
      return traced;
    }

    return tracePath(value, expr);
  }

  function tracePath(value, path) {
    if (!path) {
      return { steps: [], value };
    }
    const tokens = pathTokens(path);
    const steps = [];
    let current = value;
    tokens.forEach(function (token, index) {
      const before = current;
      const after = applyPathToken(before, token);
      steps.push(tracePathStep(token, index, before, after));
      current = after;
      if (current === undefined) {
        throw new Error(`No value matched \`${path}\`.`);
      }
    });
    return { steps, value: current };
  }

  function applyPathToken(value, token) {
    if (token.type === "field") {
      return readField(value, token.value);
    }
    if (token.type === "index") {
      return readIndex(value, token.value);
    }
    if (token.type === "slice") {
      return readSlice(value, token.start, token.end);
    }
    if (token.type === "select") {
      return selectItems(value, token.field, token.operator, token.value);
    }
    return value;
  }

  function tracePathStep(token, index, before, after) {
    if (token.type === "field") {
      const title = Array.isArray(before) ? `Map ${token.value} over each item` : `Select ${token.value}`;
      const detail = Array.isArray(before)
        ? `The current value is an array, so Restish applies .${token.value} to each item and keeps the matched values.`
        : `Restish reads the ${token.value} field from the current value.`;
      return traceStep(pathTokenLabel(token, index), title, detail, before, after);
    }
    if (token.type === "select") {
      const kept = Array.isArray(after) ? after.length : 0;
      const total = Array.isArray(before) ? before.length : 0;
      return traceStep(pathTokenLabel(token, index), `Filter ${total} items`, `Selection keeps ${kept} of ${total} items where ${token.field} ${selectionOperatorLabel(token.operator)} ${formatTraceScalar(token.value)}.`, before, after);
    }
    if (token.type === "index") {
      return traceStep(pathTokenLabel(token, index), `Pick index ${token.value}`, "Bracket indexes select one item from the current array.", before, after);
    }
    return traceStep(pathTokenLabel(token, index), "Take a slice", "Slice brackets select a range from the current array.", before, after);
  }

  function traceStep(token, title, detail, before, after) {
    return { token, title, detail, before, after };
  }

  function pathTokenLabel(token, index) {
    if (token.type === "field") {
      return `${index === 0 ? "" : "."}${formatPathField(token.value)}`;
    }
    if (token.type === "index") {
      return `[${token.value}]`;
    }
    if (token.type === "slice") {
      return `[${token.start}:${token.end === undefined ? "" : token.end}]`;
    }
    return `[${token.field} ${selectionOperatorLabel(token.operator)} ${formatTraceScalar(token.value)}]`;
  }

  function selectionOperatorLabel(operator) {
    return operator === "contains" ? "contains" : "==";
  }

  function formatPathField(field) {
    return /^[A-Za-z_][A-Za-z0-9_-]*$/.test(field) ? field : JSON.stringify(field);
  }

  function formatTraceScalar(value) {
    return typeof value === "string" && /\s/.test(value) ? JSON.stringify(value) : String(value);
  }

  function applyShorthandFilter(doc, filter) {
    const parts = splitTopLevel(filter, "|").map((item) => item.trim()).filter(Boolean);
    if (!parts.length) {
      return doc;
    }
    return parts.reduce(function (value, part) {
      return evaluateExpression(value, part);
    }, doc);
  }

  function evaluateExpression(value, expr) {
    if (expr.startsWith("{") && expr.endsWith("}")) {
      return project(value, expr.slice(1, -1));
    }
    if (expr.startsWith("..")) {
      return recursiveFind(value, expr.slice(2));
    }

    const projection = expr.match(/^(.*)\.\{(.+)\}$/);
    if (projection) {
      return project(evaluatePath(value, projection[1]), projection[2]);
    }

    const recursive = expr.match(/^(.*)\.\.([A-Za-z0-9_-]+)$/);
    if (recursive) {
      return recursiveFind(evaluatePath(value, recursive[1]), recursive[2]);
    }

    return evaluatePath(value, expr);
  }

  function evaluatePath(value, path) {
    if (!path) {
      return value;
    }
    let current = value;
    const tokens = pathTokens(path);
    for (const token of tokens) {
      if (token.type === "field") {
        current = readField(current, token.value);
      } else if (token.type === "index") {
        current = readIndex(current, token.value);
      } else if (token.type === "slice") {
        current = readSlice(current, token.start, token.end);
      } else if (token.type === "select") {
        current = selectItems(current, token.field, token.operator, token.value);
      }
      if (current === undefined) {
        throw new Error(`No value matched \`${path}\`.`);
      }
    }
    return current;
  }

  function pathTokens(path) {
    const tokens = [];
    const parts = splitPath(path);
    for (const part of parts) {
      let index = 0;
      while (index < part.length) {
        if (part[index] === "[") {
          const end = part.indexOf("]", index);
          if (end === -1) {
            throw new Error(`Unclosed bracket in \`${path}\`.`);
          }
          tokens.push(bracketToken(part.slice(index + 1, end).trim()));
          index = end + 1;
          continue;
        }
        let end = index;
        while (end < part.length && part[end] !== "[") {
          end += 1;
        }
        tokens.push({ type: "field", value: unquotePathSegment(part.slice(index, end)) });
        index = end;
      }
    }
    return tokens;
  }

  function splitPath(path) {
    const parts = [];
    let current = "";
    let quote = "";
    let depth = 0;
    for (let index = 0; index < path.length; index += 1) {
      const ch = path[index];
      if (quote) {
        current += ch;
        if (ch === quote && path[index - 1] !== "\\") {
          quote = "";
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        current += ch;
        continue;
      }
      if (ch === "[") {
        depth += 1;
      } else if (ch === "]") {
        depth = Math.max(0, depth - 1);
      }
      if (ch === "." && depth === 0) {
        if (current) {
          parts.push(current);
          current = "";
        }
        continue;
      }
      current += ch;
    }
    if (quote) {
      throw new Error(`Unclosed quote in \`${path}\`.`);
    }
    if (depth !== 0) {
      throw new Error(`Unclosed bracket in \`${path}\`.`);
    }
    if (current) {
      parts.push(current);
    }
    return parts;
  }

  function unquotePathSegment(segment) {
    if (segment.startsWith("\"") && segment.endsWith("\"")) {
      return JSON.parse(segment);
    }
    if (segment.startsWith("'") && segment.endsWith("'")) {
      return segment.slice(1, -1);
    }
    return segment;
  }

  function bracketToken(content) {
    const slice = content.match(/^(-?\d*)\s*:\s*(-?\d*)$/);
    if (slice) {
      return {
        type: "slice",
        start: slice[1] === "" ? 0 : Number(slice[1]),
        end: slice[2] === "" ? undefined : Number(slice[2])
      };
    }
    if (/^-?\d+$/.test(content)) {
      return { type: "index", value: Number(content) };
    }
    const contains = content.match(/^(@|[A-Za-z0-9_.-]+)\s+contains\s+(.+)$/);
    if (contains) {
      return { type: "select", field: contains[1], operator: "contains", value: parseScalar(contains[2].trim()) };
    }
    const equals = content.match(/^(@|[A-Za-z0-9_.-]+)\s*==\s*(.+)$/);
    if (equals) {
      return { type: "select", field: equals[1], operator: "equals", value: parseScalar(equals[2].trim()) };
    }
    throw new Error(`Unsupported selector \`[${content}]\` in this sample runner.`);
  }

  function readField(value, field) {
    if (Array.isArray(value)) {
      return value.map((item) => readField(item, field)).filter((item) => item !== undefined);
    }
    return value && value[field];
  }

  function readIndex(value, index) {
    if (!Array.isArray(value)) {
      throw new Error("Index selectors need an array.");
    }
    const resolved = index < 0 ? value.length + index : index;
    return value[resolved];
  }

  function readSlice(value, start, end) {
    if (!Array.isArray(value)) {
      throw new Error("Slice selectors need an array.");
    }
    const resolvedStart = start < 0 ? value.length + start : start;
    const resolvedEnd = end === undefined ? value.length : (end < 0 ? value.length + end + 1 : end + 1);
    return value.slice(resolvedStart, resolvedEnd);
  }

  function selectItems(value, field, operator, expected) {
    if (!Array.isArray(value)) {
      throw new Error("Selection filters need an array.");
    }
    return value.filter(function (item) {
      const selected = readSelectionField(item, field);
      return selected.matched && selectionMatches(selected.value, selected.expected(expected), operator);
    });
  }

  function readSelectionField(item, field) {
    const modifier = ".lower";
    const lower = field.endsWith(modifier);
    const path = lower ? field.slice(0, -modifier.length) : field;
    let value;
    try {
      value = path === "@" ? item : evaluatePath(item, path);
    } catch (error) {
      return {
        matched: false,
        value: undefined,
        expected: function (expected) {
          return expected;
        }
      };
    }
    return {
      matched: value !== undefined,
      value: lower ? lowerValue(value) : value,
      expected: lower ? lowerValue : function (expected) {
        return expected;
      }
    };
  }

  function selectionMatches(actual, expected, operator) {
    if (Array.isArray(actual)) {
      return actual.some(function (item) {
        return selectionMatches(item, expected, operator);
      });
    }
    if (operator === "contains") {
      if (actual === null || actual === undefined) {
        return false;
      }
      return String(actual).includes(String(expected));
    }
    return actual === expected;
  }

  function lowerValue(value) {
    if (Array.isArray(value)) {
      return value.map(lowerValue);
    }
    return typeof value === "string" ? value.toLowerCase() : value;
  }

  function project(value, spec) {
    if (Array.isArray(value)) {
      return value.map(function (item) {
        return project(item, spec);
      });
    }
    const fields = splitTopLevel(spec, ",").map((item) => item.trim()).filter(Boolean);
    const target = {};
    for (const field of fields) {
      const alias = field.match(/^([A-Za-z0-9_-]+)\s*:\s*(.+)$/);
      if (alias) {
        target[alias[1]] = evaluateExpression(value, alias[2].trim());
      } else {
        target[field] = evaluatePath(value, field);
      }
    }
    return target;
  }

  function recursiveFind(value, field) {
    const found = [];
    walk(value, function (item, key) {
      if (key === field) {
        found.push(item);
      }
    });
    return found;
  }

  function walk(value, visit, key) {
    if (value && typeof value === "object") {
      if (key !== undefined) {
        visit(value, key);
      }
      if (Array.isArray(value)) {
        value.forEach(function (item) {
          walk(item, visit);
        });
      } else {
        Object.keys(value).forEach(function (childKey) {
          walk(value[childKey], visit, childKey);
        });
      }
      return;
    }
    if (key !== undefined) {
      visit(value, key);
    }
  }

  function splitTopLevel(input, separator) {
    const parts = [];
    let current = "";
    let quote = "";
    let depth = 0;
    for (let index = 0; index < input.length; index += 1) {
      const ch = input[index];
      if (quote) {
        current += ch;
        if (ch === quote && input[index - 1] !== "\\") {
          quote = "";
        }
        continue;
      }
      if (ch === "'" || ch === "\"") {
        quote = ch;
        current += ch;
        continue;
      }
      if (ch === "{" || ch === "[") {
        depth += 1;
      } else if (ch === "}" || ch === "]") {
        depth = Math.max(0, depth - 1);
      }
      if (ch === separator && depth === 0) {
        parts.push(current);
        current = "";
      } else {
        current += ch;
      }
    }
    parts.push(current);
    return parts;
  }

  function readSharedValue(key) {
    const params = new URLSearchParams((window.location.hash || "").replace(/^#/, ""));
    return params.get(key);
  }

  function copyShareLink(key, value, button, status) {
    copyShareLinkParams({ [key]: value }, button, status);
  }

  function copyShareLinkParams(values, button, status) {
    const url = new URL(window.location.href);
    const params = new URLSearchParams((url.hash || "").replace(/^#/, ""));
    Object.keys(values).forEach(function (key) {
      const value = values[key];
      if (value) {
        params.set(key, value);
      } else {
        params.delete(key);
      }
    });
    url.hash = params.toString();
    copyText(url.toString()).then(function () {
      button.textContent = "Copied";
      setStatus(status, "Share link copied", "ok");
      window.setTimeout(function () {
        button.textContent = "Copy share link";
      }, 1600);
    }).catch(function () {
      setStatus(status, "Copy failed", "error");
    });
  }

  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(text);
    }
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.left = "-1000px";
    document.body.appendChild(textarea);
    textarea.select();
    const copied = document.execCommand("copy");
    document.body.removeChild(textarea);
    return copied ? Promise.resolve() : Promise.reject(new Error("copy failed"));
  }

  function setStatus(node, text, mode) {
    if (!node) {
      return;
    }
    node.textContent = text;
    node.dataset.mode = mode || "";
  }

  function init() {
    document.querySelectorAll("[data-restish-output-pipeline]").forEach(initOutputPipeline);
    document.querySelectorAll("[data-restish-shorthand-body]").forEach(initShorthandBody);
    document.querySelectorAll("[data-restish-query-runner]").forEach(initQueryRunner);
  }

  if (window.__RESTISH_DOCS_INTERACTIONS_TEST__) {
    window.__restishDocsInteractionsTest = {
      applyShorthandFilter,
      buildFilterTrace,
      outputActions,
      encodeShorthand,
      highlightValue,
      parseShorthand,
      parseQuerySource,
      querySample
    };
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
