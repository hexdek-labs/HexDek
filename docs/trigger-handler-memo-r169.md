# r169 — the per-Card trigger-handler memo

## What it changes

`per_card.fireTrigger` dispatches a triggered ability by walking **every
permanent on every battlefield** and asking the registry whether that
card's name has a handler for the event. Per permanent, per event, that
question costs an `RWMutex` read lock, a `sync.Map` load keyed on the
display name, and one or more string-map probes.

The overwhelming majority of permanents have **no handler at all**, so
the overwhelming majority of that work exists to produce the answer
"no" — and the answer is constant for the life of a (card name,
registry) pair.

r169 memoizes it on the `Card`, keyed on **(DisplayName, registry
generation)**.

```
before:  RLock → lookupCandidates(name) → reg.onTrigger[k] × N  → RUnlock
after:   one pointer load, a uint64 compare and a string compare
```

## Where the number came from

A 30s CPU profile on the **r168 binary** in a **verified clean band**
(474.1 → 513.7 g/min across the sample, no halving) prices the collect
closure at **4.13% of all CPU**.

That figure is the whole addressable part, and saying so required
killing a more attractive reading first. `FireCardTrigger →
per_card.fireTrigger` is **16.27% cum**, four times larger — but
`-peek` attributes **69.40%** of it to `PushPerCardTrigger`, which
recurses into `ResolveStackTop` (55.92%) and `PriorityRound` (46.89%).
That is the engine genuinely resolving the triggers. It is real work,
not overhead, and optimising it would mean changing what the game does.

Two other leads died the same way and are recorded here so they are not
re-walked:

| lead | why it is dead |
|---|---|
| cache `cardIsToken` (3.46% flat) | token-ness is **added** at six sites and ~93 `Types:` literals, sometimes after construction, and never removed. Monotonic-true: a cache may remember `true` but never `false`, and in hand/graveyard/exile/library almost every card is `false`. **The memo would miss exactly the hot case.** |
| skip `sba704_5d`'s four-zone scan (5.65%) | the skip needs a gate on "a token entered a non-battlefield zone", and the Issue Log documents per_card handlers that **bypass the canonical zone-move API**. A gate on the canonical path would miss them and leave a token alive in a graveyard — a silent rules violation. Needs a zone-write audit first; **filed, not guessed at.** |

## The key, and why not a readiness bool

Two things can change the correct answer.

**The name.** `Card.Name` is mutable: the clone and copy paths append
riders (`"Crown of Gondor (Urza copy)"`), which is the entire reason
`lookupCandidates` exists. A memo keyed on card identity alone would
serve the pre-rename table forever.

**The registry.** `Reset()` installs a brand-new registry and replays
`registerDefaults` plus every `AddResetHook` callback. Tests do call it.
Any registration at all can add a handler for a name that previously had
none.

So the key is (DisplayName, generation) and never a "have we computed
this" bool. r157 spent a whole round on exactly that mistake — three
caches guarded by readiness flags kept serving a DFC's front face for
the rest of the game after a transform reassigned the AST. A key made of
the things the answer is derived from makes the staleness impossible to
reintroduce, rather than something a future author has to remember.

The generation is a **package-level** counter, not a `Registry` field,
so that a memo built against an old registry can never be mistaken for a
current one after `Reset()` swaps the pointer. Generations are never
reused.

## Why the stored table is a copy

A memo hit takes **no lock**. That is only sound if what it hands back
can never be mutated by a concurrent registration, so the table is
copied — map and slices — at build time under the read lock, and is
immutable thereafter. Cards *with* handlers are the small minority and
the copy happens once per (card, generation), so it costs nothing on the
path it exists to speed up: a card with no handler memoizes a nil table
and allocates nothing.

`Card` is the right home for this. It already carries the r157, r158 and
r164 lazily-written caches, on the same assumption — Card objects are
per-game, decks are cloned per game — and that assumption is covered by
the gameengine and hat race gates. The slot is typed `any` because
`per_card.TriggerHandler` lives in a package that imports gameengine; a
typed field would be an import cycle. gameengine never inspects the
value.

It is deliberately **not** copied by `Clone` (clone.go's explicit field
literal). A clone simply re-memoizes on first use.

## Tests

Every dispatch test starts at the production entry point,
`gameengine.FireCardTrigger`, on a real board, and asserts **the handler
ran**. Not one of them calls `triggerTableFor`.

That rule is r168's. r167 shipped a field that compiled, had five green
tests and printed nothing through two real capped games, because all
five hand-fed the value the feature existed to transport. A memo is the
same shape of risk: a test that asks the memo what it remembers will
agree with the memo about everything, including being wrong.

| test | what it pins |
|---|---|
| `HandlerFiresOnEveryEventNotJustTheFirst` | the memo does not turn a repeated dispatch into a single one |
| `RegistrationAfterTheMemoIsBuiltStillDispatches` | the generation key really invalidates — **the control target** |
| `RegistrationBumpsTheGeneration` | the premise of the above, as its own assertion (r162) |
| `RenamedCardDispatchesUnderItsNewName` | the name half of the key |
| `ParentheticalRiderStillFindsTheBaseCard` | the memo preserved the **candidate search**, not just an exact-name probe |
| `ACardWithNoHandlerStaysSilent` | the negative direction — the memo does not invent a dispatch |
| `ADifferentEventOnTheSameCardIsNotServedTheWrongHandlers` | it is a table, not a single answer |
| `ANewRegistryBumpsTheGeneration` | the `Reset()` swap itself |

Plus two in the **caller's** package (r163): one drives
`FireCardTrigger` and asserts the counters the grinder line reads
actually move under real dispatch; one asserts on source that the log
line still asks for them.

**A trap worth recording.** The first version of these tests saw the
handler fire once and then never again, and the memo was innocent — the
same thing happened on an unpatched tree. `addPerm` builds a **0/0**,
which dies to state-based actions (CR §704.5f) the moment the first
dispatch resolves and runs SBAs. The subject left the battlefield after
one event, and every later fire found an empty board. That looks exactly
like a memo that stopped dispatching. Checking against HEAD before
believing it cost one command and saved a false root cause.

## Controls, 3/3 red

| control | what it disables | red |
|---|---|---|
| A | **the generation is pinned** — `bumpRegistryGeneration` becomes a no-op | 3, including the dispatch test |
| B | the **name** is dropped from the key | 1 |
| C | the **candidate search** is replaced by an exact-name probe | 2 |

A is the mandatory one: it is precisely the trap the design exists to
avoid, and the test that catches it fails on a *dispatch* assertion, not
on a check of the generation counter. The premise is pinned separately
so a reader does not have to infer the mechanism from a behaviour
failure.

C turned a **pre-existing** test red as well
(`TestFireTrigger_DispatchesThroughCascadeRename`), which is the best
evidence in this round that the candidate search is load-bearing and
that the memo preserved it rather than quietly narrowing it.

**r169 is NOT instrumentation.** It changes dispatch, so the behaviour
axis genuinely applies and these controls mean something — unlike r167
and r168, which could not be red on that axis at all.

## Published before the measurement

- **Hit rate.** Predicted **>99%**: the generation moves only during
  init and test `Reset()`, and card names are stable within a game.
  **Revert threshold: below 90% the memo is not earning its key and
  comes out.** r155 predicted a cache hit rate under 25% and measured
  98.1%; r164 had to correct its own prediction in the other direction.
  Predicting is free, so the number is published either way — as
  `tmemo=` on the grinder line, per window.
- **Throughput.** The closure is 4.13% and the memo does not reduce it
  to zero — a hit still costs a `DisplayName()`, a type assertion, a
  string compare and two atomics. Predicted **+2% to +3%**.

  That is **at or below the resolution of a single-segment
  comparison**: the clean-band medians of four consecutive segments span
  488.1 → 502.1 g/min on rounds that were pure instrumentation and
  *cannot* have been faster. So a segment-to-segment reading cannot
  settle this round, and an A/B of two tournament binaries at
  `--workers 1` is run instead.

## ELO

**Comparable.** No evaluation, scoring, targeting or decision path is
touched, and the controls above exist to prove the set of handlers
dispatched is unchanged.
