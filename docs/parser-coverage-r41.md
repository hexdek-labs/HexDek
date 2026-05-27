# Parser Coverage Audit — R41

Cross-references every entry in `data/rules/oracle-cards.json` against the
AST corpus loaded by `internal/astload` from `data/rules/ast_dataset.jsonl`.
The AST is produced by the Python parser (`scripts/mtg_ast.py`); this
audit verifies its output is loadable and non-empty for every real card.

## Headline

| Metric | Value |
|---|---:|
| Oracle cards examined (post-dedup, non-token, non-Un) | 31963 |
| AST corpus size | 31963 |
| Astload parse warnings | 104 |
| Parse success rate (OK + OK_VANILLA) | **100.00%** |
| Failure rate (MISSING + EMPTY_AST + PARTIAL) | 0.00% |

## Classification breakdown

| Class | Count | Share |
|---|---:|---:|
| OK | 31601 | 98.87% |
| OK_VANILLA | 361 | 1.13% |
| MISSING | 0 | 0.00% |
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
| `long_text` | 1 | Teysa of the Ghost Council (PARTIAL) |

## PARTIAL parse details

- **Teysa of the Ghost Council** — parse_errors: [teysa intensifies by 1]

## Uncovered card sample (random 1, seed=42)

Random reservoir-sample of cards whose AST is missing, empty, or only partially parsed.
Each entry is a concrete scaffold target — pick one, read its oracle text,
and either add the missing parser handler or extend the existing one until
this card lands in the OK class. Re-running with the same `--sample-seed`
yields the same set, so a follow-up audit can confirm a specific card moved.

| # | Card | Class | Oracle text (truncated) |
|---:|---|---|---|
| 1 | Teysa of the Ghost Council | PARTIAL | Starting intensity 0 When Teysa enters, create a 1/1 white and black Spirit creature token with flying. Teysa intensifi… |

## Astload corpus warnings (first 20)

_See loader output; 104 warnings total._

## Reproducing this report

```
go run ./cmd/parser-coverage --out docs/parser-coverage-r41.md
```
