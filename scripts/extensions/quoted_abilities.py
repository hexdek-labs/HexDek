#!/usr/bin/env python3
"""QUOTED-ABILITY GRANTING — cards that imprint or grant abilities written
verbatim inside double quotes.

Family covered (by parser-input phrasing, not card type):
  * Tribal / group anthems with quoted bodies:
      - 'creatures you control have "<ability>"'
      - 'all creatures have "<ability>"'        (Magus of the Tabernacle class)
      - 'all <tribe> creatures have "<ability>"' (Sedge Sliver / Sliver Gravemother)
      - 'other enchantments have "<ability>"'    (Aura Flux)
      - 'all artifacts have "<ability>"'         (Kataki, War's Wage)
      - 'commander creatures you own have "<ability>"' (Veteran Soldier class)
      - '<tribe> you control have "<ability>"'   (Hag of Mage's Doom warlocks)
  * Pronoun-led grants (chained off a previous trigger / effect):
      - 'it gains "<ability>"' / 'it has "<ability>"'
      - 'the creature gains "<ability>"' / 'that card gains "<ability>"'
      - 'they perpetually gain "<ability>"'      (Alchemy duplicates)
      - '<x> perpetually gains "<ability>"'       (Agent of Raffine class)
  * Temporary anthems with quoted body:
      - 'until end of turn, creatures you control gain "<ability>"'
      - 'until end of turn, it gains "<ability>"'
  * Mixed KW-and-quoted lists:
      - 'it has trample, haste, and "<ability>"'
      - 'creatures you control get +N/+N and gain "<ability>"'
      - 'it has flying and "<ability>"'
  * Boons / perpetual gains (Alchemy templating):
      - 'you get a one-time boon with "<ability>"'
      - 'that card perpetually gains "<ability>"'
  * Multi-quoted: '<x> gains "<a1>" and "<a2>"' (Uvilda, Mindbender Spores)
  * Special non-ability quotes:
      - 'the "legend rule" doesn't apply to ...'  (Mirror Box / Sliver Gravemother)

Strategy:
  When an effect contains "<X>", we hand the inner text X back to
  ``parser.parse_ability``. If it produces a real AST node, we wrap the grant
  as a ``Static`` whose ``Modification.args`` carry the inner AST (and the
  subject Filter). If the inner text fails to parse, we still fire the grant
  rule and stash the verbatim quoted text as a string — coverage gets credit
  for shape recognition either way (mirroring the existing ``token_specs``
  contract for tokens carrying granted abilities).

  Two registries are exported:
    - ``STATIC_PATTERNS`` for the "<filter> have/has ..." and "it gains ..."
      ability-shaped clauses (these are static rules text on a permanent or
      on a chained payoff).
    - ``EFFECT_RULES`` for the one-shot / temporary forms used inside spells
      and triggered abilities ("until end of turn, X gain ...").

Notes on quote handling:
  The parser lowercases & dash-normalises before dispatch; the quote characters
  reach us as straight ASCII ``"``. We use ``[^"]+`` inside quote spans which
  is safe because Magic oracle text never embeds another double-quote inside a
  quoted ability span — ability-text-within-ability-text is rendered with
  parentheses or italics. Two-grant cases use a tail group of ``(?:\\s+and\\s+
  "(?P<body2>[^"]+)")?``.

Inner-text recursion:
  ``_recurse_inner`` lazily imports the parser at call time (the extension is
  loaded *by* the parser, so a top-level import would be circular). Whatever
  the inner ability produces — Keyword / Triggered / Activated / Static / list
  thereof, or None — is preserved in ``Modification.args`` for downstream
  consumers. We never raise; failure becomes the verbatim string.
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
    Filter, GrantAbility, Modification, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Inner-ability recursion
# ---------------------------------------------------------------------------

def _recurse_inner(inner_text: str):
    """Try to parse the contents of a quoted ability as a self-contained
    ability. Delegates to ``parser.parse_quoted_ability`` — the canonical
    recursive entry point that handles quote/period cleanup and never raises.
    Returns whatever the parser produces (Keyword, Triggered, Activated,
    Static, list, or None)."""
    if not inner_text:
        return None
    try:
        import parser as P  # imported lazily; parser owns us
        hook = getattr(P, "parse_quoted_ability", None)
        if hook is not None:
            return hook(inner_text)
        # Fallback for older parser versions without the canonical hook.
        inner = inner_text.strip().rstrip(".").strip()
        return P.parse_ability(inner) if inner else None
    except Exception:
        return None


def _grant_payload(inner_text: str):
    """Return (parsed_ast_or_None, verbatim_text). The static/effect builders
    package both into Modification.args / GrantAbility extras so downstream
    callers can prefer the AST when available and fall back to text."""
    return (_recurse_inner(inner_text), inner_text.strip().rstrip(".").strip())


# ---------------------------------------------------------------------------
# Subject helpers — the "who is being granted the ability"
# ---------------------------------------------------------------------------

# Tribal/group subjects we recognise as the head of a "<subj> have/has ..." clause.
# Order matters within an alternation only when prefixes overlap; we capture the
# whole string and let _subject_filter bucket it.
_GROUP_SUBJ_RE = re.compile(
    r"^(?P<subject>"
    r"creatures you control"
    r"|other creatures you control"
    r"|nontoken creatures you control"
    r"|legendary creatures you control"
    r"|all creatures(?: you control)?"
    r"|all (?:[a-z][a-z'\- ]*?) creatures"   # "all sliver creatures", "all goblin creatures"
    r"|all (?:slivers|allies|elves|zombies|merfolk|goblins|wizards|warriors"
    r"|dragons|knights|soldiers|spirits|dwarves|elemental|elementals)"
    r"|all artifacts(?: you control)?"
    r"|all enchantments(?: you control)?"
    r"|all permanents(?: you control)?"
    r"|all lands(?: you control)?"
    r"|other enchantments(?: you control)?"
    r"|other artifacts(?: you control)?"
    r"|other lands(?: you control)?"
    r"|other creatures"
    r"|other permanents"
    r"|permanents you control"
    r"|artifacts you control"
    r"|enchantments you control"
    r"|lands you control"
    r"|commander creatures you own"
    r"|commanders you own"
    r"|[a-z][a-z'\- ]*? creatures you control"   # "sliver creatures you control"
    r"|[a-z][a-z'\- ]*?s you control"            # "warlocks you control" / "~s you control"
    r"|each creature you control"
    r"|each other creature you control"
    r"|blue creature cards in your hand"          # Alchemy "perpetually gain"
    r"|red creature cards in your hand"
    r"|black creature cards in your hand"
    r"|white creature cards in your hand"
    r"|green creature cards in your hand"
    r")"
)


def _subject_filter(s: str) -> Filter:
    """Bucket the captured subject phrase into a structured Filter."""
    s = s.strip().lower()
    you_control = "you control" in s or "you own" in s
    opp_control = "your opponents control" in s or "your opponent controls" in s
    # Compound subject: "creatures and planeswalkers your opponents control",
    # "creatures and lands you control", etc. Coarse-bucket as a mixed Filter
    # so downstream consumers can tell it's a multi-type grant.
    if " and " in s and any(
        n in s for n in ("planeswalker", "creatures and", "permanents and",
                         "artifacts and", "enchantments and", "lands and")
    ):
        return Filter(base="mixed_permanents", quantifier="all",
                      you_control=you_control,
                      opponent_controls=opp_control,
                      extra=(f"subject:{s}",))
    # Zone-qualified card subjects: "creature cards in your hand",
    # "artifact cards in your graveyard", "cards in your hand that share ..."
    m = re.match(r"^(.+?)\s+cards\s+in\s+(?:your|each player'?s)\s+(hand|graveyard)", s)
    if m:
        head = m.group(1).strip()
        zone = m.group(2).strip()
        return Filter(base="card", quantifier="all",
                      extra=(f"zone:{zone}", f"type:{head}"))
    # Token anthem: "warrior tokens you control"
    m = re.match(r"^([a-z'\- ]+?)\s+tokens(?:\s+you\s+control)?$", s)
    if m and m.group(1).strip() not in {"creature", "permanent", "artifact", "enchantment"}:
        return Filter(base="creature", quantifier="all",
                      you_control=you_control,
                      extra=("token_only", f"tribe:{m.group(1).strip()}"))
    # Tribal heads: "all <tribe> creatures" / "<tribe> creatures you control"
    m = re.match(r"^(?:all\s+)?([a-z'\- ]+?)\s+creatures(?:\s+you\s+control)?$", s)
    if m and m.group(1) not in {"other", "legendary", "nontoken"}:
        return Filter(base="creature", quantifier="all",
                      you_control=you_control,
                      extra=(f"tribe:{m.group(1).strip()}",))
    # Tribal head as bare plural ("warlocks you control", "all slivers")
    m = re.match(r"^(?:all\s+)?([a-z'\- ]+?)s(?:\s+you\s+control)?$", s)
    if m and m.group(1) not in {"creature", "permanent", "artifact", "enchantment",
                                 "land", "commander", "other creature"}:
        return Filter(base="creature", quantifier="all",
                      you_control=you_control,
                      extra=(f"tribe:{m.group(1).strip()}",))
    # Plain group nouns
    if "permanent" in s:
        base = "permanent"
    elif "artifact" in s:
        base = "artifact"
    elif "enchantment" in s:
        base = "enchantment"
    elif "land" in s:
        base = "land"
    elif "commander creature" in s:
        return Filter(base="creature", quantifier="all",
                      you_control=True, extra=("commander_only",))
    elif "creature cards in your hand" in s:
        return Filter(base="creature_card", quantifier="all",
                      extra=("zone:hand",))
    else:
        base = "creature"
    extras = []
    if s.startswith("other "):
        extras.append("other")
    if "legendary " in s:
        extras.append("legendary")
    if "nontoken " in s:
        extras.append("nontoken")
    return Filter(base=base, quantifier="all",
                  you_control=you_control, extra=tuple(extras))


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- 1. "the 'legend rule' doesn't apply to ..." — special non-ability quote.
# Has to win over the generic group-grant rule because the "subject" inside
# the quotes here is a *rule name*, not an ability body.
@_sp(r'^the "legend rule" doesn\'?t apply to (?P<who>[^.]+?)\s*$')
def _legend_rule_off(m, raw):
    return Static(
        modification=Modification(kind="legend_rule_off",
                                  args=(m.group("who").strip(),)),
        raw=raw,
    )


# --- 2. Group anthem with quoted ability:
#        '<group-subject> have/has "<ability>"'  (optional tail '... and "<ability2>"')
# Also accepts an interposed 'perpetually' adverb (Alchemy templating), and
# compound subjects like "creatures and planeswalkers your opponents control"
# or "cards in your hand that share a card type with that spell".
@_sp(
    r'^(?P<subject>'
    r'creatures you control|other creatures you control|nontoken creatures you control'
    r'|legendary creatures you control|all creatures(?: you control)?'
    r'|all [a-z][a-z\'\- ]*? creatures'
    r'|all (?:slivers|allies|elves|zombies|merfolk|goblins|wizards|warriors'
    r'|dragons|knights|soldiers|spirits|dwarves|elementals)'
    r'|all artifacts(?: you control)?|all enchantments(?: you control)?'
    r'|all permanents(?: you control)?|all lands(?: you control)?'
    r'|other enchantments(?: you control)?|other artifacts(?: you control)?'
    r'|other lands(?: you control)?|other creatures|other permanents'
    r'|permanents you control|artifacts you control|enchantments you control'
    r'|lands you control|commander creatures you own|commanders you own'
    r'|[a-z][a-z\'\- ]*? creatures you control'
    r'|[a-z][a-z\'\- ]*?s you control'
    # Color/tribe-prefixed creatures without "you control" suffix: "green
    # creatures have ...", "sliver creatures have ..." (global anthem).
    r'|(?:white|blue|black|red|green|colorless) creatures'
    r'|each creature you control|each other creature you control'
    r'|(?:blue|red|black|white|green) creature cards in your hand'
    # Compound subjects: "creatures and planeswalkers your opponents control",
    # "creatures and lands you control", etc.
    r'|(?:other\s+)?(?:creatures|permanents|artifacts|enchantments|lands)'
    r' and (?:creatures|permanents|artifacts|enchantments|lands|planeswalkers|tokens)'
    r'(?: (?:you|your opponents?) control)?'
    # Zone-qualified card subjects: "<type> cards in your hand ...",
    # "<type> cards in your graveyard", "cards in your hand that share ..."
    r'|(?:cards|creature cards|[a-z][a-z\'\- ]*? cards) in (?:your|each player\'?s) (?:hand|graveyard)'
    r'(?:\s+that\s+[^"]+?)?'
    # Per-turn/per-phase anthem subjects: "warrior tokens you control"
    r'|[a-z][a-z\'\- ]*? tokens you control'
    r')'
    r'\s+(?:perpetually\s+)?(?:have|has|gain|gains)\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    r'(?:\s*\.?\s*(?P<tail>(?:activate only|then |this ability)[^$]+))?'
    r'\s*\.?\s*$'
)
def _group_grants_quoted(m, raw):
    subj = _subject_filter(m.group("subject"))
    ast1, text1 = _grant_payload(m.group("body"))
    args = [text1, ast1, subj]
    if m.group("body2"):
        ast2, text2 = _grant_payload(m.group("body2"))
        args.extend([text2, ast2])
    if m.group("tail"):
        args.append(("tail", m.group("tail").strip()))
    return Static(
        modification=Modification(kind="group_quoted_ability_grant",
                                  args=tuple(args)),
        raw=raw,
    )


# --- 3. Pronoun-led perpetual / one-shot grant ridden by previous clause:
#        'it gains "<ability>"', 'it has "<ability>"',
#        'the creature gains "<ability>"',
#        'that token gains "<ability>"',
#        'they perpetually gain "<ability>"',
#        '<x> perpetually gains "<ability>"'
# Optional second quoted clause joined by 'and'.
@_sp(
    r'^(?P<subj>it|they|the (?:creature|token|duplicate|duplicates|card|cards)'
    r'|that (?:creature|token|card|player|permanent)'
    r'|those (?:creatures|tokens|cards|permanents)'
    r'|each (?:creature|token|card|permanent) [^"]*?'
    r'|the duplicate|the duplicates'
    r')'
    r'(?:\s+perpetually)?\s+(?:gains?|has|have)\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    # Allow a trailing rider clause after the quoted ability — Magic regularly
    # appends "activate only as a sorcery", "then shuffle", "this ability
    # costs {N} less", etc. We capture-and-stash so the static still parses.
    r'(?:\s*\.?\s*(?P<tail>(?:activate only|then |this ability|for as long as)[^$]+))?'
    r'\s*\.?\s*$'
)
def _pronoun_grants_quoted(m, raw):
    raw_subj = m.group("subj").strip()
    perp = "perpetually" in raw.lower()
    # Bucket subject loosely — we don't know the antecedent here.
    if raw_subj.startswith("they") or raw_subj.startswith("those") \
            or raw_subj.endswith("s") or "duplicates" in raw_subj:
        subj = Filter(base="them", targeted=False)
    elif "token" in raw_subj:
        subj = Filter(base="that_token", targeted=False)
    elif "card" in raw_subj:
        subj = Filter(base="that_card", targeted=False)
    elif "player" in raw_subj:
        subj = Filter(base="that_player", targeted=False)
    elif "permanent" in raw_subj:
        subj = Filter(base="that_permanent", targeted=False)
    else:
        subj = Filter(base="that_creature", targeted=False)
    ast1, text1 = _grant_payload(m.group("body"))
    args = [text1, ast1, subj]
    if m.group("body2"):
        ast2, text2 = _grant_payload(m.group("body2"))
        args.extend([text2, ast2])
    if m.group("tail"):
        args.append(("tail", m.group("tail").strip()))
    kind = "pronoun_perpetual_quoted_grant" if perp else "pronoun_quoted_grant"
    return Static(
        modification=Modification(kind=kind, args=tuple(args)),
        raw=raw,
    )


# --- 4. Mixed list: "it has KW1, KW2, and \"...\""
#        "it has trample, haste, and \"At the beginning of the end step, ...\""
#        "it has flying and \"...\""
@_sp(
    r'^(?P<subj>it|that creature|that token|the creature|the token)'
    r'\s+(?:has|gains)\s+'
    r'(?P<kws>[a-z][a-z, ]*?)'
    r'(?:,)?\s+and\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _pronoun_kw_then_quoted(m, raw):
    raw_subj = m.group("subj").strip().lower()
    if "token" in raw_subj:
        subj = Filter(base="that_token", targeted=False)
    else:
        subj = Filter(base="that_creature", targeted=False)
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast1, text1 = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="pronoun_kw_and_quoted_grant",
            args=(tuple(kws), text1, ast1, subj),
        ),
        raw=raw,
    )


# --- 4b. "<aura/equip subj> has KW(s) and \"<ability>\"" — fills a gap that
# equipment_aura's "<subj> has <body>" rule doesn't cover (it stops at quotes).
@_sp(
    r'^(?P<subject>enchanted creature|enchanted permanent|equipped creature)'
    r'\s+(?:has|gains)\s+'
    r'(?P<kws>[a-z][a-z, \-]*?(?:\s+\d+)?)'
    r'(?:,)?\s+and\s+'
    r'(?:has\s+)?'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*$'
)
def _aura_kw_and_quoted(m, raw):
    subj_text = m.group("subject").lower()
    if subj_text == "equipped creature":
        subj = Filter(base="equipped_creature", targeted=False)
    elif subj_text == "enchanted permanent":
        subj = Filter(base="enchanted_permanent", targeted=False)
    else:
        subj = Filter(base="enchanted_creature", targeted=False)
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast1, text1 = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="aura_kw_and_quoted_grant" if subj_text.startswith("enchanted")
            else "equip_kw_and_quoted_grant",
            args=(tuple(kws), text1, ast1, subj),
        ),
        raw=raw,
    )


# --- 2b. Conditional-prefix wrapper on a group/pronoun grant:
#   "during your end step, warrior tokens you control have \"...\""
#   "for as long as it remains exiled, it has \"...\""
#   "until end of turn, target creature gains \"...\""  (handled by EFFECT_RULES
#    already; this static form is the non-temporary variant).
# We re-dispatch the payload through the already-loaded STATIC_PATTERNS by
# stripping the prefix and packaging the result with a condition marker.
@_sp(
    r'^(?P<cond>(?:during [^,]+|for as long as [^,]+|while [^,]+))'
    r',\s+(?P<payload>.+)$'
)
def _prefixed_grant(m, raw):
    cond = m.group("cond").strip()
    payload = m.group("payload").strip().rstrip(".").strip()
    # Re-run the static patterns on the inner payload. We look at the
    # registry from this module (the outer parser will also try us recursively,
    # but that's cheap and avoids depending on import-order for the patched
    # parser module).
    for pat, builder in STATIC_PATTERNS:
        if builder is _prefixed_grant:
            continue  # avoid infinite recursion
        mm = pat.match(payload)
        if not mm:
            continue
        try:
            inner = builder(mm, payload)
        except Exception:
            continue
        if inner is None:
            continue
        # Wrap the inner static in a conditional so the prefix is preserved.
        return Static(
            condition=inner.condition,
            modification=Modification(
                kind="conditional_prefix_grant",
                args=(cond, inner.modification),
            ),
            raw=raw,
        )
    return None


# --- 4d. "<aura/equip subj> has \"A\" and \"B\"" — TWO quoted abilities, no
# keyword run. Fills a gap that 4b needed: Glowcap Lantern's equipped creature
# grants two independent quoted abilities. Each body is recursed independently.
@_sp(
    r'^(?P<subject>enchanted creature|enchanted permanent|equipped creature'
    r'|~ creatures you control|~\'s? equipped creature)'
    r'\s+(?:has|gains|have|gain)\s+'
    r'"(?P<body>[^"]+)"\s+and\s+"(?P<body2>[^"]+)"'
    r'\s*\.?\s*$'
)
def _aura_two_quoted(m, raw):
    subj_text = m.group("subject").lower()
    if "equipped" in subj_text:
        subj = Filter(base="equipped_creature", targeted=False)
        kind = "equip_two_quoted_grant"
    elif "enchanted permanent" in subj_text:
        subj = Filter(base="enchanted_permanent", targeted=False)
        kind = "aura_two_quoted_grant"
    else:
        subj = Filter(base="enchanted_creature", targeted=False)
        kind = "aura_two_quoted_grant"
    ast1, text1 = _grant_payload(m.group("body"))
    ast2, text2 = _grant_payload(m.group("body2"))
    return Static(
        modification=Modification(kind=kind,
                                  args=(text1, ast1, text2, ast2, subj)),
        raw=raw,
    )


# --- 4c. "it has KW1, KW2, and \"<ability>\". <tail>" — pronoun KW+quoted with tail.
# Same shape as #4 but accepts a trailing rider clause (activate-only/then-X).
@_sp(
    r'^(?P<subj>it|that creature|that token|the creature|the token)'
    r'\s+(?:has|gains)\s+'
    r'(?P<kws>[a-z][a-z, \-]*?)'
    r'(?:,)?\s+and\s+'
    r'"(?P<body>[^"]+)"'
    r'\s*\.?\s*(?P<tail>(?:activate only|then |this ability|for as long as)[^$]+)'
    r'\s*\.?\s*$'
)
def _pronoun_kw_then_quoted_tail(m, raw):
    raw_subj = m.group("subj").strip().lower()
    subj = (Filter(base="that_token", targeted=False) if "token" in raw_subj
            else Filter(base="that_creature", targeted=False))
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast1, text1 = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="pronoun_kw_and_quoted_grant_tail",
            args=(tuple(kws), text1, ast1, subj,
                  ("tail", m.group("tail").strip())),
        ),
        raw=raw,
    )


# --- 4e. Target/compound-target quoted grant — chained off a trigger body.
#   "target creature you control or creature card in your graveyard
#    perpetually gains \"<ability>\""
#   "another target creature gains \"<ability>\""
# This is a frequent shape inside triggered abilities; we surface it as a
# Static so parse_effect's fallback can wrap it uniformly.
@_sp(
    r'^(?P<subject>(?:another\s+)?target [^"]+?)'
    r'\s+(?:perpetually\s+)?(?:gains?|have|has|gain)\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    r'\s*\.?\s*$'
)
def _target_filter_quoted_grant(m, raw):
    subj_text = m.group("subject").strip()
    # Coarse-bucket: is it a compound "X or Y" target?
    is_compound = " or " in subj_text
    subj = Filter(base="compound_target" if is_compound else "target_filter",
                  quantifier="one", targeted=True,
                  extra=(f"subject:{subj_text}",))
    ast1, text1 = _grant_payload(m.group("body"))
    args = [text1, ast1, subj]
    if m.group("body2"):
        ast2, text2 = _grant_payload(m.group("body2"))
        args.extend([text2, ast2])
    return Static(
        modification=Modification(kind="target_quoted_grant",
                                  args=tuple(args)),
        raw=raw,
    )


# --- 5. "you get a one-time boon with \"<ability>\"" — Alchemy boon templating.
@_sp(
    r'^you get a one-time boon with\s+"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _boon_quoted(m, raw):
    ast, text = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(kind="boon_quoted",
                                  args=(text, ast)),
        raw=raw,
    )


# --- 6. "<group-subject> get +N/+N and gain \"<ability>\"" — combined buff+grant.
@_sp(
    r'^(?P<subject>creatures you control|other creatures you control'
    r'|[a-z][a-z\'\- ]*? creatures(?: you control)?)'
    r'\s+get\s+(?P<p>[+-]?\d+)/(?P<t>[+-]?\d+)\s+and\s+'
    r'(?:has|have|gain|gains)\s+'
    r'"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _group_buff_and_quoted(m, raw):
    subj = _subject_filter(m.group("subject"))
    ast, text = _grant_payload(m.group("body"))
    return Static(
        modification=Modification(
            kind="group_buff_and_quoted_grant",
            args=(int(m.group("p")), int(m.group("t")), text, ast, subj),
        ),
        raw=raw,
    )


# --- 6b. Group "<subj> have KW(s) and \"<ability>\"" — mixed KW + quoted on a group.
@_sp(
    r'^(?P<subject>creatures you control|other creatures you control'
    r'|commander creatures you own|commanders you own'
    r'|[a-z][a-z\'\- ]*? creatures(?: you control)?'
    r'|[a-z][a-z\'\- ]*?s you control)'
    r'\s+(?:have|has|gain|gains)\s+'
    r'(?P<kws>[a-z][a-z, \-]*?(?:\s+\d+)?)'
    r'(?:,)?\s+and\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    r'\s*\.?\s*$'
)
def _group_kw_and_quoted(m, raw):
    subj = _subject_filter(m.group("subject"))
    kws = [k.strip() for k in m.group("kws").split(",") if k.strip()]
    ast1, text1 = _grant_payload(m.group("body"))
    args = [tuple(kws), text1, ast1]
    if m.group("body2"):
        ast2, text2 = _grant_payload(m.group("body2"))
        args.extend([text2, ast2])
    args.append(subj)
    return Static(
        modification=Modification(kind="group_kw_and_quoted_grant",
                                  args=tuple(args)),
        raw=raw,
    )


# --- 7. Imprint copy-of-abilities (Chrome Mox / Mirror Gallery class):
#        '~ has all activated abilities of the imprinted card'
#        '~ has the imprinted card\'s activated abilities, except its mana abilities'
@_sp(
    r'^(?:~|this (?:creature|artifact|permanent|equipment))'
    r'\s+(?:has|gains)\s+'
    r'(?:all\s+)?(?:the\s+)?(?:imprinted card\'?s?\s+)?'
    r'(?P<scope>activated|triggered|all|all activated and triggered) abilities'
    r'(?:\s+of the imprinted card)?'
    r'(?:,?\s*except (?:its|all) mana abilities)?'
    r'\s*\.?\s*$'
)
def _imprint_copy_abilities(m, raw):
    scope = m.group("scope").lower()
    excepts_mana = "mana abilities" in raw.lower()
    return Static(
        modification=Modification(
            kind="imprint_copy_abilities",
            args=(scope, "no_mana" if excepts_mana else "all"),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES — one-shot grants used inside spell / triggered ability bodies
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# "<target filter> perpetually gains \"<ability>\"" — chained inside a
# triggered-ability body. Kami of Mourning / Brittle Blast patterns, now firing
# as EFFECT_RULES so the parent Triggered/Static parser sees a clean Effect.
@_er(
    r'^(?P<subject>(?:another\s+)?target [^"]+?'
    r'|that card|that creature|that token|that permanent|it|them)'
    r'\s+perpetually\s+(?:gains?|get)\s+'
    r'"(?P<body>[^"]+)"'
    r'(?:\s+and\s+"(?P<body2>[^"]+)")?'
    r'\s*\.?\s*$'
)
def _perpetual_quoted_grant(m):
    subj_text = m.group("subject").strip().lower()
    if subj_text in ("it", "them"):
        target = Filter(base="that_creature" if subj_text == "it" else "them",
                        targeted=False)
    elif subj_text.startswith("that "):
        target = Filter(base="that_" + subj_text.split()[1], targeted=False)
    else:
        target = Filter(base="creature", quantifier="one", targeted=True,
                        extra=(f"subject:{subj_text}",))
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return GrantAbility(
        ability_name=f'"{inner_text}"',
        target=target,
        duration="perpetual",
    )


# "until end of turn, creatures you control gain \"<ability>\""
@_er(
    r'^until end of turn,\s+'
    r'(?P<subject>creatures you control|other creatures you control'
    r'|[a-z][a-z\'\- ]*? creatures(?: you control)?'
    r'|it|they|target creature|target creature you control'
    r'|any number of target creatures'
    r'|up to (?:one|two|three|x|\d+) target creatures?)'
    r'\s+(?:each\s+)?(?:gain|gains|have|has)\s+'
    r'"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _eot_quoted_grant(m):
    raw_subj = m.group("subject").strip().lower()
    if raw_subj in ("it",):
        target = Filter(base="that_creature", targeted=False)
    elif raw_subj in ("they",):
        target = Filter(base="them", targeted=False)
    elif raw_subj.startswith("target creature"):
        target = Filter(base="creature", targeted=True)
    elif raw_subj.startswith("up to "):
        target = Filter(base="creature", quantifier="up_to_n", targeted=True)
    else:
        target = _subject_filter(raw_subj)
    inner_ast, inner_text = _grant_payload(m.group("body"))
    # We carry the verbatim text in ability_name for backward-compatibility
    # with the existing GrantAbility surface; downstream tooling can introspect
    # the parsed_ast on the modification path instead.
    return GrantAbility(
        ability_name=f'"{inner_text}"',
        target=target,
        duration="until_end_of_turn",
    )


# "until end of turn, creatures you control get +N/+N and gain \"<ability>\""
@_er(
    r'^until end of turn,\s+'
    r'(?P<subject>creatures you control|other creatures you control)'
    r'\s+get\s+(?P<p>[+-]?\d+)/(?P<t>[+-]?\d+)\s+and\s+'
    r'(?:gain|gains)\s+"(?P<body>[^"]+)"\s*\.?\s*$'
)
def _eot_buff_and_quoted(m):
    target = _subject_filter(m.group("subject"))
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return GrantAbility(
        ability_name=f'+{m.group("p")}/+{m.group("t")} and "{inner_text}"',
        target=target,
        duration="until_end_of_turn",
    )


# "target creature gains \"<ability>\" until end of turn"
@_er(
    r'^target creature gains\s+"(?P<body>[^"]+)"\s+until end of turn\s*\.?\s*$'
)
def _target_creature_gains_quoted(m):
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return GrantAbility(
        ability_name=f'"{inner_text}"',
        target=Filter(base="creature", targeted=True),
        duration="until_end_of_turn",
    )


# "<creature> gains \"<ability>\" until end of turn" — generic targeted grant
@_er(
    r'^(?P<filter>(?:up to (?:one|two|three|x|\d+) )?target [^"]+?creatures?(?: you control)?)'
    r'\s+(?:each\s+)?(?:gain|gains)\s+"(?P<body>[^"]+)"\s+until end of turn\s*\.?\s*$'
)
def _filter_gains_quoted_eot(m):
    inner_ast, inner_text = _grant_payload(m.group("body"))
    return GrantAbility(
        ability_name=f'"{inner_text}"',
        target=Filter(base="creature", quantifier="up_to_n", targeted=True),
        duration="until_end_of_turn",
    )


# ---------------------------------------------------------------------------
# Public API parity with equipment_aura.apply_extensions (optional)
# ---------------------------------------------------------------------------

def apply_extensions(parser_module) -> None:
    """Inline-patch a ``parser`` module: prepend STATIC_PATTERNS to the static
    fast-path and append EFFECT_RULES to the dispatch table. Mirrors the
    contract used by ``equipment_aura.apply_extensions`` for harness/test
    callers; ``parser.load_extensions`` already wires us in via the module
    registries on import, so this is opt-in only."""
    parser_module.EFFECT_RULES.extend(EFFECT_RULES)
    original_parse_static = parser_module.parse_static

    def patched_parse_static(text: str):
        cleaned = text.strip().rstrip(".")
        low = cleaned.lower()
        for pat, builder in STATIC_PATTERNS:
            m = pat.match(low)
            if not m:
                continue
            try:
                result = builder(m, text)
            except Exception:
                continue
            if result is not None:
                return result
        return original_parse_static(text)

    parser_module.parse_static = patched_parse_static


__all__ = ["STATIC_PATTERNS", "EFFECT_RULES", "apply_extensions"]
