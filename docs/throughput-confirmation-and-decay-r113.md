# r113 — (16-TIMING-2): r112's gain held, and the r109 question had a better answer

Three results. The fix is confirmed, the open question from last tick is
resolved and turned out not to be what either answer assumed, and the
residual-exponent thread is closed with evidence.

## 1. r112's throughput gain HELD, on a real window

Last tick's read was 3,811 games / 19 samples / one slow game, and I said so
rather than claiming it. Now at 19,269 games / 97 samples / 5 slow games:

```
        games   now-rate median  turns/game  SLOW/1000  worst game  slow s/turn med / max
r107    55344       370.4          55.40       0.96       151.3s        0.50 / 3.60
r109    42108       312.1          55.50       1.50       603.8s        0.73 / 7.94
r112    19269       402.1          55.50       0.26        20.0s        0.26 / 0.27
```

- **+8.6% against r107**, +28.8% against r109.
- **turns/game identical at 55.50.** That is the control: a pure cost fix must
  not change what the games do, and it didn't.
- **Slow games 0.26 per 1000 against r107's 0.96.** At r107's rate this window
  would have produced ~18.5; it produced 5. Not noise.
- **The per-turn tail is absent from this window.** r111's whole finding was
  that pathological games ran at a median 11.16 s/turn. Here the *worst* slow
  game in 19,269 runs at **0.27 s/turn**, against maxima of 3.60 (r107) and
  7.94 (r109).

Stated honestly: 19,269 games is a third of r107's window and extreme games
are rare, so "absent from this window" is what I can say. "Eliminated" is not.

## 2. The r109 question: resolved, and neither prior answer was right

Last tick I flagged that I had called r109 "no regression" off its first 64
samples (median 364.9) when its full-window median is 312.1 — ~16% below
r107 — and left it unresolved between "r109 regressed" and "the heavy-memory
period depressed it". Splitting each window into quarters settles it:

```
r107 (n=247)      rate median   sys MB   heap MB
   quarter 1          373.9       1230       763
   quarter 2          371.8       1243       757
   quarter 3          366.2       1311       793
   quarter 4          371.0       2088       789

r109 (n=289)
   quarter 1          359.0       7538       780
   quarter 2          347.9       7553       813
   quarter 3          305.1       7565       778
   quarter 4          209.9       7579       861     <- -42% from its own start

r112 (n=98)
   quarter 1          404.1       1419       759
   quarter 2          396.0       1570       811
   quarter 3          406.0       1577       784
   quarter 4          408.9       1583       778
```

**r109 did not start low.** It started at 359.0, within 3% of r107, and
degraded monotonically to 209.9 over its own window. A code regression is
flat-lower from the first sample; this is **progressive degradation inside a
single process lifetime**. So neither candidate explanation was right: r109's
code did not cost 16%, and the memory period did not depress a flat baseline —
something ate 42% of throughput *as the process aged*.

And it reframes my own error precisely. The first-64-samples number was not
wrong; it faithfully measured the undegraded phase. **The mistake was
generalising an early-window reading to the whole window** — which is a
different error from the one I confessed to, and worth naming correctly.

Live heap is flat throughout r109 (780 → 861MB), so this is not a leak in live
data. `sys` sat at ~7.5GB for the entire window while throughput fell, so the
arena growth *preceded* the decline rather than tracking it.

## 3. NEW open finding, with a watch condition that can actually fire

**Something degrades grinder throughput ~40% over a long process lifetime,
with live heap flat.** Observed in r109's process (42,108 games). *Not*
observed in r107's (55,344 games, flat) or so far in r112's (19,269, flat).
The only accompanying signal is a large `sys` — 7.5GB in r109 against 1.2–2.1GB
in r107 and 1.4–1.6GB in r112. Three windows is a correlation, not a cause,
and I am not theorising past that (lesson 20).

**Watch condition, written now rather than at read time (lesson 58):**

- Metric: r112's per-quarter `now-rate` median, against its own quarter-1
  value of **404.1**.
- Trigger: a decline of **>15%** sustained across a quarter = the r109
  phenomenon recurring under r112.
- Close condition: **flat through 55,344 games** (r107's window length, the
  longest clean run on record) = r109-process-specific, close it.
- The denominator is the quarter, not the window, which is the whole point —
  a full-window median is exactly what hid this.

## 4. The residual exponent: thread CLOSED

r112 left `StateBasedActions` at exponent ~1.20 rather than 1.0, and the brief
asked to name the cause or close it. Extending the sweep to n=2560:

```
   n=40  70µs · 80 159µs · 160 328µs · 320 732µs · 640 1.54ms
   1280 3.43ms · 2560 7.71ms
   per-doubling ratio: 2.25 · 2.06 · 2.24 · 2.10 · 2.23 · 2.25
```

**The ratio is stable at ~2.2 across the whole range, including the new
point.** A hidden quadratic term makes that ratio *climb* toward 4; it does
not. Exponent ≈1.14 and flat.

The profile at n=2560 says the same thing:

```
  50ms 15.62%  runtime.mapaccess2_fast64
  40ms 12.50%  internal/runtime/maps.(*groupsReference).group
  20ms  6.25%  (*GameState).IsCreatureOf        (46.88% cum)
```

Map access, not a scan. The characteristics cache is keyed per permanent, so
the *count* of lookups is linear while each lookup gets slightly dearer as the
map outgrows cache. That is a constant factor degrading with n — memory
hierarchy, not algorithm. `strings.ToLower` (9% in the earlier profile) is
per-type-string work inside the layer type queries: linear per permanent, also
not a superlinear term.

**Closed.** What would reopen it: a per-doubling ratio that climbs above ~2.2
instead of holding.

## Verification

Test-only Go delta (one constant in the benchmark file, adding n=2560).
`go build ./...` clean and `./internal/gameengine/` green. No engine change,
nothing to deploy; .206 stays on r112.

## Lesson 65

**A flat summary statistic over a window can hide a monotonic trend inside
it, and the trend is usually the finding.** r109's window median of 312.1 is a
true number that describes no moment of r109's life: it began at 359 and
ended at 210. I read it twice — once too early and called it fine, once whole
and called it a possible regression — and both readings missed that the shape
was a slope. When a window summary disagrees with a sub-window summary, do not
pick between them: **split the window and look at the sequence.**
