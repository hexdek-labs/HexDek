# r173 — DependencyOrder memo: invalidation matrix

**Question asked:** is `gs.charCacheEpoch` bumped on every mutation that could change
the answer `applyLayer` → `DependencyOrder` produces, so that the ordered candidate
list can be memoised per `(layer, sublayer, charCacheEpoch)`?

**Answer: no. Five holes demonstrated by driving the real code, two undetermined.**
**Recommendation: NO-GO on (A) as specified.**

Worktree `/private/tmp/claude-501/hexdek-a1`, branch `dev/r173-a1`, at `e15a7ab7`.
Nothing committed. All line numbers are against that commit.

---

## 1. What the memoised computation actually reads

The proposal memoises the output of `applyLayer`'s filter → sort → `DependencyOrder`
pipeline. That whole pipeline is the memo's input surface, not just `DependencyOrder`.

### 1a. `applyLayer` (layers.go:814) — the filter and the sort

| Read | Site | Per-permanent? |
|---|---|---|
| `gs.ContinuousEffects` (slice identity + membership) | layers.go:836 | no |
| `ce.SourcePerm.PhasedOut` | layers.go:843 | no |
| `ce.Layer` | layers.go:846 | no |
| `ce.Sublayer` | layers.go:849 | no |
| `gs.Active` | layers.go:864 | no |
| `ce.ControllerSeat` | layers.go:867 | no |
| `ce.Timestamp` | layers.go:872 | no |

The premise that the result is permanent-independent is **correct** — `perm` is not
read anywhere in the filter or the sort, and `DependencyOrder` does not take it.
The premise that is wrong is that `charCacheEpoch` covers the rest.

Note on `gs.Active` (layers.go:864-873): despite the comment calling it an "APNAP
tiebreak", it is the **primary** sort key. All of the active seat's effects sort
before all others regardless of timestamp. A change of active seat reorders the
whole candidate list, not just ties.

### 1b. `DependencyOrder` (layers.go:571)

| Read | Site |
|---|---|
| `e.Layer` (r171 fast-path scan: `== LayerAbility`, `== LayerType`, `> LayerType`) | layers.go:608, 611, 614 |
| `gs.acquireDepBuf` / `gs.releaseDepBuf` free list | layers.go:655, 707, 710 |
| everything `effectDependsOn` reads | layers.go:656 |

### 1c. `effectDependsOn` (layers.go:733)

| Read | Site |
|---|---|
| `a.SourcePerm`, `b.SourcePerm` (pointer identity only) | layers.go:738, 770, 794 |
| `a.Layer`, `b.Layer` | layers.go:765, 788 |
| **`b.Predicate(gs, a.SourcePerm)`** | layers.go:775, 799 |

`b.Predicate` is an arbitrary closure. Across 79 `ContinuousEffect{` literals and
36 `Predicate: func` closures in non-test code, the observed reads are
`target.Controller`, `target.Card`, `target.Flags`, `target.IsCreature()`,
`src.Controller` (closed over), `gs.Turn`, `gs.Seats`, `gs.Phase`.
`Permanent.IsCreature()` and the `HasKeywordOf` family re-enter
`GetEffectiveCharacteristics` — the r159 comment at layers.go:640-652 says so
explicitly ("DependencyOrder re-enters itself through effectDependsOn ->
Predicate -> GetEffectiveCharacteristics"). So **the layer-6 `DependencyOrder`
answer transitively depends on the entire layered characteristics of every
source permanent** — i.e. on every input the char cache has, plus the ones below.

---

## 2. How the epoch is bumped

`gs.charCacheEpoch++` occurs at **exactly one site**: `layers.go:214`, inside
`(*GameState).InvalidateCharacteristicsCache()`. There is no other writer.
573 call sites across the repo. Therefore any mutation path that does not reach
that function does not bump, full stop — no comment needs to be trusted.

Confirmed bumping APIs relevant to this surface:

| API | Bump site |
|---|---|
| `RegisterContinuousEffect` | layers.go:254 |
| `UnregisterContinuousEffectsForPermanent` | layers.go:281 (guarded on `removed > 0`) |
| `ApplyControlChange` (the canonical `Permanent.Controller` writer) | control_change_302_6_r102.go:106 |
| `ScanExpiredDurations` CE sweep | phases.go:361 (guarded on `expired > 0`) |
| end-of-combat CE sweep | combat.go:~2835 (guarded on `modsRemoved`) |
| seat-elimination CE sweep | multiplayer.go:1113 (guarded on length change) |
| `SetKeywordFlags` / r97 mutators | keyword_flag_mutation_r97.go |

---

## 3. The matrix

| # | Field / state | Read at | Write sites | Bumps? | Severity |
|---|---|---|---|---|---|
| 1 | `Permanent.PhasedOut` | layers.go:843 (candidate filter) | `PhaseOut` phases.go:710,735; `PhaseIn` phases.go:760,780; `per_card/game_changers.go:189`; `per_card/kaito_shizuki.go:72` | **NO — none of them call `InvalidateCharacteristicsCache`** | **CONFIRMED HOLE — high.** Demonstrated stale read. |
| 2 | `gs.Active` | layers.go:864 (primary sort key) | `AdvanceActiveSeat` extra_turn_sequencing.go:146,170,184; `multiplayer.go:1191`; runners (`runner.go:448`, `showmatch.go:1462/2105/2413`, `spectate_rooms.go:390/495`, `heimdall/contract_replay.go:227/277`, `cmd/hexdek-judge/run.go:308/328`) | **NO.** `AdvanceActiveSeat` does not bump. `internal/tournament/turn.go` bumps at untap/upkeep/cleanup *near* the rotation (turn.go:187,216,267,596) — incidental, not causal. **`internal/hexapi` calls `InvalidateCharacteristicsCache` ZERO times** and is the prod server path. | **CONFIRMED HOLE — high.** Demonstrated stale read (20 cached vs 10 fresh). |
| 3 | `gs.Phase` / `gs.Step` (read by predicate closures) | layers.go:775,799 via `b.Predicate` | `tournament/turn.go` ~20 sites; `hexapi/showmatch.go`; `heimdall` | **PARTIAL.** turn.go bumps after 4 of ~20 phase writes (untap, upkeep, cleanup×2). The `main`, `draw`, `postcombat_main`, `combat`, `end_of_combat` writes do not. hexapi: never. | **CONFIRMED HOLE — med.** Demonstrated stale read. |
| 4 | `Permanent.Controller` | `b.Predicate` closures (`registerKeywordGrantStatics` mod_kind_keyword_grant.go:61-68 closes over `src.Controller`, reads `t.Controller`) | canonical: `ApplyControlChange` (**bumps**). Raw: `per_card/gen_the_reaper_king_no_more.go:152` (fallback when `ApplyControlChange` returns false) | **MOSTLY YES.** One raw fallback does not. That fallback fires when the permanent is not on a battlefield slice or the seat index is bad. | **CONFIRMED HOLE — low (narrow).** Demonstrated stale read with the exact predicate shape. |
| 5 | `gs.ContinuousEffects` membership | layers.go:836 | canonical `RegisterContinuousEffect` (bumps) / `Unregister…` (bumps). **Raw appends: `per_card/game_changers.go:106` and `:130` — Humility's layer-1 and layer-6 effects**; also `cmd/hexdek-thor/deep_rules.go:1984` (harness) | **NO for the Humility pair.** Raw `append`, bypassing the registrar — also bypassing the HandlerID idempotency guard. | **CONFIRMED HOLE — high.** Humility's `LayerAbility` effect is the one card that makes the r171 fast path fall through to the real `effectDependsOn` scan. The card the memo most needs to be correct for is registered by the one path that does not invalidate. |
| 6 | `Permanent.Flags["kw:<name>"]` (read by predicates and by `HasKeywordOf` re-entry) | via `b.Predicate` | `SetKeywordFlags`/`ClearKeywordFlags` (bump, r97); `GrantKeywordFlag`/`RevokeKeywordFlag` (keyword_flag_grants_r87.go:134 — **deliberately do not bump**, caller's obligation, 58 sites audited at r97); **59 further raw `Flags["kw:…"]` assignments/deletes** in non-test code | **UNDETERMINED** — not individually audited here. r97's own file comment documents measured stale windows on this surface. | **UNDETERMINED — treat as unsafe.** |
| 7 | `ce.Layer`, `ce.Sublayer`, `ce.Timestamp`, `ce.ControllerSeat`, `ce.Predicate`, `ce.SourcePerm` *after registration* | layers.go:846,849,867,872; 738,765,788 | none found in production code. Only `clone.go:303` remaps `SourcePerm` while building a **fresh** `GameState`. | n/a — immutable post-registration | **SAFE.** |
| 8 | `Permanent.Counters`, `Permanent.Modifications` (reachable from predicates via `GetEffectiveCharacteristics` re-entry) | transitive | counters pipeline (`counters/`, `PutCountersTriggered`) | **UNDETERMINED** — the canonical counter path appears to invalidate, but ~180 known raw `AddCounter` sites are on record in the Issue Log (r63 sweep, "out of scope") and were not audited here. | **UNDETERMINED.** |
| 9 | `gs.acquireDepBuf` free list | layers.go:655 | internal | n/a | implementation note — see §5. |

**Count: 5 confirmed holes, 2 undetermined, 1 clean.**

---

## 4. What was actually demonstrated (throwaway probe, now deleted)

A throwaway `internal/gameengine/zz_r173_probe_test.go` was written inside the module,
run, and deleted. Results, verbatim from the run:

```
TestR173_PhaseOutDoesNotInvalidate
    epoch before=1 after PhaseOut=1
    power after PhaseOut (CR 702.26a says 2): 5        <- STALE
TestR173_ActiveSeatChangeDoesNotInvalidate
    Active=0 -> 20 | Active=1 cached -> 20 | Active=1 fresh -> 10 | epoch 2 -> 3
TestR173_DirectControllerWriteDoesNotInvalidate
    after raw Controller write: cached=5 fresh=2 epoch 1 -> 2
TestR173_TurnAndPhaseReadingPredicate
    phase-reading predicate: main=2 combat-cached=2 combat-fresh=5 epoch 1 -> 2
TestR173_RawCEAppendDoesNotInvalidate
    raw CE append: cached=2 fresh=1 epoch 0 -> 1
TestR173_MemoWouldWidenTheHole
    after un-bumped PhaseOut: cached A=5  uncached B=2
```

### The decisive one: `TestR173_MemoWouldWidenTheHole`

One anthem, source `lord`, two beneficiaries `A` and `B`. `A` is read (and cached)
first; then `lord` is phased out, which does not bump the epoch; then both are read.

- `A` = 5 (stale — it was cached before the phase-out)
- `B` = 2 (correct — it was never cached, so it recomputed and the filter at
  layers.go:843 excluded the phased-out source)

**This divergence is what a `(layer, sublayer, epoch)` memo destroys.** Today the
hole is scoped to permanents that happen to have been computed since the mutation.
A global memo freezes the first computation's candidate list and serves it to the
whole board for the rest of the epoch. The memo does not merely inherit the existing
holes — it converts every per-permanent hole into a board-wide one, and it does so
for the holes that are hardest to see (a wrong P/T, type or keyword with no error
anywhere).

---

## 5. Implementation notes for whenever this is revisited

- `DependencyOrder` returns the **caller's slice** on both fast paths (layers.go:618,
  665, 707). A memo must store a defensive copy, because `applyLayer`'s `candidates`
  is a fresh per-call allocation today and callers would start sharing it.
- The dep matrix is **borrowed from a free list** (`acquireDepBuf`/`releaseDepBuf`,
  r159) precisely because `DependencyOrder` re-enters itself through
  `Predicate → GetEffectiveCharacteristics`. A memo sits on that same re-entrant
  path and must not be populated from inside a re-entrant call, or a partially
  computed ordering gets cached.
- The r171 guard already removes the quadratic scan from 12 of the 13 `applyLayer`
  calls. What a memo would save on those twelve is a linear scan over a handful of
  effects. The real remaining cost is the `sort.SliceStable` at layers.go:865 — and
  that is the part whose inputs (`gs.Active`) are proven not to be epoch-covered.
  **The cheap part of the memo is the safe part; the expensive part is the unsafe one.**

---

## 6. Recommendation

### (A) DependencyOrder memo: **NO-GO today.**

Not because the perm-independence premise is wrong — it is right — but because
`charCacheEpoch` is not a sound generation counter for this computation. Five
mutation paths that change the answer do not reach the only line that bumps it,
and two more are undetermined. The existing per-permanent cache survives these
holes by accident (it recomputes for permanents it has not seen yet); a global
memo removes that accident.

### Minimum set of changes that would make it safe

Ordered by cost. The first two are mechanical and independently valuable — they are
live correctness bugs in the current cache, memo or no memo.

1. **Mechanical, low risk.** `PhaseOut` / `PhaseIn` (phases.go:705, 757) call
   `gs.InvalidateCharacteristicsCache()`. Same for the two raw `PhasedOut = true`
   writes in `per_card` (game_changers.go:189, kaito_shizuki.go:72), or better, route
   them through `PhaseOut`. Closes hole 1.
2. **Mechanical, low risk.** Route Humility's two effects through
   `RegisterContinuousEffect` instead of raw `append` (game_changers.go:106, 130).
   Closes hole 5 and gets the HandlerID idempotency guard for free.
3. **Mechanical, low risk.** Replace the `np.Controller = perm.Controller` fallback
   at `gen_the_reaper_king_no_more.go:152` with a bump, or drop the fallback. Closes
   hole 4.
4. **Not mechanical — a design decision.** `gs.Active`. Either (a) bump in
   `AdvanceActiveSeat` and at every direct `gs.Active =` write (10+ sites across
   four packages, including `internal/hexapi` which today never invalidates at all),
   or (b) **remove `gs.Active` from the sort**. Option (b) deserves a hard look:
   CR §613.7 makes APNAP a *tiebreak among simultaneous timestamps*, and the code at
   layers.go:864-873 uses it as a primary key, which is almost certainly a separate
   rules bug. Fixing it to a real tiebreak would both correct §613.7 and shrink the
   memo's input surface. That is a rules change and must go to 7174n1c, not be
   guessed at here.
5. **Risky / open-ended.** `gs.Phase`, `Flags["kw:"]` (59 raw sites + 58 r87 sites
   with caller-side obligation), and raw counter writes. These are reachable only
   through predicate closures, so the exposure is bounded by what predicates read —
   but "bounded by what 36 arbitrary closures read today" is not an invariant anyone
   can hold. The sound version is a **predicate purity contract**: predicates may
   read only epoch-covered state, enforced by review or by a probe harness that
   diffs memoised against non-memoised orderings over a Loki run. That is the real
   prerequisite, and it is a project, not a patch.

### (B) charCache base/layered split (~371 call sites)

Same blocker, larger blast radius, and it inherits every row above. Do not start it
on this foundation.

### Suggested sequencing

Land 1-3 as ordinary correctness fixes with regressions (they are stale-read bugs
today and worth filing on their own merits). Take item 4 to 7174n1c as a §613.7
rules question. Only once 4 is settled and a predicate-purity probe exists is (A)
worth re-costing — and by then the r171 guard may have already taken most of what
(A) was going to win.

---

## Independent verification (Hex, 2026-10-18)

The three load-bearing claims above were re-checked directly rather than
taken from the investigation, and one stale read was reproduced with a
separate throwaway probe:

```
$ grep -rn 'charCacheEpoch++' --include='*.go' internal/
internal/gameengine/layers.go:214          <- the only bump site in the codebase

$ awk '/^func PhaseOut\(|^func PhaseIn\(/,/^}/' internal/gameengine/phases.go | grep -i invalidate
(no output)

$ sed -n '100,135p' internal/gameengine/per_card/game_changers.go | grep ContinuousEffects
gs.ContinuousEffects = append(gs.ContinuousEffects, ce)     <- bypasses the registrar
gs.ContinuousEffects = append(gs.ContinuousEffects, ce2)
```

Independent probe (written, run, deleted):

```
with anthem (cached) = 5 | after PhaseOut = 5 | after explicit invalidate = 2
charCacheEpoch: 1 -> 1 across PhaseOut
STALE READ CONFIRMED: cached=5 fresh=2 (CR 702.26a)
```

A 2/2 continues to read as a 5/5 after the permanent granting it +3/+3
has phased out.

**Bound on severity, stated so it is not overread:** the epoch is
invalidated roughly 457 times per game in production (`cinval/game=457`),
so most stale windows are short and self-heal at the next unrelated
invalidation. **What is proven is that a wrong answer is reachable and
reproducible. What is NOT measured is how often it changes a game
result.** That measurement is the obvious next step and has not been
done; nobody should quote a frequency from this document.

**Status: FILED, NOT FIXED.** Items 1, 3 and 5 are mechanical one-line
bumps, but they change game outcomes and therefore break ELO
comparability for affected boards — a rules decision for a human, not a
drive-by inside a performance round. Item 2 (`gs.Active` used as a
PRIMARY sort key where the comment claims the CR 613.7 timestamp
tiebreak) is a design question and a separate suspected rules bug.
