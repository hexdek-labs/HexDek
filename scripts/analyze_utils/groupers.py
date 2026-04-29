"""Group same-shape findings into seat/turn clusters.

"If we have several of the same type of error they can be reported
as a group 'seat x, y, z, had sol ring but no CC for n turns'"
    — 7174n1c

Strategy: group by (severity, category, group_key). Within each group,
union the affected_seats and affected_turns, sum event_refs. Preserve
the CRITICAL-first ordering by sort_key.
"""

from __future__ import annotations

from collections import defaultdict

from .findings import Finding, Severity


def _compress_turn_list(turns: list) -> str:
    """Render a list of turn numbers as a compressed range string.

    Example: [1, 2, 3, 5, 7, 8, 9] → "1-3, 5, 7-9".
    """
    if not turns:
        return ""
    s = sorted(set(int(t) for t in turns if t is not None))
    if not s:
        return ""
    out = []
    start = prev = s[0]
    for n in s[1:]:
        if n == prev + 1:
            prev = n
            continue
        if start == prev:
            out.append(f"{start}")
        else:
            out.append(f"{start}-{prev}")
        start = prev = n
    if start == prev:
        out.append(f"{start}")
    else:
        out.append(f"{start}-{prev}")
    return ", ".join(out)


def _compress_seat_list(seats: list) -> str:
    seats = sorted(set(int(s) for s in seats if s is not None))
    return "[" + ",".join(str(s) for s in seats) + "]"


def group_findings(findings: list) -> list:
    """Collapse findings that share (severity, category, group_key).

    Returns a NEW list of Finding objects. Each grouped Finding's
    message is rewritten to use seat/turn ranges. The original per-
    item messages are preserved inside details["subfindings"] as a
    list of raw messages, so the renderer can produce an expandable
    detail block.
    """
    buckets: dict = defaultdict(list)
    for f in findings:
        key = (int(f.severity), f.category, f.group_key or f.message)
        buckets[key].append(f)

    grouped: list = []
    for key, items in buckets.items():
        if len(items) == 1:
            grouped.append(items[0])
            continue
        severity, category, _ = key
        union_seats: set = set()
        union_turns: set = set()
        union_refs: list = []
        subfindings: list = []
        merged_details: dict = {}
        for f in items:
            union_seats.update(f.affected_seats or [])
            union_turns.update(f.affected_turns or [])
            union_refs.extend(f.event_refs or [])
            subfindings.append(f.message)
            # Shallow-merge details, preserving last-write for scalar
            # fields and union for list/set fields.
            for k, v in (f.details or {}).items():
                if k not in merged_details:
                    merged_details[k] = v
        seat_frag = _compress_seat_list(union_seats) if union_seats else "—"
        turn_frag = (_compress_turn_list(union_turns)
                     if union_turns else "—")
        msg = (f"{category} — seats {seat_frag}, turns [{turn_frag}] "
               f"({len(items)} findings)")
        merged_details["subfindings"] = subfindings
        merged_details["finding_count"] = len(items)
        grouped.append(Finding(
            severity=Severity(severity),
            category=category,
            message=msg,
            group_key=items[0].group_key,
            affected_seats=sorted(union_seats),
            affected_turns=sorted(union_turns),
            event_refs=union_refs[:50],
            details=merged_details,
        ))
    # Canonical sort: severity, then category, then message.
    grouped.sort(key=lambda f: f.sort_key())
    return grouped
