import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { readdirSync } from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const siteDir = path.resolve(__dirname, "..");
const repoDir = path.resolve(siteDir, "..");

const source = await readFile(path.join(siteDir, "static/js/restish-playground.js"), "utf8");
const sandbox = {
  AbortController,
  URL,
  URLSearchParams,
  console,
  document: {
    readyState: "loading",
    addEventListener() {},
    createElement() {
      return {};
    },
    querySelectorAll() {
      return [];
    }
  },
  fetch() {
    throw new Error("fetch should not be called by playground unit tests");
  },
  navigator: {},
  window: {
    __RESTISH_PLAYGROUND_TEST__: true,
    addEventListener() {},
    clearTimeout,
    isSecureContext: false,
    requestAnimationFrame(callback) {
      callback();
    },
    setTimeout
  },
  MutationObserver: class {
    observe() {}
  }
};

vm.createContext(sandbox);
vm.runInContext(source, sandbox, { filename: "restish-playground.js" });

const api = sandbox.window.__restishPlaygroundTest;
assert.ok(api, "playground test API should be exposed");

const toonPlan = api.parseCommand("restish get https://api.rest.sh/items -o toon");
assert.equal(toonPlan.flags.outputFormat, "toon");
assert.equal(
  api.render({
    proto: "HTTP/2.0",
    status: 200,
    headers: { "Content-Type": "application/json" },
    body: { items: [{ id: 1, name: "Ada" }] }
  }, toonPlan, null),
  "items[1]{id,name}:\n  1,Ada\n"
);

const fixtureDir = path.join(repoDir, "internal/output/testdata/toon/encode");
let checked = 0;
for (const file of readdirSync(fixtureDir).filter((name) => name.endsWith(".json")).sort()) {
  const fixture = JSON.parse(await readFile(path.join(fixtureDir, file), "utf8"));
  for (const testCase of fixture.tests) {
    if (testCase.shouldError || !usesDefaultTOONOptions(testCase.options || {})) {
      continue;
    }
    checked += 1;
    assert.equal(
      api.encodeTOONDocument(testCase.input),
      testCase.expected,
      `${file}: ${testCase.name}`
    );
  }
}

assert.ok(checked > 0, "TOON fixture suite should include default-option cases");
console.log(`restish-playground: ${checked} TOON encode fixtures passed`);

function usesDefaultTOONOptions(options) {
  return Object.entries(options).every(([key, value]) => {
    if (key === "delimiter") return value === ",";
    if (key === "indent") return value === 2;
    if (key === "keyFolding") return value === "off";
    return false;
  });
}
