// Run with Node's test runner and Playwright on the module path; see README.md.
// The speed of a call read from a session file, on the Usage page's Requests
// tab. Claude Code's session file has no first token, only when the line that
// asked for a reply and the reply's last line were written, so its rows used
// to tell no speed at all, and each said "Not streamed", though the replies
// were streamed. A reply's speed is its whole turn now (5989b1af): every
// output token it wrote over the ms from the request to its last, so a
// session file's row tells the speed of its whole turn ("98 tok/s") where it
// used to tell the least it could have written at ("≥98 tok/s"). A call
// magpie timed keeps its own speed, and a Codex call in a turn of several is
// still timed from what asked for it, before its first output item, to its
// last (Fallmonkey on Discord). A dash left says why: failed, or a turn too
// fast to tell a speed by. A narrow window shows it as a wide one does.
//
// The rows are real ledger rows read from Claude Code's session files of
// claude-opus-5-5 replies; the first, 2,558 tokens in 25,987 ms, measured
// 100.6 tok/s on Claude Code's own stream (--include-partial-messages) from
// its first thinking event to its end.
// English and Chinese; no backend, the API is faked.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const reader = require("./reader.cjs");

const assets = path.resolve(__dirname, "../assets");
const now = Date.now();
const at = (i) => new Date(now - (i + 1) * 60e3).toISOString();

const LOG = {
  agent: "claude", provider: "session-unknown", model: "claude-opus-5-5", req: "claude-opus-5-5", served: "claude-opus-5-5",
  in: 2, cache_read: 12380, cache_write: 20122, effort: "medium", status: 0, session: "57956f49-8830-4769-a6c3-93ff87b1fe16",
  cost: 0, priced: false, source: "log", agentName: "Claude Code", icon: "claudecode-color", providerName: "Local session",
};
const GATEWAY = {
  route_id: 900, agent: "claude", agentName: "Claude Code", icon: "claudecode-color", provider: "zai", providerName: "Z.ai Coding Plan",
  host: "me@example.com", req: "claude-opus-4-1-20250805", model: "glm-fast", served: "glm-fast", in: 1200, cache_read: 0, cache_write: 0,
  status: 200, rid: "req_900", ep: "/v1/messages", cost: 0.01, priced: true,
};
const ROWS = [
  // 0: a long reply from a session file: at least 2558 / 25.987 s
  { ...LOG, t: at(0), out: 2558, ms: 25987 },
  // 1: a short one: its tokens over the wait tell too little
  { ...LOG, t: at(1), out: 462, ms: 5340 },
  // 2: a call magpie timed keeps its own speed: 135 tokens in 1.8 s after its first
  { ...GATEWAY, t: at(2), out: 135, ms: 2400, ttft_ms: 600 },
  // 3: a gateway call not streamed is still told so, with the least it
  // wrote at over its whole time: 1500 tokens in 3 s (夜松云 on Discord)
  { ...GATEWAY, route_id: 901, rid: "req_901", t: at(3), out: 1500, ms: 3000 },
  // 4: a session file that timed its turn (Codex's, one call) tells its own: 1200 tokens in 8 s
  { ...LOG, agent: "codex", agentName: "Codex", icon: "codex-color", model: "gpt-6-astra", t: at(4), out: 1200, ms: 10000, ttft_ms: 2000 },
  // 5: a failed call tells none
  { ...LOG, t: at(5), out: 1500, ms: 9000, err_type: "overloaded_error", err: "Overloaded" },
  // 6: a long reply in under a second (lines written together) tells none
  { ...LOG, t: at(6), out: 5000, ms: 300 },
  // 7: a Codex call in a turn of several, timed from what asked for it: at
  // least 1500 tokens in 60 s
  { ...LOG, agent: "codex", agentName: "Codex", icon: "codex-color", model: "gpt-6-astra", t: at(7), out: 1500, ms: 60000 },
  // 8: Claude Desktop's Code tab writes Claude Code's file: at least 1500 / 12 s
  { ...LOG, agent: "claude-desktop", agentName: "Claude Desktop", icon: "claude-color", t: at(8), out: 1500, ms: 12000 },
];

function ledger(period) {
  return {
    period, rows: ROWS, offset: 0, total: ROWS.length, calls: ROWS.length, errors: 1,
    input: 1, output: 12355, cache_read: 0, cache_write: 0, cost: 0.02, unpriced: 5,
    // only what magpie timed: 135 tokens in 1.8 s and 1200 in 8 s
    timed: 2, ttft_ms: 2600, decode_ms: 9800, decode_out: 1335,
    by: { model: [] }, bucket: "hour", series: [],
    agents: [{ id: "claude", name: "Claude Code", icon: "claudecode-color" }, { id: "codex", name: "Codex", icon: "codex-color" }, { id: "claude-desktop", name: "Claude Desktop", icon: "claude-color" }],
  };
}

const STATE = (lang) => ({
  agents: [
    { id: "claude", name: "Claude Code", path: "/t/c.json", icon: "claudecode-color", wired: true, fields: [] },
    { id: "codex", name: "Codex", path: "/t/c.toml", icon: "codex-color", wired: true, fields: [] },
  ],
  profiles: [], settings: { lang, theme: "light" }, fx: { rate: 7.2, at: new Date().toISOString() },
});

function server(lang) {
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(STATE(lang));
    if (url.pathname === "/api/usage/requests") return json(ledger(url.searchParams.get("period")));
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/usage") return json({ calls: 0, errors: 0, input: 0, output: 0, reasoning: 0, unpriced: 0, cost: 0, bucket: "day", series: [], agents: [], models: [], path: "" });
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

// A reply's speed is its whole turn now (5989b1af): every output token it
// wrote over the ms from the request to its last, the wait for the first
// token in it. So a session file's long reply, which used to tell the least
// it wrote at, tells its own speed, and a gateway call never streamed tells
// its own too rather than a floor.
const L = {
  en: {
    speed: "Speed",
    whole: "98 tok/s", short: "87 tok/s", timed: "56 tok/s", notStreamed: "500 tok/s",
    codex: "120 tok/s", codexTurn: "25 tok/s", desktop: "125 tok/s",
    wholeTip: "2,558 output tokens in 26 s, the whole turn",
    shortTip: "462 output tokens in 5.3 s, the whole turn",
    notStreamedTip: "1,500 output tokens in 3.0 s, the whole turn",
    failed: "A failed request tells no speed",
    fast: "A turn under 100 ms came too fast to tell a speed by",
  },
  zh: {
    speed: "速度",
    whole: "98 token/秒", short: "87 token/秒", timed: "56 token/秒", notStreamed: "500 token/秒",
    codex: "120 token/秒", codexTurn: "25 token/秒", desktop: "125 token/秒",
    wholeTip: "整轮 26 秒 输出 2,558 个 token",
    shortTip: "整轮 5.3 秒 输出 462 个 token",
    notStreamedTip: "整轮 3.0 秒 输出 1,500 个 token",
    failed: "失败的请求没有速度",
    fast: "不到 100 毫秒的整轮太快，测不出速度",
  },
};

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(engine + ": the speed of a call read from a session file", async (t) => {
    assert(["chromium", "webkit"].includes(engine), "BROWSER must be chromium or webkit");
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    t.after(() => browser.close());
    // a wide window, and one narrow enough that the table scrolls sideways
    for (const [lang, width] of [["en", 1400], ["zh", 1400], ["en", 560], ["zh", 560]]) {
      const w = L[lang];
      await t.test(lang + " " + width, async () => {
        const errors = [];
        const ctx = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
        const p = await ctx.newPage();
        p.setDefaultTimeout(5000);
        p.on("pageerror", (e) => errors.push(e.message));
        await p.route("**/*", server(lang));
        await p.goto("http://magpie.test/");
        await p.locator('[data-view="usage"]').first().click();
        await p.locator(":is(#usageTab .opt, .nav-s):visible").nth(1).click();
        await p.locator("#ledWrap .led tbody tr").first().waitFor();

        const heads = await p.locator(".led thead th").evaluateAll((ths) => ths.filter((th) => th.offsetParent).map((th) => th.textContent));
        assert(heads.includes(w.speed), "a Speed column: " + heads.join(", "));
        const cell = (i) => p.locator(".led tbody tr.led-row").nth(i).locator("td.speed");
        const text = async (i) => (await cell(i).textContent()).trim();
        const tip = (i) => cell(i).getAttribute("title");

        // a long reply from a session file: its whole turn, and what it was
        assert.equal(await text(0), w.whole);
        assert((await tip(0)).startsWith(w.wholeTip), await tip(0));
        // a short one tells a speed too: 462 tokens in 5.3 s
        assert.equal(await text(1), w.short);
        assert((await tip(1)).startsWith(w.shortTip), await tip(1));
        // what magpie timed is as it was
        assert.equal(await text(2), w.timed);
        // a gateway call never streamed is timed over its whole turn
        assert.equal(await text(3), w.notStreamed);
        assert((await tip(3)).startsWith(w.notStreamedTip), await tip(3));
        // a Codex turn that reasoned first counts it too
        assert.equal(await text(4), w.codex);
        // a failed call, and a turn under 100 ms, tell none, and say why
        assert.equal(await text(5), "—");
        assert.equal(await tip(5), w.failed);
        assert.equal(await text(6), "—");
        assert.equal(await tip(6), w.fast);
        // a Codex call of one, timed only over a second or more and 100 tokens or more
        assert.equal(await text(7), w.codexTurn);
        // Claude Desktop's file is Claude Code's
        assert.equal(await text(8), w.desktop);
        // the speed is read whole, at either width
        const clipped = await cell(0).evaluate((td) => td.scrollWidth > td.clientWidth + 1);
        assert(!clipped, "the speed fits its cell");

        // the row's details say it too
        // the first cell, not the time's link: at a narrow window the row is brought into sight first
        await reader.click(p, p.locator(".led tbody tr.led-row").first().locator("td").nth(1));
        const detail = p.locator(".led tbody tr.led-detail").first();
        await detail.waitFor();
        const pairs = await detail.locator("dt").evaluateAll((dts) => dts.map((dt) => [dt.textContent, dt.nextElementSibling?.textContent]));
        assert.deepEqual(pairs.find(([k]) => k === w.speed), [w.speed, w.whole], JSON.stringify(pairs));

        assert.deepEqual(errors, []);
        await ctx.close();
      });
    }
  });
}
