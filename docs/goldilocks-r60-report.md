# Thor Goldilocks — R60 Sweep Report

**Date:** 2026-05-30
**Branch:** `dev/goldilocks-r60-sweep` (built from `origin/main` HEAD `b3ae63bd`)
**Invocation:** `go run ./cmd/hexdek-thor/ --goldilocks --report /tmp/goldilocks-r60-report.md`
**Runtime:** 1.25 s (effect + keyword phases)

## Headline

```
3 invariant failures / 31,963 tests — REGRESSION vs the 2026-05-26 zero baseline.
```

The goldilocks suite was at zero through 3 consecutive sweeps (PR #237 / 2026-05-24, PR #443 follow-up / 2026-05-25, in-place re-run / 2026-05-26). Three new invariant violations have appeared since, all attributable to the PR #694 "batch AI — 5 §400.7c exile-then-cast staples" wave merged 2026-05-27 (`a5d8e530`) and its sibling work.

| metric                  |   count |
| ----------------------- | ------: |
| cards loaded (oracle)   |  35,708 |
| cards w/ testable AST   |  31,963 |
| effect tests run        |  30,341 |
| effect passes           |  30,338 |
| **effect invariants**   |   **3** |
| effect dead-effects     |       0 |
| effect panics           |       0 |
| skipped (no abilities)  |   4,106 |
| keyword tests           |   2,013 |
| keyword passes          |   2,013 |
| keyword failures        |       0 |

## Failures (per-card)

| # | Card              | Interaction              | Invariant            | Message |
|---|-------------------|--------------------------|----------------------|---------|
| 1 | Possibility Storm | `untyped_effect`         | CardIdentity         | `card "Thor Probe Bolt" appears in both seat 0 hand and seat 0 exile` |
| 2 | Knowledge Pool    | `ability_word`           | CardIdentity         | `card "Thor Probe Bolt" appears in both seat 0 hand and seat 0 exile` |
| 3 | Hostage Taker     | `parsed_effect_residual` | ZoneCastGrantExpiry  | `grant for "Opponent Creature" (zone=exile duration=while_source_on_bf) has expired but is still in ZoneCastGrants` |

## Categorization

### Category A — `spell_cast` trigger exiles the cast spell but scaffold leaves it in hand (2 of 3)

Possibility Storm and Knowledge Pool both register an `OnTrigger("...", "spell_cast", ...)` handler whose body exiles the cast spell via `MoveCard(gs, card, casterSeat, "stack", "exile", ...)`:

- `internal/gameengine/per_card/chaos_cascade.go:60` (Possibility Storm)
- `internal/gameengine/per_card/per_card_batch_ai_r60.go:336` (Knowledge Pool, `knowledgePoolOnSpellCast`)

In live gameplay the cast spell is on the stack when the trigger resolves, so the `from=stack → to=exile` move is correct. The goldilocks adversarial-cast scaffold (`cmd/hexdek-thor/opponent_autodetect.go:283-294`) instead places "Thor Probe Bolt" in the caster's HAND and calls `FireCastTriggers(gs, 1, spellCard)` without pushing it through the stack first. The on-cast handler then runs `MoveCard` against a card whose actual zone is "hand", not "stack" — depending on `MoveCard`'s tolerance the card ends up represented in both `Seat.Hand` and `Seat.Exile`, tripping `checkCardIdentity`.

**Two valid framings:**
- **Engine-truth framing:** `MoveCard` should refuse / repair when `from` zone doesn't match the card's actual zone, instead of silently double-registering. That defends every future per_card handler that calls `MoveCard` with a hardcoded `from` zone.
- **Scaffold-truth framing:** `applyAdversarialSetup` should push the synthetic cast onto `gs.Stack` (matching the real cast pipeline) before firing `FireCastTriggers`, so on-cast handlers see the canonical "card on stack, not in hand" precondition.

Recommend doing the scaffold fix first (low-risk, narrow blast radius — `opponent_autodetect.go` only) so the goldilocks signal goes back to zero, then revisit the `MoveCard` precondition guard as a separate defensive-engineering pass.

### Category B — `while_source_on_bf` grant flagged before its source's LTB ever fires (1 of 3)

Hostage Taker's ETB handler (`per_card_batch_ai_r60.go:189-234`) registers a `NewFreeCastFromExilePermission` grant with `Duration = "while_source_on_bf"` and `SourceTimestamp = perm.Timestamp` against the exiled target. `grantIsLeaked` in `internal/gameengine/zone_cast.go:773-786` then walks the battlefield looking for any permanent whose `Timestamp` matches `SourceTimestamp` — if none, the grant is flagged.

The goldilocks scaffold for `parsed_effect_residual` (`cmd/hexdek-thor/goldilocks.go:1035-1077`) places Hostage Taker via `placeSourceCard`, which currently does not stamp `Timestamp` (or stamps it as 0). The grant is registered with `SourceTimestamp = 0`, and `permanentWithTimestampExists(gs, 0)` returns nil because no battlefield permanent has the zero-sentinel timestamp — yet Hostage Taker IS sitting on the battlefield. The invariant therefore false-positives.

Recommend fixing `placeSourceCard` to assign a real `Timestamp` (mirror the canonical `nextTimestamp` flow the resolve pipeline uses) so any per_card handler that captures `perm.Timestamp` at ETB time gets a non-zero, lookup-able value. This is also the smallest-blast-radius fix and will subsume any future per_card handler that registers a `while_source_on_bf` grant during goldilocks.

## Root-cause attribution

Bisection by commit date narrows the regression window to 2026-05-26 → 2026-05-30 (4 days, ~60 merges). The smoking-gun PR is **#694 / `a5d8e530` (2026-05-27) — feat(per_card): batch AI — 5 §400.7c exile-then-cast staples**, which wired the Hostage Taker handler that produces failure #3 and which sits in the same code family as the Possibility Storm / Knowledge Pool handlers producing failures #1 + #2. The on-cast `MoveCard` pattern in Possibility Storm pre-dates PR #694 (`chaos_cascade.go` registration is older), but it was not surfaced by goldilocks until the scaffolding for `untyped_effect` / `ability_word` started exercising the adversarial-cast path on the right corpus subset.

These are real per_card / scaffold-interaction bugs, not new engine bugs. None of them reproduce in Loki (live multi-turn fuzz) because the actual cast pipeline routes the spell through the stack and fires proper LTB cleanup — both preconditions the goldilocks scaffold currently violates.

## Recommended next-fix targets

In priority order (smallest blast radius first):

1. **`placeSourceCard` timestamp stamping** — `cmd/hexdek-thor/goldilocks.go` (look for the helper that constructs `*Permanent` for the source card under test). Assign `Timestamp = gameengine.NextTimestamp(gs)` or equivalent before returning. Closes failure #3 (Hostage Taker) and pre-empts every future per_card `while_source_on_bf` grant exposure under goldilocks.

2. **Adversarial-cast scaffold pushes to stack** — `cmd/hexdek-thor/opponent_autodetect.go:283-294`. Before `FireCastTriggers`, build a `StackItem{Kind: "spell", Controller: 1, Card: spellCard}` and call `PushStackItem(gs, item)`. Closes failures #1 + #2 (Possibility Storm + Knowledge Pool) by giving the on-cast handlers the canonical "card is on stack" precondition. As a sibling change, the post-scaffold cleanup should pop the stack item if the handler didn't consume it.

3. **Defensive `MoveCard` `from`-zone validation** — engine-side, lower priority. Have `MoveCard` log a `move_card_zone_mismatch` event (and optionally repair by clearing the card from its actual zone first) when the named `from` zone doesn't match where the card actually lives. This is defense-in-depth for future per_card handlers and does not block #1 + #2.

None of the three failures need a CLAUDE.md Issue Log Open entry as standalone engine bugs — they are goldilocks scaffold gaps surfaced by recently-wired per_card work. Document them in this report and in the PR description; close them with the fixes above.

## Trajectory

| Date              | Goldilocks failures | Δ vs prior | Notes |
|-------------------|--------------------:|--:|-------|
| 2026-05-08 (pre-fix)        | **1,915** | — | baseline before `RetainEvents:true` + combat-scaffold rewrite |
| 2026-05-08 (post-fix)       | **54**    | −1,861 (−97.2%) | keyword_dead 1,795 → 0; 54 long-tail remained |
| 2026-05-25 (zero-confirm)   | **0**     | −54 (−100%)     | r60 per_card / engine work absorbed the long-tail |
| 2026-05-26 (re-run)         | **0**     | 0               | three consecutive sterile sweeps |
| **2026-05-30 (this sweep)** | **3**     | **+3** | regression introduced by PR #694 batch AI exile-then-cast wave |

Cumulative delta vs the 2026-05-08 pre-fix baseline remains **1,915 → 3 (−99.84 %)**. The 0 → 3 step is the first goldilocks regression in r60 and should land back at zero within one fix pass per the recommendations above.

## Run details

- AST corpus: `data/rules/ast_dataset.jsonl` (symlinked from main repo)
- Oracle corpus: `data/rules/oracle-cards.json` (symlinked from main repo)
- Workers: 10 (`runtime.NumCPU()`)
- Phases: off (default)
- Report: `/tmp/goldilocks-r60-report.md`
- Top failing cards: 3 unique (1 fail each)
- Failures by interaction: 3 × `goldilocks_invariant`
- Failures by invariant: 2 × CardIdentity, 1 × ZoneCastGrantExpiry
- Panics: 0

## Reproduction

```bash
git checkout dev/goldilocks-r60-sweep      # or: git fetch origin && git checkout origin/main
ln -sf $(pwd)/../../../../data/rules/ast_dataset.jsonl data/rules/ast_dataset.jsonl
ln -sf $(pwd)/../../../../data/rules/oracle-cards.json data/rules/oracle-cards.json
go run ./cmd/hexdek-thor/ --goldilocks --report /tmp/goldilocks-r60-report.md
```

Expected on current `origin/main`: 3 failures (Possibility Storm, Hostage Taker, Knowledge Pool) as listed above. Expected after the three recommended fixes ship: back to 0.
