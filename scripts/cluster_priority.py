#!/usr/bin/env python3
"""Re-rank semantic clusters by EDH play weight, not raw card count.

Premise: a cluster of 1,000 cards that nobody plays in commander matters less
than a cluster of 50 cards that show up in 30% of competitive decks. Raw card
count is a misleading priority signal — it tells you what magic *prints*, not
what magic *plays*.

Method: each card carries Scryfall's `edhrec_rank` (1 = most popular in EDH,
higher = less played). We convert that to an inclusion weight via a square-root
decay (Sol Ring=1.0, rank-100 staple ≈ 0.1, deep jank ≈ 0.01) and sum across
each cluster. Re-rank by weighted sum.

Output: data/rules/cluster_priority.md — the engine-build queue, prioritized
by competitive impact rather than print count.

Usage: python3 scripts/cluster_priority.py
"""

from __future__ import annotations

import json
import math
from collections import defaultdict
from pathlib import Path

import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from semantic_clusters import (
    ORACLE_DUMP, extract_effects, is_real_card, render_signature, pct,
)

REPORT = Path(__file__).resolve().parents[1] / "data" / "rules" / "cluster_priority.md"


# ============================================================================
# Play-weight scoring
# ============================================================================

def play_weight(card: dict) -> float:
    """Convert Scryfall's edhrec_rank into a [0, 1] play weight.

    Lower rank = higher weight. Square-root decay because we want the head of
    the distribution (Sol Ring tier) to dominate but not crush the tail to zero.

    Cards without an edhrec_rank (~17% of the pool: too new, never played in EDH,
    or banned-and-removed) get weight 0 — they don't show up in commander, so
    they shouldn't bias handler priority for EDH-focused engine work.
    """
    rank = card.get("edhrec_rank")
    if rank is None or rank <= 0:
        return 0.0
    return 1.0 / math.sqrt(rank)


# ============================================================================
# Cluster aggregation
# ============================================================================

def main() -> None:
    cards = json.loads(ORACLE_DUMP.read_text())
    real = [c for c in cards if is_real_card(c)]

    # Cluster + carry play weights
    clusters: dict[tuple, dict] = defaultdict(lambda: {
        "cards": [],          # (name, edhrec_rank, weight)
        "weight": 0.0,
    })
    total_weight = 0.0
    for c in real:
        sig = extract_effects(c)
        w = play_weight(c)
        clusters[sig]["cards"].append((c["name"], c.get("edhrec_rank"), w))
        clusters[sig]["weight"] += w
        total_weight += w

    # Sort by weighted impact
    by_weight = sorted(clusters.items(), key=lambda kv: -kv[1]["weight"])
    by_count = sorted(clusters.items(), key=lambda kv: -len(kv[1]["cards"]))

    # Singletons ranked by their card's individual play weight
    singletons = [
        (sig, info) for sig, info in clusters.items() if len(info["cards"]) == 1
    ]
    singletons_by_weight = sorted(singletons, key=lambda kv: -kv[1]["weight"])

    # Cumulative weight curves
    def curve(sorted_clusters, attr):
        out = []
        running = 0.0 if attr == "weight" else 0
        for _, info in sorted_clusters:
            running += info["weight"] if attr == "weight" else len(info["cards"])
            out.append(running)
        return out

    weight_curve = curve(by_weight, "weight")
    count_curve = curve(by_count, "count")

    # ---- Render report ----
    lines = [
        "# Cluster Priority Report",
        "",
        f"Pool: **{len(real):,} real cards**, total EDH play-weight: **{total_weight:,.1f}**",
        "",
        "Each cluster's weight = sum of its cards' EDH play-weights "
        "(`1/√edhrec_rank`; Sol Ring=1.0, rank-100=0.1, rank-10,000=0.01). "
        "Cards with no `edhrec_rank` (~17% of pool, mostly fringe sets) contribute 0.",
        "",
        "## Engine-build curve — by play impact vs by raw count",
        "",
        "Same handler count, ranked two ways. The play-weighted curve is what",
        "actually matters for an engine targeting competitive commander.",
        "",
        "| Handlers built | % cards covered (count-ranked) | % play-weight covered (weight-ranked) |",
        "|---:|---:|---:|",
    ]
    for n in (10, 30, 50, 100, 200, 500, 1000):
        if n > len(by_weight):
            continue
        ct = count_curve[n - 1]
        wt = weight_curve[n - 1]
        lines.append(
            f"| Top {n:,} | {pct(ct, len(real))} | {pct(wt, total_weight)} |"
        )

    lines += [
        "",
        "## Top 30 clusters by play-weight (build these handlers FIRST)",
        "",
        "| # | Weight | Cards | Effect signature | Top-played samples |",
        "|---:|---:|---:|---|---|",
    ]
    for i, (sig, info) in enumerate(by_weight[:30], start=1):
        # Sample the top-3 most-played cards in this cluster
        top_cards = sorted(info["cards"], key=lambda x: x[1] or 10**9)[:3]
        sample = ", ".join(
            f"{name} (#{rank})" if rank else name
            for name, rank, _ in top_cards
        )
        if len(info["cards"]) > 3:
            sample += f", … +{len(info['cards']) - 3}"
        lines.append(
            f"| {i} | {info['weight']:,.2f} | {len(info['cards']):,} | "
            f"{render_signature(sig)} | {sample} |"
        )

    lines += [
        "",
        "## Top 50 singleton clusters by play-weight",
        "",
        "These are cards with truly unique effect signatures — each one needs its",
        "own custom handler. Ranked here by EDH inclusion, so the most competitively",
        "impactful unique cards bubble to the top of the custom-handler queue.",
        "",
        "| # | Weight | EDHRec rank | Card | Why it's a singleton |",
        "|---:|---:|---:|---|---|",
    ]
    for i, (sig, info) in enumerate(singletons_by_weight[:50], start=1):
        name, rank, w = info["cards"][0]
        sig_str = render_signature(sig)
        lines.append(f"| {i} | {w:.3f} | {rank or '—'} | {name} | {sig_str} |")

    lines += [
        "",
        "## What this means",
        "",
        f"- Building the **top 10 play-weighted handlers** covers "
        f"**{pct(weight_curve[9], total_weight)}** of all EDH play (vs "
        f"{pct(count_curve[9], len(real))} of card count).",
        f"- Top 100 weighted handlers cover **{pct(weight_curve[99], total_weight)}** of EDH play.",
        f"- Top 30 singleton handlers (the truly unique custom cards in EDH) cover "
        f"the most-played one-of-a-kind effects in the format.",
        "",
        "Handler build order should follow this report, not the raw count report.",
        "Build a top-10 weighted handler → an EDH cage can simulate "
        f"~{pct(weight_curve[9], total_weight)} of the play patterns it'll actually encounter.",
    ]

    REPORT.write_text("\n".join(lines))

    # ---- Console summary ----
    print(f"\n{'═' * 60}")
    print(f"  Cluster priority — {len(real):,} cards, total weight {total_weight:,.1f}")
    print(f"{'═' * 60}")
    print(f"\n  build curve — by play weight:")
    for n in (10, 30, 100, 500, 1000):
        if n <= len(by_weight):
            print(f"    top {n:>4,} weighted handlers → {pct(weight_curve[n-1], total_weight):>6} of EDH play")
    print(f"\n  build curve — by raw card count (for comparison):")
    for n in (10, 30, 100, 500, 1000):
        if n <= len(by_count):
            print(f"    top {n:>4,} count handlers    → {pct(count_curve[n-1], len(real)):>6} of pool")

    print(f"\n  top 10 weighted clusters (build first):")
    for i, (sig, info) in enumerate(by_weight[:10], 1):
        sig_str = render_signature(sig)[:55]
        top_card = sorted(info["cards"], key=lambda x: x[1] or 10**9)[0]
        print(f"    {i:>2}. weight={info['weight']:>7,.1f}  ({len(info['cards']):>4,} cards)  {sig_str}")
        print(f"        ↳ top-played: {top_card[0]} (#{top_card[1]})")

    print(f"\n  top 10 singleton priorities (most-played unique cards):")
    for i, (_, info) in enumerate(singletons_by_weight[:10], 1):
        name, rank, w = info["cards"][0]
        print(f"    {i:>2}. #{rank:<5}  weight={w:.3f}  {name}")

    print(f"\n  → {REPORT}")


if __name__ == "__main__":
    main()
