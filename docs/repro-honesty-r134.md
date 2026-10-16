# r134 — the diagnostic told you to do something impossible

Requested first by Titanic Mistake ("5 is easier let's prioritize
that"), ahead of the exporter round.

## The text

Every `liveness/wall_clock` violation ended with:

> game STILL RUNNING at the 10m0s liveness budget (seed=… game=…) —
> abandoned by watchdog; **repro and sample-profile this seed**

That instruction cannot be followed, and it had been sitting in the
violation records — `data/judge/grinder-violations.jsonl` on .206 —
telling whoever read one to go and do an impossible thing.

## Why the seed is not enough

The seed *does* seed the game RNG, so it fixes the shuffle, the
mulligans and the opening seat.

It does **not** fix the play. Each seat's hat is built from Curse DNA
drawn out of a **mutable, evolving pool** using a **different RNG** than
the game seed:

```go
pool := sm.getOrCreateCursePool(deckKeys[i], rng)   // outer rng, not gameRng
dna, dnaIdx := pool.SelectForGame()                 // pool mutates across the run
```

Replaying a seed gives you the same opening and a differently-played
game. This was established in r131 while deciding not to chase the
22-minute game from 09/19 by replaying its seed.

## Why logging the DNA index would not fix it either

The obvious patch — record `dnaIdx` next to the seed — would be a
**second false promise**. Index N addresses a different DNA once the
pool has evolved, so a later replay would silently use something else.

Making these games genuinely replayable needs **deterministic DNA
selection derived from the game seed**. That is a design change with
real consequences for how the Curse pool evolves, so it is filed for a
human rather than done here.

## What changed

The message now states what the seed is actually good for, and where to
triage instead:

> abandoned by watchdog; the seed reproduces the SHUFFLE but NOT the
> play (hat DNA comes from a mutable pool on a different RNG), so a
> replay is a different game — **triage from deck_keys and the abort
> census instead**

Both of those exist: `deck_keys` is already written into every
`judgeViolationRecord`, and r131 made the heap abort print its own
seed, decks and object census.

`Context["seed_reproduces"] = "shuffle_only"` records the same caveat
machine-readably, so a future triage tool does not have to rediscover
the disappointment.

## The detector, written in the same round

Two regressions, **both red pre-change** — no vacuous guards in this one.

The second is **derived rather than listed**: it reads every
non-test `.go` file in `internal/judge` and fails if any of them
instructs a reader to repro, replay, reproduce or re-run *this seed*,
however phrased. A listed check on the one original string would be
re-broken by the next person who writes "replay this seed" in a new
message.

The caveat text itself is not caught, because it describes what the
seed does **not** do.
