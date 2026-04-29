#!/usr/bin/env python3
"""Anaphoric pronoun chain extension.

Family: ANAPHORIC PRONOUN CHAINS. Targets sentences and clauses that refer
back to a previously-named entity via pronouns ("it", "that creature",
"that player", "those cards", "they", "them", "its controller"). These
almost always arrive at the parser as standalone sentences after the
ability-splitter separates them from their antecedent — so from the
parser's POV they're pronoun-led fragments that need their own productions.

The shape the base parser already handles (in parser.py):
  - "it gains KW until end of turn"  → _it_gains
  - "it gets +N/+N until end of turn" → _it_gets
  - "it gains haste" (no duration)     → parse_static pronoun_grant
  - "they can't be regenerated"        → _no_regen_them
  - "untap that creature"              → _untap_that
  - "they gain haste until end of turn"→ _they_gain_haste
  - "that player discards X"           → _player_discards_chained
  - "that creature doesn't untap ..."  → parse_static stun_target

What's still leaking into PARTIAL (from a bucket of parse_errors whose
first token is it/that/those/they/its/them):
  - "its controller {verb}s ..."  (draws/loses/sacrifices/discards/creates)
  - "that player {verb}s ..."  with no duration modifier
  - "that creature/card becomes [type]"
  - "it becomes [type]" / "it becomes [token-kind]"
  - "they're N/N [type] creatures" (token type-set)
  - "they gain KW [and KW] until end of turn"  (multi-kw)
  - "they each get +N/+N until end of turn"
  - "that creature can't attack/block [this turn]"
  - "it fights target creature ..."  (body of a deathtouch/fight trigger)
  - "it must be blocked this turn if able"
  - "it perpetually gets +N/+N" / "it perpetually gains KW" (Alchemy)
  - "that creature's controller {verb}s ..."
  - "its controller may search their library for a basic land"
  - "they block this turn if able"

All of these are **statics or single-effect sentences attached to an
upstream antecedent**; none introduce new triggers. So we emit them as
STATIC_PATTERNS (for Static-shaped chained clauses) plus a handful of
EFFECT_RULES for pronoun-led verb clauses that can feed into
``parse_effect`` and thereby into the spell-effect fallback.

Ordering: specific-first. Every pattern is anchored with ``^`` so it only
matches when the pronoun is at the start of the clause — we never want to
claim a pronoun mid-sentence.
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
    Buff, CreateToken, Damage, Discard, Draw, Fight, Filter, GainLife,
    GrantAbility, LoseLife, Modification, Sacrifice, Sequence, Static,
    UnknownEffect,
    TARGET_CREATURE, TARGET_PLAYER,
)


# Shared that-thing filter — used anywhere we need to refer to the
# antecedent without repeating the base name.
_THAT_CREATURE = Filter(base="that_creature", targeted=False)
_THAT_PLAYER = Filter(base="that_player", targeted=False)
_THEM = Filter(base="them", targeted=False)
_THAT_CONTROLLER = Filter(base="that_controller", targeted=False)
_THOSE_CARDS = Filter(base="those_cards", targeted=False)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — pronoun-led clauses that ride on an antecedent
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "it gains KW [and KW] [until end of turn]" -- multi-keyword grants -----
# Base parser only handles a single keyword run ("it gains haste ..."), but
# oracle text routinely chains them ("it gains menace and haste until end of
# turn", "they gain vigilance and lifelink until end of turn").
@_sp(r"^(?:it|they) gains? ([a-z][a-z, ]*?(?:\s+and\s+[a-z]+)?)(?:\s+until end of turn)?\s*$")
def _it_gains_multi(m, raw):
    kws = m.group(1).strip()
    return Static(modification=Modification(kind="pronoun_grant_multi",
                                            args=(kws,)), raw=raw)


# --- "they each get +N/+N until end of turn" / "they gain KW until eot" ----
@_sp(r"^they each get [+\-]\d+/[+\-]\d+(?:\s+until end of turn)?\s*$")
def _they_each_get(m, raw):
    return Static(modification=Modification(kind="pronoun_them_each_buff",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it gets +N/+N and gains KW until end of turn" ------------------------
@_sp(r"^it gets [+\-]\d+/[+\-]\d+ and gains? [a-z, ]+(?:\s+until end of turn)?\s*$")
def _it_gets_and_gains(m, raw):
    return Static(modification=Modification(kind="pronoun_buff_and_grant",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it perpetually gets +N/+N" / "it perpetually gains KW" (Alchemy) -----
@_sp(r"^it perpetually (?:gets [+\-]\d+/[+\-]\d+|gains? [^.]+|becomes [^.]+)\s*$")
def _it_perpetually(m, raw):
    return Static(modification=Modification(kind="perpetual_mod",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that card perpetually gets/gains/becomes ..." ------------------------
@_sp(r"^that card perpetually (?:gets [+\-]\d+/[+\-]\d+|gains? [^.]+|becomes [^.]+)\s*$")
def _that_card_perpetually(m, raw):
    return Static(modification=Modification(kind="perpetual_mod_card",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it gets an additional -N/-N until end of turn for each ..." ----------
@_sp(r"^it gets an additional [+\-]\d+/[+\-]\d+ until end of turn for each [^.]+\s*$")
def _it_additional_per(m, raw):
    return Static(modification=Modification(kind="pronoun_buff_per",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it gets +N/+x until end of turn, where X is ..." ---------------------
@_sp(r"^(?:it|that creature) gets [+\-]\d+/[+\-]x until end of turn,? where x is [^.]+\s*$")
def _it_variable_buff(m, raw):
    return Static(modification=Modification(kind="pronoun_buff_variable",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it becomes [type]" / "it becomes a [type]" ---------------------------
# "it becomes foretold", "it becomes night", "it becomes an angel in addition
# to its other types"
@_sp(r"^it becomes (?:a |an )?[^.]+\s*$")
def _it_becomes(m, raw):
    return Static(modification=Modification(kind="pronoun_becomes",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that creature becomes [type] in addition to its other types" ---------
@_sp(r"^that creature becomes (?:a |an )?[^.]+\s+in addition to its other (?:types|colors)\s*$")
def _that_creature_becomes(m, raw):
    return Static(modification=Modification(kind="that_creature_type_add",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that creature is a [color] [type] in addition to its other ..." ------
@_sp(r"^that creature is (?:a |an )?[^.]+\s+in addition to its other (?:types|colors)\s*$")
def _that_creature_is(m, raw):
    return Static(modification=Modification(kind="that_creature_type_add",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "they're N/N [type] [creature] tokens" / "they're [type]" -------------
# Token type-set chained after a create-tokens sentence ("Create two 2/2
# tokens. They're black Zombies in addition to their other colors").
@_sp(r"^they'?re \d+/\d+ [a-z ]+?(?:\s+(?:artifact|creature)\s+(?:creature\s+)?tokens?)?\s*$")
def _theyre_pt_type(m, raw):
    return Static(modification=Modification(kind="token_pt_type_rider",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^they'?re (?:tapped|attacking|tapped and attacking|untapped|blocking)\s*$")
def _theyre_state(m, raw):
    return Static(modification=Modification(kind="pronoun_state",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^they'?re [a-z][a-z ]+\s+in addition to their other (?:types|colors)\s*$")
def _theyre_type_add(m, raw):
    return Static(modification=Modification(kind="token_type_add",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "they have \"[ability]\"" / "they have [KW]" --------------------------
@_sp(r"^they have [\"][^\"]+[\"]\s*$")
def _they_have_quoted(m, raw):
    return Static(modification=Modification(kind="them_granted_ability",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "they block this turn if able" / "they attack this turn if able" ------
@_sp(r"^they (?:block|attack) this turn if able\s*$")
def _they_must_combat(m, raw):
    return Static(modification=Modification(kind="pronoun_must_combat",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it must be blocked this turn if able" --------------------------------
@_sp(r"^it must be blocked this turn if able\s*$")
def _it_must_be_blocked(m, raw):
    return Static(modification=Modification(kind="pronoun_must_be_blocked"),
                  raw=raw)


# --- "that creature can't attack/block [this turn/combat]" ------------------
@_sp(r"^that creature can'?t (?:attack|block|be blocked)(?: this turn| this combat)?\s*$")
def _that_creature_cant(m, raw):
    return Static(modification=Modification(kind="that_creature_combat_restriction",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that creature can't attack or block for as long as ..." --------------
@_sp(r"^that creature can'?t attack or block for as long as [^.]+\s*$")
def _that_creature_cant_long(m, raw):
    return Static(modification=Modification(kind="that_creature_combat_lockout",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that creature loses flying until end of turn" / "... loses KW" -------
@_sp(r"^that creature loses [a-z, ]+(?:\s+until end of turn)?\s*$")
def _that_creature_loses(m, raw):
    return Static(modification=Modification(kind="that_creature_ability_loss",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it enters tapped and attacking" (attacking rider) --------------------
@_sp(r"^it enters tapped and attacking\s*$")
def _it_enters_t_and_a(m, raw):
    return Static(modification=Modification(kind="enters_tapped_attacking"),
                  raw=raw)


# --- "it deals damage equal to its power to [filter]" — Fight-style body ---
# Base parser has a one-off rule, but not anchored in parse_static.
@_sp(r"^it deals damage equal to its power to [^.]+\s*$")
def _it_deals_power(m, raw):
    return Static(modification=Modification(kind="pronoun_fight_body",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it fights target creature [filter]" (chained fight body) -------------
@_sp(r"^it fights target (?:creature|[a-z ]+?creature[a-z ]*?)(?:\s+an opponent controls| you don'?t control| you control)?\s*$")
def _it_fights(m, raw):
    return Static(modification=Modification(kind="pronoun_fights",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "they each deal damage equal to their power to target creature" -------
@_sp(r"^they each deal damage equal to their power to target [^.]+\s*$")
def _they_each_fight(m, raw):
    return Static(modification=Modification(kind="pronoun_them_each_fight",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it deals N damage to its controller" ---------------------------------
@_sp(r"^it deals \d+ damage to its controller\s*$")
def _it_deals_controller(m, raw):
    return Static(modification=Modification(kind="pronoun_damage_controller",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it doesn't untap during its controller's next untap step" ------------
@_sp(r"^(?:it|that creature|that permanent) doesn'?t untap during (?:its|their|your) (?:controller'?s? )?(?:next )?untap step\s*$")
def _it_stun(m, raw):
    return Static(modification=Modification(kind="pronoun_stun"), raw=raw)


# --- "its controller {verb}s ..." chain --------------------------------------
# These are the chained result sentences after something like "reveal the top
# card of target opponent's library. Its controller loses 3 life and you
# gain 3 life." Very common on mill/discard/reveal tails.
@_sp(r"^its controller loses \d+ life(?: and you gain \d+ life)?\s*$")
def _its_controller_loses(m, raw):
    return Static(modification=Modification(kind="chain_controller_loses",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^its controller loses life equal to [^.]+\s*$")
def _its_controller_loses_var(m, raw):
    return Static(modification=Modification(kind="chain_controller_loses_var",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^its controller (?:draws|mills|discards|sacrifices|exiles|reveals) [^.]+\s*$")
def _its_controller_verbs(m, raw):
    return Static(modification=Modification(kind="chain_controller_verb",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^its controller creates (?:a |an |\d+ )[^.]+\s*$")
def _its_controller_creates(m, raw):
    return Static(modification=Modification(kind="chain_controller_creates",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^its controller may [^.]+\s*$")
def _its_controller_may(m, raw):
    return Static(modification=Modification(kind="chain_controller_may",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that creature's controller / that card's controller ..." -------------
@_sp(r"^that (?:creature|card|permanent|player)'?s? controller [a-z]+s? [^.]+\s*$")
def _that_x_controller_verbs(m, raw):
    return Static(modification=Modification(kind="chain_possessive_controller",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "that player {verb}s ..." -- chained player action ---------------------
# Base parser has _player_discards_chained; we cover the other common verbs.
@_sp(r"^that player loses \d+ life\s*$")
def _that_player_loses(m, raw):
    return Static(modification=Modification(kind="chain_that_player_loses",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player gains \d+ life\s*$")
def _that_player_gains(m, raw):
    return Static(modification=Modification(kind="chain_that_player_gains",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player draws (?:a|one|two|three|x|\d+) cards?\s*$")
def _that_player_draws(m, raw):
    return Static(modification=Modification(kind="chain_that_player_draws",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player discards (?:a|one|two|three|x|\d+) cards?\s*$")
def _that_player_discards_n(m, raw):
    return Static(modification=Modification(kind="chain_that_player_discards",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player mills? \d+ cards?\s*$")
def _that_player_mills(m, raw):
    return Static(modification=Modification(kind="chain_that_player_mills",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player sacrifices [^.]+\s*$")
def _that_player_sacrifices(m, raw):
    return Static(modification=Modification(kind="chain_that_player_sacrifices",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player exiles [^.]+\s*$")
def _that_player_exiles(m, raw):
    return Static(modification=Modification(kind="chain_that_player_exiles",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player shuffles,? then draws a card for each [^.]+\s*$")
def _that_player_shuffle_draw_each(m, raw):
    return Static(modification=Modification(kind="chain_that_player_shuffle_draw",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player puts that card onto the battlefield,? then shuffles[^.]*\s*$")
def _that_player_puts_card(m, raw):
    return Static(modification=Modification(kind="chain_that_player_puts_card",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^that player may cast that card without paying its mana cost\s*$")
def _that_player_may_cast_free(m, raw):
    return Static(modification=Modification(kind="chain_that_player_cast_free"),
                  raw=raw)


# --- "that creature attacks each combat/turn if able" ----------------------
@_sp(r"^that creature attacks each (?:combat|turn) if able\s*$")
def _that_creature_must_attack(m, raw):
    return Static(modification=Modification(kind="that_creature_must_attack"),
                  raw=raw)


# --- "those cards are exiled" / "those cards leave the battlefield" --------
@_sp(r"^those cards are exiled\s*$")
def _those_exiled(m, raw):
    return Static(modification=Modification(kind="those_cards_exiled"), raw=raw)


@_sp(r"^those cards leave the battlefield(?: at end of turn)?\s*$")
def _those_leave(m, raw):
    return Static(modification=Modification(kind="those_cards_leave"), raw=raw)


@_sp(r"^they (?:may )?cast that card without paying its mana cost\s*$")
def _they_cast_free(m, raw):
    return Static(modification=Modification(kind="them_cast_free"), raw=raw)


# --- "if you do, [effect]" / "if you don't, [effect]" fallback -------------
# Base splitter already merges these with their antecedent sentence, but a
# handful slip through as orphans (modal bullets, inline choice bodies).
@_sp(r"^if you do,? [^.]+\s*$")
def _if_you_do(m, raw):
    return Static(modification=Modification(kind="if_you_do_orphan",
                                            args=(raw.strip().lower(),)), raw=raw)


@_sp(r"^if you don'?t,? [^.]+\s*$")
def _if_you_dont(m, raw):
    return Static(modification=Modification(kind="if_you_dont_orphan",
                                            args=(raw.strip().lower(),)), raw=raw)


# --- "it can't be regenerated" / mirror of "they can't be regenerated" -----
@_sp(r"^it can'?t be regenerated\s*$")
def _it_no_regen(m, raw):
    return Static(modification=Modification(kind="no_regen_tail_it"), raw=raw)


# --- "tap that creature." "sacrifice it." chained imperatives --------------
@_sp(r"^tap (?:it|them|that permanent|that card)\s*$")
def _tap_pronoun_chain(m, raw):
    return Static(modification=Modification(kind="chain_tap_pronoun"), raw=raw)


@_sp(r"^destroy (?:it|them|that creature|that permanent)\s*$")
def _destroy_pronoun_chain(m, raw):
    return Static(modification=Modification(kind="chain_destroy_pronoun"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES — pronoun-led verb clauses consumable by parse_effect
# ---------------------------------------------------------------------------
# Effect rules are consulted by parse_effect and are additive to the base
# parser's EFFECT_RULES list. Because parse_effect enforces full-string
# consumption, we only add patterns we can fully consume. Each returns an
# appropriate typed Effect node (or UnknownEffect with the shape captured).

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


@_er(r"^that player loses (\d+) life(?:\.|$)")
def _e_that_player_loses(m):
    return LoseLife(amount=int(m.group(1)), target=_THAT_PLAYER)


@_er(r"^that player gains (\d+) life(?:\.|$)")
def _e_that_player_gains(m):
    return GainLife(amount=int(m.group(1)), target=_THAT_PLAYER)


_NUM = {"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
        "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10}


@_er(r"^that player draws (a|an|one|two|three|four|five|six|seven|x|\d+) cards?(?:\.|$)")
def _e_that_player_draws(m):
    tok = m.group(1)
    n = _NUM.get(tok, tok if tok == "x" else int(tok) if tok.isdigit() else 1)
    return Draw(count=n, target=_THAT_PLAYER)


@_er(r"^that player discards (a|an|one|two|three|x|\d+) cards?(?:\.|$)")
def _e_that_player_discards(m):
    tok = m.group(1)
    n = _NUM.get(tok, tok if tok == "x" else int(tok) if tok.isdigit() else 1)
    return Discard(count=n, target=_THAT_PLAYER, chosen_by="discarder")


@_er(r"^its controller loses (\d+) life(?:\.|$)")
def _e_its_controller_loses(m):
    return LoseLife(amount=int(m.group(1)), target=_THAT_CONTROLLER)


@_er(r"^its controller draws (a|an|one|two|three|x|\d+) cards?(?:\.|$)")
def _e_its_controller_draws(m):
    tok = m.group(1)
    n = _NUM.get(tok, tok if tok == "x" else int(tok) if tok.isdigit() else 1)
    return Draw(count=n, target=_THAT_CONTROLLER)


@_er(r"^its controller sacrifices (?:it|them|that creature|that permanent)(?:\.|$)")
def _e_its_controller_sacs(m):
    return Sacrifice(query=Filter(base="that_thing", targeted=False))


# --- "it gains KW and KW until end of turn" (multi-keyword via EFFECT) -----
# Lives in EFFECT_RULES too so "it gains trample and haste until end of turn"
# can be consumed as a spell-effect fallback body.
@_er(r"^it gains ([a-z]+(?:,\s*[a-z]+)*(?:\s+and\s+[a-z]+)?) until end of turn(?:\.|$)")
def _e_it_gains_multi(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=_THAT_CREATURE)


@_er(r"^they gain ([a-z]+(?:,\s*[a-z]+)*(?:\s+and\s+[a-z]+)?) until end of turn(?:\.|$)")
def _e_they_gain_multi(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=_THEM)


# --- "it gets +N/+N and gains KW until end of turn" -- combined buff+grant -
@_er(r"^it gets ([+\-]\d+)/([+\-]\d+) and gains ([a-z][a-z, ]*?(?:\s+and\s+[a-z]+)?) until end of turn(?:\.|$)")
def _e_it_gets_and_gains(m):
    buff = Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=_THAT_CREATURE)
    grant = GrantAbility(ability_name=m.group(3).strip(), target=_THAT_CREATURE)
    return Sequence(items=(buff, grant))


# --- "they fight each other" / "they fight" --------------------------------
@_er(r"^they fight(?: each other)?(?:\.|$)")
def _e_they_fight(m):
    return Fight(a=_THAT_CREATURE, b=_THAT_CREATURE)


# --- "it fights target creature [...]" -------------------------------------
@_er(r"^it fights target ([^.]+?)(?:\.|$)")
def _e_it_fights(m):
    # Capture the filter text verbatim; leave typed parsing to downstream if
    # needed — we consume the sentence, which is the point here.
    return Fight(a=_THAT_CREATURE, b=Filter(base="creature", targeted=True))


# --- "it deals N damage to [target]" ---------------------------------------
@_er(r"^it deals (\d+) damage to ([^.]+?)(?:\.|$)")
def _e_it_deals(m):
    amt = int(m.group(1))
    return Damage(amount=amt, target=Filter(base=m.group(2).strip(),
                                            targeted=False))


# --- "it deals N damage to its controller" ---------------------------------
@_er(r"^it deals (\d+) damage to its controller(?:\.|$)")
def _e_it_deals_controller(m):
    return Damage(amount=int(m.group(1)), target=_THAT_CONTROLLER)


# --- "they're N/N [type] creatures" / "they're N/N [type]" -----------------
# These are token-setter riders but also appear in spell-effect contexts
# (e.g. Awaken), so we also let parse_effect consume them as a token-ish
# UnknownEffect to keep the whole sentence captured.
@_er(r"^they'?re (\d+)/(\d+) ([a-z ]+?) (?:creature tokens?|creatures?)(?:\.|$)")
def _e_theyre_pt_tok(m):
    p, t = int(m.group(1)), int(m.group(2))
    types = tuple(m.group(3).strip().split())
    return CreateToken(count=0, pt=(p, t), types=types)


# --- "then [followup]" — explicit chain marker, let downstream re-parse ----
@_er(r"^then ([^.]+?)(?:\.|$)")
def _e_then(m):
    # Wrap the tail as UnknownEffect with the chain marker preserved — the
    # base parser already joins "then ..." with its antecedent at split time,
    # but stray lone-`then` sentences do slip through (modal bodies, bullets).
    return UnknownEffect(raw_text="then: " + m.group(1).strip())
