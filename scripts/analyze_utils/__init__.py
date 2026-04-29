"""Forensic single-game analysis for the mtgsquad engine.

"What we see vs what is reported should be eyes on anything that looks
like it could be an error. Those are the flags. Organized in priority.
If we have several of the same type of error they can be reported as
a group 'seat x, y, z, had sol ring but no CC for n turns', 'Bolas
Citadel was activated but active players life total didn't change and
there was n casts from library zone'"
    — 7174n1c, 2026-04-16

The methodology going forward is: test → data collection → analysis →
patch → repeat. This package is the "analysis" step. Aggregate winrates
from mass-sampled gauntlets are gibberish when the engine has
orthogonal bugs; the antidote is deep forensic analysis of a SINGLE
game's full event stream, flagging anything that looks anomalous.

The "bless you" metaphor: we track not just primary actions but
EXPECTED REACTIVE TRIGGERS (Rhystic Study draw, Esper Sentinel draw,
Orcish Bowmasters ping, ETB observers, cast-count observers) and
whether they fired. A cast that doesn't trigger a present Rhystic
Study is itself a bug.

Layout:
    findings.py   — Finding dataclass, severity enum
    timelines.py  — per-seat per-turn resource state extraction
    detectors.py  — Tier 1/2/3 anomaly detectors
    signatures.py — deck-specific signature checks (storm/ramp/combo/…)
    groupers.py   — collapse same-shape findings into seat/turn groups
    rendering.py  — markdown report formatter
"""

from .findings import Finding, Severity  # noqa: F401
