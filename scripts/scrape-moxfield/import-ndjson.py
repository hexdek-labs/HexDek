#!/usr/bin/env python3
"""Convert Moxfield NDJSON scrape into engine-ready .txt decklists."""

import json
import os
import re
import sys
from pathlib import Path

NDJSON_FILES = [
    Path(__file__).parent / "output" / "deck-details.ndjson",
    Path(__file__).parent / "output" / "deck-details-run2.ndjson",
]
OUTPUT_DIR = Path(__file__).parent.parent.parent / "data" / "decks" / "moxfield"

def slugify(name: str) -> str:
    s = name.lower()
    s = re.sub(r"[^a-z0-9]+", "_", s)
    s = s.strip("_")
    return s[:60]

def convert_deck(data: dict, public_id: str) -> tuple[str | None, str | None, str]:
    """Returns (deck_text, filename, skip_reason)."""
    if data.get("format") != "commander":
        return None, None, "not commander"

    boards = data.get("boards", {})
    cmdr_board = boards.get("commanders", {}).get("cards", {})
    mb_board = boards.get("mainboard", {}).get("cards", {})

    cmdr_names = []
    for entry in cmdr_board.values():
        card = entry.get("card", {})
        name = card.get("name", "")
        if name:
            cmdr_names.append(name)

    if not cmdr_names:
        return None, None, "no commander"

    total_qty = sum(v.get("quantity", 1) for v in mb_board.values())
    total_qty += sum(v.get("quantity", 1) for v in cmdr_board.values())
    if total_qty < 98:
        return None, None, f"only {total_qty} cards"

    lines = []
    for name in cmdr_names:
        lines.append(f"COMMANDER: {name}")

    for entry in sorted(mb_board.values(), key=lambda e: e.get("card", {}).get("name", "")):
        card = entry.get("card", {})
        name = card.get("name", "")
        qty = entry.get("quantity", 1)
        if name:
            lines.append(f"{qty} {name}")

    deck_text = "\n".join(lines) + "\n"

    bracket = data.get("bracket", 0) or 0
    cmdr_slug = slugify(cmdr_names[0])
    author = data.get("createdByUser", {}).get("userName", "unknown")
    author_slug = slugify(author)[:20]
    filename = f"{cmdr_slug}_b{bracket}_{author_slug}_{public_id[:8]}.txt"

    return deck_text, filename, ""

def main():
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    imported = 0
    skipped = 0
    dupes = 0
    skip_reasons = {}
    seen_ids = set()

    for ndjson_path in NDJSON_FILES:
        if not ndjson_path.exists():
            print(f"Skipping {ndjson_path} (not found)")
            continue
        print(f"Processing {ndjson_path.name}...")
        with open(ndjson_path) as f:
            for line in f:
                row = json.loads(line)
                pid = row.get("publicId", "unknown")
                if pid in seen_ids:
                    dupes += 1
                    continue
                seen_ids.add(pid)
                data = row.get("data", {})

                text, filename, reason = convert_deck(data, pid)
                if text is None:
                    skipped += 1
                    skip_reasons[reason] = skip_reasons.get(reason, 0) + 1
                    continue

                out_path = OUTPUT_DIR / filename
                out_path.write_text(text)
                imported += 1

    print(f"\nImported: {imported}")
    print(f"Skipped: {skipped}")
    print(f"Duplicates: {dupes}")
    for reason, count in sorted(skip_reasons.items(), key=lambda x: -x[1]):
        print(f"  {reason}: {count}")
    print(f"Output: {OUTPUT_DIR}")

if __name__ == "__main__":
    main()
