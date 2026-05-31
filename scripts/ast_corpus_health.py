#!/usr/bin/env python3
"""AST corpus health report.

Walks ``data/rules/ast_dataset.jsonl`` and emits a per-mechanic coverage
+ parse-confidence + top-10-fallback-pattern report. The report is
written to ``docs/ast-corpus-health-r60.md`` (or whatever path is passed
to ``--output``).

"Fallback" is a structural signal: the parser identified an ability
node but couldn't structure its semantic payload. Concretely:

- Modification.kind in FALLBACK_MOD_KINDS (parsed_effect_residual,
  parsed_tail, untyped_effect, if_intervening_tail, custom,
  cast_trigger_tail) — the parser noted "this is a modification" but
  fell back to a raw/typed-residual representation.
- Triggered.effect.kind in FALLBACK_EFFECT_KINDS (parsed_effect_residual,
  untyped_effect, cast_trigger_tail, plus the bare "conditional" wrapper).
- Condition.kind in FALLBACK_COND_KINDS (if, conditional, raw,
  intervening_if, as_long_as) — same set the era scaffold audits use.

A higher non-fallback share means the parser produced richer semantic
output. Mechanics with low non-fallback shares are the parser's next
improvement targets — the report quantifies that.
"""

from __future__ import annotations

import argparse
import json
import re
from collections import Counter, defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
DEFAULT_OUT = ROOT / "docs" / "ast-corpus-health-r60.md"

FALLBACK_MOD_KINDS = {
    "parsed_effect_residual", "parsed_tail", "untyped_effect",
    "if_intervening_tail", "custom", "cast_trigger_tail",
}
FALLBACK_EFFECT_KINDS = {
    "parsed_effect_residual", "untyped_effect", "cast_trigger_tail",
    "conditional",  # bare wrapper — parser saw "if X, then Y" but didn't unbox Y
}
FALLBACK_COND_KINDS = {
    "if", "conditional", "raw", "intervening_if", "as_long_as",
}

# Mechanic → oracle-text substring(s) that flag a card as using it.
# Order: keyword-action / ability-word / per-era distinctive vocabulary.
# Lowercased oracle text is matched. Multiple substrings = OR.
MECHANICS = {
    # Evergreen / pre-Modern.
    "Flying":         ["flying"],
    "Trample":        ["trample"],
    "First strike":   ["first strike"],
    "Double strike":  ["double strike"],
    "Deathtouch":     ["deathtouch"],
    "Lifelink":       ["lifelink"],
    "Vigilance":      ["vigilance"],
    "Haste":          ["haste"],
    "Hexproof":       ["hexproof"],
    "Indestructible": ["indestructible"],
    "Menace":         ["menace"],
    "Reach":          ["reach"],
    "Defender":       ["defender"],
    "Flash":          ["flash"],
    # Resource-counter mechanics.
    "Counters":       ["+1/+1 counter", "-1/-1 counter", "charge counter", "loyalty counter"],
    # Action-cost / alt-cost.
    "Kicker":         ["kicker"],
    "Cycling":        ["cycling", "cycle"],
    "Madness":        ["madness"],
    "Flashback":      ["flashback"],
    "Buyback":        ["buyback"],
    "Suspend":        ["suspend"],
    "Bestow":         ["bestow"],
    "Emerge":         ["emerge"],
    "Surge":          ["surge"],
    "Escape":         ["escape from"],
    # Ability words / triggered families.
    "Landfall":       ["landfall"],
    "Morbid":         ["morbid"],
    "Ferocious":      ["ferocious"],
    "Revolt":         ["revolt"],
    "Delirium":       ["delirium"],
    "Metalcraft":     ["metalcraft"],
    "Constellation":  ["constellation"],
    "Heroic":         ["heroic"],
    "Spell mastery":  ["spell mastery"],
    "Domain":         ["domain"],
    "Threshold":      ["threshold"],
    "Hellbent":       ["hellbent"],
    "Ascend":         ["city's blessing", "ascend"],
    # Tribal / typed.
    "Equip":          ["equip "],
    "Aura":           [" aura ", "aura."],
    "Saga":           ["chapter ii"],  # disambiguate vs "saga" appearing in flavor
    # Era 2 (2015-2019).
    "Energy":         ["energy counter", "{e}"],
    "Crew":           ["crew "],
    "Vehicle":        [" vehicle"],
    "Partner":        ["partner"],
    "Adapt":          ["adapt"],
    "Amass":          ["amass"],
    "Afterlife":      ["afterlife"],
    "Spectacle":      ["spectacle"],
    "Riot":           ["riot"],
    "Eminence":       ["eminence"],
    # Era 3 (2020-2022).
    "Mutate":         ["mutate"],
    "Companion":      ["companion"],
    "Foretell":       ["foretell"],
    "Learn":          ["learn"],
    "Lesson":         [" lesson "],
    "Ward":           ["ward "],
    "Disturb":        ["disturb"],
    "Daybound":       ["daybound"],
    "Nightbound":     ["nightbound"],
    "Cleave":         ["cleave"],
    "Exploit":        ["exploit"],
    "Decayed":        ["decayed"],
    # Era 4 (2023-2026).
    "Discover":       ["discover "],
    "Descend":        ["descend "],
    "Battle":         ["battle —", "battle—", "siege"],
    "Prototype":      ["prototype"],
    "Craft":          ["craft with", "craft —"],
    "Role token":     ["role token"],
    "The Ring":       ["the ring tempts you", "ring-bearer"],
    "Bargain":        ["bargain"],
    "Celebration":    ["celebration"],
    "Commit a crime": ["commit a crime", "committed a crime"],
    "Collect evidence": ["collect evidence"],
    "Plot":           ["plot"],
    "Disguise":       ["disguise"],
    "Cloak":          ["cloak"],
    "Offspring":      ["offspring"],
    "Freerunning":    ["freerunning"],
    "Expend":         ["expend "],
    "Eerie":          ["eerie"],
    "Survival":       ["survival"],
    "Flurry":         ["flurry"],
    "Manifest dread": ["manifest dread"],
    "Warp":           ["warp"],
}


@dataclass
class CardStats:
    name: str
    oracle_text: str
    type_line: str
    abilities_total: int
    abilities_fallback: int
    fallback_kinds: Counter  # kind → count of fallbacks of that kind

    @property
    def fallback_share(self) -> float:
        if self.abilities_total == 0:
            return 0.0
        return self.abilities_fallback / self.abilities_total


def _is_fallback_modification(node: dict) -> bool:
    return (node.get("kind") or "") in FALLBACK_MOD_KINDS


def _is_fallback_effect(effect: dict | None) -> bool:
    if not effect:
        return True  # empty / null effect = fallback
    return (effect.get("kind") or "") in FALLBACK_EFFECT_KINDS


def _is_fallback_condition(cond: dict) -> bool:
    return (cond.get("kind") or "") in FALLBACK_COND_KINDS


def analyse_ability(a: dict) -> tuple[bool, str]:
    """Return (is_fallback, fallback_label).

    fallback_label is a stable string suitable for grouping; non-fallback
    abilities return ("", "").
    """
    t = a.get("__ast_type__") or ""
    if t == "Static":
        mod = a.get("modification") or {}
        if mod and _is_fallback_modification(mod):
            return True, f"Static/{mod.get('kind','?')}"
        return False, ""
    if t == "Triggered":
        eff = a.get("effect")
        if _is_fallback_effect(eff):
            return True, f"Triggered/{(eff or {}).get('kind','<empty>')}"
        return False, ""
    if t == "Activated":
        eff = a.get("effect") or {}
        # Activated effects are nested — look at effect's own kind. We
        # treat empty/None effect as fallback.
        if not eff:
            return True, "Activated/<empty>"
        # Activated effects can be structured (GrantAbility, etc.) or
        # raw — kind names ending in residual/raw/untyped are fallback.
        eff_kind = eff.get("kind") or ""
        eff_type = eff.get("__ast_type__") or ""
        if eff_kind in FALLBACK_EFFECT_KINDS:
            return True, f"Activated/{eff_kind}"
        # Untyped Modification under Activated also counts.
        if eff_type == "Modification" and (eff.get("kind") or "") in FALLBACK_MOD_KINDS:
            return True, f"Activated/{eff.get('kind')}"
        return False, ""
    if t == "Keyword":
        # Keyword abilities are by-design raw stubs — the keyword name is
        # the semantic payload. Don't count as fallback.
        return False, ""
    return False, ""


def walk_conditions(node, out: list[dict]) -> None:
    """Collect every Condition node in a nested AST blob."""
    if isinstance(node, dict):
        if node.get("__ast_type__") == "Condition":
            out.append(node)
        for v in node.values():
            walk_conditions(v, out)
    elif isinstance(node, list):
        for x in node:
            walk_conditions(x, out)


def analyse_card(row: dict) -> CardStats:
    ast = row.get("ast") or {}
    abilities = ast.get("abilities") or []
    total = len(abilities)
    fallback = 0
    kinds: Counter = Counter()
    for a in abilities:
        is_fb, label = analyse_ability(a)
        if is_fb:
            fallback += 1
            kinds[label] += 1
    # Conditions — count separately so cards with a structured trigger
    # but a raw-text condition still register as fallback-bearing.
    conds: list[dict] = []
    walk_conditions(ast, conds)
    for c in conds:
        if _is_fallback_condition(c):
            kinds[f"Condition/{c.get('kind','?')}"] += 1
    return CardStats(
        name=row.get("name", "?"),
        oracle_text=(row.get("oracle_text") or "").lower(),
        type_line=row.get("type_line") or "",
        abilities_total=total,
        abilities_fallback=fallback,
        fallback_kinds=kinds,
    )


def card_matches_mechanic(card: CardStats, needles: list[str]) -> bool:
    text = card.oracle_text
    return any(n in text for n in needles)


def render_report(
    cards: list[CardStats],
    top_patterns_limit: int = 10,
) -> str:
    total_cards = len(cards)
    cards_with_abilities = [c for c in cards if c.abilities_total > 0]
    total_abilities = sum(c.abilities_total for c in cards)
    total_fallback = sum(c.abilities_fallback for c in cards)
    nonfallback_pct = 100.0 * (total_abilities - total_fallback) / max(1, total_abilities)

    # --- Per-ability-kind aggregate ---
    kind_totals: Counter = Counter()
    for c in cards:
        kind_totals.update(c.fallback_kinds)

    # --- Parse-confidence histogram (bin by share of abilities that fell back) ---
    bins = [
        ("clean (0% fallback)",      lambda s: s == 0.0),
        ("low    (0-25%)",            lambda s: 0.0 < s <= 0.25),
        ("medium (25-50%)",          lambda s: 0.25 < s <= 0.50),
        ("high   (50-75%)",          lambda s: 0.50 < s <= 0.75),
        ("severe (75-100%)",         lambda s: s > 0.75),
    ]
    hist: dict[str, int] = {label: 0 for label, _ in bins}
    for c in cards_with_abilities:
        s = c.fallback_share
        for label, pred in bins:
            if pred(s):
                hist[label] += 1
                break

    # --- Per-mechanic coverage ---
    mechanic_rows: list[tuple[str, int, int, float]] = []
    for mech, needles in MECHANICS.items():
        matched = [c for c in cards_with_abilities if card_matches_mechanic(c, needles)]
        if not matched:
            mechanic_rows.append((mech, 0, 0, 0.0))
            continue
        m_abs = sum(c.abilities_total for c in matched)
        m_fb = sum(c.abilities_fallback for c in matched)
        coverage = 100.0 * (m_abs - m_fb) / max(1, m_abs)
        mechanic_rows.append((mech, len(matched), m_abs, coverage))
    # Sort: cards desc, then coverage asc (most cards × worst coverage first
    # = "biggest improvement opportunity").
    mechanic_rows.sort(key=lambda r: (-r[1], r[3]))

    # --- Top-10 fallback-prone oracle text patterns ---
    # Group cards by their dominant fallback kind, then for each kind take
    # the most-common 4-word substring in their oracle text. Crude but
    # effective at surfacing recurring phrases.
    by_kind: dict[str, list[CardStats]] = defaultdict(list)
    for c in cards:
        if not c.fallback_kinds:
            continue
        dominant_kind, _ = c.fallback_kinds.most_common(1)[0]
        by_kind[dominant_kind].append(c)

    top_kinds = sorted(by_kind.items(), key=lambda kv: -len(kv[1]))[:top_patterns_limit]
    pattern_rows: list[tuple[str, int, str, list[str]]] = []
    for kind, cs in top_kinds:
        # Sample 3 representative cards.
        sample_names = [c.name for c in cs[:3]]
        # Find the most-frequent 4-word ngram from card oracle texts.
        ngrams: Counter = Counter()
        for c in cs:
            tokens = re.findall(r"[a-z']+", c.oracle_text)
            for i in range(len(tokens) - 3):
                ng = " ".join(tokens[i:i+4])
                ngrams[ng] += 1
        top_ngram = ngrams.most_common(1)[0][0] if ngrams else "(no ngram)"
        pattern_rows.append((kind, len(cs), top_ngram, sample_names))

    # --- Compose report ---
    lines: list[str] = []
    lines.append("# AST corpus health report — r60")
    lines.append("")
    lines.append(f"**Dataset:** `data/rules/ast_dataset.jsonl`")
    lines.append(f"**Cards analysed:** {total_cards:,}")
    lines.append(f"**Cards with at least one ability:** {len(cards_with_abilities):,}")
    lines.append(f"**Total ability nodes:** {total_abilities:,}")
    lines.append(f"**Non-fallback share:** **{nonfallback_pct:.2f}%** ({total_abilities - total_fallback:,} structured / {total_fallback:,} fallback)")
    lines.append("")
    lines.append("Generated by `scripts/ast_corpus_health.py`. \"Fallback\" means the parser identified an ability node but couldn't structure its semantic payload — the higher the non-fallback share, the richer the AST. See the script docstring for the precise definition.")
    lines.append("")

    lines.append("## Parse-confidence histogram")
    lines.append("")
    lines.append("Per-card distribution: how many of the card's abilities fell back to raw/residual representation. \"Clean\" cards parsed end-to-end; \"severe\" cards landed in the residual bucket for most of their abilities.")
    lines.append("")
    lines.append("| Bucket | Cards | % of cards-with-abilities |")
    lines.append("|--------|-------|---------------------------|")
    total_aw = len(cards_with_abilities) or 1
    for label, _ in bins:
        n = hist[label]
        lines.append(f"| {label} | {n:,} | {100.0 * n / total_aw:.1f}% |")
    lines.append("")

    lines.append(f"## Per-mechanic coverage ({len(MECHANICS)} mechanics)")
    lines.append("")
    lines.append("For each mechanic, count cards whose oracle text contains a canonical substring for the mechanic, then compute the non-fallback share of all abilities on those cards. **Sorted by card-count descending, then coverage ascending** — so the rows at the top of each card-count band are the mechanics where parser improvements would touch the most cards AND have the largest semantic uplift.")
    lines.append("")
    lines.append("| Mechanic | Cards | Abilities | Non-fallback coverage |")
    lines.append("|----------|-------|-----------|-----------------------|")
    for mech, cards_n, abs_n, cov in mechanic_rows:
        if cards_n == 0:
            cov_str = "—"
            lines.append(f"| {mech} | 0 | 0 | {cov_str} |")
            continue
        cov_str = f"{cov:.1f}%"
        lines.append(f"| {mech} | {cards_n:,} | {abs_n:,} | {cov_str} |")
    lines.append("")

    lines.append(f"## Top-{top_patterns_limit} most fallback-prone parser kinds")
    lines.append("")
    lines.append("Grouped by the most common fallback `kind` that surfaces on each card (a card's \"dominant\" fallback). The top n-gram is the most-frequent 4-word substring shared by the cards in the bucket — useful for spotting which phrasings to target first when extending the parser.")
    lines.append("")
    lines.append("| Rank | Fallback kind | Cards | Sample oracle 4-gram | Example cards |")
    lines.append("|------|---------------|-------|----------------------|---------------|")
    for rank, (kind, n, ngram, examples) in enumerate(pattern_rows, 1):
        ex = ", ".join(examples)
        lines.append(f"| {rank} | `{kind}` | {n:,} | `{ngram}` | {ex} |")
    lines.append("")

    lines.append("## Aggregate fallback-kind histogram")
    lines.append("")
    lines.append("Total occurrence count across all cards, sorted descending. The fallback kinds that account for the most volume are the parser's largest single improvement targets.")
    lines.append("")
    lines.append("| Fallback kind | Occurrences |")
    lines.append("|---------------|-------------|")
    for kind, n in kind_totals.most_common(20):
        lines.append(f"| `{kind}` | {n:,} |")
    lines.append("")

    lines.append("## Reading guide")
    lines.append("")
    lines.append("- **Headline non-fallback share** tells you the overall parse quality.")
    lines.append("- **Mechanic coverage column** identifies WHERE the parser leaves the most semantic value on the floor. A low-coverage mechanic with many cards = biggest single improvement target.")
    lines.append("- **Top fallback kinds** identify HOW the parser is currently falling back. Each kind is a specific code path in the parser; the n-gram tells you what oracle-text shape triggers it.")
    lines.append("- **Histogram** identifies whether fallback is concentrated on a few bad cards (severe bucket) or sprinkled across many (low/medium bins).")
    lines.append("")
    lines.append("Regenerate with `python3 scripts/ast_corpus_health.py`. The script needs the gitignored `data/rules/ast_dataset.jsonl`.")
    lines.append("")
    return "\n".join(lines)


def load_cards(dataset_path: Path) -> list[CardStats]:
    cards: list[CardStats] = []
    with dataset_path.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            row = json.loads(line)
            cards.append(analyse_card(row))
    return cards


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", default=str(DATASET), help="Path to ast_dataset.jsonl")
    parser.add_argument("--output", default=str(DEFAULT_OUT), help="Path to emit the markdown report")
    parser.add_argument("--top-patterns", type=int, default=10, help="How many fallback-kind buckets to surface")
    args = parser.parse_args(argv)

    dataset_path = Path(args.dataset)
    if not dataset_path.exists():
        print(f"ERROR: dataset not found at {dataset_path}", flush=True)
        return 2

    cards = load_cards(dataset_path)
    report = render_report(cards, top_patterns_limit=args.top_patterns)
    out_path = Path(args.output)
    out_path.write_text(report, encoding="utf-8")
    print(f"Wrote {out_path} ({len(cards):,} cards analysed)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
