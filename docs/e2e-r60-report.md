# E2E r60 — Mobile + Desktop Smoke Report

**Branch:** `dev/e2e-mobile-desktop-r60`
**Date:** 2026-05-25
**Suite:** `hexdek/e2e/smoke.spec.ts` (16 cases = 8 screens × 2 viewports)
**Config:** `hexdek/playwright.e2e.config.ts`
**Run command:** `cd hexdek && npx playwright test --config=playwright.e2e.config.ts`

The suite auto-starts the Vite dev server on `localhost:5180` (with
`VITE_API_URL=https://hexdek.dev` so the SPA can fetch real data from the
production backend) and tears it down on exit. Every test runs in both
**desktop** (1440×900) and **mobile** (390×844 / iPhone 14 viewport)
projects. Screenshots land in `hexdek/e2e/screenshots/` named
`NN-screen--{desktop,mobile}.png`.

## Headline result

**16 / 16 passing.** The pass count is misleading on its own — the
smoke contract is _"the SPA route mounted"_, and a handful of screens
mount via the error boundary or the in-screen `LOAD FAILED` panel
rather than the canonical container. Those paths still satisfy the
contract (the lazy bundle loaded, React rendered something the user
can act on) but each one is a concrete finding the next branch should
chase. The findings emitter in the spec's `afterAll` logs which
fallback selector matched each case:

```
[mobile] DeckArchive: matched=text=SOMETHING BROKE — Error boundary tripped
[mobile] GameSummary: matched=text=LOAD FAILED — Backend returned 404 for /api/games/111923144/summary
[mobile] Report (index): matched=.report-grid
[mobile] Report (game): matched=.report-grid
```

(Desktop findings are equivalent — same selectors match in both projects.)

## Per-screen results

| # | Screen | Route | Desktop | Mobile | Notes |
|---|--------|-------|---------|--------|-------|
| 01 | Spectator | `/spectate` | ✅ canonical | ✅ canonical | Live showmatch panel renders, 4 seats with deck art, board state ticking. |
| 02 | DeckList | `/decks` | ✅ canonical | ✅ canonical | Shell + filters + search input mount cleanly. Default tab is MY DECKS (0 rows when logged out) — expected. |
| 03 | DeckArchive | `/decks/7174n1c/god_save_the_queen` | ⚠️ error boundary | ⚠️ error boundary | **REAL BUG** — see "Bug: cardSearchOpen TDZ" below. |
| 04 | GameSummary | `/games/:gameId/summary` | ⚠️ LOAD FAILED | ⚠️ LOAD FAILED | Backend-side: `/api/games/:id/summary` 404s for the most recent game. Page handles it gracefully. |
| 05 | SummaryArchive | `/games/summaries` | ✅ canonical | ✅ canonical | Filters render + are interactive. Backend bug: GAMES panel shows `invalid game id` — see "Bug: summaries route shadowed" below. |
| 06 | Report (index) | `/report` | ✅ canonical | ✅ canonical | `.report-grid` mounts with the most-recent game featured. |
| 07 | Report (game) | `/report/:gameId` | ✅ canonical | ✅ canonical | DECK CONTEXT pills render across both viewports; report layout adapts. |
| 08 | Forge | `/forge` | ✅ canonical | ✅ canonical | Deck picker, FORGE SUBJECT card, and analysis panels populate. Best-looking mobile screen in the suite. |

## Bugs found

### Bug 1 — DeckArchive TDZ: `cardSearchOpen` used before initialization

**Severity:** HIGH — the entire deck-detail page crashes on every load.
**Surface:** `/decks/:owner/:id` (both desktop and mobile).
**Symptom:** App-level `ErrorBoundary` catches a `ReferenceError`
and renders the "SOMETHING BROKE / Cannot access 'cardSearchOpen'
before initialization" fallback. The deck content (analysis, gauntlet,
matchups, ELO history) never paints.

**Root cause:** in `src/screens/DeckArchive.jsx` the variable is used
in a `useEffect` at line 1255 (with a `[cardSearchOpen]` dep array at
line 1259) but the `useState` declaration is at line 1337 — 82 lines
below. React captures the closure at render time and accessing the
`let`-bound variable before its declaration line trips JS's temporal
dead zone.

```text
src/screens/DeckArchive.jsx:1255   if (cardSearchOpen && cardSearchInputRef.current) { … }
src/screens/DeckArchive.jsx:1259   }, [cardSearchOpen])
src/screens/DeckArchive.jsx:1337   const [cardSearchOpen, setCardSearchOpen] = useState(false)
```

**Fix sketch:** hoist the `useState` declaration above the `useEffect`
that reads it. Same anti-pattern likely worth grepping for across other
screen components.

**Evidence:** `hexdek/e2e/screenshots/03-deck-detail--desktop.png`,
`hexdek/e2e/screenshots/03-deck-detail--mobile.png`.

### Bug 2 — `/api/games/summaries` shadowed by `/api/games/:id`

**Severity:** MEDIUM — the SummaryArchive screen renders but its GAMES
panel shows literal text `"invalid game id"` instead of a paginated
list of summaries.

**Surface:** backend route table on `hexdek.dev` and `dev.hexdek.dev`.
Both environments return the same `invalid game id` body for
`GET /api/games/summaries`.

**Root cause hypothesis:** the route matcher resolves `summaries` as the
`:id` path parameter to `/api/games/:id` before the literal
`/api/games/summaries` route can match. Server-side route ordering
needs `summaries` declared before the parametric route, or the path
should be renamed (e.g. `/api/summaries`).

**Evidence:** `hexdek/e2e/screenshots/05-summary-archive--mobile.png`
(GAMES panel body reads `invalid game id`).

### Bug 3 — GameSummary 404 for the latest game

**Severity:** LOW — the page handles the 404 gracefully (LOAD FAILED
panel). But the most recent game lacks a persisted summary, which
means the canonical "deep-link to your latest game summary" flow lands
on an error.

**Surface:** `GET /api/games/:id/summary` for game id `111923143` (the
most-recent finished game at audit time) returns HTTP 404.

**Root cause hypothesis:** the summary-generation pipeline lags behind
game completion. Worth confirming whether summaries are generated
on-demand or asynchronously and whether a "missing summary" should
trigger generation rather than 404.

**Evidence:**
```text
$ curl -sI https://hexdek.dev/api/games/111923143/summary | head -1
HTTP/2 404
```
And `hexdek/e2e/screenshots/04-game-summary--desktop.png`.

## Mobile rendering observations (no bugs, just notes)

- **Chrome `Tape` strip is hidden via `display: none`** at narrow
  widths (`src/index.css:.tape-bar { display: none; }`). Smoke
  selectors had to avoid Tape text for mobile assertions; future
  visual-audit tests should anchor on Panel headers or container
  classes too.
- **Forge mobile (`08-forge--mobile.png`)** is the cleanest responsive
  layout in the suite — deck picker, commander art, KV stats stack
  vertically with no overflow.
- **Report game mobile (`07-report-game--mobile.png`)** packs the
  DECK CONTEXT pills (~80 commander names) into a wrap grid that fills
  the viewport. Visually noisy but functional; if there's a future UX
  pass, collapsing this into a dropdown on mobile would help.

## Reproducing locally

```bash
cd hexdek
npx playwright install chromium    # first run only
npx playwright test --config=playwright.e2e.config.ts
# Screenshots: hexdek/e2e/screenshots/
# HTML report: hexdek/e2e/report/  (npx playwright show-report e2e/report)
```

To run only one project: `--project=desktop` or `--project=mobile`.
To point at a different backend: `HEXDEK_API_URL=https://… npx playwright test …`.
