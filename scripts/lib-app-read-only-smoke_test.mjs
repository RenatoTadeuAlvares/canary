import assert from "node:assert/strict";
import test from "node:test";

import { classifyReadOnlyRequest, normalizeReadOnlySmokeURL, parseLsofRecords, readOnlySmokeAuthMode } from "./lib-app-read-only-smoke.mjs";

test("fresh browser accepts only an unpaired response or a strict read-only preview grant", () => {
  assert.equal(readOnlySmokeAuthMode(401, null), "unpaired");
  assert.equal(readOnlySmokeAuthMode(200, { authenticated: true, read_only: true }), "preview");
  for (const auth of [undefined, {}, { authenticated: true }, { authenticated: true, read_only: false }, { authenticated: false, read_only: true }, { authenticated: true, read_only: "true" }]) {
    assert.throws(() => readOnlySmokeAuthMode(200, auth), /explicit read-only preview grant/);
  }
  assert.throws(() => readOnlySmokeAuthMode(403, { authenticated: true, read_only: true }), /explicit read-only preview grant/);
});

test("read-only smoke accepts only local origin URLs", () => {
  assert.equal(normalizeReadOnlySmokeURL("http://127.0.0.1:8765").origin, "http://127.0.0.1:8765");
  assert.equal(normalizeReadOnlySmokeURL("http://localhost:8765/").origin, "http://localhost:8765");
  assert.throws(() => normalizeReadOnlySmokeURL("https://example.com"), /loopback host/);
  assert.throws(() => normalizeReadOnlySmokeURL("http://127.0.0.1:8765/path"), /must be an origin/);
});

test("read-only browser policy permits same-origin reads only", () => {
  const origin = "http://127.0.0.1:8765";
  assert.equal(classifyReadOnlyRequest("GET", `${origin}/app.js`, origin).allowed, true);
  assert.equal(classifyReadOnlyRequest("HEAD", `${origin}/manifest.webmanifest`, origin).allowed, true);
  assert.equal(classifyReadOnlyRequest("GET", `${origin}/api/update`, origin).allowed, true);
  assert.equal(classifyReadOnlyRequest("POST", `${origin}/api/update`, origin).reason, "mutating_method");
  assert.deepEqual(
    classifyReadOnlyRequest("POST", `${origin}/api/pairing/sessions`, origin).reason,
    "mutating_method",
  );
  assert.deepEqual(
    classifyReadOnlyRequest("GET", "https://example.com/app.js", origin).reason,
    "external_origin",
  );
});

test("lsof field output preserves listener ownership", () => {
  assert.deepEqual(parseLsofRecords("p21511\nccanary\nf7\ntIPv6\nn*:8765\n"), [{
    pid: 21511,
    command: "canary",
    names: ["*:8765"],
  }]);
  assert.deepEqual(parseLsofRecords("p21511\nftxt\nn/Users/test/.local/bin/canary\nftxt\nn/usr/lib/dyld\n"), [{
    pid: 21511,
    command: "",
    names: ["/Users/test/.local/bin/canary", "/usr/lib/dyld"],
  }]);
});
