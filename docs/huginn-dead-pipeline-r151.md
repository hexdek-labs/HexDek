# r151 — the co-trigger pipeline has been silently dead since June, and the grinder still paid for it

Round r151 / r151.1. Commits `5652d44f`, `e167a4be`.

This round started as "read the first clean spike profile from the r150 trap"
and ended somewhere else. The trap worked; what it named was not the layers
system.

## The captures

Three post-r150.1 firings existed. Two were worth differencing. The baseline
rotates every 2 windows, so each diff spans two windows, not one — worth doing
the arithmetic before reading anything into the shares.

| spike | windows | games | alloc | steady state | excursion share |
|---|---|---|---|---|---|
| 19:42:02 | 860+1053 MB | 19 | 1913 MB | ~355 MB | **81%** |
| 20:15:21 | 3342+4388 MB | 325 | 7730 MB | ~5395 MB | **30%** |

The 19:42 capture is the cleanest signal this lane has had: its first window
completed **zero games** while allocating 860MB, so the stall is inside the
differenced interval rather than beside it. r150's first capture was ~90%
steady state and could support no claim; this one can.

`sys` stepped 1353 → 2512MB inside the 20:15 window — a real excursion
signature, not a rate blip.

## What the diffs named

Two different shapes, deliberately not merged.

**19:42** — `ResolveStackTop` 92.19% cum, through `resolveCreateToken` (23.60%)
→ `FirePermanentETBTriggers` → `fireObserverETBTriggers`, whose line 66
(`perms := make([]*Permanent, len(seat.Battlefield))`, a defensive copy per
seat per ETB) is **404MB flat, 23.14%**. Also `resolveCounterMod` 44.33% cum,
with its `LogEvent` Details map at 129MB and the `counter_placed`
`FireCardTrigger` map at 139MB — the eager-Details class, confirmed by profile.

**20:15** — `heimdall.ExtractCoTriggers` **1.89GB flat from one line, 26.91%**,
called only from `(*Showmatch).runOneGameFast`. Plus `os.readFileContents`
332MB at 1.09MB per allocation.

That `os.readFileContents` was an open unknown in this lane ("something
re-reads files during play, unidentified"). It is **identified**:
`huginn.ReadRawObservations` via `huginnAdapter.IngestCoTriggers`, once per
game.

## Three stacked defects

**1. `atomicWriteJSON` raced on a fixed `path + ".tmp"`.** The grinder runs two
workers. Both open that path with `O_TRUNC`, then each writes at its own
offset: a short document lands on top of a long one and the long one's tail
survives past its end. The live file on .206 is exactly that — a complete
array closing at line 25697 of 32695, then the middle of a longer document.
Fixed with a per-call unique temp name, so the rename is the only shared step.

**2. The failure was silent.** `IngestCoTriggers` did
`existing, err := ReadRawObservations(dir); if err != nil { return }`. The file
had been unparseable **since 2026-06-07**. Every game read it, failed, and
dropped the batch, and nothing anywhere said so. Four months. It now logs and
keeps the batch.

**3. The cost was paid anyway, every game.** A full read-modify-write of an
ever-growing history per game. Now buffered and flushed in batches, bounded by
a cap — the shape the seed sink in heimdall's `Observer` has used all along,
one function away.

And the allocation itself: `ExtractCoTriggers` paired the **raw** ETB name
list, making it quadratic in cards rather than in names. Co-trigger pairs are
keyed on card name, so a turn that put 500 Saproling tokens onto the
battlefield emitted C(500,2) = **124,750 pairs, every one of them
("Saproling","Saproling")**. The adjacent-turn arm already skipped those
self-pairs by name; the same-turn arm was the inconsistent one. Dedup fixes
the allocation and the aggregation skew together.

## Two wrong theories, killed by measurement rather than argument

Worth recording because both were attractive and both were load-bearing.

**"It's a runaway feedback loop."** A per-game read-modify-write of a growing
file is a textbook ratchet, and it would have explained `sys` never returning.
Checked before building on it: the file was 1,138,279 bytes with an **mtime of
June 7**. Not growing. Dead.

**"The file must be unparseable, so the writer early-returns."** Head and tail
both looked like valid JSON, so this needed a test, not a read. Running the
real file through the real loader gave `invalid character 'B' after top-level
value` — unparseable, but for the splice reason above, not the one assumed.

## r151.1 — the fix doubled allocation, measured four minutes later

With the pipeline working again the history reached its 20,000 cap (11MB) and
the read-modify-write ran every ~31 games. Allocation went **16.7 → ~32
MB/game**: r151 doubled the thing it set out to cut.

The per-game cost is `history_bytes * obs_per_game / flushEvery`, so the flush
interval is the lever and the cap is not — raising it cuts the cost
proportionally while keeping the history that makes the data worth collecting.
500 → 10000.

Measured after: **17.1 MB/game** over ten windows, 407 g/min cum, gc/game 0.06,
turns/game 53.8–57.3, 0 aborts, 0 slow games.

## What is and is not claimed

**Claimed:** the corruption cause is fixed and reproduced by a control; the
silent failure is now visible; the pipeline persists observations for the first
time since 2026-06-07; the quadratic term in `ExtractCoTriggers` is gone;
steady-state allocation is unchanged at baseline.

**Not claimed:** an allocation win. 17.1 against a pre-r151 16.7 is flat. What
the round bought is a **working pipeline at no steady-state cost**, not a
reduction. The storm-path saving is real arithmetic but cannot be demonstrated
end to end until another storm happens.

## Still open, named by these same profiles

- **`fireObserverETBTriggers:66`** — 404MB flat, 23.14% of the 19:42 diff. The
  per-seat battlefield copy is defensive against in-place compaction during the
  loop, so it is **not** a free deletion; it wants a scratch buffer, and
  re-entrancy needs checking first.
- **The eager `LogEvent` Details maps** — 129MB + 139MB at `resolveCounterMod`
  alone, built while the grinder runs `EventLogNone` and retains nothing.
- **The layers system** — still the steady-state top, still untouched. The
  r150 "layers share rises in a spike" hint is **not** reproduced by either of
  these cleaner captures; neither diff is led by layers. Do not carry that hint
  forward as though it were evidence.
