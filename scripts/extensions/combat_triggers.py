"""Combat-event trigger patterns.

Family: COMBAT TRIGGERS — abilities that fire from combat events
(declaring attackers/blockers, becoming blocked, dealing or being dealt
combat damage, and compound combat events).

Exported shape matches parser._TRIGGER_PATTERNS:

    TRIGGER_PATTERNS = [
        (compiled_regex, event_name, scope),
        ...
    ]

where `scope` is one of:
  - "self"         — the card itself is the actor (no capture group)
  - "actor"        — group(1) captures the actor (e.g. "a creature you control")
  - "all"          — any-player / global event (no specific actor)
  - "named"        — group(1) names a phase or similar label
  - "phase_named"  — phase trigger with explicit phase name in group(1)

Patterns are ORDERED from most specific -> most general. The parser's
first-match-wins loop relies on this ordering (e.g. "attacks alone" must
come before plain "attacks"; "blocks or becomes blocked" must come before
"blocks"; "deals combat damage to a player" before bare "deals combat
damage").

The card's own name is assumed to be pre-normalised to `~`. Common self
aliases that oracle text uses interchangeably with `~` are matched as
`(?:~|this creature|this permanent|this vehicle|this land|this artifact|
this enchantment|equipped creature|enchanted creature|this card)`.
"""

from __future__ import annotations

import re

# Self-aliases used by oracle text interchangeably with ~ (the printed name).
# Kept here, not as a top-level regex, so the patterns below read cleanly.
_SELF = (
    r"(?:~|this creature|this permanent|this vehicle|this land|"
    r"this artifact|this enchantment|this card|equipped creature|"
    r"enchanted creature)"
)

# Actor capture for "a creature you control", "another creature", etc.
# `[^,.]+?` + lazy so we don't swallow the rest of the sentence.
_ACTOR = r"([^,.]+?)"


TRIGGER_PATTERNS = [
    # ------------------------------------------------------------------
    # Compound combat triggers (must come before their singletons)
    # ------------------------------------------------------------------
    # "Whenever ~ attacks, is blocked, or blocks, ..."
    (re.compile(rf"^whenever {_SELF} attacks,?\s*is blocked,?\s*(?:or|and)\s+blocks", re.I),
     "attack_blocked_or_block", "self"),
    # "Whenever ~ attacks or blocks" / "When ~ attacks or blocks"
    (re.compile(rf"^when(?:ever)? {_SELF} attacks or blocks", re.I),
     "attack_or_block", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} attacks or blocks", re.I),
     "attack_or_block", "actor"),
    # "Whenever ~ enters or attacks" — the modern "haste-style" trigger
    (re.compile(rf"^when(?:ever)? {_SELF} enters or attacks", re.I),
     "enter_or_attack", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} enters or attacks", re.I),
     "enter_or_attack", "actor"),
    # "Whenever ~ blocks or becomes blocked by a creature"
    (re.compile(rf"^whenever {_SELF} blocks or becomes blocked(?: by [^,.]+)?", re.I),
     "block_or_becomes_blocked", "self"),
    (re.compile(rf"^whenever {_ACTOR} blocks or becomes blocked(?: by [^,.]+)?", re.I),
     "block_or_becomes_blocked", "actor"),

    # ------------------------------------------------------------------
    # Attack triggers
    # ------------------------------------------------------------------
    # Specializations first
    (re.compile(rf"^whenever {_SELF} attacks and isn['’]t blocked", re.I),
     "attack_unblocked", "self"),
    (re.compile(rf"^whenever {_ACTOR} attacks and isn['’]t blocked", re.I),
     "attack_unblocked", "actor"),
    (re.compile(rf"^whenever {_SELF} attacks alone", re.I),
     "attack_alone", "self"),
    (re.compile(rf"^whenever {_ACTOR} attacks alone", re.I),
     "attack_alone", "actor"),
    (re.compile(rf"^whenever {_SELF} attacks a player", re.I),
     "attack_player", "self"),
    (re.compile(rf"^whenever {_ACTOR} attacks a player", re.I),
     "attack_player", "actor"),
    (re.compile(rf"^whenever {_SELF} attacks a planeswalker", re.I),
     "attack_planeswalker", "self"),
    (re.compile(rf"^whenever {_ACTOR} attacks a planeswalker", re.I),
     "attack_planeswalker", "actor"),
    # Flavor riders — "while saddled", "while crewed", "while mounted"
    (re.compile(rf"^whenever {_SELF} attacks while saddled", re.I),
     "attack_while_saddled", "self"),
    (re.compile(rf"^whenever {_SELF} attacks while crewed", re.I),
     "attack_while_crewed", "self"),
    # Bare "attacks" — self + actor
    (re.compile(rf"^whenever {_SELF} attacks", re.I),
     "attack", "self"),
    (re.compile(rf"^whenever {_ACTOR} attacks", re.I),
     "attack", "actor"),

    # ------------------------------------------------------------------
    # Player-level attack declarations ("whenever you attack …")
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you attack with (?:one or more|two or more|three or more|\w+) (?:other )?creatures?[^,.]*",
                re.I), "you_attack_with", "self"),
    (re.compile(r"^whenever you attack a player", re.I), "you_attack_player", "self"),
    (re.compile(r"^whenever you attack", re.I), "you_attack", "self"),

    # ------------------------------------------------------------------
    # "Attacks you / attacks you or a planeswalker you control"
    # ------------------------------------------------------------------
    (re.compile(r"^whenever a creature attacks you or a planeswalker you control", re.I),
     "creature_attacks_you_or_pw", "self"),
    (re.compile(r"^whenever a creature attacks you", re.I),
     "creature_attacks_you", "self"),
    (re.compile(r"^whenever enchanted player is attacked", re.I),
     "enchanted_player_attacked", "self"),

    # ------------------------------------------------------------------
    # Block triggers
    # ------------------------------------------------------------------
    # "Blocks a creature with [filter]" / "blocks a creature"
    (re.compile(rf"^whenever {_SELF} blocks a creature(?: with [^,.]+)?", re.I),
     "block_creature", "self"),
    (re.compile(rf"^whenever {_ACTOR} blocks a creature(?: with [^,.]+)?", re.I),
     "block_creature", "actor"),
    (re.compile(rf"^whenever {_SELF} blocks", re.I), "block", "self"),
    (re.compile(rf"^whenever {_ACTOR} blocks", re.I), "block", "actor"),

    # ------------------------------------------------------------------
    # Becomes-blocked triggers (self + actor forms, "by a creature with X")
    # ------------------------------------------------------------------
    (re.compile(rf"^when(?:ever)? {_SELF} becomes blocked by a creature(?: with [^,.]+)?", re.I),
     "becomes_blocked_by", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} becomes blocked by a creature(?: with [^,.]+)?", re.I),
     "becomes_blocked_by", "actor"),
    (re.compile(rf"^when(?:ever)? {_SELF} becomes blocked", re.I),
     "becomes_blocked", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} becomes blocked", re.I),
     "becomes_blocked", "actor"),

    # ------------------------------------------------------------------
    # Combat-damage triggers (target specialisations first)
    # ------------------------------------------------------------------
    # "To a player or planeswalker / or battle"
    (re.compile(rf"^whenever {_SELF} deals combat damage to a player or planeswalker", re.I),
     "combat_damage_player_or_pw", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a player or planeswalker", re.I),
     "combat_damage_player_or_pw", "actor"),
    (re.compile(rf"^whenever {_SELF} deals combat damage to a player or battle", re.I),
     "combat_damage_player_or_battle", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a player or battle", re.I),
     "combat_damage_player_or_battle", "actor"),
    # To a player / opponent / planeswalker / creature / battle
    (re.compile(rf"^whenever {_SELF} deals combat damage to a player", re.I),
     "combat_damage_player", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a player", re.I),
     "combat_damage_player", "actor"),
    (re.compile(rf"^whenever {_SELF} deals combat damage to an opponent", re.I),
     "combat_damage_opponent", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to an opponent", re.I),
     "combat_damage_opponent", "actor"),
    (re.compile(rf"^whenever {_SELF} deals combat damage to a planeswalker", re.I),
     "combat_damage_planeswalker", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a planeswalker", re.I),
     "combat_damage_planeswalker", "actor"),
    (re.compile(rf"^whenever {_SELF} deals combat damage to a creature", re.I),
     "combat_damage_creature", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a creature", re.I),
     "combat_damage_creature", "actor"),
    (re.compile(rf"^whenever {_SELF} deals combat damage to a battle", re.I),
     "combat_damage_battle", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to a battle", re.I),
     "combat_damage_battle", "actor"),
    # "To target [filter]"
    (re.compile(rf"^whenever {_SELF} deals combat damage to target [^,.]+", re.I),
     "combat_damage_target", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage to target [^,.]+", re.I),
     "combat_damage_target", "actor"),
    # "To you"
    (re.compile(r"^whenever a creature deals combat damage to you", re.I),
     "creature_combat_damage_you", "self"),
    # Bare "deals combat damage"
    (re.compile(rf"^whenever {_SELF} deals combat damage", re.I),
     "combat_damage", "self"),
    (re.compile(rf"^whenever {_ACTOR} deals combat damage", re.I),
     "combat_damage", "actor"),

    # ------------------------------------------------------------------
    # "One or more creatures … deal combat damage to a player" (aggregate)
    # ------------------------------------------------------------------
    (re.compile(r"^whenever one or more ([^,.]+?) deal combat damage to a player", re.I),
     "group_combat_damage_player", "actor"),
    (re.compile(r"^whenever one or more ([^,.]+?) attack", re.I),
     "group_attack", "actor"),

    # ------------------------------------------------------------------
    # "Is dealt (combat) damage"
    # ------------------------------------------------------------------
    (re.compile(rf"^when(?:ever)? {_SELF} is dealt combat damage", re.I),
     "dealt_combat_damage", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} is dealt combat damage", re.I),
     "dealt_combat_damage", "actor"),
    (re.compile(rf"^when(?:ever)? {_SELF} is dealt damage", re.I),
     "dealt_damage", "self"),
    # actor form for "is dealt damage" already exists in core parser,
    # kept here for parity if loaded standalone:
    (re.compile(rf"^when(?:ever)? {_ACTOR} is dealt damage", re.I),
     "dealt_damage", "actor"),

    # ------------------------------------------------------------------
    # "Enters tapped and attacking"
    # ------------------------------------------------------------------
    (re.compile(r"^whenever a ([^,.]+?) enters tapped and attacking", re.I),
     "enter_tapped_attacking", "actor"),
    (re.compile(rf"^when(?:ever)? {_SELF} enters tapped and attacking", re.I),
     "enter_tapped_attacking", "self"),

    # ------------------------------------------------------------------
    # Combat-phase timing (for cards that pre-empt the existing
    # "at the beginning of" wildcard — keep for completeness)
    # ------------------------------------------------------------------
    (re.compile(r"^at the beginning of combat on your turn", re.I),
     "phase", "combat_start_yours"),
    (re.compile(r"^at the beginning of each combat", re.I),
     "phase", "combat_start_each"),
    (re.compile(r"^at end of combat", re.I),
     "phase", "end_of_combat"),
]


# ----------------------------------------------------------------------
# Effect rules: combat-flavored one-shots that commonly ride on combat
# triggers but don't get recognised as typed effects by the core parser.
# Shape mirrors how parser.parse_effect dispatches; each rule is
# (compiled_regex, effect_kind, optional_args_extractor). These are
# advisory — the parser can choose to import or ignore them.
# ----------------------------------------------------------------------

EFFECT_RULES = [
    # "Deal combat damage as though it weren't blocked"
    (re.compile(
        r"^(?:~|this creature|it)\s+deals combat damage (?:to defending player|to that player)?"
        r"\s*as though it weren['’]t blocked", re.I),
     "combat_damage_as_though_unblocked"),

    # "~ assigns combat damage equal to its toughness rather than its power"
    (re.compile(
        r"^(?:~|this creature) assigns (?:its )?combat damage equal to its toughness",
        re.I),
     "assign_damage_by_toughness"),

    # "~ can't be blocked except by [filter]"
    (re.compile(r"^(?:~|this creature) can['’]t be blocked except by [^,.]+", re.I),
     "unblockable_except_by"),

    # "~ can't attack or block alone"
    (re.compile(r"^(?:~|this creature) can['’]t attack or block alone", re.I),
     "cant_attack_or_block_alone"),

    # "~ attacks each combat if able" is already handled in parser.py,
    # but mirror the block-each-combat form here.
    (re.compile(r"^(?:~|this creature) blocks each (?:turn|combat) if able", re.I),
     "must_block_each_combat"),

    # "Prevent all combat damage that would be dealt [to/by] ~ this turn"
    (re.compile(
        r"^prevent all combat damage that would be dealt"
        r"(?: to | by )?(?:~|this creature)?\s*(?:this turn)?", re.I),
     "prevent_all_combat_damage"),
]


__all__ = ["TRIGGER_PATTERNS", "EFFECT_RULES"]


# ----------------------------------------------------------------------
# Lightweight self-check: when run directly, verify every pattern
# compiles and matches at least its own exemplar phrase. This is a
# smoke test only — it doesn't mutate the main parser.
# ----------------------------------------------------------------------

if __name__ == "__main__":
    samples = [
        "whenever ~ attacks, draw a card.",
        "whenever ~ attacks and isn't blocked, it gets +2/+0.",
        "whenever ~ attacks alone, it gets double strike.",
        "whenever ~ attacks a player, create a 1/1 soldier.",
        "whenever ~ attacks a planeswalker, draw a card.",
        "whenever ~ attacks while saddled, create a treasure.",
        "whenever a creature you control attacks, it gets +1/+0.",
        "whenever you attack, create a 1/1 goblin.",
        "whenever you attack a player, that player loses 1 life.",
        "whenever you attack with three or more creatures, draw a card.",
        "whenever a creature attacks you, it gets -1/-0.",
        "whenever a creature attacks you or a planeswalker you control, tap it.",
        "whenever enchanted player is attacked, draw a card.",
        "whenever ~ blocks, draw a card.",
        "whenever ~ blocks a creature with flying, it gets +2/+2.",
        "whenever ~ blocks or becomes blocked, it gets +1/+1.",
        "whenever ~ blocks or becomes blocked by a creature, draw a card.",
        "when ~ becomes blocked, create a treasure.",
        "when ~ becomes blocked by a creature with power 3 or greater, draw a card.",
        "whenever ~ deals combat damage to a player, draw a card.",
        "whenever ~ deals combat damage to an opponent, that player loses 2 life.",
        "whenever ~ deals combat damage to a planeswalker, draw a card.",
        "whenever ~ deals combat damage to a creature, draw a card.",
        "whenever ~ deals combat damage to a player or planeswalker, draw a card.",
        "whenever ~ deals combat damage to a player or battle, draw a card.",
        "whenever ~ deals combat damage to target creature, exile it.",
        "whenever ~ deals combat damage, create a treasure.",
        "whenever a creature deals combat damage to you, you lose 1 life.",
        "whenever one or more creatures you control deal combat damage to a player, draw a card.",
        "whenever one or more creatures you control attack, scry 1.",
        "whenever ~ is dealt damage, draw a card.",
        "whenever ~ is dealt combat damage, create a 1/1.",
        "whenever a goblin enters tapped and attacking, it gets +1/+0.",
        "whenever ~ enters or attacks, scry 1.",
        "whenever ~ attacks or blocks, it gets +1/+1.",
        "whenever ~ attacks, is blocked, or blocks, it gets +1/+1.",
        "at the beginning of combat on your turn, create a 1/1.",
        "at end of combat, sacrifice ~.",
    ]

    unmatched = []
    for s in samples:
        hit = None
        for pat, event, scope in TRIGGER_PATTERNS:
            if pat.match(s.rstrip(".")):
                hit = (event, scope)
                break
        if hit is None:
            unmatched.append(s)
        else:
            print(f"OK  [{hit[0]:<30s} {hit[1]:<6s}]  {s}")

    if unmatched:
        print("\nUNMATCHED:")
        for s in unmatched:
            print(" ", s)
        raise SystemExit(1)
    print(f"\nAll {len(samples)} samples matched.")
