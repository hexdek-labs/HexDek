#!/usr/bin/env python3
"""TOKEN SPECIFICATIONS — structured capture of verbose token-creation phrasings.

Family: CREATE-TOKEN grammar. The base parser has exactly one token rule,
    r"^create (a|one|two|three|x|\\d+) (\\d+)/(\\d+) ([a-z ]+?) creature tokens?(?:\\.|$)"
which chokes on almost every real card because real tokens carry color
adjectives, multi-color expressions, extra card-types (artifact creature,
enchantment creature), tapped/attacking/legendary modifiers, granted
keywords ("with flying and lifelink"), and in many cases a granted ability
quoted inside parentheses or double-quotes. This extension produces a
cascade of specific-first ``EFFECT_RULES`` that each feed a fully-populated
``CreateToken`` AST node.

Exported:
- ``EFFECT_RULES`` — ``(compiled_regex, builder)`` entries. Merged by
  ``parser.load_extensions`` into the global dispatch table. Ordering is
  specific-to-general so the richer shapes win over degenerate ones.

Phrasings covered (each row is a real card sample seen in the work queue):

  Pt + color(s) + subtype(s) + [artifact|enchantment] creature [with KW(s)]:
    * "create a 4/4 white Angel creature token with flying"                   (Empyrial Storm)
    * "create three 1/1 white Spirit creature tokens with flying"             (Lingering Souls variants)
    * "create two 2/2 White Knight creature tokens with vigilance"            (Assure // Assemble core)
    * "create a 2/2 White Soldier creature token with vigilance and lifelink"
    * "create a 4/4 White and Blue Sphinx creature token with flying and vigilance"
    * "create three 2/2 green and white Elf Knight creature tokens with vigilance"
    * "create four 1/1 colorless Soldier artifact creature tokens"            (Mass Production)
    * "create two 2/2 blue Bird enchantment creature tokens with flying"
    * "create a 1/1 colorless Thopter artifact creature token with flying"
  Tapped / attacking riders:
    * "create a tapped 4/1 black Skeleton creature token with menace"
    * "create a tapped, attacking 4/4 red Demon creature token with flying and trample"
    * "create X tapped 2/2 black Zombie creature tokens"
  Granted ability in quotes (the high-value case — preserves the inner text):
    * "create a 1/1 colorless Eldrazi Spawn creature token with \"Sacrifice this creature: Add {C}.\""
    * "create a 0/1 black Wizard creature token with \"Whenever you cast a noncreature spell, ...\""
    * "create a 2/2 green Bird creature token with \"Whenever a land you control enters, ...\""
  Copy-of variants:
    * "create a token that's a copy of that creature"                         (Nemesis Trap et al.)
    * "create a token that's a copy of this creature"
    * "create a token that's a copy of target [filter]"                       (already handled; we broaden)
  Named/artifact-type tokens (non-creature utility tokens):
    * "create a Treasure token"                                                (very common)
    * "create a Food token"
    * "create N {Treasure|Clue|Blood|Food|Map|Gold|Powerstone|Incubator|Shard|Role|Lander|Junk|Walker} tokens"
  "X ... where X is ...":
    * "create X 1/1 white Soldier creature tokens, where X is the number of creatures on the battlefield"
    * "create X 2/2 black Zombie Berserker creature tokens, where X is ..."

Fields populated on ``CreateToken``:
  - ``count``       — int or "x" (or left as the numeric word string for oddities)
  - ``pt``          — ``(power, toughness)`` tuple or ``None`` for non-creature tokens
  - ``types``       — ordered tuple of card types + subtypes as found
                      (e.g. ("Angel",) or ("Thopter", "artifact") or ("Treasure",))
  - ``color``       — tuple of colors ("W","U","B","R","G","C") from the adjective run
  - ``keywords``    — tuple of granted keyword strings; for quoted abilities the
                      full quoted text is stored as a single ``'ability:"<text>"'``
                      entry so downstream can recognise/parse it later
  - ``is_copy_of``  — a ``Filter`` for copy-of targets (broadens the existing rule)
  - ``legendary``   — bool
  - ``tapped``      — bool (also set for "tapped and attacking")

Ordering rule: regex that capture more fields appear first. The parser's
dispatch uses ``pat.match`` with ``m.end() >= len(text) - 2``, so rules that
intentionally leave trailing clauses unconsumed will be rejected by that
whole-text guard and fall through to the next rule. Each rule here is
designed to consume the entire clause.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import CreateToken, Filter  # noqa: E402


# ---------------------------------------------------------------------------
# Lexical helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

_COLOR_MAP = {
    "white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
    "colorless": "C",
}

# The keyword grantable inline on a token-with-KW(s) clause. Not exhaustive,
# but covers everything that appears on real printed tokens.
_GRANTED_KEYWORDS = (
    "flying", "first strike", "double strike", "deathtouch", "lifelink",
    "menace", "reach", "trample", "vigilance", "haste", "hexproof", "shroud",
    "indestructible", "defender", "flash", "protection",
    # Ability-word / rider keywords that behave like simple keywords on tokens
    "decayed", "ward", "banding", "skulk", "prowess", "horsemanship",
    "intimidate", "fear", "persist", "undying", "exalted", "toxic",
)

# Subtypes that commonly appear as the creature type of a token. We don't
# enumerate the full list — we accept any Capitalized-ish word run so that
# new sets don't break us. The regex fragment below captures one-or-more
# Title-cased or single-lowercase-word tokens before "creature token".
_TYPE_RUN = r"(?:[A-Za-z][\w-]*(?:\s+[A-Za-z][\w-]*){0,3}?)"

# Non-creature utility-token names. Real oracle text: "Treasure token",
# "Food token", "Clue token", "Blood token", "Gold token", "Powerstone token",
# "Map token", "Incubator token", "Shard token", "Junk token", "Walker token",
# "Role token" (with subtype), "Lander token", "Mutagen token".
_UTILITY_TOKEN_NAMES = (
    "Treasure", "Food", "Clue", "Blood", "Gold", "Powerstone", "Map",
    "Incubator", "Shard", "Junk", "Walker", "Lander", "Mutagen",
)


def _num(tok: str):
    """Convert a number word/digit string to int or 'x'."""
    tok = tok.lower().strip()
    if tok in _NUM_WORDS:
        v = _NUM_WORDS[tok]
        return v
    if tok.isdigit():
        return int(tok)
    return tok  # let the caller keep whatever unusual literal it found


def _parse_colors(chunk: str) -> tuple[str, ...]:
    """Parse a color-adjective run like "white", "white and blue",
    "red, white and blue", "green and white", "colorless"."""
    if not chunk:
        return ()
    chunk = chunk.lower().replace(",", " ")
    words = [w for w in re.split(r"\s+(?:and\s+)?", chunk) if w]
    out: list[str] = []
    for w in words:
        if w in _COLOR_MAP:
            out.append(_COLOR_MAP[w])
    return tuple(out)


def _parse_keywords_run(chunk: str) -> tuple[str, ...]:
    """Parse a run like "flying", "flying and lifelink",
    "flying, first strike, and haste", "flying and vigilance"."""
    if not chunk:
        return ()
    chunk = chunk.strip().rstrip(".").lower()
    chunk = re.sub(r",?\s+and\s+", ", ", chunk)
    parts = [p.strip() for p in chunk.split(",") if p.strip()]
    kws: list[str] = []
    for p in parts:
        # Allow "ward {2}" or "protection from red" as a single token.
        if p in _GRANTED_KEYWORDS or any(p.startswith(k + " ") for k in _GRANTED_KEYWORDS):
            kws.append(p)
    return tuple(kws)


# Matches either a simple keyword run OR a quoted granted ability. The
# quoted form is the high-value capture — we stash the whole text verbatim
# as a single keywords entry "ability:\"<inner>\"" so it survives round-trip.
# Regex components shared across builders:
_COLOR_RUN = r"(?:colorless|(?:white|blue|black|red|green)(?:(?:,\s*|\s+(?:and|or)\s+)(?:white|blue|black|red|green))*)"
_PT = r"(\d+|x)/(\d+|x)"
_COUNT = r"(a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)"


# ---------------------------------------------------------------------------
# EFFECT_RULES — specific-first
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# 1) Token with a quoted granted ability.
#    "create a 1/1 colorless Eldrazi Spawn creature token with
#     "Sacrifice this creature: Add {C}.""
#    We deliberately anchor the trailing quote + optional period so we
#    consume the entire clause. The quoted ability goes into keywords as
#    a single entry preserving the full text.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped(?:,\s*attacking)?|attacking)\s+)?"
    rf"{_PT}\s+({_COLOR_RUN})\s+([A-Za-z][\w\s-]*?)\s+"
    r"(artifact\s+creature|enchantment\s+creature|creature)\s+tokens?\s+"
    r"with\s+\"(.+?)\"\.?$"
)
def _token_with_quoted_ability(m):
    count = _num(m.group(1))
    tapped_run = (m.group(2) or "").lower()
    pt = (_num(m.group(3)), _num(m.group(4)))
    colors = _parse_colors(m.group(5))
    subtypes = tuple(s for s in m.group(6).split() if s)
    card_type = m.group(7).lower().strip()
    quoted = m.group(8).strip()
    types: tuple[str, ...] = subtypes
    if card_type != "creature":
        # "artifact creature" / "enchantment creature" — keep the extra type
        types = subtypes + (card_type.split()[0],)
    return CreateToken(
        count=count,
        pt=pt,
        types=types,
        color=colors,
        keywords=(f'ability:"{quoted}"',),
        tapped=("tapped" in tapped_run),
    )


# ---------------------------------------------------------------------------
# 2) Token with quoted ability but no color word (rare — e.g. "create a
#    0/1 Saproling creature token with \"...\"") — same shape minus color.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped(?:,\s*attacking)?|attacking)\s+)?"
    rf"{_PT}\s+([A-Za-z][\w\s-]*?)\s+"
    r"(artifact\s+creature|enchantment\s+creature|creature)\s+tokens?\s+"
    r"with\s+\"(.+?)\"\.?$"
)
def _token_with_quoted_ability_nocolor(m):
    count = _num(m.group(1))
    tapped_run = (m.group(2) or "").lower()
    pt = (_num(m.group(3)), _num(m.group(4)))
    subtypes = tuple(s for s in m.group(5).split() if s)
    card_type = m.group(6).lower().strip()
    quoted = m.group(7).strip()
    types = subtypes if card_type == "creature" else subtypes + (card_type.split()[0],)
    return CreateToken(
        count=count,
        pt=pt,
        types=types,
        color=(),
        keywords=(f'ability:"{quoted}"',),
        tapped=("tapped" in tapped_run),
    )


# ---------------------------------------------------------------------------
# 3) Pt + color(s) + subtype(s) + [artifact|enchantment] creature
#    token(s) with <KW(s)>. This is the workhorse rule and catches most
#    partials seen in the work queue.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped(?:,\s*attacking)?|attacking|legendary)\s+)?"
    rf"{_PT}\s+({_COLOR_RUN})\s+([A-Za-z][\w\s-]*?)\s+"
    r"(artifact\s+creature|enchantment\s+creature|creature)\s+tokens?"
    r"\s+with\s+([a-z][a-z\s,]*?)(?:\.|$)"
)
def _token_full_with_kw(m):
    count = _num(m.group(1))
    mod_run = (m.group(2) or "").lower()
    pt = (_num(m.group(3)), _num(m.group(4)))
    colors = _parse_colors(m.group(5))
    subtypes = tuple(s for s in m.group(6).split() if s)
    card_type = m.group(7).lower().strip()
    kws = _parse_keywords_run(m.group(8))
    if not kws:
        # The trailing run didn't parse as keywords — bail so a later rule
        # (or the core fallback) gets a chance.
        raise ValueError("keyword run did not resolve")
    types = subtypes if card_type == "creature" else subtypes + (card_type.split()[0],)
    return CreateToken(
        count=count,
        pt=pt,
        types=types,
        color=colors,
        keywords=kws,
        tapped=("tapped" in mod_run),
        legendary=("legendary" in mod_run),
    )


# ---------------------------------------------------------------------------
# 4) Same as (3) but no "with <KW>" tail.
#    "create four 1/1 colorless Soldier artifact creature tokens."
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped(?:,\s*attacking)?|attacking|legendary)\s+)?"
    rf"{_PT}\s+({_COLOR_RUN})\s+([A-Za-z][\w\s-]*?)\s+"
    r"(artifact\s+creature|enchantment\s+creature|creature)\s+tokens?(?:\.|$)"
)
def _token_full_no_kw(m):
    count = _num(m.group(1))
    mod_run = (m.group(2) or "").lower()
    pt = (_num(m.group(3)), _num(m.group(4)))
    colors = _parse_colors(m.group(5))
    subtypes = tuple(s for s in m.group(6).split() if s)
    card_type = m.group(7).lower().strip()
    types = subtypes if card_type == "creature" else subtypes + (card_type.split()[0],)
    return CreateToken(
        count=count,
        pt=pt,
        types=types,
        color=colors,
        keywords=(),
        tapped=("tapped" in mod_run),
        legendary=("legendary" in mod_run),
    )


# ---------------------------------------------------------------------------
# 5) Pt + subtype(s) with no color adjective (some sets / ~ self-name
#    templating) — e.g. "create a 3/3 green ~ creature token" is caught
#    by (4), but "create a 1/1 white ~ creature token with flying" falls
#    through when the subtype is ~. We handle the ~ and colorless-less
#    case as a generic fallback on color.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped(?:,\s*attacking)?|attacking|legendary)\s+)?"
    rf"{_PT}\s+([A-Za-z~][\w\s-]*?)\s+creature\s+tokens?"
    r"\s+with\s+([a-z][a-z\s,]*?)(?:\.|$)"
)
def _token_nocolor_with_kw(m):
    count = _num(m.group(1))
    mod_run = (m.group(2) or "").lower()
    pt = (_num(m.group(3)), _num(m.group(4)))
    subtypes = tuple(s for s in m.group(5).split() if s)
    kws = _parse_keywords_run(m.group(6))
    if not kws:
        raise ValueError("keyword run did not resolve")
    return CreateToken(
        count=count,
        pt=pt,
        types=subtypes,
        color=(),
        keywords=kws,
        tapped=("tapped" in mod_run),
        legendary=("legendary" in mod_run),
    )


# ---------------------------------------------------------------------------
# 6) Copy-of variants, broadened from the parser's single "target" form.
#    Captures: "a token that's a copy of that creature / this creature /
#    that card / it" and optional "except ..." rider (not parsed, but
#    preserved as a free-form note in keywords).
# ---------------------------------------------------------------------------
@_eff(
    r"^create\s+(a|one|two|three|x|\d+)\s+tokens?\s+that(?:'s| is| are)\s+(?:a\s+)?cop(?:y|ies)"
    r"\s+of\s+(that creature|this creature|that card|that permanent|that token|it|them)"
    r"(?:,?\s+except\s+(.+?))?(?:\.|$)"
)
def _token_copy_of_pronoun(m):
    count = _num(m.group(1))
    pronoun = m.group(2).lower()
    except_clause = (m.group(3) or "").strip()
    # Model the copy target as a Filter with the pronoun as base (non-target).
    tgt = Filter(base=pronoun.replace(" ", "_"), targeted=False)
    kws: tuple[str, ...] = ()
    if except_clause:
        kws = (f"except:{except_clause}",)
    return CreateToken(count=count, is_copy_of=tgt, keywords=kws)


# ---------------------------------------------------------------------------
# 7) Utility artifact tokens: "create N {Treasure|Food|Clue|Blood|Gold|
#    Powerstone|Map|Incubator|Shard|Junk|Walker|Lander|Mutagen} token(s)."
#    These have no P/T and no color; the named token type carries all the
#    rules text implicitly (game rule 113.5a / named tokens).
# ---------------------------------------------------------------------------
_UTIL_RE = "|".join(_UTILITY_TOKEN_NAMES)


@_eff(
    rf"^create\s+{_COUNT}\s+(?:(tapped)\s+)?({_UTIL_RE})\s+tokens?(?:\.|$)"
)
def _token_utility(m):
    count = _num(m.group(1))
    tapped = bool(m.group(2))
    name = m.group(3).capitalize()
    return CreateToken(
        count=count,
        pt=None,
        types=(name,),
        color=(),
        keywords=(),
        tapped=tapped,
    )


# ---------------------------------------------------------------------------
# 8) Role tokens (Wilds of Eldraine). Shape:
#    "create a {Name} Role token attached to {target filter}"
#    Roles are enchantments that aura-attach on creation. We capture the
#    role subtype and stash the attach-target in keywords for downstream.
# ---------------------------------------------------------------------------
@_eff(
    r"^create\s+(a|an|one)\s+([a-z][a-z\s-]*?)\s+role\s+token\s+"
    r"attached\s+to\s+(.+?)(?:\.|$)"
)
def _token_role(m):
    role_kind = m.group(2).strip().lower()
    attach = m.group(3).strip().lower()
    return CreateToken(
        count=1,
        pt=None,
        types=(role_kind.title(), "Role"),
        color=(),
        keywords=(f"attached_to:{attach}",),
    )


# ---------------------------------------------------------------------------
# 9) X-count with "where X is ..." rider. Example:
#    "create X 1/1 white Soldier creature tokens, where X is the number
#     of creatures on the battlefield."
#    We consume the whole clause and stash the X-definition into keywords
#    as a free-form "x:=<expr>" entry so the token spec round-trips.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+x\s+{_PT}\s+({_COLOR_RUN})\s+([A-Za-z][\w\s-]*?)\s+"
    r"(artifact\s+creature|enchantment\s+creature|creature)\s+tokens"
    r"(?:\s+with\s+([a-z][a-z\s,]*?))?"
    r",\s+where\s+x\s+is\s+(.+?)(?:\.|$)"
)
def _token_x_where(m):
    pt = (_num(m.group(1)), _num(m.group(2)))
    colors = _parse_colors(m.group(3))
    subtypes = tuple(s for s in m.group(4).split() if s)
    card_type = m.group(5).lower().strip()
    kw_run = m.group(6)
    x_def = m.group(7).strip()
    types = subtypes if card_type == "creature" else subtypes + (card_type.split()[0],)
    kws: list[str] = list(_parse_keywords_run(kw_run)) if kw_run else []
    kws.append(f"x:={x_def}")
    return CreateToken(
        count="x",
        pt=pt,
        types=types,
        color=colors,
        keywords=tuple(kws),
    )


# ---------------------------------------------------------------------------
# 10) X-count utility token with "where X is ..." — same pattern for
#     Treasure / Map / Food etc.
# ---------------------------------------------------------------------------
@_eff(
    rf"^create\s+x\s+({_UTIL_RE})\s+tokens"
    r",\s+where\s+x\s+is\s+(.+?)(?:\.|$)"
)
def _token_utility_x_where(m):
    name = m.group(1).capitalize()
    x_def = m.group(2).strip()
    return CreateToken(
        count="x",
        pt=None,
        types=(name,),
        color=(),
        keywords=(f"x:={x_def}",),
    )


__all__ = ["EFFECT_RULES"]


# ---------------------------------------------------------------------------
# Smoke test — run `python3 extensions/token_specs.py` to verify every
# rule matches its motivating exemplar and every builder returns a
# populated CreateToken.
# ---------------------------------------------------------------------------
if __name__ == "__main__":  # pragma: no cover
    samples = [
        "create a 2/2 white soldier creature token with vigilance and lifelink",
        "create a 4/4 white angel creature token with flying",
        "create three 1/1 white spirit creature tokens with flying",
        "create two 2/2 white knight creature tokens with vigilance",
        "create four 1/1 colorless soldier artifact creature tokens",
        "create a 1/1 colorless thopter artifact creature token with flying",
        "create two 2/2 blue bird enchantment creature tokens with flying",
        "create a 4/4 white and blue sphinx creature token with flying and vigilance",
        "create three 2/2 green and white elf knight creature tokens with vigilance",
        "create a tapped 4/1 black skeleton creature token with menace",
        "create a 2/2 black zombie creature token with decayed",
        'create a 0/1 black wizard creature token with "whenever you cast a noncreature spell, this token deals 1 damage to each opponent."',
        'create a 1/1 colorless eldrazi spawn creature token with "sacrifice this creature: add {c}."',
        "create a token that's a copy of that creature",
        "create a token that's a copy of this creature",
        "create a treasure token",
        "create four treasure tokens",
        "create a food token",
        "create x 1/1 white soldier creature tokens, where x is the number of creatures on the battlefield",
        "create x 2/2 black zombie berserker creature tokens, where x is the number of nontoken creatures that died this turn",
        "create a wicked role token attached to target creature you control",
    ]
    n_hit = 0
    n_miss = 0
    for s in samples:
        matched = None
        for pat, builder in EFFECT_RULES:
            m = pat.match(s)
            if m and m.end() >= len(s) - 2:
                try:
                    tok = builder(m)
                    matched = (pat.pattern[:60], tok)
                    break
                except Exception:
                    continue
        if matched:
            n_hit += 1
            pat_head, tok = matched
            print(f"OK  | {s[:80]!r}")
            print(f"    -> {tok}")
        else:
            n_miss += 1
            print(f"MISS| {s[:80]!r}")
    print(f"\ntoken_specs: {n_hit}/{len(samples)} samples matched "
          f"({len(EFFECT_RULES)} rules).")
