"""Golden-file regression tests for the MTG oracle text parser.

For each golden file in tests/golden/, we:
  1. Load the card from oracle-cards.json.
  2. Parse it with `parser.parse_card`.
  3. Compute the same signature the generator used.
  4. Assert byte-identical equality with the golden file.

If this test fails:
  - Inspect the diff between expected (golden) and actual.
  - If the parser change is INTENDED, regenerate: ``python3 tests/generate_golden.py``.
  - If the change is a regression, fix the parser / extension.

Parametrized by golden filename so each card is its own test case — one
regression = one red test, not the whole suite.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from generate_golden import compute_signature, load_cards  # noqa: E402
from parser import parse_card  # noqa: E402

GOLDEN_DIR = Path(__file__).resolve().parent / "golden"
GOLDEN_FILES = sorted(GOLDEN_DIR.glob("*.json"))


@pytest.fixture(scope="session")
def cards_by_name() -> dict[str, dict]:
    """Load the full oracle dump once per session (it's ~170MB)."""
    return load_cards()


@pytest.mark.parametrize(
    "golden_path",
    GOLDEN_FILES,
    ids=[p.stem for p in GOLDEN_FILES],
)
def test_card_signature_matches_golden(golden_path, cards_by_name):
    expected = json.loads(golden_path.read_text(encoding="utf-8"))
    card_name = expected["name"]

    card = cards_by_name.get(card_name)
    assert card is not None, (
        f"Card {card_name!r} missing from oracle-cards.json — "
        f"either regenerate the golden, or patch card_selection.py."
    )

    ast = parse_card(card)
    actual = compute_signature(ast)
    actual["group"] = expected.get("group", "misc")  # group isn't derivable from parse

    # Compare structurally: top-level counts first (biggest signal), then abilities.
    assert actual["name"] == expected["name"]
    assert actual["fully_parsed"] == expected["fully_parsed"], (
        f"{card_name}: fully_parsed flipped "
        f"({expected['fully_parsed']} -> {actual['fully_parsed']})"
    )
    assert actual["ability_count"] == expected["ability_count"], (
        f"{card_name}: ability_count changed "
        f"({expected['ability_count']} -> {actual['ability_count']})"
    )
    assert actual["parse_error_count"] == expected["parse_error_count"], (
        f"{card_name}: parse_error_count changed "
        f"({expected['parse_error_count']} -> {actual['parse_error_count']})"
    )
    assert actual["abilities"] == expected["abilities"], (
        f"{card_name}: ability signatures changed\n"
        f"  expected: {json.dumps(expected['abilities'], indent=2)}\n"
        f"  actual:   {json.dumps(actual['abilities'], indent=2)}"
    )


def test_golden_files_exist():
    """Sanity check — we should have 200 golden files."""
    assert len(GOLDEN_FILES) >= 199, (
        f"Expected ~200 golden files, found {len(GOLDEN_FILES)}. "
        f"Did someone delete fixtures?"
    )
