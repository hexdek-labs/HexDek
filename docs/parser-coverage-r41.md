# Parser Coverage Audit — R41

Cross-references every entry in `data/rules/oracle-cards.json` against the
AST corpus loaded by `internal/astload` from `data/rules/ast_dataset.jsonl`.
The AST is produced by the Python parser (`scripts/mtg_ast.py`); this
audit verifies its output is loadable and non-empty for every real card.

## Headline

| Metric | Value |
|---|---:|
| Oracle cards examined (post-dedup, non-token, non-Un) | 31979 |
| AST corpus size | 31963 |
| Astload parse warnings | 104 |
| Parse success rate (OK + OK_VANILLA) | **99.95%** |
| Failure rate (MISSING + EMPTY_AST + PARTIAL) | 0.05% |

## Classification breakdown

| Class | Count | Share |
|---|---:|---:|
| OK | 31601 | 98.82% |
| OK_VANILLA | 361 | 1.13% |
| MISSING | 16 | 0.05% |
| EMPTY_AST | 0 | 0.00% |
| PARTIAL | 1 | 0.00% |

### What each class means

- **OK** — card resolves through the astload Corpus and has at least one parsed ability.
- **OK_VANILLA** — no oracle text (basic land or vanilla creature). Empty AST is correct.
- **MISSING** — oracle card has no entry in the AST dataset. Parser pipeline never ingested it.
- **EMPTY_AST** — AST entry exists, card has oracle text, but the parser produced zero abilities. Real parser failure.
- **PARTIAL** — `fully_parsed=false` or non-empty `parse_errors`. Parser partially failed.

## Top 10 failure patterns

| Pattern | Count | Sample cards |
|---|---:|---|
| `long_text` | 7 | Tempest Efreet (MISSING); Timmerian Fiends (MISSING); Imprison (MISSING); Teysa of the Ghost Council (PARTIAL); Amulet of Quoz (MISSING) |
| `other` | 6 | Demonic Attorney (MISSING); Darkpact (MISSING); Invoke Prejudice (MISSING); Jeweled Bird (MISSING); Contract from Below (MISSING) |
| `keyword_only` | 4 | Stone-Throwing Devils (MISSING); Pradesh Gypsies (MISSING); Crusade (MISSING); Cleanse (MISSING) |

## PARTIAL parse details

- **Teysa of the Ghost Council** — parse_errors: [teysa intensifies by 1]

## Uncovered card sample (random 17, seed=42)

Random reservoir-sample of cards whose AST is missing, empty, or only partially parsed.
Each entry is a concrete scaffold target — pick one, read its oracle text,
and either add the missing parser handler or extend the existing one until
this card lands in the OK class. Re-running with the same `--sample-seed`
yields the same set, so a follow-up audit can confirm a specific card moved.

| # | Card | Class | Oracle text (truncated) |
|---:|---|---|---|
| 1 | Amulet of Quoz | MISSING | Remove this card from your deck before playing if you're not playing for ante. {T}, Sacrifice this artifact: Target opp… |
| 2 | Bronze Tablet | MISSING | Remove this card from your deck before playing if you're not playing for ante. This artifact enters tapped. {4}, {T}: E… |
| 3 | Cleanse | MISSING | Destroy all black creatures. |
| 4 | Contract from Below | MISSING | Remove this card from your deck before playing if you're not playing for ante. Discard your hand, ante the top card of … |
| 5 | Crusade | MISSING | White creatures get +1/+1. |
| 6 | Darkpact | MISSING | Remove this card from your deck before playing if you're not playing for ante. You own target card in the ante. Exchang… |
| 7 | Demonic Attorney | MISSING | Remove this card from your deck before playing if you're not playing for ante. Each player antes the top card of their … |
| 8 | Imprison | MISSING | Enchant creature Whenever a player activates an ability of enchanted creature with {T} in its activation cost that isn'… |
| 9 | Invoke Prejudice | MISSING | Whenever an opponent casts a creature spell that doesn't share a color with a creature you control, counter that spell … |
| 10 | Jeweled Bird | MISSING | Remove this card from your deck before playing if you're not playing for ante. {T}: Ante this artifact. If you do, put … |
| 11 | Jihad | MISSING | As this enchantment enters, choose a color and an opponent. White creatures get +2/+1 as long as the chosen player cont… |
| 12 | Pradesh Gypsies | MISSING | {1}{G}, {T}: Target creature gets -2/-0 until end of turn. |
| 13 | Rebirth | MISSING | Remove this card from your deck before playing if you're not playing for ante. Each player may ante the top card of the… |
| 14 | Stone-Throwing Devils | MISSING | First strike |
| 15 | Tempest Efreet | MISSING | Remove this card from your deck before playing if you're not playing for ante. {T}, Sacrifice this creature: Target opp… |
| 16 | Teysa of the Ghost Council | PARTIAL | Starting intensity 0 When Teysa enters, create a 1/1 white and black Spirit creature token with flying. Teysa intensifi… |
| 17 | Timmerian Fiends | MISSING | Remove this card from your deck before playing if you're not playing for ante. {B}{B}{B}, Sacrifice this creature: The … |

## Astload corpus warnings (first 20)

_See loader output; 104 warnings total._

## Reproducing this report

```
go run ./cmd/parser-coverage --out docs/parser-coverage-r41.md
```
