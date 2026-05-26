#!/usr/bin/env python3
"""Build the R7 precon-vibes calibration table from .profile.json +
.strategy.json sidecars in data/decks/wizards/freya/.

Picks only the 15 R7 decks (the ones imported by this branch's
pipeline run). Emits a markdown table to stdout matching R5's
column structure so the R7 doc reads as a continuation."""
import json
import os
from pathlib import Path


R7_DECKS = [
    # (filename_stem, era_label, precon_display_name)
    ("world_shaper_edge_of_eternities_commander_precon_decklist",
     "EoE", "World Shaper"),
    ("counter_intelligence_edge_of_eternities_commander_precon_decklist",
     "EoE", "Counter Intelligence"),
    ("vampiric_bloodline_innistrad_crimson_vow_commander_precon_decklist",
     "VOW", "Vampiric Bloodline"),
    ("spirit_squadron_innistrad_crimson_vow_commander_precon_decklist",
     "VOW", "Spirit Squadron"),
    ("mishra_s_burnished_banner_the_brothers_war_commander_precon_decklist",
     "BRO", "Mishra's Burnished Banner"),
    ("silverquill_influence_secrets_of_strixhaven_commander_precon_decklist",
     "C21 SoS", "Silverquill Influence"),
    ("everyone_s_invited_secret_lair_commander_2025_precon_decklist",
     "SL2025", "Everyone's Invited!"),
    ("bedecked_brokers_streets_of_new_capenna_commander_precon_decklist",
     "SNC", "Bedecked Brokers"),
    ("cabaretti_cacophony_streets_of_new_capenna_commander_precon_decklist",
     "SNC", "Cabaretti Cacophony"),
    ("maestros_massacre_streets_of_new_capenna_commander_precon_decklist",
     "SNC", "Maestros Massacre"),
    ("obscura_operation_streets_of_new_capenna_commander_precon_decklist",
     "SNC", "Obscura Operation"),
    ("riveteers_rampage_streets_of_new_capenna_commander_precon_decklist",
     "SNC", "Riveteers Rampage"),
    ("rebellion_rising_phyrexia_all_will_be_one_commander_precon_decklist",
     "ONE", "Rebellion Rising"),
    ("peace_offering_bloomburrow_commander_precon_decklist",
     "BLB", "Peace Offering"),
    ("graveyard_overdrive_modern_horizons_3_commander_precon_decklist",
     "MH3", "Graveyard Overdrive"),
]


PLAYS_LIKE_LABEL = {
    1: "Exhibition",
    2: "Core",
    3: "Upgraded",
    4: "Optimized",
}


def compute_declared_bracket(power_pct, win_line_count, combo_density, gc):
    """Apply the R1/R2/R5 declared-bracket rule.

    - B1 if power_pct < 25 AND no win_lines AND no combos
    - B4 lift if power_pct >= 80 OR gc >= 4
    - B3 lift if power_pct >= 60 OR combo_density >= 4 OR gc >= 2
    - Otherwise B2
    """
    if power_pct < 25 and win_line_count == 0 and combo_density == 0:
        return 1
    if power_pct >= 80 or gc >= 4:
        return 4
    if power_pct >= 60 or combo_density >= 4 or gc >= 2:
        return 3
    return 2


def chain_depth_summary(value_chains):
    """Return (avg, max) tuple from a value_chains list, or (None, None)
    if no chains. Mirrors R5's "Chain Depth (avg/max)" column shape."""
    if not value_chains:
        return None, None
    depths = [c.get("depth", 0) for c in value_chains]
    if not depths:
        return None, None
    return sum(depths) / len(depths), max(depths)


def recursion_ratio(value_chains):
    # strategy.json names the field recursion_depth (string:
    # "deep" / "shallow" / "infinite"). Match R5's "Recursion Ratio"
    # = fraction of chains with deep-or-infinite recursion.
    if not value_chains:
        return 0.0
    deep = sum(1 for c in value_chains
               if c.get("recursion_depth") in {"deep", "infinite"})
    return deep / len(value_chains)


def main():
    base = Path("data/decks/wizards/freya")
    rows = []
    for stem, era, display_name in R7_DECKS:
        prof = json.loads((base / f"{stem}.profile.json").read_text())
        strat = json.loads((base / f"{stem}.strategy.json").read_text())
        cmdr = prof.get("commander", "?")
        archetype = prof.get("primary_archetype", "?").lower()
        measured = prof.get("measured_bracket", 0)
        plays_like = strat.get("plays_like", 0)
        plays_like_lbl = strat.get("plays_like_label",
                                   PLAYS_LIKE_LABEL.get(plays_like, "?"))
        cmd_syn = (prof.get("commander_synergy") or
                   strat.get("commander_synergy", 0)) * 100
        win_lines = prof.get("win_line_count", 0)
        # combo_density = len(combo_notes); strategy.json carries
        # combo_notes more reliably than profile.json
        combo_notes = strat.get("combo_notes") or []
        combo_density = len(combo_notes)
        # value_chains lives on strategy.json (the hat-consumable
        # subset); profile.json carries the per-cluster export under
        # a different shape. Mirror what R5 did by reading strategy.json.
        chains = strat.get("value_chains") or []
        avg_d, max_d = chain_depth_summary(chains)
        rec_ratio = recursion_ratio(chains)
        gc = strat.get("game_changer_count", 0)
        power_pct = strat.get("power_percentile",
                              prof.get("power_percentile", 0))
        mana = prof.get("mana_base_grade", "?")
        declared = compute_declared_bracket(
            power_pct, win_lines, combo_density, gc)
        delta = declared - measured

        chain_cell = (f"{avg_d:.2f} / {max_d}"
                      if avg_d is not None and max_d is not None
                      else "— / —")
        if delta == 0:
            delta_cell = "✓"
        elif delta > 0:
            delta_cell = f"+{delta}"
        else:
            delta_cell = f"{delta}"
        if abs(delta) >= 2:
            delta_cell = f"**{delta_cell}**"
        mch_cell = f"**{measured} {label_for_bracket(measured)}**" if measured == 4 else f"{measured} {label_for_bracket(measured)}"
        declared_cell = f"**{declared}**"

        rows.append({
            "era": era,
            "precon": display_name,
            "commander": cmdr,
            "archetype": archetype,
            "measured": measured,
            "mch_cell": mch_cell,
            "plays_like_lbl": plays_like_lbl,
            "cmd_syn": cmd_syn,
            "win_lines": win_lines,
            "combo_density": combo_density,
            "chain_cell": chain_cell,
            "rec_ratio": rec_ratio,
            "gc": gc,
            "power_pct": power_pct,
            "mana": mana,
            "declared": declared,
            "declared_cell": declared_cell,
            "delta": delta,
            "delta_cell": delta_cell,
        })

    # Emit the table
    print("| # | Era | Precon | Commander | Archetype | Mch | Plays-Like | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | **Declared Brkt** | Δ |")
    print("|---|-----|--------|-----------|-----------|:---:|:----------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:--------------:|:-:|")
    for i, r in enumerate(rows, 1):
        print(f"| {i:2d} | {r['era']:7s} | {r['precon']} | {r['commander']} | "
              f"{r['archetype']:12s} | {r['mch_cell']:14s} | {r['plays_like_lbl']:10s} | "
              f"{r['cmd_syn']:.1f} | {r['win_lines']:3d} | {r['combo_density']} | "
              f"{r['chain_cell']:8s} | {r['rec_ratio']:.2f} | {r['gc']} | "
              f"{r['power_pct']} | {r['mana']} | {r['declared_cell']} | "
              f"{r['delta_cell']} |")

    # Aggregate
    n = len(rows)
    exact = sum(1 for r in rows if r["delta"] == 0)
    within1 = sum(1 for r in rows if abs(r["delta"]) <= 1)
    b4_fp = sum(1 for r in rows if r["measured"] == 4)
    measured_vs_pl_disagree = sum(
        1 for r in rows if r["measured"] != strat_plays_like_int(r))
    delta_neg = sum(1 for r in rows if r["delta"] <= -1)
    delta_pos = sum(1 for r in rows if r["delta"] >= 1)

    print()
    print(f"**Aggregate:** {exact}/{n} exact, {within1}/{n} within ±1, "
          f"{b4_fp}/{n} measured B4, {delta_neg}/{n} engine-hotter, "
          f"{delta_pos}/{n} engine-cooler.")


def strat_plays_like_int(r):
    """We re-stash plays_like in rows for cross-check; for the disagree
    count we want measured_bracket vs plays_like as integers."""
    return r.get("plays_like", 0)


def label_for_bracket(b):
    return {1: "Exhibition", 2: "Core", 3: "Upgraded", 4: "Optimized"}.get(b, "?")


if __name__ == "__main__":
    main()
