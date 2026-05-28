# Phase-Transition Stress Harness (r60 PR-C)

## Goal

PR-C from the OODA plan — deterministic phase-transition stress
harness that exercises edge cases Loki's chaos runner can't
deterministically hit at the rate needed to catch regressions
before they ship.

Loki samples random card mixes against the full turn loop, so
phase-step interactions surface stochastically and at low
density. Specific edge-case shapes (skip-untap effects layered
with extra combats, multi-modification cleanup, extra-turn
duration carry-over, cleanup-window trigger queuing) might fire
once per 10K-game gauntlet on a heavy seed — too sparse to catch
during PR review. Deterministic tests pin the CR-canonical
end-state for each shape and catch regressions on every CI run.

## What the harness covers

14 deterministic tests across the 5 charter stress axes, all in
`internal/gameengine/phase_transition_stress_r60_test.go`:

### (a) Skip-untap effects — Stasis / Frozen Aether / Sands of Time

| Test | Scenario | Pin |
|------|----------|-----|
| StasisSkipUntapStillClearsSummoningSickness | `SkipUntapStep=true` on seat | Creature stays tapped, summoning sickness still clears per §502.1 carve-out, audit event emitted |
| FrozenAetherPerPermanentSkipUntap | `Permanent.Flags["skip_untap"]=1` | Affected stays tapped; other seat permanents untap normally |
| DoesNotUntapFlagHonoredOnPermanent | `Permanent.DoesNotUntap=true` | Permanent stays tapped, `untap_skipped` event emitted with source name |

### (b) Multiple until-EOT modifications stacking + cleanup

| Test | Scenario | Pin |
|------|----------|-----|
| MultipleUntilEOTModificationsCleanedUp | 3 EOT mods + 1 permanent mod | All 3 EOT mods cleared in one pass, permanent mod survives |
| MarkedDamageWearsOffAtCleanup | Damage on multiple permanents | All `MarkedDamage` zeroed per §514.2 |
| GrantedAbilitiesClearedAtCleanup | EOT keyword grants | All `GrantedAbilities` cleared |

### (c) Extra combat phases — CR §500.7 untap-step absence

| Test | Scenario | Pin |
|------|----------|-----|
| ExtraCombatDoesNotProduceUntapStep | `AddExtraCombat(...)` | Extra combat queued; no `untap_step` / `untap_all` event fires (extra combats don't get untap steps) |
| SkipUntapAcrossExtraTurnNotDoubleApplied | Set→clear `SkipUntapStep` between turns | First untap skipped; second untap proceeds normally — single-use semantics preserved |

### (d) Extra-turn end-step duration carry-over

| Test | Scenario | Pin |
|------|----------|-----|
| ExtraTurnsPendingCounterStacks | 3× `extra_turns_pending++` | Counter accumulates to 3 (each extra-turn resolution stacks independently) |
| DurationUntilEndOfYourNextTurnSurvivesIntervening | Active seat owns the source | Duration does NOT expire at the CURRENT end step — expires at the NEXT seat-0 end step |
| UntilEndOfTurnExpiresAtCleanupNotEndStep | `until_end_of_turn` duration | Does NOT expire at `end_step`; DOES expire at `cleanup` per §514.2 (so end-step triggers see the mods) |

### (e) Cleanup-step trigger interactions per CR §514

| Test | Scenario | Pin |
|------|----------|-----|
| CleanupClearsTransientGameFlags | Fog + basilisk-class flags set | `prevent_all_combat_damage`, `basilisk_granted`, `basilisk_combat_hit`, `basilisk_marked_destroy` all clear; non-transient `was_cast` survives |
| CleanupIsIdempotent | Run cleanup twice | Second pass is a no-op; doesn't undo first pass or restore cleared state |
| DurationUntilNextEndStepExpiresAtNextEndStep | Cross-seat + same-seat | Duration expires at `end_step` regardless of controller/active alignment; doesn't fire AGAIN at `cleanup` |

## Why deterministic

Each test exercises a precise engine code path with a
hand-constructed `GameState` rather than running through the full
turn loop. Benefits:

  - **Speed**: 14 tests run in ~0.4s vs Loki's minutes-to-hours
  - **Reproducibility**: no seed-dependent variance; fail/pass is
    fixed per CR-correctness of the engine
  - **Granularity**: assertion failures point at the specific
    rule + the specific field that misfired, rather than "Loki
    found a violation in game 4773 at turn 39 with these 4 decks"
  - **CI coverage**: included in `go test ./internal/gameengine/`
    so every PR validates the contract

## What this catches

Regression scenarios this harness guards against:

  1. **Stasis-shape summoning-sickness regression** — engine
     forgetting to clear sickness during a skipped untap step
     would let creatures stay sick across multiple Stasis-locked
     turns. Caught by Test (a)#1.
  2. **Per-permanent skip-untap leak** — Static Orb / Frozen
     Aether's `skip_untap` flag affecting wrong permanents.
     Caught by Test (a)#2.
  3. **Multi-mod cleanup partial-clear** — an engine bug where
     only the first EOT modification gets cleared at cleanup
     would compound across turns. Caught by Test (b)#1.
  4. **Marked-damage carry-over** — combat damage persisting past
     cleanup would let creatures silently die on subsequent SBA
     passes. Caught by Test (b)#2.
  5. **Extra-combat untap inflation** — World at War / Aggravated
     Assault unintentionally triggering an untap pass per combat
     would enable infinite-mana exploits. Caught by Test (c)#1.
  6. **Single-shot skip-untap re-applying** — Stasis-style effects
     persisting beyond their intended one-step duration would
     soft-lock the active seat. Caught by Test (c)#2.
  7. **Extra-turn counter overwrite** — multiple extra-turn
     spells collapsing into one extra turn instead of stacking.
     Caught by Test (d)#1.
  8. **Duration premature expiry** — "until end of your next
     turn" effects expiring at the CURRENT turn's end step would
     cut off control-change scope (Threaten / Act of Treason)
     early. Caught by Test (d)#2.
  9. **Duration step misordering** — "until end of turn" effects
     expiring at end_step instead of cleanup would break end-
     step-trigger evaluation. Caught by Test (d)#3.
  10. **Cleanup flag leak** — Fog's `prevent_all_combat_damage`
      persisting past cleanup would prevent combat damage on
      subsequent turns. Caught by Test (e)#1.
  11. **Non-idempotent cleanup** — double-cleanup undoing the
      first pass's clears. Caught by Test (e)#2.
  12. **End-step duration mis-window** — `until_next_end_step`
      firing at cleanup instead of end_step (or both) would
      cause double-expiry. Caught by Test (e)#3.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git checkout dev/phase-transition-stress-r60
go test -run TestPhaseTransitionStress -count=1 -v ./internal/gameengine/
```

Expected: `PASS` on all 14 tests in ~0.4s.

## Verdict

The engine is **CR §502 / §514 / §500.7 / §613 duration-tag
compliant** across all 14 deterministic stress scenarios. No real
bugs surfaced — every scenario produces the CR-canonical
end-state on the first try. The harness now stands as a
permanent regression test for these specific phase-transition
edge cases, complementing Loki's stochastic full-game coverage.

## CR references

- **§500.7** — Extra combat phases / steps don't include an
  untap step (only the canonical first combat phase of a turn
  does).
- **§502.1** — Untap step semantics; summoning-sickness clear
  even when the step is skipped.
- **§502.2** — "Does not untap" permanents stay tapped.
- **§513.1** — End step is the FIRST step of the ending phase.
- **§514.2** — Cleanup step: damage zeroes, "until end of turn"
  effects end, hand-size enforcement runs.
- **§702.171** — Saddled wears off at end of turn (folded into
  cleanup with other EOT clears).
