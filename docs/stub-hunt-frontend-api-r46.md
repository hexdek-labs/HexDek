# Stub Hunt — Frontend + hexapi (r46)

Date: 2026-05-20
Scope: `hexdek/src/` (React/Vite), `internal/hexapi/` (Go HTTP layer)
Branch: `dev/stub-hunt-frontend-r46`

Surveyed every `.jsx` / `.js` under `hexdek/src/` and every `.go` under
`internal/hexapi/` for TODO/FIXME markers, no-op handlers, mock-data
fallbacks that mask dead endpoints, hardcoded "OPERATOR" placeholders,
and routes registered on one side of the wire with no consumer on the
other. Excluded test-only files. The grep noise from `placeholder=`
input attributes was filtered out — most of those are legitimate UI hints,
not stubs.

## Findings

| # | Severity | File:line | Issue |
|---|----------|-----------|-------|
| 1 | **High** | `hexdek/src/hooks/useData.js:143` + `services/api.js:68` + `services/mock.js:85` | `useMatchups()` calls `GET /api/tournament/stats` — no backend handler exists. Falls back to `MOCK_MATCHUPS` silently (console-warn only). The result is destructured in `Dashboard.jsx:192` but **never read**: 100% dead path. |
| 2 | High | `hexdek/src/services/api.js:65-67` | Orphan Forge API methods: `getForgeStatus`, `getForgeResults`, `startForge`. No UI caller; the gauntlet endpoints superseded them. Backend has no `/api/forge/*` routes. |
| 3 | High | `internal/hexapi/handler.go:928-972` | `handleProfile` (`GET /api/profile`) returns a hardcoded "OPERATOR / USR.0001 / 04.2026 / UNRANKED" envelope augmented with global Showmatch stats. It's a non-user-aware profile masquerading as one — real per-owner data lives at `/api/profile/{owner}`. `useProfile()` consumes this and renders it on the Dashboard as the signed-in user's stats. |
| 4 | Med  | `hexdek/src/screens/DeckArchive.jsx:733` | When the route lacks `owner`/`id`, the analysis panel is seeded from `MOCK_DECK_ANALYSIS.tinybones`. Real users hitting `/deck-archive` with no deck context see fabricated Sanguine Bond combos and "34% / 7.2 / 0.69" benchmarks attributed to nothing. |
| 5 | Med  | `hexdek/src/screens/Forge.jsx:338` | Gauntlet "RUNNING" button uses `onClick={() => {}}` instead of `disabled`. Functionally a stub click handler; semantically incorrect for a busy-state indicator. |
| 6 | Med  | `hexdek/src/components/AuthPrompt.jsx:87-105` | Discord OAuth button hard-disabled with `title="Discord OAuth coming soon — use email for now"`. No provider plumbing. Acceptable as a roadmap signal but the stub has lived through r41-r46 without movement. |
| 7 | Med  | `internal/hexapi/handler.go:2071` + `:2105` | Backend handlers `handleRivalry` and `handleThreatGraph` register `GET /api/rivalry/{owner}/{id}` and `GET /api/threat-graph/{owner}/{id}`. No frontend code (`hexdek/src/**`) references either path. Dead routes — they load data files (`data/rivalry`, `data/analytics`) only when an external caller hits them. |
| 8 | Low  | `hexdek/src/hooks/useData.js:12-37` | `useAsync` console-warns every API failure with `"API unavailable, using mock data"` and substitutes the mock fallback. Masks legitimate 4xx/5xx during dev — caller can't tell mock from real without reading the network tab. |
| 9 | Low  | `hexdek/src/screens/Report.jsx:855` | UI literally renders `"PER-TURN LOG RETENTION IS A SERVER-SIDE TODO"`. The timeline-with-no-turn-stamps fallback is honest but the text exposes the engineering todo to end users. |
| 10 | Low | `hexdek/src/services/mock.js:1-120` | Now-dead mock data: `MOCK_PROFILE`, `MOCK_MATCHUPS`, `MOCK_LIVE_STATS`, `MOCK_DECK_ANALYSIS` — after the dead consumers above are removed, several blocks become unreachable. |

## Non-issues (filtered)

The following matched stub-pattern greps but are legitimate behavior:

- `internal/hexapi/sharepage.go` / `sharepage_more.go` — the word "stub" is used
  for the OG-meta crawler fallback, which is an intentional minimal-HTML
  document, not an unimplemented feature.
- `placeholder=` HTML attributes on `<input>` elements (Splash, Friends,
  Profile, DeckCompare, Forge, DeckList, Login, BugReport, Leaderboard,
  AdminConviction, SearchBar, ImportModal, TagInput, DeckArchive) — these
  are UX hints, not code stubs.
- `internal/hexapi/deckmeta.go` `return "", nil` paths — legitimate empty-tag
  encoding.

## Top-5 inline fixes applied in this branch

1. **Delete dead `useMatchups` chain.** Drop `useMatchups` export, the
   `getTournamentStats` API method, `MOCK_MATCHUPS`, and the unused
   `matchups` destructure + import in `Dashboard.jsx`.
2. **Delete orphan Forge API methods.** Remove `getForgeStatus`,
   `getForgeResults`, `startForge` from `services/api.js`.
3. **Strip hardcoded fake fields from `handleProfile`.** The endpoint
   keeps the live-stats fields driven by Showmatch but stops shipping
   `"USR.0001"`, `"04.2026"`, `"UNRANKED"`, etc. as if they were real
   user data. Empty strings mean "no profile bound to caller" and the
   transform in `useData.js` already substitutes harmless defaults.
4. **Forge `RUNNING` button →  `disabled`.** Replace `onClick={() => {}}`
   with `disabled` so the busy state is communicated correctly to assistive
   tech and to anyone reading the JSX.
5. **Drop `MOCK_DECK_ANALYSIS.tinybones` seed in `DeckArchive.jsx`.** When
   no owner/id is present the component now stays in its empty/loading
   state instead of rendering fabricated Sanguine Bond combos.

Findings #6–#10 are noted but **not** fixed in this pass — they're either
intentional roadmap markers (Discord OAuth), separately scoped (per-turn
log retention is engine work, not API), or follow-on cleanup that should
land after the dead-consumer removals above settle (mock.js pruning).

## Verification

```
go build ./...
go test ./internal/hexapi/... -count=1 -timeout 120s
cd hexdek && VITE_API_URL="" npx vite build
```
