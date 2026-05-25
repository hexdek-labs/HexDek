#!/usr/bin/env bash
# parser-coverage.sh — re-runs the AST parser coverage audit
# (cmd/parser-coverage) and writes the report to docs/parser-coverage-r41.md
# unless --out is overridden.
#
# Usage:
#   scripts/parser-coverage.sh                       # default report path
#   scripts/parser-coverage.sh --out docs/foo.md     # custom output
#
# Requires the gitignored bulk data files:
#   data/rules/oracle-cards.json   (Scryfall, fetch via scripts/fetch-oracle.sh)
#   data/rules/ast_dataset.jsonl   (produced by cmd/hexdek-thor)
set -euo pipefail

cd "$(dirname "$0")/.."

for f in data/rules/oracle-cards.json data/rules/ast_dataset.jsonl; do
  if [[ ! -f "$f" ]]; then
    echo "parser-coverage: missing $f" >&2
    echo "  oracle-cards.json: run scripts/fetch-oracle.sh (if present) or download from Scryfall bulk data" >&2
    echo "  ast_dataset.jsonl: run cmd/hexdek-thor to regenerate" >&2
    exit 1
  fi
done

exec go run ./cmd/parser-coverage "$@"
