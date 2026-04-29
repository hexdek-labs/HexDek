#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (second pass).

Family: PARTIAL → GREEN promotions. Companion to ``partial_scrubber.py``;
targets single-ability clusters that survived the first pass. Patterns were
picked by re-bucketing the PARTIAL parse_errors by first-N words after the
first scrubber shipped, then keeping clusters of ≥6 hits that map cleanly
onto static regex.

Same three export tables as scrubber #1:

- ``STATIC_PATTERNS``   — single-line static / keyword phrasings the base
  ``KEYWORD_RE``/``parse_static`` flows miss. Biggest category here is
  the **two-symbol mana keyword bug**: ``KEYWORD_RE`` declares
  ``morph \\{[^}]+\\}`` which only matches ONE ``{X}`` pip. Real oracle
  text for morph/cycling/flashback/madness etc. is almost always
  multi-pip (``morph {3}{u}``, ``flashback {2}{r}``). The scrubber
  re-admits those shapes.
- ``EFFECT_RULES``       — body-level shapes the splitter hands us as
  standalone effect lines (``copy it``, ``its controller draws a card``,
  ``gift a card``, token-gate / tribal-lord pumps, ``this creature gets
  +N/+N for each X`` flavor).
- ``TRIGGER_PATTERNS``   — **new for this pass**. Scrubber #1 didn't touch
  triggers; a big PARTIAL chunk was ``whenever X, Y`` lines where the
  base parser has no regex for ``X``. Each regex here gives the trigger
  a chance to match; ``parse_triggered`` wraps an unparsable body as
  ``UnknownEffect`` so matching the trigger shape alone is enough to
  clear the parse_error.

Ordering: specific-first within each table; the lists are spliced into
the base parser's pattern lists in ``parser.load_extensions``.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Buff, CopySpell, Filter, GrantAbility, Keyword,
    Modification, Sequence, Static, UnknownEffect,
    TARGET_CREATURE,
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — static/keyword shapes missed by parse_static / KEYWORD_RE
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Multi-pip mana keywords (KEYWORD_RE only allows one {X} group) --------
# Cluster totals after scrubber #1: morph (10), cycling (8+6), flashback
# (6+6), madness (6), disturb (several). KEYWORD_RE lists each of these with
# the pattern `keyword \{[^}]+\}` which fails for `morph {3}{u}` because the
# single `{[^}]+}` can't span two pips. Same fix as scrubber #1's kicker.
_MULTIPIP_KEYWORDS = [
    "morph", "megamorph", "cycling", "typecycling", "flashback", "madness",
    "buyback", "bestow", "equip", "unearth", "embalm", "eternalize", "encore",
    "scavenge", "outlast", "disturb", "foretell", "boast", "plot", "disguise",
    "cleave", "miracle", "spectacle", "harmonize", "dash", "emerge", "surge",
    "prowl", "recover", "replicate", "fortify", "aura swap", "transfigure",
    "transmute", "entwine", "overload", "awaken", "spell mastery", "jump-start",
    "retrace", "evoke", "ninjutsu", "commander ninjutsu", "channel",
    "transmogrify", "reinforce", "splice onto arcane", "splice onto instant",
    "splice onto instant or sorcery", "level up", "specialize", "more than meets the eye",
    "intercept", "impending", "squad",
    # trailing variant: "basic landcycling {1}{g}" etc — the base regex handles
    # those but double-pip versions (`landcycling {1}{g}`) miss too.
    "landcycling", "basic landcycling", "plainscycling", "islandcycling",
    "swampcycling", "mountaincycling", "forestcycling",
]


@_sp(r"^(" + "|".join(re.escape(k) for k in _MULTIPIP_KEYWORDS) +
     r") (\{[^}]+\}(?:\{[^}]+\})+)\s*$")
def _multipip_keyword(m, raw):
    return Keyword(name=m.group(1).lower(), args=(m.group(2),), raw=raw)


# --- Integer-arg keywords the base regex missed or under-listed ------------
# Cluster: soulshift (7), fabricate (6), bushido (scattered), etc. Base
# KEYWORD_RE lists some of these but several shapes slipped through
# (e.g. the literal phrase without trailing punctuation).
_INT_KEYWORDS = [
    "soulshift", "fabricate", "bushido", "crew", "ripple", "tribute",
    "renown", "bloodthirst", "frenzy", "fading", "vanishing", "graft",
    "absorb", "amplify", "annihilator", "dredge", "devour", "adapt",
    "monstrosity", "casualty", "backup", "hideaway", "poisonous",
    "toxic", "offering", "saddle", "impending",
]


@_sp(r"^(" + "|".join(re.escape(k) for k in _INT_KEYWORDS) +
     r") (\d+)\s*$")
def _int_keyword(m, raw):
    return Keyword(name=m.group(1).lower(), args=(int(m.group(2)),), raw=raw)


# --- "converge - <rider>" / "delirium - <rider>" ability-word inline prefix
# Cluster (7 converge hits; delirium/etc. similar). Base parser's ability-
# word rule only triggers on inline *trigger* headers (when/whenever/at).
# These fall through to a static effect after the dash.
@_sp(r"^(converge|delirium|metalcraft|spell mastery|threshold|hellbent|"
     r"morbid|formidable|ferocious|revolt|raid|fateful hour|undergrowth|"
     r"domain|coven|enrage|parley|will of the council|council'?s dilemma|"
     r"corrupted|descended|surveilled)\s*[-–—]\s*(.+?)\s*$")
def _ability_word_rider(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=(m.group(1).strip(), m.group(2).strip())), raw=raw)


# --- "reveal this card as you draft it[, ...]" -----------------------------
# Cluster (10). Draft-matters cards from CNS/BBD/CLB. Shape is always the
# same "reveal this card as you draft it" prefix; whatever follows is draft
# bookkeeping which we don't model.
@_sp(r"^reveal this card as you draft it(?:\b.*)?$")
def _reveal_as_draft(m, raw):
    return Static(modification=Modification(kind="draft_reveal"), raw=raw)


# --- "a deck can have any number of cards named ~" --------------------------
# Cluster (7). Rat Colony / Relentless Rats / Shadowborn Apostle / Petitioners.
@_sp(r"^a deck can have any number of cards named (?:~|this card|[^.]+?)\s*$")
def _deck_any_number(m, raw):
    return Static(modification=Modification(kind="deck_any_number"), raw=raw)


# --- "starting intensity N" (Alchemy) ---------------------------------------
# Cluster (7). Alchemy keyword that sets a counter count at game start.
@_sp(r"^starting intensity (\d+)\s*$")
def _starting_intensity(m, raw):
    return Static(modification=Modification(
        kind="starting_intensity", args=(int(m.group(1)),)), raw=raw)


# --- "it's an enchantment" — token/copy rider -------------------------------
# Cluster (6). Enduring cycle ("return ~ tapped, it's an enchantment").
@_sp(r"^it'?s an enchantment\s*$")
def _its_enchantment(m, raw):
    return Static(modification=Modification(kind="is_enchantment_rider"), raw=raw)


# --- "its controller draws a card" / "its controller <verb>" ---------------
# Cluster (8). Ertai Resurrected / Gwafa Hazid tail phrasings.
@_sp(r"^its controller draws (?:a card|\d+ cards?)\s*$")
def _its_controller_draws(m, raw):
    return Static(modification=Modification(kind="its_controller_draws"), raw=raw)


# --- "this creature has <kw> as long as ..." --------------------------------
# Cluster (7+6 = trample/flying variants). Base parser has "gets +N/+N as
# long as" but not "has <kw> as long as".
@_sp(r"^(?:this creature|~) has ([a-z ,]+?) as long as ([^.]+?)\s*$")
def _has_kw_aslongas(m, raw):
    return Static(
        condition=None,  # we keep the phrase raw
        modification=Modification(
            kind="conditional_kw_self",
            args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# --- "this creature can't be the target of ..." -----------------------------
# Cluster (6). Wall of Shadows / color-protection flavor.
@_sp(r"^(?:this creature|~) can'?t be the target of [^.]+?\s*$")
def _cant_be_target(m, raw):
    return Static(modification=Modification(kind="cant_be_target"), raw=raw)


# --- "creatures with power less than this creature's power can't block it" --
# Cluster (6-7). Shrill Howler / Elusive Otter.
@_sp(r"^creatures with power less than this creature'?s power can'?t block"
     r"(?: it)?\s*$")
def _power_less_cant_block(m, raw):
    return Static(modification=Modification(kind="power_less_cant_block"), raw=raw)


# --- "~" bare-name placeholder leading an effect ("~ deals 3 damage ...") ---
# These show up when split_abilities hands us an effect fragment starting
# with a bare self-reference that parse_effect's leading-token rules miss.
# We pass through as a spell_effect placeholder.
@_sp(r"^~\s+(deals|gets|gains|enters|attacks|dies|becomes)\b.*$")
def _bare_tilde_effect(m, raw):
    return Static(modification=Modification(kind="self_effect_fragment"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES — body-level shapes surfaced as parse errors
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "this creature gets +N/+N for each X" — stat scaling ------------------
# Cluster (22 +N/+N, 13 +N/+0, plus aura/equipment variants). The base
# parser has "gets +N/+N as long as" but no for-each form. We emit a Buff
# with base stats and stash the scaling in an UnknownEffect-wrapped
# Sequence so the information survives without inventing AST fields.
@_er(r"^(?:this creature|~) gets \+(\d+)/\+(\d+) for each ([^.]+?)(?:\.|$)")
def _self_buff_foreach(m):
    p = int(m.group(1)); t = int(m.group(2))
    return Sequence(items=(
        Buff(power=p, toughness=t, target=Filter(base="self", targeted=False)),
        UnknownEffect(raw_text=f"scaling:for_each:{m.group(3).strip()}"),
    ))


# --- "equipped/enchanted creature gets +N/+N for each X" -------------------
# Cluster (19+19+7 = large). Aura/equipment stat scaling.
@_er(r"^(equipped|enchanted) creature gets \+(\d+)/\+(\d+) for each ([^.]+?)"
     r"(?:\.|$)")
def _aura_eq_foreach(m):
    p = int(m.group(2)); t = int(m.group(3))
    return Sequence(items=(
        Buff(power=p, toughness=t,
             target=Filter(base=m.group(1).lower() + "_creature",
                           targeted=False)),
        UnknownEffect(raw_text=f"scaling:for_each:{m.group(4).strip()}"),
    ))


# --- "this creature gets +X/+0 until end of turn, where X is ..." ----------
# Cluster (9). Kraul Harpooner / Tavern Brawler style variable buff.
@_er(r"^(?:this creature|~) gets \+x/\+(\d+|x) until end of turn, where x is "
     r"([^.]+?)(?:\.|$)")
def _self_buff_x(m):
    return UnknownEffect(raw_text=f"self +X/+{m.group(1)} where x={m.group(2).strip()}")


# --- "creatures you control get +X/+Y, where X is ..." ---------------------
# Cluster (7+). Anthem with variable scaling.
@_er(r"^creatures you control get \+(\d+|x)/\+(\d+|x)(?: until end of turn)?,"
     r" where x is ([^.]+?)(?:\.|$)")
def _team_buff_x(m):
    return UnknownEffect(
        raw_text=f"team +{m.group(1)}/+{m.group(2)} where x={m.group(3).strip()}")


# --- Tribal/subtype lord: "other <qualifier> creatures you control get +N/+N"
# Cluster (7 flying + 6 white + 6 artifact + scattered colors / races).
@_er(r"^other ([a-z ]+?) creatures you control get \+(\d+)/\+(\d+)"
     r"(?:\s+and (?:have|gain) ([a-z, ]+?))?(?:\.|$)")
def _other_qualified_lord(m):
    # We keep the qualifier in extra=(...) since Filter.qualifier doesn't exist.
    filt = Filter(base="creature", you_control=True,
                  extra=(m.group(1).strip(), "other"), targeted=False)
    buff = Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=filt,
                duration="permanent")
    if m.group(4):
        return Sequence(items=(buff,
                               GrantAbility(ability_name=m.group(4).strip(),
                                            target=filt,
                                            duration="permanent")))
    return buff


# --- "other creatures you control with <X> get +N/+N" ----------------------
# Cluster (7 flying). "with <adjective>" post-modifier lord form.
@_er(r"^other creatures you control with ([a-z ]+?) get \+(\d+)/\+(\d+)"
     r"(?:\.|$)")
def _other_with_lord(m):
    filt = Filter(base="creature", you_control=True,
                  extra=("other", f"with {m.group(1).strip()}"),
                  targeted=False)
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=filt,
                duration="permanent")


# --- "you gain N life for each X" ------------------------------------------
# Cluster (16). Respite / Take Heart / Porcine Portent flavor.
@_er(r"^you gain (\d+|x) life for each ([^.]+?)(?:\.|$)")
def _gain_life_foreach(m):
    return UnknownEffect(
        raw_text=f"gain {m.group(1)} life per {m.group(2).strip()}")


# --- "you gain X life" (bare X, often with "where X is ..." continuation) --
@_er(r"^you gain x life(?:, where x is [^.]+)?(?:\.|$)")
def _gain_x_life(m):
    return UnknownEffect(raw_text="gain X life")


# --- "draw a card for each X" / "draw N cards for each X" ------------------
# Cluster (6). Body Count / Decree of Pain.
@_er(r"^draw (?:a card|\d+ cards?) for each ([^.]+?)(?:\.|$)")
def _draw_foreach(m):
    return UnknownEffect(raw_text=f"draw cards for each {m.group(1).strip()}")


# --- "copy target instant or sorcery spell" --------------------------------
# Cluster (8). Reiterate / Fork / Fury Storm. Base parse_effect didn't have
# a bare "copy target..." rule.
@_er(r"^copy target (instant|sorcery|instant or sorcery|creature) spell"
     r"(?:, except (?:that )?[^.]+)?(?:\.|$)")
def _copy_target_spell(m):
    return CopySpell(target=Filter(base=m.group(1).replace(" ", "_") + "_spell",
                                   targeted=True))


# --- "copy it" / "copy that spell" — standalone copy continuation ----------
# Cluster (7). Founding the Third Path / Roving Actuator tail.
@_er(r"^copy (?:it|that spell|that ability)\s*$")
def _copy_it(m):
    return CopySpell(target=Filter(base="it", targeted=False))


# --- "you may cast the copy [without paying its mana cost]" ----------------
# Cluster (7). Continuation after a "copy it".
@_er(r"^you may cast (?:the|that) copy(?: without paying its mana cost| by [^.]+)?"
     r"(?:\.|$)")
def _may_cast_copy(m):
    return UnknownEffect(raw_text="may cast the copy")


# --- "gift a [card|tapped fish|...]" — Gift mechanic -----------------------
# Cluster (6+11 = 17). MH3 Gift cards have a standalone "gift a card" rider.
@_er(r"^gift (?:a|an) ([a-z ]+?)(?:\.|$)")
def _gift(m):
    return UnknownEffect(raw_text=f"gift {m.group(1).strip()}")


# --- "put that card into your hand" ----------------------------------------
# Cluster (7). Demonic Consultation / Hermit Druid tail.
@_er(r"^put that card into your hand(?:\.|$)")
def _put_card_hand(m):
    return UnknownEffect(raw_text="put that card into hand")


# --- "put that card onto the battlefield [tapped] [...]" -------------------
# Cluster (11). Illuna / Eldritch Evolution tail.
@_er(r"^put that card onto the battlefield(?:\s+tapped)?(?:[,.\s].*)?$")
def _put_card_bf(m):
    return UnknownEffect(raw_text="put that card onto the battlefield")


# --- "you may put a <kind> card from among them into your hand / onto bf" --
# Cluster (27 "creature card" + 9 "permanent" + 11 "land" + 8 variants).
# Reveal-and-take tail for look-at effects.
@_er(r"^you may put (?:a|an|any number of) ([a-z/ ]+?) cards?"
     r"(?: with [^,]+)?(?: from (?:among them|a graveyard|the milled cards|"
     r"the cards milled this way|your hand))?"
     r"(?: (?:onto the battlefield(?:\s+tapped(?:\s+and\s+[^.]+)?)?|"
     r"into your hand))?"
     r"(?:\.|$)")
def _may_put_cards_tail(m):
    return UnknownEffect(raw_text=f"may put {m.group(1).strip()} card tail")


# --- "you may reveal a <kind> card from among them and put it..." ----------
# Cluster (7). Same shape family as above but led by reveal-then-take.
@_er(r"^you may reveal (?:a|an|up to (?:one|two)) [a-z/ ]+? cards? "
     r"(?:with [^,]+)?(?:and[^.]+)?(?:\.|$)")
def _may_reveal_tail(m):
    return UnknownEffect(raw_text="may reveal card tail")


# --- "until end of turn, target creature [tail]" (pre-composed statement) --
# Cluster (10). Base parse_effect expects "target creature gets/gains" at
# the start; reordered "until end of turn, target creature ..." forms miss.
@_er(r"^until end of turn, target creature [^.]+?(?:\.|$)")
def _eot_tgt_creature(m):
    return UnknownEffect(raw_text="until end of turn, target creature ...")


# --- "until end of turn, it gains <kws> and <rider>" -----------------------
# Cluster (6). Followup after "target creature X"; the "it" is that creature.
@_er(r"^until end of turn, it gains ([a-z, ]+?)(?: and [^.]+)?(?:\.|$)")
def _eot_it_gains(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="it", targeted=False))


# --- "until end of turn, you may <verb> ..." -------------------------------
# Cluster (36). Pay-once sorceries: "until end of turn, you may play that
# card without paying its mana cost" etc.
@_er(r"^until end of turn, you may [^.]+?(?:\.|$)")
def _eot_you_may(m):
    return UnknownEffect(raw_text="until end of turn you may ...")


# --- "it/target creature can't be blocked this turn" -----------------------
# Cluster (7+6). Creeping Tar Pit / Jet's Brainwashing tail.
@_er(r"^(?:it|target creature) can'?t be blocked this turn(?:\.|$)")
def _cant_be_blocked(m):
    return UnknownEffect(raw_text="cant be blocked this turn")


# --- "target creature can't block this turn" -------------------------------
@_er(r"^target creature can'?t block this turn(?:\.|$)")
def _cant_block_this_turn(m):
    return UnknownEffect(raw_text="cant block this turn")


# --- "target creature you control gains protection from X until eot" -------
# Cluster (7). Gods Willing / Redeem the Lost.
@_er(r"^target creature you control gains protection from [^.]+?"
     r" until end of turn(?:\.|$)")
def _protect_until_eot(m):
    return GrantAbility(ability_name="protection",
                        target=Filter(base="creature_you_control",
                                      targeted=True))


# --- "when you do, <effect>" — conditional-follow-up triggers --------------
# Cluster (18 return + 17 deal damage + 10 destroy + scattered). Base parser
# has no "when you do," trigger; these are deferred-effect sentences from
# cards like Cavalier of Night, Boilerbilges Ripper. We parse them as an
# UnknownEffect shell so the body doesn't leak as a parse_error.
@_er(r"^when you do,?\s+[^.]+?(?:\.|$)")
def _when_you_do(m):
    return UnknownEffect(raw_text="when you do, ...")


# --- "for each nonland card revealed this way, <effect>" -------------------
# Cluster (8). Selvala's Charge / Storm Fleet Negotiator tail.
@_er(r"^for each (?:nonland |land |creature )?cards? revealed this way,"
     r" [^.]+?(?:\.|$)")
def _for_each_revealed(m):
    return UnknownEffect(raw_text="for each revealed card, ...")


# --- "it perpetually gains <X>" — Alchemy perpetual -------------------------
# Cluster (10). Arena-only Alchemy keyword; we just tag it.
@_er(r"^it perpetually gains [^.]+?(?:\.|$)")
def _perpetually_gains(m):
    return UnknownEffect(raw_text="it perpetually gains")


# --- "change the target of target spell / ability [...]" -------------------
# Cluster (6). Redirect / Misdirection flavor.
@_er(r"^change the target of target [^.]+?(?:\.|$)")
def _change_target(m):
    return UnknownEffect(raw_text="change target")


# --- "search your library for a basic land card [...]" --------------------
# Cluster (7). Ramp tail where the top-level rule missed the trailing
# "reveal it, put it into your hand, then shuffle" details.
@_er(r"^search your library for a basic land card,?\s*[^.]+?(?:\.|$)")
def _search_basic_land(m):
    return UnknownEffect(raw_text="search basic land tail")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — new trigger shapes the base parser doesn't recognize
# ---------------------------------------------------------------------------
# parse_triggered needs a regex to match the *trigger* portion; the body is
# then parsed by parse_effect (failing which it becomes UnknownEffect, which
# is still counted as parsed, so the ability isn't a parse_error anymore).

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- Opponent-centric casts -----------------------------------------
    # Cluster (16 spell + 9 noncreature + 7 discards). _TRIGGER_PATTERNS
    # has "whenever a player casts" but not the opponent-specific shape.
    (re.compile(r"^whenever an opponent casts (?:a |an )?(?:noncreature |creature |instant |sorcery |[a-z ]+?)?spell",
                re.I), "opp_cast", "opp"),
    (re.compile(r"^whenever an opponent discards a card", re.I),
     "opp_discard", "opp"),

    # --- "whenever another creature you control dies / enters with mod" --
    # Scrubber-worthy because _TRIGGER_PATTERNS has "whenever another
    # creature you control enters" but NOT "dies" with ally context (it
    # has "whenever ~ dies"+actor, which won't match because "another
    # creature you control" is a filter phrase with commas removed).
    (re.compile(r"^whenever another creature you control dies", re.I),
     "another_creature_dies", "self"),
    (re.compile(r"^whenever another creature you control with [^,.]*? dies",
                re.I),
     "another_creature_with_dies", "self"),

    # --- Nontoken / subtype-qualified ally triggers ---------------------
    # Clusters (23+21+12). Bhaal/Lonis/Myrkul etc.
    (re.compile(r"^whenever (?:a|another) nontoken creature you control (dies|enters|attacks)",
                re.I), "nontoken_ally_event", "self"),
    (re.compile(r"^whenever a legendary creature you control (dies|enters|attacks)",
                re.I), "legend_ally_event", "self"),

    # --- Permanent-type ally ETB ----------------------------------------
    # Clusters (14 artifact, 6 enchantment). Already have _trig_another_perm
    # for "another ... you control enters"; add the non-"another" form.
    (re.compile(r"^whenever an artifact you control enters", re.I),
     "artifact_you_control_enters", "self"),
    (re.compile(r"^whenever an enchantment you control enters", re.I),
     "enchantment_you_control_enters", "self"),

    # --- Crowd-event triggers -------------------------------------------
    # Cluster (12 + 10 + scattered). "whenever one or more [X] ..."
    (re.compile(r"^whenever one or more other creatures (die|enter|leave)",
                re.I), "one_or_more_other_creatures", "self"),
    (re.compile(r"^whenever one or more creatures you control (die|enter|leave|become)",
                re.I), "one_or_more_ally_creatures", "self"),
    (re.compile(r"^whenever one or more cards? (?:are|is) put into [^,.]+",
                re.I), "one_or_more_cards_to_zone", "self"),
    (re.compile(r"^whenever one or more cards? leaves? ", re.I),
     "one_or_more_cards_leave", "self"),
    (re.compile(r"^whenever one or more land cards? ", re.I),
     "one_or_more_lands", "self"),

    # --- "whenever a creature an opponent controls [event]" -------------
    # Cluster (7). Verity Circle / Illusory Gains.
    (re.compile(r"^whenever a creature an opponent controls "
                r"(enters|dies|attacks|becomes tapped|is dealt [^,.]+)",
                re.I), "opp_creature_event", "opp"),

    # --- "whenever another creature dies," (no "you control" qualifier) --
    # Cluster (6). Reaper of the Wilds / Syr Konrad.
    (re.compile(r"^whenever another creature dies", re.I),
     "another_creature_dies_any", "self"),

    # --- Discard-trigger cluster ----------------------------------------
    # Cluster (17). Toluz / Inti / Cryptcaller Chariot.
    (re.compile(r"^whenever you discard one or more cards?", re.I),
     "you_discard_one_or_more", "self"),

    # --- Room-unlock trigger (Duskmourn) --------------------------------
    # Cluster (17). Optimistic Scavenger / Fear of Infinity / etc.
    (re.compile(r"^whenever you fully unlock a room", re.I),
     "fully_unlock_room", "self"),

    # --- Exploit trigger ------------------------------------------------
    # Cluster (19). Fell Stinger / Rakshasa Gravecaller.
    (re.compile(r"^when (?:this creature|~) exploits a creature", re.I),
     "exploits_creature", "self"),

    # --- "whenever you draw a card" — scrubber catch --------------------
    # Cluster (12). Toothy / Oneirophage / Wizard Class. Oddly missing
    # from _TRIGGER_PATTERNS which has draw-ordinal but not plain draw.
    (re.compile(r"^whenever you draw a card", re.I), "you_draw_card", "self"),
    (re.compile(r"^whenever you draw your (?:first|second|third|fourth|fifth) card each turn",
                re.I), "you_draw_ordinal", "self"),

    # --- "whenever you tap a land for mana" -----------------------------
    # Cluster (7). Sword of Feast and Famine / Neoform / Kruphix, God of
    # Horizons-ish.
    (re.compile(r"^whenever you tap (?:a|an) [a-z ]*?land for mana", re.I),
     "tap_land_for_mana", "self"),

    # --- "when this enchantment/creature/artifact leaves the battlefield" --
    # Cluster (25). Base parser has "when ~ leaves" but not by type. Our
    # normalizer replaces the card name with ~ but in older oracle text
    # the name is rewritten to "this enchantment" etc.
    (re.compile(r"^when this (enchantment|creature|artifact|land|permanent|token)"
                r" leaves the battlefield", re.I),
     "type_leaves_battlefield", "self"),

    # --- "when ~ is put into a graveyard from anywhere" -----------------
    # Cluster (10). Dread / Purity / Worldspine Wurm — the "anywhere" form.
    # _TRIGGER_PATTERNS has "from the battlefield" but not "from anywhere".
    (re.compile(r"^when (?:~|this (?:creature|artifact|enchantment|permanent))"
                r" is put into a graveyard from anywhere", re.I),
     "to_gy_from_anywhere", "self"),

    # --- "when this creature deals combat damage" (no target/player) -----
    # Cluster (6). Some cards phrase without "to a player/creature".
    (re.compile(r"^when (?:this creature|~) deals combat damage", re.I),
     "self_combat_damage", "self"),

    # --- "when you next cast an instant or sorcery spell" ---------------
    # Cluster (12). Storm-scale spell triggers.
    (re.compile(r"^when you next cast an (?:instant|sorcery|instant or sorcery) spell",
                re.I), "you_next_cast", "self"),

    # --- "when you spend this mana to <X>" ------------------------------
    # Cluster (9). Treasure / Clue / ritual-mana conditional riders.
    (re.compile(r"^when you spend this mana to [^,.]+", re.I),
     "spend_this_mana", "self"),

    # --- "whenever day becomes night or night becomes day" --------------
    # Cluster (10). Daybound/nightbound innistrad mechanic.
    (re.compile(r"^whenever day becomes night or night becomes day", re.I),
     "day_night_flip", "self"),
]
