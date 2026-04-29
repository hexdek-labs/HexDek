#!/usr/bin/env python3
"""STRUCTURAL FIXES — rider composition + richer quoted-ability grants.

Implements two of the small structural fixes from
``data/rules/partial_diagnostic.md``:

* **Fix #5 — Effect-rider composition.** Verb rules currently bail when an
  oracle clause grows a trailing rider (`under your control`,
  `tapped and attacking`, `with a +1/+1 counter on it`,
  `attached to that creature`, `face down`). Cards like Breaking // Entering,
  Dance of the Dead, Command the Dreadhorde, Ragavan/Mishra (`enters tapped
  and attacking`), and Gift of Immortality all PARTIAL because of the rider
  tail, not the verb. We bolt rider-tolerant variants onto the existing verbs
  using ``EFFECT_RULES`` — no AST schema change, the rider text is preserved
  as ``with_modifications`` (Reanimate) or as a sibling Static modification.

* **Fix #6 — Richer quoted-ability grants.** ``quoted_abilities`` already
  recognises the bulk of `<subject> have/gain "<body>"` shapes, but a long
  tail still PARTIALs because the subject form falls outside the existing
  alternation: ``the target <filter> perpetually gains "..."``,
  ``the duplicate of <X> perpetually gains "..."``,
  ``the <adj> tokens has/have "..."``, ``~ perpetually gains "..."``,
  ``the [card|land|copy|enchantment] gains "..."``,
  ``you create <token> with "..."``, and
  ``another target player creates <token> with "..."``. We add high-precision
  shapes that re-use the existing ``parse_quoted_ability`` recursion, so the
  inner ability still gets parsed and stashed in ``Modification.args``. The
  result remains list-friendly (callers can grow the args tuple) without
  touching ``mtg_ast.py``.

Strategy: this file exports ``EFFECT_RULES`` and ``STATIC_PATTERNS`` exactly
like ``partial_scrubber*`` and ``quoted_abilities``. ``parser.load_extensions``
splices them into the dispatch tables. Specific shapes first; everything is
anchored so a partial match doesn't shadow an existing rule.
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
    Filter, GrantAbility, Modification, Reanimate, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Inner-ability recursion (mirrors quoted_abilities._grant_payload)
# ---------------------------------------------------------------------------

def _recurse_inner(inner_text: str):
    """Parse the contents of a quoted ability via parser.parse_quoted_ability.
    Lazy import — parser owns this extension."""
    if not inner_text:
        return None
    try:
        import parser as P  # noqa: WPS433 — lazy by design
        hook = getattr(P, "parse_quoted_ability", None)
        if hook is not None:
            return hook(inner_text)
        return P.parse_ability(inner_text.strip().rstrip(".").strip())
    except Exception:
        return None


def _grant_payload(inner_text: str):
    """Returns (parsed_ast_or_None, verbatim_text)."""
    return (_recurse_inner(inner_text), inner_text.strip().rstrip(".").strip())


# ===========================================================================
# EFFECT_RULES — fire from the verb-rule dispatch (parse_effect)
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Fix #5 — Reanimation / battlefield-placement riders.
# ---------------------------------------------------------------------------
# Cluster (~30 cards): "put X onto the battlefield <rider>" where rider is
# one or more of {tapped, under your control, face down, attacking, with a
# counter on it, with a stun counter, attached to Y, etc.}.
#
# We accept ANY rider tail that matches a permissive (but bounded) regex and
# stash it into Reanimate.with_modifications. The verb shape ("put <filter>
# card from <zone> onto the battlefield ...") stays the same; we just allow
# the rider tail.

# The set of recognized rider words. Order doesn't matter — we just stash
# whatever ran after "battlefield".
_RIDER_TAIL = (
    r"(?:\s+(?:tapped|untapped|face[\- ]down|attacking|blocking"
    r"|under (?:your|its owner'?s?|that player'?s?|their) control"
    r"|with (?:a |an |\d+ )?[+\-/\d\w]+\s+counters?(?:\s+on it)?"
    r"|with (?:a |an )?stun counters? on it"
    r"|attached to (?:that creature|target [^.]+?)"
    r"|and attacking|and (?:tapped|attacking)"
    r"|and remains? tapped"
    r"|, then shuffles?"
    r"))+"
)


# "put <filter> card[s] [<from zone>] onto the battlefield <riders>"
# The filter can include a leading actor-determiner (a / all / the / target /
# any number of / up to N / each), adjectives, mana-value/colour qualifiers.
@_er(
    r"^put\s+"
    r"(?P<who>(?:a|an|all|the|target|any number of|up to (?:one|two|three|four|x|\d+)|each)\s+"
    r"[^.]+?)"
    r"\s+(?:from\s+(?P<zone>[^.]+?)\s+)?"
    r"onto the battlefield"
    rf"(?P<riders>{_RIDER_TAIL})"
    r"\s*\.?\s*$"
)
def _put_onto_battlefield_rider(m):
    who_text = m.group("who").strip()
    zone = (m.group("zone") or "").strip() or "any_graveyard"
    rider_run = m.group("riders").strip()
    # Tokenize the riders into tags Reanimate can carry.
    riders: list[str] = []
    if "tapped" in rider_run:
        riders.append("tapped")
    if re.search(r"\bface[\- ]down\b", rider_run):
        riders.append("face_down")
    if "attacking" in rider_run:
        riders.append("attacking")
    if "blocking" in rider_run:
        riders.append("blocking")
    mc = re.search(r"with (?:a |an |\d+ )?([+\-/\d\w]+)\s+counters?", rider_run)
    if mc:
        riders.append(f"counter:{mc.group(1)}")
    if "under your control" in rider_run:
        riders.append("controller:you")
    elif "under its owner" in rider_run:
        riders.append("controller:owner")
    elif "under that player" in rider_run or "under their control" in rider_run:
        riders.append("controller:that_player")
    am = re.search(r"attached to (.+?)$", rider_run)
    if am:
        riders.append(f"attached_to:{am.group(1).strip()}")
    return Reanimate(
        query=Filter(base="card", quantifier="any", targeted=False,
                     extra=(f"who:{who_text}", f"zone:{zone}")),
        from_zone=zone,
        destination="battlefield",
        controller="you" if "controller:you" in riders else "owner",
        with_modifications=tuple(riders) or ("plain",),
    )


# "return X to the battlefield <riders>" — same shape, different verb.
@_er(
    r"^return\s+"
    r"(?P<who>(?:a|an|all|the|target|any number of|up to (?:one|two|three|four|x|\d+)|each|this)\s+"
    r"[^.]+?)"
    r"\s+(?:from\s+(?P<zone>[^.]+?)\s+)?"
    r"to the battlefield"
    rf"(?P<riders>{_RIDER_TAIL})"
    r"\s*\.?\s*$"
)
def _return_to_battlefield_rider(m):
    who_text = m.group("who").strip()
    zone = (m.group("zone") or "").strip() or "any_graveyard"
    rider_run = m.group("riders").strip()
    riders: list[str] = []
    if "tapped" in rider_run:
        riders.append("tapped")
    if "attacking" in rider_run:
        riders.append("attacking")
    if "under your control" in rider_run:
        riders.append("controller:you")
    elif "under its owner" in rider_run:
        riders.append("controller:owner")
    am = re.search(r"attached to (.+?)$", rider_run)
    if am:
        riders.append(f"attached_to:{am.group(1).strip()}")
    mc = re.search(r"with (?:a |an |\d+ )?([+\-/\d\w]+)\s+counters?", rider_run)
    if mc:
        riders.append(f"counter:{mc.group(1)}")
    return Reanimate(
        query=Filter(base="card", quantifier="any", targeted=False,
                     extra=(f"who:{who_text}", f"zone:{zone}")),
        from_zone=zone,
        destination="battlefield",
        controller="you" if "controller:you" in riders else "owner",
        with_modifications=tuple(riders) or ("plain",),
    )


# Bare "put them onto the battlefield <riders>" — pronoun subject (Command the
# Dreadhorde, Wild Magic Surge tail, Reaper King-class).
@_er(
    r"^put (?:them|those|those cards|that card|that creature)"
    r"\s+onto the battlefield"
    rf"(?P<riders>{_RIDER_TAIL})"
    r"\s*\.?\s*$"
)
def _put_pronoun_battlefield_rider(m):
    rider_run = m.group("riders").strip()
    riders = []
    if "tapped" in rider_run:
        riders.append("tapped")
    if "attacking" in rider_run:
        riders.append("attacking")
    if "face" in rider_run:
        riders.append("face_down")
    if "under your control" in rider_run:
        riders.append("controller:you")
    elif "under its owner" in rider_run:
        riders.append("controller:owner")
    return Reanimate(
        query=Filter(base="them", targeted=False),
        from_zone="implicit",
        destination="battlefield",
        controller="you" if "controller:you" in riders else "owner",
        with_modifications=tuple(riders) or ("plain",),
    )


# "<self/named> enters tapped and attacking" — Mishra/Ragavan-class. The base
# parser handles "enters tapped" as a static; "tapped and attacking" was the
# missing variant. Also covers "the creature enters tapped and attacking".
@_er(
    r"^(?:~|the (?:creature|token|copy|duplicate)|that (?:creature|token)"
    r"|[a-z][\w\s]*?)"
    r"\s+enters tapped and attacking"
    r"\s*\.?\s*$"
)
def _enters_tapped_attacking(m):
    return UnknownEffect(raw_text="enters tapped and attacking")


# "return this card to the battlefield attached to that creature" — Gift of
# Immortality / Bitterthorn class. The existing return-to-battlefield rule
# requires a specific filter; this one accepts the self-pronoun + attached.
@_er(
    r"^return this card to the battlefield"
    rf"(?:{_RIDER_TAIL})?"
    r"\s*\.?\s*$"
)
def _return_self_to_battlefield(m):
    return Reanimate(
        query=Filter(base="self", targeted=False),
        from_zone="implicit",
        destination="battlefield",
        controller="owner",
        with_modifications=("self_recur",),
    )


# ---------------------------------------------------------------------------
# Fix #6 — Richer quoted-ability grants.
# ---------------------------------------------------------------------------

# "you create <token spec> with \"<body>\"" — Rat Out, Send in the Pest. The
# token_specs extension handles bare "create <spec> with \"...\"" but the
# leading "you " causes those rules to miss. We catch the prefix and emit a
# CreateToken-shaped UnknownEffect wrapper carrying the parsed inner ability.
@_er(
    r'^you create\s+'
    r'(?P<spec>(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+[^"]+?)'
    r'\s+with\s+"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _you_create_token_with_quoted(m):
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return UnknownEffect(
        raw_text=f"you create {m.group('spec').strip()} with quoted_ability:{inner_text}"
    )


# "another target player creates <token spec> with \"<body>\"" — Cybernetica
# Datasmith class. Same payload shape, different actor.
@_er(
    r'^(?:another\s+)?target player creates\s+'
    r'(?P<spec>(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+[^"]+?)'
    r'\s+with\s+"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _target_player_creates_token_with_quoted(m):
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return UnknownEffect(
        raw_text=f"target player creates {m.group('spec').strip()} with quoted_ability:{inner_text}"
    )


# ===========================================================================
# STATIC_PATTERNS — fire from parse_static (extension fast path)
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# Build a generic "the <noun-phrase> (perpetually)? gains/has/have <quoted>"
# pattern. The noun-phrase is permissive — anything between "the " and the
# verb that doesn't itself contain " gains "/" has "/" have "/quote.
#
# Subjects we want to catch (none currently handled by quoted_abilities):
#   the target <filter>            (Brokers' Safeguard, Niambi)
#   the target nonlegendary creature (Niambi)
#   the duplicate (of <filter>)?    (Lae'zel, Clone Crafter)
#   the card / the land / the token (Realmbreaker, Cloudseeder)
#   the copy                        (Choreographed Sparks)
#   the chosen card                 (Tawnos Endures)
#   the 1/1 black rat tokens        (Mitotic Slime variants)
#   the white aura enchantment tokens (Daxos, Scriv, Smoke Spirits' Aid)
@_sp(
    r'^the\s+(?P<subject>(?!"|legend rule)[^"]*?)'
    r'\s+(?:perpetually\s+)?(?:gains?|has|have)\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    r'(?:\s*\.?\s*(?P<tail>(?:activate only|then |this ability|for as long as)[^$]+))?'
    r'\s*\.?\s*$'
)
def _the_subject_quoted_grant(m, raw):
    subj_text = m.group("subject").strip().lower()
    # Bucket the subject loosely so signature comparisons are stable.
    if "duplicate" in subj_text:
        subj = Filter(base="duplicate_of", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "token" in subj_text:
        subj = Filter(base="that_token", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif subj_text.startswith("target ") or subj_text.startswith("target nonlegendary"):
        subj = Filter(base="target_filter", quantifier="one", targeted=True,
                      extra=(f"subject:{subj_text}",))
    elif "card" in subj_text:
        subj = Filter(base="that_card", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "land" in subj_text:
        subj = Filter(base="that_land", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "creature" in subj_text:
        subj = Filter(base="that_creature", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "copy" in subj_text:
        subj = Filter(base="copy", targeted=False,
                      extra=(f"subject:{subj_text}",))
    else:
        subj = Filter(base="the_x", targeted=False,
                      extra=(f"subject:{subj_text}",))
    perp = "perpetually" in raw.lower()
    ast1, text1 = _grant_payload(m.group("body"))
    args = [text1, ast1, subj]
    if m.group("body2"):
        ast2, text2 = _grant_payload(m.group("body2"))
        args.extend([text2, ast2])
    if m.group("tail"):
        args.append(("tail", m.group("tail").strip()))
    kind = "the_subject_perpetual_quoted_grant" if perp else "the_subject_quoted_grant"
    return Static(
        modification=Modification(kind=kind, args=tuple(args)),
        raw=raw,
    )


# "the <subject> has/have/gains <KW(s)> and \"<body>\"" — mixed-list variant
# of the above. Splintering Wind ("the 1/1 green splinter tokens has flying
# and \"cumulative upkeep {g}.\""), Cloudseeder ("the card has flying and
# \"this token can block only creatures with flying.\""), Make Mischief
# (devil tokens), Rite of the Raging Storm (elemental tokens with multiple
# kws + quoted body).
@_sp(
    r'^the\s+(?P<subject>(?!"|legend rule)[^"]*?)'
    r'\s+(?:has|have|gains?)\s+'
    r'(?P<kws>[a-z][a-z, \-]*?(?:\s+\d+)?)'
    r'(?:,)?\s+and\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _the_subject_kw_and_quoted_grant(m, raw):
    subj_text = m.group("subject").strip().lower()
    if "token" in subj_text:
        subj = Filter(base="that_token", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "card" in subj_text:
        subj = Filter(base="that_card", targeted=False,
                      extra=(f"subject:{subj_text}",))
    elif "creature" in subj_text:
        subj = Filter(base="that_creature", targeted=False,
                      extra=(f"subject:{subj_text}",))
    else:
        subj = Filter(base="the_x", targeted=False,
                      extra=(f"subject:{subj_text}",))
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast1, text1 = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="the_subject_kw_and_quoted_grant",
            args=(tuple(kws), text1, ast1, subj),
        ),
        raw=raw,
    )


# "~ perpetually gains \"<body>\"" — Craving of Yeenoghu.
@_sp(
    r'^~\s+perpetually\s+(?:gains?|has)\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _self_perpetual_quoted_grant(m, raw):
    ast, text = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="self_perpetual_quoted_grant",
            args=(text, ast),
        ),
        raw=raw,
    )


# "they each have <KWs> and \"<body>\"" — Tetravus, Argothian Uprooting.
# Distinct from quoted_abilities #4 because the subject is "they each" + a
# plural KW list.
@_sp(
    r'^they each have\s+'
    r'(?P<kws>[a-z][a-z, \-]*?)'
    r'(?:,)?\s+and\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _they_each_kw_and_quoted(m, raw):
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast, text = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="they_each_kw_and_quoted_grant",
            args=(tuple(kws), text, ast,
                  Filter(base="them", targeted=False)),
        ),
        raw=raw,
    )


# "non-<tribe> creatures have \"<body>\"" — Clash of Realities. The existing
# quoted_abilities subject alternation includes "<tribe> creatures" but not
# the "non-X" prefix.
@_sp(
    r'^non-(?P<tribe>[a-z][a-z\'\- ]*?)\s+creatures\s+(?:have|has|gain|gains)\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _non_tribe_creatures_quoted(m, raw):
    tribe = m.group("tribe").strip()
    ast, text = _grant_payload(m.group("body"))
    subj = Filter(base="creature", quantifier="all",
                  extra=(f"non_tribe:{tribe}",))
    return Static(
        modification=Modification(
            kind="non_tribe_quoted_grant",
            args=(text, ast, subj),
        ),
        raw=raw,
    )


# "each creature has \"<body>\"" — Endless Whispers (a global anthem with a
# single quoted ability). Existing patterns require "you control"; this is
# the all-creatures-everywhere variant.
@_sp(
    r'^each\s+(?P<base>creature|permanent|artifact|enchantment|land)\s+(?:has|gains?)\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _each_global_quoted(m, raw):
    base = m.group("base")
    ast, text = _grant_payload(m.group("body"))
    subj = Filter(base=base, quantifier="each")
    return Static(
        modification=Modification(
            kind="each_global_quoted_grant",
            args=(text, ast, subj),
        ),
        raw=raw,
    )
