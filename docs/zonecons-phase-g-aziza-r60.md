# InstanceID Phase G — Aziza spell-copy MintSpellCopy closure (r60)

**Branch:** `dev/zonecons-phase-g-token-disappearance-r60`
**Date:** 2026-05-30
**Predecessor:** PR #851 (Phase F — `MintSpellCopy` chokepoint + 10 sibling sites) + `dev/zonecons-phase-f-5k-verify-r60` (5K seed-42 verify @ commit `07252406` — surfaced this as the dominant residual)

## Headline

Closes the lone deferred §707.10 spell-copy fabrication site that the
Phase F audit named explicitly as Phase G scope. Pre-fix, Loki r60 seed
42 / 5,000 games reported **34 ZoneConservation violations across one
game** (game 2762 / Lash Out / `h1OGVR200056` / turns 44–60), all
keyed on Aziza, Mage Tower Captain's spell-copy handler aliasing the
originating `*Card` pointer directly into a §707.2 StackItem instead of
routing through the canonical `MintSpellCopy` chokepoint. Post-fix the
same gauntlet is bit-stable clean.

## Root cause

`internal/gameengine/per_card/aziza_mage_tower_captain.go`
`azizaSpellCopy` built the copy StackItem as:

```go
copyItem := &gameengine.StackItem{
    Controller: casterSeat,
    Card:       castCard,   // ← original *Card pointer
    IsCopy:     true,
    Targets:    originatingTargets,
    CostMeta:   map[string]interface{}{},
}
gs.Stack = append(gs.Stack, copyItem)
```

When the copy resolved, `stack.go:1312`'s §707.10 cease branch fired
`MarkInstanceIDCeased(gs, item.Card.InstanceID)`. But `item.Card` IS
the source `*Card` — so the source's InstanceID retired while the
underlying card was still in seat-N's hand / graveyard / wherever.
Every subsequent invariant tick walked seat N's zones, found Lash Out
present, and flagged it as fabrication.

This is structurally worse than the 10 sibling sites Phase F closed via
the same `MintSpellCopy` route (`alania` / `zada` / `krark` / `mica` /
`mendicant` / `rootha` / `kalamax` / `ivy` / `fire_lord_azula` /
`ulalek` — those at least called `DeepCopy()` first, sharing the ID via
inheritance). Aziza shared the pointer outright.

Audit confirms Aziza was the last such site: `grep "Card:\s*castCard"`
+ `grep "Card:\s*spellCard"` over `internal/gameengine/per_card/` finds
no other handlers pushing IsCopy=true StackItems with source-aliased
*Card pointers. The two remaining IsCopy handlers that alias source
pointers (`isochron_scepter.go`, `game_changers.go::Panoptic Mirror`)
route through `InvokeResolveHook` directly without pushing the item
onto `gs.Stack` — `stack.go:1312`'s cease branch never fires for those,
so the §707.10 leak shape doesn't apply.

## Fix

Route the copy through `MintSpellCopy` (Phase F's canonical chokepoint)
and push via `PushStackItem` (canonical stack push with audit logging):

```go
copyCard := gameengine.MintSpellCopy(gs, castCard)
copyItem := &gameengine.StackItem{
    Controller: casterSeat,
    Card:       copyCard,
    IsCopy:     true,
    Targets:    originatingTargets,
    CostMeta:   map[string]interface{}{},
}
gameengine.PushStackItem(gs, copyItem)
```

`MintSpellCopy` DeepCopys the source `*Card`, clears the inherited
InstanceID, mints a fresh CP-provenance ID with lineage pointing at the
source, and stamps `IsCopy=true` on the *Card. The §707.10 cease at
resolve then retires the COPY's ID, leaving the source intact.

## Loki r60 verification

**5,000 games / seed 42 / `--invariant zone-conservation` /
`--nightmare-boards 0` / `--instanceid-strict-census`:**

| Stage | Date | Total | Game 2762 (Lash Out / Aziza) | Other |
|-------|------|------:|-----------------------------:|------:|
| Phase F 5K verify (pre-Phase G) | 2026-05-30 | 34 | 34 | 0 |
| **Phase G (this PR)** | **2026-05-30** | **0** | **0** | **0** |

Verdict: `CLEAN — All 5000 chaos games passed all invariant checks
with zero crashes` (2m38s, 32 g/s). The 34-hit Aziza/Lash Out cluster
is genuinely closed at original-cluster depth.

### Cross-seed observation

A `--seed 43 --games 5000` cross-validation pass reports **38
violations / 1 game** — a different signature (`h0OGVC000099 Silk,
Web Weaver // Silk, Web Weaver` + `h0OGVC400098 Opaline Bracers`, same
fabrication shape across turns 38–60 of game 4557). This is bit-stable
identical pre- and post-Phase G — NOT a §707.10 spell-copy class, so
the MintSpellCopy fix doesn't address it. The signature suggests a
commander-DFC / equipment-cycling lineage gap (both cards on the same
seat, distinct OG IDs, both surviving past their *Card pointer being
spliced out). Queued separately as Phase H scope; out of band for the
Aziza closure.

## Regression tests

`internal/gameengine/per_card/aziza_spell_copy_r60_test.go`:

- `TestAziza_SpellCopy_DistinctInstanceID` — pins the structural
  property: after the trigger fires, the copy StackItem's `*Card` is
  freshly minted with its own InstanceID, not aliased to the source.
- `TestAziza_SourceIDSurvivesCopyResolution` — drives the end-to-end
  leak shape: after the copy is ceased per §707.10, the source's
  InstanceID must remain in `(Minted - Ceased)`. This is the exact
  invariant violation observed in game 2762.
- `TestAziza_InsufficientCreatures_NoCopy` — defends the cost-pay
  guard (no copy when fewer than 3 untapped friendly creatures).
- `TestAziza_OpponentCast_NoCopy` — defends the controller gate.

Updated existing r51 test `TestAziza_R51_CopiesSpellOnInstantCast`
from asserting the broken contract (`top.Card == castCard`) to
asserting the new correct contract (`top.Card != castCard` AND
`top.Card.Name == castCard.Name`).

## Audit-rule callout

Any future per_card handler that pushes an `IsCopy=true` StackItem onto
`gs.Stack` MUST route the copied `*Card` through `gameengine.MintSpellCopy`
from inception. Aliasing the source `*Card` pointer (with or without
`DeepCopy`) means the §707.10 cease at resolve will retire the SOURCE's
InstanceID, flagged by `checkZoneConservation` on every subsequent
invariant tick. Phase F closed 10 sites; Phase G closed the last known
one. Handlers that resolve copies via `InvokeResolveHook` directly
(Isochron Scepter, Panoptic Mirror) are exempt — they don't push the
item to the stack so §707.10 cease never fires.

— Loki r60 / Phase G / Aziza closure
