# r181 — the hang base rate, and why my own proof bar was unachievable

I said "several hours clean would be proof that r179 fixed the hang", and I
held the r180 deploy to protect that measurement. **Both of those were wrong,
and a month of log history says so.** This is the correction.

## What I never measured: how often does the log actually freeze?

Every grinder window writes a line every 30s. A hang is therefore a gap in
those timestamps. Scanning all 91,674 grinder lines from 2026-09-17 to
2026-10-18 for gaps >= 180s:

| from | to | length | |
|---|---|---|---|
| 2026-09-21 02:20 | 2026-09-21 23:26 | 1265.5 min | server intentionally down |
| 2026-10-15 18:12 | 2026-10-15 18:24 | **12.6 min** | freeze |
| 2026-10-15 20:46 | 2026-10-15 20:56 | **9.2 min** | freeze |
| 2026-10-16 12:37 | 2026-10-16 12:49 | **12.1 min** | freeze |
| 2026-10-16 18:52 | 2026-10-16 18:57 | **4.4 min** | freeze |
| 2026-10-18 13:05 | 2026-10-18 13:24 | **19.5 min** | freeze — the one I diagnosed and SIGQUIT'd |

**The Oct 18 hang was the fifth, not the first.** I had treated it as a novel
event. It is the longest member of a cluster spanning four days.

### The number that invalidates my proof bar

Between the fourth freeze (Oct 16 18:57) and the fifth (Oct 18 13:05) the
server ran **42 hours clean — before r179 existed.**

So a few hours of quiet is entirely consistent with the *unfixed* base rate.
"Several hours clean" could never have distinguished a fixed server from a
lucky one. I set a bar that cannot return a negative, then treated the
inability to clear it as a reason to block other work. The bar had to be
several *days*, or it had to be a leading indicator rather than an absence.

**Consequence, acted on:** r180 deployed. The reason for holding it did not
survive contact with the data.

## A second failure mode, and it is NOT the same one

`grinder: aborting game at turn N — heap delta XMB exceeds budget` fires when
a single game's heap delta blows the budget. 22 occurrences, all time:

```
2026/09/22  1        2026/10/14  5        2026/10/16  3
2026/10/13  3        2026/10/15 10        2026/10/17  0
                                          2026/10/18  0
```

Two of the five freezes are immediately preceded by an **escalating
staircase** of these, each game worse than the last:

```
Oct 15 18:11:47  2423MB      Oct 15 20:46:21  2553MB
Oct 15 18:12:03  4723MB      Oct 15 20:46:28  3769MB
Oct 15 18:12:18  5889MB      Oct 15 20:46:34  4715MB
   -> freeze 18:12, death    Oct 15 20:46:47  5895MB
                                -> freeze 20:46, death
```

2.4GB to 5.9GB in 31 seconds, and 2.5GB to 5.9GB in 26 seconds. A per-game
delta that climbs monotonically across consecutive games is not one
pathological game; it is something accumulating underneath them.

**The other three freezes — including the Oct 18 one I diagnosed — have no
such precursor.** The nearest aborts to the Oct 18 freeze are two days
earlier. So either there are two distinct failure modes here, or one mode
with two presentations. I have a goroutine dump for exactly one of the five
and cannot honestly extend it to the others.

## What this does and does not say about r179

**It does not vindicate r179.** The heap-abort mode stopped after
Oct 16 17:35 and produced nothing on Oct 17 — a full day *before* r179
deployed at 13:40 on Oct 18. Whatever stopped it, it was not r179, and I must
not claim the credit.

**It does give r179 a sibling symptom worth watching.** Unbounded allocation
retained across a game is exactly the shape of `DiffBattlefield`, and an
escalating per-game heap delta is what that would look like from outside. It
is a suggestive match on two of five episodes, and nothing stronger.

## The measurement to use from here

Uptime is a weak instrument against a base rate this sparse. The strong ones
are leading indicators that fire *before* a freeze and can be counted:

1. `exceeds budget` aborts per day — currently 0 for two days.
2. The escalating staircase specifically: 3+ aborts within 60s with a rising
   delta. Two for two on predicting a freeze so far.
3. Freeze count per day from the grinder-gap scan above, which is cheap and
   runs over history as well as forward.

A day with zero aborts and zero gaps means more than an afternoon of uptime.
