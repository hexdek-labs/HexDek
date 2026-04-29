"""Deck-archetype signature helpers.

Most deck-signature detection lives in :mod:`detectors` (Tier 2) so
the detector registry stays the source of truth. This module exposes
the archetype classifier that callers can reference in the report
("Seat 0: storm deck, seat 1: voltron, seat 2: ramp…") — it's a
convenience layer, not a detection layer.

Classification is name-heuristic and deliberately simple. If the
engine ever grows a real deck-tagging pipeline, this module is where
it'd plug in.
"""

from __future__ import annotations

from typing import Any


_ARCHETYPE_RULES = [
    # (archetype_label, min_matches, set_of_marker_cards)
    ("storm", 1, {
        "Grapeshot", "Tendrils of Agony", "Brain Freeze",
        "Empty the Warrens", "Aetherflux Reservoir",
        "Storm-Kiln Artist",
    }),
    ("cedh_turbo", 2, {
        "Thassa's Oracle", "Demonic Consultation", "Tainted Pact",
        "Ad Nauseam", "Dockside Extortionist",
        "Dramatic Reversal", "Isochron Scepter",
    }),
    ("artifact_ramp", 3, {
        "Sol Ring", "Mana Crypt", "Mana Vault",
        "Chrome Mox", "Mox Amber", "Mox Opal", "Mox Diamond",
        "Thran Dynamo", "Gilded Lotus", "Coalition Relic",
        "Arcane Signet", "Fellwar Stone", "Mind Stone",
    }),
    ("recursion_graveyard", 1, {
        "Muldrotha, the Gravetide", "Meren of Clan Nel Toth",
        "Karador, Ghost Chieftain", "The Scarab God",
        "Reanimate", "Animate Dead", "Necromancy",
    }),
    ("werewolf", 2, {
        "Ulrich of the Krallenhorde", "Tovolar, Dire Overlord",
        "Arlinn Kord", "Kessig Wolf Run",
        "Immerwolf", "Huntmaster of the Fells",
    }),
    ("voltron", 2, {
        "Sword of Feast and Famine", "Sword of Fire and Ice",
        "Umezawa's Jitte", "Shadowspear",
        "Embercleave", "Colossus Hammer",
    }),
    ("aristocrats_drain", 2, {
        "Blood Artist", "Zulaport Cutthroat",
        "Cruel Celebrant", "Syr Konrad, the Grim",
        "Bastion of Remembrance",
    }),
]


def classify_seat(card_names: list) -> str:
    """Return the best-matching archetype label for a decklist."""
    names = set(card_names)
    best = ("unknown", 0)
    for label, thresh, markers in _ARCHETYPE_RULES:
        hits = len(names & markers)
        if hits >= thresh and hits > best[1]:
            best = (label, hits)
    return best[0]


def classify_all(decks: dict) -> dict:
    """seat → archetype label."""
    out = {}
    for s in decks.get("seats", []):
        out[s["seat"]] = classify_seat(s.get("card_names", []))
    return out
