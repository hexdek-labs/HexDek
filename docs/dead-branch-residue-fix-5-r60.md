# Dead-Branch Residue Fix #5 — R60 Phase 1D Saturation

Continuation of PRs #486 / #491 / #494 / #516 (fix-1 through fix-4).
After fix-4 the audit's `unused_switch_case_literals` report carried
**97** findings. This PR's task asked to investigate the next 5
candidates with the explicit caveat: **be honest if nothing real is
left.**

## Honest finding: nothing real is left

A full bucket-by-bucket sweep of the 97 remaining findings:

| Bucket | Count | Class |
|---|---:|---|
| `name` (17) + `p.Card.DisplayName(…)` (5) | 22 | card-name JSON dispatch — expected FP |
| `e.ModKind` (27) + `mod.ModKind` (9) | 36 | AST mod-kind enum — expected FP |
| `f.Base` (14) + `base` (16) | 30 | AST filter-base enum — expected FP |
| `sa.ScalingKind` | 5 | AST scaling enum — expected FP |
| `prefix` | 3 | AST-derived (residue #3) — expected FP |
| `t` (1) | 1 | `pip:C` arm — documented exception (residue #1) |

Every tag is already covered by an existing `tagInterpretation`
substring rule, and the one tag (`t`) that ISN'T pattern-matched at
the tag-level has its specific `pip:C` arm-value documented by PR
#486's investigation.

**There are no genuinely-dead arms left to delete, and no new
false-positive class to identify.** Further investigation rounds
would be busywork.

## What this PR does instead

Shift the audit-tool's posture from "find dead branches" to
"watch for new dead branches" by giving the regression signal a
shape:

### 1. `classifyCase` rule + `documentedHighSignalArms` table

New function in `cmd/audit-engine-dead/main.go` that buckets every
finding into one of four classes:

- `card-name` — JSON-driven dispatch (expected FP).
- `ast-enum` — parser-emitted enum from `scripts/mtg_ast.py`
  (expected FP). The substring rule mirrors `tagInterpretation`'s
  existing matcher.
- `high-signal-documented` — known FP that the tag-level matcher
  can't see. Currently one entry: `(t, pip:C)` for hashaton.go's
  pip-stripping loop. Each entry carries a PR reference so the
  investigation trail is preserved.
- `high-signal` — unclassified. The regression signal.

### 2. Saturation summary section in the audit report

The audit now prints, between the "By switch tag" table and the
per-case detail, a saturation summary:

```
| Bucket                 | Count | Meaning |
| card-name              |   22  | switch on Card.Name/DisplayName — JSON-driven dispatch, expected FP |
| ast-enum               |   74  | switch on AST-emitted enum (ModKind, Base, Actor, etc.) — parser-driven, expected FP |
| high-signal-documented |    0  | known false positive pinned by a per-arm regression |
| **high-signal**        |  **0**| **unclassified — investigate.** Zero is the saturation target. |
```

Readers don't have to manually inventory tag-by-tag anymore — the
"high-signal" row is the actionable number. As of this PR it's 0.

### 3. Saturation-floor regression tests

`cmd/audit-engine-dead/saturation_test.go`:

- `TestClassifyTag_CardNameBucket` — pins `name`, `p.Card.Name`,
  `perm.Card.DisplayName`, and the formatted form into card-name.
- `TestClassifyTag_ASTEnumBucket` — pins all 13 known AST-enum
  tag forms into ast-enum.
- `TestClassifyTag_HighSignalFallthrough` — pins that unrelated
  tags (`event.Kind`, `someUnknownLocal`, bare `t`) fall through
  to high-signal at the tag level.
- `TestClassifyCase_DocumentedPipExceptionTakesPrecedenceOverHighSignal`
  — pins the (tag, value)-level rule. `(t, pip:C)` →
  high-signal-documented; `(t, pip:Y)` (hypothetical) →
  high-signal so a new dead arm under the same tag would still
  surface.
- `TestDocumentedHighSignalArms_PRReferences` — every documented
  exception must cite the PR that landed it.

`cmd/audit-engine-dead/saturation_integration_test.go`:

- `TestSaturationFloor_NoHighSignalFindings` — runs the **real**
  `AnalyzePackageWithScope` against `internal/gameengine` (walking
  up from the test file to find the module root, same shape as
  `internal/astload/astload_test.go::corpusPath`). Asserts the
  high-signal bucket is 0. **If a new dead branch is added — or
  a new false-positive class appears — this test fails fast and
  prints the unclassified case literals + their file locations.**

## Verification

- `go build ./...` clean.
- `cmd/audit-engine-dead` + `internal/gameengine` + `per_card` test
  suites clean.
- Running the audit:

  ```
  $ go run ./cmd/audit-engine-dead --out /tmp/audit.md
  unused switch cases: 96
  ```

  The drop from 97 → 96 is a side-effect of adding the literal
  `"pip:C"` to `documentedHighSignalArms` — the audit's
  "appears nowhere else" check now finds the literal in
  `cmd/audit-engine-dead/main.go`. The substantive change is the
  saturation summary, not the count.

- The saturation summary table in the report header shows:

  ```
  card-name: 22
  ast-enum: 74
  high-signal-documented: 0   (← `pip:C` audit-suppressed via the map literal)
  high-signal: 0              ← saturation floor reached
  ```

## Trajectory

| Phase | Findings | Delta | Mechanism |
|---|---:|---:|---|
| Initial (PR #478) | 140 | — | — |
| Fix-1 (#486) | 131 | −9 | 3 mana.go arms deleted + Actor/Quantifier heuristic |
| Fix-2 (#491) | 125 | −6 | Controller heuristic + 4 regression files |
| Fix-3 (#494) | 114 | −11 | exLow/prefix/extra heuristic + 5 regressions |
| Fix-4 (#516) | 97 | −17 | 5 source-presence regressions |
| **Fix-5 (this PR)** | **96** | **−1 (cosmetic)** | classifyCase + saturation summary + saturation-floor test |
| Cumulative | 140 → 96 | **−44** | |

The fix-5 numeric reduction is incidental (one literal slipped into
`documentedHighSignalArms`). The substantive value is the **saturation
floor**: 0 high-signal findings, locked in by an integration test.

## What future PRs should do

Don't re-investigate the 96 remaining findings — every one is
already in a documented false-positive bucket and the saturation-floor
test confirms it.

When a new finding does appear:

1. The saturation-floor integration test will fail with the file +
   line of the unclassified case.
2. Triage:
   - If it's a genuine dead branch → delete the arm (and any
     supporting code).
   - If it's a new AST-enum / card-name local-variable name pattern
     → extend `classifyTag`.
   - If it's a per-arm exception → add to `documentedHighSignalArms`
     with a PR reference.
3. Push the saturation floor back to 0.
