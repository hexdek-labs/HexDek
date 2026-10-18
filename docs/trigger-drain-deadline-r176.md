# r176 — the `trigger_drain` game-deadline trip

Investigation only. Nothing was changed; production was read-only throughout.

---

## 1. Is `trigger_drain` genuinely new? — **Partly. The brief's premise needs a correction.**

**Measurement.** On `.206`, `~/hexdek/server.log` (50.8 MB, first line
`==== HexDekServer start Thu Sep 17 16:21:26 UTC 2026 ====`, last line 2026-10-18 10:11),
grepped over the **whole** file with no marker scoping:

| quantity | count |
|---|---|
| lines containing `trigger_drain` (any form) | 3101 |
| lines containing `trigger_drain_cap` | 3101 |
| lines whose **`deadlines=` field** contains `trigger_drain` | **129** |

So the 3101 is entirely the unrelated `guards=…,trigger_drain_cap:N,…` column. The signal of
interest is the 129.

**First occurrence** of `trigger_drain` in a `deadlines=` field — server.log line 338504:

```
2026/10/18 09:06:34 grinder: 46925 games … guards=game_deadline:1,percard_dispatch_depth_cap:37817,
sba_max_passes:101,trigger_drain_cap:20,trigger_loop_cap:74 deadlines=trigger_drain:1 …
```

The previous periodic line (09:06:04, games=46658) has **no `deadlines=` field at all** and
`guards=` with **no `game_deadline` entry**. The field is omitted when empty, so this is a clean
0 → 1 edge.

**Why "new" is only half true.** The whole distinct-value set of the field over the entire log is:

```
412  deadlines=token_create:1
 97  deadlines=token_create:2
 66  deadlines=token_create:2,trigger_drain:1
 31  deadlines=token_create:1,trigger_drain:1
 14  deadlines=token_create:3,trigger_drain:1
  9  deadlines=token_create:4,trigger_drain:1
  7  deadlines=trigger_drain:1
  2  deadlines=token_create:4,stack_drain:1,trigger_drain:1
```

638 lines carry the field at all, and the **earliest is 2026-10-17 23:28:21** — the field is about
eleven hours old. Worse, `internal/gameengine/loop_guard_labels_r162.go:54` records in its own
comment that `deadlines=` was **inert until r168**, because `game_deadline` passes only
`{site, turn, stack, deadline_utc, over_by_sec}` and `site` was missing from
`loopGuardLabelKeys`; it printed nothing "through two real capped games (599.7s and 599.3s on
2026-10-17) that were exactly what it was built to attribute."

**Conclusion.** `trigger_drain` is new *to the attribution table*, and this is the first and only
time it has been attributed — the counter never advances past `:1` in any window of the log.
But the table cannot see before 2026-10-17 23:28, and could not see correctly before r168. I
**cannot** claim this is the first time the deadline was noticed in the trigger drain; I can only
claim it is the first time anything could have said so. Treat "a new actor" as "a first
observation under new instrumentation", not as a proven first event.

All 129 hits come from the single trip below: the segment's periodic lines run every 30 s from
09:06:34 to 10:10:34 (≈129 lines), and the count stays at `trigger_drain:1` throughout.

---

## 2. The responsible game — **identified, high confidence**

Current process segment began `Sun Oct 18 08:07:50 UTC 2026 (GOMAXPROCS=4, GOMEMLIMIT=10GiB)`.
`deadlines=`/`guards=` are cumulative per process, so the trip is bracketed by the two periodic
lines at **09:06:04** (no `game_deadline`) and **09:06:34** (`game_deadline:1`,
`deadlines=trigger_drain:1`).

Exactly **one** `SLOW GAME` line falls in that 30-second window:

```
2026/10/18 09:06:18 grinder: SLOW GAME 600.2s turns=76 s/turn=7.9
  seed=1792313778539635264
  decks=[moxfield/ghen_arcanum_weaver_b2_bu9boi_PwqzlkkE
         moxfield/shorikai_genesis_engine_b2_pengu123_OWzRF7xl
         moxfield/charix_the_raging_isle_b2_brutishsparta_HReCHmcn
         moxfield/lilah_undefeated_slickshot_b3_cloudcycle_spQzDbql]
```

600.2 s is the `perGameWallClock` cap exactly (armed at `internal/hexapi/showmatch.go:1419` and
`:2059`). The game therefore started ≈ 08:56:18. This identification is corroborated
independently in §4 by goroutine dumps: a single GameState is pinned in the drain loop from
08:57:02 to 09:06:03 and has vanished by 09:07:04.

**Seed:** `1792313778539635264`.
**Commanders:** Ghen Arcanum Weaver (B2), Shorikai Genesis Engine (B2), Charix the Raging Isle
(B2), Lilah Undefeated Slickshot (B3).

Note `s/turn=7.9` is a whole-game average and is **misleading** here — §4 shows the game was not
uniformly slow, it was stuck in one place for the last nine minutes.

---

## 3. What the guard is, and how it relates to `trigger_drain_cap` — **two different mechanisms in the same loop body**

Both live in `drainPendingTriggers`, `internal/gameengine/trigger_batch.go`, inside the inner
stack-draining loop, on adjacent lines:

```go
resolves := 0                                   // LOCAL to this invocation
for len(gs.pendingTriggers) > 0 {
    ...
    for len(gs.Stack) > stackBefore {
        if gs.Flags["ended"] == 1 { return }
        if CheckGameDeadline(gs, "trigger_drain") { return }      // :167
        resolves++
        if resolves > maxTriggerDrainIterations {                  // 500, stack.go:58
            LogLoopGuardFired(gs, "trigger_drain_cap", …)          // :172
            gs.pendingTriggers = nil
            EndGameAsLoopDraw(gs, "trigger_drain_cap")             // :184
            return
        }
        PriorityRound(gs)
        if len(gs.Stack) <= stackBefore { break }
        ResolveStackTop(gs)
    }
}
```

**They are not the same mechanism.**

| | `trigger_drain` | `trigger_drain_cap` |
|---|---|---|
| kind | a **label**, not a guard | a real **guard** |
| what it is | the `site` string handed to `CheckGameDeadline` | a loop-iteration counter guard |
| what fires | `game_deadline` (wall clock, 600 s, `game_deadline.go`) | `resolves > 500` |
| reported in | `deadlines=` (breakdown of `game_deadline` by site) | `guards=` (one row per guard) |
| meaning | "the wall-clock budget expired, and **this** loop is the one that noticed" | "this specific loop ran 500 resolutions in one invocation" |
| game-independent? | yes — the deadline is armed per game, any instrumented site can report it | no — bound to this loop |

So `deadlines=trigger_drain:1` does **not** mean the drain loop ran away. It means the 600-second
budget expired while control happened to be inside this loop. The four sites are
`token_create`, `stack_drain`, `trigger_drain`, `turn_phase_boundary`
(`deadline_label_r168_test.go:62`), and `guards=game_deadline:N` always equals the sum of the
`deadlines=` entries — at 10:10:34, `game_deadline:6` = `token_create:4 + stack_drain:1 +
trigger_drain:1`. Consistent.

**Measured: `trigger_drain_cap` did *not* fire for this game.** It read `20` at 09:05:04,
09:06:04, 09:06:34 and was still `20` at 09:12:04. The 500-resolve counter was never approached.

**Why it was never approached — a real finding.** `resolves` is a *local* variable, reset to 0 on
every entry to `drainPendingTriggers`. The stall (§4) is **recursive**, not iterative: 26 nested
`drainPendingTriggers` frames, each re-entered through `ResolveStackTop`'s defer. Each frame
carries its own fresh counter, so a cascade that recurses instead of looping can never accumulate
to 500 in any single frame. The counter guard is structurally blind to this shape, and the
wall-clock deadline is the only thing that could have stopped it. That is exactly the case
`game_deadline.go`'s header was written for.

---

## 4. Token-swarm family, or a new shape? — **NOT the token-swarm family. It is the `DependencyOrder` shape (episode seven), reached by a different spine.**

Evidence: `~/stallwatch/` holds 60-second CPU profiles *and* goroutine dumps at one-minute
intervals covering 08:57:02 → 09:10:04, i.e. the whole second half of the game. Profiles were
scp'd off and run locally against `hexdek-server-r174`.

### 4a. Goroutine dumps (the per-game evidence — profiles are whole-process and mix concurrent games)

GameState `0x18b11e5c908` — **goroutine 29** — is present in the drain recursion in **every**
dump from `goro-20261018T085702Z` through `goro-20261018T090603Z` (10 consecutive dumps, 9
minutes) and is **absent** from `goro-20261018T090704Z`. That brackets the 08:56:18 → 09:06:18
game precisely.

Its frame census is **byte-identical** in all four dumps I tabulated (08:57, 09:01, 09:05, 09:06):

```
53  gameengine.ResolveStackTop
26  gameengine.drainPendingTriggers
 3  gameengine.effectDependsOn
 2  gameengine.ResolveEffect
 1  each: hexapi.(*Showmatch).runGrinder → runOneGameFast → tournament.TakeTurn →
         takeTurnImpl → runMainPhase → gameengine.CastSpell → DrainStack →
         [ResolveStackTop ⇄ drainPendingTriggers] × 26 → ResolveEffect →
         resolveCopyPermanent → FirePermanentETBTriggers → OnConstellationETB →
         FireConstellationTriggers → keywordActive → (*GameState).HasKeywordOf →
         GetEffectiveCharacteristics → applyLayer(layer 6) → DependencyOrder →
         effectDependsOn → effectDependsOn.func1 → .func1.1   [layers.go:774/776]
```

**`resolveCreateToken` and `fireObserverETBTriggers` do not appear anywhere in goroutine 29.**
They *do* appear in the whole-process CPU profiles (≈19 s and ≈10 s cum), but those belong to a
different, concurrent grinder goroutine — a separate game. Attributing them to this one would be
the mistake the goroutine dumps exist to prevent.

The recursion depth is **pinned** at 26/53 for nine minutes (`maxResolveDepth` is 50,
`stack.go:70` — it is sitting at the ceiling), but the work is not frozen: the effect-slice
length passed to `DependencyOrder` **grows monotonically** across the dumps —

| dump | `DependencyOrder({ptr, len, cap})` | n |
|---|---|---|
| 08:57:02 | `{0x18aed378d08, 0x106, 0x11f}` | 262 |
| 09:01:03 | `{0x18b077d1908, 0x197, 0x25f}` | 407 |
| 09:05:03 | `{0x18b1b162c08, 0x1d6, 0x25f}` | 470 |
| 09:06:03 | `{0x18b034c1908, 0x1e2, 0x25f}` | 482 |

So permanents keep entering: the ETB cascade re-fills `pendingTriggers` at least as fast as it
drains, *and* each step gets quadratically more expensive as the continuous-effect population
grows.

### 4b. CPU profiles (the measurement, three independent captures, in agreement)

`go tool pprof -top -cum` against the r174 binary, 60 s captures, ~79.7 s of samples each
(GOMAXPROCS=4):

| symbol | 08:57 | 09:01 | 09:05 |
|---|---|---|---|
| `FirePermanentETBTriggers` | 20.93 s (26.3 %) | 38.83 s (48.8 %) | 39.47 s (49.5 %) |
| `drainPendingTriggers` | 20.02 s (25.1 %) | 19.80 s (24.9 %) | 19.63 s (24.6 %) |
| `resolveCopyPermanent` | 18.94 s (23.8 %) | 18.90 s (23.7 %) | 19.01 s (23.8 %) |
| `OnConstellationETB` / `FireConstellationTriggers` | 18.92 s | 18.90 s | 19.01 s |
| `DependencyOrder` | 18.62 s (23.4 %) | 18.74 s (23.5 %) | 18.94 s (23.7 %) |
| `effectDependsOn` | 15.37 s | 16.05 s | 16.16 s |
| `GetEffectiveCharacteristics` | 23.58 s | 21.95 s | 21.89 s |

The `resolveCopyPermanent → OnConstellationETB → DependencyOrder` chain holds a near-constant
≈19 s of every 60 s window — one saturated core, continuously, for the whole capture series.
`effectDependsOn` carries 3–5 s *flat* per window, so the cost really is in the dependency
comparison itself, not below it. The three captures agree to within a few percent.

### 4c. Mechanism

`DependencyOrder` (`internal/gameengine/layers.go:651-663`) is a literal O(n²) double loop whose
own comment concedes "n*n is paid per call", and `effectDependsOn` re-enters
`GetEffectiveCharacteristics` through `b.Predicate` (the `recover()`-wrapped call at
`layers.go:774`, which is the stall's leaf frame). With n going 262 → 482, the per-call
comparison count goes ≈69 k → ≈232 k, each potentially re-entrant. Every constellation ETB
performs a `HasKeywordOf`, which is layer-aware and triggers the full recompute.

This is consistent with the table: **Ghen, Arcanum Weaver** is an enchantment-recursion commander,
constellation is an enchantment-ETB trigger, and `resolveCopyPermanent` is enchantment copying.
Accumulating enchantments is exactly accumulating layer-6 continuous effects.

### 4d. Classification

- **Shared with the six token-swarm episodes:** the `FirePermanentETBTriggers` spine and the
  self-refilling ETB cascade.
- **Different from them:** the spine's head is `resolveCopyPermanent`, not `resolveCreateToken`,
  and `fireObserverETBTriggers` is absent from this goroutine entirely.
- **Shared with episode seven:** `DependencyOrder` is the hot leaf, and it is where effectively
  all the CPU goes.

So: **a `DependencyOrder` blowup (episode-seven family), fed by a copy-permanent/constellation ETB
cascade rather than a token swarm.** Best read as a hybrid — the cascade supplies the ETBs, the
quadratic layer sort makes each one unaffordable. Calling it a token swarm would be wrong.

---

## 5. What I ruled out

- **`trigger_drain_cap` as the cause.** Measured flat at 20 across the entire trip window. Not the
  same mechanism as the `trigger_drain` label; see §3.
- **A grep-count reading of the premise.** `grep -c trigger_drain` returns 3101 and is entirely
  `trigger_drain_cap`. The cumulative fields were deltaed between windows, never counted.
- **"The game was uniformly slow."** `s/turn=7.9` suggests it, but ten consecutive goroutine dumps
  show one goroutine pinned in an unchanging 26-deep recursion for the final nine of the ten
  minutes. It is a point stall.
- **Token swarm.** Ruled out *for this goroutine* on direct frame evidence (§4a), despite
  `resolveCreateToken` being prominent in the whole-process profile — that is a concurrent game.
- **An earlier `trigger_drain` trip hiding in the log.** The full-file grep finds none. But I do
  **not** claim it never happened: the `deadlines=` field has only existed since 2026-10-17
  23:28:21 and was inert before r168 (§1). This is an **undetermined**, not a negative.
- **Whether the cascade would ever have terminated.** Undetermined. n was still rising at the
  moment the deadline cut in, so I have no evidence either way about a fixed point.
- **Which card in the Ghen deck is the copy source.** Not determined — the goroutine dump gives
  pointers, not card names, and the deadline draw leaves no per-card record. Re-running
  `seed=1792313778539635264` with the four listed decks is the way to get it.

## 6. Open question worth filing

The counter guard `trigger_drain_cap` cannot see a recursive cascade, because `resolves` is
per-invocation and this shape recurses 26 frames deep rather than iterating. Any runaway that
re-enters `drainPendingTriggers` through `ResolveStackTop`'s defer is invisible to it and relies
entirely on the wall-clock backstop. Not fixed here — filed for 7174n1c / wiedeman.

---

## Independent verification (Hex, 2026-10-18)

The load-bearing structural claim — that `trigger_drain_cap` is blind to
a recursive stall — was re-checked directly:

```
$ awk '/^func drainPendingTriggers/,/^}/' internal/gameengine/trigger_batch.go \
    | grep -nE 'resolves|trigger_drain|CheckGameDeadline'
35:  resolves := 0                                      <- LOCAL to the function
50:  if CheckGameDeadline(gs, "trigger_drain") {        <- wall clock, a LABEL
53:  resolves++
54:  if resolves > maxTriggerDrainIterations {          <- the 500-cap
55:    LogLoopGuardFired(gs, "trigger_drain_cap", ...)

$ grep -rn 'drainPendingTriggers(' --include='*.go' internal/gameengine/
trigger_batch.go:103   trigger_batch.go:118 (def)   stack.go:2082
```

`stack.go:2082` calls it from inside `ResolveStackTop`, so the function
is re-entrant, and `resolves` is reset at every entry. **A cascade 26
frames deep gives each frame a fresh counter, so the 500-iteration cap
can never accumulate.** Confirmed, and it matches the measurement that
the cap stayed flat at 20 across the whole window while the wall clock
was the only thing that ended the game.

Generalisation worth stating: **a counter guard declared local to a
re-entrant function cannot see recursion.** Whether other guards share
that shape is OPEN — a quick scan over the nine `LogLoopGuardFired`
labels mis-attributed one result to a test file, so it was not an audit
and no count is reported from it.

### Correction to the brief that commissioned this

The brief called `trigger_drain` "a NEW deadline actor, previously only
`token_create` ever tripped." That overstates what the evidence
supports. The `deadlines=` field is itself only ~11 hours old and was
**inert until r168** (a missing label in an allow-list), so this is a
first *observation under new instrumentation*, not a proven first
*event*. The weaker claim is the correct one.

### Why this matters for r171

r171 short-circuited `DependencyOrder` for every layer that provably
cannot contain a dependency, and **deliberately left layer 6 alone**
because layer 6 is where dependencies are real. This stall's chain ends
in `applyLayer(6) -> DependencyOrder`, with the candidate slice growing
262 -> 407 -> 470 -> 482 across successive dumps. **The one case r171
excluded on purpose is the one now stalling.** That is not a defect in
r171; it is the part it could not safely take, coming due.
