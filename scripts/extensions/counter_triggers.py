#!/usr/bin/env python3
"""Counter-event trigger / static / effect patterns.

Family: COUNTER-EVENT TRIGGERS — abilities that fire when counters are put,
removed, or doubled on permanents/players, plus statics that scale with counter
counts, and one-shot effects that manipulate counters (doubling, conversion,
distribution, replacement).

Three tables are exported:

- ``TRIGGER_PATTERNS``: ``(compiled_regex, event_name, scope)`` — merged into
  parser._TRIGGER_PATTERNS via parser.EXT_TRIGGER_PATTERNS (tried BEFORE the
  core list so more-specific shapes win).

- ``STATIC_PATTERNS``: ``(compiled_regex, builder_fn)`` — merged into
  parser.EXT_STATIC_PATTERNS. Builder is called as ``builder(match, raw)``.

- ``EFFECT_RULES``: ``(compiled_regex, builder_fn)`` — appended to
  parser.EFFECT_RULES. Builder is called as ``builder(match)`` and must return
  an Effect AST node.

Patterns are ORDERED most-specific → least-specific (first-match-wins).

Covered phrasings (non-exhaustive):
  Triggers
    * whenever one or more {kind} counters are put on ~
    * whenever a {kind} counter is put on {filter}
    * whenever a counter is put on a permanent you control
    * whenever you put one or more {kind} counters on {filter}
    * whenever you put a counter on {filter}
    * whenever a {kind} counter is removed from ~ / a {filter}
    * whenever ~ has {N or more} {kind} counters on it (Threshold-style)
    * whenever another creature dies, put a +1/+1 counter on ~
      (handled via core dies-trigger + our effect rules)
    * whenever you {scry|surveil|gain life}, put a {kind} counter on ...
      (handled via core + our put-counter effect rule)

  Statics (P/T scaling / conditional / anthem-by-counter)
    * {subject} gets +1/+1 for each {kind} counter on {scope}
    * each creature you control with a {kind} counter on it has {keyword}
    * as long as ~ has {N or more} {kind} counters on it, {effect}

  Replacement statics (put-counters replacement)
    * if one or more {kind} counters would be put on {filter},
      that many plus N {kind} counters are put on {that creature} instead
    * if one or more {kind} counters would be put on {filter},
      twice that many {kind} counters are put on {that creature} instead
    * if {an effect|you|an opponent} would put one or more counters on a
      permanent or player, put twice/half that many ... instead
    * if ~ would have {kind} counters put on it, that many plus N ...

  Effects (one-shots)
    * double the number of {kind} counters on {target}
    * put a counter on ... (generic non-+1/+1 kinds: stun, loyalty, time,
      brick, depletion, charge, flying, etc.)
    * put N {kind} counters on {target} (N words or digits, multi-kind)
    * remove a/N {kind} counters from {target}
    * move a/N {kind} counters from X to Y
    * distribute {N|X} +1/+1 counters among {filter}
    * proliferate (non-keyword, used mid-sentence or as bullet)

The counter_kind grammar accepts the forms the rules actually use:
  - ``+1/+1`` / ``-1/-1`` / ``-0/-1`` (signed P/T mods)
  - named words: stun, loyalty, time, brick, depletion, charge, flying,
    poison, lore, verse, oil, blight, study, supply, shield, rust, shred,
    quest, bounty, pressure, scream, finality, wish, etc.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Allow the module to be imported without package install.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Buff, Condition, CounterMod, Effect, Filter, GrantAbility, Modification,
    Replacement, Sequence, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Shared fragments
# ---------------------------------------------------------------------------

_SELF = (
    r"(?:~|this creature|this permanent|this vehicle|this land|"
    r"this artifact|this enchantment|this card|equipped creature|"
    r"enchanted creature)"
)

# A counter kind. Two shapes: "+1/+1" (signed P/T) or a named word.
# Kept non-capturing here; individual rules add captures as needed.
_KIND = (
    r"(?:[+-]\d+/[+-]\d+"
    r"|stun|loyalty|time|charge|brick|depletion|flying|poison|lore|verse"
    r"|oil|blight|study|supply|shield|rust|shred|quest|bounty|pressure"
    r"|scream|finality|wish|age|arrow|awakening|blood|bribery|career"
    r"|corpse|credit|crystal|cube|currency|death|delay|dream|echo|egg"
    r"|energy|experience|eyeball|fade|feather|fetch|filibuster|fungus"
    r"|fuse|game|gem|glyph|gold|growth|hatchling|healing|hit|hoofprint"
    r"|hone|hour|hourglass|hunger|ice|incarnation|incubation|infection"
    r"|ingredient|intervention|isolation|javelin|ki|kick|knowledge"
    r"|landmark|level|lithograph|magnet|manifestation|mannequin|mask"
    r"|matrix|memory|mine|mining|mire|music|muster|nest|night|omen"
    r"|ore|page|pain|paralyzation|petal|petrification|phylactery|pin"
    r"|plague|plot|plus|point|polyp|prey|pupa|quest|rejection|reprieve"
    r"|rev|ribbon|ritual|rope|satori|scroll|shell|silver|sleep|sleight"
    r"|slime|slumber|soot|soul|spite|spore|stash|storage|strife|task"
    r"|tattoo|task|tide|time|tower|trap|treasure|unity|vitality|void"
    r"|vortex|vow|wage|winch|wind|wound"
    r")"
)

# Counter-kind with optional preceding article. Used as capture group.
_KIND_CAP = rf"([+-]\d+/[+-]\d+|[a-z][a-z0-9]*)"

# Word-or-digit number. Keep in sync with parser._NUM_WORDS.
_N = r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)"

# Actor/filter (lazy, stops at clause boundary).
_FILT = r"([^,.]+?)"


def _num(token: str):
    """Translate a word/digit to int/'x'/str."""
    words = {"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4,
             "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
             "ten": 10}
    t = token.strip().lower()
    if t.isdigit():
        return int(t)
    if t == "x":
        return "x"
    return words.get(t, t)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # ------------------------------------------------------------------
    # Aggregate counter-placed triggers (must come before singletons)
    # ------------------------------------------------------------------

    # "Whenever one or more +1/+1 counters are put on ~"
    (re.compile(rf"^whenever one or more {_KIND_CAP} counters are put on {_SELF}", re.I),
     "counters_put_on_self", "self"),
    # "Whenever one or more counters are put on ~"
    (re.compile(rf"^whenever one or more counters are put on {_SELF}", re.I),
     "counters_put_on_self_any", "self"),
    # "Whenever one or more +1/+1 counters are put on <actor>"
    (re.compile(rf"^whenever one or more {_KIND_CAP} counters are put on {_FILT}", re.I),
     "counters_put_on_actor", "actor"),
    # "Whenever one or more counters are put on <actor>"
    (re.compile(rf"^whenever one or more counters are put on {_FILT}", re.I),
     "counters_put_on_actor_any", "actor"),

    # "Whenever you put one or more +1/+1 counters on <actor>"
    (re.compile(rf"^whenever you put one or more {_KIND_CAP} counters on {_FILT}", re.I),
     "you_put_counters_on", "actor"),
    # "Whenever you put one or more counters on <actor>"
    (re.compile(rf"^whenever you put one or more counters on {_FILT}", re.I),
     "you_put_counters_on_any", "actor"),

    # "Whenever you put a/the +1/+1 counter on <actor>"
    (re.compile(rf"^whenever you put a {_KIND_CAP} counter on {_FILT}", re.I),
     "you_put_counter_on", "actor"),
    # "Whenever you put a counter on <actor>"
    (re.compile(rf"^whenever you put a counter on {_FILT}", re.I),
     "you_put_counter_on_any", "actor"),

    # ------------------------------------------------------------------
    # Single-counter-placed triggers
    # ------------------------------------------------------------------

    # "Whenever a +1/+1 counter is put on ~"
    (re.compile(rf"^whenever a(?:n)? {_KIND_CAP} counter is put on {_SELF}", re.I),
     "counter_put_on_self", "self"),
    # "Whenever a counter is put on ~"
    (re.compile(rf"^whenever a(?:n)? counter is put on {_SELF}", re.I),
     "counter_put_on_self_any", "self"),
    # "Whenever a +1/+1 counter is put on <actor>"
    (re.compile(rf"^whenever a(?:n)? {_KIND_CAP} counter is put on {_FILT}", re.I),
     "counter_put_on_actor", "actor"),
    # "Whenever a counter is put on <actor>"
    (re.compile(rf"^whenever a(?:n)? counter is put on {_FILT}", re.I),
     "counter_put_on_actor_any", "actor"),

    # ------------------------------------------------------------------
    # Counter-removed triggers
    # ------------------------------------------------------------------

    # "Whenever a time counter is removed from ~" / "is removed from a card"
    (re.compile(rf"^whenever a(?:n)? {_KIND_CAP} counter is removed from {_SELF}", re.I),
     "counter_removed_from_self", "self"),
    (re.compile(rf"^whenever a(?:n)? counter is removed from {_SELF}", re.I),
     "counter_removed_from_self_any", "self"),
    (re.compile(rf"^whenever a(?:n)? {_KIND_CAP} counter is removed from {_FILT}", re.I),
     "counter_removed_from_actor", "actor"),
    (re.compile(rf"^whenever a(?:n)? counter is removed from {_FILT}", re.I),
     "counter_removed_from_actor_any", "actor"),
    # "Whenever one or more +1/+1 counters are removed from ~"
    (re.compile(rf"^whenever one or more {_KIND_CAP} counters are removed from {_SELF}", re.I),
     "counters_removed_from_self", "self"),
    (re.compile(rf"^whenever one or more {_KIND_CAP} counters are removed from {_FILT}", re.I),
     "counters_removed_from_actor", "actor"),

    # ------------------------------------------------------------------
    # Threshold triggers (counter-count state changes)
    # ------------------------------------------------------------------

    # "Whenever ~ has N or more +1/+1 counters on it, ..."
    (re.compile(rf"^whenever {_SELF} has (\d+) or more {_KIND_CAP} counters? on it", re.I),
     "self_has_n_counters", "self"),
    # "When ~ has N or more +1/+1 counters on it"
    (re.compile(rf"^when {_SELF} has (\d+) or more {_KIND_CAP} counters? on it", re.I),
     "self_has_n_counters", "self"),

    # ------------------------------------------------------------------
    # Counter-action payoff triggers: "whenever you proliferate"
    # ------------------------------------------------------------------

    (re.compile(r"^whenever you proliferate", re.I),
     "you_proliferate", "self"),
]


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# Counter-scaled statics (P/T by counter count, anthems-by-counter,
# put-counter replacement effects).
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _stat(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# "~ gets +1/+1 for each {kind} counter on {scope}"
@_stat(rf"^{_SELF} gets \+(\d+)/\+(\d+) for each {_KIND_CAP} counter on "
       r"([^.]+?)(?:\.|$)")
def _self_pt_per_counter(m, raw):
    return Static(
        modification=Modification(
            kind="pt_scale_by_counters",
            args=(int(m.group(1)), int(m.group(2)),
                  m.group(3).lower(), m.group(4).strip().lower()),
        ),
        raw=raw,
    )


# "Equipped/enchanted creature gets +1/+1 for each {kind} counter on {scope}"
@_stat(r"^(equipped creature|enchanted creature|enchanted permanent) "
       rf"gets \+(\d+)/\+(\d+) for each {_KIND_CAP} counter on "
       r"([^.]+?)(?:\.|$)")
def _attached_pt_per_counter(m, raw):
    return Static(
        modification=Modification(
            kind="attached_pt_scale_by_counters",
            args=(m.group(1).lower(),
                  int(m.group(2)), int(m.group(3)),
                  m.group(4).lower(), m.group(5).strip().lower()),
        ),
        raw=raw,
    )


# "Each creature you control with a {kind} counter on it has {keyword(s)}"
@_stat(rf"^each creature you control with a {_KIND_CAP} counter on it "
       r"(?:has|have|gets) ([^.]+?)(?:\.|$)")
def _tribal_anthem_by_counter(m, raw):
    return Static(
        modification=Modification(
            kind="anthem_by_counter_presence",
            args=(m.group(1).lower(), m.group(2).strip().lower()),
        ),
        raw=raw,
    )


# "Creatures you control with +1/+1 counters on them have {keyword(s)}"
@_stat(rf"^creatures you control with {_KIND_CAP} counters on them "
       r"(?:have|has|get) ([^.]+?)(?:\.|$)")
def _tribal_anthem_by_counters(m, raw):
    return Static(
        modification=Modification(
            kind="anthem_by_counter_presence_plural",
            args=(m.group(1).lower(), m.group(2).strip().lower()),
        ),
        raw=raw,
    )


# "As long as ~ has N or more {kind} counters on it, {effect}"
@_stat(rf"^as long as {_SELF} has (\d+) or more {_KIND_CAP} counters? on it,?\s*"
       r"([^.]+?)(?:\.|$)")
def _as_long_as_has_counters(m, raw):
    return Static(
        condition=Condition(
            kind="self_has_n_counters",
            args=(int(m.group(1)), m.group(2).lower()),
        ),
        modification=Modification(
            kind="conditional_effect",
            args=(m.group(3).strip().lower(),),
        ),
        raw=raw,
    )


# "As long as ~ has a {kind} counter on it, {effect}"
@_stat(rf"^as long as {_SELF} has a(?:n)? {_KIND_CAP} counter on it,?\s*"
       r"([^.]+?)(?:\.|$)")
def _as_long_as_has_any_counter(m, raw):
    return Static(
        condition=Condition(
            kind="self_has_counter",
            args=(m.group(1).lower(),),
        ),
        modification=Modification(
            kind="conditional_effect",
            args=(m.group(2).strip().lower(),),
        ),
        raw=raw,
    )


# ------------------------------------------------------------------
# Replacement: "If one or more {kind} counters would be put on {filter},
#              that many plus N {kind} counters are put on {tgt} instead"
# (Hardened Scales / Winding Constrictor family)
# ------------------------------------------------------------------
@_stat(rf"^if one or more {_KIND_CAP} counters would be put on {_FILT},"
       rf"\s*that many plus (\d+) (?:{_KIND_CAP} )?counters are put on "
       r"([^.]+?) instead(?:\.|$)")
def _replace_plus_n(m, raw):
    return Static(
        modification=Modification(
            kind="counter_replacement_plus_n",
            args=(m.group(1).lower(), m.group(2).strip().lower(),
                  int(m.group(3)), (m.group(4) or "").lower(),
                  m.group(5).strip().lower()),
        ),
        raw=raw,
    )


# "If one or more {kind} counters would be put on {filter},
#  twice that many {kind} counters are put on {tgt} instead"
@_stat(rf"^if one or more {_KIND_CAP} counters would be put on {_FILT},"
       rf"\s*twice that many (?:{_KIND_CAP} )?counters are put on "
       r"([^.]+?) instead(?:\.|$)")
def _replace_double(m, raw):
    return Static(
        modification=Modification(
            kind="counter_replacement_double",
            args=(m.group(1).lower(), m.group(2).strip().lower(),
                  (m.group(3) or "").lower(), m.group(4).strip().lower()),
        ),
        raw=raw,
    )


# "If one or more counters would be put on {filter},
#  that many plus N of each of those kinds of counters are put on
#  {tgt} instead" (Winding Constrictor full form)
@_stat(rf"^if one or more counters would be put on {_FILT},"
       r"\s*that many plus (\d+) of each of those kinds? of counters "
       r"are put on ([^.]+?) instead(?:\.|$)")
def _replace_plus_n_any(m, raw):
    return Static(
        modification=Modification(
            kind="counter_replacement_plus_n_any_kind",
            args=(m.group(1).strip().lower(), int(m.group(2)),
                  m.group(3).strip().lower()),
        ),
        raw=raw,
    )


# "If {an effect|you|an opponent} would put one or more counters on a
#  permanent or player, {they/it} put(s) twice/half that many of each of
#  those kinds of counters on that permanent or player instead"
@_stat(r"^if (an effect|you|an opponent|they) would put one or more "
       r"(?:([+\-0-9/]+) )?counters on (?:a |one or more )?"
       r"([^,.]+?),\s*(?:it|you|they)\s+(?:puts?|put) "
       r"(twice|half|that many plus \d+) that many "
       r"(?:of each of those kinds? of )?counters on "
       r"([^.]+?)(?: instead)?(?:\.|$)")
def _replace_global(m, raw):
    return Static(
        modification=Modification(
            kind="counter_replacement_global",
            args=(m.group(1).lower(), (m.group(2) or "").lower(),
                  m.group(3).strip().lower(), m.group(4).lower(),
                  m.group(5).strip().lower()),
        ),
        raw=raw,
    )


# "If ~ would have {kind} counters put on it,
#  that many plus N counters are put on it instead"
@_stat(rf"^if {_SELF} would have {_KIND_CAP} counters? put on it,"
       r"\s*that many plus (\d+) counters? are put on it instead(?:\.|$)")
def _replace_self_plus_n(m, raw):
    return Static(
        modification=Modification(
            kind="self_counter_replacement_plus_n",
            args=(m.group(1).lower(), int(m.group(2))),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES
# One-shot counter manipulations and triggered-ability bodies.
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ------------------------------------------------------------------
# "Double the number of {kind} counters on {target}"
# ------------------------------------------------------------------
@_eff(rf"^double the number of {_KIND_CAP} counters on ([^.]+?)(?:\.|$)")
def _double_counters(m):
    tgt = m.group(2).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature",
               "that permanent"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="double", count=1,
                      counter_kind=m.group(1).lower(), target=target)


# "Double the number of each kind of counter on {target}"
@_eff(r"^double the number of each kind of counter on ([^.]+?)(?:\.|$)")
def _double_counters_each(m):
    tgt = m.group(1).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="double", count=1, counter_kind="each", target=target)


# ------------------------------------------------------------------
# "Put N {kind} counters on {target}" — generic beyond +1/+1
# ------------------------------------------------------------------
@_eff(rf"^put ({_N}) {_KIND_CAP} counters? on ([^.]+?)(?:\.|$)")
def _put_counter_generic(m):
    n = _num(m.group(1))
    kind = m.group(2).lower()
    tgt = m.group(3).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature",
               "that permanent", "that land", "this artifact", "this land",
               "this enchantment"}:
        target = Filter(base="self", targeted=False)
    elif tgt.startswith("each "):
        target = Filter(base=tgt[5:], quantifier="each", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="put", count=n, counter_kind=kind, target=target)


# "Put a/an {kind} counter on {target}"
@_eff(rf"^put a(?:n)? {_KIND_CAP} counter on ([^.]+?)(?:\.|$)")
def _put_single_counter(m):
    kind = m.group(1).lower()
    tgt = m.group(2).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature",
               "that permanent", "this artifact", "this land",
               "this enchantment"}:
        target = Filter(base="self", targeted=False)
    elif tgt.startswith("each "):
        target = Filter(base=tgt[5:], quantifier="each", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="put", count=1, counter_kind=kind, target=target)


# ------------------------------------------------------------------
# "Remove a/N {kind} counter(s) from {target}"
# ------------------------------------------------------------------
@_eff(rf"^remove ({_N}) {_KIND_CAP} counters? from ([^.]+?)(?:\.|$)")
def _remove_counters_n(m):
    n = _num(m.group(1))
    kind = m.group(2).lower()
    tgt = m.group(3).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature",
               "that permanent"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="remove", count=n, counter_kind=kind, target=target)


# "Remove all {kind} counters from {target}"
@_eff(rf"^remove all {_KIND_CAP} counters from ([^.]+?)(?:\.|$)")
def _remove_all_counters(m):
    kind = m.group(1).lower()
    tgt = m.group(2).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature",
               "that permanent"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="remove", count="all", counter_kind=kind, target=target)


# "Remove X counters from target permanent, where X is ..."
@_eff(rf"^remove x counters? from ([^.]+?), where x is ([^.]+?)(?:\.|$)")
def _remove_x_counters(m):
    tgt = m.group(1).strip().lower()
    target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="remove", count="x", counter_kind="any", target=target)


# ------------------------------------------------------------------
# "Move a/N {kind} counters from X to Y"
# ------------------------------------------------------------------
@_eff(rf"^move ({_N}) {_KIND_CAP} counters? from ([^.]+?) (?:to|onto) "
      r"([^.]+?)(?:\.|$)")
def _move_counters(m):
    n = _num(m.group(1))
    return CounterMod(op="move", count=n, counter_kind=m.group(2).lower(),
                      target=Filter(base=m.group(4).strip().lower(),
                                    targeted="target" in m.group(4)))


@_eff(rf"^move a(?:n)? {_KIND_CAP} counter from ([^.]+?) (?:to|onto) "
      r"([^.]+?)(?:\.|$)")
def _move_single_counter(m):
    return CounterMod(op="move", count=1, counter_kind=m.group(1).lower(),
                      target=Filter(base=m.group(3).strip().lower(),
                                    targeted="target" in m.group(3)))


# ------------------------------------------------------------------
# "Distribute {N|X} +1/+1 counters among {filter}"
# ------------------------------------------------------------------
@_eff(rf"^distribute ({_N}) \+1/\+1 counters? among "
      r"(?:one, two, or three )?([^.]+?)(?:\.|$)")
def _distribute_counters(m):
    n = _num(m.group(1))
    scope = m.group(2).strip().lower()
    return CounterMod(op="distribute", count=n, counter_kind="+1/+1",
                      target=Filter(base=scope,
                                    quantifier="any",
                                    targeted="target" in scope))


# "Distribute {N} {kind} counters among any number of ..."
@_eff(rf"^distribute ({_N}) {_KIND_CAP} counters? among (?:any number of )?"
      r"([^.]+?)(?:\.|$)")
def _distribute_counters_kind(m):
    n = _num(m.group(1))
    scope = m.group(3).strip().lower()
    return CounterMod(op="distribute", count=n, counter_kind=m.group(2).lower(),
                      target=Filter(base=scope,
                                    quantifier="any",
                                    targeted="target" in scope))


# ------------------------------------------------------------------
# Proliferate (non-keyword body — bullet, reminder-less, or mid-sentence)
# ------------------------------------------------------------------
@_eff(r"^proliferate(?:\.|$)")
def _proliferate(m):
    return UnknownEffect(raw_text="proliferate")


@_eff(r"^then proliferate(?:\.|$)")
def _then_proliferate(m):
    return UnknownEffect(raw_text="proliferate")


# ------------------------------------------------------------------
# "Each opponent gets a poison counter" — player-counter one-shots
# ------------------------------------------------------------------
@_eff(rf"^each opponent gets a(?:n)? {_KIND_CAP} counter(?:\.|$)")
def _each_opp_gets_counter(m):
    return CounterMod(op="put", count=1, counter_kind=m.group(1).lower(),
                      target=Filter(base="opponent", quantifier="each",
                                    targeted=False))


@_eff(rf"^target (?:player|opponent) gets ({_N}) {_KIND_CAP} counters?"
      r"(?:\.|$)")
def _player_gets_counters(m):
    n = _num(m.group(1))
    return CounterMod(op="put", count=n, counter_kind=m.group(2).lower(),
                      target=Filter(base="player", targeted=True))


# ------------------------------------------------------------------
# "Put a number of +1/+1 counters on it equal to ..."
# ------------------------------------------------------------------
@_eff(rf"^put a number of {_KIND_CAP} counters? on ([^.]+?) equal to "
      r"([^.]+?)(?:\.|$)")
def _put_counters_equal(m):
    tgt = m.group(2).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="put", count="var", counter_kind=m.group(1).lower(),
                      target=target)


# ------------------------------------------------------------------
# "Put X +1/+1 counters on it" for "when you do, ..." chain bodies
# ------------------------------------------------------------------
@_eff(r"^put x \+1/\+1 counters? on ([^.]+?)(?:\.|$)")
def _put_x_p1p1(m):
    tgt = m.group(1).strip().lower()
    if tgt in {"~", "this creature", "this permanent", "it", "that creature"}:
        target = Filter(base="self", targeted=False)
    else:
        target = Filter(base=tgt, targeted="target" in tgt)
    return CounterMod(op="put", count="x", counter_kind="+1/+1",
                      target=target)


__all__ = ["TRIGGER_PATTERNS", "STATIC_PATTERNS", "EFFECT_RULES"]


# ---------------------------------------------------------------------------
# Self-check: when run directly, verify patterns compile and each trigger
# matches at least one exemplar. Not a regression test — smoke only.
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    trig_samples = [
        "whenever one or more +1/+1 counters are put on ~",
        "whenever one or more -1/-1 counters are put on ~",
        "whenever one or more counters are put on a permanent you control",
        "whenever a +1/+1 counter is put on ~",
        "whenever a +1/+1 counter is put on a creature you control",
        "whenever a counter is put on a creature you control",
        "whenever you put one or more +1/+1 counters on a creature you control",
        "whenever you put a +1/+1 counter on a creature",
        "whenever you put a counter on a permanent you control",
        "whenever a time counter is removed from ~",
        "whenever a counter is removed from ~",
        "whenever a -1/-1 counter is removed from a creature you control",
        "whenever one or more +1/+1 counters are removed from ~",
        "whenever ~ has 3 or more +1/+1 counters on it",
        "when ~ has 4 or more charge counters on it",
        "whenever you proliferate",
    ]
    stat_samples = [
        "~ gets +1/+1 for each +1/+1 counter on it",
        "~ gets +1/+0 for each charge counter on it",
        "equipped creature gets +1/+1 for each charge counter on ~",
        "enchanted creature gets +1/+1 for each +1/+1 counter on it",
        "each creature you control with a +1/+1 counter on it has trample",
        "creatures you control with +1/+1 counters on them have vigilance",
        "as long as ~ has 3 or more +1/+1 counters on it, it has flying",
        "as long as ~ has a +1/+1 counter on it, it can't block",
        "if one or more +1/+1 counters would be put on a creature you control, "
        "that many plus 1 +1/+1 counters are put on it instead",
        "if one or more +1/+1 counters would be put on a creature you control, "
        "twice that many +1/+1 counters are put on that creature instead",
        "if one or more counters would be put on an artifact or creature you control, "
        "that many plus 1 of each of those kinds of counters are put on that permanent instead",
        "if ~ would have +1/+1 counters put on it, that many plus 1 counters are put on it instead",
    ]
    eff_samples = [
        "double the number of +1/+1 counters on ~",
        "double the number of +1/+1 counters on it",
        "double the number of each kind of counter on ~",
        "put a +1/+1 counter on target creature",
        "put a stun counter on it",
        "put a brick counter on this artifact",
        "put a depletion counter on this land",
        "put two +1/+1 counters on this creature",
        "put three +1/+1 counters on ~",
        "remove a +1/+1 counter from this creature",
        "remove all -1/-1 counters from ~",
        "remove x counters from target permanent, where x is its mana value",
        "move a +1/+1 counter from ~ to target creature",
        "distribute three +1/+1 counters among one, two, or three target creatures",
        "distribute x +1/+1 counters among any number of target creatures",
        "proliferate",
        "each opponent gets a poison counter",
        "put x +1/+1 counters on it",
    ]

    def _show(label, samples, table):
        print(f"\n--- {label} ({len(samples)} samples) ---")
        miss = 0
        for s in samples:
            hit = None
            for entry in table:
                pat = entry[0]
                if pat.match(s.rstrip(".")):
                    hit = entry[1] if len(entry) > 1 else "(matched)"
                    break
            if hit is None:
                print(f"MISS  {s}")
                miss += 1
            else:
                label_val = hit if isinstance(hit, str) else getattr(hit, "__name__", "?")
                print(f"OK    [{label_val}]  {s}")
        return miss

    total_miss = 0
    total_miss += _show("TRIGGERS", trig_samples, TRIGGER_PATTERNS)
    total_miss += _show("STATICS", stat_samples, STATIC_PATTERNS)
    total_miss += _show("EFFECTS", eff_samples, EFFECT_RULES)
    if total_miss:
        print(f"\n{total_miss} samples unmatched.")
        raise SystemExit(1)
    print(f"\nAll samples matched across {len(TRIGGER_PATTERNS)} triggers, "
          f"{len(STATIC_PATTERNS)} statics, {len(EFFECT_RULES)} effects.")
