# r123 — a counter-removal cost the parser cannot read is a cost that is never paid

## What I was asked to do, and why I did something else

(17b-CONT) ranked **the 6 Mana Batteries first**. There are five, not six, and
**they appear in 0 of 1,758 live decks.** The ranking was by apparent
tractability, not by exposure. Measuring the exposure first is what turned
this round into something worth shipping.

| card | live decks |
|---|---|
| Black / Blue / Green / Red / White Mana Battery | **0** each |
| Calciform Pools, Dreadship Reef, Molten Slagheap, Fungal Reaches, Saltcrusted Steppe | 3, 5, 5, 1, 0 |
| Grand Warlord Radha | 1 |

15 of 1,758 decks hold any of the eleven.

## What the bailout actually is

Chasing *why* those eleven bail found a shared root, and the root is a live
bug well outside them.

`activation.go` pays a "remove N \<kind\> counters" activation cost
(CR §602.1b) through `RemoveCounterCostSpec`, which returns `ok=false` for
any shape it cannot parse with confidence. That decline is deliberate and
correct in intent — enforcement must never wrongly **block** a legal
activation. But **the boundary of the parser is the boundary of the
enforcement**: a declined cost is not paid at all, so the ability is free.

Measured over the real corpus (`ast_dataset.jsonl`, 32,418 entries):

```
activated abilities with a counter-removal cost in Cost.extra   347
  parsed by RemoveCounterCostSpec (the cost IS paid)            272
  DECLINED — the cost is NEVER paid                              75
structured remove_counters_n populated anywhere                   0
```

The last line is the r117 shape again: `gameast.Cost` defines
`RemoveCountersN`/`RemoveCountersKnd`, `RemoveCounterCostSpec` checks them
first, and **nothing in the pipeline ever populates them.** The exporter emits
a `remove_counters` key (different name, always null) and the whole mechanism
runs on the raw-text fallback.

## The 75, bucketed

| n | shape | status |
|---|---|---|
| 26 | `remove X <kind>` | self-limiting — see below |
| 20 | targets **another** permanent (`remove a counter from a permanent you control`) | out of this signature's reach |
| 18 | `remove any number of <kind>` | self-limiting |
| 9 | `remove all <kind>` | needs the permanent's current count |
| **2** | **a fixed count the word list did not contain** | **the live bug** |

"X" and "any number" are self-limiting: removing zero is a legal payment, and
every such effect scales off "counters removed this way", so a free
activation does nothing. **A fixed count is not self-limiting.** The word list
stopped at "ten", and two printed costs sit outside it:

- **Fire Nation Turret** — `remove fifty charge counters from this artifact:
  it deals 50 damage to any target`. **No mana cost. No tap cost.** The
  counter removal is the entire cost.
- **Trenzalore Clocktower** — `{1}{U}, {T}, remove twelve time counters …`

## Measured before the fix

```
Fire Nation Turret: 5/5 activations ACCEPTED with 0 charge counters;
                    opponent life 20 -> -30; counters after = map[charge:0]
```

A free, unbounded, instant-speed kill. **It is in 3 live decks.** This is
exactly the non-termination hazard the comment at the call site warns about
("any such ability whose only cost was the counter removal … was FREE and
unboundedly activatable → board-explosion non-termination, firespot r63") —
in the subset that comment's own helper declines.

A second probe, for contrast: **Sage of Hours** (`remove all +1/+1 counters:
for each five counters removed this way, take an extra turn`) also accepted
5/5 free activations, but produced **0** extra turns — the effect's `for_each`
resolves to zero. Free, but inert. That is the self-limiting shape, and it is
why r123 does not touch the "all" bucket.

## The fix

`parseSmallCountWord` now reads the full English number-word range rather than
stopping at ten. Strictly additive: it widens what is recognised as a **fixed**
count and changes nothing about the variable shapes.

## Deliberately NOT fixed, with the reason

- **"remove all" (9 abilities).** The count is the permanent's current total,
  which `RemoveCounterCostSpec(c gameast.Cost)` cannot see. Worth doing, but
  8 of the 9 effects scale off "the number of counters removed this way",
  which the effect side also cannot see — so paying the cost alone would
  remove the counters and leave the effect reading zero. **That is the r109
  trap: do not improve one half of a coupled pair.** Cost and effect have to
  land together.
- **"X" / "any number" (44).** A player choice, not a number. Self-limiting as
  above.
- **the other-permanent form (20).** Needs a target choice.
- **the 11 bailout mana cards.** Same coupling: the colour is known, the count
  is the player's choice of counters to remove. Per the r109 order note,
  emitting the printed colours while the count stays at 1 makes them STRICTLY
  WORSE, because the fallback's single mana lands in `Any`, which is a
  universal wildcard. They stay on the fallback until the count is real.

## Verification

Three regressions red pre-change — the activation accepted at 0 of 50, the
second activation accepted after the counters were spent, and 11 of 12
accepted. The fourth test is a **preservation guard** (the variable shapes must
keep declining) and is green both ways by construction; saying so rather than
counting it as a both-directions proof.

## Lesson

**A guard that declines what it cannot parse has two failure modes, not one.**
This one was written to never produce a false positive, and its doc comment
says so plainly. Nobody counted the false negatives: 75 abilities, one of them
a free repeatable 50-damage kill in live decks. **When a helper documents that
it declines a class, go count the class.**
