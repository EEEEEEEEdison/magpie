// Run with Node's test runner and Playwright on the module path; see README.md.
// The same output can arrive over very different whole-turn durations. Check
// each request, the period card, the full ranking and the tray's folded rows.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const now = Date.now();

const group = (id, input, totals = {}) => ({ id, calls: 1, input, output: totals.decode_out || totals.floor_out || 0, cache_read: 0, cache_write: 0, cost: 0, ...totals });
const GROUPS = [
  // Two floors of 1,000 tokens over 10 s and 40 s give a floor of 40 tok/s.
  group("floors", 9e6, { calls: 2, floor_out: 2000, floor_ms: 50000, floor_n: 2 }),
  group("measured", 8e6, { calls: 2, timed: 2, ttft_ms: 1000, decode_out: 400, decode_ms: 10000, decode_n: 2 }),
  ...["a", "b", "c", "d", "e"].map((id) => group(id, 7e6)),
  // These two fall past the tray's seven named rows. Together they are 60 tok/s.
  group("small-1", 1000, { timed: 1, ttft_ms: 500, decode_out: 300, decode_ms: 1000, decode_n: 1 }),
  group("small-2", 900, { timed: 1, ttft_ms: 4500, decode_out: 300, decode_ms: 9000, decode_n: 1 }),
];
const SUM = ["calls", "input", "output", "timed", "ttft_ms", "decode_ms", "decode_out", "decode_n", "floor_out", "floor_ms", "floor_n"];
const ROWS = GROUPS.slice(-2).map((g, i) => ({
  model: g.id, req: g.id, served: g.id, provider: "relay", agent: "claude", status: 200,
  t: new Date(now - (i + 1) * 60e3).toISOString(), in: g.input, out: 300, reasoning: 100, ms: g.decode_ms, ttft_ms: g.ttft_ms,
}));
function ledger(period) {
  const tot = {};
  for (const k of SUM) tot[k] = GROUPS.reduce((a, x) => a + (x[k] || 0), 0);
  return {
    period, rows: ROWS, offset: 0, total: ROWS.length, errors: 0, cache_read: 0, cache_write: 0, cost: 0, unpriced: 0, ...tot,
    by: { model: GROUPS }, bucket: "hour", series: [],
    agents: [{ id: "claude", name: "Claude Code", icon: "claudecode-color" }],
  };
}

const STATE = (lang) => ({
  agents: [{ id: "claude", name: "Claude Code", path: "/t/c.json", icon: "claudecode-color", wired: true, fields: [] }],
  profiles: [], settings: { lang, theme: "light" }, fx: { rate: 7.2, at: new Date(now).toISOString() },
});

function server(lang) {
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(STATE(lang));
    if (url.pathname === "/api/settings") return json(STATE(lang).settings);
    if (url.pathname === "/api/usage/requests") return json(ledger(url.searchParams.get("period")));
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/usage") return json({ calls: 0, errors: 0, input: 0, output: 0, reasoning: 0, unpriced: 0, cost: 0, bucket: "day", series: [], path: "", agents: [], models: [] });
    if (url.pathname === "/api/providers") return json({ providers: [], presets: [], excluded: [], models: [], gateway: { running: false } });
    if (url.pathname === "/api/sessions") return json({ sessions: [], dirs: [] });
    if (url.pathname === "/api/sessions/stats") return json({ from: "", to: "", days: [], agents: {} });
    if (url.pathname === "/api/groups") return json({ groups: [], models: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const unit = { en: "tok/s", zh: "token/秒", "zh-TW": "token/秒", ja: "トークン/秒", de: "Token/s" };

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(engine + ": group speeds count total output over total whole-turn time", async (t) => {
    assert(["chromium", "webkit"].includes(engine), "BROWSER must be chromium or webkit");
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    t.after(() => browser.close());
    for (const lang of Object.keys(unit)) {
      for (const width of [1400, 560]) {
        const tok = (n) => `${n} ${unit[lang]}`;
        await t.test(lang + " " + width, async () => {
          const errors = [];
          const ctx = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
          const p = await ctx.newPage();
          p.setDefaultTimeout(5000);
          p.setDefaultNavigationTimeout(60000);
          p.on("pageerror", (e) => errors.push(e.message));
          await p.route("**/*", server(lang));
          await p.goto("http://magpie.test/?view=usage&tab=requests");
          const rows = p.locator("#ledWrap tr.led-row td.speed");
          await rows.first().waitFor({ state: "attached" });
          assert.deepEqual((await rows.allTextContents()).map((s) => s.trim()), [tok(300), tok(33)]);

          const rank = (id) => p.locator("#ledRank .rk").filter({ has: p.locator(".rk-nm", { hasText: id }) }).locator(".rk-speed");
          assert((await rank("floors").textContent()).startsWith("≥" + tok(40)));
          assert((await rank("measured").textContent()).startsWith(tok(40)));
          assert((await rank("small-1").textContent()).startsWith(tok(300)));
          assert((await rank("small-2").textContent()).startsWith(tok(33)));
          const card = p.locator("#ledKpi .blk").nth(4);
          assert((await card.locator(".v").textContent()).startsWith(tok(50))); // 1,000 / 20 s
          assert((await card.locator(".sub").first().textContent()).startsWith("≥" + tok(40)));

          await p.addInitScript(() => {
            localStorage.setItem("magpie.panelTab", "stats");
            localStorage.setItem("magpie.panelUseSplit", "model");
            localStorage.setItem("magpie.panelUseMetric", "tokens");
          });
          await p.goto("http://magpie.test/?mode=panel");
          const other = p.locator("#panelUsage .led-rank .rk.plain .rk-speed");
          assert((await other.textContent()).startsWith(tok(60)));
          assert.deepEqual(errors, []);
          await ctx.close();
        });
      }
    }
  });
}
