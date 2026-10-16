# The heap spike names a mechanic — 2026-10-16 06:38

Status: **evidence, not a fix.** The r131 census has for the first time
produced a mechanism-bearing lead for the #1 engine item.

## What fired

A second heap spike, same shape as 05:18 (r136):

```
06:36:14  3990 games  206.0 g/min now  gc/game=0.17  heap=927MB   sys=1338MB
06:37:14  4185 games  188.2 g/min now  gc/game=0.17  heap=895MB   sys=1355MB
06:38:14  4651 games  738.1 g/min now  gc/game=0.03  heap=6198MB  sys=10127MB
06:38:44  5094 games  886.4 g/min now  gc/game=0.03  heap=835MB   sys=10146MB
```

611 aborts inside ~30 seconds, then full self-recovery.

## r136's verdict: it worked, and it did not stop the collateral

- **0 deferrals**, against dozens in the 05:18 event. The `deferralMemo`
  did what it was built for.
- **Two aborts at turns 40 and 50**, not newborns. Under the pre-r136
  ordering a long-running game over the ceiling took the deferrable
  `delta` verdict; it can now die. That is the fix demonstrating itself.
- **609 of 611 are still turn-10 newborns.** This is inherent to a
  process-wide ceiling: once the heap is over it, every freshly started
  game trips on its first sample. Not a regression, but the guard still
  cannot spare the innocent during the window.

## The census named the mechanic

```
06:38:07  aborting game at turn 10 — process heap 7425MB exceeds the 6442MB ceiling
  seed=1792132687444364945
  decks=[moxfield/sorin_of_house_markov_sorin_ravenous_neonate_b2_khazad_doom_JfUAlsri
         moxfield/ms_bumbleflower_b2_tigeratx1028_Tu_AQWaM
         moxfield/jetmir_nexus_of_revels_b4_maizescientist_-SdVWMT_
         moxfield/isshin_two_heavens_as_one_b3_wd2222_WytAG4CS]
  s0=[life=42 bf=284(tok=277) hand=4 gy=0 exile=0 lib=89]
  perms=292 tokens=277
```

**277 tokens on turn 10.** Every other abort in the event carries
perms=13..76. This is the absurd count r131 was built to surface, and
the deck holding it is
`sorin_of_house_markov_..._khazad_doom_JfUAlsri` — **the same deck r130
identified as 797 of 963 SLOW GAME lines (83% against 2.7 expected)**.

r130 was right that it was a suspect and right to refuse to call the
association a mechanism. The census has now supplied what r130 could
not: the deck is slow **because something in it makes hundreds of
tokens**, and the token count is visible in the game's own state.

## Candidate mechanism — NOT PROVEN

Measured facts about that decklist (82 entries, 26 token-related):

- **Anointed Procession** — "If an effect would create one or more
  tokens under your control, it creates twice that many instead."
  A multiplicative doubler.
- **Kambal, Profiteering Mayor** — "Whenever one or more tokens your
  opponents control enter, for each of them, create a tapped token
  that's a copy of it. **This ability triggers only once each turn.**"
  **No per_card handler exists.**
- **Caretaker's Talent** — "Whenever one or more tokens you control
  enter, draw a card. **This ability triggers only once each turn.**"
  **No per_card handler exists.**

A doubler plus an unlimited re-trigger is the shape that produces
exponential token counts. It has NOT been driven end to end, and this
file does not claim it has. The next step is a targeted run of that
seed and deck set, not another volume sweep.

## The class behind it

**131 corpus cards carry "triggers only once each turn".** The engine
has no general limiter — only ad hoc per-card implementations
(`jin_gitaxias_progress_tyrant.go`, `rivaz_of_the_claw.go`,
`magda_the_hoardmaster.go`, `pantlaza.go`). Every other card with the
clause can re-trigger without bound.

This is the same shape as r137's `cast_mode_check` finding: a clause
many cards depend on, honoured for a handful and ignored for the rest.

## One observation that needs a false-positive rate before anyone acts

Throughput halved and `gc/game` rose to 0.15–0.18 for roughly two
minutes **before** both spikes, against a normal 0.07–0.10. That is a
candidate early-warning window.

**Do not build a monitor on it yet.** r129 closed sag-as-crash-predictor
at 21% precision, and the lesson there was to measure the false-positive
rate first. Count how often gc/game sits at 0.15+ for two minutes
WITHOUT a spike following before anyone proposes alerting on it.
