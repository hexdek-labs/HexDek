#!/usr/bin/env python3
"""Scaffold-coverage regression check.

Runs each of the four era scaffold audits, captures the trigger and
condition gap percentages, and compares them against a stored baseline
in ``data/rules/scaffold-baseline.json``. Exits non-zero if any era
regresses by more than ``regression_tolerance_pct`` (default 0.5).

Usage::

    # Run audits, compare against baseline, exit 1 on regression.
    python3 scripts/scaffold_baseline_check.py

    # Run audits and rewrite the baseline file (snapshot current state).
    python3 scripts/scaffold_baseline_check.py --update-baseline

    # Run audits + print a human-readable diff vs baseline (no exit code).
    python3 scripts/scaffold_baseline_check.py --diff-only

The script is split into pure-function building blocks so the
regression-detection logic can be unit-tested without running the
audits (which need the ~50 MB ast_dataset.jsonl). See
``tests/test_scaffold_baseline_check.py``.
"""

from __future__ import annotations

import argparse
import dataclasses
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Iterable

ROOT = Path(__file__).resolve().parents[1]
BASELINE_PATH = ROOT / "data" / "rules" / "scaffold-baseline.json"
ERAS = (1, 2, 3, 4)
DEFAULT_TOLERANCE_PCT = 0.5

# Audit headline lines look like:
#   - Era 1 Condition nodes: **2499** (bucketed 2470, unbucketed 29, 1.2% gap)
#   - Era 1 Trigger nodes:   **11548** (bucketed 11548, unbucketed 0, 0.0% gap)
HEADLINE_RE = re.compile(
    r"-\s+Era\s+(\d+)\s+(Condition|Trigger)\s+nodes:\s+\*\*(\d+)\*\*\s+"
    r"\(bucketed\s+(\d+),\s+unbucketed\s+(\d+),\s+([\d.]+)%\s+gap\)"
)


# ---------------------------------------------------------------------------
# Pure-function building blocks.
# ---------------------------------------------------------------------------


@dataclasses.dataclass(frozen=True)
class EraStats:
    """One era's gap stats for a single node type (conditions or triggers)."""

    total: int
    unbucketed: int
    gap_pct: float

    def to_dict(self) -> dict:
        return {"total": self.total, "unbucketed": self.unbucketed, "gap_pct": self.gap_pct}

    @classmethod
    def from_dict(cls, raw: dict) -> "EraStats":
        return cls(total=int(raw["total"]), unbucketed=int(raw["unbucketed"]), gap_pct=float(raw["gap_pct"]))


def parse_audit_headlines(text: str) -> dict[tuple[int, str], EraStats]:
    """Extract every (era, kind) → EraStats from one or more audit stdout strings.

    ``kind`` is normalised to ``"conditions"`` or ``"triggers"`` so callers
    can index the result with a stable key.
    """
    out: dict[tuple[int, str], EraStats] = {}
    for match in HEADLINE_RE.finditer(text):
        era = int(match.group(1))
        kind = match.group(2).lower() + "s"  # "Condition" → "conditions"
        total = int(match.group(3))
        unbucketed = int(match.group(5))
        gap_pct = float(match.group(6))
        out[(era, kind)] = EraStats(total=total, unbucketed=unbucketed, gap_pct=gap_pct)
    return out


@dataclasses.dataclass(frozen=True)
class Regression:
    """A single regression entry: which era, which kind, by how much."""

    era: int
    kind: str  # "conditions" or "triggers"
    baseline_pct: float
    current_pct: float
    delta_pct: float

    def message(self) -> str:
        return (
            f"Era {self.era} {self.kind}: baseline {self.baseline_pct:.2f}% → "
            f"current {self.current_pct:.2f}% (Δ +{self.delta_pct:.2f}%)"
        )


def check_regression(
    current: dict[tuple[int, str], EraStats],
    baseline: dict[tuple[int, str], EraStats],
    tolerance_pct: float = DEFAULT_TOLERANCE_PCT,
) -> list[Regression]:
    """Return the list of (era, kind) pairs that regressed more than tolerance.

    An era/kind in baseline but absent from current counts as a regression
    (the audit must have failed). Improvements (current better than
    baseline) are silently ignored — they're great, just not failures.
    """
    out: list[Regression] = []
    for key, base in baseline.items():
        cur = current.get(key)
        if cur is None:
            out.append(
                Regression(
                    era=key[0],
                    kind=key[1],
                    baseline_pct=base.gap_pct,
                    current_pct=float("nan"),
                    delta_pct=float("inf"),
                )
            )
            continue
        delta = cur.gap_pct - base.gap_pct
        if delta > tolerance_pct:
            out.append(
                Regression(
                    era=key[0],
                    kind=key[1],
                    baseline_pct=base.gap_pct,
                    current_pct=cur.gap_pct,
                    delta_pct=delta,
                )
            )
    return out


def serialise_baseline(stats: dict[tuple[int, str], EraStats], tolerance_pct: float) -> dict:
    """Convert a stats dict back to the JSON-friendly baseline shape."""
    eras_out: dict[str, dict] = {}
    for (era, kind), s in stats.items():
        eras_out.setdefault(str(era), {})[kind] = s.to_dict()
    return {
        "regression_tolerance_pct": tolerance_pct,
        "eras": eras_out,
    }


def deserialise_baseline(raw: dict) -> tuple[dict[tuple[int, str], EraStats], float]:
    """Inverse of serialise_baseline."""
    tolerance = float(raw.get("regression_tolerance_pct", DEFAULT_TOLERANCE_PCT))
    out: dict[tuple[int, str], EraStats] = {}
    for era_str, by_kind in raw.get("eras", {}).items():
        era = int(era_str)
        for kind, payload in by_kind.items():
            out[(era, kind)] = EraStats.from_dict(payload)
    return out, tolerance


# ---------------------------------------------------------------------------
# IO-bound entry points (subprocess + filesystem).
# ---------------------------------------------------------------------------


def run_audit(era: int) -> str:
    """Invoke ``scripts/eraN_scaffold_audit.py`` and return its stdout.

    Surfaces a clear error if the AST dataset isn't accessible so the
    operator can fix it; the per-era audit script raises FileNotFoundError
    if the dataset is missing.
    """
    script = ROOT / "scripts" / f"era{era}_scaffold_audit.py"
    proc = subprocess.run(
        [sys.executable, str(script)],
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        sys.stderr.write(
            f"era{era}_scaffold_audit.py exited {proc.returncode}\n"
            f"stderr: {proc.stderr}\n"
        )
        raise SystemExit(proc.returncode)
    return proc.stdout


def collect_current_gaps(eras: Iterable[int] = ERAS) -> dict[tuple[int, str], EraStats]:
    """Run every era audit and merge the parsed headlines into one dict."""
    merged: dict[tuple[int, str], EraStats] = {}
    for era in eras:
        merged.update(parse_audit_headlines(run_audit(era)))
    return merged


def load_baseline(path: Path = BASELINE_PATH) -> tuple[dict[tuple[int, str], EraStats], float]:
    """Read the committed baseline JSON. Raises if absent or malformed."""
    with path.open("r", encoding="utf-8") as f:
        return deserialise_baseline(json.load(f))


def write_baseline(
    path: Path,
    stats: dict[tuple[int, str], EraStats],
    tolerance_pct: float,
) -> None:
    """Atomically overwrite the baseline JSON with the given stats."""
    payload = serialise_baseline(stats, tolerance_pct)
    tmp = path.with_suffix(".json.tmp")
    with tmp.open("w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2, sort_keys=True)
        f.write("\n")
    tmp.replace(path)


# ---------------------------------------------------------------------------
# CLI.
# ---------------------------------------------------------------------------


def render_diff(
    current: dict[tuple[int, str], EraStats],
    baseline: dict[tuple[int, str], EraStats],
) -> str:
    """Return a human-readable table comparing every era/kind pair."""
    lines = ["era kind        baseline   current    delta"]
    lines.append("--- ----------- ---------- ---------- --------")
    for era in sorted({k[0] for k in current} | {k[0] for k in baseline}):
        for kind in ("conditions", "triggers"):
            cur = current.get((era, kind))
            base = baseline.get((era, kind))
            if cur is None and base is None:
                continue
            cur_pct = f"{cur.gap_pct:.2f}%" if cur else "—"
            base_pct = f"{base.gap_pct:.2f}%" if base else "—"
            if cur and base:
                delta = cur.gap_pct - base.gap_pct
                delta_str = f"{delta:+.2f}%"
            else:
                delta_str = "—"
            lines.append(f"{era:<3} {kind:<11} {base_pct:>9}  {cur_pct:>9}  {delta_str:>7}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--update-baseline",
        action="store_true",
        help="Run audits and rewrite the baseline file with current numbers.",
    )
    parser.add_argument(
        "--diff-only",
        action="store_true",
        help="Print current-vs-baseline diff but exit 0 regardless of regressions.",
    )
    parser.add_argument(
        "--baseline-path",
        default=str(BASELINE_PATH),
        help="Path to the baseline JSON (defaults to data/rules/scaffold-baseline.json).",
    )
    parser.add_argument(
        "--tolerance",
        type=float,
        default=None,
        help=(
            "Override the per-era gap-pct tolerance. Defaults to the value "
            "stored in the baseline file (or 0.5 if absent)."
        ),
    )
    args = parser.parse_args(argv)

    baseline_path = Path(args.baseline_path)

    current = collect_current_gaps()

    if args.update_baseline:
        tolerance = args.tolerance if args.tolerance is not None else DEFAULT_TOLERANCE_PCT
        write_baseline(baseline_path, current, tolerance_pct=tolerance)
        print(f"Updated baseline at {baseline_path} ({len(current)} era×kind entries)")
        return 0

    if not baseline_path.exists():
        sys.stderr.write(
            f"No baseline at {baseline_path}. Run with --update-baseline to create one.\n"
        )
        return 2

    baseline, stored_tolerance = load_baseline(baseline_path)
    tolerance = args.tolerance if args.tolerance is not None else stored_tolerance

    print(render_diff(current, baseline))
    regressions = check_regression(current, baseline, tolerance_pct=tolerance)

    if not regressions:
        print(f"\nOK — no era regressed by more than {tolerance:.2f}% gap-pct.")
        return 0

    if args.diff_only:
        print(f"\n(diff-only: {len(regressions)} regression(s) found but exit code suppressed.)")
        return 0

    print(f"\nREGRESSION — {len(regressions)} era×kind pair(s) exceeded {tolerance:.2f}% tolerance:")
    for r in regressions:
        print(f"  - {r.message()}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
