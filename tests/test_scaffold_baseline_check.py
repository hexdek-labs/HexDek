"""Unit tests for the scaffold-coverage regression check.

Pure-function tests over the parsing / comparison / (de)serialisation
helpers in ``scripts/scaffold_baseline_check.py``. These tests do NOT
run the era audits — they exercise the regression-detection logic with
synthetic inputs so they pass even without the gitignored
``data/rules/ast_dataset.jsonl``.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import scaffold_baseline_check as sbc  # noqa: E402


# ---------------------------------------------------------------------------
# parse_audit_headlines
# ---------------------------------------------------------------------------


SAMPLE_AUDIT_STDOUT = """\
# Era 1 (1993-2014) Scaffold-Gap Audit

- Total cards in dataset: **31963**

- Era distribution: era1=26932, era2=537, era3=798, era4=3696

- Era 1 cards: **26932**

- Era 1 Condition nodes: **2499** (bucketed 2470, unbucketed 29, 1.2% gap)

- Era 1 Trigger nodes: **11548** (bucketed 11548, unbucketed 0, 0.0% gap)
"""


def test_parse_audit_headlines_extracts_both_kinds():
    stats = sbc.parse_audit_headlines(SAMPLE_AUDIT_STDOUT)
    assert (1, "conditions") in stats
    assert (1, "triggers") in stats
    cond = stats[(1, "conditions")]
    assert cond.total == 2499
    assert cond.unbucketed == 29
    assert cond.gap_pct == pytest.approx(1.2)
    trig = stats[(1, "triggers")]
    assert trig.total == 11548
    assert trig.unbucketed == 0
    assert trig.gap_pct == pytest.approx(0.0)


def test_parse_audit_headlines_handles_concatenated_eras():
    combined = SAMPLE_AUDIT_STDOUT + """
- Era 4 Condition nodes: **514** (bucketed 505, unbucketed 9, 1.8% gap)
- Era 4 Trigger nodes: **2515** (bucketed 2515, unbucketed 0, 0.0% gap)
"""
    stats = sbc.parse_audit_headlines(combined)
    assert len(stats) == 4
    assert stats[(4, "conditions")].gap_pct == pytest.approx(1.8)
    assert stats[(4, "triggers")].gap_pct == pytest.approx(0.0)


def test_parse_audit_headlines_returns_empty_on_garbage():
    assert sbc.parse_audit_headlines("nothing relevant here") == {}
    assert sbc.parse_audit_headlines("") == {}


# ---------------------------------------------------------------------------
# check_regression
# ---------------------------------------------------------------------------


def _stats(era_kind_to_gap: dict[tuple[int, str], float]) -> dict[tuple[int, str], sbc.EraStats]:
    """Build a stats dict from a {(era, kind): gap_pct} shorthand."""
    return {
        (e, k): sbc.EraStats(total=1000, unbucketed=int(g * 10), gap_pct=g)
        for (e, k), g in era_kind_to_gap.items()
    }


def test_check_regression_no_change():
    base = _stats({(1, "conditions"): 1.2, (1, "triggers"): 0.0})
    cur = _stats({(1, "conditions"): 1.2, (1, "triggers"): 0.0})
    assert sbc.check_regression(cur, base) == []


def test_check_regression_improvement_is_not_a_regression():
    base = _stats({(1, "conditions"): 1.2})
    cur = _stats({(1, "conditions"): 0.5})  # 0.7pp better
    assert sbc.check_regression(cur, base) == []


def test_check_regression_within_tolerance():
    """A delta of exactly the tolerance is NOT a regression."""
    base = _stats({(1, "conditions"): 1.2})
    cur = _stats({(1, "conditions"): 1.7})  # +0.5pp, equals default tolerance
    assert sbc.check_regression(cur, base, tolerance_pct=0.5) == []


def test_check_regression_just_over_tolerance_fires():
    """A delta strictly greater than tolerance IS a regression."""
    base = _stats({(1, "conditions"): 1.2})
    cur = _stats({(1, "conditions"): 1.8})  # +0.6pp, exceeds 0.5
    regressions = sbc.check_regression(cur, base, tolerance_pct=0.5)
    assert len(regressions) == 1
    r = regressions[0]
    assert r.era == 1
    assert r.kind == "conditions"
    assert r.baseline_pct == pytest.approx(1.2)
    assert r.current_pct == pytest.approx(1.8)
    assert r.delta_pct == pytest.approx(0.6)


def test_check_regression_custom_tolerance():
    base = _stats({(4, "triggers"): 0.0})
    cur = _stats({(4, "triggers"): 0.3})
    # Default 0.5 — should pass.
    assert sbc.check_regression(cur, base, tolerance_pct=0.5) == []
    # Tighter 0.2 — should fire.
    assert len(sbc.check_regression(cur, base, tolerance_pct=0.2)) == 1


def test_check_regression_missing_era_in_current_is_regression():
    """If the current run lost an era×kind from baseline, that's a failure."""
    base = _stats({(1, "conditions"): 1.2, (2, "triggers"): 0.0})
    cur = _stats({(1, "conditions"): 1.2})  # missing (2, "triggers")
    regressions = sbc.check_regression(cur, base)
    assert len(regressions) == 1
    r = regressions[0]
    assert (r.era, r.kind) == (2, "triggers")
    assert r.delta_pct == float("inf")


def test_check_regression_extra_era_in_current_is_silent():
    """A NEW era in current (not in baseline) is silently ignored."""
    base = _stats({(1, "conditions"): 1.2})
    cur = _stats({(1, "conditions"): 1.2, (5, "conditions"): 4.2})  # hypothetical era 5
    assert sbc.check_regression(cur, base) == []


def test_check_regression_multiple_simultaneous():
    base = _stats({(1, "conditions"): 1.2, (1, "triggers"): 0.0, (4, "conditions"): 1.8})
    cur = _stats({(1, "conditions"): 2.5, (1, "triggers"): 1.0, (4, "conditions"): 1.8})
    regressions = sbc.check_regression(cur, base, tolerance_pct=0.5)
    flagged = {(r.era, r.kind) for r in regressions}
    assert flagged == {(1, "conditions"), (1, "triggers")}


# ---------------------------------------------------------------------------
# serialise / deserialise round-trip
# ---------------------------------------------------------------------------


def test_baseline_serialisation_roundtrip():
    stats = _stats({
        (1, "conditions"): 1.2,
        (1, "triggers"): 0.0,
        (4, "conditions"): 1.8,
        (4, "triggers"): 0.0,
    })
    payload = sbc.serialise_baseline(stats, tolerance_pct=0.5)
    # JSON-friendly shape.
    assert payload["regression_tolerance_pct"] == 0.5
    assert "1" in payload["eras"] and "4" in payload["eras"]
    assert payload["eras"]["1"]["conditions"]["gap_pct"] == pytest.approx(1.2)
    # Round-trip preserves shape.
    recovered, tol = sbc.deserialise_baseline(payload)
    assert tol == 0.5
    assert recovered == stats


def test_deserialise_baseline_uses_default_tolerance():
    """Missing tolerance field falls back to DEFAULT_TOLERANCE_PCT."""
    raw = {
        "eras": {
            "1": {"conditions": {"total": 100, "unbucketed": 1, "gap_pct": 1.0}},
        }
    }
    stats, tol = sbc.deserialise_baseline(raw)
    assert tol == sbc.DEFAULT_TOLERANCE_PCT
    assert stats[(1, "conditions")].gap_pct == pytest.approx(1.0)


def test_write_baseline_atomic_with_tmp(tmp_path: Path):
    """write_baseline should produce a valid JSON file the loader can read."""
    stats = _stats({(2, "conditions"): 0.0, (2, "triggers"): 0.0})
    target = tmp_path / "test-baseline.json"
    sbc.write_baseline(target, stats, tolerance_pct=0.3)
    raw = json.loads(target.read_text())
    assert raw["regression_tolerance_pct"] == 0.3
    assert raw["eras"]["2"]["conditions"]["unbucketed"] == 0


# ---------------------------------------------------------------------------
# render_diff — visual smoke test
# ---------------------------------------------------------------------------


def test_render_diff_shows_per_era_lines():
    base = _stats({(1, "conditions"): 1.2, (1, "triggers"): 0.0})
    cur = _stats({(1, "conditions"): 1.5, (1, "triggers"): 0.0})
    out = sbc.render_diff(cur, base)
    assert "1   conditions" in out
    assert "1   triggers" in out
    assert "1.20%" in out and "1.50%" in out
    assert "+0.30%" in out


def test_render_diff_handles_missing_baseline_entry():
    base = _stats({(1, "conditions"): 1.2})
    cur = _stats({(1, "conditions"): 1.2, (4, "triggers"): 0.5})  # new in current
    out = sbc.render_diff(cur, base)
    # Era 4 triggers should render with — for baseline.
    assert "4   triggers" in out


def test_render_diff_handles_missing_current_entry():
    base = _stats({(1, "conditions"): 1.2, (4, "triggers"): 0.5})
    cur = _stats({(1, "conditions"): 1.2})  # missing era 4
    out = sbc.render_diff(cur, base)
    assert "4   triggers" in out
    # Current value should render as a dash placeholder.
    assert "—" in out


# ---------------------------------------------------------------------------
# Committed baseline file sanity — guards against accidental corruption.
# ---------------------------------------------------------------------------


def test_committed_baseline_parses_and_covers_all_eras():
    """The data/rules/scaffold-baseline.json file ships with 4 eras × 2 kinds."""
    stats, tol = sbc.load_baseline(sbc.BASELINE_PATH)
    expected_keys = {(e, k) for e in (1, 2, 3, 4) for k in ("conditions", "triggers")}
    assert set(stats.keys()) == expected_keys
    # Tolerance must be a positive float.
    assert tol > 0
    # Every gap_pct is in [0, 100].
    for s in stats.values():
        assert 0 <= s.gap_pct <= 100
        assert s.total > 0
        assert 0 <= s.unbucketed <= s.total
