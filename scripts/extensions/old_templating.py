#!/usr/bin/env python3
"""Pre-modern oracle templating (Alpha/Beta/Arabian/Antiquities/Legends-era).

Family: PRE-MODERN ORACLE TEMPLATING — verb/noun shapes that pre-date Sixth
Edition's rules overhaul and Magic 2010's modernized templating pass. Even
after Wizards' errata-forward program the modern oracle still carries a
long tail of these phrasings because they appear either on cards whose
mechanics never mapped cleanly onto modern templates (Echo, Rampage,
Cumulative Upkeep, Licid-style "pay X to end this effect"), or on effect
bodies that never became common enough to warrant rewording ("change the
text", "the new target must be", "tap any number of", "cast this spell
only after combat", "creatures with Xwalk can be blocked as though…").

This extension focuses on those residuals. The three exports mirror the
format used by ``combat_triggers.py`` / ``replacements.py``:

    TRIGGER_PATTERNS = [(compiled_regex, event, scope), …]
    STATIC_PATTERNS  = [(compiled_regex, builder), …]
    EFFECT_RULES     = [(compiled_regex, builder), …]

Coverage targets (from a parser_coverage.md scan across 31K real cards
and a full sweep of sets ``lea, leb, arn, atq, leg, drk, fem, ice, mir,
vis, wth, tmp, sth, exo, usg, ulg, uds``):

    47 echo {cost}                          # keyword; KEYWORD_RE has buyback but not echo
    39 target creature gains <kw> until eot # modern cards use this too; still a real gap
    28 target creature gets ±X/±X until eot, where x is …
    27 it/that/target creature becomes a <type>… until end of turn
    20 this land/artifact/creature doesn't untap during …
    18 when you control no <basic>s, sacrifice …
    16 an opponent chooses one (of those …)
    15 attach it to target permanent/creature it can enchant
    14 buyback-<cost>  ("buyback-sacrifice a land", "buyback-discard two cards")
    13 you may pay {cost} to end this effect     # Licid cycle
    10 change the text of target … by replacing all instances of …
     9 creatures with <type>walk can be blocked as though they didn't have it
     9 rampage N                              # keyword, KEYWORD_RE misses it
     9 the next time a source of your choice would deal damage …
     6 destroy it  /  destroy that creature   # pronoun-led destroy
     6 (a creature / that creature) destroyed this way can't be regenerated
     6 you may choose not to untap this land during your untap step
     3 target creature attacks this turn if able
     3 the new target must be …
     2 cast this spell only after combat      # Jabari's Influence, Glyph of Reincarnation
     2 prevent x of that damage, where x is …
     — remove this card from your deck before playing if you're not playing for ante
     — this change in ownership is permanent

Each pattern below is tagged with its fragment count and a canonical
example. Ordering follows the project convention: most specific first,
bare verb last, so the parser's first-match-wins loop is stable.

We do NOT edit parser.py. ``load_extensions`` in the parser imports this
module and merges the three lists at the right precedence points (ext
TRIGGER_PATTERNS and EXT_STATIC_PATTERNS run before the core's built-in
patterns; EFFECT_RULES are appended so the more-general built-ins keep
priority for shared shapes).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Make mtg_ast importable when this module is loaded directly.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Destroy, Effect, Filter, GrantAbility, Keyword, Modification, Prevent,
    Replacement, Static, TapEffect, UntapEffect, Discard, UnknownEffect,
    Buff, Damage,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_SELF = (
    r"(?:~|this creature|this permanent|this land|this artifact|"
    r"this enchantment|this card)"
)

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}


def _num(tok: str):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


def _mk_static(kind: str, raw: str, *extra) -> Static:
    return Static(
        modification=Modification(kind=kind, args=(raw, *extra)),
        raw=raw,
    )


# ===========================================================================
# TRIGGER_PATTERNS — pre-modern timing and phrasing
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # ------------------------------------------------------------------
    # Old-style phase triggers (pre-Sixth Edition timing). "At end of
    # turn" is the legacy wording the modern rules split into "at the
    # beginning of the end step" (for triggers) vs "until end of turn"
    # (for duration). Oracle still uses the legacy phrase in ability
    # bodies on some Alpha/Legends cards and on reprinted Arabian
    # Nights text.
    # ------------------------------------------------------------------
    (re.compile(r"^at end of turn\b", re.I), "phase", "end_step"),
    (re.compile(r"^at end of combat\b", re.I), "phase", "end_of_combat"),
    # "At end of game" — Shahrazad / Mindslaver-era subgame flavor
    (re.compile(r"^at end of (?:the )?game\b", re.I), "phase", "end_of_game"),
    # "At end of round" — multiplayer / planechase era
    (re.compile(r"^at end of (?:the )?round\b", re.I), "phase", "end_of_round"),

    # ------------------------------------------------------------------
    # "At the end of [opponent's / each opponent's / each player's] turn"
    # — legacy phrasing for end-step triggers that modern oracle
    # templates as "at the beginning of [opponent's] end step".
    # ------------------------------------------------------------------
    (re.compile(r"^at the end of each opponent'?s turn\b", re.I),
     "phase", "end_step_each_opponent"),
    (re.compile(r"^at the end of (?:target |an )?opponent'?s turn\b", re.I),
     "phase", "end_step_opponent"),
    (re.compile(r"^at the end of each player'?s turn\b", re.I),
     "phase", "end_step_each"),
    (re.compile(r"^at the end of your turn\b", re.I),
     "phase", "end_step_yours"),
    (re.compile(r"^at the end of the turn\b", re.I),
     "phase", "end_step"),

    # ------------------------------------------------------------------
    # "When you control no [basic-type]s, sacrifice this creature"
    # — Homelands / Ice Age "island-walking home" trigger (18 fragments).
    # Scope='self' so the builder knows the actor is the card itself.
    # ------------------------------------------------------------------
    (re.compile(r"^when you control no (plains|islands|swamps|mountains|forests|"
                r"[a-z]+s)(?:,)? sacrifice (?:this creature|~|this permanent)", re.I),
     "lose_home_basic", "self"),

    # ------------------------------------------------------------------
    # "When [that creature] leaves the battlefield this turn, sacrifice
    # this creature" — Heart Wolf / Champion's Helm legacy phrasing.
    # ------------------------------------------------------------------
    (re.compile(r"^when that creature leaves the battlefield this turn\b", re.I),
     "that_creature_ltb_this_turn", "self"),

    # ------------------------------------------------------------------
    # "When an opponent casts a spell/creature spell, if this permanent
    # is an enchantment, it becomes a [P/T] [type] creature" — the
    # Opal / Veiled enchantress cycle. We only match the trigger here;
    # the body is parsed as a normal effect (becomes-creature rule
    # below).
    # ------------------------------------------------------------------
    (re.compile(r"^when an opponent casts a (?:creature )?spell\b", re.I),
     "opp_cast_spell", "self"),

    # ------------------------------------------------------------------
    # "When a spell or ability an opponent controls causes you to
    # discard this card" — Wormfang / Sand Golem
    # ------------------------------------------------------------------
    (re.compile(r"^when a spell or ability an opponent controls causes you to discard "
                r"(?:this card|~)\b", re.I),
     "opp_makes_you_discard_self", "self"),

    # ------------------------------------------------------------------
    # "When you cast a creature spell, sacrifice this creature"
    # — Skittering Horror / Coffin Queen downside
    # ------------------------------------------------------------------
    (re.compile(r"^when you cast a creature spell, sacrifice (?:this creature|~)", re.I),
     "you_cast_creature_sac_self", "self"),

    # ------------------------------------------------------------------
    # "When this creature is turned face up" — morph / disguise legacy
    # shape that still appears on some modern-layout cards but misses
    # the core parser's "enters" regex.
    # ------------------------------------------------------------------
    (re.compile(r"^when (?:this creature|~) is turned face up\b", re.I),
     "turned_face_up", "self"),
]


# ===========================================================================
# STATIC_PATTERNS — always-on pre-modern shapes
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static_pattern(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Landwalk-prevention hosers (9 fragments): "Creatures with <type>walk
# can be blocked as though they didn't have <type>walk." (Quagmire,
# Undertow, Great Wall, Lord Magnus, Coastal Piracy cycle.)
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^creatures with (plains|island|swamp|mountain|forest)walk"
    r" can be blocked as though they didn'?t have \1walk\.?$"
)
def _landwalk_hoser(m, raw):
    return _mk_static("landwalk_hoser", raw, m.group(1) + "walk")


# ---------------------------------------------------------------------------
# Skip-untap restrictions (20 fragments): "This land doesn't untap during
# your untap step" / "… during its controller's next untap step" / "…
# if it has a depletion counter on it". Covers Grim Monolith, Veldt,
# Mogg Hollows, Mana Skimmer, Mindbender Spores' quoted-body form.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^(?:~|this (?:land|artifact|creature|permanent|enchantment))"
    r" doesn'?t untap during"
    r" (?:your|its controller'?s|each player'?s|the active player'?s)"
    r"(?: next)? untap step"
    r"(?: if it has [^.]+? counter(?:s)? on it)?\.?$"
)
def _no_untap_self(m, raw):
    return _mk_static("no_untap_conditional", raw)


# "That land/creature/artifact doesn't untap during its controller's next
# untap step" — used as a riding sentence on destroy/tap effects.
@_static_pattern(
    r"^that (?:land|creature|artifact|permanent) doesn'?t untap during"
    r" (?:its controller'?s|your|the active player'?s)(?: next)? untap step\.?$"
)
def _no_untap_that(m, raw):
    return _mk_static("no_untap_that_permanent", raw)


# ---------------------------------------------------------------------------
# Optional skip-untap (6 fragments): "You may choose not to untap this
# land/artifact during your untap step." (Hollow Trees, Sand Silos,
# Winding Canyons, Thawing Glaciers' cousins.)
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^you may choose not to untap"
    r" (?:this (?:land|artifact|creature|permanent)|~)"
    r" during your untap step\.?$"
)
def _optional_skip_untap(m, raw):
    return _mk_static("optional_skip_untap_self", raw)


# ---------------------------------------------------------------------------
# Echo (47 fragments): "Echo {cost}". KEYWORD_RE has buyback / bestow /
# cycling etc. but is missing echo. We fire as a Static whose body is a
# Keyword so downstream clustering treats it like any other ability
# keyword.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^echo (?:\{[^}]+\}(?:\{[^}]+\})*|\d+)\.?$"
)
def _echo_kw(m, raw):
    return Static(
        modification=Modification(kind="keyword_ref",
                                  args=("echo", raw.strip().rstrip("."))),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Rampage N (9 fragments): old Legends-era keyword. Missing from KEYWORD_RE.
# ---------------------------------------------------------------------------
@_static_pattern(r"^rampage (\d+)\.?$")
def _rampage_kw(m, raw):
    return Static(
        modification=Modification(kind="keyword_ref",
                                  args=("rampage", int(m.group(1)))),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Buyback variants (14 fragments): "Buyback-sacrifice a land",
# "Buyback-discard two cards", "Buyback {2}{B}{B}" (already covered by
# KEYWORD_RE, but only the mana-cost form). The dash-separated sac /
# discard variants appear on Pegasus Stampede, Demonic Collusion,
# Reaping the Rewards, Walk the Aeons.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^buyback[-\u2013\u2014](sacrifice|discard|return|exile|pay) ([^.]+?)\.?$"
)
def _buyback_alt(m, raw):
    return Static(
        modification=Modification(kind="keyword_ref",
                                  args=("buyback_alt", m.group(1), m.group(2).strip())),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Ante clause (Alpha / Arabian / Legends): "Remove this card from your
# deck before playing if you're not playing for ante." — only lives on
# banned cards but still in oracle text.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^remove this card from your deck before playing"
    r" if you'?re not playing for ante\.?$"
)
def _ante_clause(m, raw):
    return _mk_static("ante_clause", raw)


# ---------------------------------------------------------------------------
# "This change in ownership is permanent." — Tempest Efreet tail.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^this change in ownership is permanent\.?$"
)
def _ownership_permanent(m, raw):
    return _mk_static("ownership_permanent_rider", raw)


# ---------------------------------------------------------------------------
# "Cast this spell only after combat" / "only during combat" — legacy
# timing restrictions on Jabari's Influence, Glyph of Reincarnation.
# Core parser has "activate only as a sorcery / only during" for
# activated abilities, but NOT the instant-sorcery "cast this spell
# only …" form.
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^cast this spell only"
    r" (?:after combat|during combat|during your turn|"
    r"during an opponent'?s turn|during the (?:declare attackers|end) step)\.?$"
)
def _cast_timing(m, raw):
    return _mk_static("cast_timing_restriction", raw)


# ---------------------------------------------------------------------------
# "The new target must be a <type>." — Reflecting Mirror / Silver
# Wyvern redirect riders (3 fragments).
# ---------------------------------------------------------------------------
@_static_pattern(
    r"^the new target must be (?:a |an |)([^.]+?)\.?$"
)
def _new_target_must(m, raw):
    return _mk_static("new_target_constraint", raw, m.group(1).strip())


# ===========================================================================
# EFFECT_RULES — one-shot effect bodies in pre-modern phrasing
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _effect_rule(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Pronoun-led destroy (6+6 fragments): "Destroy it." / "Destroy that
# creature." These are effect-bodies on cards where a prior sentence
# established the referent (Stone Giant, Mogg Cannon, Puffer Extract,
# Norritt). The core parser has _destroy for "destroy target …" but
# not the bare pronoun form.
# ---------------------------------------------------------------------------
@_effect_rule(r"^destroy (?:it|that (?:creature|permanent|artifact|enchantment|land))\.?$")
def _destroy_pronoun(m):
    return Destroy(target=Filter(base="pronoun", targeted=False))


# ---------------------------------------------------------------------------
# "A/The creature(s) destroyed this way can't be regenerated." (6+3
# fragments). Kirtar's Wrath, Breaking Point, Tsabo's Assassin, Soul
# Rend, Mortify-style riders. Pre-modern "bury" legacy — the cleanest
# modeling is a Modification rider that tags the preceding destroy as
# bury-style.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^(?:a |the |)creatures? destroyed this way can'?t be regenerated\.?$"
)
def _destroyed_cant_regen_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# "That creature can't be regenerated" — standalone rider (3 fragments).
@_effect_rule(
    r"^that (?:creature|permanent) can'?t be regenerated\.?$"
)
def _that_cant_regen(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# Licid "you may pay {cost} to end this effect" (13 fragments) —
# Quickening / Flanking / Tempting / Gliding / Corrupting / Enraging
# Licids. The preceding sentence established a continuous effect; this
# is the opt-out rider.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^you may pay (\{[^}]+\}(?:\{[^}]+\})*) to end this effect\.?$"
)
def _licid_end(m):
    return UnknownEffect(raw_text=f"may_pay_to_end_effect:{m.group(1)}")


# ---------------------------------------------------------------------------
# "Attach it to target <filter>" (15 fragments) — Quickening Licid,
# Aura Graft, Stolen Uniform. The "it" refers to the Aura/Equipment
# previously introduced.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^attach it to (?:target |another |the chosen |)([^.]+?)\.?$"
)
def _attach_it_to(m):
    return UnknownEffect(raw_text=f"attach_pronoun_to:{m.group(1).strip()}")


# ---------------------------------------------------------------------------
# "Target creature gains <keyword(s)> until end of turn" (39 fragments).
# Granting a keyword for a turn is well within the parser's wheelhouse
# already (_it_gains covers the pronoun form), but the "target
# creature" form slips through because parse_effect's _it_gains regex
# insists on "it" as the subject. This explicit pattern catches the
# "target creature gains X (and Y)" variant used on Cloak of Feathers,
# Might of the Meek, Lace with Moonglove.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^target creature gains ([a-z][a-z\- ]*?(?:(?: and | )[a-z][a-z\- ]*?)*)"
    r"(?: until end of turn)?\.?$"
)
def _target_gains_kw_eot(m):
    kw = m.group(1).strip()
    return GrantAbility(
        ability_name=kw,
        target=Filter(base="creature", targeted=True),
        duration="until_end_of_turn",
    )


# ---------------------------------------------------------------------------
# "Target creature gets +X/+Y until end of turn, where X is …" (28
# fragments). The core parser's _buff rule matches the literal "+N/+M"
# form but bails on the variable tail. Ghoul's Feast, Planeswalker's
# Favor, Scent of Nightshade / Ivy / Jasmine, Bloodcurdling Scream,
# Kry Shield.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^target creature gets ([+\-]x?)/([+\-]x?)"
    r" until end of turn,? where x is ([^.]+?)\.?$"
)
def _target_buff_var_eot(m):
    ptop = m.group(1).strip()
    ttop = m.group(2).strip()
    cond = m.group(3).strip()
    # Buff only stores int power/toughness. For variable X we tag it with
    # the symbolic sign and leave the magnitude embedded in raw — the
    # engine reads cond to resolve X. Downstream clustering just needs
    # to see this as a Buff, not an UnknownEffect.
    def _sign_magnitude(tok: str) -> int:
        # "+x" or "-x" collapse to +1 / -1 as a placeholder; the real
        # magnitude is carried by cond. Plain "+0" / "-0" preserved as 0.
        if tok in ("+x", "-x"):
            return 1 if tok.startswith("+") else -1
        try:
            return int(tok.replace("+", ""))
        except ValueError:
            return 0
    return Buff(
        power=_sign_magnitude(ptop),
        toughness=_sign_magnitude(ttop),
        target=Filter(base="creature", targeted=True),
        duration="until_end_of_turn",
    )


# ---------------------------------------------------------------------------
# "Target creature attacks this turn if able." (3 fragments) — Into the
# Fray, Imps' Taunt, Norritt.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^target creature attacks this turn if able\.?$"
)
def _must_attack(m):
    return UnknownEffect(raw_text="target_creature_must_attack_eot")


# ---------------------------------------------------------------------------
# "That creature attacks this turn if able." — Norritt-style rider.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^that creature attacks this turn if able\.?$"
)
def _that_must_attack(m):
    return UnknownEffect(raw_text="that_creature_must_attack_eot")


# ---------------------------------------------------------------------------
# "Tap any number of untapped creatures you control." (3 fragments) —
# Harmony of Nature, Marshaling the Troops, Channel-effects. Core
# parser's _tap_up_to handles "tap up to N", not the open-ended "any
# number of".
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^tap any number of untapped ([^.]+?) you control\.?$"
)
def _tap_any_number(m):
    return TapEffect(target=Filter(base=m.group(1).strip(), you_control=True,
                                   targeted=False, quantifier="any",
                                   extra=("untapped",)))


# ---------------------------------------------------------------------------
# "Untap all creatures you control" (3 fragments) — Rally the Troops.
# ---------------------------------------------------------------------------
@_effect_rule(r"^untap all creatures you control\.?$")
def _untap_all_yours(m):
    return UntapEffect(target=Filter(base="creature", you_control=True,
                                     targeted=False, quantifier="all"))


# ---------------------------------------------------------------------------
# "An opponent chooses one [of those …]" (16 fragments) — Steam Augury,
# Emergent Ultimatum, Murmurs from Beyond, Library of Lat-Nam,
# Ecological Appreciation.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^an opponent chooses (one|two|three|\d+)(?: of (?:those|them|these|"
    r"those (?:piles|cards|creatures)))?(?:\s*[-–—])?\.?$"
)
def _opponent_chooses(m):
    n = _num(m.group(1))
    return UnknownEffect(raw_text=f"opponent_chooses:{n}")


# ---------------------------------------------------------------------------
# "Target opponent chooses a creature they control." — Goblin War Cry.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^target opponent chooses (?:a |an |one |)([^.]+?)(?: they control)?\.?$"
)
def _opponent_chooses_filter(m):
    return UnknownEffect(raw_text=f"target_opponent_chooses:{m.group(1).strip()}")


# ---------------------------------------------------------------------------
# "That player discards those cards." (3 fragments) — Mind Warp rider.
# ---------------------------------------------------------------------------
@_effect_rule(r"^that player discards those cards\.?$")
def _that_player_discards_those(m):
    return Discard(count="those",
                   target=Filter(base="player", targeted=False))


# ---------------------------------------------------------------------------
# "Target opponent discards two cards." (2 fragments) — Tendrils of
# Despair. The core parser's _force_discard requires "discard N cards"
# attached to a known target filter; "target opponent discards" slips.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^target opponent discards (a|an|one|two|three|four|five|\d+) cards?\.?$"
)
def _target_opp_discards(m):
    return Discard(count=_num(m.group(1)),
                   target=Filter(base="opponent", targeted=True))


# ---------------------------------------------------------------------------
# "Change the text of target <thing> by replacing all instances of X
# with Y." (10 fragments) — Alter Reality, Trait Doctoring, New Blood,
# Sleight of Mind.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^change the text of (?:target |that |)([^.]+?)"
    r" by replacing all instances of ([^.]+?) with ([^.]+?)\.?$"
)
def _change_text(m):
    return UnknownEffect(
        raw_text=f"change_text:{m.group(1).strip()}|{m.group(2).strip()}=>{m.group(3).strip()}"
    )


# ---------------------------------------------------------------------------
# "The next time a source of your choice would deal damage [to <target>]
# this turn, <effect>." (9 fragments) — Invulnerability, Reflect Damage,
# Eye for an Eye, Remove Soul-style redirects. This is a one-shot
# replacement effect; model it as a Replacement with the next-damage
# event.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^the next time a source of your choice would deal damage"
    r"(?: to ([^,]+))? this turn,"
    r" (?:(instead (?:that source|it) deals [^.]+?)|([^.]+?))\.?$"
)
def _next_time_source(m):
    to_tgt = (m.group(1) or "any target").strip()
    body = (m.group(2) or m.group(3) or "").strip()
    return Replacement(
        trigger_event="next_damage_source_choice",
        replacement=UnknownEffect(raw_text=f"target={to_tgt}; body={body}"),
    )


# ---------------------------------------------------------------------------
# "Prevent X of that damage, where X is …" (2 fragments) — Errant
# Minion, Power Leak. Parametric prevention rider.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^prevent x of that damage,? where x is ([^.]+?)\.?$"
)
def _prevent_x_paid(m):
    return Prevent(amount="x", damage_filter=Filter(base="that_damage", targeted=False),
                   duration="one_shot")


# ---------------------------------------------------------------------------
# "You gain N life for each creature tapped this way" / "… for each card
# revealed this way" (Harmony of Nature / Scent of Jasmine).
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^you gain (\d+) life for each (creature tapped|card revealed|[^.]+?) this way\.?$"
)
def _gain_life_per_tagged(m):
    return UnknownEffect(
        raw_text=f"gain_life_var:{m.group(1)}_per_{m.group(2).strip()}_this_way"
    )


# ---------------------------------------------------------------------------
# "It becomes a <P/T>? <type> creature [with <kws>] until end of turn"
# / "Target creature becomes a copy of that creature until end of turn"
# (27 fragments). Pre-modern transform phrasing (Sleight of Hand era)
# plus modern copy-spells.
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^(?:target |that |)(?:(?:non|)[a-z ]+?(?:creature|permanent|artifact))"
    r" becomes (?:a |an |)(\d+/\d+ )?"
    r"(?:(copy of [^.]+?)|([^.]+? creature(?:[^.]*?)))(?: until end of turn)?\.?$"
)
def _becomes_creature(m):
    pt = (m.group(1) or "").strip()
    copy = m.group(2)
    descr = m.group(3) or ""
    body = f"copy:{copy}" if copy else f"type:{descr.strip()};pt:{pt}"
    return UnknownEffect(raw_text=f"becomes_creature:{body}")


# Pronoun-led "it becomes a <type> creature [with X]"
@_effect_rule(
    r"^it becomes a (\d+/\d+ )?([^.]+? creature(?:[^.]*?))(?: until end of turn)?\.?$"
)
def _it_becomes(m):
    pt = (m.group(1) or "").strip()
    return UnknownEffect(raw_text=f"it_becomes_creature:type={m.group(2).strip()};pt={pt}")


# ---------------------------------------------------------------------------
# Reveal-counter driver used by Scent-cycle and Momir: "Reveal any
# number of green cards in your hand."
# ---------------------------------------------------------------------------
@_effect_rule(
    r"^reveal any number of ([a-z]+) cards in your hand\.?$"
)
def _reveal_any_number(m):
    return UnknownEffect(raw_text=f"reveal_any_number:{m.group(1)}")


# ---------------------------------------------------------------------------
# "Enchant ~" / "Enchant non-Wall creature" — pre-modern enchant
# subtype phrasing that slips the KEYWORD_RE "enchant [a-z ]+?" tail
# because ~ isn't lowercase alpha.
# ---------------------------------------------------------------------------
@_effect_rule(r"^enchant ~\.?$")
def _enchant_self_named(m):
    return UnknownEffect(raw_text="enchant:~")


# "Enchant non-wall creature" (Consuming Ferocity) — KEYWORD_RE stops at
# the first non-alpha char and "-" breaks it.
@_effect_rule(r"^enchant non-([a-z]+) creature\.?$")
def _enchant_non_subtype(m):
    return UnknownEffect(raw_text=f"enchant:non-{m.group(1)} creature")


# ===========================================================================
# __all__
# ===========================================================================

__all__ = ["TRIGGER_PATTERNS", "STATIC_PATTERNS", "EFFECT_RULES"]


# ===========================================================================
# Self-check (python3 extensions/old_templating.py)
# ===========================================================================

if __name__ == "__main__":
    samples_trigger = [
        "at end of turn, sacrifice ~",
        "at end of combat, destroy it",
        "at end of game, each player discards their hand",
        "at the end of each opponent's turn, draw a card",
        "at the end of your turn, you may exile ~",
        "when you control no islands, sacrifice this creature",
        "when you control no swamps, sacrifice this creature",
        "when an opponent casts a creature spell, it becomes a 3/3",
        "when that creature leaves the battlefield this turn, sacrifice this creature",
        "when a spell or ability an opponent controls causes you to discard this card, return this card from your graveyard",
        "when you cast a creature spell, sacrifice ~",
        "when ~ is turned face up, draw a card",
    ]
    samples_static = [
        "creatures with islandwalk can be blocked as though they didn't have islandwalk",
        "creatures with swampwalk can be blocked as though they didn't have swampwalk",
        "this land doesn't untap during your untap step",
        "this artifact doesn't untap during your untap step",
        "this land doesn't untap during your untap step if it has a depletion counter on it",
        "that land doesn't untap during its controller's next untap step",
        "you may choose not to untap this land during your untap step",
        "echo {2}{r}",
        "echo {4}",
        "rampage 1",
        "rampage 2",
        "buyback-sacrifice a land",
        "buyback-discard two cards",
        "remove this card from your deck before playing if you're not playing for ante",
        "this change in ownership is permanent",
        "cast this spell only after combat",
        "the new target must be a player",
        "the new target must be a creature",
    ]
    samples_effect = [
        "destroy it",
        "destroy that creature",
        "a creature destroyed this way can't be regenerated",
        "creatures destroyed this way can't be regenerated",
        "that creature can't be regenerated",
        "you may pay {w} to end this effect",
        "you may pay {u} to end this effect",
        "attach it to target creature",
        "attach it to another permanent it can enchant",
        "target creature gains flying until end of turn",
        "target creature gains deathtouch until end of turn",
        "target creature gains trample until end of turn",
        "target creature gets +x/+x until end of turn, where x is the number of cards revealed this way",
        "target creature gets -x/-x until end of turn, where x is the number of cards revealed this way",
        "target creature attacks this turn if able",
        "that creature attacks this turn if able",
        "tap any number of untapped creatures you control",
        "untap all creatures you control",
        "an opponent chooses one",
        "an opponent chooses one of those piles",
        "target opponent chooses a creature they control",
        "that player discards those cards",
        "target opponent discards two cards",
        "change the text of target spell or permanent by replacing all instances of one color word with another",
        "the next time a source of your choice would deal damage to you this turn, prevent that damage",
        "prevent x of that damage, where x is the amount of mana that player paid this way",
        "you gain 4 life for each creature tapped this way",
        "target creature becomes a 0/0 artifact creature",
        "it becomes a treefolk creature with haste and base power and toughness equal to this creature's power",
        "reveal any number of green cards in your hand",
        "enchant ~",
        "enchant non-wall creature",
    ]

    def _try(pats, s):
        for entry in pats:
            if len(entry) == 3:
                pat, _, _ = entry
            else:
                pat, _ = entry
            if pat.match(s.rstrip(".")):
                return True
        return False

    def _try_builder(pats, s):
        for pat, _ in pats:
            if pat.match(s.rstrip(".")):
                return True
        return False

    unmatched = []
    for s in samples_trigger:
        if not _try(TRIGGER_PATTERNS, s):
            unmatched.append(("TRIG", s))
    for s in samples_static:
        if not _try_builder(STATIC_PATTERNS, s):
            unmatched.append(("STATIC", s))
    for s in samples_effect:
        if not _try_builder(EFFECT_RULES, s):
            unmatched.append(("EFFECT", s))

    total = len(samples_trigger) + len(samples_static) + len(samples_effect)
    ok = total - len(unmatched)
    print(f"old_templating.py: {len(TRIGGER_PATTERNS)} triggers, "
          f"{len(STATIC_PATTERNS)} statics, {len(EFFECT_RULES)} effects")
    print(f"  sample coverage: {ok}/{total}")
    if unmatched:
        print("\n  UNMATCHED:")
        for kind, s in unmatched:
            print(f"    [{kind}] {s}")
        raise SystemExit(1)
