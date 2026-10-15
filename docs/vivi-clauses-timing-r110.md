# r110 — (17c): Vivi's other two clauses. One retraction, one 819-card gap.

r108 left two things about Vivi Ornitier explicitly labelled as suspicions
rather than measurements. This round measures both. One is unfounded and is
retracted. The other is much larger than the card.

Both AST nodes were transcribed from the real corpus entry on .206
(`ast_dataset.jsonl`, `Vivi Ornitier`, `fully_parsed: true`), not invented.

## 1. The `cast_filtered` noncreature filter — SUSPICION RETRACTED

r108 wrote:

> whether the `cast_filtered` trigger preserves the **noncreature** filter —
> its `target_filter` is `null` in the AST, which makes me suspect it fires on
> every spell — **but that is a suspicion, not a measurement**.

It is a measurement now, and the suspicion was wrong. The AST really does
carry `actor: null` and `target_filter: null`, with "noncreature" surviving
only in `raw` — but there is a branch r108 never looked at.
`fireObserverCastTriggers` falls through to `observerCastMatchesByRaw` when
`Actor == nil`, and that matcher parses the filter back out of `Raw`
(`case strings.HasPrefix(rest, "a noncreature spell,")`), along with the
"you cast" controller gate.

Driven through `fireCastTriggers`, with the first row as a positive control:

```
CONTROL you cast an instant (noncreature)  counters=1  opp_life_delta=1  fired=true
you cast a sorcery (noncreature)           counters=1  opp_life_delta=1  fired=true
you cast a CREATURE spell                  counters=0  opp_life_delta=0  fired=false
OPPONENT casts an instant                  counters=0  opp_life_delta=0  fired=false
```

Correct on all four. `TestViviCastFilter_r110` pins it.

**The lesson is about my own reasoning, not the engine.** A null field is
evidence that *one* path can't see the filter, not that *no* path can. r108
was right to label it a suspicion and right not to act on it.

## 2. The timing restriction — NEITHER HALF ENFORCED, and the gap is 819 cards

Vivi prints *"Activate only during your turn and only once each turn."*
Measured at `ActivateAbility`, with a positive control:

```
CONTROL  own turn, 1st activation    ok, mana=U:2 R:1
PROBE    own turn, 2nd activation    err=<nil>  mana now=U:4 R:2
PROBE    OPPONENT's turn, activation err=<nil>  mana=U:2 R:1
VERDICT: NEITHER half of the timing restriction is enforced.
```

A `{0}` mana ability with no enforced once-per-turn limit is an unbounded
mana loop. r109 did not create this — one `Any` per activation was already
unbounded — but r109 made each iteration produce `power` mana instead of 1.

### Why nothing enforces it

There are **two different representations** of a timing restriction, and the
enforcement is wired to the one almost nothing uses.

```
                                              corpus cards
Static  Modification{kind:"timing_restriction"}      819
Activated.TimingRestriction  (the struct field)        9
OVERLAP                                                0
```

- Everything that **enforces** reads the struct field:
  `tournament/turn.go:2418` (the live grinder's activation gate,
  `act.TimingRestriction == "sorcery"`), `legality.go:1110` (the §602.5d
  detector), and `IsExhaustAbility` / `IsPowerUpAbility`.
- The 819 cards put theirs in a `Static` Modification node, whose only reader
  is `resolve_helpers.go:2397` — **an explicit no-op**, commented:

  > Intentional no-op — timing restrictions are checked before resolution,
  > not applied as effects.

  The comment is right about where the check belongs and wrong about whether
  it happens. The check that belongs there reads a different field.

Breakdown of the 819:

```
455  'activate only as a sorcery'
133  'activate only during'
120  'once_per_turn'
115  'activate only once'
  5  'activate no more than twice'
  3  'activate no more than three times'
```

**455 of them are sorcery-speed restrictions** — exactly what
`legality.go:1110` and `turn.go:2418` are built to catch. The gate works. It
is pointed at a field that 9 cards populate, and none of those 9 is one of
the 455.

Note also that `'activate only during'` is **truncated** — the parser drops
"your turn and only once each turn", so even a reader of the Modification
form could not tell which turn or how many times.

### Corollary: Exhaust and Power-up are inert for every corpus card

`IsExhaustAbility` returns `ab.TimingRestriction == "exhaust"` and
`IsPowerUpAbility` returns `== "power_up"`. The field's only observed corpus
value is `"sorcery"` (9 cards). So the `activation.go:517` gate —
"activate each exhaust ability only once" — **never fires**, for

```
40  corpus cards printing Exhaust
37  corpus cards printing Power-up
```

`IsExhaustAbility`'s own doc comment claims detection via *"the AST
TimingRestriction … **or** the ability has the 'exhaust' keyword flag"*. The
body implements only the first. Recurring finding (p) again: the comment
promises a second route that was never built.

### The dangerous subset

Free-cost mana ability + an unenforced Static timing restriction — 7 cards:

```
Chandra, Hope's Beacon          Night of the Sweets' Revenge
Huatli, Poet of Unity           Ramos, Dragon Engine
Mana Bloom                      Vivi Ornitier
Wall of Roots
```

**A hypothesis, flagged as one and NOT claimed:** unbounded free mana is the
kind of thing that could stretch a game, and this lane has an open
slow-game/freeze question (issue 16). I have **not** measured whether any of
these seven appear in the slow-game deck lists, and the r106 correlation
points at a different deck with a different mechanic. Someone should check
before anyone connects them.

## Filed, not fixed

819 cards is not a tick. Enforcing sorcery-speed and once-per-turn across a
third of the activated-ability corpus changes what every deck can do and what
the AI may legally choose, and the natural fix — populate
`Activated.TimingRestriction` from the parser — is an **AST exporter change**,
which is already an item held for a human (issue 13). Sequence it there.

The audit is reproducible: `scripts/audit_timing_restriction_r110.py`,
run with `python3 -I` against `~/hexdek/data/rules/ast_dataset.jsonl` on .206.

## Lesson 62

**A null field means one reader is blind, not that every reader is.** The
cast-filter suspicion was built from "`target_filter` is null, therefore the
filter is lost" — and a different branch recovered it from the raw text. The
same round found the mirror case: the timing restriction is *present* in the
AST and *still* unenforced, because the thing that reads it is a no-op and
the thing that enforces reads a different field. **Presence and absence in a
data structure both tell you nothing until you have found every reader.**
