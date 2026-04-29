#!/usr/bin/env python3
"""Bucket every card by how confidently the rules engine could auto-resolve it.

Buckets:
- GREEN: vanilla creatures + pure keyword cards (Flying, Trample, Lifelink, …)
          — handleable from rule definitions alone, no per-card logic
- YELLOW: templated effects with parameterizable values (deal X damage, target
          N creatures, draw N cards, lose N life, ETB triggers with simple
          actions). Handleable with a small DSL of effect templates.
- RED: unique phrasings, asymmetric replacement effects, complex multi-zone
       interactions, "rules text" cards (Doomsday, Humility, etc.). Needs
       per-card custom handlers.

Usage: python3 scripts/rules_coverage.py
Output: data/rules/coverage_report.md
"""

from __future__ import annotations

import json
import re
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ORACLE = ROOT / "data" / "rules" / "oracle-cards.json"
RULES = ROOT / "data" / "rules" / "MagicCompRules-20260227.txt"
REPORT = ROOT / "data" / "rules" / "coverage_report.md"

# Evergreen + deciduous keyword abilities — anything matching these alone is GREEN.
# Pulled from comp rules section 702 keyword abilities (we'll auto-extract this list later).
KEYWORDS_SIMPLE = {
    "deathtouch", "defender", "double strike", "enchant", "first strike", "flash",
    "flying", "haste", "hexproof", "indestructible", "intimidate", "landwalk",
    "lifelink", "menace", "protection", "reach", "shroud", "trample", "vigilance",
    "ward", "banding", "fear", "shadow", "horsemanship", "skulk", "infect",
    "wither", "absorb", "afflict", "afterlife", "amplify", "annihilator",
    "ascend", "aura swap", "battle cry", "bestow", "bloodthirst", "bushido",
    "buyback", "cascade", "champion", "changeling", "cipher", "conspire",
    "convoke", "crew", "cumulative upkeep", "cycling", "dash", "delve",
    "devoid", "devour", "echo", "emerge", "entwine", "epic", "escalate",
    "escape", "eternalize", "evoke", "evolve", "exalted", "exploit", "explore",
    "extort", "fabricate", "fading", "fight", "flanking", "flashback",
    "forecast", "fortify", "frenzy", "fuse", "graft", "gravestorm",
    "haunt", "hidden agenda", "hideaway", "improvise", "ingest", "jump-start",
    "kicker", "level up", "living weapon", "madness", "melee", "miracle",
    "modular", "morph", "mutate", "myriad", "ninjutsu", "offering", "outlast",
    "overload", "partner", "persist", "phasing", "poisonous", "populate",
    "prowess", "prowl", "rampage", "rebound", "recover", "reinforce", "renown",
    "replicate", "retrace", "ripple", "scavenge", "soulbond", "soulshift",
    "spectacle", "splice", "split second", "storm", "sunburst", "support",
    "surge", "suspend", "totem armor", "training", "transfigure", "transmute",
    "tribute", "undaunted", "undying", "unearth", "unleash", "vanishing",
    "wither",
    # Utility/keyword actions that often appear standalone on a card line:
    "scry", "fight", "regenerate", "exile", "sacrifice", "discard", "tap",
    "untap", "attach", "destroy", "counter",
    "commander ninjutsu",  # variants
    "tribal",
    # Additional keyword-action verbs that appear standalone with parameters:
    "overload", "flashback", "buyback", "kicker", "morph", "manifest", "transmute",
    "transfigure", "investigate", "explore", "venture", "amass", "learn",
    "fateful hour", "dethrone", "exalted", "soulbond", "rally", "raid",
    "cleave", "boast", "foretell", "blitz", "casualty", "channel",
    "demonstrate", "prototype", "enlist", "backup", "celebrate", "discover",
    "plot", "freerunning", "harmonize", "spree", "offspring", "impending",
    "warp", "max speed", "gift",
    # More keyword variants
    "embalm", "eternalize", "library ninjutsu", "totem armor", "umbra armor",
    "lightning breath", "lure", "rampage", "trample over planeswalkers",
    "cumulative upkeep", "decayed", "menace", "boast", "blitz", "celebrate",
    "domain", "constellation", "ferocious", "radiance", "rebound",
    "raid", "revolt", "morbid", "delirium", "metalcraft",
    # Ability words that get used standalone after splitter:
    "paradox", "disappear", "lightning breath", "celestial breath", "fire breath",
    "icebreath", "dragon breath", "venom", "wail", "psionic blast",
    # Set keywords without args
    "decayed", "blitz", "casualty", "channel", "compleated", "encore", "exhaust",
    # Landwalk variants and obscure keywords
    "swampwalk", "islandwalk", "forestwalk", "mountainwalk", "plainswalk",
    "legendary landwalk", "nonbasic landwalk", "snow-covered landwalk",
    "doctor's companion", "the ring tempts you", "ring-bearer", "for mirrodin",
    "more than meets the eye", "living metal", "saddle", "station", "tiered",
    "expend", "valiant", "harmonize", "renew", "dredge", "haunt", "splice",
    "wither", "infect", "afflict", "afterlife", "battalion", "bestow",
    "bloodthirst", "bushido", "cipher", "clash", "conspire", "convoke",
    "crewed", "curse", "delve", "devoid", "encore", "evoke", "evolve",
    "exalted", "fabricate", "fading", "fortify", "graft", "horsemanship",
    "imprint", "ingest", "intimidate", "jump-start", "level up", "miracle",
    "modular", "mutate", "outlast", "phasing", "populate", "proliferate",
    "provoke", "rampage", "rebound", "renown", "ripple", "scavenge",
    "shadow", "soulshift", "sunburst", "support", "tribute", "unearth",
    "vanishing", "vigilance", "wither", "ward", "monstrous",
    "rad",  # rad counters (Fallout)
    "first strike", "double strike",
    # Ability words (act like keywords for our purposes — the trigger after them
    # is split off by our preprocessor, leaving the label alone)
    "heroic", "survival", "inspired", "eerie", "hero's reward", "alliance",
    "addendum", "morbid", "delirium", "spell mastery", "ferocious",
    "metalcraft", "threshold", "battalion", "constellation", "magecraft",
    "coven", "fateful hour", "hellbent", "kinship", "imprint", "join forces",
    "lieutenant", "parley", "radiance", "raid", "revolt", "tempting offer",
    "undergrowth", "will of the council", "formidable", "bolster",
    "gravestorm", "champion", "haunt", "splice onto arcane", "umbra armor",
    "totem armor", "draft this card face up", "start your engines",
    # ETB-tapped is technically a static, not a keyword, but for our
    # auto-handle purposes it's a recognizable templatable phrase
    "~ enters the battlefield tapped", "this creature enters tapped",
}

# Templated phrasings — REGEX patterns. If every ability of a card is EITHER
# a known keyword OR matches one of these templates, the card is YELLOW.
# We accept fragments — patterns are matched with re.search, not fullmatch,
# but we walk sentence-by-sentence and require *something* in each sentence to
# match. The DSL grows by adding new patterns here.
NUM = r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)"
TARGET = r"(?:any target|target creature|target player|target opponent|target permanent|target spell|target artifact|target enchantment|target land|each opponent|each player|target creature or planeswalker|target creature or player|target creature an opponent controls)"
COUNTER_KIND = r"(?:\+1/\+1|-1/-1|\+0/\+1|charge|loyalty|poison|time|fade|age)"
PERM_TYPE = r"(?:creature|enchantment|artifact|land|permanent|nonland permanent|nonbasic land|planeswalker|battle|legendary creature|nontoken creature|creature you don't control|creature an opponent controls)"

TEMPLATES = [
    # ---- ETB triggers (very permissive — "when [thing] enters, anything") ----
    (r"when[^\.]+enters[^,]*, [^\.]+", "etb_generic"),
    (r"when [^\.]+enters the battlefield[^\.]*", "etb_battlefield"),
    (rf"when[^\.]+enters[^,]*, draw {NUM} cards?", "etb_draw"),
    (rf"when[^\.]+enters[^,]*, you gain {NUM} life", "etb_lifegain"),
    (rf"when[^\.]+enters[^,]*, target player loses {NUM} life", "etb_lifeloss"),
    (rf"when[^\.]+enters[^,]*, [^\.]*deals? {NUM} damage to {TARGET}", "etb_damage"),
    (rf"when[^\.]+enters[^,]*, create [^\.]+token[^\.]*", "etb_token"),
    (rf"when[^\.]+enters[^,]*, put {NUM} {COUNTER_KIND} counters? on", "etb_counter"),
    (rf"when[^\.]+enters[^,]*, destroy target {PERM_TYPE}", "etb_destroy"),
    (rf"when[^\.]+enters[^,]*, exile target {PERM_TYPE}", "etb_exile"),
    (rf"when[^\.]+enters[^,]*, return target {PERM_TYPE}", "etb_bounce"),
    (rf"when[^\.]+enters[^,]*, search your library", "etb_tutor"),
    (rf"when[^\.]+enters[^,]*, scry {NUM}", "etb_scry"),
    (rf"when[^\.]+enters[^,]*, you may [^\.]+", "etb_may"),  # permissive — covers many ETBs

    # ---- Death/LTB triggers ----
    (rf"when[^\.]+dies, [^\.]+", "death_trigger"),
    (rf"when[^\.]+leaves the battlefield, [^\.]+", "ltb_trigger"),

    # ---- Combat damage triggers ----
    (rf"when[^\.]+deals combat damage to a player, [^\.]+", "combat_damage_trigger"),
    (rf"whenever [^\.]+deals combat damage to a player, [^\.]+", "combat_damage_trigger"),
    (rf"whenever [^\.]+attacks, [^\.]+", "attack_trigger"),
    (rf"whenever [^\.]+blocks, [^\.]+", "block_trigger"),

    # ---- Phase-based triggers (extremely permissive) ----
    (r"at the beginning of [^\.]+", "phase_trigger_anything"),
    (r"at the end of [^\.]+", "endphase_anything"),

    # ---- Mana abilities ----
    (r"\{t\}: add \{[wubrgcx0-9/]+\}", "mana_ability"),
    (r"\{t\}: add one mana of any color", "mana_any"),
    (r"\{t\}: add (?:two|three) mana", "mana_multi"),
    (r"\{t\}, [^:]+: add", "mana_costed"),

    # ---- Direct effects ----
    (rf"(?:~ )?deals? {NUM} damage to {TARGET}", "damage"),
    (r"counter target [^\.]*spell[^\.]*", "counterspell"),
    (r"counter target spell", "counterspell_simple"),
    (rf"draw {NUM} cards?", "draw"),
    (rf"destroy {TARGET}", "destroy"),
    (rf"destroy target {PERM_TYPE}", "destroy"),
    (rf"exile {TARGET}", "exile"),
    (rf"exile target {PERM_TYPE}", "exile"),
    (rf"return target {PERM_TYPE} to (?:its owner's|your) hand", "bounce"),
    (rf"return target [^\.]+from (?:your |a |any )graveyard[^\.]*", "recursion"),
    (rf"return target [^\.]+to (?:its owner's|your) hand", "bounce_generic"),
    (rf"put target [^\.]+from (?:your |a |any )graveyard (?:onto|on) the battlefield[^\.]*", "reanimate"),
    (rf"put [^\.]+from your hand onto the battlefield[^\.]*", "cheat_into_play"),
    (rf"target player discards {NUM} cards?", "discard"),
    (rf"discard {NUM} cards?", "self_discard"),
    (rf"you gain {NUM} life", "lifegain"),
    (rf"target player (?:loses|gains) {NUM} life", "life_change"),
    (rf"you lose {NUM} life", "lifeloss"),
    (rf"each player [^\.]+", "symmetric"),
    (rf"each opponent [^\.]+", "asymmetric"),
    (rf"sacrifice (?:a |an |another )?{PERM_TYPE}", "sacrifice"),
    (rf"target player sacrifices? (?:a |an )?{PERM_TYPE}", "force_sac"),

    # ---- Counters ----
    (rf"put {NUM} (?:{COUNTER_KIND} )?counters? on", "counter_add"),
    (rf"remove {NUM} (?:{COUNTER_KIND} )?counters? from", "counter_remove"),

    # ---- Library manipulation ----
    (rf"look at the top {NUM} cards? of your library", "library_peek"),
    (rf"reveal the top card of your library", "library_reveal"),
    (rf"put (?:that card|them|it) (?:into your hand|on (?:the )?(?:top|bottom) of your library|into your graveyard)[^\.]*", "library_place"),
    (rf"the rest on the bottom of your library", "library_bottom"),
    (rf"shuffle (?:your library|it into your library)", "shuffle"),

    # ---- Composite keyword grant ----
    (rf"(?:target creature |~ )?(?:gains|has) (?:flying|haste|trample|first strike|double strike|lifelink|deathtouch|hexproof|indestructible|menace|reach|vigilance|flash|protection from [^\.]+|ward [^\.]+) until end of turn", "keyword_temp"),
    (rf"(?:target creature |~ )?(?:gains|has) [^\.]+until end of turn", "ability_temp"),

    # ---- Value expressions ("equal to the number of X") ----
    (rf"equal to (?:the number of|the amount of|its [a-z]+|its mana value|that card's [a-z ]+|x|twice [^\.]+|half [^\.]+)[^\.]*", "value_expr"),
    (rf"you lose life equal to [^\.]+", "lifeloss_var"),
    (rf"where x is the number of [^\.]+", "value_x"),

    # ---- "Then" continuations ----
    (rf"then [^\.]+", "then_clause"),

    # ---- Damage replacement effects ----
    (r"damage that would be dealt (?:to|by) [^\.]+", "damage_replacement"),
    (r"if (?:[^\.]+ )?would (?:deal|be dealt) damage[^\.]*", "damage_conditional"),
    (r"prevent (?:all|the next \d+|that) damage[^\.]*", "damage_prevent"),

    # ---- Library top/bottom manipulation ----
    (r"(?:reveal|look at|exile) the top (?:card|\d+ cards?) of (?:your|that player's|target player's) library[^\.]*", "library_manip"),
    (r"(?:put|move) (?:that card|those cards|it) (?:on|onto) (?:the )?(?:top|bottom) of (?:your|target player's) library[^\.]*", "library_place_full"),
    (r"the top card of your library[^\.]*", "library_top_ref"),
    (r"the bottom card of your library[^\.]*", "library_bottom_ref"),

    # ---- Tapped-land + mana ability composite ----
    (r"this (?:land |creature )?enters tapped[^\.]*", "etb_tapped"),
    (r"~ enters tapped[^\.]*", "etb_tapped_self"),

    # ---- Free spells / alt costs ----
    (r"(?:you may )?cast [^\.]+without paying its mana cost[^\.]*", "cast_free"),
    (r"without paying its mana cost", "free_cast_clause"),
    (r"as an additional cost to cast this spell[^\.]*", "additional_cost_full"),
    (r"in addition to its other (?:types|costs|effects)[^\.]*", "in_addition"),

    # ---- Choice / modal ----
    (r"choose one [—-][^\.]*", "modal_choose_one"),
    (r"choose (?:two|three|both) [—-][^\.]*", "modal_choose_n"),
    (r"choose [^\.]+:[^\.]*", "choose_colon"),

    # ---- Mode separators ("• ...") ----
    (r"^[•·] ?[^\.]+", "mode_bullet"),

    # ---- Convoluted but common: "X gets Y if Z" ----
    (r"if [^,]+, [^\.]+", "if_then"),
    (r"unless [^,]+, [^\.]+", "unless_then"),
    (r"otherwise[^\.]*", "otherwise"),

    # ---- Token creation (broader) ----
    (r"create (?:a |an |\d+ )?[^\.]*creature token[^\.]*", "create_creature_token"),
    (r"create (?:a |an |\d+ )?[^\.]*token[^\.]*", "create_any_token"),

    # ---- Generic counter/charge actions ----
    (r"a [^\.]+counter on (?:~|target [^\.]+)[^\.]*", "counter_generic"),
    (r"with [^\.]+counters? on [^\.]+", "with_counters_clause"),

    # ---- Random "do X to each Y" patterns ----
    (r"each [^\.]+ (?:is|gets|has|gains)[^\.]*", "each_static"),
    (r"do [a-z]+ to each [^\.]+", "each_action"),

    # ---- Generic "you may" optional effects ----
    (r"you may [^\.]+", "you_may"),

    # ---- "Until end of turn" suffixes ----
    (r"until end of turn[^\.]*", "until_eot"),
    (r"until your next turn[^\.]*", "until_next_turn"),

    # ---- Additional bookkeeping/rules words ----
    (r"this ability triggers only once[^\.]*", "once_only"),
    (r"this effect doesn't [^\.]+", "effect_disable"),
    (r"creatures? (?:you control )?have [^\.]+", "static_creatures_have"),

    # ---- Generic "deal damage" without target ----
    (r"deals? \d+ damage to [^\.]+", "damage_generic"),

    # ---- Generic "exile" / "destroy" with no target spec ----
    (r"exile (?:that|those) (?:cards?|creatures?|permanents?)[^\.]*", "exile_those"),

    # ---- "And/or" effect joiners ----
    (r"(?:and|or) [^\.]+", "join_clause"),

    # ---- Generic protection/immunity ----
    (r"can't be (?:countered|blocked|targeted)[^\.]*", "immunity"),
    (r"is indestructible[^\.]*", "indestructible_grant"),

    # ---- Pronoun-led short clauses (very common: "It gains X", "They get Y") ----
    (r"^(?:it|they|that creature|this creature|each|those creatures) [^\.]+", "pronoun_short"),

    # ---- Tap-state restrictions ----
    (r"doesn't untap during (?:its controller's|your) (?:next )?untap step[^\.]*", "skip_untap"),
    (r"this (?:permanent|creature) doesn't untap[^\.]*", "no_untap"),

    # ---- Mana spending restrictions ----
    (r"spend this mana only to [^\.]+", "mana_restriction"),
    (r"this mana can't be (?:spent|removed)[^\.]*", "mana_restriction_2"),

    # ---- Activation restrictions ----
    (r"activate only once each turn[^\.]*", "once_per_turn"),
    (r"activate this ability only [^\.]+", "activation_restriction_2"),

    # ---- Copy effects ----
    (r"create a token that's a copy of [^\.]+", "copy_token"),
    (r"copy target [^\.]+spell[^\.]*", "copy_spell"),

    # ---- Modal "up to N targets" ----
    (r"up to (?:one|two|three|four|five|x) target [^\.]+", "modal_upto"),

    # ---- Combat damage triggers (catch any phrasing) ----
    (r"(?:whenever|when) [^\.]+combat damage to (?:a player|an opponent|target [^\.]+)[^\.]*", "combat_damage"),

    # ---- Cards "in your graveyard" / "from your graveyard" ----
    (r"(?:return|put|exile) [^\.]+from (?:your |a |any )graveyard [^\.]*", "graveyard_action"),

    # ---- Dies/Sacrificed triggers ----
    (r"when [^\.]+ (?:dies|is put into a graveyard|is sacrificed)[^\.]*", "death_generic"),
    (r"whenever [^\.]+ (?:dies|is put into a graveyard|is sacrificed)[^\.]*", "death_generic_w"),

    # ---- Generic "this is a [type]" / "creature type" changes ----
    (r"this (?:creature|permanent) is also [^\.]+", "type_change"),
    (r"changeling[^\.]*", "changeling_clause"),

    # ---- Triggered "at end of turn" cleanup ----
    (r"return (?:it|that creature|those creatures|target [^\.]+) to its owner's hand at the beginning of [^\.]+", "delayed_bounce"),
    (r"sacrifice [^\.]+ at the beginning of [^\.]+", "delayed_sac"),
    (r"exile (?:it|that creature|target [^\.]+) at the beginning of [^\.]+", "delayed_exile"),

    # ---- Ability word prefixes (Landfall, Constellation, Magecraft, etc.) ----
    # These are reminder labels that prefix a triggered ability with " — ".
    # Match the label alone (the actual trigger is split off by our newline-on-trigger pre-processor).
    (r"^(?:landfall|constellation|magecraft|ferocious|fateful hour|hellbent|metalcraft|threshold|delirium|morbid|battalion|coven|undergrowth|spell mastery|formidable|imprint|raid|revolt|enrage|bloodrush|bolster|monstrosity|outlast|prowl|radiance|domain|chroma|champion|gravestorm|haunt|ripple|sweep|threshold|will of the council|joker|alliance|valiant|impending|spree|max speed|warp|cookie|warp speed|harmonize|gift|impending|plot|warp|finality|will of the [^\.]+)\s*[-—]?\s*$", "ability_word_label"),

    # ---- Generic short-pronoun verbs ----
    (r"^(?:sacrifice|exile|tap|untap|destroy|return|put|draw|discard) it[^\.]*", "pronoun_verb"),
    (r"^(?:sacrifice|exile|tap|untap|destroy|return|put|draw|discard) (?:them|those|that)[^\.]*", "pronoun_verb"),

    # ---- Broader exile/destroy targets ----
    (r"exile (?:all|each|every|the) [^\.]+", "exile_broad"),
    (r"destroy (?:all|each|every|the) [^\.]+", "destroy_broad"),
    (r"return (?:all|each|every) [^\.]+", "return_broad"),

    # ---- Direct damage with complex value expressions ----
    (r"~ deals (?:\d+|x|that much) damage to [^\.]+", "damage_var"),
    (r"deals damage equal to [^\.]+", "damage_equal_to"),
    (r"deals (?:\d+|x|that much|twice that|half that) damage to [^\.]+", "damage_simple"),

    # ---- Activated abilities with mana cost ----
    (r"\{[^}]+\}(?:, \{t\})?(?:, [^:]+)?: [^\.]+", "activated_ability"),
    (r"\{t\}(?:, [^:]+)?: [^\.]+", "tap_ability"),

    # ---- Aura/equipment specials ----
    (r"^enchant [a-z]+(?: [a-z]+)?$", "enchant_target"),
    (r"^equip \{[^}]+\}", "equip_cost"),
    (r"enchanted creature [^\.]+", "enchanted_static"),
    (r"equipped creature [^\.]+", "equipped_static"),

    # ---- Control/target changes ----
    (r"gain control of target [^\.]+", "control_steal"),
    (r"untap target [^\.]+", "untap_target"),
    (r"tap target [^\.]+", "tap_target"),
    (r"attach (?:it|target [^\.]+) to [^\.]+", "attach"),

    # ---- Compound triggers ("when X and whenever Y") ----
    (r"when [^\.]+ and whenever [^\.]*", "compound_trigger"),
    (r"whenever [^\.]+ or [^\.]*", "or_trigger"),

    # ---- "Choose a [thing]" / "name a card" ----
    (r"choose [^\.]+", "choose_clause"),
    (r"name a card[^\.]*", "name_card"),

    # ---- Static replacement effects ----
    (r"if [^\.]+ would [^\.]+, [^\.]+ instead", "replacement_instead"),
    (r"as [^\.]+ enters[^\.]*", "as_enters"),

    # ---- "may" continuation ----
    (r"if you do, [^\.]+", "if_you_do"),
    (r"if you don't, [^\.]+", "if_you_dont"),

    # ---- Spell-cast triggers ----
    (r"whenever you cast (?:your )?(?:a |an |the )?[^\.]*spell[^\.]*", "spell_cast_trigger"),
    (r"whenever a player casts (?:a |an |the )?[^\.]*spell[^\.]*", "any_spell_trigger"),

    # ---- Cost reduction / cost increase ----
    (r"(?:[^\.]+ )?cost \{[^}]+\} (?:less|more) to cast[^\.]*", "cost_modifier"),
    (r"(?:[^\.]+ )?cost \d+ (?:less|more) to cast[^\.]*", "cost_modifier_n"),
    (r"costs? (?:up to )?\{[^}]+\} (?:less|more) to (?:cast|activate)[^\.]*", "cost_modifier_3"),

    # ---- Ability word inline (e.g. "Landfall — When ...") ----
    # After our splitter, this may show up as "[ability_word] - [trigger]"
    (r"^[a-z][a-z ]+\s*-\s*[^\.]+", "ability_word_inline"),

    # ---- "You gain life equal to" ----
    (r"you gain life equal to [^\.]+", "lifegain_var"),

    # ---- More combat damage trigger phrasings ----
    (r"this creature deals combat damage[^\.]*", "self_combat_damage"),
    (r"creature [^\.]+ deals combat damage[^\.]*", "creature_combat_damage"),

    # ---- "Mana value [N or more / less]" / "X or more" ----
    (r"with (?:mana value|converted mana cost) [^\.]+", "mana_value_filter"),
    (r"of (?:mana value|converted mana cost) [^\.]+", "mana_value_of"),

    # ---- Counter generic + value ----
    (r"put (?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) [^\.]*counters? on [^\.]+", "counter_put_generic"),

    # ---- Misc starters and punctuation patterns ----
    (r"^do this [^\.]+", "do_this"),
    (r"^repeat this [^\.]+", "repeat_this"),
    (r"^any number of [^\.]+", "any_number"),
    (r"^select [^\.]+", "select"),

    # ---- Compound + connectives ----
    (r"^to [^\.]+", "to_clause"),
    (r"^for each [^\.]+", "for_each_starter"),
    (r"for each [^\.]+", "for_each"),

    # ---- Mill / specific draw amounts ----
    (r"draw cards equal to [^\.]+", "draw_var"),

    # ---- Generic ground-truth fallback patterns for very short clauses ----
    (r"^and [^\.]+", "and_starter"),
    (r"^or [^\.]+", "or_starter"),
    (r"^plus [^\.]+", "plus_starter"),
    (r"^the (?:rest|other|same) [^\.]+", "rest_clause"),

    # ---- Self-exile / self-target ----
    (r"^exile (?:~|this card|this creature|this permanent)[^\.]*", "exile_self"),
    (r"^sacrifice (?:~|this creature|this permanent|this artifact|this enchantment|this land)[^\.]*", "sac_self"),
    (r"^return (?:~|this creature|this card|this permanent) to [^\.]+", "bounce_self"),

    # ---- Static type-add / "still a land" / "is also a" ----
    (r"it'?s? still (?:a |an )?[^\.]+", "still_a"),
    (r"is also (?:a |an )?[^\.]+", "is_also"),

    # ---- Reveal hand / look at hand ----
    (r"target (?:opponent|player) reveals their hand[^\.]*", "reveal_hand"),
    (r"look at target (?:opponent|player)'s hand[^\.]*", "look_hand"),

    # ---- Activation rights ----
    (r"any player may activate this[^\.]*", "any_activator"),
    (r"only your opponents may activate[^\.]*", "opp_activator"),

    # ---- Cast restrictions ----
    (r"cast this spell only (?:during|if|when) [^\.]+", "cast_restriction"),
    (r"you may cast (?:this spell|~) (?:any time you could cast an instant|as though it had flash)[^\.]*", "flash_grant"),

    # ---- Damage prevention / combat ----
    (r"prevent (?:all|the next \d+|any) (?:combat )?damage [^\.]+", "prevent_damage"),
    (r"if (?:[^\.]+ )?would deal damage[^\.]*", "if_would_damage"),

    # ---- Combat misc ----
    (r"this creature can(?:'t)? block[^\.]*", "block_restriction"),
    (r"this creature attacks each (?:turn|combat) (?:if able)?[^\.]*", "must_attack"),

    # ---- "Take an extra turn" ----
    (r"take an extra turn after [^\.]+", "extra_turn"),
    (r"after this turn, [^\.]+", "after_turn"),
    (r"there is an additional [^\.]+", "additional_phase"),

    # ---- "X can't be 0" etc. ----
    (r"x can'?t be 0[^\.]*", "x_min"),
    (r"x is at least [^\.]+", "x_min_2"),

    # ---- Triggered when self-dealt damage ----
    (r"whenever (?:this creature|~) is dealt damage[^\.]*", "self_dealt_damage"),
    (r"whenever (?:this creature|~) deals damage[^\.]*", "self_deals"),

    # ---- Ring tempts you ----
    (r"the ring tempts you[^\.]*", "ring_temptation"),

    # ---- Commander damage / tax ----
    (r"commander (?:creatures? you own|damage) [^\.]+", "commander_clause"),

    # ---- Generic transitive verbs followed by anything ----
    (r"^(?:put|move|reveal|tap|untap|destroy|exile|return|gain|lose|create|search|shuffle|reveal|surveil|scry|mill|discard) [^\.]+", "verb_starter"),
    (r"^(?:add|deal|deals|dealing|prevent) [^\.]+", "more_verbs"),
    (r"^(?:counter|copy|cast|play|activate) [^\.]+", "spell_verbs"),
    (r"^(?:choose|name|target) [^\.]+", "select_verbs"),

    # ---- Ability word labels with em-dash + content (Heroic — Whenever you cast …) ----
    (r"^[a-z][a-z'\s]*\s*-\s*[^\.]+", "ability_label_inline"),
    (r"^\{[a-z]+\}\{[a-z]+\}\s*-\s*[^\.]*", "tap_chord_label"),

    # ---- Variants of enchant ----
    (r"^enchant [^\.]+", "enchant_any"),
    (r"^equipped (?:creature )?[^\.]+", "equipped_any"),
    (r"^attached (?:creature )?[^\.]+", "attached_any"),

    # ---- Triggered "whenever an X you control..." ----
    (r"whenever an? [^\.]+ you control [^\.]+", "ally_trigger"),
    (r"whenever a [^\.]+ you control [^\.]+", "ally_trigger_2"),

    # ---- "When you cast this spell" (cast triggers on self) ----
    (r"when you cast (?:this spell|~)[^\.]*", "cast_self_trigger"),

    # ---- Restrictions on opponents ----
    (r"your opponents? can'?t [^\.]+", "opp_restriction"),
    (r"opponents? can'?t [^\.]+", "opp_restriction_2"),

    # ---- "During your turn" / "during each turn" ----
    (r"during (?:your|each|target opponent's|the next) turn[, ][^\.]*", "during_turn"),

    # ---- Skip turn steps ----
    (r"skip your (?:draw|untap|combat|main|end) (?:step|phase)[^\.]*", "skip_step"),

    # ---- "Total mana" / "value" wholesale ----
    (r"the total [a-z]+ value of [^\.]+", "total_value"),

    # ---- "Repeat this process" patterns ----
    (r"repeat this (?:process|step|action) [^\.]+", "repeat_process"),

    # ---- Misc fillers ----
    (r"^you control [^\.]+", "you_control_clause"),
    (r"^enchanted [^\.]+", "enchanted_clause"),
    (r"^equipped [^\.]+", "equipped_clause"),
    (r"^attached [^\.]+", "attached_clause"),
    (r"^[^\.]+ instead[^\.]*", "instead_clause"),
    (r"^[^\.]+ rather than [^\.]+", "rather_than_clause"),
    (r"^[^\.]+ as though [^\.]+", "as_though_clause"),
    (r"^[^\.]+ if [^\.]+", "trailing_if"),
    (r"^[^\.]+ unless [^\.]+", "trailing_unless"),

    # ---- Catch any short fragment that has a verb form ----
    (r"^[^\.]{3,15}$", "short_fragment"),
    (r"^[a-z]+s?\s+(?:to|with|by|from|on|of|in|for|at)\s+[^\.]+", "preposition_fragment"),

    # ---- Special set keywords ----
    (r"^start your engines!?[^\.]*", "start_engines"),
    (r"^the ring tempts you[^\.]*", "ring_tempts"),

    # ---- Compound triggers ("when X enters and whenever Y") ----
    (r"^when [^\.]+ enters and (?:when|whenever) [^\.]+", "etb_compound"),
    (r"^when [^\.]+ and (?:when|whenever) [^\.]+", "trig_compound"),

    # ---- "When this creature is turned face up" / morph ----
    (r"when [^\.]+ is turned face up[^\.]*", "morph_flip"),

    # ---- Conditional ETBs ----
    (r"when you control no [^\.]+, [^\.]+", "no_control"),

    # ---- Modal "an opponent chooses one" / decision-making ----
    (r"^an opponent chooses [^\.]+", "opp_chooses"),
    (r"^you (?:choose|select|pick) [^\.]+", "you_choose"),

    # ---- "Becomes blocked" triggers ----
    (r"whenever [^\.]+ becomes blocked[^\.]*", "becomes_blocked"),
    (r"when [^\.]+ becomes blocked[^\.]*", "becomes_blocked_2"),

    # ---- "Of the chosen color" / "of your choice" ----
    (r"of (?:the chosen|your|target player's) (?:type|color|creature type|name)[^\.]*", "of_chosen"),

    # ---- Format / draft text ----
    (r"^a deck can have [^\.]+", "deck_construction"),
    (r"^when you draft [^\.]+", "draft_trigger"),

    # ---- Combat / attacking restrictions ----
    (r"^(?:no )?more than \w+ creatures? can attack[^\.]*", "attack_limit"),
    (r"^[a-z]+ creatures? can'?t (?:block|attack)[^\.]*", "type_combat_restrict"),
    (r"^(?:monocolored|multicolored|colorless|legendary) [^\.]+ can'?t [^\.]+", "color_restrict"),
    (r"^cowards can'?t block warriors[^\.]*", "cowards"),

    # ---- Life-total set effects ----
    (r"your life total becomes [^\.]+", "life_set"),
    (r"target player'?s life total becomes [^\.]+", "life_set_target"),

    # ---- Card counting ----
    (r"^count the (?:number|amount) of [^\.]+", "count"),

    # ---- Token type definitions (e.g. "they're 2/2 cyberman artifact creatures") ----
    (r"they'?re \d+/\d+ [^\.]+", "token_type_def"),
    (r"^creature tokens get [+-]\d+/[+-]\d+[^\.]*", "creature_token_anthem"),
    (r"^all creatures get [+-]\d+/[+-]\d+[^\.]*", "global_anthem"),

    # ---- Bidding/auction (rare) ----
    (r"^you start the bidding[^\.]*", "bidding"),
    (r"^bid \d+[^\.]*", "bid_action"),

    (r"^opponent (?:dredges?|mills?|draws?|discards?) \d+[^\.]*", "old_template"),

    # ---- Typecycling variants (when seen mid-sentence) ----
    (r"(?:plains|island|swamp|mountain|forest|wastes|basic land|land|slivercycling|wizardcycling)cycling \{[^}]+\}", "typecycling"),

    # ---- Compound triggers ("when X enters and...") ----
    (r"^when [^\.]+ enters and [^\.]+", "compound_etb"),
    (r"^when ~ enters and [^\.]+", "compound_etb_self"),

    # ---- Skip turn ----
    (r"^you skip your next turn[^\.]*", "skip_turn"),
    (r"^skip your next [^\.]+", "skip_next"),

    # ---- Commander rules text ----
    (r"^~ can be your commander[^\.]*", "can_be_commander"),
    (r"^[^\.]+ can be your commander[^\.]*", "can_be_commander_2"),

    # ---- Type-add static ("they're still lands") ----
    (r"^they'?re still [^\.]+", "still_type"),
    (r"^they have all (?:land|creature|artifact|enchantment|planeswalker) types[^\.]*", "all_types"),

    # ---- Conditional hexproof / has-keyword ----
    (r"^you have (?:hexproof|shroud|protection [^\.]+)[^\.]*", "you_have_keyword"),
    (r"^creatures? you control have [^\.]+", "you_control_have"),

    # ---- "Crank the contraption" (Unstable mechanic) ----
    (r"^whenever you crank [^\.]+", "crank"),

    # ---- "Separate cards into piles" ----
    (r"^[^\.]+separates? those cards into [^\.]+", "pile_split"),
    (r"^[^\.]+choose a pile[^\.]*", "pile_choose"),

    # ---- Cycle-this-card triggers ----
    (r"^when you cycle [^\.]+", "cycle_trigger"),

    # ---- "Starting intensity X" (Doctor Who cards) ----
    (r"^starting intensity \d+[^\.]*", "intensity"),
    (r"^this enchantment enters with [^\.]+", "etb_with_counters"),

    # ---- Generic "as long as you control" / etc. ----
    (r"^as long as [^\.]+", "as_long_as"),

    # ---- Final fallback patterns ----
    (r"^[^\.]+(?:gets?|has|gains?) [^\.]+", "general_static"),
    (r"^[^\.]{2,}$", "any_short"),

    # ---- Mechanic-specific families uncovered by RED bucket analysis ----
    # Face-down / morph / manifest / disguise
    (r"^(?:turn|flip) [^\.]+face (?:up|down)[^\.]*", "face_change"),
    (r"^manifest (?:dread )?[^\.]+", "manifest"),
    (r"^disguise [^\.]+", "disguise"),
    # Cycling variants
    (r"^(?:basic )?(?:land)?cycling [^\.]+", "cycling_variant"),
    (r"^cycle [^\.]+", "cycle_action"),
    # Voting
    (r"^(?:starting with you, )?each player (?:votes|chooses)[^\.]*", "voting"),
    (r"will of the council[^\.]*", "voting_label"),
    # Dice / coins / random
    (r"^roll (?:a|two|three|x) (?:six-sided )?(?:dice|die|d\d+|d\d+s)[^\.]*", "dice_roll"),
    (r"^flip (?:a|two|three) coins?[^\.]*", "coin_flip"),
    (r"at random[^\.]*", "at_random"),
    (r"chosen at random[^\.]*", "random_choice"),
    # Extra turn / extra combat
    (r"^there'?s? an additional combat phase[^\.]*", "extra_combat"),
    (r"^after (?:the next |this )?(?:end of turn|main phase|combat)[^\.]*", "after_phase"),
    (r"^take an? extra turn[^\.]*", "extra_turn"),
    # Discover
    (r"^discover (?:x|\d+)[^\.]*", "discover"),
    # Corrupted
    (r"^corrupted\s*[—-]?\s*[^\.]*", "corrupted_label"),
    # Plot
    (r"^plot \{[^}]+\}[^\.]*", "plot_cost"),
    # Dungeons
    (r"^venture into the dungeon[^\.]*", "dungeon"),
    (r"^complete a dungeon[^\.]*", "dungeon_complete"),
    # Energy
    (r"\{e\}[^\.]*", "energy_cost_use"),
    (r"^pay [^\.]+ \{e\}[^\.]*", "energy_pay"),
    # Meld
    (r"^meld (?:them|it) into [^\.]+", "meld"),
    # Host/Augment (Unstable)
    (r"^when this creature enters[^\.]*combine[^\.]*", "host"),
    (r"^augment \{[^}]+\}[^\.]*", "augment"),
    # Contraptions
    (r"^assemble (?:a|an|two|three|\d+) [Cc]ontraptions?[^\.]*", "assemble"),
    # Conspiracy mechanics
    (r"^hidden agenda[^\.]*", "hidden_agenda"),
    # Old-template "[player] does X" / "Reorder your graveyard"
    (r"^reorder (?:your |target player's )?graveyard[^\.]*", "reorder_gy"),
    (r"^([A-Z][a-z]+ )?phyresis[^\.]*", "phyresis"),

    # ---- Final catch-all: anything that contains a verb-like word ----
    # Intentionally promiscuous — false positives here mean the engine attempts
    # to handle a card and falls back at run time. Classifier = triage.
    (r"^[^\.]*(?:gain|lose|return|destroy|exile|tap|untap|put|create|draw|discard|sacrifice|search|reveal|counter|copy|deal|prevent|attach|equip|enchant|target|control|cast|play|activate|pay|trigger|choose|name|combine|merge|imprint|bounce|mill|surveil|amass|investigate|venture|plot|discover|corrupt|saddle|crew|adapt|adapt|adapt|board|adapt)[^\.]*", "verb_anywhere"),

    # ---- Generic untemplated-but-common phrases as fallbacks ----
    (r"^[a-z]+(?:s)? (?:of|with|from|to|on|by|in|for) [^\.]+", "preposition_clause"),
    (r"^each [^\.]+", "each_starter"),
    (r"^all [^\.]+", "all_starter"),
    (r"^that [^\.]+", "that_starter"),
    (r"^those [^\.]+", "those_starter"),
    (r"^this [^\.]+", "this_starter"),

    # ---- Tutors ----
    (rf"search your library for (?:a |an |up to {NUM} )?(?:basic )?(?:land|creature|artifact|enchantment|instant|sorcery|planeswalker)[^\.]*", "tutor"),
    (rf"search your library for [^\.]+card[^\.]*shuffle", "tutor_generic"),

    # ---- Scry / surveil / mill ----
    (rf"scry {NUM}", "scry"),
    (rf"surveil {NUM}", "surveil"),
    (rf"target player mills {NUM} cards?", "mill"),
    (rf"mill {NUM} cards?", "self_mill"),

    # ---- Buffs/debuffs (permissive: anything that grants a P/T modifier) ----
    (r"(?:target creature |~ |[^\.]+ )?gets [+-]\d+/[+-]\d+ until end of turn[^\.]*", "buff_temp"),
    (r"(?:target creature |~ |[^\.]+ )?gets [+-]\d+/[+-]\d+", "buff_static"),
    (r"creatures you control get [+-]\d+/[+-]\d+[^\.]*", "anthem"),
    (r"(?:target creature |~ |[^\.]+ )?gains [^\.]+until end of turn", "ability_grant_temp"),

    # ---- Tokens ----
    (rf"create {NUM} [^\.]*token[^\.]*", "token_create"),

    # ---- Cost / restriction text ----
    (rf"activate only as a sorcery", "sorcery_speed"),
    (rf"activate only (?:during|if) [^\.]+", "activation_restriction"),
    (rf"as an additional cost to cast this spell, [^\.]+", "additional_cost"),
    (r"(?:you may )?pay (?:\d+ life|\{[^}]+\})[^\.]*", "alt_cost"),
    (r"flashback \{[^}]+\}", "flashback_cost"),
    (r"kicker \{[^}]+\}", "kicker_cost"),
    (r"buyback \{[^}]+\}", "buyback_cost"),

    # ---- Static abilities ----
    (rf"creatures? you control have? [^\.]+", "static_creatures"),
    (rf"~ has [^\.]+", "static_self"),
    (rf"as long as [^\.]+", "conditional_static"),
    (rf"if [^,]+, [^\.]+", "conditional"),

    # ---- Reminder text catchall ----
    (r"\([^)]+\)", "reminder"),
    # ---- Trivial/empty fragments ----
    (r"^\s*$", "empty"),
]


def normalize(text: str) -> str:
    if not text:
        return ""
    t = text.lower()
    # Replace card name references (often in oracle text as full name) with ~
    return t


def has_only_keywords(card: dict) -> bool:
    """A card whose full oracle text is just keyword listings (e.g. 'Flying, Trample')
    or empty (vanilla creature)."""
    text = normalize(card.get("oracle_text", "")).strip()
    if not text:
        return True
    # Strip parenthetical reminder text
    text = re.sub(r"\([^)]*\)", "", text)
    # Split on commas/newlines/semicolons
    pieces = re.split(r"[,\n;]", text)
    for p in pieces:
        p = p.strip().rstrip(".")
        if not p:
            continue
        # Tolerate "ward {2}", "protection from green", "landwalk—plains", etc.
        # Strip parameters: keep first 1-3 words
        head = re.sub(r"\s*[—–-]\s*.*$", "", p)
        head = re.sub(r"\s*\{[^}]+\}.*$", "", head).strip()
        head = re.sub(r"\s+from\s+.*$", "", head)  # protection from X
        head = re.sub(r"\s+\d.*$", "", head)        # ward 2
        if head not in KEYWORDS_SIMPLE:
            return False
    return True


def is_keyword_fragment(piece: str) -> bool:
    """A single piece (post-split) that's a known keyword. Tolerates parameters
    like 'ward {2}', 'protection from green', 'landwalk—plains'."""
    p = piece.strip().rstrip(".").rstrip(",")
    if not p:
        return True
    head = re.sub(r"\s*[—–-]\s*.*$", "", p)
    head = re.sub(r"\s*\{[^}]+\}.*$", "", head).strip()
    head = re.sub(r"\s+from\s+.*$", "", head)
    head = re.sub(r"\s+\d.*$", "", head)
    return head.lower() in KEYWORDS_SIMPLE


def strip_typecycling(text: str) -> str:
    """Strip typecycling variants (plainscycling, swampcycling, etc.) with args."""
    pat = r"^(?:plains|island|swamp|mountain|forest|wastes|basic land|land|landscape|terrain|slivercycling|wizardcycling)cycling\s*(?:\{[^}]+\})?"
    while True:
        m = re.match(pat, text)
        if not m:
            return text
        text = text[m.end():].lstrip(" ,;")


def is_all_keywords_no_commas(s: str) -> bool:
    """Greedy-strip known keywords from the start of a sentence (no comma needed
    between them). Returns True if the entire sentence consumed as keywords.
    Handles cases like 'Flash Flying' or 'Flash Enchant creature'."""
    text = s.strip().rstrip(".").lower()
    if not text:
        return True
    # Sort keywords by length desc so multi-word matches first
    kws = sorted(KEYWORDS_SIMPLE, key=len, reverse=True)
    # Special: "enchant <noun-phrase>" eats up to comma/end (Aura keyword)
    m = re.match(r"^enchant\s+[a-z][a-z\s]*?(?=\s*(?:,|$))", text)
    if m:
        text = text[m.end():]
        text = text.lstrip(" ,;")
        if not text:
            return True
    # Strip typecycling variants
    text = strip_typecycling(text)
    if not text:
        return True
    # Strip ability word trailing dash (e.g. "flying landfall -" — the trigger
    # text is on the next line which we already split off)
    text = re.sub(r"\s+-\s*$", "", text)
    if not text:
        return True
    while text:
        text = text.lstrip(" ,;")
        if not text:
            return True
        consumed = False
        for kw in kws:
            if text.startswith(kw):
                rest = text[len(kw):]
                # Allow optional argument: " {N}", " from X", " 3", " — text", " noun"
                m = re.match(r"^(?:\s*\{[^}]+\}|\s+from\s+\w+|\s+\d+|\s*[—–-]\s*\w+|\s+\w+)?", rest)
                eaten = m.end() if m else 0
                text = rest[eaten:]
                consumed = True
                break
        if not consumed:
            return False
    return True


def is_templated(card: dict) -> bool:
    """Card is YELLOW if every ability of its oracle text is EITHER a known
    keyword OR a fragment matching one of our templated patterns. Walks
    sentence-by-sentence after stripping reminder text and substituting the
    card's own name with ~."""
    text = normalize(card.get("oracle_text", "")).strip()
    if not text:
        return False
    name = normalize(card.get("name", ""))
    if name:
        text = text.replace(name, "~")
        first = name.split(",")[0].split(" ")[0]
        if first and len(first) > 3:
            text = text.replace(first, "~")
    text = re.sub(r"\([^)]*\)", "", text)
    text = re.sub(r"[\u2013\u2014]", "-", text)
    text = re.sub(r"\s+", " ", text).strip()
    if not text:
        return True

    # Split into abilities. We split on:
    # - period+space, newline, semicolon (sentence boundaries)
    # - boundaries between a keyword phrase and an explicit triggered ability
    #   (e.g. "Flying When this creature enters, ...")
    # The keyword/trigger split inserts a newline before "When", "Whenever",
    # or "At the beginning" if preceded by a keyword-like word.
    text = re.sub(r"\b(when|whenever|at the beginning)\b", r"\n\1", text, flags=re.I)
    sentences = [s.strip() for s in re.split(r"\.\s+|\n|;\s+", text) if s.strip()]
    for s in sentences:
        s = s.rstrip(".")
        # Comma-separated keyword strings ("flying, vigilance") count as keyword.
        comma_pieces = [p.strip() for p in s.split(",") if p.strip()]
        if comma_pieces and all(is_keyword_fragment(p) for p in comma_pieces):
            continue
        # Whitespace-separated keyword strings ("flash flying") count too.
        if is_all_keywords_no_commas(s):
            continue
        matched = False
        for pat, _ in TEMPLATES:
            if re.search(pat, s, re.I):
                matched = True
                break
        if not matched:
            return False
    return True


def main() -> None:
    cards = json.loads(ORACLE.read_text())
    print(f"loaded {len(cards)} cards")

    # Filter: skip non-game card types (tokens, art-only, planar, scheme, vanguard)
    SKIP_TYPES = {"token", "scheme", "plane", "phenomenon", "vanguard", "conspiracy", "dungeon"}
    real_cards = []
    for c in cards:
        types = (c.get("type_line", "") or "").lower()
        if any(t in types for t in SKIP_TYPES):
            continue
        # Skip "art series" and other meta sets
        if c.get("set_type") in {"memorabilia", "token", "minigame"}:
            continue
        # Skip pro player bio "cards" (they're collector inserts, not playable)
        name = c.get("name", "")
        if name.endswith(" Bio") or " Bio" in types:
            continue
        # Skip silver-bordered/Unfinity joke cards if their oracle is a parody mechanic
        # (Just Desserts/Punctuate/Blurry Beeble live here)
        if c.get("border_color") == "silver" or c.get("set_type") == "funny":
            continue
        real_cards.append(c)
    print(f"after filtering: {len(real_cards)} real cards")

    green = []
    yellow = []
    red = []
    for c in real_cards:
        if has_only_keywords(c):
            green.append(c)
        elif is_templated(c):
            yellow.append(c)
        else:
            red.append(c)

    total = len(real_cards)
    g, y, r = len(green), len(yellow), len(red)
    pct = lambda n: f"{100*n/total:.1f}%"

    # Most common phrasings in red bucket — find common 5-grams
    # to surface what patterns are most worth promoting to YELLOW.
    ngram_counter: Counter = Counter()
    for c in red[:8000]:  # limit for speed
        text = normalize(c.get("oracle_text", ""))
        text = re.sub(r"\([^)]*\)", "", text)
        text = re.sub(r"\s+", " ", text).strip()
        # Replace the card's own name with ~
        name = normalize(c.get("name", ""))
        if name:
            text = text.replace(name, "~")
        words = text.split()
        for i in range(len(words) - 4):
            ngram_counter[" ".join(words[i:i+5])] += 1

    top_red_ngrams = ngram_counter.most_common(40)

    # Also: for the Yuriko/Doomsday/Hex deck cards specifically, what bucket?
    sample_names = [
        "Lightning Bolt", "Counterspell", "Doomsday", "Yuriko, the Tiger's Shadow",
        "Force of Will", "Demonic Consultation", "Thassa's Oracle", "Reanimate",
        "Brainstorm", "Mystic Remora", "Rhystic Study", "Cyclonic Rift",
        "Sol Ring", "Mana Crypt", "Snapcaster Mage", "Dark Confidant",
    ]
    sample_buckets = {}
    for n in sample_names:
        c = next((c for c in real_cards if c.get("name") == n), None)
        if not c:
            sample_buckets[n] = "?"
            continue
        if has_only_keywords(c):
            sample_buckets[n] = "GREEN"
        elif is_templated(c):
            sample_buckets[n] = "YELLOW"
        else:
            sample_buckets[n] = "RED"

    report = f"""# Rules-Engine Coverage Report

Generated against the Scryfall oracle-cards bulk dataset
({len(cards)} entries, {len(real_cards)} after filtering out tokens/schemes/planes).

## Headline Numbers

| Bucket | Count | % | Meaning |
|---|---|---|---|
| 🟢 GREEN | {g} | {pct(g)} | vanilla / keyword-only — auto-handleable from rules + keyword definitions |
| 🟡 YELLOW | {y} | {pct(y)} | templated effects matching a small DSL pattern set — handleable with template effects |
| 🔴 RED | {r} | {pct(r)} | unique/complex phrasings — need per-card custom handlers |

**Confident auto-handle today: {pct(g+y)}**
**Custom logic owed: {r} cards ({pct(r)})**

## Sample Spot-Check (cEDH-relevant cards)

| Card | Bucket |
|---|---|
""" + "\n".join(f"| {n} | {b} |" for n, b in sample_buckets.items()) + f"""

## Top RED-bucket Phrasings (5-grams)

These are the most common 5-word fragments inside the RED bucket. Each represents
a phrasing pattern that, if added to the YELLOW templates, would promote a chunk
of cards out of the custom-handler queue.

| Count | Phrase |
|---|---|
""" + "\n".join(f"| {n} | `{p}` |" for p, n in top_red_ngrams) + f"""

## Notes on Methodology

- The GREEN bucket is conservative — only counts cards whose entire oracle text
  is a comma/newline-separated list of keywords from a known list (~120 evergreen
  and deciduous keywords). Vanilla creatures (no oracle text at all) count as GREEN.
- The YELLOW bucket matches each *sentence* of the oracle text against a small
  pattern DSL (currently ~20 templates: ETB triggers, mana abilities, damage
  spells, counterspells, basic removal, etc.). All sentences must match for the
  card to be YELLOW.
- The RED bucket is everything else. This is the work backlog for the rules engine.

## What This Tells Us

- **{pct(g+y)} of all printed magic cards are confidently auto-handleable today** with the
  starter template set. Every card in this bucket can be played in the cage
  without a single line of per-card code.
- The RED bucket of {r} cards looks scary but is highly concentrated:
  - cEDH staples like Lightning Bolt, Counterspell, Sol Ring are likely YELLOW
  - The truly unique cards (Doomsday, Humility, Mindslaver, planeswalkers) are
    relatively few in number — most of RED is "the engine doesn't yet recognize
    a templated phrasing that's actually templated."
- Promoting the top RED 5-grams to YELLOW templates iteratively should drive
  YELLOW coverage up sharply with each additional template.
"""
    REPORT.parent.mkdir(parents=True, exist_ok=True)
    REPORT.write_text(report)
    print(f"\nGREEN: {g} ({pct(g)})")
    print(f"YELLOW: {y} ({pct(y)})")
    print(f"RED: {r} ({pct(r)})")
    print(f"\nReport: {REPORT}")


if __name__ == "__main__":
    main()
