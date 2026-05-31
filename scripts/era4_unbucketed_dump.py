#!/usr/bin/env python3
"""Dump every Era 4 unbucketed condition raw text + Kind."""
from __future__ import annotations
import json, sys
from collections import Counter, defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import era4_scaffold_audit as audit

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"

text_counter: Counter[str] = Counter()
examples: dict[str, list[str]] = defaultdict(list)
kind_counter: Counter[str] = Counter()

with DATASET.open("r", encoding="utf-8") as f:
    for line in f:
        if not line.strip():
            continue
        row = json.loads(line)
        if audit.classify_era(row.get("oracle_text", ""), row.get("type_line", "")) != 4:
            continue
        ast = row.get("ast")
        if not ast:
            continue
        conds, _trigs = [], []
        audit.walk(ast, conds, _trigs)
        name = row.get("name", "?")
        for c in conds:
            k = (c.get("kind") or "").lower()
            if k in audit.BUCKETED_KINDS:
                continue
            if k in audit.RAW_KINDS:
                args = c.get("args") or []
                raw = args[0] if args and isinstance(args[0], str) else ""
                if raw and audit.match_raw(raw):
                    continue
                key = (raw[:300] or "<empty>").lower()
                text_counter[key] += 1
                kind_counter[k] += 1
                if len(examples[key]) < 5:
                    examples[key].append(name)
            else:
                key = f"<KIND:{k}>"
                text_counter[key] += 1
                kind_counter[k] += 1
                if len(examples[key]) < 5:
                    examples[key].append(name)

print(f"Total unbucketed: {sum(text_counter.values())}")
print(f"Kind histogram: {dict(kind_counter.most_common())}")
print()
for txt, n in text_counter.most_common():
    ex = ", ".join(examples[txt][:3])
    print(f"× {n}: {txt!r}  (e.g. {ex})")
