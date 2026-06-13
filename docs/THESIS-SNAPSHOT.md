# HexDek — State of the System (r63 snapshot)

*2026-06-13. An honest snapshot of what HexDek is and where it stands after the r60→r63 consolidation era. Written as the "thesis" reference — not a pitch, a map.*

## What HexDek is

An open-source (MIT) MTG **Commander** game engine, AI player, and analysis platform, in Go (engine/AI/tooling) + React/Vite (frontend). It simulates 4-player Commander games end-to-end with AI decision-making, deck analysis, ELO/TrueSkill rating, and a live spectator. Module `github.com/hexdek/hexdek`.

## The thesis of this era: the engine judges itself

The defining idea of r60→r63: **correctness stopped being a pile of scattered tools and became a faculty of the engine — the Hex Judge** (`internal/judge`). Before, ~13 validation surfaces spoke 7 different violation vocabularies and re-implemented the same checks. Now there is **one** faculty, one violation type (`ValidationViolation`), one router (`LogViolation`), and checks registered by dimension. Tools either folded IN as checks or shrank to thin DRIVERS that feed the Judge. Discipline: *fold N, delete N* — measured in LOC deleted and surfaces collapsed, not features added.

### Correctness = 6 dimensions

| Dimension | Question it answers | State |
|---|---|---|
| **LEGALITY** | Was each action allowed? (target/timing/priority/cost) | 100% |
| **CONSERVATION** | Are objects conserved by identity? | **100% (clean-100)** |
| **STATE-INTEGRITY** | Is state internally consistent? (SBA/layers/win-loss/orphans) | 99.75% |
| **PROGRESSION** | Does the game advance right? (triggers fire-when + APNAP order) | 99.07% |
| **OUTCOME** | Did each effect produce the *right* result? | 100% |
| **LIVENESS** | Does the game actually terminate? | 100% |

OUTCOME is the one genuinely new idea: a **second, independent AST interpreter** computes the expected post-resolution state-delta and asserts it against what the engine actually did — parity against *intent*, per-effect, at scale. It closed the engine's biggest blind spot (we used to verify "something changed," never "the *right* thing changed"). LIVENESS generalizes the infinite-loop guards into a standing sense.

### The honest score

Topline **99.80%** (400-game sweep, seed 42). It is deliberately *not* rounded to 100%. It went *down* from a prior 99.93% — because PROGRESSION coverage widened ~68% (now measuring 3,026 triggers vs ~1,847), surfacing 28 real trigger-dispatch bugs that were always there but unmeasured. **The principle: widening coverage trades a prettier number for a truer one.** The score is reproducible (`cmd/hexdek-correctness`), per-dimension, and tracked. The Judge runs live too — a sampled watchdog over the production grinder (`HEXDEK_JUDGE_SAMPLE`) writes violations to a stream that Muninn auto-files into a triage report.

## The engine

Rules engine in `internal/gameengine` — zones, stack, combat, triggers, SBAs, layers, replacement effects, the full Commander ruleset. The richest remaining frontier is **§800.4a — what happens to objects when a player leaves the game** (the eliminated-seat edge); it has produced a steady stream of real bugs (the conservation clean-100 fixes, the liveness retained-library loops) and is where active correctness work is pointed.

## The AI — YggdrasilHat

Layered evaluator: 8 evaluation dimensions, archetype-aware MCTS weights from Freya, a strategy-profile bridge, "3rd Eye" opponent tracking, an adaptive budget system, and a conviction/concession model. The **Huginn learning loop** (opponent-prediction n-tuples) was just *closed* this era — games now graduate co-trigger observations into tiers that feed back into Freya's analysis and the hat's evaluation. Play → observe → graduate → analyze → better play.

## The tooling (Norse suite, now consolidated)

Thor (oracle→AST parser), Freya (deck analysis: archetype/combos/roles/win-lines/bracket), Loki (fuzz tester), Heimdall (post-game analytics + spectator), Muninn (now the Judge's **triage clerk**), plus the server, tournament runner, and importer. The r63 cleanup deleted ~16K LOC of stub tools and dead modules, retired goldilocks into OUTCOME, folded parity's vocabulary into the canonical type, and turned the standalone validators into Judge checks. The full retirement record lives in `docs/LEGACY-EOL.md`.

## Frontend & deployment

React/Vite app (`hexdek/`) — spectator, decks, queue, party screens; deployed to MISTY behind Caddy at hexdek.dev. Engine runs on DARKSTAR (Windows 11) as a scheduled-task service on :8090. Anonymous, privacy-respecting page analytics (the "Temporal Pincer" — per-browser UUID, no PII, owner-stitched on login). Sweeping UX changes go through a whitelisted staging review before prod; backend ships freely.

## Where it's going

1. **Drain the §800.4a / eliminated-seat frontier** — the deepest correctness vein.
2. **Fix the 28 PROGRESSION trigger bugs** the widened coverage just exposed (ally_etb/etb cluster first).
3. **Mature the loops further** — Huginn graduation cadence, Freya↔hat feedback.
4. **Documentation** — this snapshot, the changelog folder, feature write-ups, and a full bug ledger (fixed / worked-around / was-a-bug-then-became-intended-change).
5. **Deck editor** — a real search/edit experience (staged, awaiting owner review).

The through-line: **an engine honest enough to measure its own correctness, and disciplined enough to shrink while it grows.**
