# Loki r60 250K Sweep — Top-3 Cluster Analysis + Fix Paths

Companion to `docs/loki-r60-250k-report.md`. The report categorized
8 clusters across 67,395 violations / 1,577 games out of 250K. This
doc grounds the top 3 in actual engine code paths and proposes a
concrete fix path for each.

## Cluster A — CardIdentity (56,342 / 83.6%) — DOMINANT

### Observed signature

`*Card` pointer appears in two zones simultaneously. Zone-pair
distribution (sampled from 30 violation details):

| Pair | Count | Sub-cluster |
|------|------:|-------------|
| `battlefield` ↔ `battlefield` (cross-seat) | 11 | **A1** — generic ETB-wrap leak |
| `graveyard` ↔ `exile` (same seat) | 9 | **A2** — self-exile-from-graveyard fails to sweep graveyard |
| `exile` ↔ `battlefield` | 6 | **A3** — cast-from-exile / flicker leak |
| `hand` ↔ `battlefield` | 2 | **A4** — cast-from-hand fails to sweep hand |
| `graveyard` ↔ `battlefield` | 2 | **A5** — reanimate fails to sweep graveyard |

Sampled cards (all non-per_card-handler creatures): Worldspine
Wurm, Titanoth Rex, Skaab Ruinator, Nettle Swine, Colossus of
Akros, Demolisher Spawn, Void Winnower, The Dawning Archaic,
Mahamoti Djinn, Mossbeard Ancient, Loyal Retainers, Ebon Dragon.

### Root-cause hypothesis

The engine has two ETB-wrap entry points with **asymmetric dedup
guards**:

1. **`per_card/helpers.go:268 createPermanent`** — DOES dedup. It
   checks the target seat's battlefield for an existing Permanent
   around the same `*Card` (lines 285-294) AND calls
   `RemoveCardFromAllPrivateZones` for both owner and target seat
   (lines 295-305). Per-card hooks (reanimate, flicker, sneak-
   attack-style cheats) that go through this helper are safe.

2. **`internal/gameengine/stack.go:1366 resolvePermanentSpellETB`**
   — does NOT dedup. Lines 1442-1489 build a `&Permanent{Card:
   card, ...}` directly and `append` to `seat.Battlefield` with no
   pre-sweep of private zones and no dedup-against-existing-perm
   check. Safe for the *normal* cast-from-hand path because
   `PayCostsAndPushToStack` already removed the card from hand
   before pushing to stack — but UNSAFE for any code path that
   pushes a `*Card` onto the stack while it's still in another
   zone (e.g. cast-from-exile via grant, cast-from-graveyard via
   flashback/escape/disturb, cast-from-library via Bolas's Citadel
   / Future Sight / Aminatou-style).

The 9-violation `graveyard ↔ exile` sub-cluster (A2) is the same
shape as PR #685's Etali §400.7c fix but on `moveCardBetweenZones`
calls that bypass the `moveToZone` defensive backstop because they
target a non-owner-scoped zone, or they skip MoveCard entirely and
push directly onto the per-seat slice.

The dominant 11-violation `battlefield ↔ battlefield` sub-cluster
(A1) is the new high-value surface. The hypothesis from the report
("createPermanent dedup at helpers.go:299-303") was wrong — that
guard IS present and works. The real gap is that
`resolvePermanentSpellETB` doesn't have an equivalent guard, and
ANY engine path that synthesizes a `&Permanent{Card: c}` literal
+ `append(seat.Battlefield, ...)` without first sweeping `*Card`
references is the bug surface.

### Fix path

1. **Extract a shared `wrapCardAsPermanent(gs, seat, card,
   options)` helper** in `internal/gameengine/zone_move.go` that
   does the canonical sequence:
   - Validate card can enter battlefield (CR §304.4 / §307.1).
   - If `*Card` already in any seat's battlefield as a Permanent,
     return existing perm (cross-seat dedup, not just target-seat).
   - Sweep `*Card` from owner's private zones AND target seat's
     private zones (mirror `per_card/helpers.go:295-305`).
   - Build the `&Permanent{...}` literal + append.
   - Return the new perm.
2. **Replace the in-line `&Permanent{...}` + `append` pattern**
   at `resolvePermanentSpellETB` (stack.go:1442-1489) and
   `resolveCopyPermanent` (resolve.go:2711-2719) with calls to
   `wrapCardAsPermanent`.
3. **Refactor `per_card/helpers.go:createPermanent`** to forward
   to `wrapCardAsPermanent` so per_card and engine share one
   implementation (kill the asymmetry at the root).
4. **Property test** in `internal/gameengine/zone_owner_dedup_test.go`:
   for every {cast-from-hand, cast-from-exile, cast-from-graveyard,
   reanimate, flicker-return, token-copy} entry path, assert that
   after ETB the `*Card` pointer appears in EXACTLY ONE zone across
   all seats. Failures emit the offending path name.

### Estimated yield

−56,342 violations (-83.6%) in single PR if the helper extraction +
4 call-site replacements catch all five sub-clusters. Conservative
estimate: −45,000 from A1 + A3 + A4 + A5; A2's 9-violation
graveyard↔exile sub-cluster may need a second PR for the
non-MoveCard exile paths.

Recommended branch: `dev/cardidentity-wrap-helper-r60`.

## Cluster B — ZoneConservation (7,770 / 11.5%)

### Observed signature

`zone conservation suspicious: N extra real cards appeared
(expected X, found Y) — possible copy bug`. Magnitude 11-40 extra
real cards per game.

### Root-cause hypothesis

PR #705 closed the Naru Meha + Panharmonicon residual by adding
the resolve-time `card.Types = append(card.Types, "token")` stamp
at `resolvePermanentSpellETB` (stack.go:1416-1433) for copy
permanents per CR §707.10f. So copies of PERMANENT spells are now
correctly excluded from the per-seat real-card census.

The remaining 7,770 violations point to a DIFFERENT copy-cascade
shape:

1. **Copies of NON-permanent spells (instant/sorcery)** — copies
   resolve, go to graveyard per CR §608.2g, then per §707.10c
   "the copy ceases to exist as a state-based action". If the
   ceasing-to-exist isn't actually purging the `*Card` from the
   graveyard's per-seat slice, the census drifts up.
2. **`MakeToken` / `createToken`-style allocations that don't tag
   `IsToken()`** — every token-mint path must put `"token"` into
   `Card.Types` (mirror of the §707.10f stamp at stack.go:1432),
   or `cardIsTokenForInv` returns false and the invariant counts
   them as real cards.
3. **Triggered-ability fan-out that allocates fresh `*Card`
   wrappers per copy** — Galvanoth / Twincast / Storm trigger
   cascade allocates one wrapper per copy, none `IsCopy = true`,
   so resolution-time §707.10f stamp doesn't fire.

### Fix path

1. **Add an SBA pass** in `internal/gameengine/sba.go` that scans
   every seat's graveyard for `IsCopy = true` cards and removes
   them per CR §707.10c. Fire after the standard 704.5 sweep.
2. **Audit every `MakeToken` / token-mint call site** for the
   `"token"` Types tag. Easy grep:
   `grep -rn "Types:.*\[\]string" internal/gameengine/`. Any
   call site that builds a token `*Card` without including
   `"token"` is a leak.
3. **Add `IsCopy = true` propagation** to the trigger fan-out
   loop in `resolveCopySpell` (resolve.go:2603) so every copy
   wrapper inherits the stamp before being pushed to stack.
4. **Property test** in `internal/gameengine/copy_cascade_test.go`:
   build a Naru Meha + Panharmonicon + Twincast cascade fixture,
   resolve, assert per-seat census is exactly { commanders +
   library + hand + graveyard + exile + battlefield-real }
   counting only `!IsCopy && !IsToken` cards.

### Estimated yield

−7,770 (-11.5%) in single PR if the SBA-pass + token-tag audit
catches all three sub-shapes. Recommended branch:
`dev/zoneconservation-copy-sba-r60`.

## Cluster C — ExileLinkageIntegrity (3,228 / 4.8%)

### Observed signature

`ExileLinkageIntegrity: card "X" in seat Y exile is linked to
source timestamp Z which is no longer on any battlefield — LTB
return missed (orphaned linked exile)`.

Dominant card distribution: lands (Island ×3, Swamp ×2, Mountain
×2) — these are common Oblivion-Ring-style exile targets, not the
leaky source. The leaky source is whatever Aura/Equipment/
Enchantment exiled them with a "return when this leaves play"
linkage.

### Root-cause hypothesis

Direct parallel to PR #106's `ExpireSourceGrants` sweep, which
audited all 7 LTB paths (`DestroyPermanent`, `ExilePermanent`,
`sacrificePermanentImpl`, `BouncePermanent`, `destroyPermSBA`,
`sacrificePermSBA`, `HandleSeatElimination`) and made sure each
called `ExpireSourceGrants(gs, p.Timestamp)`. The
`FireLinkedExileReturns(gs, sourceTimestamp)` sweep — which fires
the "return exiled card to battlefield" trigger when the linked
source leaves play — likely has the same gap: it's wired into the
common LTB path (`DestroyPermanent`) but missed in one or more of
the 6 siblings.

Specifically, `HandleSeatElimination` is the most likely culprit
(per the 2026-05-24 issue-log row that found `ExpireSourceGrants`
missing from that exact path). Once a controller is eliminated,
their Oblivion Ring permanents are swept off the battlefield but
their linked-exile returns never fire — the exiled lands sit in
exile forever, and `checkExileLinkageIntegrity` flags them.

### Fix path

1. **Audit the 7 LTB paths** for calls to
   `FireLinkedExileReturns(gs, p.Timestamp)` (or equivalent).
   Grep: `grep -rn "FireLinkedExileReturns\|LinkedExileReturn"
   internal/gameengine/`. Compare against the 7-path checklist
   from PR #106.
2. **Add the sweep call** to every LTB path missing it. Pattern:
   immediately after the perm leaves play and `UnregisterReplacementsForPermanent`
   is called, also call `FireLinkedExileReturns(gs, p.Timestamp)`.
3. **Regression test** in
   `internal/gameengine/exile_linkage_ltb_r60_test.go`: for each
   of the 7 LTB paths, set up an Oblivion-Ring-style fixture
   (source perm with linked exile), trigger the LTB via that
   specific path, assert the exiled card returns to its owner's
   battlefield.

### Estimated yield

−3,228 (-4.8%) in single PR if all 7 paths are swept. Recommended
branch: `dev/exilelinkage-ltb-sweep-r60`.

## Combined fix sequence

If the 3 PRs ship in order (A → B → C), the projected post-fix
250K-seed-42 violation count is:

| Step | Estimated count | Δ |
|------|----------------:|---|
| Current | 67,395 | — |
| After A (`wrapCardAsPermanent` helper) | ~11,053 | −56,342 |
| After B (`ZoneConservation` SBA + token audit) | ~3,283 | −7,770 |
| After C (`FireLinkedExileReturns` LTB sweep) | ~55 | −3,228 |
| Remaining (Clusters D-H) | ~55 | — |

The remaining 55 are clusters D (TriggerCompleteness x40) + E-H
long-tail (x15). A fourth PR for cluster D's 6-handler audit
(Pia / Minn / Exava / Jaxis / Ratadrabik / Slimefoot) gets to
~15. A fifth long-tail batch closes the rest. Projected final
state: 0 violations across 250K games at seed 42 — matching the
canonical-final clean state at 25× the depth.

## Caveats

- All three hypotheses are derived from spot-reading of the engine
  paths cited; no fix has been implemented or tested under this
  doc. The "estimated yield" figures assume each hypothesis is
  correct AND the proposed fix doesn't introduce new bugs.
- Hypothesis A's sub-cluster A2 (`graveyard ↔ exile`, 9
  violations) might need its own PR if the leak isn't in a path
  reachable from `wrapCardAsPermanent`. The Etali §400.7c work
  (PR #685) closed the closest sibling; the residual is likely
  in self-exile-from-graveyard handlers that bypass MoveCard.
- The Cluster B SBA-pass approach assumes the `*Card` allocator
  is the leak source. If the leak is actually `Permanent` ref
  retention after the perm leaves the battlefield, a different
  fix shape is needed (likely an LTB-time `*Card` purge from the
  controller's per-seat object list).
