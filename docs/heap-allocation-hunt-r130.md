# r130 — one deck is in 83% of every slow game we have

r129 fixed the heap guard. This is the hunt for what the guard is
supposed to be catching. **It is not finished — no reproduction yet —
and this file is the evidence so far, not a conclusion.**

## The instrument

An aborted game still logs its `SLOW GAME` line, and the line's `turns=`
matches the abort turn, so an abort can be joined to its deck list.
Only **2 of 18** aborts join (the rest completed under the slow
threshold), and both contain the same deck.

n=2 proves nothing, so the base rate decides it. Across all **963**
`SLOW GAME` lines in the log:

```
count  median_s  max_s   deck
  797     29.2    620.6  moxfield/sorin_of_house_markov_sorin_ravenous_neonate_b2_khazad_doom_JfUAlsri
   55     39.2    397.0  moxfield/sokka_and_suki_b3_brettman1908_FmvOBmQ5
   52     26.7    293.7  moxfield/sovereign_okinec_ahau_b2_buzzkillington0_mIWQKy82
```

**797 of 963 — 83%.** Expected if slow games were uniform over the 1,421
pooled decks: **2.7**. That is a ~300× enrichment, and the next deck
down is at 55.

## Two things this is NOT

**It is not "the slowest deck".** Measured both ways, its slow games are
*shorter* than everyone else's:

```
slow games WITH sorin     797   median 29.2s   p90 124.4s   max  620.6s
slow games WITHOUT sorin  166   median 58.9s   p90 658.6s   max 1162.0s
```

It is the most **frequent** offender, not the most severe one. A deck
that shows up in 83% of slow games while being individually milder is a
different shape of problem — it suggests something that happens *often*
and sometimes runs away, not something that is always catastrophic.

**It is not a new deck.** The first `SLOW GAME` containing it is
2026/10/13, which initially looked like it coincided with the abort
escalation (1 on 09/22, 3 on 10/13, 5 on 10/14, 9 on 10/15). It doesn't:
the deck file is dated **2026-06-07**, and its first appearance anywhere
in the log is **2026/09/19**, in a judge-watchdog liveness timeout. The
10/13 date is an artifact of when SLOW GAME lines start, not of when the
deck entered the pool. Claim withdrawn before it reached a ranking.

## The strongest single record

```
2026/09/19  judge-watchdog: LIVENESS — sampled game seed=1789817169370804852
            STILL RUNNING at 10m0s budget
2026/09/19  game ran 22m37.576s against a 10m0s liveness budget, turn 61
            — non-termination or pathological slowdown
```

Decks: this Sorin deck, dina_soul_steeper, arcades_the_strategist,
go_shintai_of_life_s_origin. **It carries a seed**, which is a
reproduction recipe and the obvious next step.

## The rules question, researched before touching anything

The deck contains both halves of a known mandatory loop, oracle text
read from the corpus rather than memory:

- **Bloodthirsty Conqueror** — "Whenever an opponent loses life, you gain
  that much life."
- **Vito, Thorn of the Dusk Rose** — "Whenever you gain life, target
  opponent loses that much life."
- (and **Marauding Blight-Priest**, the 1-life variant)

**The correct ruling is that this loop TERMINATES, and it is not a
draw.** §104.4b makes a loop a draw only when it repeats "a sequence of
events with no way to stop". Here the targeted opponent's life strictly
decreases every iteration, so there is a way to stop: §704.5a removes
them at 0 or less, and §117.5 / §704.3 require state-based actions to be
performed — and repeated — before any player gets priority and again
before triggered abilities go on the stack. Vito then has no legal
target.

So an engine that ends such a game as a §104.4b draw is giving the wrong
answer, and one that loops forever is giving no answer. **Which of those
happens here is exactly what the reproduction has to establish** — the
trigger machinery is guarded (`maxTriggerDrainIterations` 500,
`maxTriggerFiresPerTurn` 1000, `maxResolveDepth` 50, a wall-clock
deadline), and `drainPendingTriggers` ends the game as a §104.4b draw on
trip. Those caps are live and reset per turn in
`internal/tournament/turn.go`.

## Reproduction attempts so far — both negative, reported as such

| run | games | wall | peak RSS | loop guards | draws |
|---|---|---|---|---|---|
| sorin + the 3 decks from the 10/14 abort | 25 | 1:25 | 146MB | none | — |
| sorin + the 3 decks from the 10/13 abort | 400 | 26:46 | 159MB | **0** | **0** |

425 games, peak 159MB. Nowhere near the 2GB per-game budget, let alone
15GB. No loop guard fired and no game ended as a draw.

**That negative is close to uninformative about the explosion** and
should not be read as exoneration: 18 aborts against an order of 10^5–10^6
games is roughly 1 in 30,000, so 425 games had a ~1% chance of catching
one. It is recorded so the next round doesn't repeat it.

## What the run did establish, against a proper control

The A/B control run from r129 is the right comparison — same binary,
same `--workers 1`, same box, same server contention, decks chosen for
an unrelated reason:

```
A/B control table   200 games /   55.2s  =  0.276 s/game
sorin table         400 games / 1606.2s  =  4.016 s/game
                                            14.5x slower per game
```

So the deck's reputation is real and it is a **time** problem, measured
rather than inferred: a table containing it runs ~15× slower per game
under identical conditions. What it is *not*, across 425 games, is a
**memory** problem — and the heap explosion is what we were chasing.

Those two may still be the same underlying thing seen at different
magnitudes, but on this evidence they are separate findings, and the
15× slowdown is the one that is now measured.

## Next, in order

1. Replay `seed=1789817169370804852` with its four decks — the one
   record that names a 22-minute game.
2. If it reproduces, take a local `go test -cpuprofile`/heap profile;
   there is no pprof on .206.
3. Only then decide whether the defect is a missing SBA check inside the
   cascade (loop never terminates) or a §104.4b draw being awarded to a
   loop that should have killed a player.
