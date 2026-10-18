# r180 — the ETB observer audit, and the number it sent me to instead

Two findings. The first is the audit I set out to do. The second is a
measurement I took to size the first, which says the first is not where
the machine is actually going.

## 1. The per-ETB full-battlefield scan: a class of two, plus the scan itself

r179 filed `FireSoulbondTriggers` as an O(n^2)-over-a-token-swarm defect:
fired per creature ETB, walks the controller's whole battlefield asking a
layer-aware keyword question, all for a 2012 mechanic nearly nobody plays.
The open question was whether that shape was one card or a class.

I read every hook the ETB dispatcher calls (`etb_dispatch.go:180-300`).
The answer is **a class of two**, and the class is defined by one property:
**what the hook gates on before it starts scanning.**

Correctly gated — these test the ENTERING permanent first, so a swarm of
vanilla creature tokens short-circuits every one of them:

| hook | gate |
|---|---|
| `FireOutlawETBTriggers` | `PermIsOutlaw(perm)` |
| `OnEnchantmentETB` (eerie) | `enteredPerm.IsEnchantment()` |
| `OnConstellationETB` | enchantment |
| `OnDayboundOrNightboundETB` | daybound/nightbound card |
| `PerformExploitETB` | exploit creature |

Correctly latched — these scan, but only until a sticky per-seat flag is
set, after which they are O(1) forever:

| hook | latch |
|---|---|
| `CheckAscend` | `citys_blessing` (10 permanents) |
| `CheckStoried` | `enduring_story` (3 qualifying) |

**Ungated — the class:**

| hook | gate | why it fails |
|---|---|---|
| `FireEvolveTriggers` | `newCreature.IsCreature()` | rejects nobody in a creature-token swarm |
| `FireSoulbondTriggers` | `newCreature.IsCreature()` | same |

Both then walk the controller's entire battlefield. Both are r172 reorder
sites: that round swapped the conjuncts so the cheap rare-keyword test runs
before `IsCreature`, which cut the constant and left the scan.

They are the same defect twice, and they need the same fix: a per-seat count
of evolve/soulbond sources, maintained across enter, leave, grant, loss and
control change. That is new tracked state, which is why r179 filed rather
than fixed it. **Nothing found in this audit changes that call** — it just
doubles the payoff, because one piece of tracked state serves both.

### The scan nobody filed

`fireObserverETBTriggers` itself (`observer_triggers.go:55`) is the larger
instance of the same shape, and it is not gated at all: for **every**
permanent entering, it walks **all four seats'** battlefields, copying each
seat's battlefield slice (`make`+`copy`, four allocations per ETB), and for
every permanent walks its whole AST ability list doing string comparisons —
`EventEquals(trig.Trigger.Event, "etb")`, plus `DisplayName()` and three
`HasTriggerHook` map lookups per candidate.

Unlike the keyword hooks, this one has a fix that needs **no new tracked
state and no invalidation**: "does this card's AST contain any ability that
could ever be an observer-ETB trigger" is a pure function of `Card.AST`,
which is immutable (a DFC transform reassigns the pointer, which an
AST-keyed memo already handles — see r157). The overwhelming majority of
permanents, and 100% of vanilla tokens, answer no and could be rejected on
one memo hit instead of an ability walk.

Filed, not built, because of finding 2.

## 2. The number I did not have: strings.Contains is ~14% of the machine

I pulled the newest live captures to size the ETB class. They say something
else.

```
                              clean (r169)   partial hold (r179, 15:11)
strings.Contains (cum)           13.71%            14.18%
hat.EvaluateDetailed (cum)       24.64%            23.82%
Permanent.HasKeyword (cum)        9.24%            10.17%
```

**This is flat across clean and degraded windows, so it is not a stall
symptom — it is a constant tax on every window the machine ever runs.**
That also means it does not explain the 59% held, and I am not claiming it
does. It is simply the largest single addressable block of CPU I have
measured, and it was sitting underneath the stall work the whole time.

Every caller of that 14% is in `internal/hat`, and every one of them is
re-scanning the same immutable oracle text for the same card on every
scoring pass:

```
scoreThreat               11.37% of Contains
scoreStaxLock              9.41%
bestOpponentRemovalScore   9.24%
scoreDrainEngine           8.35%
scoreOpponentGraveyard     7.82%
scoreActivationTempo       4.17%
survivabilityMultiplier    4.09%
...
```

`HasKeyword`'s 10.17% is the same story from the other side: 44.73% of it is
`hat.evasionMultiplier`, 24.04% `hat.survivabilityMultiplier`, 12.39%
`hat.infectToxicMultiplier`. None of it is the rules engine.

`OracleTextLower` is already memoized and AST-keyed (r157), so the cost is
not building the string — it is the 16-needle `strings.Contains` sweep over
it, per permanent, per evaluation.

### Why this one is safe to fix and soulbond is not

r164 already built the mechanism: `(*Card).TextPredicate(bit, compute)`
caches a boolean in a 64-bit set on the Card, **keyed on `card.AST`**. Six
bits are in use. The r164 header states the discipline: a predicate may only
come through the memo if it is a pure function of the lowered oracle text.

That key is why this works where soulbond's does not. `charCacheEpoch` is
bumped by any characteristics mutation including a permanent entering, so an
epoch-keyed memo hits 0% precisely in the swarm that needs it.
`card.AST` does not change when a permanent enters. It changes when the card
transforms, which is exactly when the answer should change.

The r164 note also records that `strings.Contains` was **21.13%** of CPU when
that work started. It is 13.7-14.2% now. The mechanism works; it was just
never extended past six predicates.

## Status

- Soulbond + evolve: **still an owner call** (Josh / 7174n1c), unchanged, now
  worth twice as much because one piece of tracked state fixes both.
- `fireObserverETBTriggers` AST memo: filed, buildable without owner sign-off,
  not yet built.
- The hat predicate cache: extending bits 6-25 across the four largest
  consumers, each behind a differential test against a verbatim copy of the
  original scorer. `scoreThreat` (the largest, two pure clusters at
  `evaluator.go:2310` and `:2408`) is the next tranche.

## Not proven

r179's hang fix has 1h42m clean (pid 251413, started 13:40:45, 397% CPU with
the log growing normally). I said several hours would be proof; this is not
that yet. The daily heap-corruption crash last fired at 06:27 today, before
the 13:40:49 r179 deploy — but the previous one was Oct 13 17:12, a five-day
gap, so 1h42m of silence says nothing about it either way.

---

# CORRECTION (r181): the audit was incomplete, and the class has three members

**My r180 audit was wrong by omission.** I read every hook called from
`etb_dispatch.go` and `observer_triggers.go` and declared the class closed at
two. I never read `FirePermanentETBTriggers` (`etb_dispatch.go:91`), which is
a *different* entry point that also runs on every permanent entering and
calls six hooks of its own.

What caught it was not re-reading the code. It was a **goroutine dump from a
live hold**: with four workers all runnable, worker 0's stack read
`ApplyGraft <- FirePermanentETBTriggers <- resolveCreateToken`. I had been
analysing CPU profiles from these captures for rounds and had never once
opened the goroutine dumps sitting beside them.

## The third member: `ApplyGraft` (`keywords_misc.go:1164`)

```go
if !perm.IsCreature() { return }          // rejects nobody in a token swarm
for _, seat := range gs.Seats {           // ALL FOUR SEATS
    for _, g := range seat.Battlefield {
        ...
        if _, ok := keywordArgCostStrict(g.Card, "graft"); !ok { continue }
```

Same gate as evolve and soulbond — "is the entering permanent a creature",
which rejects nobody in a creature-token swarm. **It is the worst of the
three: evolve and soulbond walk only the controller's battlefield, this one
walks all four.** Graft is a 2005 Ravnica keyword on a handful of cards, so
in essentially every game the entire scan is pure loss.

## Completed audit of `FirePermanentETBTriggers`

| hook | shape | verdict |
|---|---|---|
| `ApplyStaticETBCounters` | iterates the entering permanent's own abilities | fine |
| `ApplyEntersTappedUnless` | own abilities | fine |
| `ApplySelfEntersTapped` | own abilities | fine |
| `ApplyVanishingFadingETBCounters` | own abilities | fine |
| `ApplyAmplifyETB` | gates on `keywordArgCostStrict(perm.Card,"amplify")` first | correctly gated |
| **`ApplyGraft`** | **gates on `IsCreature()`, then scans all seats** | **the class** |

## What r181 changed, and what it did not

The controller test `perm.Controller != g.Controller` was the **last** of four
predicates, after a map lookup and a full ability-list walk with a `ToLower`
per keyword. It is an int compare and on a four-player table it rejects about
three quarters of every battlefield. Hoisting it to first turns most of the
scan into one comparison.

This is behaviour-preserving **without relying on any invariant** about which
seat's `Battlefield` slice holds a permanent: the body only ran when all four
pure predicates passed, so their order cannot change the outcome. (The
invariant does appear to hold — `ApplyControlChange` moves the permanent
between slices — but the fix does not need it.)

**The scan is still there.** `ApplyGraft` remains the third member of the
owner-call class, and the per-seat tracked state that would fix evolve and
soulbond would fix this too. The owner call is now worth **triple**.

## Method note worth keeping

The watcher has been capturing `goro-*.txt` and `goro2-*.txt` next to every
`cpu-*.pb.gz` this whole time. CPU profiles answer "where is time going";
the goroutine dumps answer "what is each worker actually doing", which is a
different and in this case more useful question. Read both.
