// Run with Node's test runner and Playwright on the module path; see README.md.
// 微雨南山 and stones on Discord: WorkBuddy, Claude Desktop and the Codex app
// read magpie's settings only as they start. A row whose app started before
// magpie's last change has a Restart; it asks in magpie's own dialog, and
// only then quits the app and opens it again. Cancel quits nothing. Every
// language, both engines, the window wide and narrow and the tray panel. No
// backend, the API is faked here.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const days = (n) => new Date(Date.now() - n * 86400e3).toISOString();
const model = (value) => [{ key: "model", label: "model", value, options: [{ value: "deepseek-v4", label: "DeepSeek V4", ref: "deepseek/deepseek-v4", note: "DeepSeek · via magpie" }] }];
const workbuddy = { id: "workbuddy", name: "WorkBuddy", icon: "generic", path: "/fixture/models.json", wired: true, fields: model("deepseek-v4"), appRestart: { since: days(1) } };
// disconnected in magpie, the app still runs on magpie's settings
const claude = { id: "claude-desktop", name: "Claude Desktop", icon: "generic", path: "/fixture/claude", wired: false, fields: model(""), appRestart: { since: days(1) } };
// connected, running on the current settings: Restart is still at hand
const zcode = { id: "zcode", name: "ZCode", icon: "zcode", path: "/fixture/zcode", wired: true, fields: model("deepseek-v4"), appRunning: { since: days(1) } };
const codexApp = { id: "codex", name: "Codex", icon: "generic", path: "/fixture/codex", wired: true, fields: model("deepseek-v4"), appRunning: { since: days(1) } };

const words = {
  en: { restart: "Restart", cancel: "Cancel", line: "Connected · restart WorkBuddy to use it", done: "WorkBuddy restarted", still: "WorkBuddy is still open; it may be asking something in its window." },
  zh: { restart: "重启", cancel: "取消", line: "已连接 · 重启 WorkBuddy 后生效", done: "WorkBuddy 已重启", still: "WorkBuddy" },
  "zh-TW": { restart: "重啟", cancel: "取消", line: "已連接 · 重新啟動 WorkBuddy 後生效", done: "WorkBuddy 已重新啟動", still: "WorkBuddy" },
  ja: { restart: "再起動", cancel: "キャンセル", line: "接続済み · WorkBuddy を再起動すると反映されます", done: "WorkBuddy を再起動しました", still: "WorkBuddy" },
  de: { restart: "Neu starten", cancel: "Abbrechen", line: "Verbunden · WorkBuddy neu starten, um es zu nutzen", done: "WorkBuddy neu gestartet", still: "WorkBuddy" },
};
const stillOpen = "WorkBuddy is still open; it may be asking something in its window. Answer it there, or quit it, then open it again.";

function serve(lang, calls, fail) {
  let agents = [workbuddy, claude, zcode, codexApp];
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data, status = 200) => route.fulfill({ status, json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    const st = () => ({ agents, profiles: [], settings: { lang, theme: "light" } });
    if (url.pathname.startsWith("/api/agents/restart/")) {
      const id = url.pathname.split("/").pop();
      calls.push(route.request().method() + " " + id);
      if (fail) return json({ error: stillOpen }, 409);
      agents = agents.map((a) => (a.id === id ? { ...a, appRestart: undefined } : a));
      return json(st());
    }
    if (url.pathname.startsWith("/api/agents/restart-app/")) {
      const id = url.pathname.split("/").pop();
      calls.push(route.request().method() + " restart-app " + id);
      if (fail) return json({ error: stillOpen }, 409);
      return json(st());
    }
    if (url.pathname === "/api/state") return json(st());
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/agents/cli") return json({ agents: {}, pending: false });
    if (url.pathname === "/api/providers") return json({ providers: [], presets: [], excluded: [], gateway: { running: true, window: true } });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const row = (id) => `.row.agent[data-id="${id}"]`;

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of Object.keys(words)) {
    const w = words[lang];
    for (const [mode, width] of [["", 980], ["", 440], ["panel", 440]]) {
      test(`${engine} ${lang} ${mode || "window"} ${width}px: Restart asks, then restarts the app`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        t.after(() => browser.close());
        const page = await browser.newPage({ viewport: { width, height: 800 } });
        page.setDefaultTimeout(5000);
        page.setDefaultNavigationTimeout(60000);
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        const calls = [];
        await page.route("**/*", serve(lang, calls));
        await page.goto("http://magpie.test/" + (mode ? "?mode=" + mode : "?view=agents"));
        const go = page.locator(`${row("workbuddy")} .ag-restart`);
        await go.waitFor();
        assert.equal(await go.getAttribute("aria-label"), lang === "en" ? "Restart WorkBuddy" : await go.getAttribute("aria-label"));
        assert.doesNotMatch(await go.getAttribute("aria-label"), /\{\w+\}/);
        assert.doesNotMatch(await go.getAttribute("title"), /\{\w+\}/);
        assert.ok((await go.getAttribute("title")).includes("WorkBuddy"));
        if (!mode) {
          assert.equal((await go.textContent()).trim(), w.restart);
          assert.ok((await page.locator(`${row("workbuddy")} .ag-st-t`).textContent()).includes(w.line));
        }
        // not connected in magpie, Claude Desktop still runs on its settings
        await page.locator(`${row("claude-desktop")} .ag-restart`).waitFor();
        const box = await go.boundingBox();
        assert.ok(box.x >= 0 && box.x + box.width <= width, "past the window: " + JSON.stringify(box));
        const side = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
        assert.ok(side <= 1, "scrolls sideways: " + side);

        // asked first; Cancel and Escape quit nothing
        await go.click();
        const ask = page.locator("dialog.action-confirm[open]");
        await ask.waitFor();
        assert.ok((await ask.locator("h2").textContent()).includes("WorkBuddy"));
        assert.doesNotMatch(await ask.textContent(), /\{\w+\}/);
        assert.equal((await ask.locator("button.primary").textContent()).trim(), w.restart);
        await ask.locator("button", { hasText: w.cancel }).click();
        await ask.waitFor({ state: "detached" });
        await go.click();
        await ask.waitFor();
        await page.keyboard.press("Escape");
        await ask.waitFor({ state: "detached" });
        assert.deepEqual(calls, []);

        await go.click();
        await ask.locator("button.primary").click();
        await page.locator(`${row("workbuddy")} .ag-restart`).waitFor({ state: "detached" });
        assert.deepEqual(calls, ["POST workbuddy"]);
        assert.equal(await page.locator("#status").textContent(), w.done);
        assert.equal(await page.locator(`${row("claude-desktop")} .ag-restart`).count(), 1, "only the app asked for");
        assert.deepEqual(errors, []);
      });
    }

    test(`${engine} ${lang}: a restart that fails says why, in the reader's language`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await browser.newPage({ viewport: { width: 980, height: 800 } });
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const calls = [];
      await page.route("**/*", serve(lang, calls, true));
      await page.goto("http://magpie.test/?view=agents");
      const go = page.locator(`${row("workbuddy")} .ag-restart`);
      await go.click();
      await page.locator("dialog.action-confirm[open] button.primary").click();
      await page.waitForFunction(() => document.querySelector("#status").classList.contains("err"));
      const said = await page.locator("#status").textContent();
      assert.ok(said.includes("WorkBuddy"), said);
      if (lang === "en") assert.equal(said, stillOpen);
      else assert.notEqual(said, stillOpen, "not translated");
      assert.equal(await go.isDisabled(), false, "the button comes back for another try");
      assert.deepEqual(calls, ["POST workbuddy"]);
      assert.deepEqual(errors, []);
    });
  }
}

// A connected app running on the current settings has a Restart too: the
// page's own action, not a shortcut to find after a change magpie can't
// see. Codex's restarts only its app-server, as its stale-copy Restart does.
for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of Object.keys(words)) {
    test(`${engine} ${lang}: a running app has its own Restart`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await browser.newPage({ viewport: { width: 980, height: 800 } });
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const calls = [];
      await page.route("**/*", serve(lang, calls));
      await page.goto("http://magpie.test/?view=agents");
      const w = await page.evaluate((lang) => {
        const tr = (k, vars = {}) => ((lang === "en" ? k : I18N[lang]?.[k]) || k).replace(/\{(\w+)\}/g, (_, n) => vars[n]);
        return {
          restart: tr("Restart"),
          aria: tr("Restart {agent}", { agent: "ZCode" }),
          title: tr("Restart {agent} to read the current settings", { agent: "ZCode" }),
          quit: tr("magpie quits {agent} and opens it again, so it reads the new settings. Anything running in it stops.", { agent: "ZCode" }),
          done: tr("{agent} restarted", { agent: "ZCode" }),
          appServer: tr("Only its app-server restarts, for the new settings; its window stays. A running turn stops."),
        };
      }, lang);
      const go = page.locator(`${row("zcode")} .ag-restart-now`);
      await go.waitFor();
      assert.equal((await go.textContent()).trim(), w.restart);
      assert.equal(await go.getAttribute("aria-label"), w.aria);
      assert.equal(await go.getAttribute("title"), w.title);
      const box = await go.boundingBox();
      assert.ok(box.x >= 0 && box.x + box.width <= 980, "past the window: " + JSON.stringify(box));
      const side = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      assert.ok(side <= 1, "scrolls sideways: " + side);

      await go.click();
      const ask = page.locator("dialog.action-confirm[open]");
      await ask.waitFor();
      assert.ok((await ask.locator("h2").textContent()).includes("ZCode"));
      assert.ok((await ask.textContent()).includes(w.quit));
      await ask.locator("button.primary").click();
      await page.waitForFunction((want) => document.querySelector("#status")?.textContent === want, w.done);
      assert.deepEqual(calls, ["POST restart-app zcode"]);
      // the app still runs, so its Restart stays at hand
      await page.locator(`${row("zcode")} .ag-restart-now`).waitFor();

      // Codex's is the app-server's Restart, never a quit and reopen
      const codex = page.locator(`${row("codex")} .ag-restart-now`);
      await codex.click();
      await ask.waitFor();
      assert.ok((await ask.locator("h2").textContent()).includes("Codex"));
      assert.ok((await ask.textContent()).includes(w.appServer));
      await page.keyboard.press("Escape");
      await ask.waitFor({ state: "detached" });
      assert.deepEqual(calls, ["POST restart-app zcode"]);
      assert.deepEqual(errors, []);
    });
  }

  for (const [mode, width] of [["", 440], ["panel", 440]]) {
    test(`${engine} ${mode || "window"} ${width}px: a running app's Restart fits`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await browser.newPage({ viewport: { width, height: 800 } });
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", serve("en", [], false));
      await page.goto("http://magpie.test/" + (mode ? "?mode=" + mode : "?view=agents"));
      for (const id of ["zcode", "codex"]) {
        const go = page.locator(`${row(id)} .ag-restart-now`);
        await go.waitFor();
        const box = await go.boundingBox();
        assert.ok(box.x >= 0 && box.x + box.width <= width, id + " past the window: " + JSON.stringify(box));
      }
      const side = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      assert.ok(side <= 1, "scrolls sideways: " + side);
      assert.deepEqual(errors, []);
    });
  }

  test(`${engine}: a running app's Restart that fails says why, and comes back`, async (t) => {
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    t.after(() => browser.close());
    const page = await browser.newPage({ viewport: { width: 980, height: 800 } });
    page.setDefaultTimeout(5000);
    page.setDefaultNavigationTimeout(60000);
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const calls = [];
    await page.route("**/*", serve("en", calls, true));
    await page.goto("http://magpie.test/?view=agents");
    const go = page.locator(`${row("zcode")} .ag-restart-now`);
    await go.click();
    await page.locator("dialog.action-confirm[open] button.primary").click();
    await page.waitForFunction(() => document.querySelector("#status").classList.contains("err"));
    assert.equal(await page.locator("#status").textContent(), stillOpen);
    assert.equal(await go.isDisabled(), false, "the button comes back for another try");
    assert.deepEqual(calls, ["POST restart-app zcode"]);
    assert.deepEqual(errors, []);
  });
}
