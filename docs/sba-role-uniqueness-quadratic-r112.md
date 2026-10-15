# r112 — (16-TIMING): the 11-second turn is one O(n²) state-based action

r111 showed the pathological grinder games are **not longer in turns** (+24%)
but **~24x more expensive per turn** (median 11.16 s/turn against 0.47), and
reframed issue 16 as "what makes a turn cost 11 seconds?". This answers it.

**85% of all `StateBasedActions` time is `sba704_5y`, the Role-uniqueness SBA,
and it rescans every seat's entire battlefield once per permanent — including
in the overwhelmingly common case of a game containing no Roles at all.**

## Measuring the shape before touching anything

The standing benchmarks r80 and r85 are both pinned at `n=40`, so nothing had
ever measured board-size scaling. Their fixtures are already parameterized, so
r112 sweeps them rather than inventing new ones (`-benchtime 30x`, M4):

```
                         n=10     n=20     n=40     n=80    n=160    n=320
TriggerScan              1.6µs    2.6µs    5.2µs   11.3µs   26.9µs   57.6µs   ~linear, 0 allocs
CastObservers cold      13.9µs   27.0µs   62.8µs  142.8µs  506.9µs  673.0µs   ~linear
CastObservers warm       5.8µs   15.5µs   19.7µs   39.0µs  102.9µs  168.9µs   ~linear
EffectiveChars cold      1.2µs    1.1µs    1.0µs    1.1µs    1.1µs    1.2µs   FLAT — O(1)
StateBasedActions       16.8µs   38.5µs   90.8µs  296.9µs  910.2µs  2984µs    SUPERLINEAR
```

Two things worth noting before the main one.

- **`GetEffectiveCharacteristics` is flat in board size.** r85's doc comment
  says the first layer query after an invalidation "recomputes the board's
  characteristics". It recomputes *that permanent's*. The comment overstates
  it; benign, but it is the thing that would have sent someone to the wrong
  place.
- Only SBA is superlinear, so only SBA got the longer arm:

```
                  n=40     n=80    n=160    n=320    n=640   n=1280
StateBasedActions 97µs    218µs    762µs   2.63ms   9.19ms  32.42ms
per-doubling      —       2.24x    3.50x    3.45x    3.49x    3.53x
```

The ratio settles at ~3.5 per doubling: **exponent ≈ 1.81**, approaching
quadratic. Allocations are *exactly* linear (240 → 7,681 for a 32x board), so
this is CPU inside the scan, not garbage.

### Checking the fixture before believing the curve

`r80Board` names every permanent "Body". If any SBA grouped or compared by
name across the battlefield — the legend rule being the obvious candidate —
the curve would be my fixture's coincidence, not the engine's behaviour.
r100's lesson in reverse: a fixture that *creates* a coincidence the real path
does not have is as bad as one that omits a field. Re-run with distinct names:

```
                  n=40     n=80    n=160    n=320    n=640   n=1280
unique names      89µs    232µs    793µs   2.59ms   8.87ms  33.53ms
```

Identical. Not a fixture artifact.

## The profile

No pprof on .206 — but nothing stops a local one. At `n=1280`:

```
  flat  flat%   cum   cum%
 1.49s 80.54% 1.50s 81.08%  (*Permanent).hasType (inline)
 0.09s  4.86% 1.58s 85.41%  sba704_5y
 0.03s  1.62% 0.10s  5.41%  (*GameState).IsCreatureOf
```

**85.4% of SBA time in one function; 80.5% of total inside the `hasType` call
behind `IsRole`.**

## The code

```go
for _, holder := range s.Battlefield {          // every permanent
    rolesByCtrl := map[int][]*Permanent{}       // a map allocated per holder
    for _, s2 := range gs.Seats {
        for _, r := range s2.Battlefield {      // every permanent, again
            if r.IsRole() && r.AttachedTo == holder {
```

A full battlefield rescan per permanent, with `IsRole()` — a string scan over
`Card.Types` — at the centre. Roles are a single-set mechanic; virtually every
game in the pool has none, and every one of those games still paid the whole
n² sweep on every SBA pass, up to 40 passes per call.

## The fix

Index the Roles once, in the same seat/battlefield order the nested scan
visited them, so grouping and destruction order — and therefore the event log
— are unchanged. Return immediately when the board holds no Roles. A
`destroyed` set guards the one case the rebuild-per-holder shape used to cover
for free: a Role attached to another Role, destroyed earlier in the same pass.

```
                  n=40     n=80    n=160    n=320    n=640   n=1280
before            97µs    218µs    762µs   2.63ms   9.19ms  32.42ms
after             86µs    220µs    425µs   0.95ms   2.56ms   6.11ms
                 -12%       ~0     -44%     -64%     -72%     -81%
exponent 1.81 -> 1.20
```

**5.3x at n=1280.** Re-profiled after: no dominant function left — the top
entry is `IsCreatureOf` at 40% cumulative with `BaseCharacteristics` behind
it, which is ordinary per-permanent work. The residual 1.20 exponent is real
but no longer concentrated anywhere worth a second round today.

## What I am NOT claiming

**I have not shown this is the cause of the 11 s/turn games.** To connect them
I would need the board sizes of those games, and nothing logs them — that is
precisely the instrumentation r111 said was the useful ask. What I have is:
the per-turn cost explosion is real (r111), SBA was superlinear in board size
(measured), the cause was one function (profiled), and it is now 5.3x cheaper
at large boards. Whether prod boards reach n=640 is unmeasured.

One suggestive but **uncontrolled** observation: the most frequent deck in
r111's `>=480s` bucket is `krenko_mob_boss_b2_zdog97` (4 of 25), and Krenko is
a goblin-token engine — the archetype that produces enormous boards. So is
`ashnod_flesh_mechanist` (3 of 25). That is a hint about where to point the
board-size counter, not evidence.

## A separate finding: the §704.5 subrule letters are shifted

While reading the citation I checked it against the rules file, and from
`sba704_5w` onward the code is **one letter behind the current CR**:

```
code fn       code claims                  CR 20260819 actually says
sba704_5v     battle defense 0        704.5v  Siege battle defense 0      ok
sba704_5w     battle has no protector 704.5w  non-Siege battle defense 0  SHIFTED
sba704_5x     Siege controller=protector 704.5x battle has no protector   SHIFTED
sba704_5y     Role uniqueness         704.5y  protector can't be          SHIFTED
sba704_5z     start your engines      704.5z  Role uniqueness             SHIFTED
(none)                                704.5aa start your engines
```

Each comment's quoted **text** is the rule the function implements; each
**number** is one behind, because the CR split battle-defense-0 into Siege and
non-Siege and pushed everything down. So this function is §704.5z, not
§704.5y.

**Filed, not fixed, deliberately.** The letter is not only in comments — it is
the `rule` string passed to `destroyPermSBA`, which reaches the event log, and
outcome digests are paritycheck-gated. Renaming five functions and their
logged rule strings is a clean separate round with its own gate run, not a
rider on a performance fix. This is the complement of the hazard in the CR
citation audit: there, an edition bump changed a rule's *meaning* under a
stable number; here it changed the *numbers* under stable meanings.

## Verification

- Four new semantic regressions, plus the pre-existing
  `TestSBA_704_5y_RoleUniquenessKeepsNewest`, all green.
- The semantics did not change, so these guards pass before and after — which
  by lesson 56 means they have not earned their keep yet. So each was
  **mutation-checked** instead: dropping the controller split turns the
  different-controllers test red; bucketing every Role under one holder turns
  the different-holders test red. The benchmark delta is this change's real
  "fails pre-change" artifact.
- Eleven packages green including crcite and paritycheck; hexapi only its
  three known deterministic reds.

## Lesson 64

**A hot path's cost is a function of a variable the benchmark pinned.** r80
and r85 were both written as standing guards against exactly this class of
regression, both correctly measured the thing they were built for, and both
fixed `n=40` — so a quadratic in `n` was invisible to the instruments meant to
catch it, for as long as they existed. When a benchmark takes a size
parameter, sweep it; a single point measures a constant, not a shape.
