# r168 — the field r167 shipped never printed

## What happened

r167 added `deadlines=` to the grinder line so a capped game would name
the check site that cut it. It was gated, controlled 3/3, deployed, and
reported.

Then two real capped games happened — **599.7s at 19:32:03 and 599.3s
at 19:57:09**, both against the 600s cap, exactly the events the field
existed to attribute. The log said:

```
guards=game_deadline:2        the counter saw both
deadlines=                    absent entirely
```

## Why

`CheckGameDeadline` logs its firing with these details:

```go
LogLoopGuardFired(gs, "game_deadline", map[string]interface{}{
    "site":         site,
    "turn":         gs.Turn,
    "stack":        len(gs.Stack),
    "deadline_utc": at.UTC().Format(time.RFC3339),
    "over_by_sec":  int(time.Since(at).Seconds()),
})
```

and r162's label allow-list was:

```go
var loopGuardLabelKeys = [...]string{"event", "handler"}
```

`"site"` is not in it. `loopGuardLabel` returns `""`, `bumpLoopGuardLabel`
returns immediately on an empty label, and **no (guard, label) pair is
ever recorded for `game_deadline`.** r167's renderer was reading a table
that has no rows for this guard and never could have.

The fix is one word: `"site"` joins the allow-list, **last** in
priority, so every guard already attributed by `event` or `handler`
keeps the label it had. The only guards that pass `"site"` are
`game_deadline` (4 fixed sites) and `trigger_loop_cap`
(`ast_trigger` / `per_card_trigger`) — both bounded in the source, and
neither previously carried any label at all.

## How it got shipped

r167 had five tests. Four called `bumpLoopGuardLabel` directly with a
hand-written label; the fifth read `showmatch.go`'s source text. **Not
one of them drove `CheckGameDeadline`.** Every test asserted on the
renderer's behaviour given a label, and the defect was that the label
never arrives.

Three rules written down before shipping it, all broken by it:

- *An instrument you have not seen produce output is not an instrument.*
- *Don't reason about coverage — drive the real code over the real data.*
- *Build the instrument so it can kill your own hypothesis.*

The controls could not catch this either: all three disabled parts of
the renderer, and the renderer was correct. **A control only probes the
code it perturbs.** Nothing perturbed the path from the guard to the
label, because no test went near it.

## The tests now

All four start at the production entry point and assert on the
rendered string, with no hand-fed labels:

| test | property |
|---|---|
| `CheckGameDeadlineRendersItsSite` | one real breach → `token_create:1` |
| `EverySiteNameSurvivesToTheField` | all four live sites reach the field |
| `UnexpiredDeadlineRecordsNoLabel` | the field stays empty on the normal path |
| `ExistingGuardsKeepTheirEventLabel` | `site` last: `event` still wins |

**3 of the 4 fail against the shipped r167 binary.** That is the real
control for this round, and it is the one r167 lacked.

## Scope

Pure instrumentation, like r167: no rules, evaluation or decision path
touched, so it cannot be red on the behaviour axis. ELO comparable.

## What it does not fix

It does not make r166 confirmed. It makes the **next** capped game
decidable; the three that have already happened stay unattributed,
because the label was never recorded for them and cannot be
reconstructed from the log.

## Bystander observation

Three stall episodes in the r167 segment, each cleanly bounded and each
recovering to ~500 g/min:

```
19:05 – 19:08   209.5s game   (ended naturally, under the cap)
19:22 – 19:31   599.7s game   capped
19:48 – 19:56   599.3s game   capped
```

With 18:21's 599.9s that is **three capped games since r166 and zero
overruns**, against pre-fix overruns of 2199.1s, 1442.7s and 1365.6s.
Suggestive, still not attributed, which is the whole point of this
round.

Clean band for the segment: median **494.0 g/min** (n=94) against
r164's 490.9 and r166's 488.1. Stall band median 254.0 (n=45).
