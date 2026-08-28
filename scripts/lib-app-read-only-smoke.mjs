import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { readdir, readFile, realpath } from "node:fs/promises";
import path from "node:path";

import { launchBrowser } from "./lib-app-browser.mjs";

const allowedMethods = new Set(["GET", "HEAD"]);
const servedAssetExtensions = new Set([".css", ".js", ".png", ".webmanifest"]);

export function normalizeReadOnlySmokeURL(value) {
  let url;
  try {
    url = new URL(value);
  } catch (error) {
    throw new Error(`read-only smoke requires a valid base URL: ${error.message}`);
  }
  if (!new Set(["http:", "https:"]).has(url.protocol)) {
    throw new Error(`read-only smoke requires HTTP(S), got ${url.protocol}`);
  }
  if (!new Set(["127.0.0.1", "::1", "localhost"]).has(url.hostname)) {
    throw new Error(`read-only smoke requires a loopback host, got ${url.hostname}`);
  }
  if (url.username || url.password || (url.pathname !== "" && url.pathname !== "/") || url.search || url.hash) {
    throw new Error("read-only smoke base URL must be an origin without credentials, path, query, or fragment");
  }
  return url;
}

export function classifyReadOnlyRequest(method, requestURL, allowedOrigin) {
  const normalizedMethod = String(method || "GET").toUpperCase();
  let url;
  try {
    url = new URL(requestURL);
  } catch {
    return { allowed: false, reason: "invalid_url", method: normalizedMethod, url: requestURL };
  }
  if (url.origin !== allowedOrigin) {
    return { allowed: false, reason: "external_origin", method: normalizedMethod, url: url.href };
  }
  if (!allowedMethods.has(normalizedMethod)) {
    return { allowed: false, reason: "mutating_method", method: normalizedMethod, url: url.href };
  }
  return { allowed: true, reason: "read", method: normalizedMethod, url: url.href };
}

export function parseLsofRecords(output) {
  const records = [];
  let current = null;
  for (const line of String(output).split(/\r?\n/)) {
    if (line.startsWith("p")) {
      if (current) records.push(current);
      current = { pid: Number(line.slice(1)), command: "", names: [] };
      continue;
    }
    if (!current || line.length < 2) continue;
    if (line.startsWith("c")) current.command = line.slice(1);
    if (line.startsWith("n")) current.names.push(line.slice(1));
  }
  if (current) records.push(current);
  return records.filter((record) => Number.isSafeInteger(record.pid) && record.pid > 0);
}

export async function runReadOnlyAppSmoke({
  assetRoot,
  baseURL,
  browserName,
  browserType,
  canaryBin,
  expectedCommit,
  launchOptions,
  mobile,
  requireReady,
}) {
  const base = normalizeReadOnlySmokeURL(baseURL);
  if (!path.isAbsolute(assetRoot)) {
    throw new Error("read-only smoke requires --asset-root as an absolute path");
  }
  if (!path.isAbsolute(canaryBin)) {
    throw new Error("read-only smoke requires --canary-bin as an absolute path");
  }
  if (!/^[0-9a-f]{40}$/.test(expectedCommit)) {
    throw new Error("read-only smoke requires --expected-commit as an exact lowercase Git SHA");
  }

  const runtime = await verifyRuntime({ base, canaryBin, expectedCommit, requireReady });
  const launched = await launchBrowser(browserType, browserName, launchOptions);
  const browser = launched.browser;
  try {
    const context = await browser.newContext({
      viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 },
      isMobile: mobile,
      hasTouch: mobile,
      serviceWorkers: "block",
    });
    const requests = [];
    const blocked = [];
    await context.route("**/*", async (route) => {
      const request = route.request();
      const classification = classifyReadOnlyRequest(request.method(), request.url(), base.origin);
      requests.push(classification);
      if (!classification.allowed) {
        blocked.push(classification);
        await route.abort("blockedbyclient");
        return;
      }
      await route.continue();
    });

    const assets = await verifyServedAssets(context, base, assetRoot);
    const page = await context.newPage();
    const pageErrors = [];
    const consoleErrors = [];
    page.on("pageerror", (error) => pageErrors.push(String(error?.message || error)));
    page.on("console", (message) => {
      if (message.type() === "error" || message.type() === "warning") {
        consoleErrors.push({ text: message.text(), url: message.location().url || "" });
      }
    });

    const navigation = await page.goto(new URL("/", base).href, { waitUntil: "domcontentloaded", timeout: 15000 });
    if (!navigation || navigation.status() !== httpStatusOK) {
      throw new Error(`read-only browser root status = ${navigation?.status() ?? "missing"}, want ${httpStatusOK}`);
    }
    await page.waitForSelector("#pairingPanel:not([hidden])", { timeout: 15000 });
    const browserState = await page.evaluate(() => ({
      bottom_tabs_hidden: document.getElementById("bottomTabs")?.hidden === true,
      device_id_stored: Boolean(localStorage.getItem("ibkrDeviceID")),
      device_key_stored: Boolean(localStorage.getItem("ibkrDeviceKeyJWK")),
      pairing_text: document.getElementById("pairingText")?.textContent?.trim() || "",
      title: document.title,
    }));
    if (browserState.title !== "Canary" || !browserState.bottom_tabs_hidden || !/scan a fresh qr code/i.test(browserState.pairing_text)) {
      throw new Error(`unpaired read-only browser state failed: ${JSON.stringify(browserState)}`);
    }
    if (browserState.device_id_stored || browserState.device_key_stored) {
      throw new Error(`read-only browser created a device credential: ${JSON.stringify(browserState)}`);
    }

    const credentialCookies = (await context.cookies(base.origin))
      .map((cookie) => cookie.name)
      .filter((name) => name === "ibkr_app_device" || name === "ibkr_app_session");
    if (credentialCookies.length > 0) {
      throw new Error(`read-only browser received app credentials: ${JSON.stringify(credentialCookies)}`);
    }
    if (blocked.length > 0) {
      throw new Error(`read-only browser attempted forbidden requests: ${JSON.stringify(blocked)}`);
    }
    if (pageErrors.length > 0) {
      throw new Error(`read-only browser page errors: ${pageErrors.join("\n")}`);
    }
    const unexpectedConsole = consoleErrors.filter((entry) => (
      !(/failed to load resource/i.test(entry.text) && entry.url.endsWith("/api/bootstrap"))
      && entry.text !== "Service Worker registration blocked by Playwright"
    ));
    if (unexpectedConsole.length > 0) {
      throw new Error(`read-only browser console errors: ${JSON.stringify(unexpectedConsole)}`);
    }

    const apiRequests = requests.filter((request) => new URL(request.url).pathname.startsWith("/api/"));
    if (apiRequests.length !== 1 || apiRequests[0].method !== "GET" || new URL(apiRequests[0].url).pathname !== "/api/bootstrap") {
      throw new Error(`unexpected read-only browser API requests: ${JSON.stringify(apiRequests)}`);
    }

    console.log(JSON.stringify({
      ok: true,
      mode: "read-only",
      browser: browserName,
      channel: launched.channel || null,
      base_url: base.origin,
      app_status: runtime.app_status,
      listener: runtime.listener,
      provenance: runtime.provenance,
      static_assets: assets,
      browser_state: browserState,
      api_requests: apiRequests.map((request) => ({ method: request.method, path: new URL(request.url).pathname })),
      mutating_requests: 0,
      pairing_sessions_created: 0,
      credential_cookies_created: 0,
    }, null, 2));
  } finally {
    await browser.close();
  }
}

const httpStatusOK = 200;

async function verifyRuntime({ base, canaryBin, expectedCommit, requireReady }) {
  const addr = base.host;
  const statusResult = await runCommand(canaryBin, ["app", "status", "--addr", addr, "--json"]);
  if (statusResult.code !== 0 && statusResult.code !== 1) {
    throw new Error(`canary app status failed (${statusResult.code}): ${statusResult.stderr.trim()}`);
  }
  const status = parseCommandJSON("canary app status", statusResult.stdout);
  if (status.schema_version !== "app-status-v1" || !new Set(["ready", "attention"]).has(status.state)) {
    throw new Error(`unexpected app status contract: ${JSON.stringify({ schema_version: status.schema_version, state: status.state })}`);
  }
  const ready = status.state === "ready";
  if ((statusResult.code === 0) !== ready) {
    throw new Error(`app status exit/state mismatch: exit=${statusResult.code} state=${status.state}`);
  }
  if (requireReady && !ready) {
    throw new Error(`app status requires attention: producer=${status.alert_producer?.coverage?.state || "unknown"} dispatcher=${status.alert_dispatcher?.state || "unknown"}`);
  }

  const versionResult = await runCommand(canaryBin, ["version", "--json"]);
  if (versionResult.code !== 0) {
    throw new Error(`canary version failed (${versionResult.code}): ${versionResult.stderr.trim()}`);
  }
  const version = parseCommandJSON("canary version", versionResult.stdout);
  if (version.commit !== expectedCommit) {
    throw new Error(`installed Canary commit = ${version.commit || "missing"}, want ${expectedCommit}`);
  }
  if (version.vcs_state && version.vcs_state !== "clean") {
    throw new Error(`installed Canary reports ${version.vcs_state} provenance`);
  }
  if (status.version !== version.version) {
    throw new Error(`running app version = ${status.version}, installed binary version = ${version.version}`);
  }

  const port = base.port || (base.protocol === "https:" ? "443" : "80");
  const listenerResult = await runCommand("lsof", ["-nP", `-iTCP:${port}`, "-sTCP:LISTEN", "-Fpctn"]);
  if (listenerResult.code !== 0) {
    throw new Error(`no listener evidence for ${base.host}: ${listenerResult.stderr.trim()}`);
  }
  const listeners = parseLsofRecords(listenerResult.stdout);
  if (listeners.length !== 1) {
    throw new Error(`expected one app listener on TCP ${port}, found ${listeners.length}: ${JSON.stringify(listeners)}`);
  }
  const listener = listeners[0];
  const executableResult = await runCommand("lsof", ["-a", "-p", String(listener.pid), "-d", "txt", "-Fn"]);
  if (executableResult.code !== 0) {
    throw new Error(`cannot resolve listener executable for pid ${listener.pid}: ${executableResult.stderr.trim()}`);
  }
  const executable = parseLsofRecords(executableResult.stdout)[0]?.names.find((name) => path.isAbsolute(name));
  if (!executable) {
    throw new Error(`listener pid ${listener.pid} has no executable evidence`);
  }
  const [actualExecutable, expectedExecutable] = await Promise.all([realpath(executable), realpath(canaryBin)]);
  if (actualExecutable !== expectedExecutable) {
    throw new Error(`listener executable = ${actualExecutable}, want ${expectedExecutable}`);
  }

  return {
    app_status: {
      ready,
      state: status.state,
      producer_coverage: status.alert_producer?.coverage?.state || "unknown",
      dispatcher_state: status.alert_dispatcher?.state || "unknown",
      version: status.version,
    },
    listener: {
      pid: listener.pid,
      command: listener.command,
      endpoints: listener.names,
      executable: actualExecutable,
    },
    provenance: {
      binary: expectedExecutable,
      commit: version.commit,
      version: version.version,
      vcs_state: version.vcs_state || "unknown",
    },
  };
}

async function verifyServedAssets(context, base, assetRoot) {
  const entries = await readdir(assetRoot, { withFileTypes: true });
  const names = entries
    .filter((entry) => entry.isFile() && (entry.name === "index.html" || servedAssetExtensions.has(path.extname(entry.name))))
    .map((entry) => entry.name)
    .sort();
  if (names.length === 0) {
    throw new Error(`no embedded app assets found under ${assetRoot}`);
  }

  const aggregate = createHash("sha256");
  for (const name of names) {
    const endpoint = name === "index.html" ? "/" : `/${name}`;
    const response = await context.request.get(new URL(endpoint, base).href, { failOnStatusCode: false, timeout: 10000 });
    if (response.status() !== httpStatusOK) {
      throw new Error(`served asset ${endpoint} status = ${response.status()}, want ${httpStatusOK}`);
    }
    const [served, source] = await Promise.all([response.body(), readFile(path.join(assetRoot, name))]);
    if (!served.equals(source)) {
      throw new Error(`served asset ${endpoint} does not match ${path.join(assetRoot, name)}`);
    }
    if (!String(response.headers()["cache-control"] || "").includes("no-cache")) {
      throw new Error(`served asset ${endpoint} lacks no-cache revalidation`);
    }
    aggregate.update(name);
    aggregate.update("\0");
    aggregate.update(source);
  }
  return { count: names.length, sha256: aggregate.digest("hex"), source_root: assetRoot };
}

function parseCommandJSON(label, output) {
  try {
    return JSON.parse(output);
  } catch (error) {
    throw new Error(`${label} returned invalid JSON: ${error.message}`);
  }
}

function runCommand(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.on("error", reject);
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}
