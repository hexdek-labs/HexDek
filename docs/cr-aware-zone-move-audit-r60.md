# CR-Aware Zone-Move API Audit (r60)

**Trigger:** PR #685 (commit `cb3734c5`) closed an 828-violation Loki cluster
in `etali_primal_storm.go` that rooted in a hand-routed cross-seat exile move.
The fix is correct, but the API surface that let the bug exist is unchanged
— a per-card handler could still bypass §400.7 / §404.1 / §406.1 ("zone X is
owned by its owner") by passing the wrong `seat` arg to `moveToZone` or
manually appending to `gs.Seats[X].Exile`. This audit catalogs the engine's
zone-move surface, identifies CR-section enforcement points that should live
at the API boundary, and proposes the safest 1-2 to implement now.

## Surface inventory

Public API (callers across `internal/gameengine/**` and `per_card/**`):

| Function | File | Purpose | CR scope |
|---|---|---|---|
| `MoveCard(gs, card, ownerSeat, fromZone, toZone, reason) MoveResult` | `zone_move.go:59` | Universal non-battlefield zone change. Wraps `FireZoneChange` (§614 replacements + §903.9b commander redirect) and `FireZoneChangeTriggers`. | §614, §903.9, §400.7, §603.10 |
| `DestroyPermanent(gs, perm, source) bool` | `zone_change.go:43` | Battlefield → graveyard with §701.7 + §122.1b shield + §702.12b indestructible gates. | §701.7, §702.12b, §122.1b, §614, §403.4 |
| `ExilePermanent(gs, perm, source) bool` | `zone_change.go:169` | Battlefield → exile with `would_be_exiled` replacements. Bypasses indestructible per §701.20a (exile is not destroy). | §406, §614, §403.4 |
| `sacrificePermanentImpl(gs, perm, source, reason) bool` | `zone_change.go:243` | Battlefield → graveyard with §701.17 + §614 "would die". Ignores indestructible per §701.17b. | §701.17, §614, §403.4 |
| `BouncePermanent(gs, perm, source, dest) bool` | `zone_change.go:360` | Battlefield → hand / library top / bottom. | §701.8, §614, §403.4 |
| `(gs *GameState).moveToZone(seat, c, zone)` | `state.go:1574` | Low-level zone-list append. Idempotent insert per zone. Clears `Card.FaceDown` when moving to non-battlefield zones per §707.4. | §707.4, §304.4 / §307.1 (instant/sorcery can't enter battlefield) |
| `(gs *GameState).removePermanent(perm) bool` | `state.go:1557` | Lifts a Permanent off its controller's battlefield. Bare pointer remove; caller responsible for destination placement, LTB triggers, cache invalidation. | (none — pre-API primitive) |
| `RemoveCardFromAllPrivateZones(gs, seatIdx, card) int` | `zone_move.go:180` | Defensive sweep for callers that construct their own Permanent on rebirth (per_card hooks). | §400.7 hygiene |
| `FireZoneChange(gs, perm, card, ownerSeat, fromZone, toZone) string` | `commander.go:258` | §614 replacement chain + §903.9b commander redirect + idempotent placement via `moveToZone`. | §614, §903.9b |
| `FireZoneChangeTriggers(gs, perm, card, fromZone, toZone)` | `zone_change.go:422` | Observer + self-trigger dispatch per §603.3b APNAP batching. | §603.3b, §603.10 |

Lower-level mutators (intentional escape hatches in per_card / engine):

| Pattern | Risk |
|---|---|
| `gs.Seats[X].Exile = append(...)` / direct slice append | Bypasses §707.4 face-up clear, §400.7 owner-routing, §614 replacements, §603.10 look-back triggers. The Etali bug shape. |
| `moveCardBetweenZones(gs, seat, c, fromZone, toZone, reason)` (per_card helper) | Wraps `MoveCard` but threads through caller-supplied `seat` — same owner-routing risk if caller passes controller-seat instead of owner-seat. |

## CR coverage map

### Already enforced at the API boundary

| CR | Where enforced | Notes |
|---|---|---|
| §122.1b shield counter consumed before destroy | `DestroyPermanent` line 50 | Returns false; permanent stays on battlefield. |
| §122.2 counters cease on zone change | Implicit via `Counters: map[string]int{}` on each new Permanent construction (cast_counts.go, keywords_batch*, etc.) | `Card.Counters` field does not exist — counters live on `Permanent`, which is reborn each ETB. |
| §400.7 new object identity on zone change | Implicit via fresh `Permanent` wrapper on battlefield entry; `Modifications`/`GrantedAbilities`/`MarkedDamage` are Permanent fields that reset. | §400.7c grant-on-permanent-spell handled by §613 stack→battlefield identity flow (just fixed in PR #685). |
| §403.4 battlefield permanent is new object | `moveToZone` battlefield arm wraps Card in fresh Permanent; `MoveCard` swaps to front face via `EnsureBattlefieldFrontFace` before triggers fire. | |
| §406.3 exile via spell/ability | `ExilePermanent` runs `would_be_exiled` replacement chain. | |
| §613 layer cache invalidation on LTB (for permanents with continuous effects) | `UnregisterContinuousEffectsForPermanent` calls `InvalidateCharacteristicsCache` when any effect is removed (layers.go:257). | Gap: leavers WITHOUT continuous effects don't trigger invalidation. Other permanents' "count of X" effects may read stale board state until next mutation. |
| §614 replacement chain on zone change | `FireZoneChange` (general) + `FireDieEvent` (dies-specific) + `fireExileEvent` (exile-specific) | Routed from every battlefield-exit helper. |
| §701.7 destroy ignored on indestructible | `DestroyPermanent` line 69 | Per §702.12b. |
| §701.17b sacrifice ignores indestructible | `sacrificePermanentImpl` (no indestructible check) | Comment on line 247 documents this. |
| §704.5d tokens cease to exist on zone change | All battlefield-exit helpers branch `if !perm.IsToken()` to skip the final `FireZoneChange` zone write. | |
| §707.4 face-down clears on non-battlefield zone move | `moveToZone` line 1581 | `c.FaceDown = false` when destination is not battlefield. |
| §903.9b commander redirect | `FireZoneChange` line 297 + idempotent insert guard added in PR #549 | |

### Gaps with rules-correctness implications

1. **§400.7 / §402-406 owner-scoped private zones — `moveToZone(seat, c, zone)` accepts ANY `seat` for ANY `zone`.** For non-battlefield zones (hand, library*, graveyard, exile, command_zone), the rules-correct target seat is always `c.Owner`. A buggy caller passing `controller` instead of `owner` routes the card into a non-owner's private zone — the exact Etali shape. The destination is wrong even when the §614 replacement chain runs to completion. **Severity: high** — single-call defensive override at `moveToZone` prevents the entire bug class.

2. **§707.4a face-down exile bit on `ExilePermanent`.** Cards exiled face-down by Hideaway / Manifest / Morph / Vanishing should remain face-down in exile until revealed. `ExilePermanent`'s API has no face-down parameter; callers must set `c.FaceDown = true` AFTER the call, racing the destination zone write. **Severity: medium** — affects ~12 known cards (Hideaway 4-pack, Manifest, Morph, Manifest Dread, Vanishing). API extension required.

3. **§613 cache invalidation on ANY LTB.** `UnregisterContinuousEffectsForPermanent` only invalidates when effects are removed; a vanilla creature dying leaves cache state untouched even though aggregate-count effects ("+1/+1 for each creature you control") on OTHER permanents now have stale values. **Severity: medium-low** — most callers manually invalidate; cache misses are rare in practice. Worth enforcing for correctness but not blocking the Etali bug class.

4. **§603.10 self-trigger look-back is enforced via `FireZoneChangeTriggers`** but lower-level callers that don't route through that helper miss the look-back. (Largely covered — `moveCardBetweenZones` calls FireZoneChangeTriggers; direct slice appends don't. Direct slice appends are the bug we're attacking in gap #1.)

5. **§706 copy state on zone exit.** A Permanent that was a Clone-copy of another permanent reverts to its base identity when it leaves the battlefield (the copy effect doesn't follow the card off battlefield per §706.10). The engine's `perm.OriginalCard` field tracks the pre-copy state but battlefield-exit helpers don't restore `perm.Card = perm.OriginalCard` before the zone write. **Severity: low** — only affects copy-of-copy edge cases; needs dedicated investigation.

6. **§706.12 cloning a multipart spell.** A permanent that copies a melded / fused / mutated permanent reverts to its base on leave. Same family as gap #5.

7. **§400.7e "find the new object in a public zone" for trigger resolution.** Already handled via `FireZoneChangeTriggers` carrying `card` through; cited here for completeness.

## Proposed enforcement points

### Implementing now (this PR)

**A. `moveToZone` owner-scoped zone redirect (gap #1).** When `zone` is one of
`hand`, `library`, `library_top`, `library_bottom`, `graveyard`, `exile`, or
`command_zone`, and `c.Owner` is valid AND differs from the passed `seat`,
override `seat = c.Owner` and emit a `zone_owner_redirect` event for audit.
Battlefield path keeps controller semantics intact (gain-control effects
legitimately put a card on a non-owner's battlefield). Risk: zero behavior
change for callers that already pass the owner; surfaces and corrects the
Etali bug shape at the lowest call point. The `zone_owner_redirect` event
gives Loki and audit tooling a single signature to count.

### Deferred (future PRs)

- **B. Face-down ExilePermanent variant.** Extend the API:
  `ExilePermanentFaceDown(gs, perm, source) bool` that sets `c.FaceDown = true`
  AFTER the move resolves. Or add a `faceDown bool` parameter and update the
  ~12 known callers. Defer because it requires a survey of every Hideaway /
  Manifest / Morph / Vanishing handler.
- **C. Always-invalidate cache on LTB.** Add `gs.InvalidateCharacteristicsCache()`
  to the tail of `DestroyPermanent` / `ExilePermanent` / `sacrificePermanentImpl` /
  `BouncePermanent` unconditionally. Defer because most callers already
  invalidate explicitly; need to measure cache-miss cost before committing.
- **D. Restore `OriginalCard` on copy-permanent LTB.** Requires a survey of
  every Clone / Mirror / Copy-Token handler to verify `OriginalCard` is
  populated correctly. Defer.

## Implementation note

The §400.7 owner-redirect at `moveToZone` is the load-bearing safety net.
Going forward, per_card handlers that need to route a card to a different
seat's zone (cross-seat steal, control-change, etc.) should use the
appropriate engine API (`ChangeControl`, `BounceToHand`, `ExilePermanent`)
rather than `moveToZone` directly — `moveToZone` is for owner-targeted
placement and the redirect makes that contract enforceable.
