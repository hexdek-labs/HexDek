"""Whole-corpus coverage test.

Parses every real card in oracle-cards.json and asserts the GREEN count is
still at 31,943. If a parser/extension change drops coverage, this fails
immediately — no more silently-regressed cards sitting around for weeks.

GREEN  = parse_card returned fully_parsed=True
PARTIAL= some abilities parsed, some left as raw errors
UNPARSED= zero abilities parsed

This test is slow (~60-90s for 31k cards). Mark it with the `slow` marker so
devs can skip it during tight loops:
    python3 -m pytest tests/ -m "not slow"
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from parser import parse_card, is_real_card  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
ORACLE = ROOT / "data" / "rules" / "oracle-cards.json"

EXPECTED_GREEN = 31_944
EXPECTED_REAL = 31_965


@pytest.mark.slow
def test_full_corpus_100_percent_green():
    """All 31,943 real cards must parse GREEN (fully_parsed=True).

    The "real card" count is actually 31,965 — 22 cards remain PARTIAL
    (see ``data/rules/parser_coverage.md``). We assert the GREEN count is
    the tracked baseline so any extension-level regression surfaces as a
    test failure rather than a silent coverage drop.
    """
    with open(ORACLE, encoding="utf-8") as f:
        all_cards = json.load(f)

    real = [c for c in all_cards if is_real_card(c)]
    assert len(real) == EXPECTED_REAL, (
        f"Real-card count changed: expected {EXPECTED_REAL}, got {len(real)}. "
        f"If oracle-cards.json was updated, bump EXPECTED_REAL / EXPECTED_GREEN "
        f"to match the new corpus."
    )

    green = 0
    partial: list[str] = []
    unparsed: list[str] = []
    exceptions: list[tuple[str, str]] = []

    for c in real:
        try:
            ast = parse_card(c)
        except Exception as e:  # pragma: no cover - defensive
            exceptions.append((c.get("name", "?"), repr(e)))
            continue
        if ast.fully_parsed:
            green += 1
        elif ast.abilities:
            partial.append(ast.name)
        else:
            unparsed.append(ast.name)

    # Emit a compact failure summary (pytest -v will show this).
    if exceptions:
        sample = "\n  ".join(f"{n}: {e}" for n, e in exceptions[:10])
        pytest.fail(
            f"{len(exceptions)} cards raised during parse (first 10):\n  {sample}"
        )

    if green != EXPECTED_GREEN:
        # Show up to 20 regressed cards by name so the regression is actionable.
        regressions = (partial + unparsed)[:20]
        pytest.fail(
            f"Parser coverage regressed: "
            f"expected green={EXPECTED_GREEN}, got {green} "
            f"(partial={len(partial)}, unparsed={len(unparsed)}).\n"
            f"First regressed cards: {regressions}"
        )


@pytest.mark.slow
def test_full_corpus_no_exceptions():
    """Defensive: no card in the corpus should raise during parsing."""
    with open(ORACLE, encoding="utf-8") as f:
        all_cards = json.load(f)

    real = [c for c in all_cards if is_real_card(c)]
    bad: list[tuple[str, str]] = []
    for c in real:
        try:
            parse_card(c)
        except Exception as e:
            bad.append((c.get("name", "?"), repr(e)))
    assert not bad, (
        f"{len(bad)} cards raised during parse. First 5: {bad[:5]}"
    )
