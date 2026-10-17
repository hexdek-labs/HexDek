# r166 — the wall-clock backstop inside the token loop

## What was wrong

Two production games on 2026-10-17 ran **2199.1s** and **1442.7s** against a
600-second cap. The guard counter tells us exactly how badly:

```
13:19:23  guards=game_deadline:1     the 610.4s game
14:12:23  guards=game_deadline:2     the 2199.1s game, completed 14:12:01
14:22:53  guards=game_deadline:3     the 1442.7s game, completed 14:22:46
```

`game_deadline` bumps **once per game, on first breach** (the `ended == 1`
early return added by r161 precedes `LogLoopGuardFired`). Both later
counters therefore bumped *in the window where their game completed* —
26 and 14 minutes after the deadline had passed, with **not one of the
three check sites reached in between**: `turn_phase_boundary`
(`tournament/turn.go`), `stack_drain` (`stack.go`), `trigger_drain`
(`trigger_batch.go`).

The guard fired on the way out the door.

## What it actually is

Twelve goroutine samples over six minutes, taken by a read-only watcher
that polls the grinder window and dumps when throughput collapses. In
every one of the twelve, one worker sat on the same `GameState`
(`0x1dda3eb5208`), the same `Permanent`, the same seat and the same
ability index, while the other worker showed a different `GameState`
each time:

```
ActivateAbility(gs, 3, perm, 1, nil)
 └ DrainStack
    └ ResolveStackTop
       └ resolveActivatedAbility
          └ ResolveEffect
             └ resolveCreateToken            47.55% cum in-stall
                └ FirePermanentETBTriggers   48.47% cum in-stall
                   └ FireCardTrigger → per_card.fireTrigger
                      └ the collect closure → lookupCandidates
```

`resolveCreateToken` calls `FirePermanentETBTriggers` **inside its
per-token loop**. Each ETB fires several `FireCardTrigger` events, and
each of those walks **every permanent on every battlefield**, doing a
registry lookup by card name. The tokens join that scan as they are
minted, so creating N tokens onto a board of M costs O(N·M), and once
the tokens dominate the board, O(N²). One of the stalled decks is
**Krenko, Mob Boss**.

That accounts for the whole fingerprint:

| signal | stalled game | normal game |
|---|---|---|
| characteristic lookups | 3,639,118,439 | ~460,000 |
| cache hit rate | 100.0% | ~98% |
| trigger-guard counters | frozen | advancing |
| allocation per window | ~400MB | ~2,200MB |

Nothing is looping forever and nothing is broken. The engine is doing
exactly what the cards say, and what the cards say is enormous. No
counter-based guard can catch it, because every iteration makes real
progress — which is precisely the case the wall-clock deadline exists
for.

## Why the existing check could not see it

`DrainStack` **does** check the deadline, every iteration, and the
comment above that check says it is there to bound "a spin in something
below us that nobody capped". The check is correct. It is simply on the
wrong side of the loop body: it runs **between stack items**, and all of
this happens inside a **single resolution**.

A smoke alarm wired to go off between fires.

## The fix

One `CheckGameDeadline(gs, "token_create")` inside `resolveCreateToken`'s
per-token loop. On a breach the loop bails, `mintEvent.FinalCount` is
corrected to the number actually minted, and the game ends as a draw —
CR §104.4b, the same basis the deadline's own header already cites.

**Why the loop and not the dispatch.** A single `FirePermanentETBTriggers`
call returns in O(board); even at a million permanents that is
sub-second. Twenty-six minutes therefore cannot be one dispatch — it has
to be many iterations. The minutes accumulate across the loop, so the
loop is where the check belongs. Putting it inside the per-permanent
scan would cost a clock read per permanent per event and buy nothing.

**Opt-in, exactly as r161.** A game with no armed deadline is completely
unaffected: `GameDeadlineExceeded` is false, so tests, `paritycheck` and
the correctness harness mint every token as before. Only a caller that
arms a deadline — the live grinder, at `showmatch.go:1387` and `:2014` —
can trip this.

## What this does NOT fix

- **The quadratic itself.** This bounds the damage at ten minutes; it
  does not make token creation cheaper. The per-Card handler-set memo
  (cache the resolved `byEvent` per Card, keyed on display name plus a
  registry *generation* counter, never a readiness bool) attacks the
  constant on this exact path and is the next round.
- **Other stalls.** Twelve samples is one stall episode sampled twelve
  times, not twelve independent stalls. Every sample was this shape; no
  claim is made that every stall is.
- **Reproducibility.** `SLOW GAME` carries no seed, so none of these
  games can be replayed. Adding the seed to that line is a cheap round
  and is not done here.

## Verification

Controls 3/3, each reverting semantics only:

| control | what it disables | tests red |
|---|---|---|
| A | the guard entirely (full revert) | 3 |
| B | the `break` — report the breach but keep minting | 3 |
| C | routes through `GameDeadlineExceeded` instead — bail **silently**, no guard event, no game end | 1 |

C is the one worth keeping: a silent truncation would make tokens vanish
mid-resolution while the game carried on, which is a rules change
wearing a performance fix's clothes. It is caught by the counter and by
the `ended` flag, not by the token count.

This is **not** pure instrumentation — it changes behaviour, so the
behaviour axis genuinely applies and the controls are meaningful.

## Prediction, stated before measuring

The next game that would have run 2000 seconds finishes near 600, and —
this is the real test, and it is checkable from the log alone —
**`guards=game_deadline` bumps in a window where throughput is still
halved**, rather than in the window where the `SLOW GAME` line appears.
A bump that again coincides with completion means this check is not
being reached either, and the round has failed.

ELO comparable: no evaluation, scoring or decision path is touched.
