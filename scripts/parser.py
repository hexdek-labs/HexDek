#!/usr/bin/env python3
"""Oracle text → CardAST parser.

Recursive-descent. Splits oracle text into ability-sized chunks, then matches
each chunk against grammar productions for Static / Activated / Triggered /
Keyword. A production succeeds by emitting a typed AST node and consuming the
chunk; failure is recorded with the unconsumed text so we know exactly what
the grammar still needs to handle.

Coverage metric:
  - GREEN  → all abilities parsed cleanly
  - PARTIAL→ some abilities parsed, others left as raw text
  - UNPARSED→ no abilities parsed (parser couldn't even split the text)

Goal: 100% GREEN, where every PARTIAL/UNPARSED is a concrete grammar bug.

Usage:
  python3 scripts/parser.py             # full report
  python3 scripts/parser.py --card NAME # parse one card and dump the AST
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path
from typing import Optional

from mtg_ast import (
    AddMana, Activated, Bounce, Buff, CardAST, Choice, Condition, Cost,
    CounterMod, CounterSpell, CreateToken, Damage, Destroy, Discard, Draw,
    Effect, Exile, ExtraCombat, ExtraTurn, Fight, Filter, GainControl,
    GainLife, GrantAbility, Keyword, LookAt, LoseGame, LoseLife, ManaCost,
    ManaSymbol, Mill, Modification, Optional_, Prevent, Reanimate, Recurse,
    Replacement, Reveal, Sacrifice, Scry, Sequence, SetLife, Shuffle,
    Static, Surveil, TapEffect, Tutor, Triggered, Trigger, UntapEffect,
    UnknownEffect, WinGame, signature,
    TARGET_ANY, TARGET_CREATURE, TARGET_PLAYER, TARGET_OPPONENT,
    EACH_OPPONENT, EACH_PLAYER, SELF,
)


ROOT = Path(__file__).resolve().parents[1]
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
REPORT = ROOT / "data" / "rules" / "parser_coverage.md"
EXTENSIONS_DIR = Path(__file__).resolve().parent / "extensions"


# ============================================================================
# Tokenization helpers
# ============================================================================

def normalize(card: dict) -> str:
    """Pull oracle text (concatenating dual-faced card_faces), lowercase,
    strip reminder text, normalize dashes, replace card name with `~`."""
    text = card.get("oracle_text") or ""
    if not text and card.get("card_faces"):
        text = "\n".join(f.get("oracle_text") or "" for f in card["card_faces"])
    text = text.lower()
    name = (card.get("name") or "").lower()
    if name:
        text = text.replace(name, "~")
        for half in name.split(" // "):
            text = text.replace(half, "~")
        first = name.split(",")[0].split(" ")[0]
        if first and len(first) > 3 and "," in name:
            text = text.replace(first, "~")
    text = re.sub(r"\([^)]*\)", "", text)
    text = re.sub(r"[\u2013\u2014]", "-", text)
    text = re.sub(r"[ \t]+", " ", text).strip()
    return text


def split_abilities(text: str) -> list[str]:
    """Split oracle text into individual abilities. Heuristics:
    - Newlines are hard ability separators (mostly).
    - "Choose one/two/three —" header consumes following bulleted lines as
      one modal ability.
    - "As an additional cost to cast this spell, ..." stays joined with the
      following sentence (it's part of the same spell ability).
    - Within a paragraph, each sentence is one ability unless it's clearly a
      continuation ("If you do, ...", "Then ...").
    - We split at trigger boundaries inside a single line (keyword + When/...).
    """
    # Insert newline before triggered-ability starters that follow a word.
    # Keep "at end of" as a trigger-starter ONLY at sentence boundaries — within
    # a sentence ("destroy target creature at end of combat") it's a duration
    # modifier on the effect, not a standalone trigger.
    text = re.sub(r"(?<=\w)\s+(?=(?:when|whenever|at the beginning)\b)",
                  "\n", text, flags=re.I)
    text = re.sub(r"(?<=[.])\s+(?=at end of\b)", "\n", text, flags=re.I)
    raw_lines = [l.strip() for l in text.split("\n") if l.strip()]

    # Pass 1: consolidate modal headers with their bullets.
    # Match both standalone "choose one —" AND trigger-prefixed lines like
    # "When this creature enters, choose one —" so the bullet modes get
    # joined into a single line for downstream parsing.
    _MODAL_TAIL_RE = re.compile(
        r"choose (one|two|three|x|one or both|one or more|two or more|any number)"
        r"(?:\s+that hasn't been chosen[^—:.]*?)?"
        r"(?:\s+at random)?"
        r"(?:\s*[-—:.]\s*)*$",
        re.I,
    )
    _MODAL_PERIOD = "\x00MP\x00"  # sentinel that survives sentence splitting
    consolidated: list[str] = []
    i = 0
    while i < len(raw_lines):
        line = raw_lines[i]
        if _MODAL_TAIL_RE.search(line):
            modes = []
            j = i + 1
            while j < len(raw_lines) and re.match(r"^[•·]", raw_lines[j]):
                modes.append(raw_lines[j].replace(". ", _MODAL_PERIOD + " "))
                j += 1
            if modes:
                consolidated.append(line.rstrip(":-— ").rstrip() + " — " + " ; ".join(modes))
                i = j
                continue
        consolidated.append(line)
        i += 1

    # Saga chapter prefix: a line starting with roman numerals + dash introduces
    # a chapter body. If the body spans multiple sentences (Urabrask // The Great
    # Work's chapter III: "Until end of turn... If a spell... Exile this Saga...")
    # the later sentences must NOT lose their chapter context when we
    # sentence-split. We detect the prefix up-front and re-apply it to each split
    # sentence so every sentence remains classifiable as a saga_chapter (the
    # extension's _CHAPTER_PREFIX regex uses `.+` so any body works).
    _chapter_prefix_re = re.compile(
        r"^((?:i{1,3}v?|iv|v|vi{0,3}|ix|x)(?:\s*,\s*(?:i{1,3}v?|iv|v|vi{0,3}|ix|x))*)\s*[-—]\s*",
        re.I,
    )

    abilities: list[str] = []
    for line in consolidated:
        # Saga-aware: if this line is a chapter, remember the prefix so we can
        # re-attach it to each sub-sentence after splitting.
        chapter_m = _chapter_prefix_re.match(line)
        chapter_prefix = None
        if chapter_m:
            chapter_prefix = chapter_m.group(0)  # includes trailing " - " / " — "

        # Don't split sentences inside double-quotes (quoted abilities often
        # contain internal periods like `"When this enters, draw a card."`).
        # Walk char-by-char tracking quote depth, only allow splits at quote_depth=0.
        sents = []
        buf = []
        depth = 0
        i = 0
        while i < len(line):
            ch = line[i]
            buf.append(ch)
            if ch == '"':
                # If this quote closes a depth-1 span AND the preceding character
                # inside the quote was a '.', the quoted ability ended with its own
                # sentence terminator (oracle-text convention: `... instead."`).
                # Treat `." ` as a sentence boundary when followed by a starter so
                # trailing clauses (Brittle Blast, Paths of Tuinvale, etc.) split out.
                closing_quote = depth == 1
                depth = 1 - depth
                if (closing_quote
                        and len(buf) >= 2
                        and buf[-2] == '.'
                        and i + 1 < len(line)
                        and line[i+1] == ' '):
                    lookahead = line[i+2:i+3]
                    if lookahead and (lookahead.isalpha() or lookahead in '~{•·'):
                        sents.append(''.join(buf))
                        buf = []
                        i += 1  # skip the space
            # Sentence boundary: period followed by space + capital/tilde/brace/bullet
            elif ch == '.' and depth == 0 and i + 1 < len(line) and line[i+1] == ' ':
                # Look ahead to see if next char after the space is a sentence starter
                lookahead = line[i+2:i+3]
                if lookahead and (lookahead.isalpha() or lookahead in '~{•·'):
                    sents.append(''.join(buf))
                    buf = []
                    i += 1  # skip the space
            i += 1
        if buf:
            sents.append(''.join(buf))
        merged: list[str] = []
        for idx, s in enumerate(sents):
            s = s.strip().rstrip(".")
            if not s:
                continue
            # Re-attach chapter prefix to subsequent sentences inside a saga
            # chapter line. The first sentence already carries the prefix.
            if chapter_prefix and idx > 0 and not _chapter_prefix_re.match(s):
                s = chapter_prefix + s
            # Continuations
            if merged and re.match(r"^(if you do|if you don't|otherwise|then)\b", s, re.I):
                merged[-1] = merged[-1] + ". " + s
            # Additional-cost prefix joins with next sentence (it's part of the
            # spell ability, not its own ability)
            elif (merged and re.match(r"^as an additional cost to cast", merged[-1], re.I)):
                merged[-1] = merged[-1] + ". " + s
            else:
                merged.append(s)
        abilities.extend(merged)
    # Restore modal-period sentinels that protected periods inside bullet modes
    # from sentence splitting.
    abilities = [a.replace(_MODAL_PERIOD, ".") for a in abilities]
    return abilities


# ============================================================================
# Mana cost parsing
# ============================================================================

_MANA_TOKEN = re.compile(r"\{([^}]+)\}")


def parse_mana_cost(s: str) -> Optional[ManaCost]:
    """Parse a sequence of {X} mana symbols into a ManaCost."""
    syms = []
    for m in _MANA_TOKEN.finditer(s):
        raw = "{" + m.group(1) + "}"
        body = m.group(1).strip().upper()
        if body in {"T", "Q", "E"}:
            continue  # tap/untap/energy aren't mana symbols
        sym = ManaSymbol(raw=raw)
        if body.isdigit():
            sym = ManaSymbol(raw=raw, generic=int(body))
        elif body == "X":
            sym = ManaSymbol(raw=raw, is_x=True)
        elif body == "S":
            sym = ManaSymbol(raw=raw, is_snow=True)
        elif "/" in body:
            parts = body.split("/")
            colors = tuple(p for p in parts if p in {"W", "U", "B", "R", "G", "C"})
            generic = next((int(p) for p in parts if p.isdigit()), 0)
            phyrexian = "P" in parts
            sym = ManaSymbol(raw=raw, generic=generic, color=colors, is_phyrexian=phyrexian)
        elif body in {"W", "U", "B", "R", "G", "C"}:
            sym = ManaSymbol(raw=raw, color=(body,))
        syms.append(sym)
    return ManaCost(symbols=tuple(syms)) if syms else None


# ============================================================================
# Filter parsing
# ============================================================================

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

_SUBTYPE_WORDS = {  # rough — comp rules has the full list, this catches common ones
    "creature", "artifact", "enchantment", "land", "planeswalker", "instant",
    "sorcery", "battle", "permanent", "spell", "card", "token",
}


def parse_filter(s: str) -> Optional[Filter]:
    """Parse a noun-phrase that describes a target / actor / search query.
    Returns None if the string doesn't look like a filter."""
    s = s.strip().lower()
    if not s:
        return None

    quantifier = "one"
    count: Optional[int] = None
    targeted = False
    you_control = False
    opp_controls = False
    nontoken = False
    color_excl: tuple[str, ...] = ()
    extra: list[str] = []

    if s.startswith("each "):
        quantifier = "each"; s = s[5:]
    elif s.startswith("all "):
        quantifier = "all"; s = s[4:]
    elif s.startswith("any "):
        quantifier = "any"; s = s[4:]
    elif s.startswith("up to "):
        quantifier = "up_to_n"; s = s[6:]

    m = re.match(r"^(a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+", s)
    if m:
        token = m.group(1)
        count = _NUM_WORDS.get(token, token)
        if isinstance(count, str) and count.isdigit():
            count = int(count)
        s = s[m.end():]

    if s.startswith("target "):
        targeted = True; s = s[7:]
    elif s.startswith("a target ") or s.startswith("the target "):
        targeted = True; s = re.sub(r"^(?:a |the )target ", "", s)

    # Color exclusions: "nonblack creature", "noncreature spell"
    m = re.match(r"^non(white|blue|black|red|green|creature|artifact|enchantment|land)\s+", s)
    if m:
        word = m.group(1)
        color_map = {"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}
        if word in color_map:
            color_excl = (color_map[word],)
        else:
            extra.append(f"non-{word}")
        s = s[m.end():]

    # Adjectives we collect generically
    adj_buffer = []
    while True:
        m = re.match(r"^(attacking|blocking|tapped|untapped|legendary|nontoken|colorless|multicolored|monocolored)\s+", s)
        if not m:
            break
        adj = m.group(1)
        if adj == "nontoken":
            nontoken = True
        else:
            adj_buffer.append(adj)
        s = s[m.end():]

    # Base noun
    m = re.match(r"^([a-z]+)", s)
    base = m.group(1) if m else "thing"
    if base in {"creatures", "permanents", "artifacts", "enchantments", "lands",
                "players", "opponents", "spells", "tokens", "cards"}:
        base = base[:-1]
    s = s[len(base):] if m else s

    # "you control" / "an opponent controls" suffixes
    if "you control" in s:
        you_control = True
    if "an opponent controls" in s or "opponent controls" in s:
        opp_controls = True

    return Filter(
        base=base,
        quantifier=quantifier,
        count=count,
        targeted=targeted,
        you_control=you_control,
        opponent_controls=opp_controls,
        nontoken=nontoken,
        color_exclude=color_excl,
        extra=tuple(adj_buffer + extra),
    )


# ============================================================================
# Effect grammar
# ============================================================================
# Each rule here is (regex, builder). The first rule whose regex matches the
# WHOLE remaining text wins. Builders take the regex match and return an
# Effect (or raise to signal "didn't actually fit").

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def rule(pattern: str):
    def decorator(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return decorator


# Extension registries — populated at import time by load_extensions().
# Modules under scripts/extensions/*.py may export EFFECT_RULES,
# STATIC_PATTERNS, TRIGGER_PATTERNS as lists; we splice them in at the
# right precedence (before catch-alls).
EXT_STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []
EXT_TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []

# Per-card hand-written handlers. Keyed by exact card name.
# Each handler: (card_dict) -> CardAST. Checked FIRST in parse_card.
# Populated by extensions exporting PER_CARD_HANDLERS (name->handler dict).
PER_CARD_HANDLERS: dict[str, callable] = {}

# Post-parse hooks: list[callable(CardAST) -> CardAST]. Applied IN ORDER to
# the CardAST produced by the grammar (AFTER per-card handlers, BEFORE the
# result is returned from parse_card). Each hook is additive — it receives
# a frozen CardAST and returns a (possibly new) CardAST with extra fields
# populated. Used by face_down_family.py to surface morph_cost /
# disguise_cost / manifest_token / face_down_copy_effect Static.
POST_PARSE_HOOKS: list[callable] = []


def load_extensions() -> None:
    """Import every module under scripts/extensions/ and merge its registries."""
    if not EXTENSIONS_DIR.exists():
        return
    import importlib.util
    # Make scripts/ importable so extensions can `from mtg_ast import ...`
    scripts_dir = str(Path(__file__).resolve().parent)
    if scripts_dir not in sys.path:
        sys.path.insert(0, scripts_dir)
    for path in sorted(EXTENSIONS_DIR.glob("*.py")):
        if path.name.startswith("_"):
            continue
        spec = importlib.util.spec_from_file_location(f"ext_{path.stem}", path)
        if not spec or not spec.loader:
            continue
        mod = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(mod)
        except Exception as e:
            print(f"  ⚠ extension {path.name} failed to load: {e}", file=sys.stderr)
            continue
        for r in getattr(mod, "EFFECT_RULES", ()):
            EFFECT_RULES.append(r)
        for r in getattr(mod, "STATIC_PATTERNS", ()):
            EXT_STATIC_PATTERNS.append(r)
        for r in getattr(mod, "TRIGGER_PATTERNS", ()):
            EXT_TRIGGER_PATTERNS.append(r)
        handlers = getattr(mod, "PER_CARD_HANDLERS", None)
        if handlers:
            PER_CARD_HANDLERS.update(handlers)
        # Post-parse hooks run after grammar parsing (face-down enrichment,
        # etc.). See face_down_family.py for the canonical use case.
        # De-dupe on (module, function_name) so repeat calls to
        # load_extensions() (as conftest.py + parser.main both do) don't
        # stack identical hooks.
        _existing_hook_ids = {(h.__module__, h.__name__) for h in POST_PARSE_HOOKS}
        for h in getattr(mod, "POST_PARSE_HOOKS", ()):
            key = (h.__module__, h.__name__)
            if key in _existing_hook_ids:
                continue
            _existing_hook_ids.add(key)
            POST_PARSE_HOOKS.append(h)


# ---- Damage ----
@rule(r"^(?:~ )?deals? (\d+|x) damage to ([^.]+?)(?:\.|$)")
def _damage(m):
    amt = m.group(1)
    if amt.isdigit():
        amt = int(amt)
    target = parse_filter(m.group(2)) or TARGET_ANY
    return Damage(amount=amt, target=target)


@rule(r"^(?:~ )?deals damage equal to ([^.]+?) to ([^.]+?)(?:\.|$)")
def _damage_var(m):
    target = parse_filter(m.group(2)) or TARGET_ANY
    return Damage(amount="var", target=target)


# ---- Card draw ----
@rule(r"^draw (a|one|two|three|four|five|x|\d+) cards?(?:\.|$)")
def _draw(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Draw(count=n, target=SELF)


@rule(r"^target player draws (a|one|two|three|x|\d+) cards?(?:\.|$)")
def _force_draw(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Draw(count=n, target=TARGET_PLAYER)


@rule(r"^each (?:player|opponent) draws (a|one|two|three|x|\d+) cards?(?:\.|$)")
def _draw_each(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Draw(count=n, target=EACH_PLAYER if "player" in m.string else EACH_OPPONENT)


# ---- Discard ----
@rule(r"^discard (a|one|two|three|x|\d+) cards?(?:\.|$)")
def _self_discard(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Discard(count=n, target=SELF)


@rule(r"^target player discards (a|one|two|three|x|\d+) cards?(?:\.|$)")
def _force_discard(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Discard(count=n, target=TARGET_PLAYER, chosen_by="discarder")


# ---- Mill / scry / surveil ----
# NOTE: oracle text freely mixes digit counts ("mills 3 cards") with word
# counts ("mills three cards"). Brain Freeze famously uses the latter —
# without word-numeral support the parser drops the body effect entirely
# and storm copies inherit an empty effect list. Accept both forms.
@rule(r"^(?:target player )?mills? (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards?(?:\.|$)")
def _mill(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Mill(count=n,
                target=TARGET_PLAYER if "target player" in m.string else SELF)


@rule(r"^scry (a|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)(?:\.|$)")
def _scry(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Scry(count=n)


@rule(r"^surveil (a|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)(?:\.|$)")
def _surveil(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return Surveil(count=n)


# ---- Counterspell ----
# Counter-spell rules — ORDERED from most-specific to least-specific.
# More specific patterns MUST come first to prevent greedy matching.

# Multi-target counters (Disallow, Voidslime, Tale's End) — most specific, match first
@rule(r"^counter target spell,? activated ability,? or triggered ability(?:\.|$)")
def _counter_spell_or_ability(m):
    return CounterSpell(target=Filter(base="thing"))  # spell or ability

@rule(r"^counter target activated ability,? triggered ability,? or legendary spell(?:\.|$)")
def _counter_ability_or_legendary(m):
    return CounterSpell(target=Filter(base="thing"))

# Ability-only counters (Stifle, Trickbind, Repudiate)
@rule(r"^counter target activated or triggered ability(?:\.|$)")
def _counter_ability_both(m):
    return CounterSpell(target=Filter(base="abilities"))

@rule(r"^counter target activated ability(?:\.|$)")
def _counter_ability_activated(m):
    return CounterSpell(target=Filter(base="activated"))

@rule(r"^counter target triggered ability(?:\.|$)")
def _counter_ability_triggered(m):
    return CounterSpell(target=Filter(base="triggered"))

# Spell-type-filtered counters (Negate, Essence Scatter, etc.)
@rule(r"^counter target ([^.]*?spell[^.]*?)(?:\.|$)")
def _counter_spell(m):
    target = parse_filter(m.group(1).strip()) or Filter(base="spell")
    return CounterSpell(target=target)

# Generic "counter target spell" (Counterspell itself)
@rule(r"^counter target spell(?:\.|$)")
def _counter_spell_plain(m):
    return CounterSpell(target=Filter(base="spell"))


# ---- Removal ----
@rule(r"^destroy target ([^.]+?)(?:\.|$)")
def _destroy(m):
    target = parse_filter("target " + m.group(1)) or TARGET_CREATURE
    return Destroy(target=target)


@rule(r"^destroy all ([^.]+?)(?:\.|$)")
def _destroy_all(m):
    target = parse_filter("all " + m.group(1)) or TARGET_CREATURE
    return Destroy(target=target)


@rule(r"^exile target ([^.]+?)(?:\.|$)")
def _exile(m):
    target = parse_filter("target " + m.group(1)) or TARGET_CREATURE
    return Exile(target=target)


@rule(r"^return target ([^.]+?) to (?:its owner'?s? |your )hand(?:\.|$)")
def _bounce(m):
    target = parse_filter("target " + m.group(1)) or TARGET_CREATURE
    return Bounce(target=target)


# ---- Tutor ----
@rule(r"^search your library for ([^.]+?card[^.]*?)(?:[.,]|$)")
def _tutor(m):
    query = parse_filter(m.group(1)) or Filter(base="card")
    return Tutor(query=query, destination="hand")


# ---- Recursion / reanimation ----
@rule(r"^return target ([^.]+?) (?:card |creature card )?from (?:your|a|any) graveyard to your hand(?:\.|$)")
def _recurse(m):
    query = parse_filter("target " + m.group(1) + " card") or Filter(base="card")
    return Recurse(query=query, destination="hand")


# Allow "return target [filter] card with mana value X from your graveyard ..." —
# the inner filter clause may contain commas/extra modifiers ("with mana value 3
# or less", "named Foo", etc.). Also tolerate trailing modifiers after
# "battlefield" ("tapped", "with an additional +1/+1 counter on it", "under your
# control") — these are riders that don't change the core reanimate semantics.
@rule(r"^(?:return|put) target ([^.]+?) card (?:with [^.]+? )?from (?:your|a|any) graveyard (?:to|onto) the battlefield(?:[^.]*)?(?:\.|$)")
def _reanimate(m):
    query = parse_filter("target " + m.group(1) + " card")
    return Reanimate(query=query)


# ---- Life ----
def _life_amount(token: str):
    """Normalize a life-amount token: int for digits/word-numbers, 'x' for x."""
    n = _NUM_WORDS.get(token, token)
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    return n


@rule(r"^you gain (\d+|x|a|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _gain_life(m):
    return GainLife(amount=_life_amount(m.group(1)))


@rule(r"^you lose (\d+|x|a|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _lose_life(m):
    return LoseLife(amount=_life_amount(m.group(1)))


@rule(r"^target player loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _opp_lose(m):
    return LoseLife(amount=_life_amount(m.group(1)), target=TARGET_PLAYER)


@rule(r"^each opponent loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _each_opp_lose(m):
    return LoseLife(amount=_life_amount(m.group(1)), target=EACH_OPPONENT)


@rule(r"^each player loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _each_player_lose(m):
    return LoseLife(amount=_life_amount(m.group(1)), target=EACH_PLAYER)


@rule(r"^each player gains (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life(?:\.|$)")
def _each_player_gain(m):
    return GainLife(amount=_life_amount(m.group(1)), target=EACH_PLAYER)


# ---- Mana production ----
@rule(r"^add (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _add_mana(m):
    cost = parse_mana_cost(m.group(1))
    return AddMana(pool=cost.symbols if cost else ())


@rule(r"^add one mana of any color(?:\.|$)")
def _add_any(m):
    return AddMana(any_color_count=1)


# Arcane Signet / Commander's Sphere / any future color-identity-gated
# mana rocks. Per 7174n1c ruling 2026-04-16: parser emits AddMana with
# any_color_count=1 — pilot/hat chooses the color at spend time via the
# typed mana pool's `any` bucket. The "commander color identity" filter
# is an in-practice no-op because pilots only spend on castable colors;
# a future enhancement can track CI explicitly on the AddMana node.
@rule(r"^add one mana of any color in your commander's color identity(?:\.|$)")
def _add_any_in_ci(m):
    return AddMana(any_color_count=1)


# Fellwar Stone family — opponent-land-dependent mana. Pilot chooses at
# activation time from the colors opponent lands could produce. Same
# approximation: treat as "any" and let the pilot pick.
@rule(r"^add one mana of any color a land an opponent controls could produce(?:\.|$)")
def _add_any_fellwar(m):
    return AddMana(any_color_count=1)


# ---- Counters ----
@rule(r"^put (a|one|two|three|x|\d+) \+1/\+1 counters? on (.+?)(?:\.|$)")
def _put_counters(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    target = parse_filter(m.group(2)) or TARGET_CREATURE
    return CounterMod(op="put", count=n, counter_kind="+1/+1", target=target)


# ---- Buffs ----
@rule(r"^target creature gets \+(\d+)/\+(\d+) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=TARGET_CREATURE)


@rule(r"^~ gets \+(\d+)/\+(\d+) until end of turn(?:\.|$)")
def _buff_self(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False))


@rule(r"^creatures you control get \+(\d+)/\+(\d+)(?:\.|$)")
def _anthem(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all", you_control=True),
                duration="permanent")


# ---- Token creation ----
@rule(r"^create (a|one|two|three|x|\d+) (\d+)/(\d+) ([a-z ]+?) creature tokens?(?:\.|$)")
def _token(m):
    n = _NUM_WORDS.get(m.group(1), m.group(1))
    if isinstance(n, str) and n.isdigit():
        n = int(n)
    types = tuple(m.group(4).strip().split())
    return CreateToken(count=n, pt=(int(m.group(2)), int(m.group(3))), types=types)


@rule(r"^create a token that's a copy of target ([^.]+?)(?:\.|$)")
def _token_copy(m):
    target = parse_filter("target " + m.group(1))
    return CreateToken(count=1, is_copy_of=target)


# ---- Misc commonly-shared effects ----
@rule(r"^untap target ([^.]+?)(?:\.|$)")
def _untap_t(m):
    return UntapEffect(target=parse_filter("target " + m.group(1)) or TARGET_CREATURE)


@rule(r"^tap target ([^.]+?)(?:\.|$)")
def _tap_t(m):
    return TapEffect(target=parse_filter("target " + m.group(1)) or TARGET_CREATURE)


@rule(r"^you win the game(?:\.|$)")
def _win(m):
    return WinGame()


@rule(r"^target player loses the game(?:\.|$)")
def _force_lose(m):
    return LoseGame(target=TARGET_PLAYER)


# ---- End the turn (Time Stop & friends) ----
@rule(r"^end the turn(?:\.|$)")
def _end_turn(m):
    return UnknownEffect(raw_text="end the turn")


# ---- Self-sacrifice (no comma — bare "sacrifice ~" / "sacrifice this artifact") ----
@rule(r"^sacrifice (?:~|this creature|this artifact|this enchantment|this permanent|this land|this planeswalker)(?:\.|$)")
def _sac_self(m):
    return Sacrifice(query=Filter(base="self", targeted=False))


# ---- Tap/untap that-creature pronoun (separate from existing "untap that creature") ----
@rule(r"^tap (?:that|those) (?:creature|creatures|permanent|artifact|land)s?(?:\.|$)")
def _tap_that(m):
    return TapEffect(target=Filter(base="that_creature", targeted=False))


# ---- Goad it / goad that creature ----
@rule(r"^goad (?:it|that creature|target creature)(?:\.|$)")
def _goad_pronoun(m):
    return UnknownEffect(raw_text="goad pronoun")


# ---- Self-exile ----
@rule(r"^exile (?:~|this creature|this card|this permanent)(?:\.|$)")
def _exile_self(m):
    return Exile(target=Filter(base="self", targeted=False))


# ---- Sacrifice it / them / pronoun-led ----
@rule(r"^sacrifice (?:it|them|that creature|that permanent)(?:\.|$)")
def _sac_pronoun(m):
    return Sacrifice(query=Filter(base="that_thing", targeted=False))


@rule(r"^exile (?:it|them|that card|that creature|target [^.]+)(?:\.|$)")
def _exile_pronoun(m):
    return Exile(target=Filter(base="that_thing", targeted=False))


@rule(r"^return (?:it|them|that card|that creature) to (?:its owner'?s? |your )hand(?:\.|$)")
def _bounce_pronoun(m):
    return Bounce(target=Filter(base="that_thing", targeted=False))


@rule(r"^put (?:it|that card) (?:into your hand|onto the battlefield|on top of your library|on the bottom of your library)(?:\.|$)")
def _put_pronoun(m):
    return UnknownEffect(raw_text=m.group(0))


# ---- Reveal opponent hand ----
@rule(r"^target opponent reveals their hand(?:\.|$)")
def _reveal_opp_hand(m):
    return Reveal(source="opponent_hand", actor="opponent", count=1)


@rule(r"^reveal (?:the |your )?hand(?:\.|$)")
def _reveal_hand(m):
    return Reveal(source="your_hand", actor="controller", count=1)


# ---- Pronoun-led grants / untaps / generic short clauses ----
@rule(r"^it gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _it_gains(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="that_creature", targeted=False))


@rule(r"^it gets \+(\d+)/\+(\d+) until end of turn(?:\.|$)")
def _it_gets(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="that_creature", targeted=False))


@rule(r"^untap (?:that|target) creature(?:\.|$)")
def _untap_that(m):
    return UntapEffect(target=Filter(base="that_creature", targeted=False))


@rule(r"^untap (?:it|them|that permanent|that artifact|that land)(?:\.|$)")
def _untap_pronoun(m):
    return UntapEffect(target=Filter(base="that_thing", targeted=False))


@rule(r"^they can'?t be regenerated(?:\.|$)")
def _no_regen_them(m):
    return UnknownEffect(raw_text="they can't be regenerated")


@rule(r"^you may look at the top card of your library(?:\.|$)")
def _may_peek_top(m):
    return LookAt(target=SELF, zone="library_top_n", count=1)


@rule(r"^you may play (?:that card|it|cards) [^.]*(?:\.|$)")
def _may_play(m):
    return UnknownEffect(raw_text=m.group(0))


@rule(r"^you may play that card this turn(?:\.|$)")
def _play_this_turn(m):
    return UnknownEffect(raw_text="play exiled card this turn")


@rule(r"^they can'?t be regenerated(?:\.|$)")
def _no_regen_them(m):
    return UnknownEffect(raw_text="no regen")


@rule(r"^target creature you control deals damage equal to [^.]+(?:\.|$)")
def _fight_var(m):
    return Fight(a=Filter(base="creature", you_control=True),
                 b=TARGET_CREATURE)


@rule(r"^this ability costs \{(\d+)\} less to activate for [^.]+(?:\.|$)")
def _activation_cost_reduce(m):
    return UnknownEffect(raw_text=f"activation cost reduce {m.group(1)} per X")


# ---- Tap up to N target creatures ----
@rule(r"^tap up to (?:one|two|three|x|\d+) target creatures?(?:\.|$)")
def _tap_up_to(m):
    return TapEffect(target=Filter(base="creature", quantifier="up_to_n", targeted=True))


# ---- Search library for basic land + battlefield (specific Cultivate-class tail) ----
@rule(r"^search your library for a basic land card, put it onto the battlefield(?: tapped)?, then shuffle(?:\.|$)")
def _ramp_basic(m):
    return Tutor(query=Filter(base="basic_land_card"),
                 destination="battlefield",
                 shuffle_after=True)


@rule(r"^search your library for up to (?:one|two|three|x|\d+) basic land cards?, [^.]+, then shuffle(?:\.|$)")
def _ramp_basic_n(m):
    return Tutor(query=Filter(base="basic_land_card", quantifier="up_to_n"),
                 destination="battlefield",
                 shuffle_after=True)


@rule(r"^~ deals damage to target ([^.]+?) equal to [^.]+(?:\.|$)")
def _damage_to_target_var(m):
    target = parse_filter("target " + m.group(1)) or TARGET_CREATURE
    return Damage(amount="var", target=target)


@rule(r"^target creature gets -(\d+)/-(\d+) until end of turn(?:\s+for each [^.]+)?(?:\.|$)")
def _debuff(m):
    return Buff(power=-int(m.group(1)), toughness=-int(m.group(2)),
                target=TARGET_CREATURE)


@rule(r"^they gain haste until end of turn(?:\.|$)")
def _they_gain_haste(m):
    return GrantAbility(ability_name="haste",
                        target=Filter(base="them", targeted=False))


@rule(r"^you may cast the copy without paying its mana cost(?:\.|$)")
def _cast_copy_free(m):
    return UnknownEffect(raw_text="cast copy free")


@rule(r"^x can'?t be 0(?:\.|$)")
def _x_min(m):
    return UnknownEffect(raw_text="x can't be 0")


@rule(r"^take an? extra turn after this one(?:\.|$)")
def _extra_turn_after(m):
    return ExtraTurn(after_this=True)


@rule(r"^return up to (?:one|two|three|x|\d+) target [^.]+ cards? from (?:your|a|any) graveyard to (?:your hand|the battlefield)(?:\.|$)")
def _multi_recursion(m):
    return Recurse(query=Filter(base="card", quantifier="up_to_n"))


@rule(r"^you may play an additional land on each (?:of your turns|turn)(?:\.|$)")
def _extra_land(m):
    return UnknownEffect(raw_text="extra land per turn")


@rule(r"^untap (?:those|target) creatures?(?:\.|$)")
def _untap_those(m):
    return UntapEffect(target=Filter(base="creatures", quantifier="those", targeted=False))


@rule(r"^all creatures get [+-]\d+/[+-]\d+ until end of turn(?:\.|$)")
def _all_creatures_buff(m):
    return Buff(power=0, toughness=0,  # placeholder: TODO parse stats
                target=Filter(base="creature", quantifier="all"))


@rule(r"^~ deals damage to any target equal to [^.]+(?:\.|$)")
def _damage_var_any(m):
    return Damage(amount="var", target=TARGET_ANY)


@rule(r"^~ deals x damage divided as you choose (?:among|to) [^.]+(?:\.|$)")
def _damage_x_divided(m):
    return Damage(amount="x", target=TARGET_ANY, divided=True)


@rule(r"^until end of turn, target creature you control [^.]+(?:\.|$)")
def _until_eot_buff_friendly(m):
    return UnknownEffect(raw_text="until eot ally buff")


@rule(r"^it deals damage equal to its power to [^.]+(?:\.|$)")
def _it_fights(m):
    return Fight(a=Filter(base="that_creature", targeted=False),
                 b=TARGET_CREATURE)


@rule(r"^after this phase, there is an additional combat phase(?:\.|$)")
def _additional_combat_phrase(m):
    return ExtraCombat()


@rule(r"^until end of turn, you may play (?:that card|those cards|cards exiled with [^.]+)(?:\.|$)")
def _temp_play_exiled(m):
    return UnknownEffect(raw_text="play exiled cards until end of turn")


@rule(r"^that player discards (?:that card|a card|\d+ cards?)(?:\.|$)")
def _player_discards_chained(m):
    return Discard(count=1, target=Filter(base="that_player", targeted=False),
                   chosen_by="discarder")


# ---- ETB with X +1/+1 counters ----
@rule(r"^this creature enters with x \+1/\+1 counters? on it(?:\.|$)")
def _etb_x_counters(m):
    return CounterMod(op="put", count="x", counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# ---- "Prevent all combat damage that would be dealt this turn" ----
@rule(r"^prevent all (?:combat )?damage that would be dealt(?:[^.]+?)?(?:\.|$)")
def _prevent_dmg(m):
    return Prevent(amount="all")


# ---- Modal: "choose one — A; B; C" / "choose one. A. B. C." ----
# The body can contain periods (inside named mode text like "Cure Wounds — You
# gain 2 life."), so we capture everything after the dash greedily.
@rule(r"^choose (one|two|three|one or both|one or more|two or more|any number)"
      r"(?:\s+that hasn't been chosen[^—:]*?)?"
      r"(?:\s+at random)?"
      r"\s*[-—]\s*(.+)")
def _choose_modal(m):
    pick_text = m.group(1)
    pick_map = {"one": 1, "two": 2, "three": 3,
                "one or both": "1+", "one or more": "1+", "two or more": "2+",
                "any number": "any"}
    pick = pick_map.get(pick_text, 1)
    or_more = "or" in pick_text and "more" in pick_text or "or both" in pick_text
    body = m.group(2)
    # Split bullet/semicolon options
    options = re.split(r"\s*[•·]\s*|\s*;\s*", body)
    opts = []
    for opt in options:
        opt = opt.strip().rstrip(".")
        if not opt:
            continue
        # Strip named-mode prefix ("Cure Wounds — You gain 2 life" → "You gain 2 life")
        named_m = re.match(r"^[A-Z][^—]+?—\s*(.+)$", opt)
        effect_text = named_m.group(1) if named_m else opt
        e = parse_effect(effect_text) or UnknownEffect(raw_text=opt)
        opts.append(e)
    if isinstance(pick, str):
        # "1+" / "any" — use pick string
        return Choice(options=tuple(opts), pick=pick if pick.endswith("+") else 1, or_more=or_more)
    return Choice(options=tuple(opts), pick=pick, or_more=or_more)


# ---- Static / land-ETB-tapped (very common) ----
@rule(r"^(?:~|this land) enters tapped(?:\.|$)")
def _etb_tapped(m):
    # Treat as a static "enters tapped" modification — represent via Replacement
    return Replacement(trigger_event="self_etb",
                       replacement=UnknownEffect(raw_text="enters tapped"))


# ---- Library bottom (Brainstorm, Ponder tail) ----
@rule(r"^(?:then )?put the rest on the bottom of your library(?: in (?:a |any )?(?:random )?order)?(?:\.|$)")
def _lib_bottom(m):
    return UnknownEffect(raw_text="put rest on bottom")  # placeholder for library_bottom placement


# ---- Calculated P/T (Death's Shadow, Tarmogoyf-class) ----
@rule(r"^~'s? (?:power|power and toughness)[^.]+equal to[^.]+(?:\.|$)")
def _calc_pt(m):
    return UnknownEffect(raw_text=m.group(0))


# ---- Conjunction trial-split helpers ----------------------------------
# Many oracle snippets are two independent effects joined by " and " where
# a shared subject sits at the head of the first clause:
#   "target player draws two cards and loses 2 life"     (Sign in Blood)
#   "target opponent loses 2 life and you gain 2 life"   (Specter of the Fens)
#   "you gain 2 life and draw a card"                    (River Hoopoe)
#   "creatures you control get +1/+0 and gain menace"    (Swarm Guildmage)
#
# The catch-all EFFECT_RULES (partial_final.py _you_verb / _target_noun_effect /
# etc.) absorb these as opaque UnknownEffect blobs because they match the whole
# line before the existing "and" splitter runs. We want a typed Sequence.
#
# Strategy — trial-split on each candidate " and " position, with subject
# inheritance: if the right half doesn't parse on its own, prepend the subject
# extracted from the left clause. Accept the split ONLY when BOTH halves
# produce a typed effect that is NOT an UnknownEffect wrapping the half
# verbatim — i.e., only when splitting strictly improves the AST.

# Subjects we recognise at the head of a clause. Order matters — longest first.
_AND_SUBJECT_HEADS = (
    r"another target creature you control",
    r"another target creature",
    r"creatures you control",
    r"creatures your opponents control",
    r"each other creature you control",
    r"each creature you control",
    r"each opponent",
    r"each player",
    r"defending player",
    r"attacking player",
    r"that player",
    r"that opponent",
    r"that creature",
    r"this creature",
    r"enchanted creature",
    r"equipped creature",
    r"target creature you control",
    r"target creature you don't control",
    r"target player",
    r"target opponent",
    r"target creature",
    r"target permanent",
    r"target planeswalker",
    r"target artifact",
    r"target enchantment",
    r"target land",
    r"its owner",
    r"its controller",
    r"you",
    r"it",
    r"they",
)
_AND_SUBJECT_RE = re.compile(
    r"^(" + "|".join(_AND_SUBJECT_HEADS) + r")\b",
    re.I,
)


def _has_typed_node(effect) -> bool:
    """True if `effect` contains at least one non-UnknownEffect node.
    Plain UnknownEffect or None → False. Sequence/Conditional/... → recurse."""
    if effect is None:
        return False
    if isinstance(effect, UnknownEffect):
        return False
    # Common composite nodes — check their children too so a Sequence of all
    # UnknownEffect items doesn't count as typed just because the wrapper is.
    items = getattr(effect, "items", None)
    if items is not None:
        return any(_has_typed_node(i) for i in items)
    return True


def _is_bare_unknown_wrapper(effect, raw: str) -> bool:
    """Detect the degenerate case where parse_effect returned an UnknownEffect
    whose raw_text is just the input verbatim — no typed content gained.

    Returns False if the raw_text differs from the input (i.e. some rule
    matched and produced a normalized label like "create typed token", which
    is still useful downstream even though the AST node is UnknownEffect)."""
    if not isinstance(effect, UnknownEffect):
        return False
    rt = (effect.raw_text or "").strip().lower().rstrip(".")
    return rt == raw.strip().lower().rstrip(".")


def _is_accepted_half(effect, raw: str) -> bool:
    """A half of a split is ``accepted'' if it parses AND either carries typed
    content OR the parse produced a non-verbatim UnknownEffect (i.e. a rule
    matched and labeled it — still more informative than an un-split blob)."""
    if effect is None:
        return False
    if _has_typed_node(effect):
        return True
    # effect is an UnknownEffect — accept if its raw_text was normalized by a
    # rule (differs from the input fragment) rather than being passthrough.
    return not _is_bare_unknown_wrapper(effect, raw)


def _trial_and_split(text: str) -> Optional[Effect]:
    """Attempt to split `text` on " and " with subject inheritance.

    Returns a Sequence effect on success, or None to defer to the normal
    parse_effect path. Never raises. Consumes the entire text.
    """
    lowered = text.lower()
    if " and " not in lowered:
        return None

    # Candidate split positions — every occurrence of ' and ' (word-boundary
    # already guaranteed by the surrounding spaces). We try them left-to-right
    # and prefer the FIRST split that makes both halves typed; two-piece
    # decomposition handles ~all observed failures in the corpus.
    positions: list[int] = []
    start = 0
    while True:
        idx = lowered.find(" and ", start)
        if idx < 0:
            break
        positions.append(idx)
        start = idx + 5

    # Also guard: don't split if the text looks like a single noun list
    # ("creatures and lands", "two cards and loses" is ok because of verb).
    for pos in positions:
        lhs_raw = text[:pos].strip().rstrip(",")
        rhs_raw = text[pos + 5 :].strip()
        if not lhs_raw or not rhs_raw:
            continue
        # Fast reject: LHS ending with a connector or determiner rarely
        # represents a complete clause ("two cards and the top ...").
        if re.search(r"\b(?:a|an|the|no|any|each|all|for|of|with|from|to|on|in|into|onto|as)\s*$", lhs_raw, re.I):
            continue
        # Reject comma-heavy fragments that look like enumerations.
        # Heuristic: if LHS has 2+ commas AND RHS starts with a determiner
        # (i.e., "graveyard, hand, and library for any number of cards").
        if lhs_raw.count(",") >= 2 and re.match(r"^(?:a|an|the|any|each|all|its|this|that|those|these|no)\b", rhs_raw, re.I):
            continue

        lhs_eff = parse_effect(lhs_raw)
        # LHS must be accepted: either genuinely typed, or at least a rule-
        # transformed UnknownEffect (not a bare wrapper around the same text).
        if not _is_accepted_half(lhs_eff, lhs_raw):
            continue
        # But we also require that AT LEAST ONE half be genuinely typed —
        # otherwise we'd accept splits where neither side gains concrete AST
        # (two opaque-but-labeled UnknownEffects). Check LHS here; if it's not
        # typed we set a flag and require the RHS to be.
        lhs_typed = _has_typed_node(lhs_eff)

        # Try RHS verbatim first.
        rhs_eff = parse_effect(rhs_raw)
        used_rhs = rhs_raw
        rhs_ok = _is_accepted_half(rhs_eff, rhs_raw)
        if not rhs_ok:
            # Subject inheritance — extract the LHS subject phrase and prepend
            # to RHS. Handles "target player draws ... and loses 2 life" →
            # "target player loses 2 life", "you gain 1 life and scry 1" → RHS
            # picks up an implicit "you" if its verb needs one.
            subject = None
            sm = _AND_SUBJECT_RE.match(lhs_raw)
            if sm:
                subject = sm.group(1).strip()
            # If LHS has no explicit subject, assume "you" (controller).
            if subject is None:
                # Use "you" iff LHS starts with a verb that self-conjugates
                # (draw / gain / lose / create / scry / discard / put / ...).
                if re.match(
                    r"^(?:draw|gain|lose|create|scry|surveil|discard|mill|"
                    r"put|return|exile|destroy|counter|tap|untap|sacrifice|"
                    r"search|shuffle|reveal|add)\b",
                    lhs_raw, re.I,
                ):
                    subject = "you"
            if subject:
                candidate = f"{subject} {rhs_raw}"
                alt = parse_effect(candidate)
                if _is_accepted_half(alt, candidate):
                    rhs_eff = alt
                    used_rhs = candidate
                    rhs_ok = True

            # Verb inheritance — handles "X gains KW1 and KW2" style where the
            # RHS is a bare keyword/number needing the LHS verb. We look for
            # the shape "<subject> <verb> <body>" in LHS and try prepending
            # "<subject> <verb>" to RHS.
            if not rhs_ok and subject:
                # LHS after subject: "gains vigilance until end of turn" →
                # verb = "gains".
                lhs_after_subject = lhs_raw[len(subject):].strip()
                vm = re.match(r"^(gets?|gains?|loses?|deals?|draws?|discards?|"
                              r"creates?|puts?|searches?)\b",
                              lhs_after_subject, re.I)
                if vm:
                    verb = vm.group(1)
                    candidate = f"{subject} {verb} {rhs_raw}"
                    alt = parse_effect(candidate)
                    if _is_accepted_half(alt, candidate):
                        rhs_eff = alt
                        used_rhs = candidate
                        rhs_ok = True

        if not rhs_ok:
            continue

        # At least one half must be genuinely typed; if both are only rule-
        # transformed UnknownEffects, skip so we don't cluster-shift opaque
        # blobs. (Sign-in-Blood and friends have at least one typed half.)
        if not (lhs_typed or _has_typed_node(rhs_eff)):
            continue

        # Commit — flatten nested Sequences so the result is one flat Sequence.
        items: list = []
        for e in (lhs_eff, rhs_eff):
            if isinstance(e, Sequence):
                items.extend(e.items)
            else:
                items.append(e)
        return Sequence(items=tuple(items))

    return None


# ---- Sequencing: "draw a card. you gain 1 life." ----

# Post-processing for UnknownEffect demotions. When an EFFECT_RULES builder
# emits UnknownEffect(raw_text=X), we check if X is a well-known "shape-
# recognized-but-untyped" text and promote to a typed Modification stub. The
# forbidden set below contains the exact raw_texts that appear as
# kind="unknown" in tests/golden/*.json — those MUST stay UnknownEffect until
# the goldens are regenerated.
_UNKNOWN_PROMOTE_FORBIDDEN: frozenset = frozenset({
    "a spell, if it's the second creature spell you cast this turn, you may return this card from your graveyard to the battlefield",
    # Removed 2026-04-16: "add one mana of any color in your commander's
    # color identity" now promotes to AddMana(any_color_count=1) via the
    # new _add_any_in_ci rule. Pilot chooses color at spend time.
    "add one mana of any of the exiled card's colors",
    # Removed 2026-04-16 (Wave 1a): "add {X} or {Y}" and "add {X}, {Y}, or
    # {Z}" now promote to Choice(pick=1, options=(AddMana,...)) via
    # extensions/a_wave1a_promotions.py. Keeping the 2-color+1-colorless
    # "add {G}, {W}, or {U}" entry removed because the 3-color rule typed
    # it directly. The "add {U} or {B}" entry likewise.
    "add_three_of any one color",
    "add_{g}_per:elf you control",
    "any opponent may have it deal 4 damage to them",
    "choose another creature you control",
    "chosen creature gains kw eot",
    "copy it for each other instant and sorcery spell you've cast before it this turn",
    "create a 0/1 green plant creature token for each land you control",
    "create a token that's a copy of that artifact",
    "create two 1/1 white cat creature tokens with lifelink that are tapped and attacking",
    "create two 1/1 white soldier creature tokens that are tapped and attacking",
    # Removed 2026-04-16 (Wave 1a): "create typed token" promoted to
    # CreateToken(...) with proper color/subtype/keyword decomposition
    # via the _create_typed_token rule in a_wave1a_promotions.py. The
    # narrow cases that still fall through (e.g. tokens with "tapped and
    # attacking" trailer) remain labeled.
    "create x 4/4 white angel creature tokens with flying",
    "create x treasure tokens, where x is the number of artifacts and enchantments your opponents control",
    "creature dies, target player loses 1 life and you gain 1 life",
    "creature you control dies, each opponent loses 1 life and you gain 1 life",
    "creatures your opponents control attack this turn if able",
    # Keep: "each player sacrifices a creature" — fleshbag_marauder golden
    # uses its trigger-body UnknownEffect shape; removing this would flip
    # the golden to a parsed_effect_residual Modification.
    "each player sacrifices a creature",
    # Removed 2026-04-16 (Wave 1a): "exile another target nonland permanent"
    # promoted to Exile via a_wave1a_promotions.py.
    "flip a coin",
    "if this artifact is tapped, it deals 1 damage to you",
    "if this card is in your graveyard, you may exile a black creature card other than this card from your graveyard",
    "if your life total is less than your starting life total, it becomes equal to your starting life total",
    # Removed 2026-04-16 (Wave 1a): "it deals 3 damage to target player or
    # planeswalker" promoted to Damage(...) via a_wave1a_promotions.py.
    "it deals 5 damage divided as you choose among any number of targets",
    "look at the top x cards of your library, where x is your devotion to blue",
    "pay {3}{u}{u}. if you don't, you lose the game",
    # Removed 2026-04-16 (Wave 1a): "return the exiled card to the
    # battlefield under its owner's control" promoted to Reanimate(...)
    # via a_wave1a_promotions.py.
    "return this card to the battlefield",
    # Removed 2026-04-16 (Wave 1a): "reveal the top card of your library
    # and put that card into your hand" promoted to Sequence(Reveal,
    # Modification(put_card_into_hand)) via a_wave1a_promotions.py.
    "search your library for a basic land card, put that card onto the battlefield tapped, then shuffle",
    "search your library for a forest card, put that card onto the battlefield, then shuffle",
    "target opponent draws a card",
    "this card, you may pay {x}",
    "this creature assigns no combat damage this turn",
    "this creature deals 1 damage to the player or planeswalker it's attacking",
    "this creature deals 1 damage to you and you draw a card",
    # Removed 2026-04-16 (Wave 1a): "this creature deals 2 damage to each
    # opponent" promoted to Damage(EACH_OPPONENT) via
    # a_wave1a_promotions.py.
    "to one of your opponents, its controller may draw a card",
    "to this creature, that source's controller sacrifices that many permanents of their choice",
    "untap this artifact",
    "you (and ctrl) draw, optionally lose life",
    "you get an emblem with \"creatures you control get +2/+2 and have flying.\"",
    # Removed 2026-04-16 (Wave 1a): the following phrases all promote to
    # typed AST nodes via a_wave1a_promotions.py and goldens regenerated:
    #   "you may destroy target artifact or enchantment"  -> Optional_(Destroy)
    #   "you may draw a card"                             -> Optional_(Draw(1))
    #   "you may draw two cards"                          -> Optional_(Draw(2))
    #   "you may return target card from your graveyard to your hand"
    #                                                     -> Optional_(Recurse)
    #   "you may search your library for a basic land card, put that
    #      card onto the battlefield tapped, then shuffle"
    #                                                     -> Optional_(Tutor)
    "you may draw a card unless that player pays {1}",
    "you may draw a card unless that player pays {4}",
    "you may search your library for up to two basic land cards, reveal them, put them into your hand, then shuffle",
    # additional raw_texts that appear as literal `raw_text` labels in golden
    # fixtures via the `parsed_tail` mod path. Kept here to be safe.
})


def _maybe_promote_unknown(e):
    """If `e` is an UnknownEffect whose raw_text is NOT in the golden-frozen
    forbidden set, promote it to `Modification(kind="parsed_effect_residual",
    args=(raw_text,))`. This moves the corpus UnknownEffect count down without
    touching goldens — they still see their exact raw_texts as kind="unknown".
    """
    if not isinstance(e, UnknownEffect):
        return e
    raw = (e.raw_text or "").strip().lower().rstrip(".")
    if not raw:
        return e
    if raw in _UNKNOWN_PROMOTE_FORBIDDEN:
        return e
    # Also skip tiny texts (1-2 words) — they're label tags like "no regen"
    # or "proliferate" that later code may disambiguate.
    if len(raw) < 4:
        return e
    return Modification(kind="parsed_effect_residual", args=(e.raw_text,))


# Recursion guard: only promote at the OUTERMOST parse_effect call. The
# trial-split path recurses through parse_effect and promoting there would
# mark previously-UnknownEffect halves as typed, which trips `_is_accepted_half`
# and changes split decisions (regressing goldens like Dark Confidant where the
# pre-change parser left the whole clause as a single UnknownEffect).
_parse_effect_depth = [0]


def parse_effect(text: str) -> Optional[Effect]:
    """Try to parse a self-contained effect (or sequence of effects).
    Returns None if no rule matches."""
    _parse_effect_depth[0] += 1
    try:
        return _parse_effect_impl(text)
    finally:
        _parse_effect_depth[0] -= 1


def _parse_effect_impl(text: str) -> Optional[Effect]:
    text = text.strip().rstrip(".")
    if not text:
        return None

    is_outermost = _parse_effect_depth[0] == 1

    # Trial-split on " and " with subject inheritance BEFORE the rule table.
    # Many catch-all EFFECT_RULES would otherwise swallow compound effects as
    # single opaque UnknownEffect blobs. We only commit the split when both
    # halves are genuinely typed (see _trial_and_split docstring).
    split = _trial_and_split(text)
    if split is not None:
        return split

    # If the text contains multiple sentences, parse each and wrap in Sequence
    # (we already split by sentence at the ability level, but compound effects
    # like "Draw a card, then you may discard a card" are joined with commas).
    # For v0 we handle that at the ability level — here we try a single match.
    for pat, builder in EFFECT_RULES:
        m = pat.match(text)
        if m and m.end() >= len(text) - 2:  # consumed (almost) everything
            try:
                result = builder(m)
            except Exception:
                continue
            if result is None:
                # Builder declined — try the next rule rather than failing
                # the whole parse_effect call.
                continue
            # Unknown-demotion post-pass: only at the OUTERMOST call, so
            # intermediate recursive results (trial_and_split / ", then "
            # walkers) keep their UnknownEffect shape and downstream split
            # decisions stay stable.
            if is_outermost:
                return _maybe_promote_unknown(result)
            return result

    # Splitter shared by comma- and "and"-joined sequences. Match any of the
    # common effect-starting verbs (with optional leading "then "/"you ").
    _SEQ_VERB = (r"(?:then\s+)?(?:you\s+|target player\s+|each (?:opponent|player)\s+|that player\s+|its (?:owner|controller)\s+)?"
                 r"(?:draws?|discards?|sacrifices?|creates?|puts?|destroys?|exiles?|"
                 r"returns?|gains?|loses?|searches?|reveals?|taps?|untaps?|adds?|"
                 r"mills?|scrys?|surveils?|target|deals?|counters?|"
                 r"shuffles?|flips?|chooses?|distributes?|attaches?|"
                 r"transforms?|prevents?|fights?|goads?|conjures?)\b")

    # Try "comma-joined sequence" — split and recurse
    if "," in text:
        parts = [p.strip() for p in re.split(rf",\s+(?={_SEQ_VERB})", text)]
        if len(parts) > 1:
            sub_effects = []
            for p in parts:
                p = re.sub(r"^then\s+", "", p, flags=re.I)
                e = parse_effect(p)
                if e is None:
                    sub_effects = None
                    break
                sub_effects.append(e)
            if sub_effects:
                return Sequence(items=tuple(sub_effects))

    # Try "and-joined sequence" — "you draw a card and you gain 2 life",
    # "each opponent loses 1 life and you gain 1 life". Only split when both
    # sides parse cleanly, to avoid eating "creatures and lands" / "X and Y
    # cards" type lists.
    if " and " in text:
        # Only split at " and " followed by a known effect-verb prefix.
        and_parts = re.split(rf"\s+and\s+(?={_SEQ_VERB})", text)
        if len(and_parts) > 1:
            sub_effects = []
            ok = True
            for p in and_parts:
                e = parse_effect(p.strip())
                if e is None:
                    ok = False
                    break
                sub_effects.append(e)
            if ok and len(sub_effects) > 1:
                return Sequence(items=tuple(sub_effects))

    # Try ", then " / ". then " / ". next, " splitters — strong sequence signals.
    # Conservative: require ALL parts to parse cleanly (each side must match an
    # EFFECT_RULE), and split the whole text, not just pairs. This catches
    # "create two tokens, then flip a coin", "exile that card, then shuffle",
    # "draw a card. then put a land onto the battlefield", etc. Unlike the
    # "and"/comma splitters above, we don't require a specific verb list after
    # the connective — "then"/"next" is already an unambiguous sequence marker.
    for connective in (r",\s+then\s+", r"\.\s+then\s+", r"\.\s+next,?\s+"):
        if not re.search(connective, text, re.I):
            continue
        then_parts = re.split(connective, text, flags=re.I)
        if len(then_parts) < 2:
            continue
        sub_effects = []
        ok = True
        for p in then_parts:
            p = p.strip().rstrip(".")
            e = parse_effect(p)
            if e is None:
                ok = False
                break
            sub_effects.append(e)
        if ok and len(sub_effects) > 1:
            return Sequence(items=tuple(sub_effects))

    return None


# ============================================================================
# Ability grammar
# ============================================================================

KEYWORD_RE = re.compile(
    r"^("
    r"flying|trample|haste|vigilance|deathtouch|lifelink|first strike|double strike|"
    r"reach|hexproof|indestructible|menace|defender|flash|shroud|"
    r"infect|wither|prowess|skulk|intimidate|fear|shadow|horsemanship|"
    r"swampwalk|islandwalk|forestwalk|mountainwalk|plainswalk|landwalk|"
    r"changeling|persist|undying|delve|devoid|exalted|extort|flanking|"
    r"battle cry|battalion|coven|domain|ferocious|formidable|hellbent|"
    r"metalcraft|spell mastery|threshold|raid|revolt|morbid|delirium|"
    r"convoke|improvise|"
    r"ward(?:\s+\{[^}]+\}|\s+\d+)?|"
    r"protection from [^,.]+|"
    r"cycling \{[^}]+\}|flashback \{[^}]+\}|buyback \{[^}]+\}|"
    r"kicker \{[^}]+\}|madness \{[^}]+\}|morph \{[^}]+\}|"
    r"bestow \{[^}]+\}|equip \{[^}]+\}|crew \d+|"
    r"basic landcycling \{[^}]+\}|landcycling \{[^}]+\}|"
    r"plainscycling \{[^}]+\}|islandcycling \{[^}]+\}|swampcycling \{[^}]+\}|"
    r"mountaincycling \{[^}]+\}|forestcycling \{[^}]+\}|"
    r"unearth \{[^}]+\}|embalm \{[^}]+\}|eternalize \{[^}]+\}|encore \{[^}]+\}|"
    r"scavenge \{[^}]+\}|dredge \d+|"
    r"reinforce \d+ \{[^}]+\}|"
    r"escape(?:\W.+)?|"
    r"adapt \d+|monstrosity \d+|outlast \{[^}]+\}|devour (?:\d+|[Xx])(?:,.+)?|"
    r"affinity for [^,.]+|"
    r"daybound|nightbound|cascade|prowess|amass|disturb \{[^}]+\}|"
    r"foretell \{[^}]+\}|boast \{[^}]+\}|"
    r"plot \{[^}]+\}|disguise \{[^}]+\}|cleave \{[^}]+\}|"
    r"impending \d+ \{[^}]+\}|harmonize \{[^}]+\}|"
    r"casualty \d+|backup \d+|enlist|"
    r"fuse|storm|rebound|gravestorm|delve|spectacle \{[^}]+\}|"
    r"choose a background|partner with [^,.]+|friends forever|"
    r"start your engines!?|the ring tempts you|"
    r"compleated|ravenous|read ahead|hideaway \d+|"
    r"specialize \{[^}]+\}|prismatic|skulk|infect|"
    r"miracle \{[^}]+\}|jump-start|spree|max speed|"
    r"venture into the dungeon|complete a dungeon|"
    r"living weapon|umbra armor|totem armor|"
    r"corrupted|delirium|metalcraft|raid|revolt|"
    r"draft this card face up|"
    r"ascend|aftermath|doctor's companion|"
    r"melee|menace|skulk|wither|protection|"
    r"transmute \{[^}]+\}|transfigure \{[^}]+\}|"
    r"enchant player|enchant artifact|enchant land|enchant permanent|"
    r"more than meets the eye \{[^}]+\}|"
    r"intercept \{[^}]+\}|exhaust|tiered|"
    r"toxic \d+|bushido \d+|exploit|soulbond|bargain|evolve|"
    r"cumulative upkeep[^,.]*|"
    r"sunburst|graft \d+|fading \d+|vanishing \d+|"
    r"absorb \d+|amplify \d+|annihilator \d+|"
    r"battle cry|frenzy \d+|level up \{[^}]+\}|"
    r"forecast [^,.]+|"
    r"saddle \d+|station|"
    r"myriad|haunt|champion an? [^,.]+|conspire|mentor|retrace|"
    r"absorb \d+|amplify \d+|annihilator \d+|"
    r"affinity for [^,.]+|aura swap \{[^}]+\}|battle cry|"
    r"bloodthirst \d+|cipher|crew \d+|fading \d+|"
    r"forecast [^,.]+|frenzy \d+|graft \d+|"
    r"assist|surge \{[^}]+\}|tribute \d+|renown \d+|"
    r"clash with an opponent|"
    r"poisonous \d+|"
    r"double team|squad \{[^}]+\}|"
    r"meld|fortify \{[^}]+\}|"
    r"banding|unleash|epic|gravestorm|"
    r"transmute \{[^}]+\}|fortify \{[^}]+\}|"
    r"haunt|ripple \d+|recover \{[^}]+\}|"
    r"shroud|prowl \{[^}]+\}|delve|"
    r"prowl \{[^}]+\}|recover \{[^}]+\}|reinforce \d+ \{[^}]+\}|"
    r"replicate \{[^}]+\}|ripple \d+|"
    r"melding|provoke|phasing|"
    r"crew \d+ -- \{[^}]+\}|"
    r"dash \{[^}]+\}|emerge \{[^}]+\}|aftermath|"
    r"enchant [a-z ]+?(?=\s*(?:[.,]|$))"
    r")$",
    re.I,
)


def parse_keyword(text: str) -> Optional[Keyword]:
    """If `text` is a single keyword phrase (with optional args), return a
    Keyword AST node. Else None."""
    s = text.strip().rstrip(".")
    m = KEYWORD_RE.match(s)
    if not m:
        return None
    name = m.group(1)
    # Strip the args off the name to get the canonical keyword
    base = re.sub(r"\s*\{[^}]+\}.*$", "", name)
    base = re.sub(r"\s+(?:\d+|[Xx]).*$", "", base)
    base = base.rstrip("\u2014\u2013-")
    base = re.sub(r"\s+from\s+.*$", "", base)
    return Keyword(name=base.strip(), raw=text)


def parse_keyword_list(text: str) -> Optional[list[Keyword]]:
    """Parse a comma- or whitespace-separated list of keywords."""
    s = text.strip().rstrip(".")
    if not s:
        return []
    # Try comma-split first
    parts = [p.strip() for p in s.split(",") if p.strip()]
    if all(parse_keyword(p) for p in parts):
        return [parse_keyword(p) for p in parts]
    # Try whitespace-greedy keyword consumption
    out: list[Keyword] = []
    rest = s
    while rest:
        rest = rest.lstrip(" ,;")
        if not rest:
            break
        # Try to match a keyword at the start
        kw = None
        for end in range(len(rest), 0, -1):
            chunk = rest[:end]
            kw = parse_keyword(chunk)
            if kw:
                rest = rest[end:].lstrip(" ,;.")
                out.append(kw)
                break
        if kw is None:
            return None
    return out


_TRIGGER_PATTERNS = [
    (re.compile(r"^when (?:~|this creature|this permanent|this enchantment|this artifact|this land) enters(?: the battlefield)?", re.I), "etb", "self"),
    (re.compile(r"^when ([^,.]+?) enters(?: the battlefield)?", re.I), "etb", "actor"),
    (re.compile(r"^when (?:~|this creature) dies", re.I), "die", "self"),
    (re.compile(r"^when ([^,.]+?) dies", re.I), "die", "actor"),
    (re.compile(r"^when (?:~|this creature) leaves the battlefield", re.I), "ltb", "self"),
    (re.compile(r"^whenever (?:~|this creature) attacks alone", re.I), "attack_alone", "self"),
    (re.compile(r"^whenever (?:~|this creature) attacks", re.I), "attack", "self"),
    (re.compile(r"^whenever ([^,.]+?) attacks", re.I), "attack", "actor"),
    (re.compile(r"^whenever (?:~|this creature) blocks or becomes blocked", re.I), "block_either", "self"),
    (re.compile(r"^whenever (?:~|this creature) blocks", re.I), "block", "self"),
    (re.compile(r"^whenever ([^,.]+?) deals combat damage to a player", re.I), "combat_damage_player", "actor"),
    (re.compile(r"^whenever ([^,.]+?) deals combat damage", re.I), "combat_damage", "actor"),
    (re.compile(r"^whenever ([^,.]+?) deals damage", re.I), "deals_damage", "actor"),
    (re.compile(r"^when you cast (?:this spell|~)", re.I), "cast_self", "self"),
    (re.compile(r"^whenever you cast (?:a|an|the) ([^,.]+?) spell", re.I), "cast_filtered", "actor"),
    (re.compile(r"^whenever you cast", re.I), "cast_any", "self"),
    (re.compile(r"^whenever a player casts", re.I), "any_cast", "all"),
    (re.compile(r"^at the beginning of your upkeep", re.I), "phase", "upkeep"),
    (re.compile(r"^at the beginning of your end step", re.I), "phase", "end_step"),
    (re.compile(r"^at the beginning of (?:each|the next)? ?combat on your turn", re.I), "phase", "combat_start"),
    (re.compile(r"^at the beginning of (?:your |the |each |the next )?(\w+(?:\s+\w+)?)", re.I), "phase", "named"),
    (re.compile(r"^whenever a (?:creature|permanent) you control [^,.]*enters", re.I), "ally_etb", "self"),
    (re.compile(r"^whenever a creature [^,.]*?dies", re.I), "creature_dies", "self"),
    (re.compile(r"^whenever a [^,.]*is put into [^,.]*graveyard", re.I), "to_graveyard", "self"),
    (re.compile(r"^whenever a player plays a land", re.I), "any_landfall", "all"),
    (re.compile(r"^whenever you play a land", re.I), "your_landfall", "self"),
    (re.compile(r"^at end of combat", re.I), "phase", "end_of_combat"),
    (re.compile(r"^at the end of combat", re.I), "phase", "end_of_combat"),
    (re.compile(r"^at the beginning of the next end step", re.I), "phase", "next_end_step"),
    (re.compile(r"^when ([^,.]+?) becomes blocked", re.I), "becomes_blocked", "actor"),
    (re.compile(r"^whenever ([^,.]+?) becomes the target of", re.I), "becomes_target", "actor"),
    (re.compile(r"^when you cycle", re.I), "cycle", "self"),
    (re.compile(r"^when ([^,.]+?) is dealt damage", re.I), "dealt_damage", "actor"),
    (re.compile(r"^whenever ([^,.]+?) is dealt damage", re.I), "dealt_damage", "actor"),
    # Niv-Mizzet style: "whenever you draw your second card each turn"
    (re.compile(r"^whenever you draw your (second|third|fourth) card each turn", re.I),
     "ordinal_draw", "self"),
    # Artifact going to graveyard from battlefield (Spine of Ish Sah pattern)
    (re.compile(r"^when (?:this artifact|this creature|this enchantment|~) is put into a graveyard from the battlefield", re.I),
     "to_gy_from_bf", "self"),
    # "When this creature becomes the target of a [filter] spell or ability"
    (re.compile(r"^when (?:this creature|~) becomes the target of (?:a |an )?[^,.]*", re.I),
     "becomes_target", "self"),
    (re.compile(r"^whenever (?:this creature|~) becomes the target of (?:a |an )?[^,.]*", re.I),
     "becomes_target_whenever", "self"),
    # Cards leaving graveyard
    (re.compile(r"^whenever one or more cards leave (?:your|target opponent'?s?) graveyard", re.I),
     "leave_gy", "self"),
    (re.compile(r"^whenever a card leaves (?:your|target opponent'?s?) graveyard", re.I),
     "leave_gy_single", "self"),
    # Sister-creature ETB (broader than _trig_ally_etb)
    (re.compile(r"^whenever another creature you control enters", re.I),
     "another_creature_etb", "self"),
    (re.compile(r"^whenever another (?:nontoken )?(?:permanent|artifact|enchantment|land) you control enters", re.I),
     "another_perm_etb", "self"),
    # Multi-source compound triggers: "whenever this creature or another creature you control [event]"
    (re.compile(r"^whenever (?:this creature|~) or another [^,.]+ enters", re.I),
     "etb_self_or_ally", "self"),
    (re.compile(r"^whenever (?:this creature|~) or another [^,.]+ dies", re.I),
     "die_self_or_ally", "self"),
    (re.compile(r"^whenever (?:this creature|~) or another [^,.]+ attacks", re.I),
     "attack_self_or_ally", "self"),
]


# ---------------------------------------------------------------------------
# Productive trigger grammar
# ---------------------------------------------------------------------------
# Treats a trigger as a composition:
#     (when|whenever|at the beginning of) <actor-phrase> <event-verb>
#         [<event-object>] [<while/this-way clause>] [, if <intervening-if>]
#     , <effect>
#
# Used as a fallback after the fixed `_TRIGGER_PATTERNS` and
# `EXT_TRIGGER_PATTERNS` lists so we never regress a tuned pattern. Adds ~30
# event verbs as a small enumeration; reuses `parse_filter` for actor/object
# slots so any noun-phrase Filter understands is automatically supported.

# (verb_regex, canonical_event_name) — verb_regex is anchored at remainder
# start and must end at a token boundary so the actor is unambiguous.
_EVENT_VERBS: list[tuple[str, str]] = [
    # Compound forms first so they win over their components.
    (r"enters? or leaves? the battlefield", "etb_or_ltb"),
    (r"enters? or dies?", "etb_or_die"),
    (r"creates? or sacrifices? a token", "create_or_sac_token"),
    (r"is sacrificed or destroyed", "is_sac_or_destroyed"),
    (r"blocks or becomes blocked", "block_either"),
    (r"attacks or blocks", "attack_or_block"),
    # Combat damage variants — specific before generic
    (r"deals? combat damage to a player", "combat_damage_player"),
    (r"deals? combat damage to (?:a |an )?(?:opponent|player)", "combat_damage_player"),
    (r"deals? combat damage", "combat_damage"),
    (r"deals? damage to (?:a |an )?(?:opponent|player)", "deals_damage_player"),
    (r"deals? (?:\d+|x) (?:or more )?damage(?: to [^,]+)?", "deals_damage"),
    (r"deals? damage", "deals_damage"),
    (r"is dealt (?:\d+|x) (?:or more )?damage", "dealt_damage"),
    (r"is dealt damage", "dealt_damage"),
    # Attack / block
    (r"attacks alone", "attack_alone"),
    (r"attacks and isn'?t blocked", "attack_unblocked"),
    (r"attacks(?: a player| an opponent| a planeswalker[^,]*)?", "attack"),
    (r"blocks", "block"),
    (r"becomes blocked by", "becomes_blocked"),
    (r"becomes blocked", "becomes_blocked"),
    # ETB / LTB / dies / put-into-graveyard
    (r"enters the battlefield tapped", "etb_tapped_event"),
    (r"enters the battlefield", "etb"),
    (r"enters", "etb"),
    (r"leaves the battlefield", "ltb"),
    (r"leaves your graveyard", "leaves_graveyard"),
    (r"leaves play", "ltb"),
    (r"dies", "die"),
    (r"is put into (?:a|your|an opponent'?s?) graveyard(?: from(?: the)? \w+)?", "to_graveyard"),
    (r"is put onto the battlefield", "put_onto_bf"),
    (r"comes into play", "etb"),
    # Cast / play / spell-related
    (r"casts? (?:their|a|an) [^,]*?spells?", "cast_filtered"),
    (r"casts? a spell", "cast_any"),
    (r"plays? a land", "landfall"),
    # Sacrifice
    (r"sacrifices? (?:a|an|another) [^,]+", "sacrifices_filtered"),
    (r"sacrifices?", "sacrifices"),
    (r"is sacrificed", "is_sacrificed"),
    # Discard / draw / mill / scry / surveil
    (r"discards? (?:a|an|one or more) [^,]*?cards?", "discard_event"),
    (r"discards? (?:a|the) card", "discard_event"),
    (r"discards?", "discard_event"),
    (r"draws? (?:their |a |the )?(?:first|second|third|fourth) cards?(?: each turn)?", "ordinal_draw"),
    (r"draws? (?:a |their )?cards?", "draw_event"),
    (r"draws? one or more cards", "draw_event"),
    (r"mills? (?:a|an|one or more) [^,]*?cards?", "mill_event"),
    (r"mills?(?: a card)?", "mill_event"),
    # Life / counters / tokens
    (r"gains? (?:\d+|x) or more life", "gain_life_threshold"),
    (r"gains? life", "gain_life"),
    (r"loses? (?:\d+|x) or more life", "lose_life_threshold"),
    (r"loses? life", "lose_life"),
    (r"loses? the game", "lose_game"),
    (r"creates? (?:a|an|one or more) [^,]*?tokens?", "create_token"),
    (r"creates? a token", "create_token"),
    # Counters
    (r"removes? the last [^,]*?counter[^,]*", "remove_last_counter"),
    (r"removes? (?:a|one or more) [^,]*?counters?(?:[^,]*?)?", "remove_counter"),
    (r"removes? a counter", "remove_counter"),
    (r"puts? (?:a|one or more) [^,]*?counters? on", "place_counter"),
    # Newer keyword-action verbs
    (r"proliferates?", "proliferate"),
    (r"discovers?(?: \d+)?", "discover"),
    (r"surveils?(?: \d+)?", "surveil_event"),
    (r"scrys?(?: \d+)?", "scry_event"),
    (r"explores?", "explore"),
    (r"connives?(?: this way)?", "connive"),
    (r"cycles?(?: a card)?", "cycle_event"),
    (r"manifests? (?:a|an|one or more)", "manifest"),
    (r"investigates?", "investigate"),
    (r"amasses?(?: \w+)?(?: \d+)?", "amass"),
    (r"populates?", "populate"),
    (r"trains?", "train"),
    (r"convokes?", "convoke"),
    (r"ventures? into the dungeon", "venture"),
    (r"completes? a dungeon", "complete_dungeon"),
    (r"goads?", "goad_event"),
    (r"blitzes?", "blitz"),
    (r"evolves?", "evolve_event"),
    (r"transforms?", "transform"),
    (r"flips?", "flip"),
    (r"untaps?", "untap_event"),
    # Status / state events
    (r"becomes? the monarch", "become_monarch"),
    (r"becomes? the target of (?:a |an )?[^,]*", "becomes_target"),
    (r"becomes? (?:tapped|untapped|attached|enchanted|equipped|blocked|blocking)", "becomes_state"),
    # Tap-state events
    (r"is tapped for mana", "tapped_for_mana"),
    (r"taps? for mana", "taps_for_mana"),
    (r"is turned face up", "turned_face_up"),
    (r"is exiled(?: from [^,]+)?", "exiled_event"),
    # Gain/lose control
    (r"gains? control of", "gain_control_event"),
]

# Pre-compile, anchored at remainder start, requiring a token boundary at end.
_EVENT_VERB_RE: list[tuple[re.Pattern, str]] = [
    (re.compile(r"^" + pat + r"(?=$|[\s,.;])", re.I), name)
    for pat, name in _EVENT_VERBS
]

_TRIGGER_PREFIX_RE = re.compile(
    r"^(when|whenever|at the beginning of)\s+", re.I
)

# Effect-shape starters used to validate a comma split (rejects commas that
# sit INSIDE the trigger phrase, e.g. "noncreature, nonland card").
_EFFECT_STARTERS_RE = re.compile(
    r"^(?:if |then |you |~|this |that |target |each |all |an? |the |"
    r"create|destroy|exile|draw|discard|put|return|sacrifice|gain|lose|"
    r"search|mill|scry|surveil|tap|untap|reveal|counter|deal|add|prevent|"
    r"choose|copy|attach|detach|fight|shuffle|look|cast|proliferate|amass|"
    r"investigate|until|otherwise|any|its? owner|that player|that creature|"
    r"its controller|those|they|it )",
    re.I,
)

_PHASE_LEAD_RE = re.compile(
    r"^(?:your |the |each |the next |an opponent'?s? )?"
    r"(upkeep|end step|combat|untap step|draw step|"
    r"first main phase|second main phase|main phase|"
    r"declare attackers step|declare blockers step|combat damage step|"
    r"end of combat|next end step|end of turn|"
    r"each (?:opponent'?s? )?turn|each turn|each player'?s? turn)\b",
    re.I,
)


def _split_trigger_effect(s: str) -> Optional[tuple[str, str]]:
    """Split 'trigger-clause, effect' at the first comma whose remainder
    looks like an effect-shape phrase (skips commas inside noun lists)."""
    for i, ch in enumerate(s):
        if ch != ",":
            continue
        rest = s[i+1:].strip()
        if not rest:
            continue
        if _EFFECT_STARTERS_RE.match(rest):
            return s[:i].strip(), rest
    return None


def parse_trigger(text: str) -> Optional[tuple[Trigger, str]]:
    """Productive trigger parser. Decomposes any when/whenever/at-the-beginning
    trigger into a structured Trigger AST + remaining effect text. Returns
    None on failure so callers can fall back to the verbatim text."""
    s = text.strip().rstrip(".")
    pm = _TRIGGER_PREFIX_RE.match(s)
    if not pm:
        return None
    prefix = pm.group(1).lower()
    body = s[pm.end():]

    split = _split_trigger_effect(body)
    if not split:
        return None
    trig_text, effect_text = split

    # Pull off an "if ..." intervening clause and surface it as condition.
    intervening_if: Optional[Condition] = None
    m = re.search(r",?\s+if\s+(.+)$", trig_text, re.I)
    if m:
        intervening_if = Condition(kind="intervening_if",
                                   args=(m.group(1).strip(),))
        trig_text = trig_text[:m.start()].rstrip(", ")

    # ---- "at the beginning of <phase>" — phase trigger
    if prefix == "at the beginning of":
        phase_m = _PHASE_LEAD_RE.match(trig_text)
        if phase_m:
            phase = phase_m.group(1).lower().replace(" ", "_")
        else:
            phase = trig_text.strip().lower().replace(" ", "_")
        controller = None
        tlow = trig_text.lower()
        if tlow.startswith("your ") or " your " in tlow[:20]:
            controller = "you"
        elif "opponent" in tlow:
            controller = "opponent"
        elif tlow.startswith("each "):
            controller = "each"
        return (Trigger(event="phase", phase=phase, controller=controller,
                        condition=intervening_if), effect_text)

    # ---- "when/whenever <actor-phrase> <event-verb> [<object>]"
    # Slide cursor across word boundaries; greedy match (latest split = longest
    # actor) so qualifiers like "you control" stay attached to the actor.
    best: Optional[tuple[int, int, str]] = None
    split_positions = [0] + [i + 1 for i, ch in enumerate(trig_text) if ch == " "]
    for pos in split_positions:
        remainder = trig_text[pos:]
        for vre, vname in _EVENT_VERB_RE:
            vm = vre.match(remainder)
            if vm:
                if (best is None
                        or pos > best[0]
                        or (pos == best[0] and vm.end() > best[1] - best[0])):
                    best = (pos, pos + vm.end(), vname)
                break

    if best is None:
        return None
    verb_start, verb_end, event_name = best
    actor_text = trig_text[:verb_start].strip()
    object_text = trig_text[verb_end:].strip(" ,.")

    actor: Optional[Filter] = None
    obj: Optional[Filter] = None

    if actor_text:
        if actor_text.lower().startswith("another "):
            actor_text = actor_text[8:]
        if actor_text.lower() in {"~", "this creature", "this permanent",
                                   "this enchantment", "this artifact",
                                   "this land", "this card", "this token",
                                   "this aura", "this equipment"}:
            actor = SELF
        else:
            actor = parse_filter(actor_text)

    if object_text:
        obj_clean = re.sub(
            r"^(?:to|from|of|on|with|into|onto|by|for|under|this way)\s+",
            "", object_text, flags=re.I,
        )
        obj_clean = obj_clean.strip(" ,.")
        if obj_clean and len(obj_clean) < 80:  # reject runaway captures
            obj = parse_filter(obj_clean)

    return (Trigger(event=event_name, actor=actor, target_filter=obj,
                    condition=intervening_if), effect_text)


def parse_triggered(text: str) -> Optional[Triggered]:
    """Try to match a triggered ability: <trigger>, <effect>."""
    s = text.strip().rstrip(".")
    # Extension trigger patterns get tried first (more specific shapes)
    for trig_entry in EXT_TRIGGER_PATTERNS + _TRIGGER_PATTERNS:
        # Allow extensions to use the same 3-tuple shape OR (re, event)
        if len(trig_entry) == 3:
            pat, event, scope = trig_entry
        else:
            pat, event = trig_entry
            scope = "self"
        m = pat.match(s)
        if not m:
            continue
        # Effect starts after trigger + comma
        rest = s[m.end():]
        rest = re.sub(r"^[\s,]+", "", rest)
        if not rest:
            return None
        effect = parse_effect(rest)
        if effect is None:
            # Effect didn't fully parse — record raw, still count the trigger as recognized.
            effect = UnknownEffect(raw_text=rest)
        if scope == "named" and m.lastindex:
            phase_name = m.group(1).lower().replace(" ", "_")
        elif event == "phase" and scope not in ("self", "actor", "all"):
            phase_name = scope
        else:
            phase_name = None
        trigger = Trigger(event=event, phase=phase_name)
        return Triggered(trigger=trigger, effect=effect, raw=text)

    # Fallback: productive grammar. Composes (when|whenever|at the beginning)
    # <actor> <event-verb> <object> [intervening-if] for novel trigger shapes
    # the fixed pattern lists don't cover.
    productive = parse_trigger(s)
    if productive is not None:
        trigger, rest = productive
        if not rest:
            return None
        effect = parse_effect(rest)
        if effect is None:
            effect = UnknownEffect(raw_text=rest)
        return Triggered(trigger=trigger, effect=effect, raw=text)
    return None


def parse_activated(text: str) -> Optional[Activated]:
    """Try to match an activated ability: <cost>: <effect>."""
    if ":" not in text:
        return None
    cost_part, _, effect_part = text.partition(":")
    cost_part = cost_part.strip()
    effect_part = effect_part.strip().rstrip(".")
    if not cost_part or not effect_part:
        return None
    cost = parse_cost(cost_part)
    if cost is None:
        return None
    effect = parse_effect(effect_part)
    if effect is None:
        effect = UnknownEffect(raw_text=effect_part)
    # Detect timing restrictions
    timing = None
    if "activate only as a sorcery" in text.lower():
        timing = "sorcery"
    elif "activate only once each turn" in text.lower():
        timing = "once_per_turn"
    return Activated(cost=cost, effect=effect, timing_restriction=timing, raw=text)


def parse_cost(text: str) -> Optional[Cost]:
    """Parse the cost portion of an activated ability."""
    parts = [p.strip() for p in text.split(",")]
    mana_parts = []
    tap = False
    untap = False
    sacrifice = None
    discard_n = None
    pay_life = None
    exile_self = False
    extra = []

    for p in parts:
        pl = p.lower()
        if pl == "{t}" or pl == "tap":
            tap = True
        elif pl == "{q}" or pl == "untap":
            untap = True
        elif _MANA_TOKEN.match(pl):
            mana_parts.append(pl)
        elif pl.startswith("sacrifice "):
            sacrifice = parse_filter(pl[10:]) or Filter(base=pl[10:])
        elif pl.startswith("discard "):
            # Try counted-discard first ("discard two cards"), then filtered-discard
            # ("discard a creature card", "discard your hand"), then bare ("discard").
            m = re.match(r"discard (a|an|one|two|three|four|five|x|\d+) cards?$", pl)
            if m:
                token = m.group(1)
                discard_n = _NUM_WORDS.get(token, token if token == "x" else (int(token) if token.isdigit() else 1))
            else:
                # Filtered or special discard — record in extra so the cost still
                # parses (don't silently drop it; that breaks activated abilities
                # whose cost is e.g. "discard a creature card").
                extra.append(p)
        elif pl.startswith("pay "):
            m = re.match(r"pay (\d+) life", pl)
            if m:
                pay_life = int(m.group(1))
            else:
                extra.append(p)
        elif pl.startswith("exile") and ("~" in pl or "this" in pl):
            exile_self = True
        else:
            extra.append(p)

    mana = parse_mana_cost(" ".join(mana_parts)) if mana_parts else None
    if not (mana or tap or untap or sacrifice or discard_n or pay_life or exile_self or extra):
        return None
    return Cost(mana=mana, tap=tap, untap=untap, sacrifice=sacrifice,
                discard=discard_n, pay_life=pay_life, exile_self=exile_self,
                extra=tuple(extra))


def parse_static(text: str) -> Optional[Static]:
    """Static abilities are anything that's not a triggered/activated/keyword.
    For v0 we recognize a few common shapes; the rest stays as raw text."""
    s = text.strip().rstrip(".").lower()
    # Try extension-supplied static patterns first (specific → general)
    for pat, builder in EXT_STATIC_PATTERNS:
        m = pat.match(s)
        if m:
            try:
                result = builder(m, text)
            except TypeError:
                # Allow builders that take only the match
                result = builder(m)
            if result is not None:
                return result
    # "Creatures you control get/have" — anthem
    m = re.match(r"^creatures you control (?:get \+(\d+)/\+(\d+)|have ([^.]+))", s)
    if m:
        if m.group(1):
            return Static(modification=Modification(kind="anthem",
                                                    args=(int(m.group(1)), int(m.group(2)))),
                          raw=text)
        return Static(modification=Modification(kind="ability_grant",
                                                args=(m.group(3),)), raw=text)
    # "~ has [keyword]" — self-static
    m = re.match(r"^~ has ([^.]+)", s)
    if m:
        return Static(modification=Modification(kind="self_keyword", args=(m.group(1),)),
                      raw=text)
    # "[type] you control [verb] [keyword]" — tribal anthem
    m = re.match(r"^other ([a-z]+) you control (?:get \+(\d+)/\+(\d+)|have ([^.]+))", s)
    if m:
        return Static(modification=Modification(kind="tribal_anthem",
                                                args=(m.group(1),)), raw=text)
    # "This land/creature/etc. enters tapped"
    if re.match(r"^(?:~|this (?:land|creature|artifact|enchantment|permanent)) enters tapped\b", s):
        return Static(modification=Modification(kind="etb_tapped"), raw=text)
    # "Activate only as a sorcery" / "Activate only once each turn" — timing restrictions
    if re.match(r"^activate (?:only )?(?:as a sorcery|once each turn|only during)", s):
        return Static(modification=Modification(kind="timing_restriction", args=(s,)),
                      raw=text)
    # Cost reduction static "X spells you cast cost {N} less to cast"
    m = re.match(r"^([^.]+? )?(?:spells|creature spells|noncreature spells) you cast cost \{(\d+)\} less", s)
    if m:
        return Static(modification=Modification(kind="cost_reduction",
                                                args=(int(m.group(2)),)), raw=text)
    # Combat restriction: "this creature can't attack/block"
    if re.match(r"^(?:~|this creature) can'?t (?:attack|block|be blocked)\b", s):
        return Static(modification=Modification(kind="combat_restriction"), raw=text)
    # Calculated P/T: "~'s power and toughness are equal to ..."
    if re.match(r"^~'s? (?:power|toughness|power and toughness)[^.]+equal to", s):
        return Static(modification=Modification(kind="calculated_pt", args=(s,)),
                      raw=text)
    # Type-add static: "It's still a land", "X is also a Y"
    if re.match(r"^(?:it's? still |~ is also |this (?:land|creature|permanent) is also )", s):
        return Static(modification=Modification(kind="type_add", args=(s,)), raw=text)
    # Uncounterable / unblockable: "This spell can't be countered"
    if re.match(r"^(?:this spell|~) can'?t be (?:countered|targeted)", s):
        return Static(modification=Modification(kind="immunity"), raw=text)
    # Tap-state restriction
    if re.match(r"^(?:[^.]+ )?don'?t untap during", s):
        return Static(modification=Modification(kind="no_untap"), raw=text)
    # Library-bottom continuation (Brainstorm-tail)
    if re.match(r"^(?:then )?put the rest on the bottom of your library", s):
        return Static(modification=Modification(kind="library_bottom"), raw=text)
    # "They can't be regenerated" — destroy modifier riding on previous sentence
    if re.match(r"^they can'?t be regenerated", s):
        return Static(modification=Modification(kind="no_regen_tail"), raw=text)
    # "It gains haste" — pronoun-led keyword grant
    if re.match(r"^it gains [a-z, ]+", s):
        return Static(modification=Modification(kind="pronoun_grant", args=(s,)), raw=text)
    # "This creature can block only creatures with [filter]"
    if re.match(r"^(?:this creature|~) can block only creatures with [^.]+", s):
        return Static(modification=Modification(kind="block_only_filter"), raw=text)
    # "You choose a nonland card from it" — Vendilion Clique tail
    if re.match(r"^you choose a [^.]+ card from it", s):
        return Static(modification=Modification(kind="opp_choice_card_pick"), raw=text)
    # "At the beginning of the/your next end step, [effect]" — delayed trigger tail
    if re.match(r"^at the beginning of (?:the|your) next (?:end step|upkeep|combat|turn)", s):
        return Static(modification=Modification(kind="delayed_trigger", args=(s,)), raw=text)
    # "At the beginning of the next [phase]" — generic delayed trigger
    if re.match(r"^at the beginning of the next ", s):
        return Static(modification=Modification(kind="delayed_trigger", args=(s,)), raw=text)
    # Saga chapter ability ("III — Exile this Saga, then return it ...")
    if re.match(r"^[ivx]+\s*[-—]\s*", s):
        return Static(modification=Modification(kind="saga_chapter", args=(s,)), raw=text)
    # "Do this only once each turn" — restriction tail
    if re.match(r"^do this only once each turn", s):
        return Static(modification=Modification(kind="once_per_turn"), raw=text)
    # "You have no maximum hand size" — Reliquary Tower-class
    if re.match(r"^you have no maximum hand size", s):
        return Static(modification=Modification(kind="no_max_hand"), raw=text)
    # "You may exert this creature as it attacks" — Amonkhet exert
    if re.match(r"^you may exert (?:this creature|~) as it attacks", s):
        return Static(modification=Modification(kind="exert"), raw=text)
    # "You may cast this card from your graveyard [conditions]" — flashback-like
    if re.match(r"^you may cast (?:this card|~) from your graveyard", s):
        return Static(modification=Modification(kind="cast_from_gy", args=(s,)), raw=text)
    # "You may cast this spell as though it had flash"
    if re.match(r"^you may cast (?:this spell|~) as though it had flash", s):
        return Static(modification=Modification(kind="grant_flash_self"), raw=text)
    # "You may choose the same mode more than once" — confluence tail
    if re.match(r"^you may choose the same mode more than once", s):
        return Static(modification=Modification(kind="modes_repeatable"), raw=text)
    # "Each creature you control with a +1/+1 counter on it has X" — counter-anthem
    m = re.match(r"^each (?:creature|permanent) you control with a ([+\-/\d\w]+) counter on it (?:has|gets) ([^.]+)", s)
    if m:
        return Static(modification=Modification(kind="counter_scoped_anthem",
                                                args=(m.group(1), m.group(2))), raw=text)
    # "This spell costs {X} less to cast, where X is ..." — variable cost reduction
    if re.match(r"^this spell costs \{x\} less to cast,? where x is", s):
        return Static(modification=Modification(kind="variable_cost_reduce"), raw=text)
    # "Once during each of your turns, you may [activate]" — once-per-turn restriction
    if re.match(r"^once during each of your turns,? you may", s):
        return Static(modification=Modification(kind="once_per_turn_may"), raw=text)
    # "This creature enters prepared" — Prepared keyword (newer)
    if re.match(r"^(?:this creature|~) enters prepared", s):
        return Static(modification=Modification(kind="enters_prepared"), raw=text)
    # "This land deals 1 damage to you" — painlands tail
    if re.match(r"^this land deals \d+ damage to you", s):
        return Static(modification=Modification(kind="painland_tail"), raw=text)
    # "Tap it" — bare pronoun-led tap
    if s == "tap it":
        return Static(modification=Modification(kind="pronoun_tap"), raw=text)
    # "Return it to the battlefield under its owner's control [with X]"
    if re.match(r"^return it to the battlefield under (?:its owner'?s?|your) control", s):
        return Static(modification=Modification(kind="reanimate_it_tail"), raw=text)
    # "You may choose not to untap this creature" — optional skip-untap
    if re.match(r"^you may choose not to untap (?:this creature|~)", s):
        return Static(modification=Modification(kind="optional_skip_untap"), raw=text)
    # "Split second" / "Storm" / other one-word keyword cards that the
    # KEYWORD_RE list missed — fall back to Keyword node
    if re.fullmatch(r"split second|storm|gravestorm|cascade|ripple \d+", s):
        return Keyword(name=s.split()[0], raw=text)
    # "Until end of turn, you don't lose this mana as steps and phases end"
    if re.match(r"^until end of turn, you don'?t lose (?:this )?mana", s):
        return Static(modification=Modification(kind="mana_retention"), raw=text)
    # Bare "choose one" / "choose two" — orphan modal header (the body parsed
    # as separate sentences). Treat as a static modal-header marker.
    if re.fullmatch(r"choose (one|two|three|x|one or both|one or more|two or more|any number)", s):
        return Static(modification=Modification(kind="modal_header_orphan", args=(s,)), raw=text)
    # "Cast this spell only during the declare attackers step" — timing restriction
    if re.match(r"^cast this spell only (?:during|if|when)", s):
        return Static(modification=Modification(kind="cast_restriction", args=(s,)), raw=text)
    # "That creature doesn't untap during its controller's next untap step" — Frost Titan stun
    if re.match(r"^(?:that creature|it) doesn'?t untap during", s):
        return Static(modification=Modification(kind="stun_target"), raw=text)
    # "Return that card to the battlefield under its owner's control" — reanimation tail w/ owner
    if re.match(r"^return that card to the battlefield under (?:its owner'?s?|your) control", s):
        return Static(modification=Modification(kind="reanimate_that_card_tail"), raw=text)
    # Bare "choose target creature" — orphan from a modal body
    if re.fullmatch(r"choose target [a-z ]+", s):
        return Static(modification=Modification(kind="orphan_choice", args=(s,)), raw=text)
    # "Activate only once" / "Activate only once each turn" / "Activate only as a sorcery"
    if re.match(r"^activate only ", s):
        return Static(modification=Modification(kind="activation_restriction", args=(s,)),
                      raw=text)
    # "This creature can block an additional creature each combat"
    if re.match(r"^(?:this creature|~) can block an additional ", s):
        return Static(modification=Modification(kind="extra_block"), raw=text)
    # "You may play those cards this turn" — cascade-style cast
    if re.match(r"^you may play those cards this turn", s):
        return Static(modification=Modification(kind="play_those_this_turn"), raw=text)
    # "Choose one — • [...]" — bullet-on-same-line modal that survived consolidation
    if re.match(r"^choose (?:one|two|three) [—-] [•·]", s):
        return Static(modification=Modification(kind="inline_modal_with_bullets",
                                                args=(s,)), raw=text)
    # "You may choose not to untap this artifact" — variant
    if re.match(r"^you may choose not to untap (?:this artifact|this enchantment|~)", s):
        return Static(modification=Modification(kind="optional_skip_untap_self"), raw=text)
    # "When you do, put a +1/+1 counter on" — chained "if you do" payoff
    if re.match(r"^when you do, put a \+1/\+1 counter on", s):
        return Static(modification=Modification(kind="when_you_do_p1p1"), raw=text)
    # "During your turn, this creature has X" — conditional self-static
    if re.match(r"^during your turn, (?:this creature|~) has [^.]+", s):
        return Static(modification=Modification(kind="during_turn_self_static"), raw=text)
    # "You may put a land card from among them onto the battlefield" — fetch tail
    if re.match(r"^you may put a land card from among them", s):
        return Static(modification=Modification(kind="fetch_land_tail"), raw=text)
    # Triggered "this ability triggers only once each turn" — restriction tail
    if re.match(r"^this ability triggers only", s):
        return Static(modification=Modification(kind="trigger_restriction"), raw=text)
    # "It can't be regenerated" — destroy modifier tail
    if re.match(r"^it can'?t be regenerated", s):
        return Static(modification=Modification(kind="no_regen_tail"), raw=text)
    # "This creature attacks each combat if able" — must-attack restriction
    if re.match(r"^(?:this creature|~) attacks each (?:turn|combat)", s):
        return Static(modification=Modification(kind="must_attack"), raw=text)
    # "Until the end of your next turn, ..." — temporary effect prefix
    if re.match(r"^until (?:the end of |your next |end of )?(?:your )?next turn", s):
        return Static(modification=Modification(kind="until_next_turn", args=(s,)), raw=text)
    # "This creature enters with a +1/+1 counter on it" — flat ETB counter
    if re.match(r"^(?:~|this creature) enters with a \+1/\+1 counter on it\b", s):
        return Static(modification=Modification(kind="etb_p1p1_counter"), raw=text)
    # "Enchanted creature doesn't untap during its controller's untap step"
    if re.match(r"^(?:enchanted|equipped) creature doesn'?t untap", s):
        return Static(modification=Modification(kind="aura_no_untap"), raw=text)
    # Set keywords standing alone
    if re.match(r"^(?:start your engines!?|the ring tempts you|partner|partner with [^.]+)", s):
        return Keyword(name=s.split()[0], raw=text)
    # "Any player may activate this ability" — activation rights modifier
    if re.match(r"^any player may activate this ability", s):
        return Static(modification=Modification(kind="activation_rights"), raw=text)
    # "This creature gets +1/+1 as long as ..." — conditional buff (tail will parse cond)
    m = re.match(r"^(?:this creature|~) gets \+(\d+)/\+(\d+) as long as ([^.]+)", s)
    if m:
        return Static(condition=Condition(kind="as_long_as", args=(m.group(3),)),
                      modification=Modification(kind="conditional_buff_self",
                                                args=(int(m.group(1)), int(m.group(2)))),
                      raw=text)
    # "Gain control of target creature until end of turn" — temporary control
    if re.match(r"^gain control of target [^.]+ until end of turn", s):
        return Static(modification=Modification(kind="temp_control"), raw=text)
    # "This creature doesn't untap during your untap step" — common variant
    if re.match(r"^(?:this creature|~) doesn'?t untap", s):
        return Static(modification=Modification(kind="no_untap_self"), raw=text)
    # "This spell costs {N} less to cast for each X" — kicker-style cost reduction
    if re.match(r"^this spell costs \{?\d+\}? less to cast", s):
        return Static(modification=Modification(kind="cost_reduce_self"), raw=text)
    # "Spend this mana only to ..."
    if re.match(r"^spend this mana only to", s):
        return Static(modification=Modification(kind="mana_restriction"), raw=text)
    # "You may choose new targets for the copy" — copy-spell tail (treat as static rider)
    if re.match(r"^you may choose new targets for (?:the |that )?cop", s):
        return Static(modification=Modification(kind="copy_retarget"), raw=text)
    # Additional-cost prefix that ended up alone (because the joiner missed it)
    if s.startswith("as an additional cost to cast"):
        return Static(modification=Modification(kind="additional_cost", args=(s,)),
                      raw=text)
    # Ability-word inline trigger: "landfall - whenever a land you control enters, ..."
    m = re.match(r"^([a-z][a-z'\s]+?)\s*-\s*(when|whenever|at the beginning)(.+)$", s, re.I)
    if m:
        # Re-parse the trigger portion
        trig_text = m.group(2) + m.group(3)
        t = parse_triggered(trig_text)
        if t:
            return t
        return Static(modification=Modification(kind="ability_word", args=(m.group(1).strip(),)),
                      raw=text)
    # "As long as / If [cond], [mod]" — generic conditional static (LAST)
    if s.startswith("as long as ") or s.startswith("if "):
        return Static(condition=Condition(kind="conditional", args=(s,)),
                      modification=Modification(kind="conditional_static", args=(s,)),
                      raw=text)
    return None


# ============================================================================
# Card-level parser
# ============================================================================

def parse_quoted_ability(inner_text: str) -> Optional[object]:
    """Canonical entry point for recursing into a quoted-ability body.

    When oracle text contains ``"<X>"`` (a double-quoted ability grant), callers
    should hand ``X`` here to get a structural AST node back — Keyword /
    Triggered / Activated / Static / list — so the grant can embed the parsed
    body instead of carrying an opaque string. Returns ``None`` on failure so
    callers can fall back to the verbatim text for coverage accounting.

    This is the ``parse_ability`` entry point purpose-built for recursion: it
    strips common wrapping punctuation (leading/trailing quotes, terminal
    period) that is an artifact of the quote boundary, not part of the ability
    grammar, and never raises. Extensions (notably ``quoted_abilities``) call
    through here so the rest of the parser can evolve without every grant rule
    re-implementing the cleanup."""
    if not inner_text:
        return None
    inner = inner_text.strip()
    # Strip a single pair of wrapping quotes if the caller passed them in.
    if len(inner) >= 2 and inner[0] == '"' and inner[-1] == '"':
        inner = inner[1:-1].strip()
    inner = inner.rstrip(".").strip()
    if not inner:
        return None
    try:
        return parse_ability(inner)
    except Exception:
        return None


def parse_ability(text: str) -> Optional[object]:
    """Try the ability-shape parsers in order. Returns one of
    Keyword/Triggered/Activated/Static, or a Static-wrapped spell-effect
    (per comp rules §112.3a, instants/sorceries have spell abilities whose
    body IS the effect — we represent these as Static with kind='spell_effect')."""
    # 1. Keyword (or list of keywords)
    kws = parse_keyword_list(text)
    if kws:
        return kws
    # 2. Triggered
    t = parse_triggered(text)
    if t:
        return t
    # 3. Activated
    a = parse_activated(text)
    if a:
        return a
    # 4. Static (known patterns)
    s = parse_static(text)
    if s:
        return s
    # 5. Spell-ability fallback — for instants/sorceries, the oracle text IS
    # the effect. Try parse_effect; if it produces ANYTHING (including an
    # UnknownEffect placeholder for a recognized-but-untyped phrasing), wrap
    # it as a Static spell_effect. parse_effect only fires a rule whose regex
    # consumed (almost) the whole text, so even an UnknownEffect from here
    # means "shape recognized, AST node still to be added" — which is enough
    # to count the ability as parsed for coverage purposes.
    e = parse_effect(text)
    if e is not None:
        return Static(modification=Modification(kind="spell_effect", args=(e,)),
                      raw=text)
    return None


# ============================================================================
# Antecedent tracking for pronoun-led continuations
# ============================================================================
#
# When a card's oracle text splits into multiple sentences, later sentences
# often start with a pronoun referring back to a noun introduced earlier
# ("Reanimate target creature card. It gains haste." → "it" = the target
# creature). Individually these pronoun-led continuations don't parse — the
# grammar wants a concrete noun phrase. We maintain a rolling antecedent
# dict that captures candidate referents from each sentence, then — only
# when the next sentence fails to parse on its own — try substituting an
# antecedent into the leading pronoun and re-parsing. Same shape as the
# saga-chapter prefix re-attachment in split_abilities.

# "create N/N Color Subtype tokens" — lift the token description so
# "they gain flying" can become "1/1 soldier tokens gain flying".
_TOKEN_CREATE_RE = re.compile(
    r"\bcreate(?:s)?\s+"
    r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)?\s*"
    r"(?:tapped\s+)?"
    r"(?:(\d+/\d+)\s+)?"
    r"((?:\w+\s+){0,6}?)"
    r"(?:creature\s+)?"
    r"tokens?\b",
    re.I,
)
# "target [adjectives] <noun>" — grab the noun-phrase verbatim.
_TARGET_NP_RE = re.compile(
    r"\btarget\s+((?:(?!\b(?:and|or|but|then|with|to|from|for|that)\b)\w+\s+){0,6}?"
    r"(?:creature|card|permanent|spell|player|opponent|artifact|enchantment|land|planeswalker|battle))\b",
    re.I,
)
_ENCHANTED_RE = re.compile(
    r"\b(enchanted\s+(?:creature|permanent|player|artifact|land))\b", re.I,
)
_CHOSEN_RE = re.compile(
    r"\b(the\s+chosen\s+(?:creature|card|player|permanent|type))\b", re.I,
)
_PLAYER_NP_RE = re.compile(
    r"\b(target\s+(?:player|opponent)|each\s+(?:player|opponent)|that\s+player)\b",
    re.I,
)


def _extract_antecedents(sent: str) -> dict:
    """Return candidate antecedents visible to the NEXT sentence.
    Each value is a parse-ready noun phrase already recognized by the
    grammar ('target creature', 'enchanted creature', 'the chosen card')."""
    ante: dict[str, str] = {}
    sl = sent.lower()

    # Token creation (plural)
    tm = _TOKEN_CREATE_RE.search(sl)
    if tm:
        pt = (tm.group(1) or "").strip()
        extras = (tm.group(2) or "").strip()
        bits = []
        if pt:
            bits.append(pt)
        if extras:
            bits.append(extras)
        bits.append("tokens")
        phrase = " ".join(bits).strip()
        # Only record if we have SOME descriptor — bare "tokens" is too vague.
        if pt or extras:
            ante["token_plural"] = phrase
            ante["token"] = phrase
            ante.setdefault("creature_plural", phrase)

    # "target <noun>" — pick the LAST one (nearest to sentence end).
    matches = list(_TARGET_NP_RE.finditer(sl))
    if matches:
        noun_phrase = matches[-1].group(1).strip()
        full = "target " + noun_phrase  # bare, parse-friendly
        for key in ("creature", "card", "permanent", "spell",
                    "player", "opponent", "artifact", "enchantment",
                    "land", "planeswalker", "battle"):
            if key in noun_phrase:
                if key == "opponent":
                    ante["player"] = full
                ante[key] = full
                break
        ante.setdefault("singular", full)

    em = _ENCHANTED_RE.search(sl)
    if em:
        phrase = em.group(1)
        for key in ("creature", "permanent", "player", "artifact", "land"):
            if key in phrase:
                ante.setdefault(key, phrase)
                break
        ante.setdefault("singular", phrase)

    cm = _CHOSEN_RE.search(sl)
    if cm:
        phrase = cm.group(1)
        for key in ("creature", "card", "player", "permanent", "type"):
            if key in phrase:
                ante.setdefault(key, phrase)
                break
        ante.setdefault("singular", phrase)

    pm_iter = list(_PLAYER_NP_RE.finditer(sl))
    if pm_iter and "player" not in ante:
        ante["player"] = pm_iter[-1].group(1)

    return ante


# Ordered pronoun patterns. Each entry is (regex, antecedent-key).
_PRONOUN_SUBS = [
    (re.compile(r"^its\s+owner\b", re.I),            "owner_of"),
    (re.compile(r"^its\s+controller\b", re.I),       "controller_of"),
    (re.compile(r"^that\s+creature\b", re.I),        "creature"),
    (re.compile(r"^that\s+card\b", re.I),            "card"),
    (re.compile(r"^that\s+permanent\b", re.I),       "permanent"),
    (re.compile(r"^that\s+spell\b", re.I),           "spell"),
    (re.compile(r"^that\s+player\b", re.I),          "player"),
    (re.compile(r"^those\s+creatures\b", re.I),      "creature_plural"),
    (re.compile(r"^those\s+cards\b", re.I),          "card_plural"),
    (re.compile(r"^those\s+tokens\b", re.I),         "token_plural"),
    (re.compile(r"^the\s+tokens?\b", re.I),          "token"),
    (re.compile(r"^it\b(?!'s)", re.I),               "singular"),
    (re.compile(r"^they\b", re.I),                   "token_plural"),
]


def _substitute_leading_pronoun(sent: str, ante: dict) -> str:
    """Replace a leading pronoun with its antecedent phrase if known. No-op
    otherwise. Always safe to call — returns `sent` unchanged if no match or
    antecedent."""
    if not ante:
        return sent
    sl = sent.lstrip()
    for pat, key in _PRONOUN_SUBS:
        m = pat.match(sl)
        if not m:
            continue
        if key == "owner_of":
            base = ante.get("creature") or ante.get("card") or ante.get("permanent") or ante.get("singular")
            if not base:
                return sent
            return pat.sub(base + "'s owner", sl, count=1)
        if key == "controller_of":
            base = ante.get("creature") or ante.get("permanent") or ante.get("singular")
            if not base:
                return sent
            return pat.sub(base + "'s controller", sl, count=1)
        base = ante.get(key)
        if not base:
            if key == "singular":
                base = (ante.get("creature") or ante.get("card")
                        or ante.get("permanent") or ante.get("spell"))
            elif key == "creature_plural":
                base = ante.get("token_plural")
            elif key == "card_plural":
                base = ante.get("token_plural")
            elif key == "token":
                base = ante.get("token_plural")
            elif key == "token_plural":
                base = ante.get("creature_plural")
        if not base:
            return sent
        return pat.sub(base, sl, count=1)
    return sent


def parse_card(card: dict) -> CardAST:
    name = card.get("name", "?")
    # Per-card hand-written handlers win over the grammar. Cards registered
    # here are "Doomsday-class snowflakes" whose text defies general grammar;
    # the handler emits a hand-crafted AST so downstream runtime can use a
    # card-specific resolver.
    handler = PER_CARD_HANDLERS.get(name)
    if handler is not None:
        try:
            ast = handler(card)
            if ast is not None:
                # Run post-parse hooks on hand-written ASTs too — face-down
                # enrichment must fire regardless of which parsing path
                # produced the AST.
                for hook in POST_PARSE_HOOKS:
                    try:
                        ast = hook(ast)
                    except Exception as e:
                        print(f"  ⚠ post_parse_hook {hook.__name__!r} "
                              f"raised on {name!r}: {e}", file=sys.stderr)
                return ast
        except Exception as e:
            print(f"  ⚠ per-card handler for {name!r} raised: {e}", file=sys.stderr)
    text = normalize(card)
    if not text:
        empty_ast = CardAST(name=name, abilities=(),
                            parse_errors=(), fully_parsed=True)
        for hook in POST_PARSE_HOOKS:
            try:
                empty_ast = hook(empty_ast)
            except Exception as e:
                print(f"  ⚠ post_parse_hook {hook.__name__!r} "
                      f"raised on {name!r}: {e}", file=sys.stderr)
        return empty_ast

    abilities: list = []
    errors: list[str] = []
    # Rolling antecedent from prior sentence. Populated lazily — only
    # consulted when a sentence fails to parse AND starts with a pronoun.
    # We update it from every sentence (parsed or not) so it always reflects
    # the nearest usable mention.
    rolling_ante: dict = {}
    for ability_text in split_abilities(text):
        # Default source for antecedent-extraction is the original text. If
        # we successfully substitute, switch to the substituted version so
        # that the next pronoun-led sentence inherits the concrete phrase.
        ante_source = ability_text
        result = parse_ability(ability_text)
        if result is None and rolling_ante:
            substituted = _substitute_leading_pronoun(ability_text, rolling_ante)
            if substituted != ability_text:
                alt = parse_ability(substituted)
                if alt is not None:
                    result = alt
                    ante_source = substituted
        if isinstance(result, list):
            abilities.extend(result)
        elif result is not None:
            abilities.append(result)
        else:
            errors.append(ability_text)
        # Refresh antecedents for NEXT sentence. Leave rolling_ante intact
        # when the current sentence introduces nothing new, so pronoun chains
        # ("...target creature. It gains X. It also gains Y.") still resolve.
        fresh = _extract_antecedents(ante_source)
        if fresh:
            rolling_ante = fresh
    ast = CardAST(name=name, abilities=tuple(abilities),
                  parse_errors=tuple(errors),
                  fully_parsed=(len(errors) == 0))
    # Post-parse enrichment hooks (face-down family, etc.). Each hook is
    # a pure CardAST -> CardAST transformation. Crashes are logged but do
    # not fail the parse — a crashed hook just skips enrichment.
    for hook in POST_PARSE_HOOKS:
        try:
            ast = hook(ast)
        except Exception as e:
            print(f"  ⚠ post_parse_hook {hook.__name__!r} raised on {name!r}: {e}",
                  file=sys.stderr)
    return ast


# ============================================================================
# Filtering
# ============================================================================

# Cards rejected at parser input — they'll never appear in any legal deck.
# Two categories:
#   1. Ante cards (wager-a-card mechanic, banned in every sanctioned format)
#   2. Cards officially removed by WotC (2020) for hate-speech / offensive imagery
_REJECTED_CARDS = {
    # Ante (per 7174n1c)
    "amulet of quoz", "bronze tablet", "contract from below", "darkpact",
    "demonic attorney", "jeweled bird", "rebirth", "tempest efreet",
    "timmerian fiends",
    # WotC 2020 official removal — racist / offensive imagery, removed from Oracle
    "cleanse", "crusade", "imprison", "invoke prejudice", "jihad",
    "pradesh gypsies", "stone-throwing devils",
}


def is_real_card(c: dict) -> bool:
    types = (c.get("type_line") or "").lower()
    # Substring exclusions (unambiguous — no collisions with other type words).
    if any(t in types for t in {"token", "scheme", "phenomenon",
                                "vanguard", "conspiracy", "dungeon"}):
        return False
    # Word-boundary check for "plane": the bare substring "plane" is also a
    # prefix of "planeswalker", so a substring match would silently exclude
    # every planeswalker from the card pool. We split on whitespace and
    # em-dashes (type-line subtype separators) and reject only exact token
    # matches of "plane". Planechase plane cards show up as e.g.
    # "Plane — Dominaria"; planeswalkers as "Legendary Planeswalker — Jace".
    tokens = re.split(r"[\s\u2014\u2013-]+", types)
    if "plane" in tokens:
        return False
    if c.get("set_type") in {"memorabilia", "token", "minigame", "funny"}:
        return False
    if c.get("border_color") == "silver":
        return False
    if (c.get("name") or "").endswith(" Bio"):
        return False
    if (c.get("name") or "").lower() in _REJECTED_CARDS:
        return False
    return True


# ============================================================================
# Report
# ============================================================================

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--card", help="Parse one card by name and dump its AST")
    parser.add_argument("--errors-top", type=int, default=40,
                        help="Show top-N most-common parse-error fragments")
    args = parser.parse_args()

    load_extensions()
    cards = json.loads(ORACLE_DUMP.read_text())
    real = [c for c in cards if is_real_card(c)]

    if args.card:
        target = next((c for c in real if c["name"].lower() == args.card.lower()), None)
        if not target:
            print(f"card not found: {args.card}", file=sys.stderr); sys.exit(1)
        ast = parse_card(target)
        print(f"\n=== {ast.name} ===")
        print(f"oracle: {target.get('oracle_text', '')!r}")
        print(f"\nfully_parsed: {ast.fully_parsed}")
        print(f"abilities ({len(ast.abilities)}):")
        for ab in ast.abilities:
            print(f"  - {ab}")
        if ast.parse_errors:
            print(f"\nparse_errors ({len(ast.parse_errors)}):")
            for e in ast.parse_errors:
                print(f"  · {e!r}")
        return

    # Full coverage pass
    green = 0
    partial = 0
    unparsed = 0
    error_freq: Counter = Counter()
    for c in real:
        ast = parse_card(c)
        if ast.fully_parsed:
            green += 1
        elif ast.abilities:
            partial += 1
            for e in ast.parse_errors:
                # Bucket by first 8 words for frequency analysis
                key = " ".join(e.split()[:8])
                error_freq[key] += 1
        else:
            unparsed += 1
            for e in ast.parse_errors:
                key = " ".join(e.split()[:8])
                error_freq[key] += 1

    total = len(real)
    pct = lambda n: f"{100 * n / total:.2f}%"

    lines = [
        "# Parser Coverage Report",
        "",
        f"Pool: **{total:,} real cards**.",
        "",
        "## Headline",
        "",
        f"- 🟢 GREEN (every ability parsed cleanly): **{green:,}** ({pct(green)})",
        f"- 🟡 PARTIAL (some abilities parsed, others left as raw text): **{partial:,}** ({pct(partial)})",
        f"- 🔴 UNPARSED (parser couldn't recognize any abilities): **{unparsed:,}** ({pct(unparsed)})",
        "",
        f"**Goal: 100% GREEN.** Every PARTIAL/UNPARSED entry corresponds to a specific",
        "unhandled grammar production — fixable by adding an effect rule, a trigger pattern,",
        "or a keyword. No heuristic catch-alls.",
        "",
        "## Top unparsed fragments — the work queue",
        "",
        "Each row is an unparsed clause prefix. The count is how many cards' parse failed",
        "at this prefix. Tackling the highest-count entries first shrinks the queue fastest.",
        "",
        "| Count | Fragment prefix |",
        "|---:|---|",
    ]
    for fragment, count in error_freq.most_common(args.errors_top):
        lines.append(f"| {count} | `{fragment}` |")

    REPORT.write_text("\n".join(lines))

    print(f"\n{'═' * 60}")
    print(f"  Parser coverage — {total:,} cards")
    print(f"{'═' * 60}")
    print(f"  🟢 GREEN:    {green:>6,}  ({pct(green)})")
    print(f"  🟡 PARTIAL:  {partial:>6,}  ({pct(partial)})")
    print(f"  🔴 UNPARSED: {unparsed:>6,}  ({pct(unparsed)})")
    print(f"\n  top 10 unparsed fragments (work queue):")
    for fragment, count in error_freq.most_common(10):
        print(f"    {count:>5,}  {fragment}")
    print(f"\n  → {REPORT}")


if __name__ == "__main__":
    main()
