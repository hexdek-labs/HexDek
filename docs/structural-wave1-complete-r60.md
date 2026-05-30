# Structural Cleanup — Wave 1 Complete (r60)

**Date:** 2026-05-30
**Driver:** `docs/structural-analysis-r60.md` (PR #766)
**Scope:** Wave 1 of the three-wave structural cleanup — the four no-risk / high-leverage PRs.

---

## What shipped

| PR | Branch | Title | Net production LOC |
|---|---|---|---|
| #768 | `dev/structural-wave1-pr3-predefined-tokens-r60` | refactor(per_card): retire inline Treasure/Clue mints — use canonical helpers | **−70** |
| #769 | `dev/structural-wave1-pr4-face-down-helper-r60`   | refactor(resolve): extract manifestTopOfLibrary — dedupe manifest + manifest dread | **−10** |
| #770 | `dev/structural-wave1-pr5-batch-aj-platinum-angel-r60` | fix(per_card): batch_aj — route Triskaidekaphobia through MarkSeatLostByEffect | **+6** |
| #771 | `dev/structural-wave1-pr8-claudemd-cleanup-r60`   | docs(claude-md): retire stale issue-log rows — direct-Lost + removePermanent sweeps | doc-only |

**Net production LOC delta across Wave 1: ~−74.**

---

## PR-by-PR detail

### PR #768 — Predefined-token consolidation (−70 LOC)

Seven inline `&Card{Types:["token","artifact","treasure"]}` + `enterBattlefieldWithETB` sites across 6 per_card files (`dockside_extortionist.go`, `edward_kenway.go`, `grim_hireling.go`, `malcolm.go`, `the_ghoul_gunslinger.go`, `the_rani.go`) routed through canonical `gameengine.CreateTreasureToken` / `CreateClueToken`.

**Correctness payoff:** Pre-PR each inline site silently skipped `MintTokenInstanceID` (InstanceID Phase 5 plumbing), `Turn.TreasuresCreated` / `Turn.TokensCreated` / `Turn.ArtifactsEntered` counter bumps (treasure-payoff cards + artifact-ETB observers), and the canonical event-log shape.

**Stale-callout correction:** `mr_house_president_and_ceo.go` was flagged by the structural analysis (L101 inline / L142 canonical) but verified during the sweep — both call sites are already canonical. The audit row was retired without code change.

### PR #769 — Face-down token helper (−10 LOC)

`resolveModificationEffect`'s `manifest` (L3820+) and `manifest_dread` (L4485+) arms each built a structurally identical 2/2 face-down Creature inline (~22 LOC of `Card{}` + `Permanent{}` + `FrontFaceAST` / `Name` + ETB cascade). Extracted `manifestTopOfLibrary` primitive at the top of `resolve_helpers.go`; both arms now call it.

Behavior unchanged — both arms retain their distinct event-log shapes (`sub_kind=ModKind` vs `sub_kind="manifest_n"` with `Amount`).

### PR #770 — batch_aj Platinum Angel fix (+6 LOC, real correctness win)

`batch_aj_win_the_game.go:344` was the lone surviving direct `seat.Lost = true` setter flagged by §2-C of the structural analysis. Pre-fix, Triskaidekaphobia's "any seat at exactly 13 life loses the game" state-trigger skipped the §614 `would_lose_game` replacement chain entirely — **Platinum Angel and Angel's Grace were inert against this card's loss.**

Now routes through `gameengine.MarkSeatLostByEffect` (the canonical CR §104.3e chokepoint shipped 2026-05-29), which fires §614 first then stamps `LossReason` + `LostByEffect` + `Lost` atomically. The `losers[]` accumulator now reflects only seats whose loss actually applied (Platinum-Angel'd seats correctly stay out of the list).

**Regression:** `TestTriskaidekaphobia_PlatinumAngelCancelsLoss` pins the end-to-end fix — seat at 14 life with Platinum Angel does NOT lose when Triskaidekaphobia picks lose-1 mode. Existing Triskaidekaphobia test updated to expect the canonical `"card_effect: "` LossReason prefix + `LostByEffect=true`.

### PR #771 — Stale CLAUDE.md issue-log cleanup (doc-only)

Two stale findings retired from the Issue Log:

- **2026-05-28 Open row** ("9 per_card direct-Lost setters") moved to Resolved. The 2026-05-29 `MarkSeatLostByEffect` extraction closed 7 of 8 card-effect sites; PR #770 closed the lone batch_aj holdout. Verified 2026-05-30: `grep '\.Lost = true' internal/gameengine/per_card/**` yields only the intentional `helpers.go:84` `emitWin` opponent-lockstep (§104.2c win path).
- **2026-05-25 row** ("removePermanent sweep — 4 of 5 sites still standing") marked `[SUPERSEDED]` in place; new 2026-05-30 closure row added above. `etrata` / `bilbo` / `thassa` all now route through `gameengine.ExilePermanent` (verified 2026-05-30).

---

## What's next

Wave 1 is closed. The structural analysis sequences Wave 2 and Wave 3 as follows:

- **Wave 2 (medium-risk, biggest payoff):**
  - PR #1a — Export `MoveCardToZone`, sweep ~50 per_card files' manual graveyard appends (~−180 LOC subset).
  - PR #1b — Sweep library/hand/exile appends (~−120 LOC subset).
- **Wave 3 (counter-system migration close-out):**
  - PR #2 — `counters.InitializeCounter` API + 10-site migration (~−30 LOC + counter-doubler correctness).
  - PR #10 — Retire `sba.go:1190-1215` legacy `Counters[]` dual-path (~−15 LOC, only after PR #2 + verification sweep).

Wave 2 is the right next target — biggest LOC payoff (~−300 net) and biggest correctness win (silently-skipped descend / creature_died / exile counters + missed `FireZoneChangeTriggers` observers like Tergrid / Bolas's Citadel / Sefris / descend payoffs).
