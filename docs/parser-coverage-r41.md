# Parser Coverage Audit — R41

Cross-references every entry in `data/rules/oracle-cards.json` against the
AST corpus loaded by `internal/astload` from `data/rules/ast_dataset.jsonl`.
The AST is produced by the Python parser (`scripts/mtg_ast.py`); this
audit verifies its output is loadable and non-empty for every real card.

## Headline

| Metric | Value |
|---|---:|
| Oracle cards examined (post-dedup, non-token, non-Un) | 31247 |
| AST corpus size | 31963 |
| Astload parse warnings | 104 |
| Parse success rate (OK + OK_VANILLA) | **100.00%** |
| Failure rate (MISSING + EMPTY_AST + PARTIAL) | 0.00% |

## Classification breakdown

| Class | Count | Share |
|---|---:|---:|
| OK | 30886 | 98.84% |
| OK_VANILLA | 361 | 1.16% |
| MISSING | 0 | 0.00% |
| EMPTY_AST | 0 | 0.00% |
| PARTIAL | 0 | 0.00% |

### What each class means

- **OK** — card resolves through the astload Corpus and has at least one parsed ability.
- **OK_VANILLA** — no oracle text (basic land or vanilla creature). Empty AST is correct.
- **MISSING** — oracle card has no entry in the AST dataset. Parser pipeline never ingested it.
- **EMPTY_AST** — AST entry exists, card has oracle text, but the parser produced zero abilities. Real parser failure.
- **PARTIAL** — `fully_parsed=false` or non-empty `parse_errors`. Parser partially failed.

## Top 10 failure patterns

_No failures detected._

## Astload corpus warnings (first 20)

_See loader output; 104 warnings total._

## AST node-family coverage (R60 batch)

The card-level parse success rate above stays at 100% (every real card
resolves through the AST corpus with at least one parsed ability). The
next coverage frontier is the **per-node-family scaffold gap** — the
share of `Condition.kind` / `Trigger.event` values inside parsed ASTs
that `cmd/hexdek-thor/conditional_setup.go` does NOT route to a
registered scaffold slug. Each era audit (`scripts/era{1,2,3,4}_scaffold_audit.py`)
walks the AST corpus, classifies each node as bucketed (route exists)
or unbucketed (no route — engine cannot prime the world the listener
expects during goldilocks/keyword tests).

### Top-10 lowest-coverage AST node families (pre-fix)

Aggregated across all four eras at the start of the sweep:

| # | Family | Kind/Event | Count | Era |
|--:|---|---|--:|---|
| 1 | Trigger `self_put_into_graveyard_from_bf` | trigger event | 27 | 4 |
| 2 | Condition `if` (raw, named-counter cluster) | condition kind | 11 | 1+4 |
| 3 | Trigger `ally_type_to_gy_from_bf` | trigger event | 12 | 4 |
| 4 | Trigger `specialize_from_zone` | trigger event | 10 | 4 |
| 5 | Trigger `type_to_gy_from_bf` | trigger event | 8 | 4 |
| 6 | Trigger `ring_tempts_you` | trigger event | 7 | 4 |
| 7 | Trigger `to_gy_from_bf` | trigger event | 6 | 4 |
| 8 | Trigger `opp_type_to_gy_from_bf` | trigger event | 5 | 4 |
| 9 | Trigger `ally_typed_to_gy` | trigger event | 5 | 4 |
| 10 | Trigger `opp_creature_to_gy` / `self_to_gy` | trigger event | 6 | 4 |

The `*_to_gy_from_bf` cluster (entries 1, 3, 5, 7, 8, 9, 10 plus tribal /
nontoken / self_die_or_ally tail) is the parser's underscore-zone-change
wrapper for "battlefield → graveyard"; the substring catch on the prose
"dies" / "is put into a graveyard" upstream of these cases never matched
the underscore form, so every variant fell through to the empty bucket.
The named-counter condition cluster is the long-tail counter-on-self
threshold check ("X or more <named> counters on it") where the existing
`condScaffoldCountersOnSelfGE` matcher's outer guard rejected "this
enchantment has" / "this artifact has" surface variants.

### Three families fixed in this batch

1. **Trigger `*_to_gy_from_bf` family** — 67 nodes. New `classifyTrigger`
   exact-match cases route the 11 variants (self / ally / ally_type / type
   / opp_type / ally_typed / tribal / nontoken / opp_creature / self_to_gy
   / self_die_or_ally) to the canonical `creature_dies` scaffold.

2. **Era-4 long-tail trigger slugs** — 27 nodes. `specialize_from_zone`
   → `specialize_creature`, `ring_tempts_you` / `train` → `when_you_do`,
   `modified_creature_event` → `counters_put_on_self`,
   `face_down_creature_event` → `turned_face_up`, `compound_tribe_enter`
   → `tribe_you_control_etb`, `it_state_change` → `becomes_tapped_trigger`.

3. **Named-counter threshold raw-text** — 11 nodes. Extended both the
   outer guard in `condScaffoldCountersOnSelfGE` (now accepts "this
   enchantment has" / "this artifact has" + "counters on this artifact" /
   "counters on this enchantment" / etc.) and the recognized counter-name
   list (added release / dread / wreck / luck / arrowhead / echo / bounty
   / rad / phyresis). Per_card handlers reading `cs.subtype` now receive
   the printed counter name instead of a `+1/+1` fallback.

### Before / after (era scaffold audits)

Pre-fix and post-fix unbucketed counts across the four era audits. `gap %`
is unbucketed / total nodes in that era. Era 4 historically only reported
total trigger count; this batch adds trigger bucketing for parity with
era 1/2/3 (the pre-fix column is the counterfactual the audit would have
reported under the old `classifyTrigger` routes).

| Era | Conditions (before → after) | Triggers (before → after) |
|---|---|---|
| Era 1 | 174 / 2499 (7.0%) → **167 / 2499 (6.7%)** | 48 / 11548 (0.4%) → **47 / 11548 (0.4%)** |
| Era 2 | 0 / 75 (0.0%) → **0 / 75 (0.0%)** | 0 / 408 (0.0%) → **0 / 408 (0.0%)** |
| Era 3 | 1 / 55 (1.8%) → **1 / 55 (1.8%)** | 0 / 538 (0.0%) → **0 / 538 (0.0%)** |
| Era 4 | 57 / 514 (11.1%) → **57 / 514 (11.1%)** | ~174 / 2515 (6.9%) → **76 / 2515 (3.0%)** |

The headline movement is **Era 4 triggers, −98 unbucketed nodes / −56%**
— the dominant cluster from the cross-era top-10. Era 1 conditions move
by −7 (named-counter cluster matched 7 of the 11 raw-text fragments;
the rest are "doesn't have" / "no X counters" negation cases that route
to a different scaffold and were not in scope for this batch). Era 1
triggers move by −1 (`compound_tribe_enter` is the only `*_to_gy_from_bf`
family member with an Era-1 occurrence). Era 2 / Era 3 were already
fully clean on the trigger side and are unaffected by the cross-era
batch; their condition gaps are likewise unchanged because the fixed
clusters have no Era-2/3 members.

### Reproducing the per-era audits

```
python3 scripts/era1_scaffold_audit.py
python3 scripts/era2_scaffold_audit.py
python3 scripts/era3_scaffold_audit.py
python3 scripts/era4_scaffold_audit.py
```

Each writes `data/rules/era{N}_scaffold_audit.md` and prints the
top-50 unbucketed Kinds + raw fragments to stdout.

## Reproducing this report

```
go run ./cmd/parser-coverage --out docs/parser-coverage-r41.md
```
