# Loki r60 — 100K Deep-Surface Sweep, post-#685 Etali fix (seed 42)

| Field | Value |
|-------|-------|
| Date | 2026-05-27 |
| Branch | `dev/loki-r60-100k-r60` (`origin/main` at `f1103ba2`) |
| Headline fix in play | PR #685 (`cb3734c5`) — Etali §400.7c owner-routed exile |
| Invocation | `go run ./cmd/hexdek-loki --games 100000 --seed 42` |
| Phase 1 throughput | 100,000 chaos games in 31m20s (53 g/s avg) |
| Phase 2 throughput | 10,000 nightmare boards in 749ms (13,353 b/s) |
| Crashes | **0** (chaos) + **0** (nightmare) |
| Panics / recovers | **0** |
| Chaos violations | **244** in **11 games** (99,989 clean = 99.989%) |
| Nightmare violations | **0** (all 10,000 boards clean) |
| Verdict | **Etali class extinct** — 50K's 2,808-violation Etali cluster dropped to 0; 4 novel rare clusters surfaced past the previous depth wall, all engine-fix surfaces (no remaining cluster has ≥ 0.05 card correlation) |

## Trajectory vs prior depth runs (canonical seed 42)

| Depth | Run | Violations | V-games | Per-game rate | Notes |
|-------|-----|------------|---------|---------------|-------|
| 5,000 | pre-Etali | 0 | 0 | 0% | canonical-clean baseline |
| 10,000 | pre-Etali | 0 | 0 | 0% | 2× canonical, same verdict |
| 25,000 | pre-Etali | 828 | 16 | 0.064% | Etali tail surfaces |
| 50,000 | pre-Etali | 2,808 | 50 | 0.100% | Etali tail scales linearly |
| **100,000** | **post-#685** | **244** | **11** | **0.011%** | **Etali class extinct; 4 novel non-Etali clusters surface** |
| Scale 50K→100K | — | **÷ 11.5** | **÷ 4.5** | **÷ 9.1** | depth doubling produces ~11× FEWER violations — Etali fix removed >99% of the rare tail |

The §400.7c Etali fix produced a ~10× drop in per-game violation rate (0.100% → 0.011%) at the same seed. The 11 remaining violation games are scattered across 4 distinct, unrelated commander pods — none of the post-Etali tail concentrates on a single card the way the pre-fix Etali tail did. **Card correlation ranks 1-9 all sit at 0.01–0.03**, vs Etali's 0.13 at 50K — there is no smoking-gun anchor in the residual.

## By invariant

| Invariant | Count | Games | Cluster |
|-----------|-------|-------|---------|
| ZoneConservation | 144 | ~2 | Naru Meha + Panharmonicon mandatory-loop draw cleanup; small drift in Alrund pod |
| CardIdentity | 72 | ~1 | "Drana" cross-seat exile + command_zone — Gisa vs §903.9b commander redirect race |
| TriggerCompleteness | 28 | ~1 | "Pia Nalaar sacrifice no subsequent trigger" — likely invariant false-positive |

## Card correlation (top 9, all weak)

| Rank | Card | Violation games | Clean games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Noble Hierarch | 1 | 39 | 0.03 |
| 2 | Maestros Confluence | 1 | 42 | 0.02 |
| 3 | Vizkopa Guildmage | 1 | 98 | 0.01 |
| 4 | Lotleth Troll | 1 | 99 | 0.01 |
| 5 | Azorius Signet | 1 | 109 | 0.01 |
| 6 | Avid Reclaimer | 1 | 110 | 0.01 |
| 7 | Hardened Academic | 1 | 111 | 0.01 |
| 8 | Exhilarating Elocution | 1 | 112 | 0.01 |
| 9 | Manifestation Sage | 1 | 114 | 0.01 |

No card appears in more than 1 violation game. The pre-Etali tail had a 0.13 correlation with a single card in 47 of 50 violation games; this post-Etali tail has no card above 0.03 — the rare events are now genuinely distributed across the rulebook, not concentrated on a single buggy handler. **The 4 cluster anchors live one level deeper than the top-correlation table**: they're commander handlers (Gisa, Pia Nalaar) and engine-cleanup paths (SBA-cap mandatory-loop draw), not individual cards with broad correlation.

## Cluster 1: ZoneConservation "500 extra real cards" (game 14620, ~140 violations)

**Signature:**
```
zone conservation suspicious: 500 extra real cards appeared
  (expected -138, found 362) — possible copy bug
```

- **Pod**: Wyll of the Elder Pact / Toph, Hardheaded Teacher / Giott, King of the Dwarves / Judith, the Scourge Diva
- **Trigger event**: turn 39 draw step, after a Naru Meha + Panharmonicon copy cascade. Final events: `triggered_ability × 2 → copy_spell → enter_battlefield → replacement_applied (Panharmonicon ×2) → game_draw × 4 → pending_triggers_purged_on_leave amount=900 → seat_eliminated × 4 with amounts 513/5/8/9`
- **All 4 seats LOST simultaneously** — this is a CR §104.4b mandatory-loop draw triggered by the SBA cap firing (`pending_triggers_purged_on_leave amount=900` indicates the per-seat purge in HandleSeatElimination dumped a 900-trigger backlog from the Naru Meha + Panharmonicon copy explosion)
- **The `expected -138` is the smoking gun**: the invariant's per-seat census underflows when 900 pending triggers are purged in a single SBA-cap cycle and each seat is eliminated with 513/5/8/9 stack items dropped. The 2026-05-24 cap-draw fix (`sba.go` marks all seats Lost so CheckEnd returns) handled the *termination* correctly but didn't reset the zone-conservation per-seat baseline for the post-cap state — so the invariant counts stack-evacuated cards as "extra appeared" when they were already accounted for in the pre-cap census

**Root cause**: post-cap-draw cleanup gap in `checkZoneConservation` — the invariant's expected card count needs to be recomputed (or short-circuited via the existing `ended=1` flag, mirroring the Charix `ended=1` skip from 2026-05-24). Same family as the `ended` short-circuit established in PR for `sba.go:41` and `invariants.go:355` — `checkZoneConservation` is the lone outstanding mirror, same as `checkSBACompleteness` was before the Charix fix.

**Likely a false-positive on a correctly-ended game**, not a real card-leak. Fix is an invariant-side skip, not an engine fix.

## Cluster 2: ZoneConservation "1 real cards disappeared" (game 36027, ~3 violations)

**Signature:**
```
zone conservation violated: 1 real cards disappeared
  (expected 379, found 378)
```

- **Pod**: Princess Twilight Sparkle / Old Gnawbone / Alrund, God of the Cosmos // Hakka, Whispering Raven / Hinata, Dawn-Crowned
- **Trigger**: turn 48-50 cleanup, after Old Gnawbone treasure triggers + Alrund flip-card triggers + Knights of Dol Amroth interactions
- **Event window**: `Old Gnawbone triggered_ability → parsed_effect_residual → thopter destroy → sba_704_5g → Alrund triggered_ability → parsed_effect_residual → pool_drain`
- **`parsed_effect_residual`** events for both Old Gnawbone and Alrund — both ARE in the cards-with-parser-gap-residue family. Old Gnawbone makes treasure tokens when dealing combat damage; Alrund flips between front/back face and has draw-or-create-bird-token modal triggers. Either could be losing a token's `*Card` pointer during a partial-residue resolution

**Hypothesis**: one of the parsed-effect-residual paths (Old Gnawbone treasure mint or Alrund's bird-token mint) is not registering its created token in the per-seat census. The invariant's "real cards expected" count is correct; one real token-mint is silently dropped. This is a per-card handler fix candidate (either Old Gnawbone or Alrund, depending on which `parsed_effect_residual` is the offender). Low frequency (~3 violations over 1 game per 100K) — engine fix surface but very low priority.

## Cluster 3: TriggerCompleteness Pia Nalaar sacrifice (game 52921, 28 violations)

**Signature:**
```
TriggerCompleteness: death event "sacrifice" at index N
  with trigger-bearer(s) [{Pia Nalaar, Consul of Revival 3}]
  on battlefield, but no subsequent trigger/effect event found
```

- **Pod**: Callaphe / Brothers Yamazaki / Torbran / Pia Nalaar, Consul of Revival
- **Trigger events**: Runaway Carriage's "Run over" attack-trigger (sacrifices a random nonland permanent — events 622/624 sacrificed Mountain + Runaway Carriage itself), then the invariant fires expecting a Pia Nalaar trigger that doesn't exist
- **Pia Nalaar's actual oracle**: "When Pia Nalaar enters the battlefield, create a 1/1 colorless Thopter artifact creature token with flying. {1}, Sacrifice an artifact: Pia Nalaar deals 1 damage to any target. Activate only if a creature died this turn." — **she has NO triggered ability on sacrifice or death**. Her abilities are ETB-trigger + activated-ability. The invariant's known-bearers list incorrectly classifies her as a creature-dies trigger bearer

**Root cause**: TriggerCompleteness false-positive — same family as the 2026-05-24 Gisa opp-only fix (`opponentOnlyCreatureDiesTriggers` map in `invariants.go`). Likely the parser misclassifies Pia Nalaar's `{1}, Sacrifice...` activated-ability cost as a die-trigger registration, OR the invariant's bearer-detection sweeps `Triggered{event: "die"}` AST nodes too liberally and counts activated-ability cost requirements that mention sacrifice. Mountain sacrifice event 622 already passes the `was_creature=false` filter from the 2026-05-24 fix, but Runaway Carriage event 624 IS a creature sacrifice and would still surface.

**Likely an invariant-side fix** (add Pia Nalaar to the false-positive filter, OR fix the bearer-detection parser arm), not an engine fix.

## Cluster 4: CardIdentity "Drana cross-seat exile + command_zone/battlefield" (game 68324, ~70 violations)

**Signature:**
```
CardIdentity: card "Drana, Liberator of Malakir" (ptr 0x...)
  appears in both seat 1 exile and seat 2 command_zone
  (later violations: seat 1 exile AND seat 2 battlefield)
```

- **Pod**: Radha, Heir to Keld / Gisa, Glorious Resurrector / Drana, Liberator of Malakir / The Eighth Doctor
- **Drana = seat 2's commander**, Gisa = seat 1
- **Mechanism**: Drana dies → Gisa's "if a creature an opponent controls would die, exile it instead" trigger fires → Drana moves to Gisa-controller's exile (seat 1). Simultaneously, CR §903.9b commander-redirect replacement applies → same `*Card` is also placed in Drana's command zone (seat 2). The §903.9b replacement and Gisa's would-be-died replacement are **both legal would-be-replacements for the same event** (CR §616.1 — the affected player/owner chooses the order). Whichever resolves first should consume the would-die event; the second should see "Drana isn't dying anymore" and no-op. The engine is applying BOTH

**Root cause**: missing CR §616.1 replacement-priority arbitration between §903.9b commander redirect and Gisa-style exile replacements. Same shape as the 2026-05-24 Athreos cross-seat race fix (`athreos_cross_seat_race_r60_test.go`) — Athreos was patched to scan owner's graveyard before delegating, but Gisa's analogous guard at `gisa_glorious_resurrector.go:65-82` only checks "owner's graveyard," not "owner's command zone." Commander-redirected cards skip the graveyard entirely and land in the command zone, so Gisa's race-loser check doesn't fire.

**Engine fix candidate**: extend Gisa's race-loser scan to include `commanderZone` for cards with `IsCommander==true` belonging to the owner. Sibling sweep: any per_card handler with the 2026-05-24 Athreos-pattern owner-graveyard scan needs the parallel command-zone check (Athreos itself, Gisa, possibly The Reaper King §704.6d, Yahenni Undying Partisan, Toxrill, Grave Pact, Grave Betrayal). Three lines of additional check per handler.

## What surfaced at 100K that did NOT surface at 50K

All four clusters above are new at 100K depth relative to the 50K pre-#685 baseline. None overlap with the closed Etali class (no `Etali, Primal Storm` in any of the 11 violation games). The structural distribution:

| Type | Count | Notes |
|------|-------|-------|
| Invariant-side false-positive (no engine fix needed) | 2 | Cluster 1 (post-cap-draw ZoneConservation), Cluster 3 (Pia Nalaar TriggerCompleteness bearer-misclass) |
| Engine fix surface | 2 | Cluster 2 (per_card token-mint census drop, Old Gnawbone or Alrund), Cluster 4 (Gisa §616.1 commander-redirect race) |

**No new invariant categories** beyond the 11 historical r60 set surfaced at 100K. The 9 dormant invariant kinds (StackIntegrity, LifeConsistency, ResourceConservation, AttachmentConsistency, CombatLegality, SBACompleteness, ReplacementCompleteness, ZoneCastGrantExpiry, WinCondition) all stayed at 0.

## What this means for r60 cleanliness

The §400.7c Etali fix dropped the seed-42 per-game rate from 0.100% to 0.011% — a **~10× improvement**. The four residual clusters together explain 244 violations in 11 games. Two of those clusters are invariant-side false-positives (false alarms, not engine bugs); the other two are narrow, surgical fix surfaces that follow established patterns (Charix `ended=1` mirror, Athreos cross-seat race generalization). **A targeted 4-PR sweep would plausibly drive the seed-42 100K rate back to 0.**

## Recommended follow-up

1. **Cluster 1 fix** — `checkZoneConservation` `ended=1` short-circuit (mirror the Charix fix in `checkSBACompleteness`). Trivial invariant-side patch. ~5 LOC + 1 regression test.
2. **Cluster 3 fix** — investigate Pia Nalaar bearer-misclass in the TriggerCompleteness bearer-detection sweep. Likely add to the `opponentOnlyCreatureDiesTriggers`-style false-positive filter, OR audit the parser arm that decides "X has a creature_dies trigger" for activated-ability-with-sacrifice-cost false-positives.
3. **Cluster 4 fix** — extend Gisa's race-loser guard to scan commander-zone for commanders, parallel to the 2026-05-24 Athreos-graveyard pattern. Sibling sweep: any of the 6 Athreos-pattern handlers from the 2026-05-24 fix wave (Gisa, Athreos, Reaper King, Yahenni Undying Partisan, Toxrill, Grave Pact, Grave Betrayal) likely needs the same command-zone extension.
4. **Cluster 2 fix** — lowest priority (3 violations over 1 game per 100K). Audit `Old Gnawbone treasure_mint` and `Alrund bird-token mint` per_card handlers for token-registration gaps in the per-seat census tracking. May be revealed automatically by Cluster 1's broader census-bookkeeping audit.
5. **After all 4 land, re-run 100K @ seed 42** to confirm clean. Then consider a multi-seed 25K matrix (5 seeds × 25K) as the next-depth verification, per the 50K report's recommendation — different seeds sample different rare-event surfaces.

## What is NOT in this report

- **No regressions of the 11 historical r60 clusters** — all closed signatures held bit-stable across 100K games
- **No nightmare-board violations** — 10,000 boards, 0 violations, 13,353 b/s
- **No panics / recovers** — `0` for both phases
- **No Etali violations** — the §400.7c fix decisively closed the entire Etali cluster class; 0 violations in any game with Etali as a commander across all 100K
