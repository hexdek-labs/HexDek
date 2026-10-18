# r175 — `StateBasedActions` breakdown

**Question asked:** `StateBasedActions` is 30.28% cumulative in the production CPU profile and
has never been decomposed. What is actually in there, and what of it is worth attacking?

**Short answer up front:** `StateBasedActions` is a **pure wrapper** — its flat time is
0.042%–0.067%. The 30% belongs entirely to its ~30 callees. But the premise is *not* merely
"the items already on the list, re-attributed": the single largest component inside SBA is
`cardIsToken` at ~3.8% of total engine CPU, driven **entirely by scanning the four players'
libraries**, and that has not been on anyone's list. Two further specific, unaddressed items
are below it. Roughly half of SBA's 30% is irreducible rules work.

---

## 1. Provenance — band readings and capture agreement

Three 30-second CPU captures off prod (192.168.1.206, `127.0.0.1:6060`), analysed locally
against the exact deployed binary `hexdek-server-r174`
(Build ID `e8cdcf5065068b6dfaba83fbe1a6819272e202b3`).

| Capture | `g/min now` BEFORE | `g/min now` AFTER | Wall | Samples | Verdict |
|---|---|---|---|---|---|
| sba1 | **859.0** (09:11:03) | **874.2** (09:11:35) | 30.01s | 119.25s (397%) | clean |
| sba2 | **861.8** (09:13:44) | **870.0** (09:14:16) | 30.0s | ~119.19s | clean |
| sba3 | **908.2** (09:14:43) | **879.8** (09:15:16) | 30.0s | ~119.13s | clean |

All six readings are above the ~760 floor and consistent with the reference band
(859.9 → 816.2). One earlier attempt was **discarded unmeasured**: at 09:09 the server was
reading 622–670 g/min (a `SLOW GAME 599.9s` was in flight), so the capture script was made to
block until the band recovered rather than sample a stall.

**The three captures agree.** `StateBasedActions` cumulative: **30.37% / 30.69% / 30.32%**.
Flat: 0.067% / 0.059% / 0.042%. Every ranking position below is stable across all three; no
finding in this document appears in only one capture.

---

## 2. Flat vs cumulative — the wrapper result

```
flat    flat%    cum      cum%
0.08s   0.067%   36.22s   30.37%   gameengine.StateBasedActions      (capture 1)
0.07s   0.059%   36.58s   30.69%   gameengine.StateBasedActions      (capture 2)
0.05s   0.042%   36.12s   30.32%   gameengine.StateBasedActions      (capture 3)
```

**Measurement.** The loop body itself costs essentially nothing. "SBA is 30%" is, on its own,
not an actionable statement — it is the sum of 30 independent rules checks.

Callee decomposition (capture 1 / 2 / 3, as % *of the SBA subtree*):

| Callee | cap1 | cap2 | cap3 | % of **total** CPU (cap1) |
|---|---|---|---|---|
| `sba704_5d` (tokens in non-battlefield zones) | 19.85% | 21.49% | 19.55% | **6.03%** |
| `sba704_5f` (toughness ≤ 0) | 18.42% | 16.35% | 17.03% | **5.59%** |
| `sba704_5r` (counter limits) | 6.65% | 6.23% | 5.98% | 2.02% |
| `sba704_5n` (equipment attachment) | 6.07% | 5.99% | 5.81% | 1.84% |
| `sba704_5j` (legend rule) | 5.36% | 5.71% | 5.23% | 1.63% |
| `sba704_5g` (lethal damage) | 5.02% | 5.06% | 4.71% | 1.53% |
| `sba704_5e` (copies outside legal zones) | 4.28% | 4.84% | 4.71% | 1.30% |
| `sba704_5m` (aura attachment) | 4.03% | 3.91% | 4.43% | 1.22% |
| `sba704_5i` (planeswalker/other) | 3.81% | 3.83% | 3.88% | 1.16% |
| `sba704_5q` (counter pair removal) | 3.45% | 3.72% | 3.57% | 1.05% |
| `sba704_5aa` (Speed — **declared STUB**) | 2.95% | 2.62% | — | 0.90% |
| `sba704_5h` (deathtouch) | 2.40% | 1.97% | — | 0.73% |
| `RefreshDevotionCreatureToggles` | 2.21% | 2.24% | — | 0.67% |
| remaining 17 helpers | ~15% | ~16% | ~16% | ~4.6% |

### Leaf costs inside the SBA subtree (`-focus=StateBasedActions`)

| Leaf | cap1 flat | cap2 | cap3 | nature |
|---|---|---|---|---|
| `cardIsToken` (inline) | **3.97%** | 3.93% | 3.53% | call volume |
| `(*Permanent).hasType` | 1.87% | 1.80% | 2.27% | call volume |
| `strings.ToLower` | 1.64% | 1.59% | 1.93% | **pure waste** (see §4.3) |
| `sba704_5d` own frame | 1.53% | 1.89% | 1.68% | call volume |
| `(*GameState).HasTypeOf` | 1.10% | 1.03% | 1.02% | call volume |
| `(*GameState).IsCreatureOf` | 0.91% | — | 0.91% | per-call expense |
| `containsSubstrFold` | 0.91% | 0.86% | 0.79% | `parseCounterLimit`, already reduced |
| `runtime.tryDeferToSpanScan` | 0.85% | 1.03% | 0.89% | GC write barriers |
| `removeCopiesFromZone` | 0.71% | 0.85% | 0.83% | **unconditional rebuild** (§4.4) |
| `GetEffectiveCharacteristics` | 0.69% | 0.68% | 0.65% | cache-hit path |

---

## 3. Call counts — passes and breadth

The "N passes × M permanents" hypothesis was tested directly by instrumenting
`StateBasedActions` locally (temporary, uncommitted, **reverted**; tree verified clean) and
running the real tournament driver against three different deck pools.

| Deck pool | games | SBA calls | **mean passes** | 1 pass | 2 passes | battlefield perms/call | non-BF cards/call |
|---|---|---|---|---|---|---|---|
| `7174n1c` | 40 | 40,561 | **1.013** | 98.71% | 1.29% | 27.92 | 370.33 |
| `moxfield_300` | 40 | 18,445 | **1.007** | 99.27% | 0.73% | 21.82 | 384.98 |
| `wizards` | 40 | 55,523 | **1.012** | 98.80% | 1.20% | 39.13 | 359.55 |

Non-battlefield breakdown (cards walked per SBA call, all four seats summed):

| Pool | hand | graveyard | exile | **library** | library share |
|---|---|---|---|---|---|
| `7174n1c` | 16.39 | 40.14 | 2.27 | **311.53** | **84.1%** |
| `moxfield_300` | 17.90 | 2.51 | 0.27 | **364.33** | **94.6%** |
| `wizards` | 18.31 | 18.23 | 2.64 | **320.38** | **89.1%** |

Prod corroboration: the `sba_max_passes` guard tripped **104 times in 48,827 games (0.21%)**.

### Three measured conclusions

1. **There is no pass-count problem.** 98.7–99.3% of SBA invocations run exactly one pass.
   The loop is already optimal. *Any* proposal premised on reducing SBA pass count is
   attacking something that does not exist.
2. **The breadth asymmetry is the whole story.** Per pass the engine walks ~28 battlefield
   permanents but ~370 non-battlefield cards — a **13:1 ratio**, of which **84–95% is the
   libraries**. This is exactly why `sba704_5d`, whose per-card work is a three-line slice
   scan, outranks `sba704_5f`, whose per-permanent work is a full layer-system query.
3. `sba704_5d` and `sba704_5e` each walk that same ~370-card set independently, so the four
   libraries are traversed **twice per SBA call**.

---

## 4. Ranked shortlist

### 4.1 — `sba704_5d`: scanning 1,200+ library cards per SBA call · ~6.0% of total CPU

**Measured.** `sba704_5d` is 6.03% / 6.59% / 5.93% of total engine CPU. `cardIsToken` accounts
for 4.74s of its 7.19s (capture 1) and the `-peek` shows **99.79% of all `cardIsToken` time in
the whole process originates in `sba704_5d`**. `cardIsToken` is three lines — a scan of
`c.Types` for `"token"`. So this is **100% call volume, not per-call expense**: ~370 calls per
pass, ~311 of them against library cards.

**Reducible?** The two obvious routes are both explicitly closed: memoising `cardIsToken` is
off-limits (token-ness is stamped after construction at ~93 sites), and gating the zone scan on
the canonical zone-move API is off-limits without a zone-write audit (handlers bypass it).

One route is *not* closed and is the single most promising item here: **replace the `Types`
slice scan with a bool field on `Card`, written at the same ~93 sites that currently append the
`"token"` type.** This is not a cache and not a gate — it is the same information in a cheaper
representation, so neither objection applies, and it cannot go stale relative to the Types
slice if the write is colocated. Expected effect is a large fraction of the 3.8%, because the
remaining cost is then a predictable bool load instead of a pointer-chasing slice scan.
*Inference, not measured.* Prerequisite: enumerate the ~93 stamp sites and prove the set is
closed. Whether a *safe* way to skip the library walk entirely exists is **undetermined** —
it requires the zone-write audit that is out of scope for this round.

### 4.2 — `sba704_5f` / `IsCreatureOf`: characteristics-cache misses · ~5.3% of total CPU

**Measured.** `sba704_5f` is 5.59% of total; **92.8% of it is `IsCreatureOf`**, which is in turn
84.7% `GetEffectiveCharacteristics`. Critically, inside the SBA subtree
`GetEffectiveCharacteristics` spends **61.7% of its time in `BaseCharacteristics` (4.07s = 3.41%
of total CPU) plus 18.8% in `applyLayer`** — that is the **cache-miss** path. Only 0.82s is the
hit path.

**Nature: per-call expense, not volume** — only ~28 permanents are walked per pass. The misses
are the cost. `BaseCharacteristics` itself is allocation-heavy (`growslice` 0.57s + `newobject`
0.42s + `typedslicecopy` 0.20s = 1.19s) plus `partitionTypes` 0.97s and `cardColors` 0.77s.

**Reducible?** The cache is keyed on a **single global epoch** — `gs.charCacheEpoch++` at
`layers.go:214`, reached from **365 call sites**, firing **441 times per game**. Any
characteristics mutation anywhere invalidates *every* permanent's entry. Finer invalidation
granularity (per-permanent, or per-seat) would convert most of these misses into hits.
*Inference, not measured* — I did not measure what fraction of the 441 invalidations/game are
genuinely global. That measurement is the right next step before anyone writes code.
Note the global hit rate is already 98.3%, so this is the classic "rare miss, expensive miss"
shape: 1.7% of calls are consuming ~80% of the function's time.

### 4.3 — `HasTypeOf` / `HasSupertypeOf` lowercasing compile-time constants · ~1.8% of total CPU, pure waste

**Measured.** `HasTypeOf` begins with `strings.ToLower(strings.TrimSpace(want))`. Every SBA
caller passes a **string literal that is already lowercase**: `sba704_5n` passes `"equipment"`,
`"fortification"`, `"land"`; `sba704_5m` and `sba704_5i` similarly; `sba704_5j` passes
`"legendary"` to `HasSupertypeOf`. Inside the SBA subtree:

- `HasTypeOf` → `strings.ToLower` **1.37s** + `strings.TrimSpace` **0.43s**
- `HasSupertypeOf` → `strings.ToLower` **0.29s**
- total **≈2.09s = 1.75% of total engine CPU**, i.e. **~50% of all `HasTypeOf` time**

Callers inside SBA: `5n` 1.77s, `5i` 0.92s, `5m` 0.85s, plus `5j` 0.90s via `HasSupertypeOf`.

**Reducible: yes, and this is the lowest-risk item on the list.** The normalisation exists to
defend against mixed-case callers; the fix is a fast path that skips it when the argument is
already normalised, or normalised constants at the call sites. No rules semantics are touched.
*Caveat:* `strings.ToLower` also appears at 6.24% globally, so part of this is the known item —
but the SBA share specifically is provably redundant work on literals, which the global figure
does not tell you.

### 4.4 — `sba704_5e` / `removeCopiesFromZone`: unconditional slice rebuild · ~1.4% of total CPU, 43% of it GC write barriers

**Measured.** 1.30% / 1.48% / 1.43% of total CPU, and the `-peek` shows **`gcWriteBarrier` is
42.95% of `removeCopiesFromZone`** (0.64s of 1.49s).

**Cause (read from source, `sba.go`).** `removeCopiesFromZone` does
`kept := (*zone)[:0]` then `kept = append(kept, c)` for every surviving card and
`*zone = kept` **unconditionally** — there is no `removed > 0` guard. With ~370 cards across
four seats, that is **~1,480 write-barriered pointer stores per SBA call that are all no-ops**,
because a copy in hand/graveyard/exile/library is vanishingly rare.

**This is distinct from the `sba704_5d` item already investigated.** `sba704_5d` gates its
`*zone = kept` on `removed > 0`; `sba704_5e` does not gate at all. The same pattern, one
instance fixed and one never touched.

**Reducible: yes, directly.** Scan first, rebuild only when a copy is actually found. No rules
behaviour changes — the output is identical, only the no-op writes disappear. Lowest-effort
item after 4.3.

### 4.5 — Linear `Types` scans for near-absent mechanics · ~2.8% of total CPU combined

**Measured.** `(*Permanent).hasType` is 1.87% / 1.80% / 2.27% of total CPU inside SBA, and the
`-peek` attributes it: **`IsBattle` 57.1%, `IsSaga` 21.0%, `IsWorld` 21.0%** (from `sba704_5v`,
`sba704_5s`, `sba704_5k`). Separately `sba704_5aa` is ~0.90% of total, of which 0.41s is
`runtime.mapaccess1_faststr` — it scans every permanent's `Card.Types` *and* does a
`p.Flags["start_your_engines"]` map lookup per permanent per pass, for a mechanic the source
comment itself labels **"STUB — Speed mechanic not modeled."**

**Reducible: yes, moderate effort.** A per-permanent type bitmask computed once (alongside the
existing characteristics work) would collapse `IsBattle`/`IsSaga`/`IsWorld`/`IsCreature` to a
bit test. For `sba704_5aa` specifically, hoisting the map probe behind the cheap Types check —
or a per-seat "any start-your-engines permanent" flag — removes most of the 0.90%.
Battles, Sagas and World permanents are genuinely rare in the pool, so nearly all of this work
returns false.

---

## 5. What I judge irreducible — these are rules checks that must run

Stated explicitly, because "it's expensive" is not the same as "it's waste":

- **The ~28-permanent battlefield walk in `sba704_5f`/`5g`/`5h`/`5i`.** CR §704.3 requires these
  to be evaluated whenever a player would receive priority. The *walk* is mandatory; only the
  per-permanent cost (§4.2) is attackable. The loop itself is not waste.
- **The one-pass loop structure.** Measured at 1.007–1.013 passes. Already optimal; nothing to
  win here.
- **`destroyPermSBA`, `counters.PairRemoveSBA` (0.72s), `auraTargetMatchesEnchantClause`
  (0.29s).** These only execute when an SBA actually fires. Real work, proportional to real
  events.
- **`parseCounterLimit`** (1.41s / 1.18% of total, via `sba704_5r`). Already addressed in an
  earlier round; now consistent with the stated 1.07% and inside the noise band relative to
  everything above it. Not worth a second pass.
- **`RefreshDevotionCreatureToggles`** (~0.67%). Must run before the death SBAs by CR §711.2;
  the ordering comment in `sba.go` is correct.

## 6. Verdict on the premise

**Partially false, in a way worth stating plainly.** `StateBasedActions` *is* a thin wrapper —
0.042%–0.067% flat — so the headline "SBA is 30%" was not by itself actionable, and a chunk of
its 30% is the already-known items (`strings.ToLower`, `GetEffectiveCharacteristics`,
`IsCreatureOf`, `cardIsToken`) now merely localised to their SBA call sites.

But the decomposition was **not** wasted, because it produced three things the flat list could
not:

1. `cardIsToken`'s 3.8% is **one call site** (`sba704_5d`, 99.79%) and its driver is
   **library scanning** — ~311 of 370 cards per pass. The global 3.71% figure told you the cost;
   it did not tell you it was libraries, and the representation fix in §4.1 is only visible
   from here.
2. `sba704_5e`'s unconditional rebuild (§4.4) is a **previously unexamined** instance of a
   pattern already fixed in its sibling, and 43% of its cost is GC write barriers on no-op stores.
3. Half of `HasTypeOf`'s time is **lowercasing string literals** (§4.3) — invisible in a global
   `strings.ToLower` number.

Recommended order of attack: **§4.3 → §4.4 → §4.1 → §4.2 → §4.5**, cheapest and safest first.
§4.2 should be preceded by a measurement of invalidation locality, not jumped into.

---

*Method notes.* Profiles taken with `curl .../debug/pprof/profile?seconds=30` on .206, analysed
locally with `go tool pprof -top -cum` / `-peek` / `-focus` against the deployed binary. Pass
and breadth counts came from temporary local instrumentation of `StateBasedActions`, run
against `cmd/hexdek-tournament` on three deck pools; the instrumentation was reverted and the
working tree verified clean. No production code was changed, nothing was deployed or restarted,
and the scratch files created on .206 were removed.

---

## Note on provenance and on two corrections to the brief (Hex, 2026-10-18)

This investigation was commissioned with a brief that contained a wrong
hypothesis, and the useful part of the result is that it said so:

- I wrote that `StateBasedActions` at 30.28% was "the real #1 and has
  never been attacked as a whole." It is a **wrapper**, flat
  0.042–0.067%. The headline number was never actionable.
- I instructed it to find out "how many SBA passes a typical game runs",
  on the assumption that the until-no-change loop was iterating.
  **98.7–99.3% of calls run exactly one pass.** The premise was false and
  any work aimed at reducing pass count would have attacked nothing.

Both are recorded in the round's dead-end list so they are not re-walked.

**One collision the next round must not miss:** shortlist item 3
(`HasTypeOf`/`HasSupertypeOf` lowercasing constants) overlaps work
already done and binned in r174, where a fast path *inside* `HasTypeOf`
measured **0.94x — slower** than the `strings.ToLower` it replaced. This
document's angle is different (the SBA call sites pass constants, so the
fix would be at the call sites), which is not automatically dead — but
it must be measured against r174's negative result rather than started
fresh.

**Nothing here is shipped.** It is analysis. The ranked order suggested
is 3 → 4 → 1 → 2 → 5; on the evidence above item **4** (`sba704_5e`
rebuilding every zone slice with no `removed > 0` guard, while its twin
`sba704_5d` has one) is the cleanest, since it is a direct omission with
no semantic change and an identified sibling that was already fixed.
