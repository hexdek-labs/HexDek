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

## Reproducing this report

```
go run ./cmd/parser-coverage --out docs/parser-coverage-r41.md
```
