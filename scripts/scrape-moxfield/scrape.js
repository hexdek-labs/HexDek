/* eslint-disable no-console */
'use strict';

/**
 * Moxfield public-deck collector — list-browsing pace.
 *
 * Single tab, headed, stealth. Navigates filtered explore-page URLs and
 * clicks "View More" with randomized human-like timing. Instead of
 * scraping deck cards out of the DOM, this version intercepts the page's
 * own search-sfw API responses (which the page fetches naturally when
 * View More is clicked) and persists their structured payload to CSV.
 *
 * Operator: you sit and watch it run. Ctrl-C any time; resume via
 * state.json on next start.
 */

const path = require('path');
const fs = require('fs');
const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');

puppeteer.use(StealthPlugin());

// ---------- config ----------
const ROOT = __dirname;
const OUTPUT_DIR = path.join(ROOT, 'output');
const CSV_PATH = path.join(OUTPUT_DIR, 'share-links.csv');
const STATE_PATH = path.join(OUTPUT_DIR, 'state.json');

const TARGET_TOTAL = 2000;

// Slices: each is one explore-page filter combination we'll page through.
// The bracket= URL param is a no-op on Moxfield's API, so diversity comes
// from format × sort variation. Bracket distribution falls out of the data.
const SLICES = [
  // Commander, varied sorts (most populous format → biggest budget)
  { fmt: 'commander', sort: 'updated',  target: 1500 },
  { fmt: 'commander', sort: 'likes',    target: 1200 }, // top-likes is where cEDH lives
  { fmt: 'commander', sort: 'viewCount',target: 1000 },
  { fmt: 'commander', sort: 'created',  target: 800 },
  // Constructed formats
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
// Sum target ≈ 8950 (dedup headroom on top of cap)

// ---------- randomness helpers ----------
const rand = (min, max) => Math.random() * (max - min) + min;
const randInt = (min, max) => Math.floor(rand(min, max + 1));
const choice = (arr) => arr[Math.floor(Math.random() * arr.length)];

// List-browsing dwell — faster than deck-reading. You're skimming a grid.
function dwellMs() {
  const r = Math.random();
  if (r < 0.60) return randInt(1500, 6000);    // 60%   1.5–6s    quick browse
  if (r < 0.85) return randInt(6000, 25000);   // 25%   6–25s     lingered on a card
  if (r < 0.97) return randInt(25000, 90000);  // 12%   25–90s    paused / context switch
  if (r < 0.995) return randInt(90000, 240000);// 2.5%  1.5–4min  looked away
  return randInt(240000, 600000);              // 0.5%  4–10min   distracted
}

function microPauseMs() {
  const r = Math.random();
  if (r < 0.7)  return randInt(120, 600);
  if (r < 0.95) return randInt(600, 1800);
  return randInt(1800, 5000);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

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

const CSV_HEADER = 'publicId,publicUrl,name,format,bracket,autoBracket,colorIdentity,likeCount,viewCount,bookmarkCount,mainboardCount,authorUserName,createdAtUtc,lastUpdatedAtUtc,collectedAtUtc,sourceSlice\n';

function ensureCsvHeader() {
  if (!fs.existsSync(CSV_PATH)) {
    fs.writeFileSync(CSV_PATH, CSV_HEADER);
  }
}

function csvEscape(value) {
  if (value === null || value === undefined) return '';
  const s = String(value);
  if (s.includes(',') || s.includes('"') || s.includes('\n')) {
    return '"' + s.replace(/"/g, '""') + '"';
  }
  return s;
}

function appendCsvRow(row) {
  // Synchronous append → no race, no duplicate headers.
  const line = [
    csvEscape(row.publicId),
    csvEscape(row.publicUrl),
    csvEscape(row.name),
    csvEscape(row.format),
    csvEscape(row.bracket),
    csvEscape(row.autoBracket),
    csvEscape((row.colorIdentity || []).join('')),
    csvEscape(row.likeCount),
    csvEscape(row.viewCount),
    csvEscape(row.bookmarkCount),
    csvEscape(row.mainboardCount),
    csvEscape(row.authorUserName),
    csvEscape(row.createdAtUtc),
    csvEscape(row.lastUpdatedAtUtc),
    csvEscape(new Date().toISOString()),
    csvEscape(row.sourceSlice),
  ].join(',') + '\n';
  fs.appendFileSync(CSV_PATH, line);
}

// ---------- humanlike browser actions ----------
async function humanScroll(page, direction = 'down') {
  const ticks = randInt(2, 6);
  const sign = direction === 'down' ? 1 : -1;
  for (let i = 0; i < ticks; i++) {
    const dy = sign * randInt(80, 380);
    await page.mouse.wheel({ deltaY: dy });
    await sleep(randInt(50, 200));
  }
}

async function maybeHoverDeckCard(page) {
  if (Math.random() > 0.25) return;
  try {
    const handles = await page.$$('a[href^="/decks/"]:not([href*="/decks/public"]):not([href="/decks/following"]):not([href="/decks/liked"])');
    if (!handles.length) return;
    const target = choice(handles);
    const box = await target.boundingBox();
    if (!box) return;
    await page.mouse.move(
      box.x + rand(2, box.width - 2),
      box.y + rand(2, box.height - 2),
      { steps: randInt(6, 14) }
    );
    await sleep(microPauseMs());
  } catch (_e) {}
}

async function maybeScrollUpAndDown(page) {
  if (Math.random() > 0.10) return;
  await humanScroll(page, 'up');
  await sleep(microPauseMs());
  await humanScroll(page, 'down');
}

// ---------- page interaction ----------
function buildSliceUrl(slice) {
  const params = new URLSearchParams();
  if (slice.fmt) params.set('fmt', slice.fmt);
  if (slice.sort) {
    params.set('sortType', slice.sort);
    params.set('sortDirection', 'descending');
  }
  return `https://moxfield.com/decks/public?${params.toString()}`;
}

async function waitForResults(page, timeoutMs = 30000) {
  await page.waitForFunction(
    () =>
      Array.from(document.querySelectorAll('a[href^="/decks/"]')).some((a) => {
        const h = a.getAttribute('href') || '';
        return /^\/decks\/[A-Za-z0-9_-]{15,}/.test(h);
      }),
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
  const visible = await el.evaluate((b) => {
    const rect = b.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0 && !b.disabled;
  });
  if (!visible) return false;

  const box = await el.boundingBox();
  if (box) {
    await page.mouse.move(
      box.x + rand(4, box.width - 4),
      box.y + rand(4, box.height - 4),
      { steps: randInt(8, 18) }
    );
    await sleep(microPauseMs());
  }
  await el.click();
  return true;
}

// ---------- network interceptor ----------
function attachInterceptor(page, ctx) {
  page.on('response', async (response) => {
    const url = response.url();
    if (!url.includes('/v2/decks/search-sfw')) return;
    if (response.status() !== 200) return;
    let json;
    try {
      json = await response.json();
    } catch (_e) {
      return;
    }
    if (!json || !Array.isArray(json.data)) return;

    let added = 0;
    for (const d of json.data) {
      const id = d.publicId;
      if (!id) continue;
      if (ctx.state.seenIds[id]) continue;
      ctx.state.seenIds[id] = ctx.currentLabel;
      appendCsvRow({
        publicId: id,
        publicUrl: d.publicUrl || `https://moxfield.com/decks/${id}`,
        name: d.name,
        format: d.format,
        bracket: d.bracket,
        autoBracket: d.autoBracket,
        colorIdentity: d.colorIdentity,
        likeCount: d.likeCount,
        viewCount: d.viewCount,
        bookmarkCount: d.bookmarkCount,
        mainboardCount: d.mainboardCount,
        authorUserName: d.createdByUser && d.createdByUser.userName,
        createdAtUtc: d.createdAtUtc,
        lastUpdatedAtUtc: d.lastUpdatedAtUtc,
        sourceSlice: ctx.currentLabel,
      });
      added++;
      ctx.sliceAdded += 1;
    }
    if (added > 0) {
      ctx.lastBatchAdded = added;
    } else {
      ctx.consecutiveEmpty += 1;
    }
    if (added > 0) ctx.consecutiveEmpty = 0;
  });
}

// ---------- slice runner ----------
async function runSlice(page, slice, ctx) {
  const label = `${slice.fmt}-${slice.sort}`;
  const url = buildSliceUrl(slice);
  ctx.currentLabel = label;
  ctx.sliceAdded = 0;
  ctx.consecutiveEmpty = 0;
  ctx.lastBatchAdded = -1;

  log(`---- slice ${label} (target ${slice.target}) ----`);
  log(`navigating to ${url}`);

  await page.goto(url, { waitUntil: 'networkidle2', timeout: 60000 });
  try {
    await waitForResults(page, 30000);
  } catch (_e) {
    log(`! no results rendered for slice ${label}, skipping`);
    return;
  }

  await sleep(dwellMs());
  await maybeHoverDeckCard(page);

  const startCount = ctx.sliceAdded;
  // The first navigation already triggered one search-sfw call → ~64 decks
  // captured by the interceptor. Now we click View More until target.

  while (ctx.sliceAdded < slice.target) {
    await humanScroll(page, 'down');
    await sleep(microPauseMs());
    await maybeScrollUpAndDown(page);

    const totalUnique = Object.keys(ctx.state.seenIds).length;
    log(`  slice ${label}: sliceAdded=${ctx.sliceAdded}/${slice.target} grandTotal=${totalUnique}/${TARGET_TOTAL} consecEmpty=${ctx.consecutiveEmpty}`);

    if (totalUnique >= TARGET_TOTAL) {
      log('hit overall target, returning early');
      ctx.state.perSliceCollected[label] = ctx.sliceAdded;
      saveState(ctx.state);
      return;
    }

    if (ctx.consecutiveEmpty >= 2) {
      log(`  slice ${label}: stalled (${ctx.consecutiveEmpty} consecutive empty batches), moving on`);
      break;
    }

    const wait = dwellMs();
    log(`  dwelling ${(wait / 1000).toFixed(1)}s before next View More`);
    await sleep(wait);

    const clicked = await clickViewMore(page);
    if (!clicked) {
      log(`  slice ${label}: no more View More button, slice exhausted`);
      break;
    }
    // wait briefly for the API response to land before next loop iteration
    await sleep(randInt(800, 2200));
  }

  ctx.state.perSliceCollected[label] = ctx.sliceAdded;
  saveState(ctx.state);
}

// ---------- main ----------
async function main() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  ensureCsvHeader();
  const state = loadState();

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
  const page = pages[0] || (await browser.newPage());
  await page.setExtraHTTPHeaders({ 'Accept-Language': 'en-US,en;q=0.9' });

  const ctx = {
    state,
    currentLabel: 'warmup',
    sliceAdded: 0,
    consecutiveEmpty: 0,
    lastBatchAdded: -1,
  };
  attachInterceptor(page, ctx);

  log('warming up: visiting homepage first');
  await page.goto('https://moxfield.com/', { waitUntil: 'networkidle2', timeout: 60000 });
  await sleep(randInt(2500, 6000));
  await humanScroll(page, 'down');
  await sleep(randInt(1500, 4000));

  for (let i = state.sliceCursor; i < SLICES.length; i++) {
    const slice = SLICES[i];
    state.sliceCursor = i;
    saveState(state);

    try {
      await runSlice(page, slice, ctx);
    } catch (err) {
      log(`!! error in slice ${i}: ${err.message}`);
      log('   pausing 30–60s and continuing to next slice');
      await sleep(randInt(30000, 60000));
    }

    if (Object.keys(state.seenIds).length >= TARGET_TOTAL) {
      log('overall target reached');
      break;
    }

    const pause = randInt(8000, 35000);
    log(`-- finished slice. inter-slice pause ${(pause / 1000).toFixed(1)}s --`);
    await sleep(pause);
  }

  state.sliceCursor = SLICES.length;
  saveState(state);

  log(`done. unique decks collected: ${Object.keys(state.seenIds).length}`);
  log(`csv: ${CSV_PATH}`);
  log(`state: ${STATE_PATH}`);
  log('leaving browser open. close it manually when ready.');
}

main().catch((err) => {
  console.error('fatal:', err);
  process.exit(1);
});
