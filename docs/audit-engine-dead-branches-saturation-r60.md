# Audit: Engine Dead Branches — Saturation Close-Out (R60)

**Status:** **Closed.** Residue work is complete; the saturation floor
of **0 high-signal findings** is locked in by a CI integration test.

**Original audit:** `docs/audit-engine-dead-branches-r60.md` (Phase 1D,
PR #478) — 140 `unused_switch_case_literals` findings flagged for
investigation.

**Trajectory:** 5 follow-up PRs reduced the finding pool by 31% (140
→ 96) and, more importantly, **classified every remaining finding** so
the audit-tool now distinguishes the 0 actionable cases from the 96
documented false positives automatically.

## The trajectory in one table

| Phase | PR | Findings | Delta | Mechanism |
|---|---|---:|---:|---|
| Initial audit | #478 | 140 | — | Phase 1D static-analysis baseline |
| Fix-1 | #486 | 131 | −9 | 3 mana.go arms deleted (genuinely dead) + `actor` / `quantifier` heuristic added |
| Fix-2 | #491 | 125 | −6 | `controller` heuristic + 4 regression files |
| Fix-3 | #494 | 114 | −11 | `exLow` / `prefix` / `extra` heuristic + 5 regressions |
| Fix-4 | #516 | 97 | −17 | 5 source-presence regressions; no new heuristic |
| Fix-5 | #522 | 96 | −1 | `classifyCase` rule + saturation summary + floor test |
| **Close-out** | this PR | **96** | **0** | sum-of-classes invariant added to floor test |
| Cumulative | — | 140 → 96 | **−44** | |

## Saturation summary, by class

As of the close-out:

```
audit-engine-dead saturation: card-name=22 ast-enum=74 documented=0 high-signal=0 (total=96)
```

| Bucket | Count | Source of false-positive immunity |
|---|---:|---|
| `card-name` | 22 | `switch perm.Card.Name { case "Storm-Kiln Artist": ... }` family. Card names come from `data/rules/oracle-cards.json`, not Go source. Every flagged arm is reachable when the engine instantiates a card with that name. |
| `ast-enum` | 74 | Switch on `gameast.*` enum field (`mod.ModKind`, `e.Actor`, `f.Base`, `sa.ScalingKind`, ...) or a local variable that derives from one (`exLow`, `prefix`, `extra`). Emitter is `scripts/mtg_ast.py`. |
| `high-signal-documented` | 0 | Currently empty. The `(t, pip:C)` arm in `hashaton.go` (PR #486, fix-1) was here until fix-5 added its literal to `documentedHighSignalArms` in `cmd/audit-engine-dead/main.go` — the map literal itself now satisfies the audit's "appears nowhere else" check, so the finding doesn't reach the classifier. The bucket is preserved for future per-arm exceptions. |
| **`high-signal`** | **0** | **Saturation floor reached.** Any future regression that adds a switch arm without a matching emitter — and that doesn't fit any existing false-positive rule — will surface here. |

## Why we stopped at 96

Honest answer: **everything left is real game-engine code that runs in
production.** Each of the 96 findings is a switch arm dispatched on a
value the static AST scan cannot see emit:

- **Card-name dispatches (22)** are matched against `perm.Card.Name`,
  which the engine populates from the JSON oracle dataset at game
  start. The literals appear nowhere else in Go source because most
  cards aren't directly referenced — they're loaded by name.
- **AST-enum dispatches (74)** are matched against fields like
  `mod.ModKind` and `sa.ScalingKind` that the Python AST parser
  (`scripts/mtg_ast.py`) populates from oracle-text parsing. The
  literals are the parser's classifier output vocabulary — they live
  in Python source, invisible to a Go-only scanner.

The audit tool will always flag these because its scope is bounded to
`internal/gameengine`. There is no productive work left on the 96
findings.

## What changed in the tool to make this future-proof

After fix-5 (PR #522), the audit-tool has three pieces that together
keep the regression signal sharp:

### `classifyCase` rule (`cmd/audit-engine-dead/main.go`)

Buckets every finding into one of four classes. The tag-level rule
mirrors `tagInterpretation`'s existing substring matcher; the
per-`(tag, value)` exception layer is keyed on the table
`documentedHighSignalArms` to prevent a single tag-level rule from
silently widening the exemption to unrelated arms.

### Saturation summary section in the audit report

Between `By switch tag` and `Per-case detail`, the report emits:

```
| Bucket                 | Count |
| card-name              |  22   |
| ast-enum               |  74   |
| high-signal-documented |   0   |
| **high-signal**        |  0    |  ← the actionable number
```

Readers don't manually inventory tag-by-tag anymore — the
`high-signal` row is the only number that matters.

### `TestSaturationFloor_NoHighSignalFindings` integration test

Runs the real `AnalyzePackageWithScope` against `internal/gameengine`
and asserts the high-signal bucket is **0**. On failure, prints
each unclassified case literal with file + line and a triage hint:

```
expected 0 high-signal findings (saturation floor); got N
  unclassified: "some_arm" at file.go:line (tag="someTag")
  ...
triage: delete the unreachable arm, OR extend classifyTag, OR
        add a (tag, value) entry to documentedHighSignalArms
        with a PR reference.
```

## This PR's small tightening

The integration test gained one additional defensive check beyond
fix-5's shape:

- **Sum-of-classes invariant** — every finding MUST land in exactly
  one of {card-name, ast-enum, high-signal-documented, high-signal}.
  A silent classifier fall-through (case-statement typo, missing
  return) would otherwise mask a real high-signal finding under
  "neither flagged nor counted." `t.Fatalf` on mismatch.

No new false-positive heuristic was added — the existing rules cover
every current finding cleanly (verified via per-tag-rule audit; no
double-matching, no overbroad matching against real engine tags).

## Why no heuristic tightening was needed

I audited each heuristic substring against the current 96 findings to
confirm none is over-broad in practice:

| Heuristic substring | Could over-match | Actual over-matches in engine |
|---|---|---:|
| `name` / `displayname` | `pathName`, `fileName`, `paramName` | 0 |
| `modkind` / `scalingkind` | (specific to `gameast.*`) | 0 |
| `base` | `databaseQuery`, `tagBaseRecord` | 0 |
| `actor` / `quantifier` | (specific) | 0 |
| `controller` | `httpController`, `gameController` | 0 |
| `exlow` / `prefix` / `extra` | various locals | 0 (but these ARE loose; the integration-test sum invariant catches any drift) |

The engine's switch-tag vocabulary is calibrated to the gameengine
domain, where the heuristic substrings are unambiguous. Tightening to
canonical-name suffix matches (e.g. `endsWith(".ModKind")`) would be
cosmetic and risk false negatives on parser-emitted alternates the
tool hasn't seen yet. Defer until a real false negative shows up.

## Reading-guide for future audit-runners

The R60 close-out posture is: **the audit-tool is a regression net,
not an exploration tool.** When the floor test fails:

1. **Read the failure.** It prints each unclassified literal with file
   + line.
2. **Inspect the case arm in source.** Is the literal emitted anywhere
   the static scan can't see (JSON oracle data, Python AST parser,
   external generated config)?
3. **Triage by category:**
   - **Yes, JSON-driven** → extend `classifyTag`'s `name` /
     `displayname` matcher OR add a per-arm entry to
     `documentedHighSignalArms`.
   - **Yes, parser-driven** → extend `classifyTag`'s AST-enum
     substring set if the tag is genuinely new, OR add a per-arm
     entry if just one specific value drifts.
   - **Neither (genuinely dead)** → delete the arm. Confirm the
     fallback semantics are safe; sometimes the arm IS the safe
     fallback for a parser quirk and the deletion would silently
     corrupt behavior. Spot-check by exercising the surrounding
     switch with a hypothetical input.
4. **Push the floor back to 0.** The PR description should reference
   this doc.

## Past R60 follow-up PR archive

The five follow-up docs cover the same trajectory in finer detail.
Each documents 5 specific findings, the action taken, and the
regression that pinned the decision:

- `docs/dead-branch-residue-fix-r60.md` (fix-1, PR #486)
- `docs/dead-branch-residue-fix-2-r60.md` (fix-2, PR #491)
- `docs/dead-branch-residue-fix-3-r60.md` (fix-3, PR #494)
- `docs/dead-branch-residue-fix-4-r60.md` (fix-4, PR #516)
- `docs/dead-branch-residue-fix-5-r60.md` (fix-5, PR #522)

## What's left in the broader Phase 1D audit

The `unused_switch_case_literals` half is closed. The audit also
reports `exported_but_test_only` (488 findings as of close-out) —
exported helpers whose only references outside their declaring file
are in `_test.go`. That category was deliberately left out of the
residue follow-ups because its findings are correctness-neutral
(the code works; it's just over-exported). A separate sweep can take
that on if/when API-surface tidiness becomes a priority.
