/* eslint-disable no-console */
'use strict';

/**
 * Unified Moxfield collector — single user with two tabs.
 *
 * One browser, two pages. A=explore-page paginator (collects share-link
 * batches via search-sfw response interception); B=deck-detail fetcher
 * (calls /v3/decks/all/{id} via page.evaluate(fetch) inside the same
 * browser session, so cookies + CF clearance are shared).
 *
 * One main loop alternates actions between the two tabs and dwells
 * between every action. The activity pattern looks like a single human
 * with two tabs open — not two parallel sessions. Dwells are roughly
 * doubled vs the previous pure-Phase-2 pace.
 *
 * Resumes from existing share-links.csv + details-state.json. Targets
 * 1000 unique full deck records and stops cleanly.
 */

const path = require('path');
const fs = require('fs');
const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');

puppeteer.use(StealthPlugin());

// ---------- config ----------
const ROOT = __dirname;
const OUTPUT_DIR = path.join(ROOT, 'output');
// Run 2 — separate file namespace so the prior 513-deck corpus is preserved.
const RUN_TAG = 'run2';
const CSV_PATH = path.join(OUTPUT_DIR, `share-links-${RUN_TAG}.csv`);
const STATE_PATH = path.join(OUTPUT_DIR, `state-${RUN_TAG}.json`);
const DETAILS_NDJSON = path.join(OUTPUT_DIR, `deck-details-${RUN_TAG}.ndjson`);
const ERRORS_NDJSON = path.join(OUTPUT_DIR, `deck-detail-errors-${RUN_TAG}.ndjson`);
const DETAILS_STATE_PATH = path.join(OUTPUT_DIR, `details-state-${RUN_TAG}.json`);

const TARGET_DETAILS = 1000;

const SLICES = [
  { fmt: 'commander', sort: 'updated',  target: 1500 },
  { fmt: 'commander', sort: 'likes',    target: 1200 },
  { fmt: 'commander', sort: 'viewCount',target: 1000 },
  { fmt: 'commander', sort: 'created',  target: 800 },
  { fmt: 'modern',    sort: 'updated',  target: 600 },
  { fmt: 'modern',    sort: 'likes',    target: 400 },
  { fmt: 'pioneer',   sort: 'updated',  target: 500 },
  { fmt: 'pioneer',   sort: 'likes',    target: 300 },
  { fmt: 'standard',  sort: 'updated',  target: 500 },
  { fmt: 'legacy',    sort: 'updated',  target: 400 },
  { fmt: 'legacy',    sort: 'likes',    target: 200 },
  { fmt: 'vintage',   sort: 'updated',  target: 250 },
  { fmt: 'pauper',    sort: 'updated',  target: 400 },
  { fmt: 'pauper',    sort: 'likes',    target: 200 },
  { fmt: 'historicbrawl', sort: 'updated', target: 400 },
  { fmt: 'brawl',     sort: 'updated',  target: 300 },
];

// ---------- randomness ----------
const rand = (a, b) => Math.random() * (b - a) + a;
const randInt = (a, b) => Math.floor(rand(a, b + 1));
const choice = (arr) => arr[Math.floor(Math.random() * arr.length)];
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Doubled dwell — list-tabbing pace, two-tab user.
function dwellMs() {
  const r = Math.random();
  if (r < 0.65) return randInt(700, 4000);    // 65%   0.7–4s     normal
  if (r < 0.90) return randInt(4000, 18000);  // 25%   4–18s      lingered
  if (r < 0.985) return randInt(18000, 70000);//  8.5% 18s–1m10s  paused
  if (r < 0.998) return randInt(70000, 240000);// 1.3% 1m10–4m    looked away
  return randInt(240000, 600000);              //  0.2% 4–10m    distracted
}

function microPauseMs() {
  const r = Math.random();
  if (r < 0.7) return randInt(150, 900);
  if (r < 0.95) return randInt(900, 2500);
  return randInt(2500, 8000);
}

function logTimestamp() {
  return new Date().toISOString().replace('T', ' ').slice(0, 19);
}
function log(...args) {
  console.log(`[${logTimestamp()}]`, ...args);
}

// ---------- state ----------
function loadState() {
  if (!fs.existsSync(STATE_PATH)) {
    return { seenIds: {}, sliceCursor: 0, perSliceCollected: {} };
  }
  return JSON.parse(fs.readFileSync(STATE_PATH, 'utf8'));
}
function saveState(state) {
  fs.writeFileSync(STATE_PATH, JSON.stringify(state, null, 2));
}
function loadDetailsState() {
  if (!fs.existsSync(DETAILS_STATE_PATH)) return { done: {}, errors: {} };
  return JSON.parse(fs.readFileSync(DETAILS_STATE_PATH, 'utf8'));
}
function saveDetailsState(s) {
  fs.writeFileSync(DETAILS_STATE_PATH, JSON.stringify(s, null, 2));
}

// ---------- csv ----------
const CSV_HEADER = 'publicId,publicUrl,name,format,bracket,autoBracket,colorIdentity,likeCount,viewCount,bookmarkCount,mainboardCount,authorUserName,createdAtUtc,lastUpdatedAtUtc,collectedAtUtc,sourceSlice\n';

function ensureCsvHeader() {
  if (!fs.existsSync(CSV_PATH)) fs.writeFileSync(CSV_PATH, CSV_HEADER);
}
function csvEscape(v) {
  if (v === null || v === undefined) return '';
  const s = String(v);
  if (s.includes(',') || s.includes('"') || s.includes('\n')) {
    return '"' + s.replace(/"/g, '""') + '"';
  }
  return s;
}
function appendCsvRow(d, sliceLabel) {
  const line = [
    csvEscape(d.publicId),
    csvEscape(d.publicUrl),
    csvEscape(d.name),
    csvEscape(d.format),
    csvEscape(d.bracket),
    csvEscape(d.autoBracket),
    csvEscape((d.colorIdentity || []).join('')),
    csvEscape(d.likeCount),
    csvEscape(d.viewCount),
    csvEscape(d.bookmarkCount),
    csvEscape(d.mainboardCount),
    csvEscape(d.authorUserName),
    csvEscape(d.createdAtUtc),
    csvEscape(d.lastUpdatedAtUtc),
    csvEscape(new Date().toISOString()),
    csvEscape(sliceLabel),
  ].join(',') + '\n';
  fs.appendFileSync(CSV_PATH, line);
}

// ---------- humanlike actions ----------
async function humanScroll(page, dir = 'down') {
  const ticks = randInt(2, 6);
  const sign = dir === 'down' ? 1 : -1;
  for (let i = 0; i < ticks; i++) {
    await page.mouse.wheel({ deltaY: sign * randInt(80, 380) });
    await sleep(randInt(50, 220));
  }
}
async function maybeHoverDeckCard(page) {
  if (Math.random() > 0.20) return;
  try {
    const handles = await page.$$('a[href^="/decks/"]:not([href*="/decks/public"]):not([href="/decks/following"]):not([href="/decks/liked"])');
    if (!handles.length) return;
    const target = choice(handles);
    const box = await target.boundingBox();
    if (!box) return;
    await page.mouse.move(box.x + rand(2, box.width - 2), box.y + rand(2, box.height - 2), { steps: randInt(6, 14) });
    await sleep(microPauseMs());
  } catch (_e) {}
}

// ---------- tab A: paginator ----------
function buildSliceUrl(s) {
  const p = new URLSearchParams();
  p.set('fmt', s.fmt);
  p.set('sortType', s.sort);
  p.set('sortDirection', 'descending');
  return `https://moxfield.com/decks/public?${p.toString()}`;
}

async function waitForResults(page, timeoutMs = 30000) {
  await page.waitForFunction(
    () => Array.from(document.querySelectorAll('a[href^="/decks/"]')).some((a) => /^\/decks\/[A-Za-z0-9_-]{15,}/.test(a.getAttribute('href') || '')),
    { timeout: timeoutMs }
  );
}

async function clickViewMore(page) {
  const handle = await page.evaluateHandle(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    return btns.find((b) => /view\s*more/i.test(b.textContent || '')) || null;
  });
  const el = handle.asElement();
  if (!el) return false;
  const ok = await el.evaluate((b) => {
    const r = b.getBoundingClientRect();
    return r.width > 0 && r.height > 0 && !b.disabled;
  });
  if (!ok) return false;
  const box = await el.boundingBox();
  if (box) {
    await page.mouse.move(box.x + rand(4, box.width - 4), box.y + rand(4, box.height - 4), { steps: randInt(8, 18) });
    await sleep(microPauseMs());
  }
  await el.click();
  return true;
}

function attachShareLinkInterceptor(page, ctx) {
  page.on('response', async (response) => {
    const url = response.url();
    if (!url.includes('/v2/decks/search-sfw')) return;
    if (response.status() !== 200) return;
    let json;
    try { json = await response.json(); } catch (_e) { return; }
    if (!Array.isArray(json.data)) return;

    let added = 0;
    for (const d of json.data) {
      const id = d.publicId;
      if (!id) continue;
      if (ctx.state.seenIds[id]) continue;
      ctx.state.seenIds[id] = ctx.currentSliceLabel;
      appendCsvRow(d, ctx.currentSliceLabel);
      // Only enqueue for detail fetch if not already done from a prior run
      if (!ctx.detailsState.done[id]) {
        ctx.detailsQueue.push(id);
      }
      added++;
    }
    if (added > 0) {
      ctx.lastAReturned = added;
      log(`  [A] +${added} share-links (queue=${ctx.detailsQueue.length})`);
    } else {
      ctx.lastAReturned = 0;
    }
  });
}

async function actionAStartSlice(tabA, ctx) {
  const slice = SLICES[ctx.state.sliceCursor];
  if (!slice) return false;
  ctx.currentSliceLabel = `${slice.fmt}-${slice.sort}`;
  const url = buildSliceUrl(slice);
  log(`[A] navigating to slice ${ctx.currentSliceLabel} (cursor=${ctx.state.sliceCursor})`);
  try {
    await tabA.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 });
  } catch (e) {
    log(`[A] navigate threw: ${e.message} — interceptor may still have caught the response`);
  }
  // Don't gate success on DOM render — the interceptor is the source of truth
  // for "did this slice produce share-links". Try to wait for View More so
  // subsequent A_more clicks work, but tolerate the timeout.
  try {
    await waitForResults(tabA, 30000);
  } catch (_e) {
    log(`[A] DOM cards didn't render in 30s — that's ok, we have network data`);
  }
  return true;
}

async function actionAClickMore(tabA, ctx) {
  await humanScroll(tabA, 'down');
  await sleep(microPauseMs());
  await maybeHoverDeckCard(tabA);
  const clicked = await clickViewMore(tabA);
  if (!clicked) {
    log(`[A] no more View More on ${ctx.currentSliceLabel}, advancing slice`);
    ctx.state.sliceCursor++;
    ctx.aSliceExhausted = true;
    return false;
  }
  // Wait briefly for the response interceptor to land
  await sleep(randInt(800, 2200));
  return true;
}

// ---------- tab B: deck-detail fetcher ----------
async function fetchDeckDetailFromTabB(tabB, publicId) {
  return tabB.evaluate(async (id) => {
    const url = `https://api2.moxfield.com/v3/decks/all/${id}`;
    const r = await fetch(url, { credentials: 'include', headers: { Accept: 'application/json' } });
    return {
      status: r.status,
      ok: r.ok,
      body: r.ok ? await r.json() : (await r.text()).slice(0, 500),
    };
  }, publicId);
}

async function actionBFetchOne(tabB, ctx) {
  if (ctx.detailsQueue.length === 0) return null;
  const id = ctx.detailsQueue.shift();
  if (ctx.detailsState.done[id]) return 'skipped';

  let result;
  try {
    result = await fetchDeckDetailFromTabB(tabB, id);
  } catch (e) {
    log(`  [B] ${id} threw: ${e.message}`);
    fs.appendFileSync(ERRORS_NDJSON, JSON.stringify({ publicId: id, error: e.message, at: new Date().toISOString() }) + '\n');
    return 'error';
  }

  if (result.ok) {
    fs.appendFileSync(DETAILS_NDJSON, JSON.stringify({ publicId: id, fetchedAtUtc: new Date().toISOString(), data: result.body }) + '\n');
    ctx.detailsState.done[id] = { fetchedAtUtc: new Date().toISOString(), status: 200 };
    ctx.detailsCount = Object.keys(ctx.detailsState.done).length;
    if (ctx.detailsCount % 10 === 0) saveDetailsState(ctx.detailsState);
    return 'ok';
  }

  fs.appendFileSync(ERRORS_NDJSON, JSON.stringify({ publicId: id, status: result.status, body: result.body, at: new Date().toISOString() }) + '\n');
  if (result.status === 404) {
    ctx.detailsState.done[id] = { fetchedAtUtc: new Date().toISOString(), status: 404 };
  }
  if (result.status === 403) ctx.consecutive403 = (ctx.consecutive403 || 0) + 1;
  else ctx.consecutive403 = 0;
  if (result.status === 429) {
    log('!! 429 — backing off 3–6 min');
    await sleep(randInt(180000, 360000));
  }
  return 'error';
}

// ---------- action picker ----------
function pickAction(ctx) {
  const queueLen = ctx.detailsQueue.length;
  const sliceExhausted = ctx.aSliceExhausted;

  // If A is in a fresh slice with no results yet, kick it
  if (!ctx.aInitialized) return 'A_start_slice';

  // If A's slice exhausted, advance to next
  if (sliceExhausted) return 'A_start_slice';

  // If queue is empty, must do A to refill
  if (queueLen === 0) return 'A_more';

  // If queue is small and we're below target, occasionally refill
  if (queueLen < 30 && ctx.detailsCount < TARGET_DETAILS) {
    if (Math.random() < 0.5) return 'A_more';
  }

  // Default: drain the queue (B is the bottleneck for deck details)
  // ~85% B, 15% A to keep CSV trickling
  if (Math.random() < 0.15) return 'A_more';
  return 'B_fetch';
}

// ---------- main ----------
async function main() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  ensureCsvHeader();

  const state = loadState();
  const detailsState = loadDetailsState();
  const detailsCount = Object.keys(detailsState.done).length;

  // Build the initial detailsQueue from CSV minus already-done.
  // (CSV may have IDs from prior runs that haven't been fetched yet.)
  const seenInQueue = new Set();
  const detailsQueue = [];
  if (fs.existsSync(CSV_PATH)) {
    const lines = fs.readFileSync(CSV_PATH, 'utf8').split('\n');
    for (const line of lines) {
      if (!line || line.startsWith('publicId,')) continue;
      const id = line.split(',')[0].trim();
      if (!id || seenInQueue.has(id)) continue;
      seenInQueue.add(id);
      if (!detailsState.done[id]) detailsQueue.push(id);
    }
  }
  log(`resume: ${Object.keys(state.seenIds).length} share-links banked, ${detailsCount} details done, ${detailsQueue.length} in queue, target ${TARGET_DETAILS}`);

  if (detailsCount >= TARGET_DETAILS) {
    log('target already met. exiting.');
    return;
  }

  log('launching headed browser');
  const browser = await puppeteer.launch({
    headless: false,
    defaultViewport: null,
    args: [
      '--window-size=1440,960',
      '--disable-blink-features=AutomationControlled',
      '--lang=en-US',
    ],
  });
  const pages = await browser.pages();
  const tabA = pages[0] || (await browser.newPage());
  const tabB = await browser.newPage();
  for (const p of [tabA, tabB]) {
    await p.setExtraHTTPHeaders({ 'Accept-Language': 'en-US,en;q=0.9' });
  }

  const ctx = {
    state, detailsState, detailsQueue,
    detailsCount,
    currentSliceLabel: '',
    aInitialized: false,
    aSliceExhausted: false,
    lastAReturned: -1,
    consecutive403: 0,
  };
  attachShareLinkInterceptor(tabA, ctx);

  // Moxfield's analytics/ad pings keep the page from ever reaching networkidle2,
  // so use domcontentloaded + a manual settle, and don't let a goto timeout kill the run.
  async function safeGoto(page, url, settle = [2500, 6000]) {
    try {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 });
    } catch (e) {
      log(`  goto warning ${url}: ${e.message} — continuing`);
    }
    await sleep(randInt(settle[0], settle[1]));
  }

  log('warming up tabA on homepage');
  await safeGoto(tabA, 'https://moxfield.com/');
  log('warming up tabB on /decks/public');
  await safeGoto(tabB, 'https://moxfield.com/decks/public');

  // Main loop
  while (ctx.detailsCount < TARGET_DETAILS) {
    const action = pickAction(ctx);

    try {
      if (action === 'A_start_slice') {
        if (ctx.state.sliceCursor >= SLICES.length) {
          log('all slices exhausted, no more share-links to add');
          if (ctx.detailsQueue.length === 0) {
            log('queue also empty, stopping');
            break;
          }
          // queue still has items: keep draining via B
          ctx.aInitialized = true;
          ctx.aSliceExhausted = false;
        } else {
          const ok = await actionAStartSlice(tabA, ctx);
          if (ok) {
            ctx.aInitialized = true;
            ctx.aSliceExhausted = false;
          }
        }
      } else if (action === 'A_more') {
        if (!ctx.aInitialized || ctx.aSliceExhausted) {
          // can't click more on a non-initialized slice
          ctx.aInitialized = false;
        } else {
          await actionAClickMore(tabA, ctx);
        }
      } else if (action === 'B_fetch') {
        const r = await actionBFetchOne(tabB, ctx);
        if (r === 'ok') {
          if (ctx.detailsCount % 25 === 0) {
            log(`  [B] details progress: ${ctx.detailsCount}/${TARGET_DETAILS}`);
          }
        }
      }
    } catch (err) {
      log(`!! action ${action} threw: ${err.message}`);
      await sleep(randInt(20000, 60000));
    }

    saveState(ctx.state);

    if (ctx.consecutive403 >= 5) {
      log(`!! ${ctx.consecutive403} consecutive 403s — pausing 5–10 min and reloading tabB`);
      await sleep(randInt(300000, 600000));
      try {
        await tabB.goto('https://moxfield.com/decks/public', { waitUntil: 'domcontentloaded', timeout: 60000 });
        await sleep(randInt(3000, 8000));
      } catch (_e) {}
      ctx.consecutive403 = 0;
    }

    const wait = dwellMs();
    log(`  action=${action} count=${ctx.detailsCount}/${TARGET_DETAILS} q=${ctx.detailsQueue.length} dwell=${(wait/1000).toFixed(1)}s`);
    await sleep(wait);
  }

  saveDetailsState(ctx.detailsState);
  saveState(ctx.state);

  log(`done. details=${ctx.detailsCount} share-links=${Object.keys(ctx.state.seenIds).length}`);
  log(`csv: ${CSV_PATH}`);
  log(`ndjson: ${DETAILS_NDJSON}`);
  log('leaving browser open. close when ready.');
}

main().catch((e) => {
  console.error('fatal:', e);
  process.exit(1);
});
