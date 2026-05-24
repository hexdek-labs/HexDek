#!/usr/bin/env python3
"""Era 1 (1993-2014) scaffold-gap audit.

Walks data/rules/ast_dataset.jsonl, classifies each card by era using the
same heuristics as cmd/hexdek-thor/corpus_audit.go :: classifyCardEra,
recursively collects every Condition and Trigger node, and emits a histogram
of Condition.kind / Trigger.event values for the era-1 slice.

Conditions whose kind is in the canonical-scaffold set are tagged BUCKETED;
raw-text conditions (intervening_if / as_long_as / conditional / raw) are
opportunistically matched against the detectConditionScaffold patterns to
estimate the bucketed share. Everything else lands in the unbucketed
histogram.

Output: data/rules/era1_scaffold_audit.md  +  prints top-50 to stdout.
"""

from __future__ import annotations

import json
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT = ROOT / "data" / "rules" / "era1_scaffold_audit.md"

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
    # Era 4 carry-over (already bucketed in Go scaffold).
    "landfall", "you_descended_this_turn",
    "it_was_a_creature", "no_creatures_on_battlefield",
    # Era 1 R60 sweep additions — 19 new structured-Kind scaffolds.
    "tribute_not_paid", "tribute_wasnt_paid",
    "bargained", "was_bargained",
    "shares_creature_type",
    "not_their_turn", "not_your_turn",
    "more_life_than_opponent", "you_more_life",
    "no_time_counters", "time_counter_on_self",
    "wasnt_cast",
    "lost_life_last_turn",
    "damage_dealt_to_self_this_turn", "damage_taken_this_turn",
    "more_cards_in_hand_than_opponents", "more_cards_than_each_opponent",
    "self_is_untapped",
    "self_has_no_counter", "no_named_counter",
    "surge_cost_paid",
    "library_empty",
    "colored_mana_spent",
    "self_is_suspended",
    "life_above_starting",
    "opponent_more_life",
    "wasnt_blocking",
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
    # Era 1 R60 sweep — 19 new raw-text patterns. Ordered most-specific first
    # so the broader matchers below (life/hand/cast/you_control) don't eat
    # them. Mirrors the detectConditionScaffold ordering in conditional_setup.go.
    ("tribute_not_paid", re.compile(r"tribute (?:wasn't|was) paid")),
    ("bargained", re.compile(r"it was bargained|this spell was bargained|if it was bargained")),
    ("shares_creature_type", re.compile(r"shares a creature type")),
    ("time_counter_on_self", re.compile(r"no time counters|had no time counters")),
    ("self_is_suspended", re.compile(r"this card is suspended|while it'?s suspended|while suspended")),
    ("wasnt_cast", re.compile(r"(?:it|he|she|they) (?:wasn't|weren't) cast|wasn't cast")),
    ("wasnt_blocking", re.compile(r"wasn't blocking|wasn't being blocked by")),
    ("surge_cost_paid", re.compile(r"surge cost was paid")),
    ("library_empty", re.compile(r"no cards in.*library|library has no cards|library is empty")),
    ("colored_mana_spent", re.compile(r"\{[wubrgc]\}(?:\{[wubrgc]\})?\s*was spent to cast|mana from a treasure was spent to cast")),
    ("lost_life_last_turn", re.compile(r"lost life last turn|last turn.*(?:lost|lose) life")),
    ("damage_dealt_to_self_turn", re.compile(r"damage was dealt to (?:it|this).*this turn|damage dealt to it.*this turn")),
    ("more_cards_than_opponents", re.compile(r"more cards in hand than each opponent|more cards in.*hand.*than (?:each|any)")),
    ("self_is_untapped", re.compile(r"this creature is untapped|~ is untapped|(?:this artifact|this permanent) is untapped")),
    ("self_has_no_counter", re.compile(r"has no \S+ counters? on it|had no \S+ counters? on it")),
    ("life_above_starting", re.compile(r"(?:greater|more) than your starting life total|above your starting life total")),
    ("opponent_more_life", re.compile(r"(?:an? |any )?opponent has more life(?: than you)?")),
    ("more_life_than_opponent", re.compile(r"more life than (?:an?|each) opponent|no opponent has more life than (?:that player|you)")),
    ("not_their_turn", re.compile(r"not their turn|not your turn|isn't their turn|during another player'?s turn|on a turn that isn'?t yours")),
    # Era 4 carry-over (already bucketed in Go scaffold): you cast it / had
    # counters / planeswalker-ETB / still-on-battlefield. These weren't in
    # era1's pattern list, depressing the bucketed share.
    ("had_counters_on_it", re.compile(r"had (?:a |one or more |any |a \+1/\+1 |a death )?counters? on it")),
    ("you_cast_from_hand", re.compile(r"you cast it from your hand|you cast it from a graveyard|you cast it(?! from)")),
    ("planeswalker_etb_turn", re.compile(r"planeswalker (?:entered|enters) the battlefield.*this turn|planeswalker.*you've cast.*this turn")),
    ("artifact_etb_turn", re.compile(r"artifact (?:entered|enters) the battlefield.*this turn.*(?:under your control|you control)")),
    ("still_on_battlefield", re.compile(r"it's on the battlefield|is still on the battlefield")),
    ("reveal_land_otherwise_hand", re.compile(r"if it's a land card.*(?:onto the battlefield|put it onto).*otherwise")),
    ("you_control_raw", re.compile(r"you control")),
]


def match_raw(text: str) -> str | None:
    t = text.lower()
    for name, rx in RAW_PATTERNS:
        if rx.search(t):
            return name
    return None


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
    raw_buckets = Counter()
    raw_unbucketed_text = Counter()
    raw_unbucketed_examples = defaultdict(list)

    total_cards = 0
    era1_cards = 0
    era_counts = Counter()

    with DATASET.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            row = json.loads(line)
            total_cards += 1
            era = classify_era(row.get("oracle_text", ""), row.get("type_line", ""))
            era_counts[era] += 1
            if era != 1:
                continue
            era1_cards += 1
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
                trig_events[ev] += 1

    total_conds = sum(cond_kinds.values())
    total_bucket = sum(cond_kinds_bucketed.values())
    total_unbucket = sum(cond_kinds_unbucketed.values())

    lines = []
    lines.append("# Era 1 (1993-2014) Scaffold-Gap Audit\n")
    lines.append(f"- Total cards in dataset: **{total_cards}**\n")
    lines.append(f"- Era distribution: " +
                 ", ".join(f"era{e}={era_counts[e]}" for e in sorted(era_counts)) + "\n")
    lines.append(f"- Era 1 cards: **{era1_cards}**\n")
    lines.append(f"- Era 1 Condition nodes: **{total_conds}** "
                 f"(bucketed {total_bucket}, unbucketed {total_unbucket}, "
                 f"{100.0*total_unbucket/max(1,total_conds):.1f}% gap)\n")
    lines.append(f"- Era 1 Trigger nodes: **{sum(trig_events.values())}**\n")

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

    lines.append("\n## Top trigger events\n")
    for ev, n in trig_events.most_common(40):
        lines.append(f"- `{ev or '<empty>'}` × {n}")

    OUT.write_text("\n".join(lines))
    print("\n".join(lines))


if __name__ == "__main__":
    main()
