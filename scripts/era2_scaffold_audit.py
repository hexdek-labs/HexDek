#!/usr/bin/env python3
"""Era 2 (2015-2019) scaffold-gap audit.

Mirrors scripts/era1_scaffold_audit.py but filters for cards classified as
era 2 by classifyCardEra (partner, experience, eminence, energy, crew,
adapt, amass, afterlife, spectacle, riot).

Output: data/rules/era2_scaffold_audit.md  +  prints top-50 to stdout.
"""

from __future__ import annotations

import json
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT = ROOT / "data" / "rules" / "era2_scaffold_audit.md"

ERA4_KW = ["discover", "descend", "battle", "prototype", "craft",
           "role token", "finality counter", "the ring"]
ERA3_KW = ["daybound", "nightbound", "disturb", "cleave", "decayed",
           "exploit", "companion", "mutate", "foretell", "learn",
           "ward", "perpetual", "conjure"]
ERA2_KW = ["partner", "experience counter", "eminence", "energy counter",
           "crew", "adapt", "amass", "afterlife", "spectacle", "riot"]

TARGET_ERA = 2


def classify_era(oracle_text: str, type_line: str) -> int:
    text = (oracle_text or "").lower()
    types = (type_line or "").lower()
    for kw in ERA4_KW:
        if kw in text or kw in types:
            return 4
    for kw in ERA3_KW:
        if kw in text:
            return 3
    for kw in ERA2_KW:
        if kw in text:
            return 2
    return 1


BUCKETED_KINDS = {
    "fateful_hour", "life_threshold",
    "threshold", "card_count_zone",
    "metalcraft", "morbid", "ferocious", "revolt", "delirium",
    "devotion",
    "you_attacked_this_turn",
    "you_control",
    "paid_optional_cost", "for_each",
    "etb_as", "enters_as", "enters_with",
    "did_prior_action",
    "mana_spent",
    # Era 1 audit additions.
    "was_kicked",
    "hellbent", "raid", "attacked_this_turn",
    "spell_mastery", "gained_life_this_turn", "creature_died_this_turn",
    "no_spells_cast_last_turn", "two_plus_spells_cast_last_turn",
    "you_control_creature_power_ge",
    "etb_tapped_unless", "domain", "etb_if", "repeat_n",
    "lieutenant", "ki_counters_ge_2", "self_is_tapped",
    "attacked_or_blocked_this_combat", "coven", "self_has_counter",
    "didnt_attack_this_turn", "dealt_damage_to_opponent_this_turn",
    "no_mana_spent_to_cast",
    # Era 2 + Era 4 audit additions (dev/era2-era4-scaffolds branch).
    "landfall", "you_descended_this_turn",
    "it_was_a_creature", "no_creatures_on_battlefield",
}

RAW_KINDS = {"intervening_if", "as_long_as", "conditional", "raw", "if"}

# Era 2-flavored raw patterns: energy, crew, partner, vehicles, eminence,
# experience counters, afterlife, spectacle, riot, adapt, amass + the usual
# evergreen patterns carry over.
RAW_PATTERNS = [
    ("energy_payment", re.compile(r"pay (?:\{e\}|one energy|two energy|three energy|x energy)|spend.*energy")),
    ("energy_threshold", re.compile(r"(?:two|three|four|five|six|seven|eight) or more energy")),
    ("crew_paid", re.compile(r"crew (?:\d|n)|crewed this turn|was crewed|whenever.*crews")),
    ("vehicle_animated", re.compile(r"becomes? (?:an? )?(?:artifact )?creature|until end of turn.*vehicle")),
    ("partner_pair", re.compile(r"partner with|your partner")),
    ("eminence_command", re.compile(r"in the command zone|from the command zone")),
    ("experience_count", re.compile(r"experience counter")),
    ("afterlife_die", re.compile(r"afterlife|dies.*create.*spirit")),
    ("spectacle_cost", re.compile(r"spectacle|opponent lost life")),
    ("riot_choice", re.compile(r"riot|haste or a \+1/\+1")),
    ("adapt_n", re.compile(r"adapt \d|no \+1/\+1 counters on it")),
    ("amass_n", re.compile(r"amass \d|amass an? army|army you control")),
    ("vehicle_smith", re.compile(r"vehicle|vehicles")),
    ("history_matters", re.compile(r"historic|historic spell|legendary, an artifact, or a saga")),
    ("ascend_city", re.compile(r"ascend|city's blessing|you have the city's blessing")),
    ("raid_attacked", re.compile(r"raid|creature attacked this turn|attacked with a creature this turn")),
    ("revolt_perm_left", re.compile(r"revolt|permanent.*left the battlefield.*this turn")),
    ("delirium_types", re.compile(r"delirium|four or more card types")),
    ("metalcraft_three_art", re.compile(r"metalcraft|three or more artifacts")),
    ("ferocious_power4", re.compile(r"ferocious|creature with power (?:four|4)")),
    ("formidable", re.compile(r"formidable|total power (?:eight|8)")),
    ("inspired_untap", re.compile(r"inspired|whenever.*becomes untapped")),
    ("rebound_cast", re.compile(r"rebound|exile.*instead of.*graveyard.*beginning of your next upkeep")),
    ("strive_extra_target", re.compile(r"strive|for each target beyond the first")),
    ("constellation", re.compile(r"constellation|whenever an enchantment enters")),
    ("heroic_targeted", re.compile(r"heroic|whenever you cast a spell that targets")),
    ("inspired_untaps", re.compile(r"becomes untapped")),
    ("madness_cast", re.compile(r"madness|cast it for its madness")),
    ("emerge_sacrifice", re.compile(r"emerge|sacrifice a creature.*cast")),
    ("escalate_modes", re.compile(r"escalate|choose one or more")),
    ("kicker_paid", re.compile(r"kicked|kicker (?:cost )?was paid|multikicker|kicker \{")),
    ("surge_friend_cast", re.compile(r"surge|teammate has cast another spell")),
    ("awaken_land", re.compile(r"awaken|land you control becomes a")),
    ("rally_ally_etb", re.compile(r"rally|ally enters the battlefield")),
    # Era 2 scaffold additions (dev/era2-era4-scaffolds branch).
    ("velocity_counters", re.compile(r"velocity counter")),
    ("not_declared_attacker", re.compile(r"isn't being declared as an attacker|not declared as an attacker")),
    ("mana_value_le", re.compile(r"mana value (?:of \S+ )?is \d+ or (?:less|fewer)")),
    ("crewed_by_subtype", re.compile(r"an? \w+ crewed (?:it|this vehicle) this turn")),
    ("is_subtype", re.compile(r"that creature is an? \w+")),
    ("eminence_command_zone", re.compile(r"in the command zone|from the command zone")),
    # Era 4 raw matchers added so the cross-era Python audit reflects Go-side
    # bucketing for shared mechanics.
    ("planeswalker_etb_turn", re.compile(r"planeswalker (?:entered|enters) the battlefield.*this turn|planeswalker.*you've cast.*this turn")),
    ("artifact_etb_turn", re.compile(r"artifact (?:entered|enters) the battlefield.*this turn.*(?:under your control|you control)")),
    ("reveal_land_otherwise_hand", re.compile(r"if it's a land card.*(?:onto the battlefield|put it onto).*otherwise")),
    ("had_counters_on_it", re.compile(r"had (?:a |one or more |any |a \+1/\+1 |a death )?counters? on it")),
    ("you_cast_from_hand", re.compile(r"you cast it from your hand|you cast it from a graveyard|you cast it(?! from)")),
    ("still_on_battlefield", re.compile(r"it's on the battlefield|is still on the battlefield")),
    ("hellbent_no_hand", re.compile(r"hellbent|no cards in.*hand")),
    ("monarch", re.compile(r"the monarch|you('re| are) the monarch")),
    ("initiative", re.compile(r"the initiative|have initiative")),
    ("gained_life_turn", re.compile(r"(?:gained|gain) life.*this turn")),
    ("life_lost_turn", re.compile(r"(?:lost|lose) life.*this turn")),
    ("opponent_lost_life", re.compile(r"opponent.*(?:lost life|lose life|dealt damage).*this turn")),
    ("life_above", re.compile(r"(?:you have|your life total is) \d+ or more life")),
    ("life_below", re.compile(r"(?:you have|your life total is) \d+ or (?:less|fewer) life")),
    ("cast_spell_turn", re.compile(r"(?:cast a spell|cast a noncreature|cast an instant|you cast).*this turn")),
    ("second_spell_turn", re.compile(r"(?:second|third) (?:spell|creature|noncreature|instant|sorcery|artifact|enchantment)")),
    ("creature_etb_turn", re.compile(r"creature (?:entered|enters).*(?:this turn|battlefield)")),
    ("drawn_card_turn", re.compile(r"(?:drew|drawn|draw) a card.*this turn")),
    ("attacked_turn", re.compile(r"attacked this turn|you attacked|creature attacked")),
    ("sacrificed_turn", re.compile(r"sacrific.*this turn")),
    ("combat_damage", re.compile(r"combat damage.*(?:this turn|to a player|dealt)")),
    ("landfall_turn", re.compile(r"landfall|land.*entered|played a land.*this turn")),
    ("discarded_turn", re.compile(r"discard.*this turn")),
    ("enchanted_creature", re.compile(r"enchanted creature")),
    ("equipped_creature", re.compile(r"equipped creature")),
    ("any_player_phase", re.compile(r"(?:each player|each opponent).*(?:upkeep|end step)")),
    ("upkeep", re.compile(r"upkeep")),
    ("died_this_turn", re.compile(r"died this turn")),
    ("graveyard_creatures", re.compile(r"graveyard.*creature card|creatures in.*graveyard")),
    ("graveyard_card", re.compile(r"graveyard")),
    ("mana_spent_raw", re.compile(r"mana was (?:spent|paid)|amount of mana (?:spent|paid)|mana value of \S+ is \d+ or (?:greater|more)")),
    ("permanent_left_bf", re.compile(r"permanent left")),
    ("becomes_tapped", re.compile(r"becomes tapped|is tapped")),
    ("becomes_target", re.compile(r"becomes (?:the|a) target")),
    ("tokens_created", re.compile(r"tokens.*created this turn")),
    ("you_control_raw", re.compile(r"you control")),
]


def match_raw(text: str) -> str | None:
    t = text.lower()
    for name, rx in RAW_PATTERNS:
        if rx.search(t):
            return name
    return None


# ---------------------------------------------------------------------------
# Trigger-side bucketing — mirrors classifyTrigger + triggerConditionActions
# in cmd/hexdek-thor/conditional_setup.go. A trigger event is BUCKETED when
# (a) it matches one of the EVENT_EXACT slugs (Era 2 R60 additions), or
# (b) it satisfies one of the substring catches (cast/spell, enters, dies,
#     attack, combat damage, gain/lose life, draw, discard, leaves/ltb,
#     sacrific, phase).
# Anything else lands in trig_events_unbucketed for the report.
# ---------------------------------------------------------------------------

TRIGGER_EVENT_EXACT = {
    "when_you_do", "counters_put_on_self", "tribe_you_control_etb",
    "self_crews_vehicle", "you_whenever", "ally_etb", "ally_typed_etb_a",
    "ally_subtype_deal_damage", "self_and", "etb_or_another",
    "opp_creature_event", "self_saddles_mount", "becomes_tapped",
    "you_get_energy", "player_wins_coin_flip", "you_action",
    "becomes_crewed", "becomes_crewed_first", "opp_draw_card",
    # Phase slugs that don't need a substring match.
    "etb", "attacks", "deal_combat_damage", "deals_combat_damage", "phase",
}

TRIGGER_SUBSTRING_CATCHES = [
    "dies", "is put into a graveyard",
    "enters", "attack", "combat damage",
    # Era 2 R60 follow-up — underscore-form combat damage slugs
    # (combat_damage_player, group_combat_damage_player, etc.) the parser
    # canonicalizes from prose. The prose "combat damage" substring missed
    # them because the slug uses underscores. Adding "combat_damage" here
    # mirrors the Go-side change in classifyTrigger and is the single
    # largest gap-closer for Era 2 (32/59 = 54%).
    "combat_damage",
    "cast", "spell",
    "gain", "lose",  # paired with "life" check below
    "draw", "discard",
    "leaves", "ltb", "sacrific",
]

# Era 2 R60 follow-up — long-tail event slugs that map to existing scaffolds
# (see classifyTrigger in cmd/hexdek-thor/conditional_setup.go for routing).
# These are EXACT matches; the substring catches above don't see them.
TRIGGER_EXTRA_EXACT = {
    "die", "to_graveyard",   # → creature_dies
    "etb_as",                # → creature_etb
    "cycle",                 # → discard
    "block",                 # → attacks
    "coin_flip_result",      # → player_wins_coin_flip
    "lose_game",             # → sacrifice
    # Second-tier long tail — see classifyTrigger comment block for the
    # per-slug routing rationale.
    "beginning_of_ordinal_step",  # → upkeep
    "token_event",                # → creature_etb
    "nontoken_ally_event",        # → ally_etb
    "nontoken_creature_event",    # → creature_etb
    "compound_opp_tribe_event",   # → opp_creature_event
    "one_or_more_typed_event",    # → tribe_you_control_etb
    "ally_explore",               # → ally_etb
    "self_and_another",           # → self_and
    "conditional_state",          # → when_you_do
    "misc_when",                  # → when_you_do
    "spend_this_mana",            # → you_get_energy
}


def classify_trigger_event(event: str, phase: str) -> bool:
    """Returns True if classifyTrigger would route this event to a registered
    triggerConditionActions slug (i.e. the trigger is scaffold-bucketed)."""
    e = (event or "").lower().strip()
    p = (phase or "").lower().strip()
    if not e and not p:
        return False
    if e in TRIGGER_EVENT_EXACT or e in TRIGGER_EXTRA_EXACT:
        return True
    # Substring catches mirroring the Go switch.
    for kw in TRIGGER_SUBSTRING_CATCHES:
        if kw in e:
            # The "gain"/"lose" catches also require "life".
            if kw in ("gain", "lose") and "life" not in e:
                continue
            return True
    if p:
        return True  # phase events route to phase_<name> slug
    return False


def walk(node, conds, trigs):
    if isinstance(node, dict):
        t = node.get("__ast_type__")
        if t == "Condition":
            conds.append(node)
        elif t == "Trigger":
            trigs.append(node)
        for v in node.values():
            walk(v, conds, trigs)
    elif isinstance(node, list):
        for x in node:
            walk(x, conds, trigs)


def main():
    cond_kinds = Counter()
    cond_kinds_bucketed = Counter()
    cond_kinds_unbucketed = Counter()
    trig_events = Counter()
    trig_events_bucketed = Counter()
    trig_events_unbucketed = Counter()
    raw_buckets = Counter()
    raw_unbucketed_text = Counter()
    raw_unbucketed_examples = defaultdict(list)

    total_cards = 0
    target_cards = 0
    era_counts = Counter()

    with DATASET.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            row = json.loads(line)
            total_cards += 1
            era = classify_era(row.get("oracle_text", ""), row.get("type_line", ""))
            era_counts[era] += 1
            if era != TARGET_ERA:
                continue
            target_cards += 1
            ast = row.get("ast")
            if not ast:
                continue
            conds, trigs = [], []
            walk(ast, conds, trigs)
            name = row.get("name", "?")
            for c in conds:
                k = (c.get("kind") or "").lower()
                cond_kinds[k] += 1
                if k in BUCKETED_KINDS:
                    cond_kinds_bucketed[k] += 1
                elif k in RAW_KINDS:
                    args = c.get("args") or []
                    raw_txt = ""
                    if args and isinstance(args[0], str):
                        raw_txt = args[0]
                    bucket = match_raw(raw_txt) if raw_txt else None
                    if bucket:
                        raw_buckets[bucket] += 1
                        cond_kinds_bucketed[k] += 1
                    else:
                        cond_kinds_unbucketed[k] += 1
                        key = (raw_txt[:80] or "<empty>").lower()
                        raw_unbucketed_text[key] += 1
                        if len(raw_unbucketed_examples[key]) < 3:
                            raw_unbucketed_examples[key].append(name)
                else:
                    cond_kinds_unbucketed[k] += 1
            for tg in trigs:
                ev = (tg.get("event") or "").lower()
                ph = (tg.get("phase") or "").lower()
                trig_events[ev] += 1
                if classify_trigger_event(ev, ph):
                    trig_events_bucketed[ev] += 1
                else:
                    trig_events_unbucketed[ev] += 1

    total_conds = sum(cond_kinds.values())
    total_bucket = sum(cond_kinds_bucketed.values())
    total_unbucket = sum(cond_kinds_unbucketed.values())

    lines = []
    lines.append(f"# Era {TARGET_ERA} (2015-2019) Scaffold-Gap Audit\n")
    lines.append(f"- Total cards in dataset: **{total_cards}**\n")
    lines.append(f"- Era distribution: " +
                 ", ".join(f"era{e}={era_counts[e]}" for e in sorted(era_counts)) + "\n")
    lines.append(f"- Era {TARGET_ERA} cards: **{target_cards}**\n")
    lines.append(f"- Era {TARGET_ERA} Condition nodes: **{total_conds}** "
                 f"(bucketed {total_bucket}, unbucketed {total_unbucket}, "
                 f"{100.0*total_unbucket/max(1,total_conds):.1f}% gap)\n")
    total_trigs = sum(trig_events.values())
    total_trig_bucket = sum(trig_events_bucketed.values())
    total_trig_unbucket = sum(trig_events_unbucketed.values())
    lines.append(f"- Era {TARGET_ERA} Trigger nodes: **{total_trigs}** "
                 f"(bucketed {total_trig_bucket}, unbucketed {total_trig_unbucket}, "
                 f"{100.0*total_trig_unbucket/max(1,total_trigs):.1f}% gap)\n")

    lines.append("\n## Top unbucketed condition Kinds\n")
    for k, n in cond_kinds_unbucketed.most_common(60):
        lines.append(f"- `{k or '<empty>'}` × {n}")

    lines.append("\n## Top unbucketed raw-text fragments (kind in raw/intervening_if/as_long_as)\n")
    for txt, n in raw_unbucketed_text.most_common(60):
        ex = ", ".join(raw_unbucketed_examples[txt][:3])
        lines.append(f"- × {n}: `{txt}`  _(e.g. {ex})_")

    lines.append("\n## Bucketed condition Kinds (sanity)\n")
    for k, n in cond_kinds_bucketed.most_common(20):
        lines.append(f"- `{k}` × {n}")

    lines.append("\n## Top unbucketed trigger events\n")
    if not trig_events_unbucketed:
        lines.append("_(none — every Era 2 trigger event maps to a scaffold slug)_")
    for ev, n in trig_events_unbucketed.most_common(40):
        lines.append(f"- `{ev or '<empty>'}` × {n}")

    lines.append("\n## Top trigger events (bucketed + unbucketed)\n")
    for ev, n in trig_events.most_common(40):
        marker = "" if ev in trig_events_bucketed else " _(unbucketed)_"
        lines.append(f"- `{ev or '<empty>'}` × {n}{marker}")

    OUT.write_text("\n".join(lines))
    print("\n".join(lines))


if __name__ == "__main__":
    main()
