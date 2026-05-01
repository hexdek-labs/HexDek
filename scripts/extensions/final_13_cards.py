#!/usr/bin/env python3
"""Final 13 cards — grammar rules for the last 14 unparsed abilities.

Groups:
  1. Forced-attack: "X attacks each combat if able"
  2. P/T = expression: "X's power and toughness are each equal to..."
  3. Entry modals: "as X enters, choose/sacrifice..."
  4. Monstrosity trigger: "when X becomes monstrous, ..."
  5. Composite trigger: "X or another Y enters, ..."
  6. Novel triggers: "search your library", "card is put into zone"
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mtg_ast import Modification, Static  # noqa: E402

_SELF = r"(?:~|this creature|this permanent)"
_NAMED = r"[a-z][a-z',\- ]+"


# ---------------------------------------------------------------------------
# Group 1: Forced-attack — "<creature> attacks each combat if able"
# ---------------------------------------------------------------------------

_FORCED_ATTACK = re.compile(
    rf"^(?:other )?(?:({_NAMED})|({_SELF})|([a-z]+ creatures? you control))"
    r"\s+attacks?\s+(?:(?:that player|each opponent)\s+)?(?:each|this)\s+combat\s+if able$",
    re.I,
)


def _build_forced_attack(m: re.Match, raw: str) -> Optional[Static]:
    subject = (m.group(1) or m.group(2) or m.group(3) or "").strip()
    return Static(
        modification=Modification(kind="forced_attack", args=(subject,)),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Group 2: P/T = expression — "X's power [and toughness] [is|are] equal to..."
# ---------------------------------------------------------------------------

_PT_EQUAL = re.compile(
    rf"^({_NAMED})'s\s+power(?:\s+and\s+toughness)?(?:\s+(?:is|are))"
    r"(?:\s+each)?\s+equal\s+to\s+(.+)$",
    re.I,
)


def _build_pt_equal(m: re.Match, raw: str) -> Optional[Static]:
    expression = m.group(2).strip()
    return Static(
        modification=Modification(kind="variable_pt", args=(expression,)),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Group 3: Entry modals — "as X enters, choose/sacrifice..."
# ---------------------------------------------------------------------------

_AS_ENTERS = re.compile(
    rf"^as\s+(?:{_NAMED}|{_SELF})\s+enters,\s+(.+)$",
    re.I,
)


def _build_as_enters(m: re.Match, raw: str) -> Optional[Static]:
    body = m.group(1).strip()
    return Static(
        modification=Modification(kind="as_enters", args=(body,)),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Group 4: Monstrosity trigger — "when X becomes monstrous, <effect>"
# ---------------------------------------------------------------------------

_BECOMES_MONSTROUS = re.compile(
    rf"^when\s+(?:{_NAMED}|{_SELF})\s+becomes\s+monstrous\b",
    re.I,
)

# ---------------------------------------------------------------------------
# Group 5: Composite trigger — "X or another Y enters, <effect>"
# ---------------------------------------------------------------------------

_OR_ANOTHER_ENTERS = re.compile(
    rf"^whenever\s+(?:{_NAMED}|{_SELF})\s+or\s+another\s+([a-z]+)\s+"
    r"(?:you control\s+)?enters\b",
    re.I,
)

# ---------------------------------------------------------------------------
# Group 6a: Search library trigger — "whenever you search your library, <effect>"
# ---------------------------------------------------------------------------

_SEARCH_LIBRARY = re.compile(
    r"^whenever\s+you\s+search\s+your\s+library\b",
    re.I,
)

# ---------------------------------------------------------------------------
# Group 6b: Card put into zone — "whenever a X card is put into your Y from Z"
# ---------------------------------------------------------------------------

_CARD_PUT_INTO = re.compile(
    r"^whenever\s+a\s+([a-z]+)\s+card\s+is\s+put\s+into\s+your\s+(\w+)"
    r"\s+from\s+your\s+(\w+)\b",
    re.I,
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, object]] = [
    (_FORCED_ATTACK, _build_forced_attack),
    (_PT_EQUAL, _build_pt_equal),
    (_AS_ENTERS, _build_as_enters),
]


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    (_BECOMES_MONSTROUS, "becomes_monstrous", "self"),
    (_OR_ANOTHER_ENTERS, "etb_or_another", "self"),
    (_SEARCH_LIBRARY, "search_library", "self"),
    (_CARD_PUT_INTO, "card_put_into_zone", "self"),
]
