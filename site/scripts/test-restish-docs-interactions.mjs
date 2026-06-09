import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const siteDir = path.resolve(__dirname, "..");

const source = await readFile(path.join(siteDir, "static/js/restish-docs-interactions.js"), "utf8");
const sandbox = {
  URL,
  URLSearchParams,
  console,
  document: {
    readyState: "loading",
    addEventListener() {},
    createElement() {
      return {
        setAttribute() {},
        select() {},
        style: {}
      };
    },
    querySelectorAll() {
      return [];
    },
    body: {
      appendChild() {},
      removeChild() {}
    },
    execCommand() {
      return true;
    }
  },
  navigator: {},
  window: {
    __RESTISH_DOCS_INTERACTIONS_TEST__: true,
    addEventListener() {},
    isSecureContext: false,
    location: {
      href: "https://rest.sh/docs/guides/input/",
      hash: ""
    },
    setTimeout
  }
};

vm.createContext(sandbox);
vm.runInContext(source, sandbox, { filename: "restish-docs-interactions.js" });

const api = sandbox.window.__restishDocsInteractionsTest;
assert.ok(api, "docs interactions test API should be exposed");

function normalize(value) {
  return JSON.parse(JSON.stringify(value));
}

assert.deepEqual(
  normalize(api.parseShorthand("user.name: Ada, tags[]: cli, tags[]: docs, enabled: true, count: 3")),
  {
    user: { name: "Ada" },
    tags: ["cli", "docs"],
    enabled: true,
    count: 3
  },
  "shorthand body parser handles nested fields, arrays, booleans, and numbers"
);

assert.deepEqual(
  normalize(api.parseShorthand("user{name: Ada Lovelace, email: ada@example.com}, active: true, joined_at: 2026-06-08")),
  {
    user: {
      name: "Ada Lovelace",
      email: "ada@example.com"
    },
    active: true,
    joined_at: "2026-06-08"
  },
  "shorthand body parser supports compact object assignment"
);

assert.deepEqual(
  normalize(api.parseShorthand(`metadata{
  tags: [docs, cli, beta],
  flags{dry_run: true, notify: false}
},
score: 98.6`)),
  {
    metadata: {
      tags: ["docs", "cli", "beta"],
      flags: {
        dry_run: true,
        notify: false
      }
    },
    score: 98.6
  },
  "shorthand body parser supports nested compact object assignment"
);

assert.deepEqual(
  normalize(api.parseShorthand(`order.id: ord-123,
items: [
  {sku: mug-01, qty: 2, price: 14.5},
  {sku: tee-02, qty: 1, price: 28},
],
rush: false`)),
  {
    order: {
      id: "ord-123"
    },
    items: [
      { sku: "mug-01", qty: 2, price: 14.5 },
      { sku: "tee-02", qty: 1, price: 28 }
    ],
    rush: false
  },
  "shorthand body parser supports formatted arrays with a trailing comma"
);

assert.deepEqual(
  normalize(api.parseShorthand(`{
  "name": "Ada Lovelace",
  "tags": ["docs", "json"],
  "active": true
}`)),
  {
    name: "Ada Lovelace",
    tags: ["docs", "json"],
    active: true
  },
  "shorthand body parser accepts plain JSON"
);

assert.ok(
  api.highlightValue("order.id: ord-123", "shorthand").includes('<span class="restish-token-key">order</span><span class="restish-token-punctuation">.</span><span class="restish-token-key">id</span>'),
  "shorthand highlighter treats dotted paths as one key"
);

assert.deepEqual(
  normalize(api.parseShorthand('enabled: "true", missing: "null", blank: ""')),
  {
    enabled: "true",
    missing: "null",
    blank: ""
  },
  "quoted shorthand scalars remain strings"
);

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body.account.{name, active, created_at, tags}")),
  {
    name: "Restish Docs",
    active: true,
    created_at: "2026-06-08T09:30:00Z",
    tags: ["docs", "cli", "api"]
  },
  "query projection"
);

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, 'body.items[format == "jpeg"].{name, owner: owner.name, size_bytes}')),
  [
    {
      name: "Dragonfly JPEG",
      owner: "Ada",
      size_bytes: 184523
    }
  ],
  "query selection maps projection across array items"
);

{
  const trace = api.buildFilterTrace(api.querySample, 'body.items[format == "jpeg"].{name, owner: owner.name, size_bytes}');
  assert.deepEqual(
    normalize(trace.steps.map((step) => step.token)),
    ["body", ".items", "[format == jpeg]", "{name, owner: owner.name, size_bytes}"],
    "filter trace exposes path, selection, and projection tokens"
  );
  assert.equal(trace.steps[1].title, "Select items", "filter trace names ordinary field selection");
  assert.equal(trace.steps[2].title, "Filter 3 items", "filter trace counts selected array items");
  assert.deepEqual(
    normalize(trace.value),
    [
      {
        name: "Dragonfly JPEG",
        owner: "Ada",
        size_bytes: 184523
      }
    ],
    "filter trace final value matches filter result"
  );
}

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body.items[public == true].name")),
  ["Dragonfly JPEG", "CLI Screenshot"],
  "query boolean selection across array fields"
);

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body.items[name.lower contains dragonfly].links.self")),
  ["https://api.rest.sh/images/jpeg"],
  "query selection supports contains with lower modifier"
);

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body.events[type == deploy].changes.name")),
  [
    ["output guide", "TOON renderer"]
  ],
  "query nested arrays after selection"
);

{
  const trace = api.buildFilterTrace(api.querySample, "body.events[type == deploy].changes.name");
  assert.equal(trace.steps.at(-1).title, "Map name over each item", "filter trace explains array mapping");
  assert.deepEqual(
    normalize(trace.value),
    [
      ["output guide", "TOON renderer"]
    ],
    "filter trace handles nested arrays after selection"
  );
}

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body..download")),
  [
    "https://api.rest.sh/images/jpeg?download=1",
    "https://api.rest.sh/images/svg?download=1",
    "https://api.rest.sh/images/png?download=1"
  ],
  "recursive shorthand query"
);

assert.deepEqual(
  normalize(api.applyShorthandFilter(api.querySample, "body..download|[@ contains jpeg]")),
  ["https://api.rest.sh/images/jpeg?download=1"],
  "recursive shorthand query can filter current values"
);

{
  const trace = api.buildFilterTrace(api.querySample, "body..download|[@ contains jpeg]");
  assert.deepEqual(
    normalize(trace.steps.map((step) => step.token)),
    ["body", "..download", "|", "[@ contains jpeg]"],
    "filter trace shows pipe as a collection boundary"
  );
  assert.equal(trace.steps[2].title, "Pipe the current result", "filter trace explains pipe boundary");
  assert.deepEqual(
    normalize(trace.value),
    ["https://api.rest.sh/images/jpeg?download=1"],
    "filter trace final value matches piped filter result"
  );
}

assert.equal(
  api.applyShorthandFilter(api.querySample, 'headers."Content-Type"'),
  "application/json",
  "quoted field segment query"
);

assert.equal(
  api.applyShorthandFilter(api.querySample, 'headers_all."Set-Cookie"[0]'),
  "session=docs; Path=/; HttpOnly",
  "headers_all root supports repeated headers"
);

assert.ok(
  api.highlightValue("body..download|[@ contains jpeg]", "filter").includes('<span class="restish-token-operator">contains</span>'),
  "filter highlighter marks contains as query syntax"
);

assert.deepEqual(
  normalize(api.parseQuerySource('{"items":[{"id":1,"name":"Ada"}]}')),
  {
    body: {
      items: [{ id: 1, name: "Ada" }]
    },
    headers: {},
    headers_all: {},
    links: {},
    proto: "",
    status: 0
  },
  "plain JSON body gets wrapped as a normalized body"
);

assert.equal(
  api.applyShorthandFilter(api.parseQuerySource('{"items":[{"id":1,"name":"Ada"}]}'), "body.items[0].name"),
  "Ada",
  "filters run against pasted plain JSON bodies"
);

const encodedSample = api.encodeShorthand(api.querySample);
assert.ok(encodedSample.includes('headers:{"Content-Type":application/json'), "share encoding groups multi-key header objects");
assert.ok(encodedSample.includes("links:{self:https://api.rest.sh/example,next:https://api.rest.sh/example?page=2"), "share encoding groups multi-key link objects");
assert.ok(encodedSample.includes("account:{id:acct_docs,name:Restish Docs"), "share encoding preserves nested account objects");
assert.ok(encodedSample.includes("tags:[docs,cli,api]"), "share encoding groups arrays without repeated paths");
assert.ok(encodedSample.includes("items:[{id:101,name:Dragonfly JPEG"), "share encoding preserves arrays of objects");
assert.equal(encodedSample.includes("tags[]"), false, "share encoding does not repeat array paths");

assert.deepEqual(
  normalize(api.parseShorthand(encodedSample)),
  normalize(api.querySample),
  "share shorthand round trips the normalized sample response"
);

assert.equal(
  api.encodeShorthand({ body: { items: [{ id: 1, name: "Ada" }] } }),
  "body.items:[{id:1,name:Ada}]",
  "share encoding shortens single-key object chains"
);

assert.deepEqual(
  normalize(api.parseShorthand('body:{basics:{profiles:[github,docs]}}')),
  { body: { basics: { profiles: ["github", "docs"] } } },
  "nested shorthand object and array literals parse"
);

assert.deepEqual(
  normalize(api.parseShorthand("body:{name:API Technologies}")),
  { body: { name: "API Technologies" } },
  "bare literal strings may contain spaces"
);

assert.deepEqual(
  normalize(api.parseShorthand('"meta:field": "value:with:colon"')),
  { "meta:field": "value:with:colon" },
  "quoted keys and values can contain colons"
);

assert.equal(
  api.parseQuerySource('{"status":201,"body":{"id":"new"}}').status,
  201,
  "normalized responses stay normalized"
);

assert.equal(api.outputActions.redirect.active.includes("format"), false, "raw redirect skips formatting");
console.log("restish-docs-interactions: parser and query fixtures passed");
