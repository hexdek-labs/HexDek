/* eslint-disable no-console */
'use strict';

/**
 * Phase 2: per-deck full-data fetcher.
 *
 * Reads share-links.csv (Phase 1 output), then for each publicId fetches
 * https://api2.moxfield.com/v3/decks/all/{id} from inside a real browser
 * page context (page.evaluate + fetch with credentials), so Cloudflare
 * sees a normal logged-out browser session and serves the JSON.
 *
 * Output: deck-details.ndjson — one JSON object per line, the full
 * Moxfield deck payload (mainboard, sideboard, commander, hubs, etc.)
 * Resumable via details-state.json.
 *
 * The output format is what HexDek's FetchDeckByID consumer expects.
 */

const path = require('path');
const fs = require('fs');
const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');

puppeteer.use(StealthPlugin());

// ---------- config ----------
const ROOT = __dirname;
const OUTPUT_DIR = path.join(ROOT, 'output');
const SHARE_LINKS_CSV = path.join(OUTPUT_DIR, 'share-links-run2.csv');
const DETAILS_NDJSON = path.join(OUTPUT_DIR, 'deck-details-run2.ndjson');
const ERRORS_NDJSON = path.join(OUTPUT_DIR, 'deck-detail-errors-run2.ndjson');
const STATE_PATH = path.join(OUTPUT_DIR, 'details-state-run2.json');

// Stop when this many unique decks are in done.
const TARGET_DETAILS = 1000;
// Optional limit for testing — set 0 to ignore.
const LIMIT = parseInt(process.env.LIMIT || '0', 10);

// ---------- pacing ----------
// Per-deck dwell — doubled from the tightened pace, slow and deliberate.
function dwellMs() {
  const r = Math.random();
  if (r < 0.75) return randInt(500, 2400);   // 75%  0.5–2.4s
  if (r < 0.95) return randInt(2400, 7000);  // 20%  2.4–7s
  if (r < 0.995) return randInt(7000, 24000);//  4.5% 7–24s
  return randInt(24000, 60000);              //  0.5% 24–60s
}

const rand = (min, max) => Math.random() * (max - min) + min;
const randInt = (min, max) => Math.floor(rand(min, max + 1));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function logTimestamp() {
  return new Date().toISOString().replace('T', ' ').slice(0, 19);
}
function log(...args) {
  console.log(`[${logTimestamp()}]`, ...args);
}

// ---------- state ----------
function loadState() {
  if (!fs.existsSync(STATE_PATH)) return { done: {}, errors: {} };
  return JSON.parse(fs.readFileSync(STATE_PATH, 'utf8'));
}
function saveState(state) {
  fs.writeFileSync(STATE_PATH, JSON.stringify(state, null, 2));
}

// ---------- csv input ----------
function loadPublicIdsFromCsv(csvPath) {
  if (!fs.existsSync(csvPath)) {
    throw new Error(`CSV not found: ${csvPath}`);
  }
  const lines = fs.readFileSync(csvPath, 'utf8').split('\n');
  const ids = [];
  const seen = new Set();
  for (const line of lines) {
    if (!line || line.startsWith('publicId,')) continue;
    const id = line.split(',')[0].trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    ids.push(id);
  }
  return ids;
}

// ---------- fetch in page context ----------
async function fetchDeckFromPage(page, publicId) {
  return page.evaluate(async (id) => {
    const url = `https://api2.moxfield.com/v3/decks/all/${id}`;
    const r = await fetch(url, {
      credentials: 'include',
      headers: { 'Accept': 'application/json' },
    });
    return {
      status: r.status,
      ok: r.ok,
      body: r.ok ? await r.json() : (await r.text()).slice(0, 500),
    };
  }, publicId);
}

// ---------- main ----------
async function main() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  const state = loadState();

  log('launching headed browser');
  const browser = await puppeteer.launch({
    headless: false,
    defaultViewport: null,
    args: [
      '--window-size=1280,900',
      '--disable-blink-features=AutomationControlled',
      '--lang=en-US',
    ],
  });
  const pages = await browser.pages();
  const page = pages[0] || (await browser.newPage());
  await page.setExtraHTTPHeaders({ 'Accept-Language': 'en-US,en;q=0.9' });

  log('warming up: visiting moxfield.com');
  try {
    await page.goto('https://moxfield.com/decks/public', { waitUntil: 'domcontentloaded', timeout: 60000 });
  } catch (e) {
    log(`  warmup goto warning: ${e.message} — continuing`);
  }
  await sleep(randInt(2500, 6000));

  let okCount = 0;
  let errCount = 0;
  let consecutive403 = 0;

  // Outer loop: keep re-reading CSV until target is met or no progress possible.
  outer: while (Object.keys(state.done).length < TARGET_DETAILS) {
    const publicIds = loadPublicIdsFromCsv(SHARE_LINKS_CSV);
    let toFetch = publicIds.filter((id) => !state.done[id]);
    if (LIMIT > 0) toFetch = toFetch.slice(0, LIMIT);

    if (toFetch.length === 0) {
      log(`waiting: csv has ${publicIds.length} ids, all done or queued. polling for more in ~60s...`);
      await sleep(randInt(45000, 90000));
      continue;
    }
    log(`csv has ${publicIds.length} unique publicIds, ${Object.keys(state.done).length} done, processing ${toFetch.length} new`);

    for (let i = 0; i < toFetch.length; i++) {
      const id = toFetch[i];

    let result;
    try {
      result = await fetchDeckFromPage(page, id);
    } catch (err) {
      log(`! fetch threw for ${id}: ${err.message}`);
      fs.appendFileSync(ERRORS_NDJSON, JSON.stringify({ publicId: id, error: err.message, at: new Date().toISOString() }) + '\n');
      state.errors[id] = err.message;
      saveState(state);
      errCount++;
      await sleep(randInt(2000, 6000));
      continue;
    }

    if (result.ok) {
      const line = JSON.stringify({ publicId: id, fetchedAtUtc: new Date().toISOString(), data: result.body }) + '\n';
      fs.appendFileSync(DETAILS_NDJSON, line);
      state.done[id] = { fetchedAtUtc: new Date().toISOString(), status: 200 };
      okCount++;
      consecutive403 = 0;
    } else {
      // 404 = deleted; 403 = cloudflare or blocked; 429 = rate-limited
      fs.appendFileSync(ERRORS_NDJSON, JSON.stringify({
        publicId: id, status: result.status, body: result.body, at: new Date().toISOString()
      }) + '\n');
      state.errors[id] = { status: result.status };
      if (result.status === 404) {
        // Deleted decks: mark done so we don't retry forever
        state.done[id] = { fetchedAtUtc: new Date().toISOString(), status: 404 };
      }
      errCount++;
      if (result.status === 403) consecutive403++;
      if (result.status === 429) {
        log(`!! rate limited (429), backing off 2–5 min`);
        await sleep(randInt(120000, 300000));
      }
    }

    if ((i + 1) % 25 === 0) saveState(state);

    if (consecutive403 >= 5) {
      log(`!! ${consecutive403} consecutive 403s — Cloudflare may be challenging. Pausing 5–10 min and reloading the page.`);
      await sleep(randInt(300000, 600000));
      try {
        await page.goto('https://moxfield.com/decks/public', { waitUntil: 'domcontentloaded', timeout: 60000 });
        await sleep(randInt(3000, 8000));
      } catch (_e) {}
      consecutive403 = 0;
    }

    if ((i + 1) % 10 === 0) {
      log(`progress: ${i + 1}/${toFetch.length}   ok=${okCount}   err=${errCount}   doneTotal=${Object.keys(state.done).length}`);
    }

    if (Object.keys(state.done).length >= TARGET_DETAILS) {
      log(`reached TARGET_DETAILS=${TARGET_DETAILS}, stopping cleanly`);
      break outer;
    }

    await sleep(dwellMs());
    } // close inner for
    // inner for finished — we drained the current toFetch slice. Loop back to re-read CSV.
    saveState(state);
  } // close outer while

  saveState(state);
  log(`done. ok=${okCount} err=${errCount} total_done=${Object.keys(state.done).length}`);
  log(`output: ${DETAILS_NDJSON}`);
  log(`errors: ${ERRORS_NDJSON}`);
  log('leaving browser open. close it manually when ready.');
}

main().catch((err) => {
  console.error('fatal:', err);
  process.exit(1);
});
