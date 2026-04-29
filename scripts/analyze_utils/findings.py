"""Finding dataclass + severity enum for the forensic analyzer.

Every detector returns a list of Finding objects. The reporter groups
and renders them. Groupers merge same-shape findings into seat/turn
clusters so "seat 0 turn 1" + "seat 0 turn 2" + … + "seat 3 turn 8"
becomes ONE summary line:

    Seats [0,1,2,3] had Sol Ring inert for turns [1-8].

Finding carries enough context that we can render either as terse
single-line summaries or as expandable detail blocks. event_refs is a
list of indices into the events JSONL — callers of rendering can emit
links/cross-refs if they want to.

Severity codes (for sort order in the report):
    CRITICAL — almost certainly an engine bug; fix before next gauntlet
    WARN     — suspicious, worth investigating
    INFO     — informational signal (resource waste etc.) not a bug
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import IntEnum
from typing import Any


class Severity(IntEnum):
    """Priority order — lower number = higher priority in the report.

    IntEnum (not StrEnum) so we can sort findings by severity directly.
    """
    CRITICAL = 0
    WARN = 1
    INFO = 2

    def label(self) -> str:
        return {0: "CRITICAL", 1: "WARN", 2: "INFO"}[int(self)]


@dataclass
class Finding:
    """A single anomaly the analyzer surfaced."""
    severity: Severity
    category: str
    message: str
    # Detector slug — used for grouping same-shape findings. Detectors
    # that emit one Finding per (seat, turn) should share a group_key
    # so the grouper can merge them. Detectors that emit one Finding
    # per atomic event (like a missed Rhystic Study draw) should give
    # EACH event a unique group_key if the events are truly independent.
    group_key: str = ""
    affected_seats: list[int] = field(default_factory=list)
    affected_turns: list[int] = field(default_factory=list)
    # Indices into the events JSONL that evidence this finding.
    event_refs: list[int] = field(default_factory=list)
    # Optional extra structured data the detector wants to surface
    # (e.g. {"artifact": "Sol Ring", "expected_pips": "{C}{C}"}).
    details: dict = field(default_factory=dict)

    def sort_key(self) -> tuple:
        """Canonical sort: severity first, then category, then message."""
        return (int(self.severity), self.category, self.message)
