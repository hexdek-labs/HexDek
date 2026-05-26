#!/usr/bin/env python3
"""Extract concise per-deck data for the precon shape-scan doc.

Reads strategy.json + profile.json for each deck in the sorted
data/decks/wizards/*.txt list, slices [start, end] (0-indexed),
and emits one stanza per deck with the raw signals — the human-
written sections (Verdict, Reasoning) go on top of this output in
the docs/precon-shape-scans/group-*.md doc.
"""
import json
import os
import re
import sys
from pathlib import Path


def main():
    if len(sys.argv) < 3:
        print("usage: precon_shape_scan_extract.py <start_1idx> <end_1idx>", file=sys.stderr)
        sys.exit(2)
    start_1 = int(sys.argv[1])
    end_1 = int(sys.argv[2])

    base = Path("data/decks/wizards")
    freya_dir = base / "freya"
    txt_files = sorted(base.glob("*.txt"))
    selected = txt_files[start_1 - 1:end_1]

    for deck_path in selected:
        stem = deck_path.stem
        prof_p = freya_dir / f"{stem}.profile.json"
        strat_p = freya_dir / f"{stem}.strategy.json"
        if not prof_p.exists() or not strat_p.exists():
            print(f"### {stem}\n- (missing freya sidecars)\n")
            continue
        prof = json.loads(prof_p.read_text())
        strat = json.loads(strat_p.read_text())

        commander = prof.get("commander", "?")
        archetype = prof.get("primary_archetype", "?")
        secondary = prof.get("secondary_archetype", "")
        intent = prof.get("intent", "")
        bracket = prof.get("measured_bracket", "?")
        bracket_lbl = prof.get("measured_bracket_label", "?")
        synergy = prof.get("commander_synergy", 0) * 100
        win_line = prof.get("primary_win_line", "")
        win_line_count = prof.get("win_line_count", 0)
        gameplan = prof.get("gameplan_summary", "")
        power_pct = prof.get("power_percentile", strat.get("power_percentile", 0))
        gc = strat.get("game_changer_count", 0)
        gc_cards = strat.get("game_changer_cards", []) or []
        combo_notes = strat.get("combo_notes", []) or []
        win_lines_arr = strat.get("win_lines", []) or []
        top_roles = prof.get("top_roles", [])
        themes = prof.get("commander_themes", []) or strat.get("commander_themes", [])
        mana_grade = prof.get("mana_base_grade", "?")
        rec_lands = prof.get("recommended_lands", 0)
        land_count = prof.get("land_count", 0)
        star_cards = prof.get("star_cards", []) or []
        solid = prof.get("solid_cards", []) or []
        cuttable = prof.get("cuttable_cards", []) or []
        clusters = prof.get("synergy_clusters", []) or []
        value_chains = strat.get("value_chains", []) or []

        # Extract deck filename → precon name + set
        # e.g., "animated_army_bloomburrow_commander_precon_decklist" →
        # ("Animated Army", "Bloomburrow")
        precon_name, set_label = parse_precon_name(stem)

        print(f"## {commander} — {precon_name} ({set_label})")
        print(f"- file: `{stem}.txt`")
        print(f"- archetype: {archetype}" + (f" / {secondary}" if secondary else ""))
        print(f"- measured_bracket: **{bracket} {bracket_lbl}**")
        print(f"- power_pct: {power_pct} | mana: {mana_grade} | cmdr_syn: {synergy:.1f}%")
        print(f"- intent: {intent}")
        if gameplan:
            print(f"- gameplan: {gameplan}")
        print(f"- top_roles: {', '.join(f'{r['role']}({r['count']})' for r in top_roles[:3])}")
        if themes:
            print(f"- themes: {', '.join(themes[:5])}")
        if win_line:
            print(f"- primary_win_line: {win_line}")
        print(f"- win_line_count: {win_line_count}")
        if gc:
            print(f"- game_changers: {gc} — {', '.join(gc_cards[:5])}")
        if combo_notes:
            print(f"- combo_notes: {len(combo_notes)} — {combo_notes[0][:120]}")
        # First 5 win lines (terse, just type + pieces)
        if win_lines_arr:
            print(f"- first_win_lines:")
            for wl in win_lines_arr[:5]:
                pieces = ' + '.join(wl.get('pieces', []))
                wtype = wl.get('type', '?')
                wclass = wl.get('class', '')
                cls = f" [{wclass}]" if wclass else ""
                print(f"  - {wtype}{cls}: {pieces}")
        # Top synergy clusters
        if clusters:
            print(f"- synergy_clusters:")
            for c in clusters[:3]:
                print(f"  - {c.get('name', '?')} ({c.get('member_count', '?')} cards, score={c.get('score', '?')}): {c.get('theme', '')}")
        # Value chains (1-line each)
        if value_chains:
            print(f"- value_chains:")
            for vc in value_chains[:3]:
                print(f"  - {vc.get('name', '?')} (depth {vc.get('depth', '?')}, redundancy {vc.get('redundancy', '?')}, recursion {vc.get('recursion_depth', '?')})")
        # Top 5 star cards
        if star_cards:
            star_names = [c.get('name', '?') for c in star_cards[:5]]
            print(f"- stars: {', '.join(star_names)}")
        print()


def parse_precon_name(stem):
    """Convert filename stem to (Precon Name, Set Label)."""
    s = stem.replace("_commander_precon_decklist", "")
    # Find the split between precon name and set name. We don't have
    # a reliable boundary, so use a heuristic: known set tokens.
    set_markers = [
        ("bloomburrow", "Bloomburrow / BLB"),
        ("murders_at_karlov_manor", "Karlov Manor / MKM"),
        ("doctor_who", "Doctor Who"),
        ("commander_2016", "Commander 2016"),
        ("commander_2014", "Commander 2014"),
        ("commander_2013", "Commander 2013"),
        ("commander_2017", "Commander 2017"),
        ("commander_2018", "Commander 2018"),
        ("commander_2019", "Commander 2019"),
        ("commander_2020", "Commander 2020"),
        ("commander_2021", "Commander 2021"),
        ("kamigawa_neon_dynasty", "Neon Dynasty / NEO"),
        ("march_of_the_machine", "March of the Machine / MOM"),
        ("phyrexia_all_will_be_one", "Phyrexia All Will Be One / ONE"),
        ("duskmourn", "Duskmourn / DSK"),
        ("outlaws_of_thunder_junction", "Thunder Junction / OTJ"),
        ("adventures_in_the_forgotten_realms", "Forgotten Realms / AFR"),
        ("the_lord_of_the_rings", "Lord of the Rings / LTR"),
        ("modern_horizons_3", "Modern Horizons 3 / MH3"),
        ("dominaria_united", "Dominaria United / DMU"),
        ("innistrad_midnight_hunt", "Midnight Hunt / MID"),
        ("innistrad_crimson_vow", "Crimson Vow / VOW"),
        ("the_lost_caverns_of_ixalan", "Lost Caverns / LCI"),
        ("the_brothers_war", "Brothers' War / BRO"),
        ("aetherdrift", "Aetherdrift / DFT"),
        ("final_fantasy", "Final Fantasy / FIN"),
        ("fallout", "Fallout / PIP"),
        ("secrets_of_strixhaven", "Secrets of Strixhaven"),
        ("edge_of_eternities", "Edge of Eternities / EoE"),
        ("warhammer_40_000", "Warhammer 40,000"),
        ("secret_lair_commander_2025", "Secret Lair Commander 2025"),
        ("streets_of_new_capenna", "Streets of New Capenna / SNC"),
        ("commander_2011", "Commander 2011"),
        ("commander_2015", "Commander 2015"),
    ]
    for token, label in set_markers:
        if token in s:
            name = s.replace("_" + token, "").replace(token + "_", "")
            return title_case(name), label
    return title_case(s), "?"


def title_case(s):
    return ' '.join(w.capitalize() for w in s.split('_'))


if __name__ == "__main__":
    main()
