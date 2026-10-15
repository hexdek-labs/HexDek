# r115 — the throughput sag is one worker captured by one enormous game

r113 and r114 characterised the sag and eliminated four causes. This round
finds the mechanism, explains why r114's search missed it, and contradicts one
of r113's claims with fresh data.

**One of the two grinder workers gets captured by a single enormous game. The
other keeps completing games at its normal rate. Throughput halves until the
long game ends.**

## 1. It is not aging — the fresh process started slow

The r114 `kill -QUIT` gave a clean process to restart the watch on. Eighths
from marker 220271:

```
    8th      time   games  rate med  rate min  <300     sys   heap
      1  06:39:53    1345     202.9     171.9    12    1360    786
      2  06:45:53    3009     218.1     179.9     8    1380    867
      3  06:51:53    5375     396.1     320.1     0    4726    822
      4  06:57:53    7732     392.9     341.9     0    4730    778
      5  07:03:53    9543     289.9     192.1     6    4733    848
      6  07:09:53   10615     198.1     106.0    12    4735    912
      7  07:15:53   12046     206.0     166.2    10    4736    943
      8  07:25:23   15726     395.8     284.0     1   12865    812
```

**The process began in a sag and is fast now, 15,726 games later.** Aging is
dead as an explanation — r114 had already downgraded it to "episodic", and
this settles it.

It also makes the r113 trigger unusable on this window: eighth-1 is itself a
sag, so "15% below eighth-1" would need 172.5, and the *recovery* to 396 reads
as a +95% improvement. A baseline sampled at an arbitrary start is not a
baseline.

The useful shape is **bimodal: ~200 and ~395, a clean 2:1** — and the grinder
runs exactly **2 workers**.

## 2. Both sags are bracketed by one enormous game

```
  sag 06:34:23 -> 06:43:53  (9m)    SLOW GAME 610.7s turns=53 logged 06:43:45
                                    => started ~06:33:34   rate 214.1 -> 347.3
  sag 07:01:23 -> 07:14:53  (13m)   SLOW GAME 828.2s turns=56 logged 07:14:36
                                    => started ~07:00:48   rate 189.4 -> 360.0
```

Start times line up with the sag onsets to within a minute, durations match
the sag lengths, and throughput recovers the moment each one is logged. Two
for two.

The decks involved:

```
610.7s  ms_bumbleflower · krenko_mob_boss · mass_of_mysteries · acererak_the_archlich
828.2s  oloro_lifegain · jodah_the_unifier · jodah_the_unifier · jon_irenicus
```

**Krenko, Mob Boss** is the deck r111 measured as most frequent in the `>=480s`
bucket, and the second game carries *two* Jodah, the Unifier decks. Both are
wide-board archetypes, which is the lead r111 pointed at.

## 3. Why r114 looked and found nothing

r114 reported "zero slow games in the 06:16–06:32 sag" and used that to
eliminate slow games as a cause. The observation was correct and the inference
was wrong, for a reason worth writing down:

**`SLOW GAME` is logged when the game COMPLETES.** A fourteen-minute game is
invisible for fourteen minutes and then appears at the far edge of the window
it dominated. Searching *inside* a sag for the slow game causing it is
searching the one interval guaranteed not to contain the line.

My own alignment script repeated the mistake in miniature: it asked whether a
`SLOW GAME` was logged within 120s *after* a sag ended, and reported `NONE` for
both — because the line at 06:43:45 falls inside the sag's last sample at
06:43:53, eight seconds early. The numbers in the table above were right while
the script's verdict line was wrong. Reading the rows rather than the verdict
is the only reason this round has a mechanism.

It also explains the two measurements that made the sag look mysterious:
the captured worker is still burning CPU and allocating, so **CPU stays at
199% and GC-per-second stays flat** while completed games halve. Those
eliminations in r114 were sound; they just pointed away from a cause that was
hiding behind a logging convention.

## 4. r113's "the per-turn tail is absent" is contradicted

r113 said r112 had removed the pathological tail, with the worst slow game in
19,269 games running at 0.27 s/turn. In this window:

```
  610.7s / 53 turns = 11.5 s/turn
  828.2s / 56 turns = 14.8 s/turn
```

That is exactly r111's pathological class — a median 11.16 s/turn — alive in
15,726 games. **r112 reduced the class; it did not eliminate it.** r113's
hedge ("absent from this window" rather than "eliminated") was the correct
call and is now cashed in.

## 5. Withdrawing the deck-list re-request — again

r114 re-requested the per-game deck-list diagnostic, on the grounds that
identifying which matchups run during a sag was the only way to test the last
candidate. **That is no longer true: the `SLOW GAME` line already carries the
deck list of the game responsible**, which is exactly how the two above were
identified. No new logging is needed.

This is the second reversal on the same item — withdrawn in r111, re-requested
in r114, withdrawn again now. Each turn had a reason, but the net effect is
that I asked a human twice for something I did not need, so it is worth being
blunt about rather than letting it quietly drop off the list.

## Where this leaves issue 16

The sag is not a separate phenomenon. It is **the r111 per-turn cost pathology,
observed from the throughput side**: one expensive game captures one of two
workers for ten to fourteen minutes. Fixing the sag and fixing the pathological
games are the same work, and r112 already took the largest bite out of it.

The next measurable step is unchanged and needs nobody: **sweep the other hot
paths for board-size superlinearity** (combat, trigger dispatch, the stack —
r112 only swept what r80/r85 had fixtures for). The two games here are
wide-board decks, which is where another quadratic would show.

No code change this round. `go build ./...` clean. Prod stays on r112, pid
2872253 unchanged, 0 panics, health 200, one wrapper.

## Lesson 67

**An event logged on COMPLETION is invisible for exactly as long as it
matters.** When a log line is written at the end of a thing, align on its
START — timestamp minus duration — not on its timestamp. r114 searched a sag
window for the slow game causing it and found none, because that window is the
one interval the line cannot appear in.
