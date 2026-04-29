"""OctoHat smoke test — deep inspection.

Same game setup as octo_smoke.py but emits a richer per-game and per-seat
breakdown so we can see what's actually happening inside the 2000+ events.
"""
from __future__ import annotations

import collections
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from octo_smoke import run_one_game, TEST_DECKS  # noqa: E402
from extensions.policies import OctoHat  # noqa: E402


def analyze_game(game_idx, game):
    events = game.events
    print(f"\n=== Game {game_idx}: {len(events)} events, winner seat {game.winner}, turns {game.turn} ===")

    # Event type distribution
    by_type = collections.Counter(e.get("type", "?") for e in events)
    print(f"  event-type distribution ({len(by_type)} distinct types):")
    for ev_type, count in by_type.most_common(15):
        print(f"    {count:>5} {ev_type}")

    # Per-seat breakdown
    by_seat = collections.Counter()
    for e in events:
        s = e.get("seat")
        if s is not None:
            by_seat[s] += 1
    print(f"  events attributed per seat: {dict(by_seat)}")

    # Casts per seat + which cards
    casts_per_seat = collections.defaultdict(list)
    for e in events:
        if e.get("type") == "cast":
            seat = e.get("seat", -1)
            card = e.get("card", "?")
            cmc = e.get("cmc", 0)
            casts_per_seat[seat].append((card, cmc))
    print(f"  casts per seat:")
    for seat, casts in sorted(casts_per_seat.items()):
        print(f"    seat {seat}: {len(casts)} casts, avg_cmc={sum(c[1] for c in casts)/max(1, len(casts)):.1f}")
        # List top 5 most-cast cards this game
        card_counter = collections.Counter(c[0] for c in casts)
        for name, n in card_counter.most_common(5):
            print(f"      {n}x {name}")

    # Mana events
    mana_events = [e for e in events if e.get("type") == "add_mana"]
    print(f"  add_mana events: {len(mana_events)}")
    if mana_events:
        sources = collections.Counter(e.get("source", "?") for e in mana_events)
        print(f"    sources: {dict(sources)}")

    # Trigger / SBA events
    trigger_events = [e for e in events if e.get("type", "").startswith("trigger")]
    sba_events = [e for e in events if e.get("type", "").startswith("sba_")]
    print(f"  trigger events: {len(trigger_events)}, SBA events: {len(sba_events)}")
    sba_types = collections.Counter(e.get("type", "?") for e in sba_events)
    if sba_types:
        print(f"    SBA breakdown: {dict(sba_types)}")

    # Damage events breakdown
    damage_events = [e for e in events if e.get("type") == "damage"]
    print(f"  damage events: {len(damage_events)}")
    if damage_events:
        by_phase = collections.Counter(e.get("phase", "?") for e in damage_events)
        by_target_kind = collections.Counter(e.get("target_kind", "?") for e in damage_events)
        print(f"    by phase: {dict(by_phase)}")
        print(f"    by target kind: {dict(by_target_kind)}")
        combat_dmg = [e for e in damage_events if e.get("phase") == "combat"]
        total_combat_dmg = sum(e.get("amount", 0) for e in combat_dmg)
        print(f"    total combat damage: {total_combat_dmg}")

    # Life total evolution
    print(f"  final life totals:")
    for s in game.seats:
        lost_marker = " LOST" if s.lost else ""
        reason = f" ({s.loss_reason})" if s.lost and getattr(s, "loss_reason", None) else ""
        print(f"    seat {s.idx} ({s.commander_names[0] if s.commander_names else '?'}): "
              f"{s.life} life{lost_marker}{reason}")

    # Commander damage — did anyone accumulate?
    any_cdmg = False
    for s in game.seats:
        for dealer, by_name in (s.commander_damage or {}).items():
            for nm, dmg in by_name.items():
                if dmg > 0:
                    any_cdmg = True
                    print(f"    cdmg: seat {s.idx} took {dmg} from {nm} (seat {dealer})")
    if not any_cdmg:
        print(f"    (no commander damage accumulated — nobody was hit by a commander in combat)")

    # Anomaly spotting: events that look like they might be bugs
    anomalies = []
    # Anomaly 1: cast events with cmc=0 where the card isn't free (lands etc.)
    for e in events:
        if e.get("type") == "cast" and e.get("cmc") == 0:
            card = e.get("card", "")
            if card and not any(t in card.lower() for t in ["land", "petal", "mox", "sol ring", "mana vault", "mana crypt"]):
                anomalies.append(f"cast cmc=0 on non-free card: {card}")
    # Anomaly 2: damage events where amount <= 0
    for e in events:
        if e.get("type") == "damage" and (e.get("amount") or 0) <= 0:
            anomalies.append(f"damage event with amount={e.get('amount')}: source={e.get('source_card')}, target_kind={e.get('target_kind')}")
    if anomalies:
        print(f"  ANOMALIES FLAGGED: {len(anomalies)}")
        for a in anomalies[:10]:
            print(f"    - {a}")
        if len(anomalies) > 10:
            print(f"    ... ({len(anomalies) - 10} more)")

    return {
        "event_types": by_type,
        "damage_events": len(damage_events),
        "anomalies": anomalies,
    }


def main():
    print(f"=== OctoHat deep inspection: {len(TEST_DECKS)} seats ===")
    print(f"Decks: {TEST_DECKS}")

    all_anomalies = []
    aggregate_event_types = collections.Counter()

    for i in range(3):  # 3 games for focused look
        g = run_one_game(OctoHat, seed=42 + i)
        result = analyze_game(i, g)
        aggregate_event_types += result["event_types"]
        all_anomalies.extend(result["anomalies"])

    print(f"\n=== Aggregate event-type distribution (3 games) ===")
    for ev_type, count in aggregate_event_types.most_common(30):
        print(f"  {count:>6} {ev_type}")

    print(f"\n=== Total anomalies across 3 games: {len(all_anomalies)} ===")
    if all_anomalies:
        anom_counts = collections.Counter(a[:80] for a in all_anomalies)
        for a, n in anom_counts.most_common(15):
            print(f"  {n}x {a}")


if __name__ == "__main__":
    main()
