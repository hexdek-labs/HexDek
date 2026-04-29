"""Markdown report renderer for the forensic analyzer.

Priority-ordered top, expandable detail blocks below. The report is
designed to be scanned: the reader should be able to glance at the
CRITICAL section and immediately see "there's a Sol Ring bug, seat X,
N turns, click to expand for the full list".

Report structure (canonical):

    # Game <id> Analysis
    ## Summary             — one-liner facts
    ## CRITICAL flags      — priority-ordered, grouped
    ## WARN flags
    ## INFO flags
    ## Per-seat overview   — archetype, commander, life arc, final state
    ## Detector coverage   — which detectors ran, which fired
"""

from __future__ import annotations

from collections import defaultdict
from typing import Iterable

from .findings import Finding, Severity
from .groupers import _compress_turn_list


def render_report(
    game_id: str,
    final_state: dict,
    decks: dict,
    archetype_by_seat: dict,
    grouped_findings: list,
    coverage: list,
    total_events: int,
) -> str:
    """Return the full markdown report as a string."""
    lines: list = []

    # --- Title + summary -------------------------------------------------
    lines.append(f"# Game `{game_id}` Forensic Analysis")
    lines.append("")
    lines.append(
        "> _\"What we see vs what is reported should be eyes on anything "
        "that looks like it could be an error. Those are the flags.\"_  "
        "— 7174n1c, 2026-04-16"
    )
    lines.append("")

    winner = final_state.get("winner")
    winner_cmdr = None
    if isinstance(winner, int):
        for s in decks.get("seats", []):
            if s["seat"] == winner:
                winner_cmdr = s.get("commander_name")
                break
    end_reason = final_state.get("end_reason", "")
    total_turns = final_state.get("total_turns", 0)

    by_sev: dict = defaultdict(int)
    for f in grouped_findings:
        by_sev[f.severity] += 1

    lines.append("## Summary")
    lines.append("")
    lines.append(f"- **Winner**: seat {winner} "
                 f"(`{winner_cmdr}`)" if winner_cmdr
                 else f"- **Winner**: {winner}")
    lines.append(f"- **End reason**: {end_reason}")
    lines.append(f"- **Turns played**: {total_turns}")
    lines.append(f"- **Events captured**: {total_events}")
    lines.append(
        f"- **Findings**: "
        f"{by_sev[Severity.CRITICAL]} CRITICAL / "
        f"{by_sev[Severity.WARN]} WARN / "
        f"{by_sev[Severity.INFO]} INFO"
    )
    lines.append("")

    # --- Seats ----------------------------------------------------------
    lines.append("### Seats")
    lines.append("")
    lines.append("| Seat | Commander | Archetype | Final life | Status |")
    lines.append("| ---- | --------- | --------- | ---------- | ------ |")
    for s in final_state.get("seats", []):
        idx = s["idx"]
        cmdrs = ", ".join(s.get("commander_names", []))
        arch = archetype_by_seat.get(idx, "?")
        life = s.get("life", "?")
        if s.get("lost"):
            status = f"LOST ({s.get('loss_reason','—')})"
        else:
            status = "alive" if winner != idx else "**WINNER**"
        lines.append(f"| {idx} | {cmdrs} | {arch} | {life} | {status} |")
    lines.append("")

    # --- Findings sections ----------------------------------------------
    for sev in (Severity.CRITICAL, Severity.WARN, Severity.INFO):
        sev_findings = [f for f in grouped_findings if f.severity == sev]
        if not sev_findings:
            continue
        lines.append(f"## {sev.label()} flags")
        lines.append("")
        for idx, f in enumerate(sev_findings, 1):
            lines.append(_render_finding(idx, f))
        lines.append("")

    # --- Per-seat timeline overview -------------------------------------
    lines.append("## Per-seat overview")
    lines.append("")
    for s in final_state.get("seats", []):
        idx = s["idx"]
        cmdrs = ", ".join(s.get("commander_names", []))
        lines.append(f"### Seat {idx}: {cmdrs}")
        lines.append("")
        bf = [p["name"] for p in s.get("battlefield", [])]
        lines.append(f"- Final life: **{s.get('life')}**, "
                     f"hand={len(s.get('hand', []))}, "
                     f"library={s.get('library_count', 0)}, "
                     f"graveyard={len(s.get('graveyard', []))}")
        if bf:
            lines.append(f"- Battlefield ({len(bf)}): "
                         + ", ".join(sorted(bf)))
        cmd_dmg = s.get("commander_damage") or {}
        if cmd_dmg:
            bits = []
            for dealer, by_name in cmd_dmg.items():
                if isinstance(by_name, dict):
                    for nm, amt in by_name.items():
                        bits.append(f"{amt} from seat {dealer} ({nm})")
            if bits:
                lines.append("- Commander damage taken: "
                             + "; ".join(bits))
        cmd_tax = s.get("commander_tax") or {}
        if cmd_tax:
            lines.append("- Commander casts: "
                         + ", ".join(f"{n}×{v}"
                                     for n, v in cmd_tax.items()))
        lines.append("")

    # --- Detector coverage ----------------------------------------------
    lines.append("## Detector coverage")
    lines.append("")
    lines.append("| Detector | Findings | Error |")
    lines.append("| -------- | -------- | ----- |")
    for name, count, err in coverage:
        err_s = err or ""
        lines.append(f"| `{name}` | {count} | {err_s} |")
    lines.append("")

    lines.append("---")
    lines.append("")
    lines.append("_Generated by `scripts/analyze_single_game.py` — "
                 "file under `scripts/analyze_utils/`._")
    return "\n".join(lines) + "\n"


def _render_finding(idx: int, f: Finding) -> str:
    """One finding as a <details>-wrapped section so the markdown viewer
    can show a one-liner + expandable detail."""
    subs = f.details.get("subfindings") if f.details else None
    count = (f.details.get("finding_count") if f.details else None) or 1
    seats = (", ".join(str(s) for s in f.affected_seats)
             if f.affected_seats else "—")
    turns = (_compress_turn_list(f.affected_turns)
             if f.affected_turns else "—")
    header = f"{idx}. **[{f.category}]** {f.message}"
    body_lines = [
        "",
        f"   - Seats: `{seats}`, Turns: `{turns}`",
        f"   - group_key: `{f.group_key or '—'}`",
    ]
    if f.event_refs:
        refs = ", ".join(str(r) for r in f.event_refs[:10])
        more = "" if len(f.event_refs) <= 10 else \
            f" (+{len(f.event_refs)-10} more)"
        body_lines.append(f"   - Event seqs: `{refs}`{more}")
    if f.details:
        # Filter out keys we already rendered.
        filtered = {k: v for k, v in f.details.items()
                    if k not in ("subfindings", "finding_count")}
        if filtered:
            body_lines.append("")
            body_lines.append("   <details><summary>Details</summary>")
            body_lines.append("")
            for k, v in filtered.items():
                body_lines.append(f"   - `{k}`: `{v}`")
            body_lines.append("")
            body_lines.append("   </details>")
    if subs and count > 1:
        body_lines.append("")
        body_lines.append("   <details><summary>Subfindings "
                         f"({count})</summary>")
        body_lines.append("")
        for m in subs[:30]:
            body_lines.append(f"   - {m}")
        if len(subs) > 30:
            body_lines.append(f"   - … and {len(subs)-30} more")
        body_lines.append("")
        body_lines.append("   </details>")
    return header + "\n" + "\n".join(body_lines)
