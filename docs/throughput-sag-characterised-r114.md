# r114 — the throughput decay is EPISODIC, not progressive; four causes eliminated

The r113 watch item existed to decide one thing: whether the throughput decay
was specific to r109's process. **It is not.** A second process shows the same
shape, so it is a live, recurring property of the grinder.

Along the way this round corrects r113's characterisation of the phenomenon
and my own reading of a goroutine dump.

## 1. The watch: fired in substance, not against its own trigger

r113 wrote the trigger as **">15% sustained decline across a quarter"** against
r112's quarter-1 of 404.1. r112's quarters:

```
   401.9 · 408.1 · 402.0 · 355.9      q4 = -11.4%
```

**-11.4% does not meet the trigger.** But splitting the same window into
eighths:

```
    8th  games@end  rate med  rate min  <300  sys   heap  gc/gm
      1       4968     404.1     312.0     0  1419   760   0.08
      2       9826     397.9     194.0     1  1571   807   0.08
      3      14825     406.0     340.0     0  1578   802   0.08
      4      19890     409.8     331.9     0  1583   757   0.07
      5      24930     408.1     360.0     0  1589   837   0.07
      6      29817     396.2     313.8     0  1595   799   0.08
      7      34494     396.0     201.9     3  1601   864   0.07
      8      37791     206.0     174.0    18  1604   914   0.13
```

Seven eighths flat at ~400, then **-49%** with 18 of ~25 samples below 300.
Sustained for ~19 minutes and ~3,800 games.

So I wrote a trigger at the wrong granularity. The quarter averages one
collapsed eighth with one and a half flat ones and lands under the threshold.
**This is lesson 43 — a measurement's granularity is part of the measurement —
biting one tick after I made granularity the lesson in r113.** I am calling the
watch fired on the finer evidence and saying plainly that the literal trigger
did not.

## 2. It is EPISODIC, not progressive — this corrects r113

r113 said r109 "degraded monotonically". At eighth resolution it did not:

```
r109  334.1 · 367.0 · 345.9 · 360.9 · 376.0 · 224.1 · 287.9 · 195.9
```

Noisy throughout, sagging at eighth 6, **recovering at 7**, sagging again at 8.
r112 behaves the same way: a dip to 232/202 at 06:10 that **recovered to 400+
by 06:14**, then the sustained sag from ~06:16.

The clean monotonic slope in r113's quarter table was an artifact of quarter
granularity, exactly as the -11.4% was. Same error, both directions.

**Corrected characterisation: the grinder runs at ~400 g/min and drops into
sustained ~200 g/min episodes that recover on their own.**

## 3. Four candidate causes, each measured and eliminated

**External CPU contention — no.** During a live sag: `hexdek-server` at 199%
CPU, load average 2.03 on 8 cores, nothing else on the box above 1%.

**GC work — no, and the signal that suggested it is a ratio artifact.**
`gc/game` doubles during a sag (0.07 → 0.13), which looks like the cause. It
is not. Splitting every sample in the window by observed rate:

```
FAST (>=350 g/min)   n=168   games/30s 202.0   GC/30s 15.0   gc/game 0.074
SLOW (<350 g/min)    n= 36   games/30s 104.0   GC/30s 14.5   gc/game 0.139
```

**GC cycles per unit time are identical.** Games per unit time halve. `gc/game`
doubles mechanically. The denominator moved, not the numerator — r111's lesson
applied to my own metric.

**Slow games — no.** The last `SLOW GAME` before the sustained sag was at
06:11:18; the sag ran 06:16–06:32 with **zero** slow games in it. And
turns/game stayed flat at 51.6–57.6 throughout, so the games are ordinary.

**Showmatch contention — not supported.** The grinder shares the process with a
continuous showmatch loop, so this was the obvious structural suspect.
Correlating each grinder sample against showmatch `finished` events within
±150s: slow samples median 2.0, fast samples median 1.0 (means 1.52 vs 1.33),
with one slow sample having zero nearby and six fast samples having some. Too
weak to carry the explanation. Recorded so nobody re-runs it.

## 4. The goroutine dump — taken live, and it corrected me

Lesson 19 says take the dump when you have a live subject, so I `kill -QUIT`'d
the sagging process (wrapper relaunches in 4s; cost paid, state lost
deliberately). **22 goroutines total**, with one in
`tournament.takeTurnImpl` via `runGrinder → runOneGameFast`, and one in
`takeTurnImpl` via `NewShowmatch → runLoop → runOneGame`.

I read that as "the grinder runs one game at a time". **The restart line says
otherwise and it is explicit:**

```
grinder: starting 2 parallel workers (NumCPU=8 GOMAXPROCS=2, GOGC=50, memlimit=8GB)
```

Two grinder workers; the dump caught one of them mid-turn and the other
between games. And the 199% CPU is not a symptom — **`GOMAXPROCS=2` is
configuration**, set deliberately alongside `GOGC=50` and an 8GB memlimit.
Inferring the configuration from a snapshot was the wrong move when the
process prints it at startup.

## 5. What is left, and the request I am reversing

With contention, GC, slow games and showmatch eliminated, and turn counts flat,
the remaining candidate is **which decks the grinder is playing**. Grinder
throughput is bounded by mean game wall-time across 2 workers; mean game time
doubled with turn count unchanged, so per-turn cost doubled — and r111 already
established that per-turn cost varies enormously with board state. A stretch of
expensive matchups would produce exactly this: sustained, self-recovering,
content-neutral sags.

**I cannot test that, because only `SLOW GAME` lines carry a deck list — and
the instrument that would fix it is the per-game deck-list diagnostic I
withdrew in r111.**

I withdrew it correctly: it was proposed to separate r111's (a) from (b), and
the severity gradient answered that for free. **I am re-requesting it for a
different question, with a different justification**, and saying so rather than
quietly re-asking: log the deck list on every grinder game, so a throughput sag
can be joined to the matchups running during it. Small, reversible, and it is
now the only way to test the last standing candidate.

## Status

- No code change this round. Prod is still r112 (`337d08e0…`), restarted by the
  QUIT: **pid 2850301 → 2872253**, health 200, one wrapper, hash-verified.
- **The r112 watch window is void** — the QUIT reset the process. The quarter
  series restarts from the new log marker.

## Lesson 66

**When you write a threshold, write it at the granularity the phenomenon
lives at — and if you do not know that yet, say the threshold is provisional.**
r104's watch had no trigger and ran five ticks. r113's watch had a trigger and
still missed, because a quarter is wider than the event. A threshold is two
decisions, a number *and* a window, and I only argued about the number.
