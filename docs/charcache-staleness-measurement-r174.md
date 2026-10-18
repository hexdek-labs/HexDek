# r174 — how often does the characteristics cache actually return a wrong answer?

**Question:** r173 proved that five mutation paths bypass `charCacheEpoch` and that each
can produce a stale read. It did not measure how often that happens. This is the
measurement. The number is the input to the decision on whether three one-line fixes
are worth breaking ELO comparability.

Worktree `/private/tmp/claude-501/hexdek-a2`, branch `dev/r174-a2`, at `87a742ab`.
Nothing committed. All instrumentation listed in §6 has been deleted; the worktree
contains only this file.

---

## 0. Headline

Over **24,638,626 cache hits in 172 real 4-player Commander games**:

| | n (cache hits) | disagreements | rate | **effective** disagreements | effective rate |
|---|---|---|---|---|---|
| **A — main** (GreedyHat, 120 games) | 17,147,696 | 1,692 | **9.87e-5** (1 in 10,135) | 486 | **2.83e-5** (1 in 35,284) |
| **B — YggdrasilHat** (budget 150, 12 games) | 1,249,793 | 398 | 3.19e-4 | 84 | 6.72e-5 |
| **C — phasing/Humility-enriched decks** (GreedyHat, 40 games) | 6,241,137 | 4,895 | 7.84e-4 | 937 | 1.50e-4 |
| **all** | 24,638,626 | 6,985 | 2.84e-4 | 1,507 | 6.12e-5 |

*"Disagreement"* = a cache **hit** whose memoised `Characteristics` differs from a forced-fresh
recomputation of the same permanent in the same state, on any compared field.
*"Effective"* = the consumer that made the read actually consumes the field that differed
(e.g. `ToughnessOf` reading a stale `Toughness`, not `HasTypeOf` reading a permanent whose
only stale field is `Keywords`). The effective count is the honest outcome-relevant subset;
it is still an over-count — see §3.

**And the result that decides the question:**

> **Of the five r173 holes, four (`PhaseOut`/`PhaseIn`, `gs.Active`, raw `Permanent.Controller`,
> Humility's raw `ContinuousEffects` append) caused ZERO of the 6,985 disagreements, and the
> fifth (`gs.Phase`/`Step`) caused ZERO as well — despite `gs.Phase`/`Step` having moved
> between store and read on 11.2 million of the 24.6 million hits, and despite cohort C
> being deliberately built from decks containing Teferi's Protection / Humility / Out of
> Time / Oubliette / Teferi, Mage of Zhalfir, in which real phasing fired (794 hits saw the
> phased-out set change under a live cache entry).**
>
> **100% of the causally-attributable disagreements trace to holes that are NOT on the r173
> list of five.**

---

## 1. Causal attribution

Attribution is not "which inputs moved" (that is ambiguous — `gs.Phase` moves on ~46% of all
hits, so it co-moves with everything). It is a **counterfactual**: on each disagreement,
revert one witnessed input at a time to the value it had when the entry was stored,
recompute fresh, and ask whether the cached answer is now reproduced. The input that
reproduces it is the cause. If no single revert works, reverting all of them together is
tried; if that also fails, the cause is outside the witnessed set.

| cause | cohort A (120g) | cohort B (ygg) | cohort C (hostile) |
|---|---|---|---|
| **hole 1 `PhaseOut`/`PhaseIn`** | **0** | **0** | **0** (794 exposure hits) |
| **hole 2 `gs.Active`** | **0** (369 exposure) | **0** (9,026 exposure) | **0** (179 exposure) |
| **hole 3 `gs.Phase` / `Step`** | **0** (7,840,008 exposure) | **0** (573,805 exposure) | **0** (2,804,964 exposure) |
| **hole 4 raw `Permanent.Controller`** | **0** | **0** | **0** |
| **hole 5 Humility raw CE append** | **0** | **0** | **0** |
| **hole 7 — NEW: `GrantedAbilities` append** | **734** (43%) | **378** (95%) | **2,819** (58%) |
| hole 8 — raw counter add (r173 "undetermined") | 0 | 20 (5%) | 255 (5%) |
| not explained by any witnessed input | 952 (56%) | 0 | 1,819 (37%) |
| nothing witnessed moved at all | 6 | 0 | 2 |

### Hole 7 — NEW, and the largest single cause

`resolveGrantAbility`, **`internal/gameengine/resolve.go:2712`**:

```go
t.Permanent.GrantedAbilities = append(t.Permanent.GrantedAbilities, e.AbilityName)
```

`Permanent.GrantedAbilities` is folded into the layer base by `BaseCharacteristics` →
`foldGrantedAbilities` (`layers.go:362`), so it is a characteristics input. This site does
not call `InvalidateCharacteristicsCache`. The canonical EOT-grant path
(`mod_tail_grant_keyword_eot_r63.go:95`) *does* invalidate, which is why this was never
noticed; the AST `grant_ability` path does not. The audit also shows 6 further files with
`GrantedAbilities = append` and zero `InvalidateCharacteristicsCache` anywhere in them
(`clone.go`, `per_card/vecna_trilogy.go`, `per_card/obeka_support.go`,
`per_card/urza_chief_artificer.go`, `per_card/sac_outlets.go`, and 3 of 4 appends in
`keywords_batch6.go`) — not individually verified, listed as leads.

Observed instances (cached → fresh, i.e. the grant the cache failed to see):

```
Urdnan, Dromoka Warrior   []                 -> [first strike]
Heliod, Sun-Crowned       [indestructible]   -> [indestructible lifelink]
The Wind Crystal          []                 -> [flying lifelink]
Stromkirk Captain         [first strike]     -> [first strike deathtouch and lifelink]
Rapid Augmenter           [haste]            -> [haste unblockable]
```

All of these are "target creature gains X until end of turn" — i.e. **the engine grants the
keyword and then, for a window averaging ~40 reads, every consumer that asks still gets the
pre-grant answer.** It is a one-line fix (`gs.InvalidateCharacteristicsCache()` after the
append loop) and, unlike the r173 three, it is demonstrably load-bearing.

### Hole 8 — raw counter adds

r173 listed this as UNDETERMINED (~180 raw `AddCounter` sites on record from the r63 sweep).
It is **confirmed causal**: 255 disagreements in cohort C and 20 in cohort B, e.g.
`Abigale, Poet Laureate` reading 1/1 from cache against 2/2 fresh. Zero in cohort A.

### Hole 9 — NEW class: characteristic-defining abilities over unwitnessed state

The large "not explained by any witnessed input" bucket is **not** cascade in the main
cohort: at every one of those 958 events the harness swept the whole battlefield and found
**0 other permanents also stale** (`CO-STALENESS: 0-other-perms-also-stale 958`). They are
single-permanent, and the inputs that moved are outside the permanent entirely. The example
census identifies the shape:

```
Body of Knowledge  Power,Toughness  cached 3/3 -> fresh 4/4   (x3 distinct P/T levels)
```

> *Body of Knowledge's power and toughness are each equal to the number of cards in your hand.*

Nothing invalidates the characteristics cache when a card is drawn or discarded, so every
`*/*` CDA over a dynamic count is stale from the moment the count changes until something
unrelated bumps the epoch. This class is **not fixable by adding an invalidate at a
mutation site** — there is no bounded set of sites. It needs either an epoch bump on the
zone/count transitions those effects read, or exclusion of CDA permanents from the memo.

In cohort C the same bucket *is* largely cascade (1,216 of 1,819 events had 3–5 other
permanents simultaneously stale) — tribal anthem boards, where one stale source drags a
whole board of vampires with it.

**Honest limit on this bucket:** the harness has no revert for "a permanent entered or left
the battlefield", so any disagreement where the population also moved cannot be ruled in or
out by counterfactual. That is at most 121 of cohort A's 1,692 (the hole-lists containing
`x-population`). The other ≥831 unexplained events had no population change.

---

## 2. Staleness duration

Measured as the number of **consecutive disagreeing reads served from one stale entry before
an unrelated invalidation healed it** (the entry's next store closes the window).

| cohort | windows (n) | min | p50 | p90 | max | mean |
|---|---|---|---|---|---|---|
| A — main | 42 | 1 | 23 | 118 | 370 | 40.3 |
| B — Yggdrasil | 4 | 20 | 126 | 126 | 126 | 99.5 |
| C — hostile | 54 | 3 | 93 | 195 | 270 | 90.6 |

The "~440 invalidations per game heal everything quickly" intuition is **wrong at the tail**.
A stale entry typically survives tens of reads and the worst observed window served **370
consecutive wrong answers**. The windows are rare (42 in 120 games — roughly one stale
window every three games) but they are not short.

---

## 3. Outcome relevance

Raw consumer class is misleading: `A-sba` dominates every cohort simply because
`StateBasedActions` sweeps the whole board every pass. The meaningful filter is whether the
consumer read the field that was stale. Cohort A, effective 486 of 1,692:

| accessor :: stale fields | n |
|---|---|
| `HasKeywordOf` :: Keywords | 210 |
| `ToughnessOf` :: Power,Toughness | 110 |
| `HasKeywordOf` :: Power,Toughness,Keywords | 57 |
| `ToughnessOf` :: Power,Toughness | 60 |
| `sba704_5j` :: Keywords | 36 |
| `HasLostAllAbilities` :: Keywords | 7 |
| `PowerOf` :: Power,Toughness | 3 |

By class (cohort A, effective): `A-sba` 208, `A-combat` 56, `A-resolution` 26, `C-other` 196.
Cohort B adds `B-ai-eval` 9 of 84 — i.e. the AI evaluator is a *small* share of the
outcome-relevant reads; most land in real rules machinery.

The single most alarming caller, seen in both cohort A (42 hits) and C (303 hits):

```
HasKeywordOf < keywordActive < permHasSplitSecond < SplitSecondActive
```

A stale keyword read there decides **whether players may respond to a spell at all**.

**486 effective in cohort A is still an upper bound.** It counts, for example, a stale
`Toughness` read by `sba704_5g` (lethal damage) even when both the cached and the fresh value
sit on the same side of the death threshold, and it treats the three direct-struct readers
(`sba704_5j` etc.) as consuming every field. The number of disagreements that *flipped a
branch* was not measured and is some fraction of 486. A defensible reading of cohort A is
**order 10^-5 per cache hit, i.e. a few hundred branch-exposed wrong reads per 120 games**,
concentrated in a handful of long windows rather than spread thinly.

---

## 4. Validity of the measurement

Four controls, all passed:

1. **Positive control.** The harness was pointed at the exact r173 `PhaseOut` repro (2/2 bear,
   +3/+3 anthem, `PhaseOut` the anthem source). It detected the stale read and attributed it
   to `1-phasing` by counterfactual. A zero in §1 is therefore a measured zero, not a blind
   harness.
2. **Behavioural control.** The same 120 + 40 + 12 games were replayed with every probe nil.
   The per-game `(turns, surviving-seat signature, final battlefield size)` signatures are
   **byte-identical** to the instrumented runs, for both GreedyHat and YggdrasilHat cohorts.
   The harness does not perturb the games it measures.
3. **Oracle determinism.** On every disagreement the forced-fresh recomputation was run twice
   and the two results compared. **0 non-deterministic recomputes in 6,985 events.** "Fresh"
   is a stable oracle.
4. **Blind audit, for the prefilter's blind spot.** Running a full forced-fresh comparison on
   every hit is too slow, so the harness prefilters: it only compares when some witnessed
   input moved (that fires on ~47% of hits). To bound what the prefilter misses, 1 read in 25
   was compared **unconditionally**. Cohort A: **6 disagreements in 357,054 unconditioned
   reads**. Extrapolated over the 8,925,661 reads the prefilter skipped, that is ≈150 missed
   disagreements (Poisson 95% CI on 6 events, 2.2–13.1 → ≈55–330), so the corrected cohort-A
   total is ≈1,840 (1,750–2,020) and the corrected rate **≈1.07e-4 (1.02e-4 – 1.18e-4)**.
   The headline 9.87e-5 is a slight under-count; the correction does not change any
   conclusion.

Known limits, stated rather than hidden:

- `ForceFreshCharacteristics` snapshots and restores every battlefield permanent's memo slot,
  the epoch and the hit/miss counters, but it re-evaluates layer predicates an extra time.
  Control 2 shows no observable consequence; it is not a proof that no predicate anywhere has
  a side effect.
- Sample is 172 games on 16 moxfield decks (cohort A/B) and 16 filtered decks (cohort C), at a
  40-turn cap, driven through `tournament.takeTurnImpl`. `internal/hexapi` (the prod server
  path, which r173 notes calls `InvalidateCharacteristicsCache` zero times) was **not**
  measured. A hexapi-driven cohort would be the obvious follow-up and could move cohort A's
  zeroes — in particular hole 2 (`gs.Active`), whose only incidental bumps come from
  `tournament/turn.go`.
- Population change is not revertible, bounding attribution as described in §1.

---

## 5. Recommendation

**Do not spend an ELO break on the three one-line r173 fixes.** Across 24.6M cache hits in
172 games, including a cohort deliberately stacked with phasing and Humility, holes 1–5
caused **zero** observed wrong answers. They remain real — r173 demonstrated each by
construction and the positive control here reproduces hole 1 on demand — but their in-game
frequency is indistinguishable from zero at this sample size. Fixing them buys correctness
on paper and costs rating comparability for no measured behavioural gain. The honest reading
is: **leave them, and write the measurement down next to them so nobody re-litigates it.**

**Spend the ELO break on hole 7 instead, if one is being spent at all.** `resolve.go:2712` is
the same size of fix, it is 43–95% of every disagreement measured, and it is wrong in the
direction that matters: a creature that was granted first strike / deathtouch / lifelink /
unblockable keeps reading as if it were not, for tens of reads, into SBA, combat and split-
second checks. If exactly one line is going to move, it is that one.

Suggested ordering, by measured payoff per line of change:

1. **hole 7** — `resolve.go:2712`, one line. 43–95% of disagreements. Plus a sweep of the six
   `GrantedAbilities`-appending files that contain no invalidate at all.
2. **hole 8** — the raw `AddCounter` sites. Confirmed causal, bounded work, already on the
   Issue Log from the r63 sweep.
3. **hole 9** — CDA over dynamic counts (`Body of Knowledge` class). The largest bucket in
   cohort A and the only one that is an architecture question rather than a missing call.
   Needs a decision, not a patch.
4. **holes 1–5** — only if hole 7/8/9 are being fixed anyway and the ELO break is already
   being taken. Bundling them is free at that point; taking a break *for* them is not.

One measurement that would change this recommendation: re-running cohort A through the
`internal/hexapi` game driver instead of the tournament runner. That path never invalidates
on its own, and hole 2 is the one most likely to light up there.

---

## 6. Instrumentation — exactly what was added, and that it is gone

**Production code touched (temporary, now reverted):**

- `internal/gameengine/zz_charcache_probe_r174.go` — new file. Two nil function pointers
  (`CharCacheHitProbe`, `CharCacheStoreProbe`), `ForceFreshCharacteristics`,
  `CachedCharacteristics`, `CharCacheStats`, `CharCacheCECount`. **Deleted.**
- `internal/gameengine/layers.go` — two call sites inside `GetEffectiveCharacteristics`, each
  a nil compare plus a call: one on the cache-hit return, one after the cache store. Inert
  whenever the pointers are nil, which is always unless a test sets them. **Reverted;
  `git diff` on the file is empty.**

**Test-only (now deleted):** `internal/tournament/zz_r174_charcache_test.go` — the harness:
witness capture, counterfactual reverter, co-staleness sweep, caller classification, the
three cohort runners, the behavioural control and the positive control.

**Data:** `data/rules/ast_dataset.jsonl` was symlinked into the worktree from the primary
checkout (gitignored, not copied, link removed).

Nothing was committed, nothing was pushed, and the production server was not touched.

Raw run logs (scratchpad, not in the repo):
`r174_FINAL_greedy.log`, `r174_FINAL_hostile.log`, `r174_FINAL_ygg.log`,
`r174_control_greedy.log`, `r174_control_ygg.log`.

---

## Independent verification (Hex, 2026-10-18)

The headline claim — that the dominant cause is a NEW hole at
`resolveGrantAbility`, not any of the five r173 holes — was re-checked
directly rather than taken from the investigation:

```
$ awk 'NR>=2660 && NR<=2760' internal/gameengine/resolve.go \
    | grep -nE '^func |GrantedAbilities = append|InvalidateCharacteristicsCache|^}'
26:  gs.InvalidateCharacteristicsCache()          <- line 2685, BEFORE the function
30:  func resolveGrantAbility(...)                <- line 2689
53:  t.Permanent.GrantedAbilities = append(...)   <- line 2712
64:  }                                            <- line 2723, function ends

$ grep -n 'InvalidateCharacteristicsCache' internal/gameengine/resolve.go
2685   2847      <- both OUTSIDE resolveGrantAbility
```

`resolveGrantAbility` never invalidates, and `foldGrantedAbilities`
(`keyword_source_unification_r66.go:76`) folds `p.GrantedAbilities` into
the layer base — so appending to it changes the computed answer while
leaving the epoch untouched. Confirmed.

A crude per-file audit of the wider family (appends vs invalidates per
file, non-test) shows several more files with raw appends and **no**
invalidate anywhere in them at all:

```
internal/gameengine/clone.go                       appends=1 invalidates=0
internal/gameengine/per_card/vecna_trilogy.go      appends=1 invalidates=0
internal/gameengine/per_card/obeka_support.go      appends=2 invalidates=0
internal/gameengine/per_card/urza_chief_artificer.go appends=2 invalidates=0
internal/gameengine/per_card/sac_outlets.go        appends=1 invalidates=0
```

That count is a POINTER, NOT A FINDING: a file containing an invalidate
does not mean this particular append is followed by one, and a file
without one may be writing to a permanent that is about to be
invalidated anyway (clone.go almost certainly is). Each needs the same
counterfactual treatment the measurement applied to the five r173 holes
before anyone calls it a bug.

**What this changes about the r173 recommendation:** the three one-line
fixes filed there are measured at a real-game frequency of **zero**.
They remain correct-in-principle, but they are not where the damage is.
`resolve.go:2712` is the same size of fix and accounts for most of what
was actually observed. Still filed rather than fixed — it moves game
outcomes and breaks ELO comparability for affected boards, which is a
human's call.
