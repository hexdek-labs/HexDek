# Parser Coverage Audit — R41

Cross-references every entry in `data/rules/oracle-cards.json` against the
AST corpus loaded by `internal/astload` from `data/rules/ast_dataset.jsonl`.
The AST is produced by the Python parser (`scripts/mtg_ast.py`); this
audit verifies its output is loadable and non-empty for every real card.

## Headline

| Metric | Value |
|---|---:|
| Oracle cards examined (post-dedup, non-token, non-Un) | 35708 |
| AST corpus size | 31963 |
| Astload parse warnings | 104 |
| Parse success rate (OK + OK_VANILLA) | **89.48%** |
| Failure rate (MISSING + EMPTY_AST + PARTIAL) | 10.52% |

## Classification breakdown

| Class | Count | Share |
|---|---:|---:|
| OK | 31601 | 88.50% |
| OK_VANILLA | 349 | 0.98% |
| MISSING | 3745 | 10.49% |
| EMPTY_AST | 12 | 0.03% |
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
| `other` | 2750 | Brightglass Gearhulk // Brightglass Gearhulk (MISSING); Phyrexian Broodstar (MISSING); Gigantosaurus // Gigantosaurus (MISSING); Gleeful Demolition // Gleeful Demolition (MISSING); Tatsunari, Toad Rider // Tatsunari, Toad Rider (MISSING) |
| `long_text` | 614 | Foraging Squirrels // Foraging Squirrels (cont'd) (MISSING); Strixhaven (MISSING); Jasconian Isle (MISSING); Hadran, Naya Sunseeder (MISSING); Dark Wings Bring Your Downfall (MISSING) |
| `keyword_only` | 257 | Angel // Demon (MISSING); Tundra (EMPTY_AST); Titania (MISSING); Heavily Armored (MISSING); Coalition Corps (MISSING) |
| `choose_one_modal` | 61 | The Great Aerie (MISSING); Seek Bolas's Counsel (MISSING); Guild Pact (MISSING); Yet Another Night in Vegas (MISSING); Dance, Pathetic Marionette (MISSING) |
| `replacement_effect` | 58 | Command Mine (MISSING); Selesnya Loft Gardens (MISSING); Power Level Analyzer (MISSING); Oozeavite (MISSING); Plots That Span Centuries (MISSING) |
| `double_faced_or_meld` | 11 | Incubation Triformer (MISSING); That's No Moonmist (MISSING); Grimlock, Dinobot Leader // Grimlock, Ferocious King (MISSING); Phyrexian Adapter (MISSING); Day // Night (MISSING) |
| `saga_or_class` | 6 | New Argive (MISSING); Coal Hill School (MISSING); The Legend of Arena (MISSING); War of the Spark (MISSING); Your Favorite Missing Character (MISSING) |
| `adventure` | 1 | Adventurer Beguiler (MISSING) |

## PARTIAL parse details

- **Teysa of the Ghost Council** — parse_errors: [teysa intensifies by 1]

## Astload corpus warnings (first 20)

_See loader output; 104 warnings total._

## Reproducing this report

```
go run ./cmd/parser-coverage --out docs/parser-coverage-r41.md
```
