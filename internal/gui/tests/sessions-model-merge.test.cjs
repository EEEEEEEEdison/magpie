// Run with Node's test runner and Playwright on the module path; see README.md.
// The Sessions tab counts a model by its own name, not the one it was sold
// under: one model called as workbuddy2api/glm-5.3-flash by one provider and
// as glm-5.3-flash by another is one bar, one entry in the picker and one
// filter, both spellings' tokens under it (yetone: 另外这里模型肯定是合并过
// 的，不能出现前缀的). The overview behind the filter is asked for that name,
// which Stats.Overview matches either spelling of a session's models against
// (provider.BareModel, as the Requests tab's model ranking and
// usage.Filter.keepsRecord already do). English and Chinese, Chromium and
// WebKit; no backend, the API is faked.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const iso = (d) => [d.getFullYear(), d.getMonth() + 1, d.getDate()].map((n) => String(n).padStart(2, "0")).join("-");
const to = new Date();
const today = iso(to);
// one model, two names: its seller's in front of it, and its own
const SOLD = "workbuddy2api/glm-5.3-flash", OWN = "glm-5.3-flash";
const rows = [
  { agent: "claude", cwd: "/work/alpha", model: SOLD, bare: OWN, input: 30000, output: 3000, cache_read: 0, cache_write: 0, cost: 1.5, priced: true },
  { agent: "claude", cwd: "/work/beta", model: OWN, bare: OWN, input: 10000, output: 1000, cache_read: 0, cache_write: 0, cost: 0.5, priced: true },
  { agent: "claude", cwd: "/work/alpha", model: "gpt-5", bare: "gpt-5", input: 1000, output: 100, cache_read: 0, cache_write: 0, cost: 0.1, priced: true },
];
const days = [{ date: today, usage: rows, active: [] }];
const sess = (id, cwd, model) => ({
  key: "claude:" + id, agent: "claude", name: "Claude Code", icon: "claude", id, cwd, title: "task " + id,
  start: to.toISOString(), last: to.toISOString(), input: 30000, output: 3000, cache_read: 0, cache_write: 0, cost: 1.5, priced: true,
  models: [{ model, bare: OWN, input: 30000, output: 3000, cache_read: 0, cache_write: 0, cost: 1.5, priced: true }],
  resume: "claude --resume " + id,
});
const sessions = [sess("a", "/work/alpha", SOLD), sess("b", "/work/beta", OWN)];

function serve(lang, seen) {
  const state = { agents: [{ id: "claude", name: "Claude Code", path: "/test/settings.json", fields: [] }], profiles: [], settings: { lang, theme: "light" } };
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    const q = url.searchParams;
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/sessions/progress") return json({ indexing: false });
    if (url.pathname === "/api/sessions") return json({ sessions, dirs: ["/test/sessions"] });
    if (url.pathname === "/api/sessions/stats") return json({ from: today, to: today, days, agents: { claude: "Claude Code" } });
    if (url.pathname === "/api/sessions/overview") {
      seen.push(q.toString());
      return json({ count: 2, median: 44000, p90: 44000, days: [2], top: { tokens: [], cost: [], active: [] }, messages: [30], output: [0],
        shape: { messages: { edges: [1, 6, 16, 31, 61, 121], counts: [1, 1, 0, 0, 0, 0], total: 2 }, minutes: { edges: [1, 6, 16, 31, 61, 121], counts: [1, 1, 0, 0, 0, 0], total: 2 },
          autonomy: { edges: [0, 1, 3, 6, 11, 21], counts: [0, 1, 1, 0, 0, 0], total: 2 } },
        tools: { calls: 0, sessions: 0, top: [], categories: [], weeks: [] }, skills: { calls: 0, count: 0, top: [] } });
    }
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/usage/quotas") return json([]); // an array, as the Go side writes it
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    const body = await fs.readFile(file).catch(() => null);
    await (body ? route.fulfill({ body, contentType }) : route.fulfill({ status: 404, body: "" }));
  };
}

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh"]) {
    test(`${engine} ${lang}: a model is one row, under its own name`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      const context = await browser.newContext({ viewport: { width: 1100, height: 760 }, reducedMotion: "reduce", timezoneId: "Asia/Shanghai" });
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      const errors = [], seen = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", serve(lang, seen));
      await page.addInitScript(() => { localStorage.setItem("magpie.usageTab", "sessions"); localStorage.setItem("magpie.sessRange", "all"); });
      t.after(() => browser.close());
      await page.goto("http://magpie.test/?view=usage");
      await page.locator("#sessGrid:not([hidden])").waitFor();
      await page.waitForFunction(() => document.querySelector("#sessStats .kpi b"));

      const bars = page.locator("#sessModels .sess-bar");
      await bars.first().waitFor();
      assert.deepEqual(await bars.locator(".n").allInnerTexts(), [OWN, "gpt-5"], "one model's two names are one bar, another model its own");
      assert.doesNotMatch(await page.locator("#sessModels").innerText(), /workbuddy2api/, "no seller's name on the panel");
      // what the model spent under both names, and the page's own token total
      const fmt = (n) => page.evaluate((x) => fmtN(x), n);
      assert.equal((await bars.first().locator(".v").innerText()).trim(), await fmt(44000));
      assert.equal((await page.locator("#sessStats .kpi").nth(1).locator("b").innerText()).trim(), await fmt(45100));
      // the picker offers it once, under that name too
      await page.locator("#sessModel").click();
      const menu = page.locator(".sess-menu:visible");
      await menu.waitFor();
      const names = (await menu.locator(".pm-name").allInnerTexts()).map((s) => s.trim());
      assert(names.includes(OWN) && names.includes("gpt-5"), names.join(", "));
      assert(!names.some((n) => n.includes("workbuddy2api")), names.join(", "));
      await page.locator("#sessModel").click(); // the menu closes on its button

      // picked: both sessions called it, either way round, so both stay listed
      await bars.first().click();
      await page.waitForFunction(() => document.querySelector("#sessModels .sess-bar.on"));
      assert.equal(await page.locator("#sessList .row.sess").count(), 2);
      assert(seen.some((s) => s.includes("model=glm-5.3-flash")), seen.join("\n"));
      assert(!seen.some((s) => s.includes("workbuddy2api")), "the overview is asked for the model's own name: " + seen.join("\n"));
      assert.deepEqual(errors, []);
    });
  }
}
