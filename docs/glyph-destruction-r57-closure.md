# Glyph of Destruction hand↔graveyard leak — r57 closure verification

**Date:** 2026-05-20
**Branch:** `dev/glyph-destruction-r57`
**Base:** main @ `9146e11` (post Bontu r56 fix; post the R54/R55 wave)
**Request:** Investigate Loki r55-surfaced "Glyph of Destruction
appears in both seat 3 hand and seat 3 graveyard" CardIdentity leak;
find root cause + fix.

## Headline

**The Glyph of Destruction leak is already closed.** No fix shipped on
this branch — the closure landed in r54 (`1b9a799`) as the Krark,
the Thumbless bounce-back fix. Fresh 15K-game verification on current
main confirms zero Glyph hits across two independent seeds.

This document is the paper trail tying the user-named symptom (Glyph
of Destruction game-490 / r53 / r54) to the engine-level fix (Krark
trigger §608.2b stale-stack guard) so future seed sweeps that surface
the same shape can be triaged in under a minute.

## Repro on current main

```
cmd/hexdek-loki --games 10000 --seed 48 --report /tmp/glyph-r57-seed48-10k.md
cmd/hexdek-loki --games  5000 --seed 56 --report /tmp/glyph-r57-seed56-5k.md
```

| Run                 | Total viol. | Dirty games | Glyph of Destruction hits |
|---------------------|------------:|------------:|--------------------------:|
| seed 48 / 10000 g   |         668 |          47 |                     **0** |
| seed 56 /  5000 g   |         664 |          27 |                     **0** |
| combined / 15000 g  |        1332 |          74 |                     **0** |

Verification command:

```
$ grep -c "Glyph of Destruction" /tmp/glyph-r57-seed48-10k.md
0
$ grep -c "Glyph of Destruction" /tmp/glyph-r57-seed56-5k.md
0
```

Top-1 CardIdentity message on current main:
- seed 48: `Mire's Grasp (ptr 0xc00663c6c0) appears in both seat 0 exile and seat 0 battlefield`
- seed 56: `Swamp (ptr 0xc00c482120) appears in both seat 2 exile and seat 2 battlefield`

Both are exile↔battlefield (different family from Glyph's hand↔graveyard).

## What the Glyph leak was

Per the r53 cluster-hunt doc and the r55 post-R54 validation, the
exact symptom was:

```
CardIdentity: card "Glyph of Destruction" (ptr 0xc006c110e0) appears
  in both seat 3 hand and seat 3 graveyard
```

Volume: ~236 hits in 7500 games on seed 48 (r53 baseline), all from
game 490. Commanders included **Krark, the Thumbless** on seat 3.

## Root cause (per the r54 fix commit body)

The leak was a downstream symptom of Krark's coin-flip lose-branch
bounce:

> `krarkTrigger`'s lose branch unconditionally called
> `MoveCard(card, owner, "stack", "hand", ...)`.
> `removeCardFromZone("stack")` is intentionally a no-op
> (`zone_move.go:239` — battlefield/stack source removal is the
> caller's responsibility), so when `stackIdx < 0` the engine appended
> the `*Card` to hand without removing it from its actual current zone.
> Same pointer ended up in both graveyard and hand.

Why `stackIdx < 0`: per_card triggers dispatch through
`PushPerCardTrigger` which routes via the stack. When the
`spell_cast` event fires inside another resolution frame (CR §608.2c
nested-resolve), the trigger queues into `gs.pendingTriggers` and
drains at the outermost frame end. By that point, the original spell
may have already resolved into graveyard/exile/hand — its `*Card` is
no longer on the stack but still exists elsewhere.

Glyph of Destruction (`{R}` instant, Wall-pump + indestructible UEOT +
delayed-destroy) cast on a turn where Krark was in play hit this race:
Glyph's resolve drained the stack and routed the card to graveyard;
the queued Krark trigger then fired with `stackIdx < 0`, hit the
broken bounce, and re-appended the same `*Card` pointer to hand.

## The fix

Commit `1b9a799` (merged `6586a85`):

> Per CR §608.2b, when the trigger's target (the cast spell) is no
> longer on the stack at resolution time, the bounce effect does
> nothing for that target. Add an early-return when `stackIdx < 0`
> mirroring the existing WIN branch's `spell_not_on_stack` guard.

The fix in `per_card/krark.go` (~line 103):

```go
if stackIdx < 0 {
    emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
        "seat":  perm.Controller,
        "flip":  "lose",
        "spell": card.DisplayName(),
        "noop":  "spell_no_longer_on_stack",
        "rule":  "608.2b",
    })
    return
}
```

Plus the existing regression test
`TestKrark_R54_LoseFlipNoOpWhenSpellLeftStack` in
`krark_bounceback_r54_test.go` (line 47) — uses `Glyph of Destruction`
by name as the test fixture spell card. So the fix is already pinned
against the exact game-490 leak shape; an accidental regression of
the §608.2b stale-stack guard would fail that test.

## Per-run impact (per r54 / r55 docs)

r54 Krark-fix interim (`--games 7500 --seed 48`):
- Total: 780 → 544 (−30%)
- CardIdentity: 616 → 380 (−38%; all 236 game-490 Glyph hits cleared)
- Clean game rate: 99.37% → 99.48% (+0.11pp)

r55 post-R54 wave validation (`--games 7500 --seed 48`):
- Total: 780 → 346 (−56%)
- CardIdentity: 616 → 182 (−70%)
- Glyph of Destruction: 0 hits

r57 fresh verification (this doc):
- 15K games across seeds 48 + 56: 0 Glyph hits

## Top-1 leads after this closure

The current top-1 CardIdentity surface on main (post r54 Krark + r54
Athreos + r55 wave + r56 Bontu) is **Mire's Grasp** appearing in both
seat 0 exile and seat 0 battlefield (10K seed 48 — 164 CardIdentity
hits). Different anti-pattern family (exile↔battlefield aura
attachment) than Glyph's hand↔graveyard cast-resolve race. That
becomes the natural r58+ target.

Cross-seed (seed 56), the top-1 CardIdentity is **Swamp** appearing
in both seat 2 exile and seat 2 battlefield — same exile↔battlefield
shape on a basic land, suggesting the leak surface is a generic
permanent recovery handler rather than card-specific.

## Verdict

**No fix needed.** The Glyph of Destruction leak surfaced in Loki
r53/r55 was closed by the r54 Krark bounce-back fix. The fix is
test-pinned by `TestKrark_R54_LoseFlipNoOpWhenSpellLeftStack` (which
uses Glyph of Destruction as its fixture spell, so a future
regression of the §608.2b guard would fail loudly there).

This doc captures the symptom→fix mapping so the next seed sweep that
hits a similar shape can be diagnosed without re-walking the
forensics. Branch `dev/glyph-destruction-r57` ships this doc only.
