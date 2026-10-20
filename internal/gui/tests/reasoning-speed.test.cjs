// Run with Node's test runner and Playwright on the module path; see README.md.
// A reply that reasoned is timed by its whole turn (usage.DecodeOf): every
// token it wrote, its reasoning among them, over the ms from the request to
// its last token, the wait for its first one in that time. A Codex turn of
// 2000 reasoning tokens, written before its first content at 14 s, and a
// 200-token answer over the 1.5 s after, is 2200 tokens in 15.5 s, 142 — not
// the 200 in 1.5 s, 133, its answer from its first text counted (tony on
// Discord: gpt-6.1-sol read 163 tok/s with a 14 s first token). So one whose
// answer came after its reasoning, one that wrote only tool calls, one whose
// summary came late and one with redacted thinking all tell their turn's
// speed alike, and one that didn't reason is timed as before. An older route
// without its own reasoning count is timed as any other. The Routing page's
// rows, a session's average and the Usage page's rows and their tooltip all
// say it, in English, Chinese, Chinese (Taiwan), Japanese and German; no
// backend, the API is faked.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const now = new Date();
const day = [now.getFullYear(), now.getMonth() + 1, now.getDate()].map((n) => String(n).padStart(2, "0")).join("-");
const seat = { id: "codex", provider: "codex", name: "Codex", who: "test@example.com", kind: "account", model: "gpt-6.1-sol" };
const at = (i) => new Date(now.getTime() - (i + 1) * 60e3).toISOString();
// newest first: [out, reasoning (top level), reasoning (usage only), ms, ttft, firstText]
const timing = [
  [2200, 2000, 0, 15500, 14000, 14000], // the whole turn: 2200 in 15.5 s, 142 tok/s
  [1300, 0, 1000, 9000, 6000, 6000],    // an older route without its own count: 1300 in 9 s, 144
  [1500, 1400, 0, 9000, 8000, 0],       // reasoning, then tool calls only: 1500 in 9 s, 167
  [500, 0, 0, 7000, 2000, 2000],         // no reasoning: 500 in 7 s, 71 tok/s
  [1500, 1400, 0, 15000, 1000, 0],      // its content over 12 s of its 15 s: 100 tok/s
  [1500, 1400, 0, 9000, 8000, 0],       // a late summary: 1500 in 9 s, 167
  [120, 0, 0, 9000, 1000, 0],           // redacted thinking without a count: 120 in 9 s, 13
];
const routes = timing.map(([out, reasoning, usageReasoning, ms, ttft, firstText], i) => ({
  ...(i === 4 ? { firstReasoning: 1000, flow: 12000 } : {}),
  ...(i === 6 ? { reasoningHidden: true, flow: 7000 } : {}),
  id: 100 - i, seq: 100 - i, time: at(i), agent: "codex", session: "think-chat", model: "codex/gpt-6.1-sol", provider: "codex",
  order: [seat], tries: [{ id: seat.id, model: seat.model, start: at(i), done: true, status: 200, ms, ttft, firstText }],
  done: true, status: 200, ms, ttft, firstText, tokens: out + 9000, out, ...(reasoning ? { reasoning } : {}),
  usage: [{ provider: "codex", model: seat.model, in: 9000, out, ...(reasoning || usageReasoning ? { reasoning: reasoning || usageReasoning } : {}) }],
}));
const rows = timing.map(([out, reasoning, usageReasoning, ms, ttft, firstText], i) => ({
  ...(i === 4 ? { first_reasoning_ms: 1000, flow_ms: 12000 } : {}),
 ...(i === 6 ? { reasoning_hidden: true, flow_ms: 7000 } : {}),
  route_id: 100 - i, t: at(i), agent: "codex", agentName: "Codex", icon: "codex-color", provider: "codex", providerName: "Codex",
  host: "test@example.com", req: "codex/gpt-6.1-sol", model: "gpt-6.1-sol", served: "gpt-6.1-sol", in: 9000, out,
  ...(reasoning || usageReasoning ? { reasoning: reasoning || usageReasoning } : {}), ms, ttft_ms: ttft, ...(firstText ? { first_text_ms: firstText } : {}),
  status: 200, ep: "/backend-api/codex/responses", cost: 0.01, priced: true,
}));
// what the server sums: all 8620 tokens of the 7 turns, in their 73.5 s of turns
const M = { id: "gpt-6.1-sol", calls: 7, input: 63000, output: 8620, cache_read: 0, cache_write: 0, cost: 0.04, timed: 7, ttft_ms: 40000, decode_ms: 73500, decode_out: 8620 };

function serve(lang) {
  const state = { agents: [{ id: "codex", name: "Codex", path: "/t/c.toml", icon: "codex-color", fields: [] }], profiles: [], settings: { lang, theme: "light" } };
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/gateway/trace") {
      if (url.searchParams.get("wait")) await new Promise((r) => setTimeout(r, 20e3));
      return json({ mine: true, now: now.toISOString(), seq: 1, totals: { requests: 0, rerouted: 0, errors: 0 }, routes: [] });
    }
    if (url.pathname === "/api/gateway/history") return json({ cut: false, days: [{ day, requests: routes.length }], routes: url.searchParams.get("day") ? routes : [] });
    if (url.pathname === "/api/usage/requests") {
      return json({ period: url.searchParams.get("period"), rows, offset: 0, total: rows.length, ...M, unpriced: 0, errors: 0, by: { model: [M] }, bucket: "hour", series: [], agents: [{ id: "codex", name: "Codex", icon: "codex-color" }] });
    }
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/usage") return json({ calls: 0, errors: 0, input: 0, output: 0, reasoning: 0, unpriced: 0, cost: 0, bucket: "day", series: [], agents: [], models: [], path: "" });
    if (url.pathname === "/api/providers") return json({ providers: [], presets: [], excluded: [], models: [], gateway: { running: true, window: true } });
    if (url.pathname === "/api/sessions") return json({ sessions: [], dirs: [] });
    if (url.pathname === "/api/sessions/stats") return json({ from: "", to: "", days: [], agents: {} });
    if (url.pathname === "/api/groups") return json({ groups: [], models: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    return route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const L = {
  "zh-TW": { s: (n) => `${n} token/秒`, speed: "速度", note: /整輪 .* 輸出 2,200 個 token/, rule: /回覆寫的每個 token（含推理）都按整輪時間算/ },
  en: { s: (n) => `${n} tok/s`, speed: "Speed", note: /2,200 output tokens in .* s, the whole turn/, rule: /Every token a reply wrote, its reasoning among them/ },
  zh: { s: (n) => `${n} token/秒`, speed: "速度", note: /整轮 .* 输出 2,200 个 token/, rule: /回复写的每个 token（含推理）都按整轮时间算/ },
  ja: { s: (n) => `${n} トークン/秒`, speed: "速度", note: /ターン全体の .* で 2,200 トークンを出力/, rule: /推論も含めてすべてターン全体の時間で数えます/ },
  de: { s: (n) => `${n} Token/s`, speed: "Tempo", note: /2\.200 Ausgabe-Token in .* der ganze Turn/, rule: /Jedes von einer Antwort geschriebene Token, Reasoning eingeschlossen/ },
};

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of Object.keys(L)) {
    test(`${engine} ${lang}: a reply that reasoned is timed by its whole turn`, async (t) => {
      const w = L[lang];
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await browser.newPage({ viewport: { width: 1280, height: 900 }, reducedMotion: "reduce" });
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", serve(lang));

      // the Routing page: each row, and the session's average
      await page.goto("http://magpie.test/?view=routing");
      await page.locator(".rt-days .rt-day").nth(1).click();
      const reqs = page.locator(".rt-req");
      await reqs.nth(timing.length - 1).waitFor();
      const speedOf = async (i) => (await reqs.nth(i).locator(".speed").count()) ? (await reqs.nth(i).locator(".speed .v").textContent()).trim() : "";
      assert.equal(await speedOf(0), w.s(142), "the whole turn, not the 1,467 its answer's 200 in 1.5 s read");
      assert.equal(await speedOf(1), w.s(144), "an older route without its own count: timed as any other");
      assert.equal(await speedOf(2), w.s(167), "tool calls after reasoning are in the turn");
      assert.equal(await speedOf(3), w.s(71), "no reasoning: as before");
      assert.equal(await speedOf(4), w.s(100), "its content over 12 s of its 15 s");
      assert.equal(await speedOf(5), w.s(167), "a late summary is in the turn");
 assert.equal(await speedOf(6), w.s(13), "redacted reasoning is in the turn");
      assert.match(await reqs.nth(0).locator(".speed").getAttribute("title"), w.rule);
      await reqs.nth(0).click();
      await page.waitForTimeout(200);
      const story = await page.locator(".rt-steps").textContent();
      assert(story.includes(w.s(142)) && !/1[,.]?467/.test(story), "the story's speed: " + story);
      await page.locator(".rt-group-by button").nth(1).click();
      const group = page.locator("button.rt-session").first();
      await group.waitFor();
      // the session's average: its turns' tokens over their time, reasoning in it
      assert.equal((await group.locator(".speed .v").textContent()).trim(), w.s(117));
      assert.match(await group.locator(".speed").getAttribute("title"), w.rule);

      // the Usage page's requests: each row, its tooltip, and the period's
      await page.goto("http://magpie.test/");
      await page.locator('[data-view="usage"]').first().click();
      await page.locator(":is(#usageTab .opt, .nav-s):visible").nth(1).click();
      await page.locator("#ledWrap .led tbody tr").first().waitFor();
      await page.waitForTimeout(300);
      const heads = await page.locator(".led thead th").evaluateAll((ths) => ths.filter((th) => th.offsetParent).map((th) => th.textContent));
      const col = heads.indexOf(w.speed);
      assert(col > 0, "a Speed column: " + heads.join(", "));
      const cell = (i) => page.locator(".led tbody tr.led-row").nth(i).locator("td").filter({ visible: true }).nth(col);
      assert.equal((await cell(0).textContent()).trim(), w.s(142));
      assert.match(await cell(0).getAttribute("title"), w.note);
      assert.equal((await cell(1).textContent()).trim(), w.s(144));
      assert.equal((await cell(2).textContent()).trim(), w.s(167));
      assert.equal((await cell(3).textContent()).trim(), w.s(71));
      assert.equal((await cell(4).textContent()).trim(), w.s(100));
      assert.equal((await cell(5).textContent()).trim(), w.s(167));
 assert.equal((await cell(6).textContent()).trim(), w.s(13));
      const kpi = page.locator("#ledKpi .blk").nth(4);
      assert.equal((await kpi.locator(".v").textContent()).trim(), w.s(117));
      assert.match(await kpi.getAttribute("title") || await kpi.locator("[title]").first().getAttribute("title"), w.rule);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, "no sideways scroll");
      assert.deepEqual(errors, []);
    });
  }
}
