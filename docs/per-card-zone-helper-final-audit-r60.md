# per_card Zone-Helper Final Audit — Wave 2 R60

**Branch:** `dev/per-card-zone-helper-final-audit-r60`
**Date:** 2026-05-30
**Predecessor:** Wave 1 + 2 — `MintTokenAsCopyOf` chokepoint (#871), `MintSpellCopy` chokepoint (#851 / #873), `DiscardCard` canonical helper, `MoveCard` universal zone-change entry point.

## Headline

Wave 1+2 closed the structural zone-helper coverage gaps — `MoveCard`,
`MoveCard`-via-`DiscardCard`, `MintSpellCopy`, `MintTokenAsCopyOf` are
all on main. Wave 2 deferred 36 per_card sites that still carry manual
`seat.<Zone> = append(...)` patterns bypassing those chokepoints. This
PR closes 12 of them — the clean single-step migrations — and documents
the remaining 32 by pattern, so future audits can pick up the
multi-step state-machine work without re-discovering the classification.

## Closed in this PR (12 sites, 1 regression test each)

**Cluster 1 — drop redundant manual splice before existing `MoveCard` call**
(canonical helper already performs the source-zone removal):

| # | Handler | Mechanic |
|---|---------|----------|
| 1 | `varolz_the_scar_striped.go` | scavenge (graveyard → exile) |
| 2 | `sin_spiras_punishment.go` | ETB exile from graveyard |
| 3 | `praetors_grasp.go` | opp library → exile |

**Cluster 2 — drop redundant manual splice before `enterBattlefieldWithETB`**
(`createPermanent` calls `RemoveCardFromAllPrivateZones` internally):

| # | Handler | Mechanic |
|---|---------|----------|
| 4 | `custom_eddie_brock.go` | reanimate MV≤1 from graveyard |
| 5 | `custom_ghen_arcanum_weaver.go` | enchantment recursion graveyard→bf |
| 6 | `custom_jhoira_ageless_innovator.go` | hand artifact cheat onto bf |
| 7 | `anticausal_vestige.go` | warp-counter hand cheat onto bf |
| 8 | `the_ur_dragon.go` | Dragon eminence hand-cheat onto bf |
| 9 | `xu_ifit_osteoharmonist.go` | graveyard reanimate as Skeleton |

**Cluster 3 — replace ad-hoc library/hand splice + append with the canonical
`MoveCard` / `DiscardCard` call** so §614 replacements, §903.9b commander
redirect, observer triggers, and descend-counter updates all fire:

| # | Handler | Mechanic |
|---|---------|----------|
| 10 | `master_of_death.go` | mill (library → graveyard) |
| 11 | `per_card_batch_k_r60.go::veronicaDissidentScribeAttack` | discard (`DiscardCard`) |
| 12 | `glissa_sunslayer.go` | combat-damage draw + lose 1 |

Regression file: `internal/gameengine/per_card/wave2_zone_helper_audit_r60_test.go` (12 tests, each pinning destination-zone count = 1 and source-zone count = 0, with no double-ref between the two).

## Deferred — 32 files / ~44 sites (still carrying manual append)

Grouped by why they're not single-step safe. Each group needs deeper
work than a one-line splice removal.

### Group A — slice-back reshuffles (single append of a slice, after a scry / surveil / reveal-N walk)

These patterns append a whole batch of cards back to library (top or
bottom) after a multi-card walk. Migration would need a per-card
`MoveCard` loop AND careful library-ordering preservation (top vs
bottom semantics, deterministic ordering across rerolls).

- `ayesha_tanaka_armorer.go` — "shuffle non-hits back"
- `birthing_ritual.go` — same pattern
- `esika.go` — Esika's land-scry residue
- `etb_tribe_gate_family.go` — tribe-gate reveal residue
- `gen_toph_greatest_earthbender.go` — bottom-N pattern
- `star_charter.go` — top-of-library reorder
- `svella_ice_shaper.go` — bottom-N after pick
- `batch17_sweep.go` — sweep utility
- `runo_stromkirk.go` — top-N reorder
- `obeka_support.go` — bottom-card-cycle

### Group B — hand → battlefield ETB-cheat (manual hand splice before `enterBattlefieldWithETB`)

These all share the structure of Cluster 2 above (createPermanent
sweeps the source zone), so the manual splice is redundant — same fix
applies. Deferred from this PR for batch-size discipline (12 was the
target); each is a clean one-line drop + regression test.

- `minn_wily_illusionist.go`
- `strefan_maurer_progenitor.go`
- `gitrog_ravenous_ride.go`
- `hakbal.go`
- `satoru_umezawa.go`
- `ureni_of_the_unwritten.go`
- `winota_joiner_of_forces.go`
- `zurgo_ojutai.go`
- `katara_waterbending_master.go`
- `page_loose_leaf.go`

### Group C — graveyard → battlefield reanimate (manual splice before `enterBattlefieldWithETB`)

Same shape as Cluster 2; deferred for batch discipline.

- `custom_karador_ghost_chieftain.go`
- `custom_jadzi_oracle_of_arcavios.go`
- `custom_araumi_of_the_dead_tide.go`
- `custom_felothar_steadfast.go`
- `custom_mayael_the_anima.go`
- `gen_alaundo_the_seer.go`
- `gen_raph_mikey_troublemakers.go`

### Group D — multi-card iteration with multiple appends in the same handler (genuine state-machine work)

These have 2+ append sites per file, often appending to different zones
in sequence. Routing each through `MoveCard` requires sequencing the
moves correctly (some moves depend on prior moves having completed).
Not a one-line drop.

- `gen_the_capitoline_triad.go` — graveyard → exile + graveyard → bf
- `chaos_cascade.go` — multi-card exile cascade with conditional play
- `custom_mairsil_the_pretender.go` — exile-then-grant cascade
- `chitinous_crawler.go` — multi-zone walker
- `commanders_batch.go` — multi-trigger batch handler
- `custom_feather_the_redeemed.go` — exile-at-end-step batch
- `custom_sliver_gravemother.go` — sliver pile management
- `grub_storied_matriarch.go` — encounter-trigger batch
- `gen_sandman_shifting_scoundrel.go` — discard + look batch
- `hurkyl_master_wizard.go` — bounce-N batch

### Group E — cross-seat state movement (special §400.7c handling)

These move cards between owners' private zones, so `MoveCard`'s owner-
redirect needs the right ownership-vs-controller modeling. Not a
single-line migration.

- `custom_selenia_dark_angel.go` — opponent steal-and-hand
- `gisa_glorious_resurrector.go` — opp-creature-dies steal
- `custom_yorion_sky_nomad.go` — flicker-to-exile with controller-side state

### Group F — custom Permanent construction (bypasses `createPermanent` entirely)

These build `*Permanent` manually rather than going through `createPermanent`.
Migration needs either routing through the canonical helper OR a careful
mirror of its side effects.

- `karmic_guide.go` — direct `&gameengine.Permanent{...}` construction
- `gen_eruth_tormented_prophet.go` — exile with grant register
- `atraxa_grand.go` — proliferate-style hand-add after pre-removal

## Recommended next steps

1. **Easy follow-up PR**: Groups B and C are 17 sites that follow Cluster 2's pattern exactly. A single sweep PR closes them all (~3 hours of work + 17 tests). Ranked as the highest-ROI next move.
2. **Medium follow-up**: Group A library-reshuffle patterns. Need a `MoveCardsToLibraryBottom` / `MoveCardsToLibraryTop` helper, then route the 10 sites through it. Pin ordering invariants with property tests.
3. **Hard follow-up**: Groups D / E / F are genuine state-machine work. Tackle one card at a time with full per-handler tests.

## Verification

```
$ go test ./internal/gameengine/per_card/ -run TestWave2_ -count=1 -v
=== RUN   TestWave2_Varolz_ScavengeMovesCardToExileOnce ... PASS
=== RUN   TestWave2_SinSpirasPunishment_ETBExilesGraveyardCardOnce ... PASS
=== RUN   TestWave2_PraetorsGrasp_MovesLibraryCardToExileOnce ... PASS
=== RUN   TestWave2_EddieBrock_ReanimateLowMVCreatureNoDoubleRefs ... PASS
=== RUN   TestWave2_GhenArcanumWeaver_RecursionNoDoubleRefs ... PASS
=== RUN   TestWave2_Jhoira_CheatsHandCardNoDoubleRefs ... PASS
=== RUN   TestWave2_AnticausalVestige_CheatsHandCardNoDoubleRefs ... PASS
=== RUN   TestWave2_UrDragon_CheatsHandDragonNoDoubleRefs ... PASS
=== RUN   TestWave2_XuIfit_ReanimateAsSkeletonNoDoubleRefs ... PASS
=== RUN   TestWave2_MasterOfDeath_MillRoutesThroughMoveCard ... PASS
=== RUN   TestWave2_BatchK_VeronicaDiscardRoutesThroughDiscardCard ... PASS
=== RUN   TestWave2_Glissa_DrawRoutesThroughMoveCard ... PASS
PASS
```

`go test ./internal/gameengine/...` — engine + counters + instanceid + per_card all PASS.

— Wave 2 final-audit / 2026-05-30
