# r117 — §107.4c is void as filed, and the reason is much larger than the issue

I took §107.4c, the `Any`-pays-`{C}` hole, ranked first for six ticks. The
first check killed it, and what killed it is the finding.

**The engine does not enforce coloured mana costs on the main-phase cast
path.** Measured at the production enumerator: a `{U}{U}{U}` spell is offered
as castable to a player whose entire pool is three red mana.

## 1. §107.4c is unreachable — retracted

The issue was: `CanPayColored` adds `p.Any` to availability for every colour
including `"C"`, and `PayColoredPip` falls through to `p.Any` for a `{C}`
pip, violating CR §107.4c. That is still true of the code. It cannot fire.

Caller count over `internal/`, excluding comment lines and `_test.go`, and
confirming each symbol is defined (the r105 method):

```
function             defined   prod calls
CanPayColored              1            0
PayColoredPip              1            0
AddMana                    1           56
EnsureTypedPool            1           36
PayGenericCost             1           10
CanPayGeneric              1            3
PayManaCost                0            0      <- does not exist
```

`PayManaCost` is a name I had been carrying in the round brief. It is not a
function. That is r105's nine-nonexistent-functions lesson arriving again, in
my own notes this time.

**The typed `ColoredManaPool` is written heavily and its colour predicates are
never read.** Same shape as open issue (2), the write-only counter DB.

## 2. What the cast path actually does

`buildCastableList` (`tournament/turn.go:1324`) — the single production
cast-candidate enumerator — is two integers:

```go
availableMana := seat.Mana.Total()
...
cost := gameengine.CalculateTotalCost(gs, c, seatIdx)
if cost > availableMana { continue }
```

Payment is `PayGenericCost`, generic by name and by body.

Measured through that entry point, with a positive control so an all-offered
run cannot masquerade as a result:

```
CONTROL  cost 9 vs 3 mana                    -> not offered   (the total gate works)
r117 Red Spell      {2}{R}        on {R}{R}{R} -> offered      colour-legal: true
r117 Blue Spell     {2}{U}        on {R}{R}{R} -> offered      colour-legal: FALSE
r117 Triple Blue    {U}{U}{U}     on {R}{R}{R} -> offered      colour-legal: FALSE
r117 Five Colour    {W}{U}{B}{R}{G} on {R}{R}{R} -> not offered  (CMC 5 > 3, not a colour decision)
```

The last row matters: it is excluded on **total**, not colour, so it is not
evidence of a colour gate.

## 3. The structural cause

**The runtime `gameengine.Card` has no mana-cost string.** It carries
`CMC int` and `Colors []string` and nothing that records the pip breakdown.
There is nowhere to put `{2}{R}` as distinct from `{2}{U}`.

So `CalculateTotalCost` returns an int because an int is all there is to
return, and `CanPayColored`/`PayColoredPip` are dead because the cost side of
the typed-mana design was never built. They are the half of a feature whose
other half does not exist — which is why six ticks of ranking them first was
six ticks aimed at nothing.

## 4. The moderating fact, stated because it bounds the severity

The AI does carry a colour check — `gameengine.CanPayColoredCost`, a
heuristic over untapped lands — called from five sites in
`internal/hat/yggdrasil.go`. Attributing each to its enclosing function:

```
  L7313  ChooseResponse
  L7405  ChooseResponse
  L7928  stackResolutionSignals
  L7991  maybeCastDefensiveAnswer
  L8162  maybeCastReactiveRemoval
```

**All five are instant-speed response paths. None is in any
`ChooseCastFromHand`** — and there are four implementations of that method
(Greedy, MCTS, Octo, Poker), which is what `turn.go:983` hands the
colour-blind list to.

So the main-phase cast has no colour check at any layer: not the enumerator,
not the chooser, not the payer. Responses have one, in the AI, as policy.

## 5. Filed, not fixed — and this one needs a human

Enforcing colour requires putting pip data on the runtime `Card` and
threading it through cost calculation and payment. That changes **what every
deck in the pool can cast**, which is strictly larger than r105's alt-cost
cast driver — an item already held for a human for the same reason. It is
also the kind of change where the ELO pool's results before and after are not
comparable.

Not a tick, and not mine to start alone.

No engine change this round: one probe and this write-up. `go build ./...`
clean, `./internal/tournament/` green. Prod stays on r116.

## Lesson 69

**Start a filed issue by counting the callers of the function it names.**
§107.4c was ranked first for six consecutive ticks on the strength of a real
rules violation in real code, and one caller count — thirty seconds — would
have shown the round was aimed at an unreachable function and retargeted it
every one of those times. r105 taught "check for production callers first"
about code I was *about to write*; the same check belongs on the issue I am
*about to take*.
