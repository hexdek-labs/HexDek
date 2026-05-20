# Stub Hunt — Misc (docs / scripts / root / components deep) — r47

Date: 2026-05-20
Scope (areas not yet covered by r46 passes):
- `docs/` TODO-state markers
- `scripts/` (shell + Python helpers)
- Repository root (`*.md`, `*.txt`, tracked binaries)
- `hexdek/src/components/`, `hooks/`, `i18n*`, `services/` deep
- post-r46 residue in `services/mock.js` + `hooks/useData.js`

Branch: `dev/stub-hunt-misc-r47`

The r46 frontend pass left several dead exports + stale mock data that
became unreferenced once `useMatchups` and the orphan Forge methods were
deleted. This pass catalogs that residue plus stubs found in the
"adjacent" surfaces that weren't on r46's beat — the Python parser, the
locale plumbing, scripts/, and tracked binaries at the repo root.

## Findings

| # | Severity | Where | Issue |
|---|----------|-------|-------|
| 1 | **High** | `/gen-handlers` (repo root) | A 5 MB `Mach-O 64-bit executable x86_64` binary is tracked in git. First committed in `214f05c "Engine"`. Not in `.gitignore` (the existing rules cover `/hexdek-*` and `*-linux` but not this name). Source lives at `cmd/gen-handlers/main.go`; the binary should be a `go run` / `go build` artifact, not a checked-in blob. |
| 2 | High | `scripts/parser.py:1052` (`_all_creatures_buff`) | `@rule(r"^all creatures get [+-]\d+/[+-]\d+ until end of turn...")` matches the text but the regex has no capture groups, so the handler returns `Buff(power=0, toughness=0, ...)` with a literal `# placeholder: TODO parse stats` comment. Every "all creatures get +N/+M until end of turn" effect parses to a no-op buff. Real consumer: `cmd/hexdek-oracle-sync/main.go:262` re-parses changed cards via `scripts/parser.py`. |
| 3 | Med | `hexdek/src/i18n.js:31-35` | Comment claims `Stub catalogs (ja/de/fr/ko/zh/pt) are intentionally {} and fall through to English`. False — all 8 locale catalogs are populated (each 73 lines, includes `app`, `nav`, `theme`, `common`, `dashboard`, `decks`, `bug_report`, etc.). Stale claim from an earlier scaffold. |
| 4 | Med | `hexdek/src/hooks/useData.js:156` / `:164` | Two dead exports left over after r46's frontend cleanup: `useDeckAnalysis(deckId)` and `useDeckDetail(owner, id)`. Grep shows no callers anywhere in `hexdek/src/`. `useDeckAnalysis` is the last reference to `MOCK_DECK_ANALYSIS`. |
| 5 | Med | `hexdek/src/services/mock.js:94-114` | `MOCK_DECK_ANALYSIS` constant — only consumer was the now-dead `useDeckAnalysis` hook and the (r46-removed) DeckArchive seed. Pure dead data; the "Sanguine Bond / Exquisite Blood" sample analysis no longer ships into any code path. |
| 6 | Med | `hexdek/src/components/AuthPrompt.jsx:87-105` | Discord OAuth `<button disabled title="Discord OAuth coming soon">` — same finding as r46 #6, repeated here because it lives in components/ and hasn't moved. Tracked for visibility only. |
| 7 | Low | `hexdek/src/components/MagicLinkConsole.jsx:17-28` | The "auth handshake" console is **theatrical** — a scripted sequence of `+ token signature OK`, `+ session stitched`, `+ profile hydrated` lines on a 220-2280 ms timer. None correspond to real work; the actual auth verification finishes before the component mounts. Acceptable as UX flair, flagged because the line text reads like progress reporting. |
| 8 | Low | `docs/HexDek TODO Board.md` | 22 unchecked board items, mostly per-card cost-enforcement gaps (Giada/Aminatou/Azami/Bilbo/etc.). Not stubs per se — they're engine-work tickets — but the board hasn't been reconciled with the r36-r46 per_card stub-batch ports, so some may already be closed. |
| 9 | Low | `docs/architecture/Tool - Import.md:25,39` + `docs/architecture/Decklist to Game Pipeline.md:34` + `cmd/hexdek-import/main.go:7,15,49` | Use the literal `XXXXX` / `XXXXXX` as a Moxfield deck-id placeholder. Cosmetic — example URLs in docs/usage text. |
| 10 | Low | `PROJECT_AUDIT.md` (root) | Self-describes as `expected to go stale; regenerate by re-running the sanity commands` and is dated 2026-04-15. 35 days old; entries reference parser counts that have shifted in the r36-r46 work. |

## Non-issues (filtered)

- `scripts/coverage_honest.py` mentions "stub" frequently — that's the
  vocabulary for AST modification kinds (`custom(slug)`, `OracleText`), not
  a code stub.
- `return null` guards throughout components/ are legitimate empty-state
  rendering.
- `placeholder="..."` HTML attributes on inputs are UX hints, not stubs.
- `cmd/hexdek-import/main.go` `XXXXX` is a Moxfield URL placeholder in
  help text — cosmetic.

## Top-5 inline fixes applied in this branch

1. **Remove `/gen-handlers` binary** from tracking; add it to `.gitignore`.
2. **Fix `scripts/parser.py:1052`** — capture and parse the `+N/+M` stats so
   "All creatures get +X/+Y until end of turn" effects produce a real Buff
   instead of a silent 0/0 no-op.
3. **Correct stale comment** in `hexdek/src/i18n.js` — the non-English
   catalogs are populated, not empty stubs.
4. **Delete dead exports** `useDeckAnalysis` + `useDeckDetail` from
   `hexdek/src/hooks/useData.js`, along with the now-orphan
   `MOCK_DECK_ANALYSIS` import.
5. **Delete dead `MOCK_DECK_ANALYSIS`** export from `hexdek/src/services/mock.js`.

Findings #6–#10 are out of scope for this branch: #6 is a roadmap signal
already noted in r46; #7 is intentional UX flair; #8 is a board-reconciliation
ask, not a code change; #9 is one cosmetic placeholder string repeated in
3 docs (low value); #10 is a regenerate-not-edit document.

## Verification

```
go build ./...
go test ./cmd/hexdek-oracle-sync/... -count=1 -timeout 60s  # touches parser.py path
python3 -c "import sys; sys.path.insert(0, 'scripts'); import parser; \
    rule = next(r for r,_ in parser._RULES if 'all creatures get' in r.pattern); \
    print(rule.pattern)"   # smoke
cd hexdek && VITE_API_URL='' npx vite build
```
