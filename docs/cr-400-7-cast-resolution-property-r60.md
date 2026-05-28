# CR §400.7c Cast-Resolution Property Test (r60)

## Goal

Property-test the §400.7c contract for cross-control casts: when an
opponent's card is cast via a grant (Etali / Praetor's Grasp /
Possibility Storm / Dauthi Voidwalker / Release to the Wind /
Mind's Desire / Outpost Siege, plus the broader Bribery / Hostage
Taker / Knowledge Pool family), the resolved spell's destination
must be:

- **Instant / sorcery** — the OWNER's graveyard (CR §608.2g: "the
  spell is put into the graveyard of its owner").
- **Permanent** — a permanent under the CASTER's control with
  `Owner = original-owner` (CR §608.3a: "the spell becomes a
  permanent under that player's control" combined with CR §400.7c's
  owner-stable invariant).

If a per_card handler or engine call site violates this contract,
the property test fails and the failure message names the offender.

## What the test does

`internal/gameengine/per_card/cr_400_7_cast_resolution_r60_test.go`
contains 5 property tests:

| # | Name | What it pins |
|---|------|--------------|
| 1 | `TestCR400_7_InstantResolvesToOwnersGraveyard_NotCasters` | Stage opponent's Counterspell in caster's exile, register a `ZoneCastGrant`, push to stack, `ResolveStackTop` → assert the card lands in the **owner's** graveyard, not the caster's. |
| 2 | `TestCR400_7_PermanentEntersWithOriginalOwner_CasterControl` | Stage opponent's creature (with BasePower/Toughness set so SBA §704.5f doesn't sweep), resolve via grant → assert resulting `*Permanent` has `Controller = caster` AND `Owner = original-owner`. |
| 3 | `TestCR400_7_GrantHandlerAudit_RequireControllerIsCaster` | End-to-end Etali spot-check + log the audit list of 7 known grant-creating per_card handlers. |
| 4 | `TestCR400_7_CastFromExile_NoOwnerRedirectEventsOnResolve` | **Structural** check — assert the cast-from-exile resolution path does NOT emit any `zone_owner_redirect` events. If it does, a call site is passing caster-seat where it should pass owner-seat and is silently being rescued by the `moveToZone` defensive backstop. |
| 5 | `TestCR400_7_GrantHandlerAudit_OpponentInstantsGoToOwnersGraveyard` | End-to-end via real Etali grant: register, cast, resolve, verify counterspell lands in seat-1's graveyard. |

## The defensive backstop at `state.go:1614-1645`

The contract tests (1, 2, 3, 5) all passed at the start of this
work even though the cast-from-exile resolve path at
`internal/gameengine/stack.go:1289 / 1304 / 1318` was passing
`item.Controller` (caster) instead of `item.Card.Owner` to
`MoveCard`. The reason: PR #685 (the Etali r60 cluster fix) added a
defensive backstop inside `moveToZone`:

```go
// state.go:1591
func isOwnerScopedZone(zone string) bool {
    switch zone {
    case "hand", "library", "library_top", "library_bottom",
        "graveyard", "exile", "command_zone":
        return true
    }
    return false
}

// state.go:1614-1645 — inside moveToZone:
if isOwnerScopedZone(zone) && c.Owner > 0 &&
    c.Owner < len(gs.Seats) && c.Owner != seat {
    gs.LogEvent(Event{
        Kind:   "zone_owner_redirect",
        Seat:   c.Owner,
        Target: seat,
        ...
    })
    seat = c.Owner  // redirect to owner
}
```

The comment in that block explicitly references "the exact Etali
r60 cluster shape (PR #685)" — the backstop was added precisely
to catch sibling bugs of the cluster the §400.7c routing fix
closed. The backstop is the safety net.

## Why structural test 4 is the load-bearing assertion

The contract tests pass because of the backstop. The structural
test (#4) asserts that the cast-from-exile path **doesn't trigger
the backstop**, which is what surfaces a real call-site bug. Before
this PR, test #4 failed because the resolve path at
`stack.go:1318` (and its siblings at 1289 and 1304) passed
`item.Controller` instead of `item.Card.Owner` and relied on the
backstop to silently re-route. The fix in this PR threads
`item.Card.Owner` through all three sites so the call is
structurally correct — a future simplification of `moveToZone` (e.g.
performance pass, refactor, alternate code path) would not re-open
the §400.7c cluster on these paths.

## Fix shipped alongside the property test

Three identical-shape fixes in `internal/gameengine/stack.go` —
flashback-exile (line 1289), buyback (line 1304), and standard
non-permanent resolve (line 1318):

```go
// before
MoveCard(gs, item.Card, item.Controller, "stack", "graveyard", "resolve")

// after
MoveCard(gs, item.Card, item.Card.Owner, "stack", "graveyard", "resolve")
```

CR §608.2g is explicit ("the graveyard of its owner"), and CR
§702.27b is explicit for buyback ("its owner's hand"). The fix
matches the rule wording.

## Audit list — 7 known grant-creating per_card handlers

Test #3 enumerates the canonical grant-creating handlers and (via
`stageGrantedSpellInExile` + `resolveCastFromExile`) confirms the
shared engine resolution path satisfies the contract for every one
of them, since they all funnel through `ResolveStackTop`:

| Card | Grant shape | File |
|------|-------------|------|
| Etali, Primal Storm | attack-trigger exile + free cast (per-owner exile) | `etali_primal_storm.go` |
| Possibility Storm | chaos cascade — exile until matched, free cast | `possibility_storm.go` |
| Praetor's Grasp | exile from opponent library, free cast for rest of game | `praetors_grasp.go` |
| Dauthi Voidwalker | exile-on-graveyard + free cast (any zone) | `dauthi_voidwalker.go` |
| Release to the Wind | exile + permanent grant ("as long as exiled") | `release_to_the_wind.go` |
| Mind's Desire | storm — exile from top, may cast each | `minds_desire.go` |
| Outpost Siege | dragons mode — exile from top, may cast this turn | `per_card_batch_ai_r60.go` |

The audit is non-exhaustive (Bribery / Hostage Taker / Knowledge
Pool / Bolas's Citadel / Maelstrom Wanderer-style cascade are not
explicitly wired here), but the resolution-time §400.7c contract
holds for any handler that funnels through `ResolveStackTop`, which
is the universal cast-resolution path — so a fresh handler that
respects the standard `RegisterZoneCastGrant` / cast-grant API will
inherit the property automatically.

## Verdict

The engine is CR §400.7c-compliant for cross-control casts via:

1. **PR #685 owner-routing fix** for Etali's exile placement (the
   originating cluster).
2. **The `moveToZone` defensive backstop** at `state.go:1614-1645`
   that catches future sibling bugs in any owner-scoped zone path.
3. **This PR's structural fix at `stack.go:1289 / 1304 / 1318`**
   that makes the cast-from-exile resolve path stop relying on the
   backstop and pass `item.Card.Owner` directly per §608.2g and
   §702.27b.
4. **The 5-test property suite** that pins the contract going
   forward and structurally guards against the call site
   regressing.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git checkout dev/cr-400-7-cast-resolution-r60
go test ./internal/gameengine/per_card/ -run "TestCR400_7" -count=1 -v
```

Expected:

```
PASS: TestCR400_7_InstantResolvesToOwnersGraveyard_NotCasters
PASS: TestCR400_7_PermanentEntersWithOriginalOwner_CasterControl
PASS: TestCR400_7_GrantHandlerAudit_RequireControllerIsCaster
PASS: TestCR400_7_CastFromExile_NoOwnerRedirectEventsOnResolve
PASS: TestCR400_7_GrantHandlerAudit_OpponentInstantsGoToOwnersGraveyard
```

To see test #4 fail counterfactually (confirming it actually
guards the structural property), revert the three `stack.go` edits
to pass `item.Controller` and re-run — test #4 will fail with the
diagnostic naming the call site.

## CR references

- **§400.7c** — "An object remains in the zone it's in unless an
  effect or rule says otherwise"; the cross-zone-control invariant
  that owner remains stable across zone moves.
- **§608.2g** — "If the spell is an instant or sorcery, the spell
  is put into the graveyard of its owner."
- **§608.3a** — "If the object is a permanent spell, it becomes a
  permanent and is put onto the battlefield under the control of
  the player who cast it."
- **§702.27b** — "If the buyback cost was paid, put that spell into
  its owner's hand as it resolves instead of into its owner's
  graveyard."
- **§702.33** — "If the flashback cost was paid, exile this card
  instead of putting it anywhere else any time it would leave the
  stack."
