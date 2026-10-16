# The once-each-turn limiter, and a correction to the r110 filing

Status: **measurement and plan.** No code. Two prior claims of mine are
corrected here, and one candidate is killed.

## 1. The gc/game precursor is dead — 17% precision

r138 flagged that throughput halved and `gc/game` rose to 0.15–0.18 for
~2 minutes before both heap spikes, and said explicitly that nobody
should build a monitor before measuring the false-positive rate (the
r129 lesson, which closed sag-as-crash-predictor at 21%).

Measured over all 884 grinder windows of 2026-10-16. An "episode" is
>= 4 consecutive 30s windows (>= 2 minutes) at gc/game >= 0.15:

```
00:49:13 -> 00:51:13   5 windows  peak 0.36    no spike
01:44:13 -> 01:45:43   4 windows  peak 0.30    no spike
05:28:55 -> 05:30:55   5 windows  peak 0.24    no spike
06:36:14 -> 06:37:44   4 windows  peak 0.18    SPIKE followed
07:00:14 -> 07:05:14  11 windows  peak 0.23    no spike
07:20:44 -> 07:25:14  10 windows  peak 0.23    no spike
```

**Six episodes, one spike: 17% precision.** And the 05:18 spike had no
qualifying episode before it at all, so recall is 1 of 2.

Statistically indistinguishable from the sag predictor r129 closed.
**Do not build this monitor.** The item is closed, not deferred.

## 2. The r110 filing is wrong for 102 cards

r110 has been carried for many rounds as:

> the parser emits `Modification(kind="timing_restriction")` as a STATIC
> node instead of populating the structured `Activated.timing_restriction`
> FIELD.

The first half is right. **The proposed destination is wrong for a fifth
of the population.** Measured on the live dataset:

```
timing_restriction static nodes            831   (not 819 — the dataset moved)

by restriction                              by sibling ability types on the card
  activate only as a sorcery     455          (Activated,)                   385
  activate only during           133          (Activated, Triggered)         142
  once_per_turn                  120          (Activated, Keyword)           133
  activate only once             115          (Activated, Keyword, Triggered) 67
  activate no more than twice      5          (Triggered,)                    58
  activate no more than three      3          (Keyword, Triggered)            42
                                              (Keyword,)                       2
                                              ()                               2
```

**102 of these cards carry no Activated ability at all.** Kambal,
Profiteering Mayor is one: its "this ability triggers only once each
turn" limits a *triggered* ability. `Activated.timing_restriction`
cannot hold it. Any fix that only populates that field silently drops
those 102.

## 3. What the limiter actually needs: a chokepoint that does not exist

The parser does emit the clause — this is not a grammar gap:

```
Static{Modification{kind: "timing_restriction", args: ["once_per_turn"]}}
raw: "this ability triggers only once each turn"
```

Two problems, in order:

**(a) The node is detached.** It is a sibling of the ability it limits,
with nothing binding the two. "This ability" has no referent in the AST.
For the 58 Triggered-only cards the binding is unambiguous (exactly one
candidate); for the 142 Activated+Triggered cards it is not.

**(b) There is no single place to enforce it.** AST `Triggered`
abilities are dispatched from **31 type-assertion sites across 18
files** — `combat.go`, `etb_dispatch.go`, `zone_change.go`, `phases.go`,
`event_ast_dispatch.go`, `observer_triggers.go`, `layers.go`,
`class_level_triggers.go`, the keyword files, and more. Adding a
per-turn guard at each is the multi-site whack-a-mole this codebase has
been bitten by before.

This is why r139 ships no code. The honest order is:

1. **Bind the restriction to its ability in the exporter** — the parser
   knows which clause it split off. Emitting it as a field on the
   Triggered/Activated node it modifies removes the referent problem at
   the source and costs the engine nothing.
2. **Introduce one chokepoint** for AST triggered-ability firing, or a
   shared guard the 31 sites call, before any enforcement lands.
3. **Then** enforce, with the per-turn counter reset where
   `maxTriggerFiresPerTurn` is reset (`internal/tournament/turn.go:146`).

Doing (3) first, at one or two convenient sites, would produce a card
that honours its limiter in combat and ignores it on ETB — worse than
the current uniform "never honoured", because it would look fixed.

## Relation to the token runaway

Kambal is in the r138 Sorin deck alongside Anointed Procession, and its
unlimited re-trigger remains the leading candidate for the 277-token
census. **That candidate is unchanged and still unproven** — this
document explains why the fix is three steps rather than one, not that
the suspicion has weakened.
