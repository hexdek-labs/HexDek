# Legacy & End-of-Life Log

A durable record of every retired tool, module, and feature surface:
what it was, when and why it went, and what (if anything) replaced it.
Newest entries first within each group. When you retire something, add
it here in the same commit.

Conventions: LOC counts are production lines at deletion time (tests
excluded per the repo's LOC accounting). "Judge" = the Hex Judge
(`internal/judge`), the engine-embedded correctness faculty whose
five-dimension consolidation (r63) absorbed most of the standalone
validation surfaces below.

---

## Folded into the Hex Judge

Surfaces whose CHECKS live on inside the Judge's dimensions; the
standalone host was deleted once the fold was complete ("fold N,
delete N").

| Date | What | LOC | Fold destination | Notes |
|------|------|----:|------------------|-------|
| 2026-06-12 | **Goldilocks dead-effect sweep** (`cmd/hexdek-thor` goldilocks mode, partial) | n/a (file retained) | Judge **OUTCOME** dimension, "Dead" sub-class | Goldilocks checked *something-changed*; OUTCOME checks *the-RIGHT-thing-changed* — strictly stronger. The goldilocks.go file remains for keyword-observability scaffolding shared with other thor modes. |
| 2026-06-12 | **Feynman count-heuristic** (hat `checkZoneAccounting` + fallback) | ~145 | Judge **CONSERVATION** (InstanceID strict census) | 499/500 of count-shape warnings were false positives; the identity census is the production conservation authority. |
| 2026-06-12 | **Legacy zone-conservation count path** (`checkZoneConservationLegacyCount` + `countRealCards` + baseline writes) | ~73 | Judge **CONSERVATION** | Pre-Phase-4 fallback; unminted states are struct-literal fixtures with nothing to conserve. |
| 2026-06-12 | **`internal/validation` package** | n/a (renamed) | `internal/judge` | The consolidation-step-1 leaf package (ValidationViolation / Event / LossReason / LogViolation router) WAS the Judge's core; promoted wholesale, 21 importers rewritten, zero semantic change. |
| 2026-06-12 | **paritycheck violation vocabulary** (`Divergence` violation half) | ~30 (refactored) | `judge.ValidationViolation` | The parity Event schema was promoted VERBATIM as the canonical `judge.Event` (wire format pinned byte-for-byte); `paritycheck.Event` is a true alias. |
| 2026-06-12 | **freya deck-legality module** (`cmd/hexdek-freya/legality.go`) | 428 | Judge **LEGALITY** (`judge.CheckDeckLegality`) | The five Commander deck checks (count/identity/singleton/banlist/commander) moved to first-principles checks; freya is a ~70-LOC driver; `judge.IsCommanderBanned` is the one banned list. |
| 2026-06-12 | **`lastEvent` single-slot event store** + `RetainEvents` bool | ~30 + 60 refs | `EventPolicy {full\|ring\|none}` on the one `EventLog` | The dual path whose silent-drop split caused the goldilocks 1,795 keyword_dead misreports. Zero value now means FULL retention — the footgun class cannot recur by omission. |

## Superseded by generic dispatch (Wave-1b inline-arm folds)

Hardcoded per-card inline arms deleted once a generic engine path
covered the printed ability — each was a double-fire once both ran.

| Date | What | Superseded by |
|------|------|---------------|
| 2026-06-12 | **Young Pyromancer / Third Path Iconoclast / Monastery Mentor** inline token arms (cast_counts.go) | Raw-aware AST observer-cast dispatch (`observer_raw_dispatch.go`) |
| ~2026-06 (r62) | **Storm-Kiln Artist** inline Treasure arm | per_card magecraft handler (`FireMagecraftTriggers`) |
| ~2026-06 (r62) | **Niv-Mizzet, Parun** inline draw arm | per_card instant_or_sorcery_cast handler |

## Orphaned stubs & dead tools (deleted outright)

Zero non-self importers, zero script/CI wiring, no live runbook.

### 2026-06-12 — kill-list Part E (PR #1051, −8,985 LOC)

| Tool | LOC | Why dead |
|------|----:|----------|
| `cmd/hexdek-odin` | 418 | Duplicated loki + the invariant table; no unique functionality |
| `cmd/hexdek-valkyrie` | 573 | Subset of the tournament runner; never matured |
| `cmd/hexdek-contrib` | 336 | Distributed-compute CLIENT proof-of-concept; the server half (hexapi/contrib*, tournament/chunk) remains live |
| `cmd/hexdek-ceiling` | 847 | One-off B5 bracket-calibration benchmark |
| `cmd/hexdek-huginn` | 1,409 | Tier-3 interaction-promotion CLI; `internal/huginn` itself remains live (freya/tournament/hexapi/hat import it) |
| `cmd/dump_drift` | 706 | Keyword-parser drift reporter; analytics only |
| `cmd/hexdek-artfetch` | 394 | Scryfall art prefetcher; unrelated to rules |
| `cmd/hexdek-oracle-sync` | 1,652 | Oracle bulk-data refresh pipeline; superseded by `scripts/fetch-oracle.sh` + Thor re-parse |
| `cmd/hexdek-heimdall-scanner` | 205 | Per-card health analytics; not rules enforcement |

Seven dead Thor modules in the same PR (each wired only by a flag + a
module-table row): `adversarial` 317 · `combo_pairs` 263 ·
`density_stress` 448 · `multiplayer_chaos` 350 · `symmetry` 343 ·
`rollback_torture` 396 · `clock_pressure` 274.

### 2026-06-12 — dead-code sweep (PR #1054, −5,472 LOC net)

196 unreachable functions across 50 files in `internal/` — chiefly the
unwired keyword-primitive library in `gameengine/keywords_batch*.go`
(kicker/suspend/vanishing/echo/morph/soulbond/haunt families written in
batch sweeps and never wired into any dispatch). Verified with
`deadcode -test` + registry/reflection/build-tag audits. Notable KEEPS
from that sweep: `gameast` marker-interface methods (type-system
required), anticheat `Scheduler/Worker/Cauterize` (dormant in-flight
surface), `per_card.findGiftRecipient` (explicit keep-alive anchor).

### 2026-06-13 — round-2 step-1 orphans (this entry's PR)

| Tool | LOC | Why dead |
|------|----:|----------|
| `cmd/snapshot-backfill` | 1,025 | Zero references anywhere — no importer, no script, no doc |
| `cmd/ceiling-check` | 152 | Kill-list-era sibling of the deleted `hexdek-ceiling` that survived round 1; only mention was a stale row in the r60 coverage table |
| `cmd/audit-test-coverage` | 597 | One-shot generator of `docs/test-coverage-audit-r60.md` (single commit, 2026-05-27, never rerun, no runbook); the report it produced remains |

## Evaluated and KEPT (for the record)

| Tool | LOC | Why kept |
|------|----:|----------|
| `cmd/audit-ast-oracle` | 1,129 | Live runbook: `docs/audit-ast-vs-oracle-r60.md` documents regeneration (`go run ./cmd/audit-ast-oracle --out … --top 50`) and the AST↔oracle drift audit (840 drift entries at last run) is the corpus-health check the Judge lanes' parser-fidelity findings feed into. Recurring value every corpus refresh. |
| `cmd/validate-composition-prior` | 310 | Scientific validation harness for `trueskill.CompositionPrior`, which is LIVE production code; `docs/composition-elo-validation-r60.md` documents the rerun procedure (`-seed/-bootstrap/-test` flags). Needed whenever the prior is retuned. |

## Known-dead features awaiting a decision (not yet EOL'd)

| Feature | State | Note |
|---------|-------|------|
| Delve cost payment (`PayDelve`, `HasDelve`, `DelveMaxReduction`) | Unwired — zero callers; delve cards cast at full cost | A FEATURE gap, not dead weight: wire it or delete it. Flagged in the r63 conservation report (PR #1062). |
