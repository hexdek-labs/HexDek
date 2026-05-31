#!/usr/bin/env python3
"""Era 3 (2020-2022) scaffold-gap audit.

Walks data/rules/ast_dataset.jsonl, classifies each card by era using the
same heuristics as cmd/hexdek-thor/corpus_audit.go :: classifyCardEra,
recursively collects every Condition and Trigger node, and emits a histogram
of Condition.kind / Trigger.event values for the era-3 slice.

Era 3 markers (companion, mutate, foretell, learn, ward, daybound/nightbound,
disturb, cleave, decayed, exploit, perpetual, conjure) drive cards into this
bucket via classifyCardEra; everything earlier-falling lands in era 1 or 2.

Conditions whose kind is in the canonical-scaffold set are tagged BUCKETED;
raw-text conditions (intervening_if / as_long_as / conditional / raw) are
opportunistically matched against the detectConditionScaffold patterns to
estimate the bucketed share. Everything else lands in the unbucketed
histogram.

Output: data/rules/era3_scaffold_audit.md  +  prints top-50 to stdout.
"""

from __future__ import annotations

import json
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT = ROOT / "data" / "rules" / "era3_scaffold_audit.md"
TARGET_ERA = 3

# ---------------------------------------------------------------------------
# Era classifier — mirrors classifyCardEra in cmd/hexdek-thor/corpus_audit.go.
# ---------------------------------------------------------------------------

ERA4_KW = ["discover", "descend", "battle", "prototype", "craft",
           "role token", "finality counter", "the ring"]
ERA3_KW = ["daybound", "nightbound", "disturb", "cleave", "decayed",
           "exploit", "companion", "mutate", "foretell", "learn",
           "ward", "perpetual", "conjure"]
ERA2_KW = ["partner", "experience counter", "eminence", "energy counter",
           "crew", "adapt", "amass", "afterlife", "spectacle", "riot"]


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

# ---------------------------------------------------------------------------
# Canonical scaffold-bucketed condition kinds (setupCondition switch +
# detectConditionScaffold structured-Kind switch).
# ---------------------------------------------------------------------------

BUCKETED_KINDS = {
    # setupCondition switch
    "fateful_hour", "life_threshold",
    "threshold", "card_count_zone",
    "metalcraft", "morbid", "ferocious", "revolt", "delirium",
    "devotion",
    "you_attacked_this_turn",
    "you_control",
    # Tier 1 structured
    "paid_optional_cost", "for_each",
    "etb_as", "enters_as", "enters_with",
    "did_prior_action",
    "mana_spent",
    # Era 1 audit additions (dev/era1-scaffolds branch).
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
    # Era 3 audit additions (dev/era3-scaffolds branch).
    "control_n_creatures", "you_control_n_creatures",
    "control_n_lands", "you_control_n_lands",
    "counter_doubler", "counter_replacement_boost",
    "token_doubler", "token_replacement_boost",
    "persist_check", "undying_check", "no_counters",
    "revealed_card_type", "card_type_reveal",
    "equipped", "equipment_attached",
    "hand_size_le", "hand_size_lt", "hand_size_ge", "hand_size_threshold",
    "put_counter_this_turn",
    "cast_n_spells_this_turn",
    "full_party",
    "total_toughness",
    "main_phase", "first_combat_phase",
    # Era 3 sweep r60 additions (dev/era3-scaffold-sweep-r60 branch).
    "discarded_nonland",
    "control_n_tapped_creatures",
    "gained_n_life_this_turn",
    "drawn_n_cards_this_turn",
    "starting_player", "you_were_starting_player",
    "quest_counters",
    "dragon_beheld",
}

RAW_KINDS = {"intervening_if", "as_long_as", "conditional", "raw", "if"}

# Raw-text detector approximation — only enumerates the TOP patterns from
# conditional_setup.go. Anything not matched lands as unbucketed-raw with
# its raw text recorded for manual inspection.
RAW_PATTERNS = [
    ("kicker_was_paid", re.compile(r"was kicked|kicker (?:cost )?was paid|if (?:it|this) was kicked|multikicker|for each time .* was kicked")),
    ("opponent_more_lands", re.compile(r"more land.*than you|controls more.*than you")),
    ("died_this_turn", re.compile(r"died this turn")),
    ("delirium", re.compile(r"delirium|four or more card types")),
    ("spell_mastery", re.compile(r"spell mastery|(?:two|2) or more instant.*graveyard|(?:two|2) or more sorcer")),
    ("graveyard_creatures", re.compile(r"graveyard.*creature card|creatures in.*graveyard")),
    ("graveyard_card", re.compile(r"graveyard")),
    ("energy", re.compile(r"energy")),
    ("gained_life_turn", re.compile(r"(?:gained|gain) life.*this turn")),
    ("cast_spell_turn", re.compile(r"(?:cast a spell|cast a noncreature|cast an instant|you cast).*this turn")),
    ("creature_etb_turn", re.compile(r"creature (?:entered|enters).*(?:this turn|battlefield)")),
    ("drawn_card_turn", re.compile(r"(?:drew|drawn|draw) a card.*this turn")),
    ("attacked_turn", re.compile(r"attacked this turn|you attacked|creature attacked")),
    ("sacrificed_turn", re.compile(r"sacrific.*this turn")),
    ("combat_damage", re.compile(r"combat damage.*(?:this turn|to a player|dealt)")),
    ("landfall_turn", re.compile(r"landfall|land.*entered|played a land.*this turn")),
    ("discarded_turn", re.compile(r"discard.*this turn")),
    ("enchanted_creature", re.compile(r"enchanted creature")),
    ("opponent_lost_life", re.compile(r"opponent.*(?:lost life|lose life|dealt damage).*this turn")),
    ("life_above", re.compile(r"(?:you have|your life total is) \d+ or more life")),
    ("life_below", re.compile(r"(?:you have|your life total is) \d+ or (?:less|fewer) life")),
    ("any_player_phase", re.compile(r"(?:each player|each opponent).*(?:upkeep|end step)")),
    ("delayed_draw_next_upkeep", re.compile(r"next turn.*upkeep.*(?:draw|upkeep)")),
    ("upkeep", re.compile(r"upkeep")),
    ("hellbent", re.compile(r"hellbent|no cards in.*hand")),
    ("monarch", re.compile(r"the monarch|you('re| are) the monarch")),
    ("initiative", re.compile(r"the initiative|have initiative")),
    ("revolt_raw", re.compile(r"revolt|permanent.*left the battlefield.*this turn")),
    ("metalcraft_raw", re.compile(r"metalcraft|three or more artifacts")),
    ("ferocious_raw", re.compile(r"ferocious|creature with power (?:four|4)")),
    ("formidable", re.compile(r"formidable|total power (?:eight|8)")),
    ("permanent_left_bf", re.compile(r"permanent left")),
    ("second_spell_turn", re.compile(r"(?:second|third) (?:spell|creature|noncreature|instant|sorcery|artifact|enchantment)")),
    ("descended_turn", re.compile(r"descend")),
    ("life_lost_turn", re.compile(r"(?:lost|lose) life.*this turn")),
    ("tokens_created", re.compile(r"tokens.*created this turn")),
    ("cast_from_exile", re.compile(r"cast.*from exile")),
    ("exile_linked", re.compile(r"exiled with.*(?:return|leaves)")),
    ("cycled", re.compile(r"cycle|cycling")),
    ("mutates", re.compile(r"mutate")),
    ("unlock_door", re.compile(r"unlock.*(?:door|this room|this enchantment)")),
    ("prior_turn_spell_count", re.compile(r"last turn.*(?:no spells|no spell was cast|cast two or more spells)")),
    ("paired_soulbond", re.compile(r"soulbond|(?:is|are) paired")),
    ("turned_face_up", re.compile(r"turned face up")),
    ("beginning_of_step", re.compile(r"beginning of (?:combat|each combat|.*draw step|.*end step|.*main phase|.*untap step)")),
    ("tribe_etb", re.compile(r"(?:another|a|an) \w+.*(?:enters|is put onto).*(?:under your control|you control)")),
    ("mana_spent_raw", re.compile(r"mana was (?:spent|paid)|amount of mana (?:spent|paid)|mana value of \S+ is \d+ or (?:greater|more)")),
    ("becomes_tapped", re.compile(r"becomes tapped|is tapped")),
    ("becomes_target", re.compile(r"becomes (?:the|a) target")),
    ("until_eot_delayed", re.compile(r"until end of turn.*(?:whenever|delayed)|next cleanup step")),
    ("land_play_or_tap", re.compile(r"plays a land|tapped for mana")),
    # Era 3 audit additions — text-form fallbacks for the new structured
    # Kinds above. Order matters: more-specific patterns first so they
    # don't get eaten by the broader "you_control_raw" sweep.
    ("counter_doubler", re.compile(r"(one or more.*counters.*plus one|that many plus one.*counter|twice that many counters)")),
    ("token_doubler", re.compile(r"(create.*tokens?.*(?:plus one|twice that many)|one or more tokens?.*plus one)")),
    ("persist_undying_check", re.compile(r"had no (?:\+1/\+1|-1/-1|counters)")),
    ("card_type_reveal", re.compile(r"if it's a (?:land|creature|permanent|instant|sorcery|artifact|enchantment) card|is a (?:creature|land|permanent|instant|sorcery|artifact|enchantment) card")),
    ("equipment_attached", re.compile(r"is equipped|equipped creature|while equipped")),
    ("hand_size_threshold", re.compile(r"(fewer than|no more than|or more|or fewer).*cards in.*hand")),
    ("put_counter_this_turn", re.compile(r"(you put|was put on|have put).*counter.*this turn")),
    ("cast_n_spells_this_turn", re.compile(r"you('ve| have)? cast.*(?:two|three|four|\d+).*spells.*this turn")),
    ("full_party", re.compile(r"full party")),
    ("total_toughness", re.compile(r"total toughness")),
    ("main_phase_first_combat", re.compile(r"main phase|first combat phase")),
    ("control_n_tapped_creatures", re.compile(r"(?:you )?control.*(?:two|three|four|\d+).*or more.*tapped creature")),
    ("control_n_creatures", re.compile(r"you control.*(?:two|three|four|five|\d+).*or more.*creature")),
    ("control_n_lands", re.compile(r"you control.*(?:two|three|four|five|six|seven|eight|\d+).*or more.*lands?")),
    # Era 3 r60 — text-form fallbacks for the new structured Kinds above.
    ("discarded_nonland", re.compile(r"discarded a nonland|discarded.*nonland card")),
    ("drawn_n_cards_this_turn", re.compile(r"(?:you've|you have) drawn.*or more.*cards.*this turn")),
    ("gained_n_life_this_turn", re.compile(r"gained.*(?:or more|at least).*life this turn")),
    ("starting_player", re.compile(r"starting player")),
    ("quest_counters", re.compile(r"quest counter")),
    ("dragon_beheld", re.compile(r"(?:dragon )?was beheld")),
    # Era 3 r60 sweep — raw-text fragments surfaced by the 11 unbucketed
    # Era 3 conditions. Order matters; specific patterns first.
    ("judgment_counter", re.compile(r"judgment counter")),
    ("was_bargained", re.compile(r"(?:if|was) bargained")),
    ("saddled_and_damage", re.compile(r"(?:is |was )?saddled and a creature was dealt")),
    ("excess_damage", re.compile(r"excess damage")),
    ("perpetual_effect", re.compile(r"perpetually (?:gains|loses|has)")),
    ("didnt_have_keyword", re.compile(r"(?:didn't|did not) have (?:decayed|deathtouch|flying|haste|hexproof|reach|trample|vigilance)")),
    ("you_cast_it", re.compile(r"\byou cast it\b")),
    ("cast_creature_and_noncreature", re.compile(r"(?:cast (?:both )?a creature spell and a noncreature spell)")),
    # Parser-coverage R60 batch — named-counter threshold cluster.
    # Mirror of the era1 RAW_PATTERNS addition.
    ("named_counter_threshold_on_perm",
     re.compile(r"(?:\d+|one|two|three|four|five|six|seven|eight|nine|ten|thirteen|twenty) or more \w+ counters? on (?:it|this artifact|this enchantment|this creature|this permanent|them|that)|there are (?:\d+|one|two|three|four|five|six|seven|eight|nine|ten) or more \w+ counters? on (?:it|this)")),
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
# in cmd/hexdek-thor/conditional_setup.go. Ported from scripts/era2_scaffold_audit.py
# so the era3 sweep can report the same trigger gap metric.
# ---------------------------------------------------------------------------

TRIGGER_EVENT_EXACT = {
    "when_you_do", "counters_put_on_self", "tribe_you_control_etb",
    "self_crews_vehicle", "you_whenever", "ally_etb", "ally_typed_etb_a",
    "ally_subtype_deal_damage", "self_and", "etb_or_another",
    "opp_creature_event", "self_saddles_mount", "becomes_tapped",
    "you_get_energy", "player_wins_coin_flip", "you_action",
    "becomes_crewed", "becomes_crewed_first", "opp_draw_card",
    "etb", "attacks", "deal_combat_damage", "deals_combat_damage", "phase",
    # Era 3 r60 sweep — 5 new scaffolds.
    "mutates", "turned_face_up", "exploits_creature",
    "specialize_creature", "unlock_door",
}

TRIGGER_SUBSTRING_CATCHES = [
    "dies", "is put into a graveyard",
    "enters", "attack", "combat damage",
    "combat_damage",   # underscore form — Era 2 r60 sweep
    "cast", "spell",
    "gain", "lose",   # paired with "life"
    "draw", "discard",
    "leaves", "ltb", "sacrific",
]

TRIGGER_EXTRA_EXACT = {
    # Era 2 r60 sweep — long-tail event slugs routed to existing scaffolds.
    "die", "to_graveyard",   # → creature_dies
    "etb_as",                # → creature_etb
    "cycle",                 # → discard
    "block",                 # → attacks
    "coin_flip_result",      # → player_wins_coin_flip
    "lose_game",             # → sacrifice
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
    # Era 3 r60 sweep — routed to existing scaffolds.
    "face_up_as", "as_transform",              # → turned_face_up
    "ally_exploits",                            # → exploits_creature
    "fully_unlock_room",                        # → unlock_door
    "ally_targeted_by_opp",                     # → attacks (becomes_target)
    "becomes_blocked",                          # → attacks
    "dealt_damage", "deals_damage",
    "damage_prevented_this_way", "ally_source_damage",  # → combat_damage
    "remove_counter", "counter_put_on_actor",
    "counters_put_on_self_any", "counters_put_on_actor",
    "creature_modified_event",                  # → counters_put_on_self
    "card_put_into_zone", "permanent_to_gy",
    "card_milled_via", "compound_card_zone_event",      # → creature_dies
    "foretell_card",                            # → cast_spell
    "attached_as", "equipped_trigger",
    "day_night_flip", "transforms",
    "next_time_one_or_more_enter",              # → creature_etb
    "cycle_card",                               # → discard
    "you_commit_crime", "commit_crime",
    "pay_cost_multiple", "misc_whenever_a",
    "you_conjure_one_or_more", "you_mechanic",  # → when_you_do
    "self_or_another_when",                     # → etb_or_another
    "becomes_state",                            # → becomes_tapped_trigger
    "becomes_target",                           # → attacks
    "player_land_play",                         # → upkeep (landfall-style)
    # Parser-coverage R60 batch — `*_to_gy_from_bf` + Era-4 long-tail
    # slugs. Mirrors the classifyTrigger additions in conditional_setup.go.
    "self_put_into_graveyard_from_bf",
    "ally_type_to_gy_from_bf", "type_to_gy_from_bf",
    "to_gy_from_bf", "opp_type_to_gy_from_bf",
    "ally_typed_to_gy", "tribal_to_gy_from_bf",
    "nontoken_type_to_gy", "opp_creature_to_gy",
    "self_to_gy", "self_die_or_ally_gy",
    "specialize_from_zone",
    "ring_tempts_you", "train",
    "modified_creature_event",
    "face_down_creature_event",
    "compound_tribe_enter",
    "it_state_change",
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
    for kw in TRIGGER_SUBSTRING_CATCHES:
        if kw in e:
            if kw in ("gain", "lose") and "life" not in e:
                continue
            return True
    if p:
        return True
    return False


# ---------------------------------------------------------------------------
# AST walker.
# ---------------------------------------------------------------------------

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
    lines.append("# Era 3 (2020-2022) Scaffold-Gap Audit\n")
    lines.append(f"- Total cards in dataset: **{total_cards}**\n")
    lines.append(f"- Era distribution: " +
                 ", ".join(f"era{e}={era_counts[e]}" for e in sorted(era_counts)) + "\n")
    lines.append(f"- Era 3 cards: **{target_cards}**\n")
    lines.append(f"- Era 3 Condition nodes: **{total_conds}** "
                 f"(bucketed {total_bucket}, unbucketed {total_unbucket}, "
                 f"{100.0*total_unbucket/max(1,total_conds):.1f}% gap)\n")
    total_trigs = sum(trig_events.values())
    total_trig_bucket = sum(trig_events_bucketed.values())
    total_trig_unbucket = sum(trig_events_unbucketed.values())
    lines.append(f"- Era 3 Trigger nodes: **{total_trigs}** "
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
        lines.append("_(none — every Era 3 trigger event maps to a scaffold slug)_")
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
