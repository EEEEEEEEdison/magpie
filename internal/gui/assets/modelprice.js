// Price controls belong to the provider editor, loaded when Names & levels opens.
window.modelPrice = function modelPrice(row, m, prefs, pref, priceNow, drawReset, draft) {
  const id = m.id;
  // its cost to the user, $ per million tokens, as Usage counts it
  // (#819): a box empty is its list price. The 1-hour cache write,
  // empty, is 2× input as Anthropic bills it, and only a Claude model
  // (or one priced with one) has it (PAMI on Discord). A long-context
  // price (OpenAI's over 272K) is the whole request's when its input
  // is over that size, empty parts its list's
  const parts = [["input", "Input"], ["output", "Output"], ["cache_read", "Cache read"], ["cache_write", "Cache write"], ["cache_write_1h", "Cache write 1h"]];
  const tierParts = parts.slice(0, 4);
  const shown = (n) => String(Math.round(n * 1e6) / 1e6);
  const listTier = m.list?.tiers?.[0];
  const priceBox = el("div", "mprice");
  // a Remote magpie's model is listed at what that magpie counts it at
  priceBox.title = m.remoteList
    ? t("What {id} costs, in US dollars per million tokens, as the Usage page counts it; empty: what the other magpie counts it at, shown greyed. A price set here isn't multiplied by the provider's price rate", { id: m.id })
    : t("What {id} costs, in US dollars per million tokens, as the Usage page counts it; empty: its list price, shown greyed. A price set here isn't multiplied by the provider's price rate", { id: m.id });
  if (m.list?.peak == "ollama") priceBox.title += "\n" + t("Off-peak price, the list's or one set here: the Usage page doubles it from 12:00 to 18:00 UTC, Monday to Friday");
  else if (m.list?.peak) priceBox.title += "\n" + t("Off-peak price, the list's or one set here: the Usage page doubles it from 01:00 to 04:00 and 06:00 to 10:00 UTC, Monday to Friday{x}", { x: m.list.peak == "deepseek" ? t(", except on Chinese public holidays") : "" });
  priceBox.append(el("span", "", t("Price, $ / 1M tokens")));
  const typed = draft.priceTyped?.[id];
  const cell = (value, placeholder, k, label) => {
    const i = input(value, placeholder);
    i.defaultValue = i.value;
    i.inputMode = "decimal";
    i.dataset.part = k;
    i.setAttribute("aria-label", label);
    blurOnEnter(i);
    i.onchange = () => takePrice();
    return i;
  };
  const oneHour = (pr) => pr ? pr.cache_write_1h || 2 * pr.input : 0;
  const cells = parts.map(([k, l], n) => {
    const box = el("label", "mpart");
    const pr = priceNow();
    const value = typed ? typed[n] : !pr ? "" : k === "cache_write_1h" ? pr.cache_write_1h ? shown(pr.cache_write_1h) : "" : shown(pr[k]);
    const placeholder = !m.list ? "" : k === "cache_write_1h" ? shown(oneHour(m.list)) : shown(m.list[k]);
    const i = cell(value, placeholder, k, t(l));
    if (k === "cache_write_1h") {
      box.title = t("A cache write kept for an hour, as Anthropic bills it; empty: 2× input");
      box.hidden = !/claude/i.test(m.id) && !m.list?.cache_write_1h && !pr?.cache_write_1h && !(typed && typed[n]);
    }
    box.append(el("span", "", t(l)), i);
    priceBox.append(box);
    return i;
  });
  // the long-context price, a quiet row of its own: shown where there is
  // one, else behind a link
  const tierRow = el("div", "mtier");
  tierRow.title = t("What the whole request costs when its input — cached tokens and cache writes included — is over this many tokens, as OpenAI bills gpt-6-astra over 272K; empty: the list's");
  const tierNow = () => priceNow()?.tiers?.[0];
  const above = cell(typed ? typed[parts.length] : tierNow() ? tokenSize(tierNow().above) : "", listTier ? tokenSize(listTier.above) : "272K", "above", t("Over input tokens"));
  above.classList.add("mabove");
  const overBox = el("label", "mpart");
  overBox.append(el("span", "", t("Long-context price, over")), above);
  tierRow.append(overBox);
  const tierCells = tierParts.map(([k, l], n) => {
    const box = el("label", "mpart");
    const i = cell(typed ? typed[parts.length + 1 + n] : tierNow() ? shown(tierNow()[k]) : "", listTier ? shown(listTier[k]) : "", "tier_" + k, t("Long-context price") + " · " + t(l));
    box.append(el("span", "", t(l)), i);
    tierRow.append(box);
    return i;
  });
  // the days it holds on, "2006-01-02": a surcharge the vendor charges
  // only for a while (OpenAI's long-context price from the day it began
  // to the day it ended) is billed within them, its price above outside
  const tierDay = (k, label, ph) => {
    const i = input(tierNow()?.[k] || "", ph);
    i.defaultValue = i.value;
    i.dataset.part = "tier_" + k;
    i.setAttribute("aria-label", label);
    i.spellcheck = false;
    i.onkeydown = (e) => { e.stopPropagation(); if (e.key === "Enter") i.blur(); };
    i.onchange = () => takePrice();
    return i;
  };
  const fromDay = tierDay("from", t("Long-context price from"), "");
  const toDay = tierDay("to", t("Long-context price to"), "");
  const fromBox = el("label", "mpart");
  fromBox.append(el("span", "", t("Starts on")), fromDay);
  const toBox = el("label", "mpart");
  toBox.append(el("span", "", t("Ends on")), toDay);
  tierRow.append(fromBox, toBox);
  // the model's own answer: none at all, whatever its list says. A
  // vendor that stopped charging one is not billed it any more, and a
  // relay that never charged it is not billed its model's.
  const offTier = el("button", "text action mtier-off", t("No long-context price"));
  offTier.title = t("The model has no long-context price: a request over any size is billed at the price above, whatever its list says");
  offTier.onclick = (e) => { e.preventDefault(); tierOpen = false; takePrice("off"); };
  tierRow.append(offTier);
  const noTierNow = () => !!(prefs[id]?.price ? prefs[id].price.no_tiers : m.price?.no_tiers);
  const moreTier = el("button", "text action mtier-add", t("Long-context price"));
  moreTier.title = tierRow.title;
  let tierOpen = false;
  moreTier.onclick = (e) => {
    e.preventDefault();
    tierOpen = true;
    if (noTierNow()) takePrice(); // on again: the list's, until one is typed
    else drawTier();
    above.focus({ preventScroll: true });
  };
  const drawTier = () => {
    const off = noTierNow();
    tierRow.hidden = off || (!tierOpen && !tierNow() && !listTier);
    moreTier.hidden = !tierRow.hidden;
    moreTier.textContent = off ? t("Long-context price: none") : t("Long-context price");
  };
  priceBox.append(moreTier, tierRow);
  drawTier();
  const showPrice = () => {
    const pr = priceNow();
    cells.forEach((c, i) => { const k = parts[i][0]; c.value = !pr ? "" : k === "cache_write_1h" ? pr.cache_write_1h ? shown(pr.cache_write_1h) : "" : shown(pr[k]); });
    const tr = tierNow();
    above.value = tr ? tokenSize(tr.above) : "";
    fromDay.value = tr?.from || "";
    toDay.value = tr?.to || "";
    tierCells.forEach((c, i) => { c.value = tr ? shown(tr[tierParts[i][0]]) : ""; });
    drawTier();
  };
  const takePrice = (mode) => {
    const off = mode === "off";
    const v = cells.map((c) => c.value.trim());
    const tv = [above, ...tierCells].map((c) => c.value.trim());
    const dates = [fromDay, toDay].map((c) => c.value.trim());
    const x = pref();
    delete x.price; delete x.ownPrice;
    if (draft.priceTyped) delete draft.priceTyped[id];
    if (!off && v.every((s) => s === "") && tv.every((s) => s === "") && dates.every((s) => s === "")) {
      if (m.price) x.ownPrice = true;
      drawReset();
      return;
    }
    const bad = (msg) => { status(msg, "err"); showPrice(); drawReset(); };
    const price = {};
    for (const [i, [k]] of tierParts.entries()) {
      const n = v[i] === "" ? m.list?.[k] ?? (k.startsWith("cache") ? 0 : NaN) : decimalOf(v[i]);
      // a model with no list price is given its input and output one
      // box at a time: what is typed stays, waiting for the other, and
      // the Save asks for it (PAMI on Discord: each box typed was
      // emptied again, so a price could never be set)
      if (Number.isNaN(n) && v[i] === "" && v.every((s) => s === "" || decimalOf(s) >= 0)) {
        (draft.priceTyped = draft.priceTyped || {})[id] = [...v, ...tv];
        status(t("{id} has no list price: give its input and output prices", { id: m.id }), "warn");
        drawReset();
        return;
      }
      if (!Number.isFinite(n) || n < 0) return bad(Number.isNaN(n) && v[i] === "" ? t("{id} has no list price: give its input and output prices", { id: m.id }) : t("A price is a number of dollars, 0 or more"));
      price[k] = n;
    }
    if (v[4] !== "") {
      const n = decimalOf(v[4]);
      if (!(n >= 0)) return bad(t("A price is a number of dollars, 0 or more"));
      if (n > 0) price.cache_write_1h = n;
    }
    // the long-context price: its parts typed, else the list's, else the
    // price's own; none typed, the list's as it is. Off is the model's
    // own word that it has none.
    const from = tierNow() || listTier;
    if (off) {
      price.no_tiers = true;
    } else if (tv.some((s) => s !== "") || dates.some((s) => s !== "")) {
      const day = (s) => s === "" || /^\d{4}-\d{2}-\d{2}$/.test(s) && !Number.isNaN(Date.parse(s));
      if (!day(dates[0]) || !day(dates[1])) return bad(t("A day is written YYYY-MM-DD, like 2026-09-25"));
      // a surcharge that ends before it begins is no window at all; the
      // days compare as written, a "2006-01-02" day sorting by itself
      if (dates[0] && dates[1] && dates[0] > dates[1]) return bad(t("A long-context price starts on or before the day it ends"));
      const size = tv[0] === "" ? from?.above : tokensTyped(tv[0]);
      if (!(size > 0)) return bad(t("A long-context price starts over a number of input tokens, like 272K"));
      const tier = { above: size };
      for (const [i, [k]] of tierParts.entries()) {
        const n = tv[i + 1] === "" ? from?.[k] ?? price[k] : decimalOf(tv[i + 1]);
        if (!(n >= 0)) return bad(t("A price is a number of dollars, 0 or more"));
        tier[k] = n;
      }
      if (dates[0]) tier.from = dates[0];
      if (dates[1]) tier.to = dates[1];
      price.tiers = [tier, ...((priceNow() || m.list)?.tiers || []).slice(1).filter((x) => x.above > size)];
    } else if (m.list?.tiers?.length) price.tiers = m.list.tiers;
    if (!m.price || !samePrice(m.price, price)) x.price = price;
    showPrice();
    drawReset();
  };
  row.append(priceBox);
  return showPrice;
};
