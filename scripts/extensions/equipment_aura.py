#!/usr/bin/env python3
"""Equipment & Aura static pattern extensions.

This module owns the grammar productions that show up on Auras and Equipment.
Two tables are exported:

- ``STATIC_PATTERNS``: ``(compiled_regex, builder_fn)`` entries to be merged
  into ``parser.parse_static``. Each builder receives the regex Match object
  and the raw (original-case) text. It must return a ``Static`` / ``Keyword``
  AST node, or ``None`` to signal "not actually a match".

- ``EFFECT_RULES``: ``(compiled_regex, builder_fn)`` entries to be appended to
  ``parser.EFFECT_RULES``. These match the body of an ability clause (inside a
  triggered/activated/spell ability) and return an ``Effect`` AST node.

Design notes:
  * "equipped creature / enchanted creature / enchanted permanent / enchanted
    land / enchanted artifact" are all handled by a single regex alternation
    captured as ``subject`` — this lets one pattern cover both Aura and
    Equipment buff/grant clauses.
  * +N/+N and -N/-N buffs are captured with a signed pattern so "-3/-0" style
    auras land in the same node as "+2/+2".
  * "enchanted creature gets +X and has <keywords>" combines a Buff with a
    GrantAbility — we emit a ``Static`` with ``kind='aura_buff_grant'`` and
    carry the parts in ``args`` so downstream code can split them.
  * Keyword-ability headers that Equipment/Aura cards use as standalone one-
    liners (Living Weapon, For Mirrodin!, Umbra Armor, Reconfigure {X}, Job
    Select, Modified …) are emitted as ``Keyword`` nodes.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Allow this module to live in scripts/extensions/ and still import the AST
# from scripts/mtg_ast.py without a package install.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Buff, Filter, GrantAbility, Keyword, Modification, Static, Trigger,
    Triggered, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Shared pieces
# ---------------------------------------------------------------------------

# Subject alternation — most Aura / Equipment statics are about "the attached
# thing". We keep both sides so builders can tell whether it was an Aura body
# ("enchanted X") or Equipment body ("equipped creature"), because that's the
# structural distinction downstream code cares about.
_SUBJ = (
    r"(?P<subject>"
    r"equipped creature"
    r"|enchanted creature"
    r"|enchanted permanent"
    r"|enchanted artifact"
    r"|enchanted land"
    r"|enchanted forest|enchanted plains|enchanted island"
    r"|enchanted swamp|enchanted mountain"
    r")"
)


def _subject_filter(subject: str) -> Filter:
    """Map the matched subject phrase onto a Filter AST node."""
    s = subject.lower()
    if s.startswith("equipped"):
        return Filter(base="equipped_creature", targeted=False)
    if s == "enchanted creature":
        return Filter(base="enchanted_creature", targeted=False)
    if s == "enchanted permanent":
        return Filter(base="enchanted_permanent", targeted=False)
    if s == "enchanted artifact":
        return Filter(base="enchanted_artifact", targeted=False)
    # land / basic-land variants
    base = s.replace("enchanted ", "enchanted_")
    return Filter(base=base, targeted=False)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# Ability-level static patterns. Each builder returns a Static / Keyword / None.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Buff-only: "<subject> gets +N/+N" (also handles negative mods and X) ---
@_sp(rf"^{_SUBJ} gets (?P<p>[+-]?\d+|[+-]x)/(?P<t>[+-]?\d+|[+-]x)\s*$")
def _subj_gets(m, raw):
    subj = m.group("subject")
    p_txt = m.group("p").lower()
    t_txt = m.group("t").lower()
    p = int(p_txt) if p_txt.lstrip("+-").isdigit() else p_txt
    t = int(t_txt) if t_txt.lstrip("+-").isdigit() else t_txt
    return Static(
        modification=Modification(
            kind="aura_buff" if subj.startswith("enchanted") else "equip_buff",
            args=(p, t, _subject_filter(subj)),
        ),
        raw=raw,
    )


# --- "<subject> gets +N/+N and has <keywords>" (or quoted ability text) ---
@_sp(
    rf"^{_SUBJ} gets (?P<p>[+-]?\d+|[+-]x)/(?P<t>[+-]?\d+|[+-]x) "
    r"and (?:has|gains) (?P<body>.+)$"
)
def _subj_gets_and_has(m, raw):
    subj = m.group("subject")
    p_txt, t_txt = m.group("p").lower(), m.group("t").lower()
    p = int(p_txt) if p_txt.lstrip("+-").isdigit() else p_txt
    t = int(t_txt) if t_txt.lstrip("+-").isdigit() else t_txt
    return Static(
        modification=Modification(
            kind="aura_buff_grant" if subj.startswith("enchanted")
            else "equip_buff_grant",
            args=(p, t, m.group("body").strip(), _subject_filter(subj)),
        ),
        raw=raw,
    )


# --- "<subject> has <keywords>" — pure keyword grant (flying, trample, …) ---
@_sp(rf"^{_SUBJ} (?:has|gains) (?P<body>[^.\"]+?)\s*$")
def _subj_has(m, raw):
    subj = m.group("subject")
    return Static(
        modification=Modification(
            kind="aura_grant" if subj.startswith("enchanted") else "equip_grant",
            args=(m.group("body").strip(), _subject_filter(subj)),
        ),
        raw=raw,
    )


# --- "<subject> has \"<inline ability text>\"" — bestowed inline ability ---
@_sp(rf"^{_SUBJ} (?:has|gains) \"(?P<body>[^\"]+)\"\s*\.?\s*$")
def _subj_has_quoted(m, raw):
    subj = m.group("subject")
    return Static(
        modification=Modification(
            kind="aura_inline_ability" if subj.startswith("enchanted")
            else "equip_inline_ability",
            args=(m.group("body").strip(), _subject_filter(subj)),
        ),
        raw=raw,
    )


# --- "<subject> has protection from <X>" (plus optional "and from Y") ---
@_sp(
    rf"^{_SUBJ} has protection from (?P<what>[a-z]+)"
    r"(?: and from (?P<also>[a-z]+))?\s*$"
)
def _subj_protection(m, raw):
    args = [m.group("what")]
    if m.group("also"):
        args.append(m.group("also"))
    return Static(
        modification=Modification(
            kind="aura_protection" if m.group("subject").startswith("enchanted")
            else "equip_protection",
            args=tuple(args) + (_subject_filter(m.group("subject")),),
        ),
        raw=raw,
    )


# --- "<subject> can't attack", "can't block", "can't attack or block", etc. ---
@_sp(
    rf"^{_SUBJ} can'?t (?P<what>attack(?: or block)?|block(?: or attack)?"
    r"|be blocked(?:[^.]*)?|become untapped|have counters put on it"
    r"|transform|be the target of[^.]*|attack you[^.]*)"
    r"(?:\s+and[^.]*)?\s*$"
)
def _subj_cant(m, raw):
    subj = m.group("subject")
    return Static(
        modification=Modification(
            kind="aura_restriction" if subj.startswith("enchanted")
            else "equip_restriction",
            args=(m.group("what").strip(), _subject_filter(subj)),
        ),
        raw=raw,
    )


# --- "<subject> can't attack or block[, and its activated abilities can't …]" ---
@_sp(
    rf"^{_SUBJ} can'?t attack or block, and its activated abilities can'?t be activated\s*$"
)
def _subj_lockdown(m, raw):
    return Static(
        modification=Modification(
            kind="aura_lockdown",
            args=(_subject_filter(m.group("subject")),),
        ),
        raw=raw,
    )


# --- "<subject> attacks each combat/turn if able" ---
@_sp(rf"^{_SUBJ} attacks (?:each combat|each turn|if able)(?:[^.]*)?\s*$")
def _subj_must_attack(m, raw):
    return Static(
        modification=Modification(
            kind="aura_must_attack" if m.group("subject").startswith("enchanted")
            else "equip_must_attack",
            args=(_subject_filter(m.group("subject")),),
        ),
        raw=raw,
    )


# --- "<subject> doesn't untap during its controller's untap step[…]" ---
@_sp(
    rf"^{_SUBJ} doesn'?t untap during its controller'?s untap step"
    r"(?:[^.]*)?\s*$"
)
def _subj_no_untap(m, raw):
    return Static(
        modification=Modification(
            kind="aura_no_untap" if m.group("subject").startswith("enchanted")
            else "equip_no_untap",
            args=(_subject_filter(m.group("subject")),),
        ),
        raw=raw,
    )


# --- "<subject> loses all abilities[, and …]" / type-strip ---
@_sp(rf"^{_SUBJ} loses (?P<what>[^.]+)$")
def _subj_loses(m, raw):
    return Static(
        modification=Modification(
            kind="aura_loses" if m.group("subject").startswith("enchanted")
            else "equip_loses",
            args=(m.group("what").strip(), _subject_filter(m.group("subject"))),
        ),
        raw=raw,
    )


# --- "<subject> is a [P/T] [types] creature …" (Frogify / Darksteel Mutation) ---
@_sp(
    rf"^{_SUBJ} is (?:an? )?(?P<body>[^.]*?(?:creature|artifact|enchantment)[^.]*)$"
)
def _subj_is_type(m, raw):
    return Static(
        modification=Modification(
            kind="aura_becomes" if m.group("subject").startswith("enchanted")
            else "equip_becomes",
            args=(m.group("body").strip(), _subject_filter(m.group("subject"))),
        ),
        raw=raw,
    )


# --- "<subject> is goaded" / "is monstrous" / other single-word state adds ---
@_sp(rf"^{_SUBJ} is (?P<what>goaded|monstrous|tapped|an? [a-z ]+?)\s*$")
def _subj_state(m, raw):
    return Static(
        modification=Modification(
            kind="aura_state" if m.group("subject").startswith("enchanted")
            else "equip_state",
            args=(m.group("what").strip(), _subject_filter(m.group("subject"))),
        ),
        raw=raw,
    )


# --- "you control enchanted creature/permanent" — control-swap aura ---
@_sp(r"^you control enchanted (?P<what>creature|permanent|artifact)\s*$")
def _you_control_enchanted(m, raw):
    return Static(
        modification=Modification(
            kind="aura_control_swap",
            args=(m.group("what"),),
        ),
        raw=raw,
    )


# --- "as this aura enters, choose a <thing>" / "as ~ enters, choose …" ---
@_sp(
    r"^as (?:this aura|this equipment|~) enters(?:[^,]*)?,\s*(?P<body>.+)$"
)
def _as_aura_enters(m, raw):
    return Static(
        modification=Modification(
            kind="aura_as_enters_choice",
            args=(m.group("body").strip(),),
        ),
        raw=raw,
    )


# --- "this effect doesn't remove this aura" — pump-on-blink rider ---
@_sp(r"^this effect doesn'?t remove (?:this aura|~)\s*$")
def _effect_doesnt_remove(m, raw):
    return Static(
        modification=Modification(kind="aura_persistent_effect"),
        raw=raw,
    )


# --- Standalone keyword-ability one-liners ------------------------------------

@_sp(r"^living weapon\s*$")
def _living_weapon(m, raw):
    return Keyword(name="living weapon", raw=raw)


@_sp(r"^for mirrodin!?\s*$")
def _for_mirrodin(m, raw):
    return Keyword(name="for mirrodin", raw=raw)


@_sp(r"^umbra armor\s*$")
def _umbra_armor(m, raw):
    return Keyword(name="umbra armor", raw=raw)


@_sp(r"^job select\s*$")
def _job_select(m, raw):
    return Keyword(name="job select", raw=raw)


@_sp(r"^reconfigure (?P<cost>\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _reconfigure(m, raw):
    return Keyword(name="reconfigure", args=(m.group("cost"),), raw=raw)


# --- Equip variants: "equip legendary creature {N}", "equip-pay {N}", etc. ---
@_sp(r"^equip legendary creature (?P<cost>\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _equip_legendary(m, raw):
    return Keyword(name="equip legendary creature",
                   args=(m.group("cost"),), raw=raw)


@_sp(r"^equip-pay (?P<body>.+)$")
def _equip_pay_alt(m, raw):
    # "equip-pay {3} or discard a card" / "equip-pay {E}{E}"
    return Keyword(name="equip", args=(m.group("body").strip(),), raw=raw)


# --- Aura-graveyard triggers that parser._TRIGGER_PATTERNS misses ----------
# These are strictly triggered abilities, but since we can't extend the
# parser's private trigger list, we emit Triggered nodes from here. The main
# parser accepts whatever truthy value parse_static returns.
@_sp(
    r"^when (?:this aura|this equipment|~) is put into a graveyard "
    r"from the battlefield,\s*(?P<body>.+)$"
)
def _aura_to_gy(m, raw):
    # Bind the tail as an UnknownEffect (keeps the trigger structurally typed
    # even if we haven't built the body effect yet).
    return Triggered(
        trigger=Trigger(event="ltb_to_graveyard"),
        effect=UnknownEffect(raw_text=m.group("body").strip()),
        raw=raw,
    )


@_sp(r"^when (?:this aura|this equipment|~) leaves the battlefield,\s*(?P<body>.+)$")
def _aura_ltb(m, raw):
    return Triggered(
        trigger=Trigger(event="ltb"),
        effect=UnknownEffect(raw_text=m.group("body").strip()),
        raw=raw,
    )


# Enchanted-land-tapped-for-mana triggers (Utopia Sprawl / Verdant Haven class)
@_sp(
    r"^whenever enchanted (?P<land>land|forest|plains|island|swamp|mountain) "
    r"is tapped for mana,\s*(?P<body>.+)$"
)
def _enchanted_land_tapped(m, raw):
    return Triggered(
        trigger=Trigger(event="enchanted_land_tapped_for_mana",
                        actor=Filter(base=f"enchanted_{m.group('land')}",
                                     targeted=False)),
        effect=UnknownEffect(raw_text=m.group("body").strip()),
        raw=raw,
    )


# "when enchanted <subject> becomes tapped/blocked/targeted, …"
@_sp(
    rf"^when {_SUBJ} becomes (?P<what>tapped|blocked|the target of [^,]+),"
    r"\s*(?P<body>.+)$"
)
def _subj_becomes(m, raw):
    return Triggered(
        trigger=Trigger(event=f"becomes_{m.group('what').split()[0]}",
                        actor=_subject_filter(m.group("subject"))),
        effect=UnknownEffect(raw_text=m.group("body").strip()),
        raw=raw,
    )


# "whenever this equipment becomes unattached …"
@_sp(
    r"^whenever (?:this equipment|~) becomes (?:un)?attached"
    r"(?: from [^,]+| to [^,]+)?,\s*(?P<body>.+)$"
)
def _equip_attached_trigger(m, raw):
    return Triggered(
        trigger=Trigger(event="equipment_attach_state_change"),
        effect=UnknownEffect(raw_text=m.group("body").strip()),
        raw=raw,
    )


# Multi-symbol equip cost: parser.KEYWORD_RE only captures a single {…} group,
# so costs like "equip {1}{R}" or "equip {B/P}{B/P}" fall through. Catch them
# here as a standalone Keyword.
@_sp(r"^equip (?P<cost>\{[^}]+\}(?:\{[^}]+\})+)\s*$")
def _equip_multi(m, raw):
    return Keyword(name="equip", args=(m.group("cost"),), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# Body-of-effect patterns — used inside triggered/activated abilities whose
# subject references the attached permanent.
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# "attach <equipment> to <target>" — used in Helm of Kaldra style triggers.
@_er(
    r"^attach (?:this equipment|~|those equipment|that equipment) "
    r"to (?P<who>[^.]+?)(?:\.|$)"
)
def _attach(m):
    return UnknownEffect(raw_text=f"attach to {m.group('who').strip()}")


# "that creature gains <keyword(s)> until end of turn" — post-attach rider.
@_er(
    r"^that creature gains (?P<kw>[a-z, ]+?) until end of turn(?:\.|$)"
)
def _that_creature_gains(m):
    return GrantAbility(
        ability_name=m.group("kw").strip(),
        target=Filter(base="that_creature", targeted=False),
        duration="until_end_of_turn",
    )


# "<subject> gets +N/+N until end of turn" as an activated/triggered body.
@_er(
    rf"^{_SUBJ} gets (?P<p>[+-]?\d+)/(?P<t>[+-]?\d+) until end of turn(?:\.|$)"
)
def _subj_gets_eot(m):
    p = int(m.group("p"))
    t = int(m.group("t"))
    return Buff(power=p, toughness=t, target=_subject_filter(m.group("subject")))


# "<subject> gains <keyword> until end of turn"
@_er(
    rf"^{_SUBJ} gains (?P<kw>[a-z, ]+?) until end of turn(?:\.|$)"
)
def _subj_gains_eot(m):
    return GrantAbility(
        ability_name=m.group("kw").strip(),
        target=_subject_filter(m.group("subject")),
        duration="until_end_of_turn",
    )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def apply_extensions(parser_module) -> None:
    """Patch a ``parser`` module in place: append EFFECT_RULES and wrap
    ``parse_static`` so STATIC_PATTERNS fire before the original fallthrough.

    Intended to be called from a test harness or an opt-in runner — this
    extension module does *not* monkey-patch on import.
    """
    parser_module.EFFECT_RULES.extend(EFFECT_RULES)
    original_parse_static = parser_module.parse_static

    def patched_parse_static(text: str):
        # Preserve original-case `raw`, but match against the lowercased/
        # cleaned form the main parser already produced.
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


__all__ = [
    "STATIC_PATTERNS",
    "EFFECT_RULES",
    "apply_extensions",
]
