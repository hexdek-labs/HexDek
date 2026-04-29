#!/usr/bin/env python3
"""Single-game forensic analysis harness for mtgsquad.

OWNER'S METHODOLOGY (baked into this tool's reason-for-being):

    "What we see vs what is reported should be eyes on anything
    that looks like it could be an error. Those are the flags.
    Organized in priority. If we have several of the same type of
    error they can be reported as a group 'seat x, y, z, had sol
    ring but no CC for n turns', 'Bolas Citadel was activated but
    active players life total didn't change and there was n casts
    from library zone'"
        — 7174n1c, 2026-04-16

Mass-sampling gauntlets produce confident-looking winrates that are
gibberish when the engine has orthogonal bugs. The antidote is deep
forensic analysis of a SINGLE game's full event stream.

Usage:
    python3 scripts/analyze_single_game.py \\
        --game-id <id> \\
        --events path/to/events.jsonl \\
        --final-state path/to/state.json \\
        --decks path/to/decks.json \\
        --output path/to/report.md

Companion flags on gauntlet_poker.py / gauntlet.py dump the three
inputs:

    python3 scripts/gauntlet_poker.py --games 1 --seed 42 \\
        --emit-events   dump/events.jsonl \\
        --emit-final-state dump/state.json \\
        --emit-decks    dump/decks.json \\
        --game-id       investigation-001

Detector coverage: Tier 1 (10+ CRITICAL engine-bug detectors),
Tier 2 (5 WARN deck-signature detectors), Tier 3 (4 INFO resource-
waste detectors). See scripts/analyze_utils/detectors.py for the
full registry.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from analyze_utils import Finding, Severity  # noqa: E402,F401
from analyze_utils.detectors import run_all_detectors  # noqa: E402
from analyze_utils.groupers import group_findings  # noqa: E402
from analyze_utils.rendering import render_report  # noqa: E402
from analyze_utils.signatures import classify_all  # noqa: E402
from analyze_utils.timelines import build_timeline  # noqa: E402


def _load_events(path: str) -> list:
    out: list = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            out.append(json.loads(line))
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game-id", type=str, default=None,
                    help="label for the report header; falls back to "
                         "the game_id inside the events file")
    ap.add_argument("--events", type=str, required=True,
                    help="JSONL event-stream dump")
    ap.add_argument("--final-state", type=str, required=True,
                    help="JSON final Game state dump")
    ap.add_argument("--decks", type=str, required=True,
                    help="JSON deck metadata dump")
    ap.add_argument("--output", type=str, required=True,
                    help="output markdown path")
    ap.add_argument("--verbose", action="store_true",
                    help="print coverage + finding counts to stderr")
    args = ap.parse_args()

    t0 = time.time()

    events = _load_events(args.events)
    final_state = json.loads(Path(args.final_state).read_text())
    decks = json.loads(Path(args.decks).read_text())

    # Derive game_id from CLI or first event.
    game_id = args.game_id
    if not game_id:
        for e in events:
            if e.get("game_id"):
                game_id = e["game_id"]
                break
    if not game_id:
        game_id = "<unknown>"

    num_seats = max(
        len(final_state.get("seats", [])),
        len(decks.get("seats", [])),
        1,
    )

    # Walk events → timeline → detectors → group → render.
    tl = build_timeline(events, num_seats)
    findings, coverage = run_all_detectors(events, final_state, decks, tl)
    archetype_by_seat = classify_all(decks)

    grouped = group_findings(findings)

    md = render_report(
        game_id=game_id,
        final_state=final_state,
        decks=decks,
        archetype_by_seat=archetype_by_seat,
        grouped_findings=grouped,
        coverage=coverage,
        total_events=len(events),
    )

    Path(args.output).parent.mkdir(parents=True, exist_ok=True)
    Path(args.output).write_text(md)

    dt = time.time() - t0

    if args.verbose:
        by_sev = {Severity.CRITICAL: 0, Severity.WARN: 0,
                  Severity.INFO: 0}
        for f in grouped:
            by_sev[f.severity] += 1
        print(f"[analyze] events={len(events)}  turns="
              f"{final_state.get('total_turns', 0)}  findings="
              f"{len(grouped)}  "
              f"(CRIT={by_sev[Severity.CRITICAL]}, "
              f"WARN={by_sev[Severity.WARN]}, "
              f"INFO={by_sev[Severity.INFO]})  "
              f"elapsed={dt:.2f}s",
              file=sys.stderr)
        fired = sum(1 for (_, n, _) in coverage if n > 0)
        errors = sum(1 for (_, _, err) in coverage if err)
        print(f"[analyze] detector coverage: "
              f"{fired}/{len(coverage)} fired, "
              f"{errors} errored",
              file=sys.stderr)
    print(f"Wrote {args.output}  "
          f"({len(grouped)} findings, "
          f"{sum(1 for f in grouped if f.severity == Severity.CRITICAL)} "
          f"CRITICAL) in {dt:.2f}s")
    return 0


if __name__ == "__main__":
    sys.exit(main())
