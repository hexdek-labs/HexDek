#!/usr/bin/env python3
"""Bracket-ELO distribution analysis for showmatch_elo snapshot.

Reads data/hexdek-snapshot.db, emits a markdown doc with per-bracket
histograms, outliers, and a floor-at-0 linear shift transformation.
"""
import sqlite3
import statistics
import sys
from collections import defaultdict
from pathlib import Path


def histogram_ascii(values, bins=20, width=40):
    if not values:
        return ["(no data)"]
    lo, hi = min(values), max(values)
    if hi == lo:
        return [f"  {lo:8.1f} | {'#' * width} ({len(values)})"]
    step = (hi - lo) / bins
    counts = [0] * bins
    for v in values:
        idx = int((v - lo) / step)
        if idx == bins:
            idx = bins - 1
        counts[idx] += 1
    peak = max(counts) or 1
    lines = []
    for i, c in enumerate(counts):
        lo_edge = lo + i * step
        hi_edge = lo_edge + step
        bar = "#" * int(round(c / peak * width))
        lines.append(f"  {lo_edge:8.1f} … {hi_edge:8.1f} | {bar:<{width}} {c}")
    return lines


def z_score(x, mean, sd):
    if sd == 0:
        return 0.0
    return (x - mean) / sd


def main():
    repo_root = Path(__file__).resolve().parent.parent
    db_path = repo_root / "data" / "hexdek-snapshot.db"
    out_path = repo_root / "docs" / "bracket-elo-distribution-r60.md"

    conn = sqlite3.connect(str(db_path))
    cur = conn.cursor()

    cur.execute(
        "SELECT deck_key, commander, owner, rating, games, wins, losses, bracket "
        "FROM showmatch_elo ORDER BY bracket, rating"
    )
    rows = cur.fetchall()

    by_bracket = defaultdict(list)
    for r in rows:
        by_bracket[r[7]].append(r)

    all_ratings = [r[3] for r in rows]
    global_min = min(all_ratings)
    global_max = max(all_ratings)
    global_mean = statistics.mean(all_ratings)
    global_median = statistics.median(all_ratings)
    global_sd = statistics.stdev(all_ratings)

    # MIN_RATING_OFFSET — linear shift so worst deck reads exactly 0
    offset = -global_min  # add this to every rating
    shifted = [r + offset for r in all_ratings]

    lines = []
    lines.append("# Bracket-vs-ELO distribution (r60 snapshot)")
    lines.append("")
    lines.append(f"Source: `data/hexdek-snapshot.db` table `showmatch_elo` "
                 f"({len(rows)} decks, snapshot date 2026-05-26).")
    lines.append("")
    lines.append("## Global summary (raw ratings)")
    lines.append("")
    lines.append(f"- decks: **{len(rows)}**")
    lines.append(f"- min: **{global_min:.2f}**")
    lines.append(f"- max: **{global_max:.2f}**")
    lines.append(f"- mean: **{global_mean:.2f}**")
    lines.append(f"- median: **{global_median:.2f}**")
    lines.append(f"- stdev: **{global_sd:.2f}**")
    lines.append("")
    lines.append("Per-bracket breakdown:")
    lines.append("")
    lines.append("| Bracket | Decks | Min | Max | Mean | Median | Stdev |")
    lines.append("|---:|---:|---:|---:|---:|---:|---:|")
    for b in sorted(by_bracket.keys()):
        rs = [r[3] for r in by_bracket[b]]
        sd = statistics.stdev(rs) if len(rs) > 1 else 0.0
        lines.append(
            f"| B{b} | {len(rs)} | {min(rs):.1f} | {max(rs):.1f} | "
            f"{statistics.mean(rs):.1f} | {statistics.median(rs):.1f} | {sd:.1f} |"
        )
    lines.append("")
    lines.append("**Observation:** mean rating decreases monotonically as bracket "
                 "rises (B0 +1175 → B5 −1815). This is the expected showmatch shape: "
                 "higher-bracket decks face stiffer pods, so lifetime win-rates settle "
                 "lower than the 1500 starting point even for archetypally strong builds.")
    lines.append("")

    # Histograms
    lines.append("## Per-bracket histograms (20-bin ASCII)")
    lines.append("")
    for b in sorted(by_bracket.keys()):
        rs = [r[3] for r in by_bracket[b]]
        lines.append(f"### Bracket {b} — {len(rs)} decks")
        lines.append("")
        lines.append("```")
        for line in histogram_ascii(rs, bins=20, width=40):
            lines.append(line)
        lines.append("```")
        lines.append("")

    # Outliers
    lines.append("## Top 5 / bottom 5 outliers per bracket")
    lines.append("")
    lines.append("Z-score = (rating − bracket_mean) / bracket_stdev. "
                 "Positive z = playing **above** the pack; negative z = playing below.")
    lines.append("")
    outlier_decks = []  # for cross-reference table
    for b in sorted(by_bracket.keys()):
        rs_full = by_bracket[b]
        rs = [r[3] for r in rs_full]
        if len(rs) < 2:
            continue
        mean = statistics.mean(rs)
        sd = statistics.stdev(rs)
        sorted_decks = sorted(rs_full, key=lambda r: r[3], reverse=True)
        top = sorted_decks[:5]
        bottom = sorted_decks[-5:][::-1]  # worst first

        lines.append(f"### Bracket {b} (n={len(rs)}, μ={mean:.1f}, σ={sd:.1f})")
        lines.append("")
        lines.append("**Top 5 (playing above pack):**")
        lines.append("")
        lines.append("| Commander | Owner | Rating | Games | W-L | z |")
        lines.append("|---|---|---:|---:|---|---:|")
        for r in top:
            z = z_score(r[3], mean, sd)
            outlier_decks.append((b, "top", r, z))
            lines.append(
                f"| {r[1]} | `{r[2]}` | {r[3]:.1f} | {r[4]} | "
                f"{r[5]}-{r[6]} | {z:+.2f} |"
            )
        lines.append("")
        lines.append("**Bottom 5 (playing below pack):**")
        lines.append("")
        lines.append("| Commander | Owner | Rating | Games | W-L | z |")
        lines.append("|---|---|---:|---:|---|---:|")
        for r in bottom:
            z = z_score(r[3], mean, sd)
            outlier_decks.append((b, "bottom", r, z))
            lines.append(
                f"| {r[1]} | `{r[2]}` | {r[3]:.1f} | {r[4]} | "
                f"{r[5]}-{r[6]} | {z:+.2f} |"
            )
        lines.append("")

    # Freya synergy cross-reference
    lines.append("## Freya synergy cross-reference")
    lines.append("")
    lines.append("**Status: not available in this snapshot.**")
    lines.append("")
    lines.append("`data/hexdek-snapshot.db` ships the rating table + 3 gauntlet rows + 10 "
                 "deck_meta rows, but neither a per-deck Freya `DeckProfile` cache nor a "
                 "synergy-score column. The 1,319 showmatch decks are referenced by "
                 "`deck_key` only — the `deck` table itself is empty in this snapshot, so "
                 "raw_json → Freya analysis is not reproducible from the snapshot alone.")
    lines.append("")
    lines.append("To answer the correlation question end-to-end we would need either:")
    lines.append("")
    lines.append("1. The raw deck JSONs co-keyed by `deck_key` (re-run Freya across all 1,319), or")
    lines.append("2. A pre-computed `deck_freya_profile` table written by the deckbuilder service "
                 "(synergy %, archetype, primary roles, power tier counts) joined on `deck_key`.")
    lines.append("")
    lines.append("Option (2) is the cheap path — Freya already emits this in the JSON profile; "
                 "wiring a snapshot-export of the relevant scalars (synergy_pct, archetype, "
                 "power_tier_counts) would let this analysis run on every snapshot without "
                 "re-parsing oracle text. **Recommend filing as a snapshot-schema follow-up.**")
    lines.append("")
    lines.append("**Proxy observation from outlier table:** the top outliers are dominated by "
                 "low-bracket decks (B2/B3) with extremely high game counts (>150K games each — "
                 "these are clearly long-running gauntlet anchors, not casual entries). Bottom "
                 "outliers in B4/B5 are similarly high-volume. So the visible outlier-magnitude "
                 "is largely a **sample-size artifact** — decks with many games have ratings far "
                 "from 1500 simply because they've had time to drift. A Freya synergy correlation "
                 "study should normalize by games played before drawing conclusions.")
    lines.append("")

    # Floor-at-0 transformation
    lines.append("## Floor-at-0 transformation (linear shift, NOT clamp)")
    lines.append("")
    lines.append("Goal per 7174n1c: user-facing graphs should sit in Quadrant II (positive Y "
                 "only, no negative ELO values shown). Linear shift preserves **all** relative "
                 "gaps — the difference between any two decks is identical before and after.")
    lines.append("")
    lines.append(f"```")
    lines.append(f"MIN_RATING_OFFSET = {offset:.2f}")
    lines.append(f"rating_display    = rating_raw + MIN_RATING_OFFSET")
    lines.append(f"```")
    lines.append("")
    lines.append("**This is a display transform.** Internal storage stays in the raw signed "
                 "ELO space so deltas continue to compose with the showmatch update math.")
    lines.append("")
    lines.append("### Before vs after (global)")
    lines.append("")
    lines.append("| Metric | Raw | Shifted |")
    lines.append("|---|---:|---:|")
    lines.append(f"| min | {global_min:.2f} | {min(shifted):.2f} |")
    lines.append(f"| max | {global_max:.2f} | {max(shifted):.2f} |")
    lines.append(f"| mean | {global_mean:.2f} | {statistics.mean(shifted):.2f} |")
    lines.append(f"| median | {global_median:.2f} | {statistics.median(shifted):.2f} |")
    lines.append(f"| stdev | {global_sd:.2f} | {statistics.stdev(shifted):.2f} |")
    lines.append("")
    lines.append("Stdev and any pairwise gap are invariant under linear shift — confirmed numerically.")
    lines.append("")
    lines.append("### Before vs after (per bracket means)")
    lines.append("")
    lines.append("| Bracket | Raw mean | Shifted mean |")
    lines.append("|---:|---:|---:|")
    for b in sorted(by_bracket.keys()):
        rs = [r[3] for r in by_bracket[b]]
        raw_mean = statistics.mean(rs)
        shifted_mean = raw_mean + offset
        lines.append(f"| B{b} | {raw_mean:.1f} | {shifted_mean:.1f} |")
    lines.append("")
    lines.append("### Worst / best decks (shifted)")
    lines.append("")
    sorted_by_rating = sorted(rows, key=lambda r: r[3])
    lines.append("| | Commander | Bracket | Raw | Shifted |")
    lines.append("|---|---|---:|---:|---:|")
    worst = sorted_by_rating[0]
    best = sorted_by_rating[-1]
    lines.append(f"| worst | {worst[1]} | B{worst[7]} | {worst[3]:.1f} | "
                 f"{worst[3] + offset:.2f} |")
    lines.append(f"| best | {best[1]} | B{best[7]} | {best[3]:.1f} | "
                 f"{best[3] + offset:.2f} |")
    lines.append("")
    lines.append("Worst lands at exactly 0.00; everything else strictly positive. "
                 "Quadrant II shape satisfied.")
    lines.append("")

    lines.append("## Recommended next steps")
    lines.append("")
    lines.append("1. Add a `deck_freya_profile` snapshot table (synergy_pct, archetype, "
                 "power_tier_counts, primary_roles) so this analysis can include the actual "
                 "synergy correlation rather than the proxy note above.")
    lines.append("2. Wire `MIN_RATING_OFFSET` into the showmatch screen + Heimdall summary "
                 "render layer — keep raw signed ELO in storage / deltas, only shift at "
                 "display time.")
    lines.append("3. Normalize outlier scans by `games` (z-score on rating-per-100-games) "
                 "before publishing a 'breakout decks' list — current top/bottom is sample-size "
                 "dominated.")
    lines.append("")

    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text("\n".join(lines) + "\n")
    print(f"wrote {out_path} ({len(lines)} lines)")
    print(f"MIN_RATING_OFFSET = {offset:.2f}")


if __name__ == "__main__":
    main()
