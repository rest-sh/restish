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

function complete(input) {
  return JSON.parse(JSON.stringify(api.completeCommand(input, input.length)));
}

assert.deepEqual(
  complete("restish ex"),
  {
    value: "restish example ",
    cursor: "restish example ".length,
    matches: ["example"],
    applied: true
  },
  "root command completion"
);

assert.deepEqual(
  complete("restish example li"),
  {
    value: "restish example list-",
    cursor: "restish example list-".length,
    matches: ["list-books", "list-images", "list-items"],
    applied: true
  },
  "shared prefix completion for generated operations"
);

assert.deepEqual(
  complete("restish example list-im"),
  {
    value: "restish example list-images ",
    cursor: "restish example list-images ".length,
    matches: ["list-images"],
    applied: true
  },
  "single generated operation completion"
);

assert.deepEqual(
  complete("restish example get-image "),
  {
    value: "restish example get-image ",
    cursor: "restish example get-image ".length,
    matches: ["gif", "heic", "jpeg", "png", "webp"],
    applied: false
  },
  "generated path parameter suggestions"
);

assert.deepEqual(
  complete("restish api.rest.sh/images -o j"),
  {
    value: "restish api.rest.sh/images -o json ",
    cursor: "restish api.rest.sh/images -o json ".length,
    matches: ["json"],
    applied: true
  },
  "output format value completion"
);

const printCompletion = complete("restish api.rest.sh/images --rsh-print H");
assert.ok(printCompletion.matches.includes("HBhbp"), "print transcript completion should include HBhbp");
assert.equal(
  printCompletion.matches.includes("Hhbp"),
  false,
  "print transcript completion should not include the invalid Hhbp spec"
);

assert.deepEqual(
  complete("restish api.rest.sh/im"),
  {
    value: "restish api.rest.sh/images",
    cursor: "restish api.rest.sh/images".length,
    matches: ["api.rest.sh/images", "api.rest.sh/images/jpeg"],
    applied: true
  },
  "URL-ish docs path completion"
);

assert.equal(
  api.shouldApplyCompletionKey({ key: "Tab", shiftKey: false }),
  true,
  "plain Tab should apply completions"
);
assert.equal(
  api.shouldApplyCompletionKey({ key: "Tab", shiftKey: true }),
  false,
  "Shift+Tab should keep the browser focus traversal behavior"
);

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
