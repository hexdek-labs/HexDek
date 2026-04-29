#!/usr/bin/env python3
"""Static-analysis combo / infinite-loop detector for the mtgsquad card pool.

Heuristic scanner over the parser AST. Surfaces CANDIDATES for human review;
not a full solver. Pulls six families of suspect cards and one simple 2-card
pair detector on top.

Families:
  1. MANA_POSITIVE   — activated abilities that produce more mana than they cost
  2. UNTAP_TRIGGER   — static/triggered abilities that untap a permanent
  3. STORM_ENGINE    — triggered abilities that fire on spell cast (any/filtered)
  4. ITERATIVE_DRAW  — triggered on draw/etb whose effect draws a card
  5. DOUBLER         — static replacements that double a resource (tokens, counters, mana)
  6. UNTAP_ACTIVATED — activated abilities whose effect untaps a permanent
                      (the "other half" of the 2-card pair search)

Output:
  - data/rules/combo_detections.md     (human-readable tables)
  - data/rules/combo_detections.json   (programmatic export)
  - stdout: top 20 highest-confidence 2-card infinite combos

Usage:
  python3 scripts/combo_detector.py

Read-only: does not modify parser.py, mtg_ast.py, extensions/, or tests.
"""

from __future__ import annotations

import json
import re
import sys
from collections import defaultdict
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Optional

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = Path(__file__).resolve().parent
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
OUT_MD = ROOT / "data" / "rules" / "combo_detections.md"
OUT_JSON = ROOT / "data" / "rules" / "combo_detections.json"

# Make sibling modules importable, exactly like parser.py does.
if str(SCRIPTS) not in sys.path:
    sys.path.insert(0, str(SCRIPTS))

from mtg_ast import (  # noqa: E402
    Activated, AddMana, Buff, CardAST, Choice, Conditional, CounterMod,
    CreateToken, Damage, Draw, Effect, Filter, GainLife, LoseLife, ManaCost,
    ManaSymbol, Modification, Optional_, Replacement, Sequence, Static,
    TapEffect, Triggered, Trigger, UntapEffect,
)
import parser as mtgparser  # noqa: E402


# ============================================================================
# AST walking helpers
# ============================================================================


def walk_effects(effect: Any):
    """Yield every EffectNode in an effect tree (pre-order)."""
    if effect is None:
        return
    yield effect
    if isinstance(effect, Sequence):
        for item in effect.items:
            yield from walk_effects(item)
    elif isinstance(effect, Choice):
        for opt in effect.options:
            yield from walk_effects(opt)
    elif isinstance(effect, Optional_):
        yield from walk_effects(effect.body)
    elif isinstance(effect, Conditional):
        yield from walk_effects(effect.body)
        if effect.else_body is not None:
            yield from walk_effects(effect.else_body)


def effect_has(effect: Any, *kinds: str) -> bool:
    for node in walk_effects(effect):
        if getattr(node, "kind", "") in kinds:
            return True
    return False


def mana_pool_size(add: AddMana) -> int:
    """Approximate mana produced by an AddMana node.

    Treats {X} as 1 (unknown but at least 1), generics count their pip value,
    'any color' contributions add via any_color_count.
    """
    total = add.any_color_count or 0
    for sym in add.pool or ():
        if sym.is_x:
            total += 1
        elif sym.generic:
            total += sym.generic
        else:
            # colored / snow / hybrid single symbol = 1 mana each
            total += 1
    return total


def is_any_color(add: AddMana) -> bool:
    """Does this AddMana produce mana of any color (or hybrid multi-color)?"""
    if add.any_color_count and add.any_color_count > 0:
        return True
    for sym in add.pool or ():
        if len(sym.color) >= 2:
            return True
    return False


def cost_cmc(cost) -> int:
    if cost is None or cost.mana is None:
        return 0
    return cost.mana.cmc


def cost_description(cost) -> str:
    if cost is None:
        return "free"
    parts = []
    if cost.mana is not None and cost.mana.symbols:
        parts.append("".join(s.raw for s in cost.mana.symbols))
    if cost.tap:
        parts.append("{T}")
    if cost.untap:
        parts.append("{Q}")
    if cost.sacrifice:
        parts.append(f"sac {cost.sacrifice.base}")
    if cost.discard:
        parts.append(f"discard {cost.discard}")
    if cost.pay_life:
        parts.append(f"pay {cost.pay_life} life")
    if cost.exile_self:
        parts.append("exile ~")
    return " + ".join(parts) or "free"


# ============================================================================
# Detection passes
# ============================================================================


@dataclass
class Detection:
    name: str
    category: str
    reason: str
    confidence: float = 0.5
    extras: dict = field(default_factory=dict)


# Regex fallbacks for cards that didn't fully parse (heuristic safety net).
_RE_SPELL_CAST_TRIGGER = re.compile(
    r"(?i)\bwhen(?:ever)? (?:you|a player|an opponent|each player)\s+cast(?:s)?"
)
_RE_STORM = re.compile(r"(?i)\bstorm\b")
_RE_DOUBLE_TOKEN = re.compile(
    r"(?i)(?:would create|create).{0,40}(?:twice that many|instead that player creates twice)"
)
_RE_DOUBLE_COUNTER = re.compile(
    r"(?i)(?:would have|have) (?:twice that many|that many plus)"
)
_RE_DOUBLE_MANA = re.compile(r"(?i)adds twice that much mana|adds that much mana plus")
_RE_UNTAP_TRIGGER_PHRASE = re.compile(r"(?i)\buntap (?:all|each|target) ")
_RE_UNTAP_LAND = re.compile(r"(?i)\buntap target (?:land|permanent|creature|artifact)")
_RE_DRAW_ON_CAST = re.compile(
    r"(?i)\bwhen(?:ever)?\b.{0,120}\b(?:cast|you draw|draws your)\b.{0,120}\bdraw(?: a)? card"
)


def detect_mana_positive(card: dict, ast: CardAST) -> Optional[Detection]:
    """Activated ability producing MORE mana than its cost, or {T}-only mana dork
    of 2+ any-color. Classic infinite-mana enablers.
    """
    best: Optional[tuple[int, int, str]] = None  # (surplus, produced, desc)
    for ab in ast.abilities:
        if not isinstance(ab, Activated):
            continue
        for node in walk_effects(ab.effect):
            if getattr(node, "kind", "") != "add_mana":
                continue
            produced = mana_pool_size(node)
            if produced == 0:
                continue
            paid = cost_cmc(ab.cost)
            # "Any color" / "any type" outputs are worth more because they
            # enable the output to re-pay the activation cost.
            any_color = is_any_color(node)
            # {T}-only (no mana cost) mana dorks producing 2+ any color = classic combo piece.
            tap_only = ab.cost.tap and paid == 0 and not ab.cost.sacrifice
            if tap_only and produced >= 2 and any_color:
                surplus = produced  # infinite-mana candidate
                desc = f"{cost_description(ab.cost)} → produces {produced} any-color"
                if best is None or surplus > best[0]:
                    best = (surplus, produced, desc)
                continue
            # Cost mana vs output mana imbalance.
            if produced > paid:
                surplus = produced - paid
                desc = f"{cost_description(ab.cost)} (cmc {paid}) → {produced} mana"
                if best is None or surplus > best[0]:
                    best = (surplus, produced, desc)
    if best is None:
        return None
    surplus, produced, desc = best
    confidence = 0.5
    if surplus >= 2:
        confidence = 0.85
    elif surplus == 1:
        confidence = 0.6
    return Detection(
        name=card["name"],
        category="mana_positive",
        reason=desc,
        confidence=confidence,
        extras={"surplus": surplus, "produced": produced},
    )


def detect_untap_trigger(card: dict, ast: CardAST) -> Optional[Detection]:
    """Triggered/Static abilities that UNTAP a permanent on a condition.
    Intruder Alarm, Seedborn Muse, Kinnan-style auras, etc.
    """
    for ab in ast.abilities:
        if isinstance(ab, Triggered):
            if effect_has(ab.effect, "untap"):
                return Detection(
                    name=card["name"],
                    category="untap_trigger",
                    reason=f"triggered on {ab.trigger.event} → untap",
                    confidence=0.8,
                    extras={"event": ab.trigger.event},
                )
        elif isinstance(ab, Static):
            # Static "at the beginning / each ... untap" is typically modeled
            # as a Triggered via phase events. Look into raw text as a fallback.
            if ab.raw and _RE_UNTAP_TRIGGER_PHRASE.search(ab.raw):
                return Detection(
                    name=card["name"],
                    category="untap_trigger",
                    reason="static ability referencing untap",
                    confidence=0.5,
                )
    # Regex fallback for PARTIAL-parsed cards.
    text = (card.get("oracle_text") or "").lower()
    if text and _RE_UNTAP_TRIGGER_PHRASE.search(text) and (
        "when" in text or "whenever" in text or "beginning" in text
    ):
        return Detection(
            name=card["name"],
            category="untap_trigger",
            reason="regex: untap clause inside a trigger",
            confidence=0.4,
        )
    return None


def detect_storm_engine(card: dict, ast: CardAST) -> Optional[Detection]:
    """Trigger-per-spell-cast abilities (excluding strict self-cast, though we
    include both since strict self-cast + cost-reducers still combo). Storm,
    Aetherflux Reservoir, Storm-Kiln Artist, etc.
    """
    cast_events = {"cast_any", "cast_filtered"}
    for ab in ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event in cast_events:
            return Detection(
                name=card["name"],
                category="storm_engine",
                reason=f"trigger on {ab.trigger.event}",
                confidence=0.7,
                extras={"event": ab.trigger.event},
            )
    # Keyword: Storm.
    text = (card.get("oracle_text") or "").lower()
    keywords = [k.lower() for k in (card.get("keywords") or [])]
    if "storm" in keywords or _RE_STORM.search(text):
        # Actual keyword-line match (avoid false-positive "storm crow" name).
        if re.search(r"(?im)^storm\b", text) or " storm (" in text or text.startswith("storm "):
            return Detection(
                name=card["name"],
                category="storm_engine",
                reason="has Storm keyword",
                confidence=0.9,
            )
    if text and _RE_SPELL_CAST_TRIGGER.search(text):
        return Detection(
            name=card["name"],
            category="storm_engine",
            reason="regex: trigger on cast",
            confidence=0.5,
        )
    return None


def detect_iterative_draw(card: dict, ast: CardAST) -> Optional[Detection]:
    """Trigger on draw/ETB whose effect draws a card (Niv-Mizzet Parun,
    Alhammarret's Archive + draw triggers, etc.). Also catches Psychosis
    Crawler-class ("deals damage on draw") which combos with Niv."""
    interesting_events = {"draw_event", "etb", "cast_any", "cast_filtered"}
    for ab in ast.abilities:
        if not isinstance(ab, Triggered):
            continue
        if ab.trigger.event not in interesting_events:
            continue
        if effect_has(ab.effect, "draw", "damage"):
            # Must actually draw, or deal damage that can chain to life-loss/draw engines.
            return Detection(
                name=card["name"],
                category="iterative_draw",
                reason=f"trigger on {ab.trigger.event} → draw/damage",
                confidence=0.65,
                extras={"event": ab.trigger.event},
            )
    text = (card.get("oracle_text") or "").lower()
    if text and _RE_DRAW_ON_CAST.search(text):
        return Detection(
            name=card["name"],
            category="iterative_draw",
            reason="regex: draw trigger on cast",
            confidence=0.45,
        )
    return None


def detect_doubler(card: dict, ast: CardAST) -> Optional[Detection]:
    """Static replacement effects that double a resource. The parser models many
    doublers imperfectly (replacement effects are §614 territory), so we lean
    heavily on raw text + keyword hints while using AST presence as a boost.
    """
    text = (card.get("oracle_text") or "").lower()
    if not text:
        return None
    matches: list[str] = []
    if _RE_DOUBLE_TOKEN.search(text):
        matches.append("tokens doubled")
    if _RE_DOUBLE_COUNTER.search(text):
        matches.append("counters doubled")
    if _RE_DOUBLE_MANA.search(text):
        matches.append("mana doubled")
    # Simple "X would, Y instead" replacement on resources.
    if "if " in text and "instead" in text:
        if "counter" in text and ("that many plus" in text or "twice" in text):
            matches.append("counter replacement")
        if "token" in text and ("twice" in text or "that many plus" in text):
            matches.append("token replacement")
    if not matches:
        return None
    conf = 0.85 if len(matches) >= 2 else 0.7
    # Boost if the parser identified a Replacement node.
    for ab in ast.abilities:
        if isinstance(ab, Static) and ab.modification and ab.modification.kind in {
            "replacement", "replacement_effect",
        }:
            conf = min(0.95, conf + 0.1)
            break
    return Detection(
        name=card["name"],
        category="doubler",
        reason=", ".join(matches),
        confidence=conf,
    )


def detect_untap_activated(card: dict, ast: CardAST) -> Optional[Detection]:
    """Activated abilities whose EFFECT is to untap a permanent. These are the
    'other half' of an untap-trigger combo and also pair with mana-positive
    rocks (Staff of Domination–class)."""
    for ab in ast.abilities:
        if not isinstance(ab, Activated):
            continue
        if effect_has(ab.effect, "untap"):
            return Detection(
                name=card["name"],
                category="untap_activated",
                reason=f"{cost_description(ab.cost)} → untap",
                confidence=0.7,
                extras={"cost_cmc": cost_cmc(ab.cost)},
            )
    return None


# ============================================================================
# 2-card pair inference
# ============================================================================


@dataclass
class Pair:
    a: str
    b: str
    reason: str
    confidence: float

    def key(self) -> tuple[str, str]:
        return tuple(sorted((self.a, self.b)))  # type: ignore[return-value]


def infer_pairs(detections_by_card: dict[str, list[Detection]]) -> list[Pair]:
    """Cheap 2-card pair heuristics (MVP — not a 3-card solver):

    A. Untap-trigger + mana-positive     → classic infinite mana
       (Intruder Alarm + any {T}: add {X}{X} creature,
        Seedborn Muse + mana-positive rocks/creatures, etc.)

    B. Untap-trigger + untap-activated   → infinite untap loops
       (rare, mostly covered by A, but included for mana-less engines)

    C. Storm engine + untap-trigger      → infinite storm count
       (any spell-cast trigger + an untap that refunds the cast cost)

    D. Iterative-draw + doubler          → infinite draw / mill / damage
       (Niv-Mizzet Parun + Thought Reflection-class doublers)

    E. Untap-activated creature + mana-positive creature (self-pair attempt)
       — when a single card has BOTH (Grim Monolith, Deserted Temple), skip;
       when two different cards have the halves, emit.
    """
    pairs: list[Pair] = []

    # Bucket cards by category for cheap cross-products.
    by_cat: dict[str, list[Detection]] = defaultdict(list)
    for dets in detections_by_card.values():
        for d in dets:
            by_cat[d.category].append(d)

    mana_positive = by_cat.get("mana_positive", [])
    untap_triggers = by_cat.get("untap_trigger", [])
    untap_activated = by_cat.get("untap_activated", [])
    storm_engines = by_cat.get("storm_engine", [])
    iterative_draws = by_cat.get("iterative_draw", [])
    doublers = by_cat.get("doubler", [])

    def _emit(a: str, b: str, reason: str, conf: float):
        if a == b:
            return
        pairs.append(Pair(a=a, b=b, reason=reason, confidence=conf))

    # (A) Untap-trigger × mana-positive — infinite mana candidate.
    # Prefer mana engines that produce "any color" (they can pay arbitrary activation costs).
    for mp in mana_positive:
        if mp.extras.get("surplus", 0) < 2:
            continue
        for ut in untap_triggers:
            if ut.name == mp.name:
                continue
            conf = min(0.95, 0.4 + 0.3 * mp.confidence + 0.3 * ut.confidence)
            _emit(
                ut.name, mp.name,
                f"untap-trigger ({ut.reason}) refunds mana engine ({mp.reason})",
                conf,
            )

    # (B) Untap-trigger × untap-activated — untap loops.
    for ua in untap_activated:
        for ut in untap_triggers:
            if ua.name == ut.name:
                continue
            conf = min(0.8, 0.35 + 0.25 * ua.confidence + 0.25 * ut.confidence)
            _emit(
                ut.name, ua.name,
                f"untap-trigger ({ut.reason}) chains with untap-activated ({ua.reason})",
                conf,
            )

    # (C) Storm engine × untap-trigger.
    for se in storm_engines:
        for ut in untap_triggers:
            if se.name == ut.name:
                continue
            conf = min(0.75, 0.3 + 0.25 * se.confidence + 0.25 * ut.confidence)
            _emit(
                se.name, ut.name,
                f"spell-cast trigger ({se.reason}) + untap refund ({ut.reason})",
                conf,
            )

    # (D) Iterative-draw × doubler.
    for idr in iterative_draws:
        for db in doublers:
            if idr.name == db.name:
                continue
            conf = min(0.7, 0.25 + 0.25 * idr.confidence + 0.25 * db.confidence)
            _emit(
                idr.name, db.name,
                f"draw-trigger ({idr.reason}) doubled by ({db.reason})",
                conf,
            )

    # Dedup (a,b) unordered.
    seen: dict[tuple, Pair] = {}
    for p in pairs:
        k = p.key()
        if k not in seen or p.confidence > seen[k].confidence:
            seen[k] = p
    return sorted(seen.values(), key=lambda p: -p.confidence)


# ============================================================================
# Main
# ============================================================================


CATEGORY_ORDER = [
    "mana_positive",
    "untap_trigger",
    "untap_activated",
    "storm_engine",
    "iterative_draw",
    "doubler",
]

CATEGORY_TITLES = {
    "mana_positive":   "Mana-positive engines (infinite-mana candidates)",
    "untap_trigger":   "Untap triggers",
    "untap_activated": "Untap-on-activation (pair fuel)",
    "storm_engine":    "Storm engines (trigger-per-cast)",
    "iterative_draw":  "Iterative draw engines",
    "doubler":         "Doublers (replacement effects on resources)",
}


def main() -> int:
    mtgparser.load_extensions()
    cards = json.loads(ORACLE_DUMP.read_text())
    real = [c for c in cards if mtgparser.is_real_card(c)]
    # Deduplicate by name (oracle dump is already deduped but be defensive).
    by_name: dict[str, dict] = {}
    for c in real:
        by_name.setdefault(c["name"], c)

    detections_by_card: dict[str, list[Detection]] = defaultdict(list)
    passes = [
        detect_mana_positive,
        detect_untap_trigger,
        detect_untap_activated,
        detect_storm_engine,
        detect_iterative_draw,
        detect_doubler,
    ]

    parsed_errors = 0
    for i, (name, card) in enumerate(by_name.items()):
        try:
            ast = mtgparser.parse_card(card)
        except Exception:
            parsed_errors += 1
            continue
        for fn in passes:
            try:
                det = fn(card, ast)
            except Exception:
                det = None
            if det is not None:
                detections_by_card[name].append(det)

    # Aggregate per category.
    per_cat: dict[str, list[Detection]] = defaultdict(list)
    for dets in detections_by_card.values():
        for d in dets:
            per_cat[d.category].append(d)
    for cat in per_cat:
        per_cat[cat].sort(key=lambda d: (-d.confidence, d.name))

    # 2-card pairs.
    pairs = infer_pairs(detections_by_card)

    # ---- JSON export ----
    # Pairs cross-product can explode (hundreds of thousands). We cap the
    # export at the top-K by confidence; the full count still appears in stats
    # so callers know how much was pruned.
    PAIRS_EXPORT_CAP = 5000
    json_payload = {
        "stats": {
            "total_cards_scanned": len(by_name),
            "parse_errors": parsed_errors,
            "categories": {cat: len(per_cat.get(cat, [])) for cat in CATEGORY_ORDER},
            "pairs_found_total": len(pairs),
            "pairs_exported": min(len(pairs), PAIRS_EXPORT_CAP),
        },
        "categories": {
            cat: [asdict(d) for d in per_cat.get(cat, [])]
            for cat in CATEGORY_ORDER
        },
        "pairs": [asdict(p) for p in pairs[:PAIRS_EXPORT_CAP]],
    }
    OUT_JSON.write_text(json.dumps(json_payload, indent=2, sort_keys=False))

    # ---- Markdown report ----
    lines: list[str] = []
    lines += [
        "# Combo Detections",
        "",
        "Static-analysis scan of the card pool for common infinite / engine patterns.",
        "Heuristic, NOT a solver — every entry is a candidate for human review.",
        "",
        "## Summary",
        "",
        f"- Cards scanned: **{len(by_name):,}**",
        f"- Parse errors (skipped): **{parsed_errors:,}**",
        f"- 2-card pair candidates: **{len(pairs):,}**",
        "",
        "| Category | Count |",
        "|---|---:|",
    ]
    for cat in CATEGORY_ORDER:
        lines.append(f"| {CATEGORY_TITLES[cat]} | {len(per_cat.get(cat, [])):,} |")
    lines.append("")

    for cat in CATEGORY_ORDER:
        bucket = per_cat.get(cat, [])
        lines += [
            f"## {CATEGORY_TITLES[cat]} ({len(bucket):,})",
            "",
            "| Card | Confidence | Reason |",
            "|---|---:|---|",
        ]
        for d in bucket[:200]:  # cap each section to keep the file readable
            reason = d.reason.replace("|", "\\|")
            lines.append(f"| {d.name} | {d.confidence:.2f} | {reason} |")
        if len(bucket) > 200:
            lines.append(f"| … | | +{len(bucket) - 200} more in JSON |")
        lines.append("")

    lines += [
        "## Top 2-card combo pairs",
        "",
        "Cross-product of detection buckets. Confidence is a coarse blend of",
        "each side's individual confidence — not a solver score.",
        "",
        "| # | Card A | Card B | Confidence | Reason |",
        "|---:|---|---|---:|---|",
    ]
    for i, p in enumerate(pairs[:100], 1):
        reason = p.reason.replace("|", "\\|")
        lines.append(f"| {i} | {p.a} | {p.b} | {p.confidence:.2f} | {reason} |")
    if len(pairs) > 100:
        lines.append(f"| … | | | | +{len(pairs) - 100} more in JSON |")
    lines.append("")

    OUT_MD.write_text("\n".join(lines))

    # ---- stdout summary ----
    print("═" * 60)
    print("  Combo Detector")
    print("═" * 60)
    print(f"  cards scanned: {len(by_name):,}")
    print(f"  parse errors:  {parsed_errors:,}")
    print()
    print("  category counts:")
    for cat in CATEGORY_ORDER:
        print(f"    {CATEGORY_TITLES[cat]:<52} {len(per_cat.get(cat, [])):>6,}")
    print()
    print(f"  pair candidates: {len(pairs):,}")
    print()
    print("  top 20 2-card infinite-combo candidates:")
    print("  " + "-" * 56)
    for i, p in enumerate(pairs[:20], 1):
        print(f"  {i:>2}. [{p.confidence:.2f}] {p.a}  +  {p.b}")
        print(f"       {p.reason}")
    print()
    print(f"  → {OUT_MD}")
    print(f"  → {OUT_JSON}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
