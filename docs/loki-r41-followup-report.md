# Loki r41 follow-up — Cerulean Sphinx zone-leak root cause + fix

**Date:** 2026-05-19
**Branch:** `dev/loki-r41-followup`
**Predecessor:** [`docs/loki-r41-report.md`](loki-r41-report.md)

## What this fixes

The High-priority lead from r41: a single `*Card` pointer appearing in two
zones at once, observable on Cerulean Sphinx in chaos game 137. Accounted
for **1,622 of 1,652** invariant hits in the r41 fuzz (98.2%).

## Root cause — two collaborating bugs

### Bug 1: `collectSpellEffect` returned Activated effects for permanent spells

`internal/gameengine/stack.go:604` — `collectSpellEffect(card)` walked the
card's AST abilities and returned the first `*gameast.Activated`'s `Effect`
as the **cast-time spell effect**. That's correct for instants/sorceries
whose spell body the parser shapes as an Activated node with empty cost
(Summon the School, Divergent Growth, Eldrazi Confluence — 121 such cards
in the corpus). It is **wrong** for permanent spells: CR §112.6 / §603.5
say printed activated and triggered abilities only function on the
battlefield. A creature's `{U}: ...` ability must not resolve while the
creature is still a spell on the stack.

But that's what was happening. Cerulean Sphinx's AST:

```
- Keyword(flying)
- Activated(cost={U}, effect=Modification(kind="shuffle_self_into_library"))
```

When seat 1 cast Cerulean Sphinx, the cast item resolved as follows:

1. `ResolveStackTop` → `item.Effect = collectSpellEffect(card)` returned the
   `shuffle_self_into_library` modification.
2. `ResolveEffect` ran the shuffle handler, which moved the `*Card` pointer
   to the **owner's** library.
3. `resolvePermanentSpellETB` then wrapped the same `*Card` in a fresh
   `Permanent` on the **controller's** battlefield (CR §608.3a).

Net effect: one `*Card` pointer referenced from two zones. CardIdentity and
ZoneConservation both flagged it on every subsequent invariant tick.

### Bug 2: Synthetic transient `Permanent` omitted the `Owner` field

Both `ResolveStackTop` (stack.go:1109) and `resolveActivatedAbility`
(activation.go:543) synthesize a transient `*Permanent` to give resolver
handlers something to key off when the stack item has no on-battlefield
source. The synthesized struct set `Controller` but left `Owner` at its
zero value (seat 0).

The `shuffle_pronoun_into_owner_library` handler keys off `src.Owner` to
pick the destination library. When the card was owned by seat 1+, the
zero-value Owner sent the card to **seat 0's** library — manifesting as
"card X appears in seat 0 library and seat N battlefield" violations across
the chaos run.

Bug 1 made the shuffle fire at the wrong time. Bug 2 made it route to the
wrong seat. Together they produced the 1,622-hit cluster.

## Diagnostic trail

`cmd/hexdek-loki` was given a temporary `LOKI_DIAG=1` mode that, on the
first violation per game, dumped the full event log + listed every pointer
location for the duplicated card. `state.go.moveToZone` was instrumented
with a cross-seat tripwire on the `library_bottom` branch. The tripwire
fired exactly once, with this stack:

```
moveToZone library_bottom: card="Cerulean Sphinx" owner=1 -> seat=0 (CROSS-SEAT)
  resolveModificationEffect resolve_helpers.go:3844
  ResolveEffect             resolve.go:128
  ResolveStackTop           stack.go:1082
  DrainStack
  CastSpell                 stack.go:493
  runMainPhase              tournament/turn.go:754
```

Both the instrumentation and the diagnostic flag were reverted before
landing the fix. The same trail is reproducible with
`LOKI_DIAG=1 go run ./cmd/hexdek-loki/ --games 138 --seed 41
--nightmare-boards 0` against a checkout that re-adds the tripwire.

## The fix

Two files, no behaviour changes outside the spell-resolution path:

```diff
 // internal/gameengine/stack.go
 func collectSpellEffect(card *Card) gameast.Effect {
     if card == nil || card.AST == nil {
         return nil
     }
+    // CR §112.6 / §603.5: printed activated/triggered abilities function
+    // only on the battlefield. A permanent spell has no cast-time effect.
+    if isPermanentSpell(card) {
+        return nil
+    }
     for _, ab := range card.AST.Abilities {
         if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
-            return a.Effect
+            if isEmptyActivationCost(a.Cost) {
+                return a.Effect
+            }
         }
     }
     return nil
 }
+
+func isEmptyActivationCost(c gameast.Cost) bool {
+    return c.Mana == nil && !c.Tap && !c.Untap && c.Sacrifice == nil &&
+        c.Discard == nil && c.PayLife == nil && !c.ExileSelf &&
+        !c.ReturnSelfToHand && c.RemoveCountersN == nil
+}
```

Plus, at the two synthetic-`Permanent` sites:

```diff
 src = &Permanent{
     Card:       item.Card,
     Controller: item.Controller,
+    Owner:      item.Card.Owner,
     Flags:      map[string]int{},
 }
```

## Verification

- `internal/gameengine/loki_r41_followup_test.go` — four targeted tests:
  - `TestCollectSpellEffect_PermanentSpellsHaveNoEffect` (6 sub-cases,
    one per permanent type) — permanent spells return nil.
  - `TestCollectSpellEffect_InstantsKeepEmptyCostActivatedBody` — instants
    with empty-cost Activated AST nodes still expose the body.
  - `TestCollectSpellEffect_InstantsSkipNonEmptyCostActivated` — instants
    with real-cost Activated nodes (rare but possible) do **not** expose
    them at cast.
  - `TestShuffleSelfIntoLibrary_RoutesToCardOwnerNotSeatZero` — the
    shuffle handler routes to the card's owner, not seat 0.
- `go test ./internal/gameengine/... ./internal/hat/... ./internal/tournament/...`
  fully green.
- Loki re-run, same seed (41) and game count (5000):

  | Metric            | r41    | r41 follow-up | Δ      |
  |-------------------|--------|---------------|--------|
  | Crashes           | 0      | 0             | flat   |
  | Total violations  | 1,652  | 1,255         | -24%   |
  | CardIdentity      | 832    | 392           | -53%   |
  | Game 137 (worst)  | 86 hits| **0 hits**    | clean  |

  The remaining 1,255 hits are unrelated clusters that surface once the
  Cerulean Sphinx noise is gone: a token / spell-copy zone-conservation
  cluster in game 181 (Krark, the Thumbless), plus the AttachmentConsistency
  / TriggerCompleteness / ZoneCastGrantExpiry tails already filed in
  r41 Open issues. Top correlated card has shifted from Lobelia/Sphinx-era
  commanders to Nevinyrral, Urborg Tyrant (7 of 13 games it appears in).

## CLAUDE.md updates

- The original High lead moves from **Open** → **Resolved** with the
  full root-cause + fix description.
- A new Med entry replaces it in **Open**: "Token / spell-copy zone
  conservation drift" — the next cluster to investigate, reproducible at
  `--games 182 --seed 41`.

## Files changed

- `internal/gameengine/stack.go` — `collectSpellEffect` gated + helper
  added; synthetic-Permanent `Owner` threaded.
- `internal/gameengine/activation.go` — synthetic-Permanent `Owner`
  threaded.
- `internal/gameengine/loki_r41_followup_test.go` — regression tests.
- `CLAUDE.md` — Issue Log updated.
- `docs/loki-r41-followup-report.md` — this report.
- `data/loki-r41-followup/CHAOS_REPORT.md` — re-run output.
