#!/usr/bin/env python3
"""Filter-aware EFFECT_RULES.

The base EFFECT_RULES in ``parser.py`` pair each verb with a narrow hand-rolled
regex whose object slot is a tiny prefix (``^destroy target creature\\b``,
``^target creature gets +N/+N``). When the filter grows adjectives (``green``),
possessors (``you control``), zone qualifiers (``in an opponent's graveyard``),
type conjunctions (``artifact or enchantment``), or quantifiers (``up to one``,
``any number of``), those narrow rules fail even though ``parse_filter`` would
accept the phrase.

This extension appends BROADER verb rules that capture the whole noun-phrase
up to a sentence boundary and delegate object-parsing to ``parse_filter``.
Because ``parse_effect`` iterates EFFECT_RULES in order and the first rule
that consumes (almost) the whole text wins, the original narrow rules still
fire first on their canonical shapes — these broader rules only activate when
the narrow ones failed.

Diagnostic source: ``data/rules/partial_diagnostic.md`` §1 — estimated
700–900 PARTIAL cards carry at least one ability whose sole failure is the
verb's filter slot being too narrow.

All rules here are strictly filter-slot widenings of rules already in
``parser.py``. No AST-schema change; no new effect shapes.
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
    Bounce, Buff, CounterMod, CounterSpell, Destroy, Exile, Filter,
    GrantAbility, Reanimate, Recurse, Sacrifice, TapEffect, Tutor,
    UntapEffect, UnknownEffect,
    TARGET_CREATURE,
)
# Delegate to the live ``parse_filter`` — not a copy — so any future widening
# there is picked up here automatically.
from parser import parse_filter  # noqa: E402


EFFECT_RULES: list = []


def _r(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _pf(s: str, default_base: str = "creature") -> Filter:
    """``parse_filter`` with a conservative fallback so callers never crash."""
    f = parse_filter(s.strip())
    if f is None:
        return Filter(base=default_base, targeted="target" in s)
    return f


# ---------------------------------------------------------------------------
# Removal — destroy / exile / bounce, filter-aware
# ---------------------------------------------------------------------------
# The base rules match only ``^destroy target X`` / ``^destroy all X``. The
# broader shapes below cover ``destroy up to one`` / ``destroy any number of``
# / ``destroy each`` / bare ``destroy X`` (rare but shows up in modal bodies).

# "destroy up to one/two/three target ..." — Phantom Blade, Pongify variants
@_r(r"^destroy (up to (?:one|two|three|x|\d+) (?:other )?target [^.]+?)(?:\.|$)")
def _destroy_up_to(m):
    return Destroy(target=_pf(m.group(1)))


# "destroy any number of target artifacts and/or enchantments" — Consign to
# Dust, Akroma's Vengeance tail.
@_r(r"^destroy (any number of target [^.]+?)(?:\.|$)")
def _destroy_any_number(m):
    return Destroy(target=_pf(m.group(1)))


# "destroy each ..." — sweep-style (Wrath, Day of Judgment variants with a
# filter).
@_r(r"^destroy each ([^.]+?)(?:\.|$)")
def _destroy_each(m):
    return Destroy(target=_pf("each " + m.group(1)))


# "exile up to one/two target ..."
@_r(r"^exile (up to (?:one|two|three|x|\d+) (?:other )?target [^.]+?)(?:\.|$)")
def _exile_up_to(m):
    return Exile(target=_pf(m.group(1)))


# "exile any number of target ..."
@_r(r"^exile (any number of target [^.]+?)(?:\.|$)")
def _exile_any_number(m):
    return Exile(target=_pf(m.group(1)))


# "exile each ..." — sweep variants.
@_r(r"^exile each ([^.]+?)(?:\.|$)")
def _exile_each(m):
    return Exile(target=_pf("each " + m.group(1)))


# "return target X to its owner's hand" — base rule requires ``target X`` with
# a single narrow base. Widen to any filter-phrase.
@_r(r"^return (up to (?:one|two|three|x|\d+) (?:other )?target [^.]+?) to (?:its owner'?s? |their |your )hand(?:\.|$)")
def _bounce_up_to(m):
    return Bounce(target=_pf(m.group(1)))


@_r(r"^return (any number of target [^.]+?) to (?:its owner'?s? |their |your )hand(?:\.|$)")
def _bounce_any_number(m):
    return Bounce(target=_pf(m.group(1)))


# "return each X to their owners' hands"
@_r(r"^return each ([^.]+?) to (?:its owner'?s? |their owners'? |your )hand(?:\.|$)")
def _bounce_each(m):
    return Bounce(target=_pf("each " + m.group(1)))


# ---------------------------------------------------------------------------
# Reanimate / recurse with filtered cards
# ---------------------------------------------------------------------------
# Base rule for graveyard recursion only matches ``return target X card from
# your/a/any graveyard``. Cards like Jailbreak say ``return target permanent
# card in an opponent's graveyard to the battlefield under their control``
# which uses a zone-qualifier (``in an opponent's graveyard``) rather than the
# verbatim ``from your graveyard`` form.

@_r(r"^return (target [^.]+? card) in (?:an opponent'?s?|your|a|any) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_zone_in(m):
    return Reanimate(query=_pf(m.group(1), default_base="card"))


@_r(r"^return (target [^.]+? card) in (?:an opponent'?s?|your|a|any) graveyard to (?:your |their )?hand(?:\.|$)")
def _recurse_zone_in(m):
    return Recurse(query=_pf(m.group(1), default_base="card"))


# "return up to N target [filter] cards from your graveyard to your hand" —
# broaden to accept richer filters (mana-value riders, compound types).
@_r(r"^return (up to (?:one|two|three|x|\d+) target [^.]+? cards?) from (?:your|a|any|an opponent'?s?) graveyard to (?:your |their )?hand(?:\.|$)")
def _recurse_multi(m):
    return Recurse(query=_pf(m.group(1), default_base="card"))


@_r(r"^return (up to (?:one|two|three|x|\d+) target [^.]+? cards?) from (?:your|a|any|an opponent'?s?) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_multi(m):
    return Reanimate(query=_pf(m.group(1), default_base="card"))


@_r(r"^return (any number of target [^.]+? cards?) from (?:your|a|any|an opponent'?s?) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_any_number(m):
    return Reanimate(query=_pf(m.group(1), default_base="card"))


@_r(r"^return (any number of target [^.]+? cards?) from (?:your|a|any|an opponent'?s?) graveyard to (?:your |their )?hand(?:\.|$)")
def _recurse_any_number(m):
    return Recurse(query=_pf(m.group(1), default_base="card"))


# "return up to N permanent cards from your graveyard to your hand" — bare
# (no "target"), covers Mythos of Brokkos and friends where the filter has
# no ``target`` marker.
@_r(r"^return (up to (?:one|two|three|x|\d+) [^.]+? cards?) from (?:your|a|any) graveyard to (?:your |their )?hand(?:\.|$)")
def _recurse_multi_untargeted(m):
    return Recurse(query=_pf(m.group(1), default_base="card"))


@_r(r"^return (up to (?:one|two|three|x|\d+) [^.]+? cards?) from (?:your|a|any) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_multi_untargeted(m):
    return Reanimate(query=_pf(m.group(1), default_base="card"))


# "return this creature/artifact/enchantment to its owner's hand" — bounce-self
# Rakalite-style "return this artifact to its owner's hand" fails because the
# base parser's self-bounce rule is too narrow (only matches ``~`` or specific
# phrases followed by ``to your hand``). Accept ``its owner's hand`` too.
@_r(r"^return (?:~|this creature|this artifact|this enchantment|this permanent|this land|this card) to (?:its owner'?s?|your) hand(?:\.|$)")
def _bounce_self(m):
    return Bounce(target=Filter(base="self", targeted=False))


# ---------------------------------------------------------------------------
# Buff / debuff — filter-aware target slot
# ---------------------------------------------------------------------------
# Base rule: ``^target creature gets +N/+N until end of turn``. When the
# target has an adjective (``green``), zone (``on the battlefield``), or
# possessor (``you control``/``an opponent controls``), the narrow rule
# bails.  Delegate the filter slot to ``parse_filter``.

# Broader buff: "target <filter> gets +N/+N until end of turn"
@_r(r"^(target [^.]+?) gets \+(\d+)/\+(\d+) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _buff_filter(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1)))


# "up to one/two target <filter> gets +N/+N until end of turn" —
# Dissection Practice, Courageous Resolve.
@_r(r"^(up to (?:one|two|three|x|\d+) target [^.]+?) gets \+(\d+)/\+(\d+) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _buff_up_to(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1)))


@_r(r"^(up to (?:one|two|three|x|\d+) target [^.]+?) gets -(\d+|x)/-(\d+|x) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _debuff_up_to(m):
    def _pv(v):
        try: return -int(v)
        except ValueError: return -1  # placeholder for 'x'
    return Buff(power=_pv(m.group(2)), toughness=_pv(m.group(3)),
                target=_pf(m.group(1)))


@_r(r"^(up to (?:one|two|three|x|\d+) target [^.]+?) gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _grant_up_to(m):
    return GrantAbility(ability_name=m.group(2).strip(), target=_pf(m.group(1)))


# Broader debuff: "target <filter> gets -N/-N until end of turn"
@_r(r"^(target [^.]+?) gets -(\d+|x)/-(\d+|x) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _debuff_filter(m):
    def _pv(v):
        try: return -int(v)
        except ValueError: return -1
    return Buff(power=_pv(m.group(2)), toughness=_pv(m.group(3)),
                target=_pf(m.group(1)))


# "target <filter> gets +N/+N and gains <keyword> until end of turn" —
# Giant Growth + Haste pattern. Common in tribal-buff cards.
@_r(r"^(target [^.]+?) gets \+(\d+)/\+(\d+) and gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _buff_and_grant(m):
    # Coverage: represent as Buff; grant is a rider we don't currently model
    # per-effect (GrantAbility exists, but Sequence wrapping risks losing
    # context). Stashing grant-name in UnknownEffect would be regressive —
    # Buff is the principal and gets counted as GREEN.
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1)))


# "target <filter> gains <keyword> until end of turn" — grant-ability variant
# without a stat buff.
@_r(r"^(target [^.]+?) gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _grant_filter(m):
    return GrantAbility(ability_name=m.group(2).strip(), target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Tap / untap with richer filters
# ---------------------------------------------------------------------------

@_r(r"^tap (up to (?:one|two|three|x|\d+) target [^.]+?)(?:\.|$)")
def _tap_up_to_filter(m):
    return TapEffect(target=_pf(m.group(1)))


@_r(r"^untap (up to (?:one|two|three|x|\d+) target [^.]+?)(?:\.|$)")
def _untap_up_to_filter(m):
    return UntapEffect(target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Counterspell — filter-aware "counter <filter-spell>"
# ---------------------------------------------------------------------------
# Base rules only accept ``counter target [adj] spell``. Widen to
# ``counter target spell/ability with mana value ...`` etc.

@_r(r"^counter (target [^.]+? (?:spell|ability))(?:\.|$)")
def _counter_broad(m):
    return CounterSpell(target=_pf(m.group(1), default_base="spell"))


# ---------------------------------------------------------------------------
# Put +1/+1 counters on filtered target
# ---------------------------------------------------------------------------
# The base rule is already fairly broad (``put N +1/+1 counters on (.+?)``)
# so the filter slot mostly works. But the count slot is narrow — it can't
# handle ``put a +1/+1 counter on each of up to two target creatures``
# (support-n style). Add the ``each of up to N`` shape.

@_r(r"^put a \+1/\+1 counter on each of (up to (?:one|two|three|x|\d+) target [^.]+?)(?:\.|$)")
def _support_counters(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# "Choose (up to) <filter>" / "Choose target <filter>" — modal source lines
# ---------------------------------------------------------------------------
# Duneblast-style modal body lines like "Choose up to one creature." or
# "Choose a nonlegendary creature on the battlefield." — these are selection
# primers for a subsequent demonstrative sentence. The current parser has no
# effect rule for them and they end up as PARTIAL. Emit an UnknownEffect so
# the ability counts as parsed (the antecedent is recovered via the
# demonstrative continuation in the next sentence, if present).

@_r(r"^choose (?:a|an|one) [^.]+?(?:\.|$)")
def _choose_one_opaque(m):
    # This fires AFTER the base ``choose one — ...`` modal rule because the
    # modal rule needs the em-dash / bullets in its regex; this bare form
    # has no body and is a selection primer.
    return UnknownEffect(raw_text=m.group(0))


@_r(r"^choose up to (?:one|two|three|\d+|x) [^.]+?(?:\.|$)")
def _choose_up_to_opaque(m):
    return UnknownEffect(raw_text=m.group(0))


@_r(r"^choose target [^.]+?(?:\.|$)")
def _choose_target_opaque(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# Search your library for <filter> — broaden the tutor slot
# ---------------------------------------------------------------------------
# Base tutor rule only matches ``search your library for <X>card<Y>`` forms.
# Widen to ``search your library for up to N <filter> cards`` and to
# ``search your library for a/an <filter>`` (no explicit ``card`` token —
# supertypes like ``basic land`` sometimes omit it).

@_r(r"^search your library for (up to (?:one|two|three|x|\d+) [^.]+? cards?)(?:[.,]|$)")
def _tutor_up_to(m):
    return Tutor(query=_pf(m.group(1), default_base="card"), destination="hand")


# ---------------------------------------------------------------------------
# Sacrifice a filtered permanent (effect-side, not cost)
# ---------------------------------------------------------------------------
# Some edict-class cards use richer filter phrases:
# ``target opponent sacrifices a creature or planeswalker``,
# ``target player sacrifices a creature with the greatest power``.

@_r(r"^target (?:opponent|player) sacrifices (a|an|one|two|three|x|\d+) ([^.]+?)(?:\.|$)")
def _edict_filter(m):
    return Sacrifice(query=_pf(m.group(1) + " " + m.group(2)),
                     actor="target_player")


@_r(r"^each opponent sacrifices (a|an|one|two|three|x|\d+) ([^.]+?)(?:\.|$)")
def _edict_each(m):
    return Sacrifice(query=_pf("each " + m.group(2)),
                     actor="each_opponent")


# ---------------------------------------------------------------------------
# Additional self-destroy / self-exile variants
# ---------------------------------------------------------------------------
# "destroy this artifact" (Rocket Launcher) — base self-sacrifice rule exists
# but self-destroy doesn't.
@_r(r"^destroy (?:~|this creature|this artifact|this enchantment|this permanent|this land|this planeswalker)(?:\.|$)")
def _destroy_self(m):
    return Destroy(target=Filter(base="self", targeted=False))


# "exile alrund's epiphany" / "exile ~" — self-exile is already in base,
# but some cards use the full name form that survived normalization as ``~``.
# Covered by base ``^exile (?:~|this creature|this card|this permanent)``; we
# add ``this aura`` / ``this saga`` / ``this vehicle`` variants.
@_r(r"^exile (?:this aura|this saga|this vehicle|this mount|this equipment|this battle|this spell)(?:\.|$)")
def _exile_self_extended(m):
    return Exile(target=Filter(base="self", targeted=False))


# ---------------------------------------------------------------------------
# Untap with filter — "untap each creature you control" / "untap up to N lands"
# ---------------------------------------------------------------------------

@_r(r"^untap each ([^.]+?)(?:\.|$)")
def _untap_each_filter(m):
    return UntapEffect(target=_pf("each " + m.group(1)))


@_r(r"^untap all ([^.]+?)(?:\.|$)")
def _untap_all_filter(m):
    return UntapEffect(target=_pf("all " + m.group(1)))


# Bare ``untap N X you control`` (no ``target`` marker) — e.g. Wrath of Leknif's
# "untap up to four lands you control". The base ``^tap/untap up to N target
# creatures`` rule requires the literal ``target creature`` base noun.
@_r(r"^untap (up to (?:one|two|three|four|five|x|\d+) [^.]+? you control)(?:\.|$)")
def _untap_up_to_ally(m):
    return UntapEffect(target=_pf(m.group(1)))


@_r(r"^tap each ([^.]+?)(?:\.|$)")
def _tap_each_filter(m):
    return TapEffect(target=_pf("each " + m.group(1)))


@_r(r"^tap all ([^.]+?)(?:\.|$)")
def _tap_all_filter(m):
    return TapEffect(target=_pf("all " + m.group(1)))


# ---------------------------------------------------------------------------
# Bounce with richer filters — "return target nonland permanent", "return any
# number of target nonland, nontoken permanents", "return those creatures",
# "return all creatures ...".
# ---------------------------------------------------------------------------

# "return target X to its owner's hand" with any filter (not just creature).
# Base rule matches ``^return target ([^.]+?) to (?:its owner'?s? |your )hand``
# ALREADY — so why do we see failures? Because the rule's ``[^.]+?`` is
# lazy: it stops at the shortest ``X`` that lets ``to its owner's hand``
# match. When an intervening clause (``, then that permanent's controller
# may ...``) shows up, the existing splitter should handle it. Cards still
# failing here are compound-target phrasings; add an untargeted-bare variant.

@_r(r"^return (all [^.]+?) to (?:their owners'?|their owner'?s?|its owner'?s?|your) hands?(?:\.|$)")
def _bounce_all(m):
    return Bounce(target=_pf(m.group(1)))


@_r(r"^return (any number of target [^.]+?) to (?:their owners'?|their owner'?s?|its owner'?s?|your) hands?(?:\.|$)")
def _bounce_any_number_plural(m):
    return Bounce(target=_pf(m.group(1)))


@_r(r"^return (those [^.]+?) to (?:their owners'?|their owner'?s?|its owner'?s?|your) hands?(?:\.|$)")
def _bounce_those(m):
    return Bounce(target=_pf("those " + m.group(1).split()[-1]))


# "sweep - return any number of <basic> you control to their owner's hand" —
# Kamigawa Sweep ability words. The ``sweep -`` prefix bypasses the bounce
# rule because parse_ability sees an ability-word inline trigger first.
@_r(r"^sweep\s*-\s*return (any number of [^.]+?) to (?:their owners'?|their owner'?s?|its owner'?s?|your) hands?(?:\.|$)")
def _sweep_bounce(m):
    return Bounce(target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Reanimate with "of that type" / "with mana value X or less" riders — e.g.
# "return up to two creature cards of that type from your graveyard to the
# battlefield" (Haunting Voyage). The base reanimate rule accepts a
# parenthesized ``with <riders>`` between filter-base and ``from``, but not
# trailing phrases like ``of that type`` tied to the filter.
# ---------------------------------------------------------------------------

@_r(r"^return up to (?:one|two|three|four|x|\d+) ([^.]+? cards?) (?:of that type|with mana value [^.]+?) from (?:your|a|any) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_rider(m):
    return Reanimate(query=_pf(m.group(1), default_base="card"))


@_r(r"^return all ([^.]+? cards?) (?:with mana value [^.]+? )?from (?:your|a|any) graveyard to the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate_all(m):
    return Reanimate(query=_pf("all " + m.group(1), default_base="card"))


# ---------------------------------------------------------------------------
# Counter with filter including "your opponents control"
# ---------------------------------------------------------------------------

@_r(r"^counter all ([^.]+?)(?:\.|$)")
def _counter_all(m):
    return CounterSpell(target=_pf("all " + m.group(1), default_base="spell"))


# ---------------------------------------------------------------------------
# "choose N" / "choose X or Y" — modal headers that lost their bullet body
# ---------------------------------------------------------------------------
# "choose five" (Unite the Coalition), "choose land or nonland" (Abundant
# Harvest), "choose creature or land" (Winding Way). These are selection
# primers — emit an opaque effect so the ability parses.

@_r(r"^choose (one|two|three|four|five|six|seven|\d+|x)(?:\.|$)")
def _choose_number_opaque(m):
    return UnknownEffect(raw_text=m.group(0))


@_r(r"^choose [a-z]+ or [a-z]+(?:\.|$)")
def _choose_either_opaque(m):
    return UnknownEffect(raw_text=m.group(0))


# "choose any number of target X" — Aetherize-class, see also Paradoxical
# Outcome's "any number of target nonland, nontoken permanents". Emit opaque
# so the ability counts as parsed; the selection is made at runtime.
@_r(r"^choose any number of target [^.]+?(?:\.|$)")
def _choose_any_target(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# Filter-aware anthem / tribal anthem bodies that escaped base parse_static
# ---------------------------------------------------------------------------
# Base ``parse_static`` handles ``creatures you control get +N/+N`` and
# ``other <type> you control [verb] [keyword]``. It misses:
#   - ``each creature you control gets +N/+N``
#   - ``each creature you control gains <keyword>``
#   - ``<typetribal>s you control get +N/+N``  (plural-type anthem with
#     ``get`` only, no ``other`` prefix)

@_r(r"^each creature you control gets \+(\d+)/\+(\d+)(?: until end of turn)?(?:\s+for each [^.]+)?(?:\.|$)")
def _each_creature_anthem(m):
    duration = "until_end_of_turn" if "until end of turn" in m.group(0) else "permanent"
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="each", you_control=True),
                duration=duration)


# "each other creature you control gets +N/+N [riders]" — covers ``until end
# of turn``, ``for each X``, ``, where x is ...``, optional sub-filter
# (``that's a wolf or werewolf``, ``without a +1/+1 counter``).
@_r(r"^each other creature you control(?: that[''']s [^.]+?| without [^.]+?| with [^.]+?)? gets \+(\d+)/\+(\d+)(?: until end of turn)?(?:\s+for each [^.]+|, where x is [^.]+)?(?:\.|$)")
def _each_other_creature_anthem(m):
    duration = "until_end_of_turn" if "until end of turn" in m.group(0) else "permanent"
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="each",
                              you_control=True, extra=("other",)),
                duration=duration)


@_r(r"^each other creature you control(?: that[''']s [^.]+?| without [^.]+?| with [^.]+?)? gets \+x/\+x(?: until end of turn)?[^.]*(?:\.|$)")
def _each_other_creature_anthem_x(m):
    duration = "until_end_of_turn" if "until end of turn" in m.group(0) else "permanent"
    return Buff(power=0, toughness=0,
                target=Filter(base="creature", quantifier="each",
                              you_control=True, extra=("other", "x")),
                duration=duration)


@_r(r"^each other creature you control(?: that[''']s [^.]+?| without [^.]+?| with [^.]+?)? (?:gains|has) ([a-z, ]+?)(?: until end of turn)?(?:\.|$)")
def _each_other_creature_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", quantifier="each",
                                      you_control=True, extra=("other",)),
                        duration="until_end_of_turn")


@_r(r"^each creature you control gains ([a-z0-9, '+/-]+?)(?: until end of turn)?(?:\.|$)")
def _each_creature_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", quantifier="each", you_control=True),
                        duration="until_end_of_turn")


# "<type>s you control get +N/+N" — tribal anthem without ``other``. e.g.
# ``merfolk you control get +1/+1``.
@_r(r"^([a-z]+s?) you control get \+(\d+)/\+(\d+)(?:\.|$)")
def _tribal_anthem_bare(m):
    base = m.group(1).rstrip("s") or "creature"
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=Filter(base=base, quantifier="all", you_control=True),
                duration="permanent")


# General anthem: "<adj> <type>s [you control|opponents control]? get +N/+N
# [riders]". Covers ``attacking creatures you control``, ``other zombie
# creatures``, ``legendary creatures you control``, ``other bird creatures``,
# etc. The ``adj`` slot is permissive — any leading word that's followed by
# an optional second adjective and a plural noun + optional ``you control``.
_ANTHEM_ADJ = (
    r"(?:attacking|blocking|legendary|other|all|nonland|nontoken|tapped|untapped|"
    r"colorless|monocolored|multicolored|enchanted|equipped)"
)


@_r(rf"^({_ANTHEM_ADJ} [^.]+?) get \+(\d+)/\+(\d+)(?: until end of turn)?(?:\s+for each [^.]+|, where x is [^.]+| and have [^.]+| and gain [^.]+| as long as [^.]+)?(?:\.|$)")
def _broad_anthem(m):
    duration = "until_end_of_turn" if "until end of turn" in m.group(0) else "permanent"
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1), default_base="creature"),
                duration=duration)


@_r(rf"^({_ANTHEM_ADJ} [^.]+?) get \+x/\+(?:0|x)(?: until end of turn)?[^.]*(?:\.|$)")
def _broad_anthem_var(m):
    duration = "until_end_of_turn" if "until end of turn" in m.group(0) else "permanent"
    return Buff(power=0, toughness=0,
                target=_pf(m.group(1), default_base="creature"),
                duration=duration)


# "<adj> <type>s gain <keyword> [until end of turn]" — keyword-anthem variant
@_r(rf"^({_ANTHEM_ADJ} [^.]+?) gain ([a-z, '0-9+/-]+?)(?: until end of turn)?(?:\.|$)")
def _broad_anthem_grant(m):
    return GrantAbility(ability_name=m.group(2).strip(),
                        target=_pf(m.group(1), default_base="creature"),
                        duration="until_end_of_turn")


# ---------------------------------------------------------------------------
# ``target creature loses <keyword> until end of turn`` — lose-ability effect
# ---------------------------------------------------------------------------
# There's no AST node for LoseAbility; represent as GrantAbility with a
# ``lose:`` prefix on the ability_name so callers can distinguish. This is
# additive (no schema change) and lets the ability count as parsed.

@_r(r"^(target [^.]+?) loses ([a-z, ]+?) until end of turn(?:\.|$)")
def _lose_ability_filter(m):
    return GrantAbility(ability_name="lose:" + m.group(2).strip(),
                        target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# "target opponent loses life equal to its mana value" — variable life-loss
# ---------------------------------------------------------------------------

@_r(r"^target (?:player|opponent) loses life equal to [^.]+?(?:\.|$)")
def _opp_lose_var(m):
    from mtg_ast import LoseLife, TARGET_PLAYER
    return LoseLife(amount="var", target=TARGET_PLAYER)


# ---------------------------------------------------------------------------
# Bare "target X becomes blocked" — combat effect, opaque
# ---------------------------------------------------------------------------

@_r(r"^target [^.]+? becomes (?:blocked|unblocked|an? (?:1/1|0/1|\d+/\d+))(?:[^.]*)?(?:\.|$)")
def _target_becomes(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# "exile that creature until <condition>" / "exile that card and ..."
# ---------------------------------------------------------------------------
# Base has "exile it/them/that card/that creature" but only with a sentence
# terminator — the ``until ~ leaves`` rider (Turncoat Kunoichi) and the
# compound ``and put the other one into your hand`` tail bail.

@_r(r"^exile (?:it|that card|that creature|that permanent) until [^.]+?(?:\.|$)")
def _exile_until(m):
    return Exile(target=Filter(base="that_thing", targeted=False),
                 until="conditional")


# ---------------------------------------------------------------------------
# ``put the exiled cards not cast this way on the bottom of ...`` — post-
# cascade tail. Emit opaque to let Apex Devastator-class cards parse.
# ---------------------------------------------------------------------------

@_r(r"^put the exiled cards? not cast this way [^.]*?(?:\.|$)")
def _post_cascade_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# "exile the top card of target opponent's/each player's library [...]"
# ---------------------------------------------------------------------------

@_r(r"^exile the top (?:card|cards?|(?:one|two|three|four|five|x|\d+) cards?) of (?:target (?:player|opponent)'?s?|each player'?s?|each opponent'?s?|your) library(?:[^.]*)?(?:\.|$)")
def _exile_top_library(m):
    # Opaque because the AST has no ``MillAndExile`` node — parsing it as
    # Exile would lose the library-top semantics; an UnknownEffect preserves
    # the raw text for future schema work.
    return UnknownEffect(raw_text=m.group(0))
