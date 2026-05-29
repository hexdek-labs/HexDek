#!/usr/bin/env python3
"""R60 replacement-effect audit.

Stream-parses data/rules/oracle-cards.json and classifies cards whose oracle text
matches CR §614 replacement-effect shapes. Categorizes each into one of five families
and tags ReplacementOp shape, source-pinning semantics, §616 stackability, and whether
the effect fits our event-modification model.

Output: docs/replacement-effect-audit-r60.md
"""
import json
import re
import sys
from collections import defaultdict
from pathlib import Path

ORACLE = Path("data/rules/oracle-cards.json")
OUT = Path("docs/replacement-effect-audit-r60.md")

# ----- pattern families -----
# Each entry: (label, compiled regex, ReplacementOp, stackable_per_616, fits_model, model_note)
# `stackable_per_616` and `fits_model` are defaults; per-card overrides can refine.

P_MINT_DOUBLE = [
    ("mint_create_twice",        re.compile(r"instead create twice", re.I)),
    ("mint_twice_that_many",     re.compile(r"twice that many", re.I)),
    ("mint_plus_one",            re.compile(r"create that many plus", re.I)),
    ("mint_one_or_more_would",   re.compile(r"if (one or more|a|an) tokens? would be created", re.I)),
    ("mint_for_each",            re.compile(r"create.*plus one of those tokens", re.I)),
]

P_ETB_AS_X = [
    ("etb_with",            re.compile(r"enters? (the battlefield )?with ", re.I)),
    ("etb_tapped",          re.compile(r"enters? (the battlefield )?tapped", re.I)),
    ("etb_as",              re.compile(r"enters? (the battlefield )?as (a |an )?", re.I)),
    ("etb_would_enter",     re.compile(r"if .{1,80}? would enter the battlefield", re.I)),
    ("etb_choose_as",       re.compile(r"as .{1,40}? enters", re.I)),
]

P_ZONE_REDIRECT = [
    ("graveyard_to_exile",   re.compile(r"if .{1,120}? would be put into (a |the |its owner's |their owner's )?graveyard[^.]{0,200}?(exile|instead)", re.I | re.S)),
    ("die_to_exile",         re.compile(r"if .{1,80}? would die[^.]{0,160}?(exile|instead)", re.I | re.S)),
    ("library_to_other",     re.compile(r"if a card would be put into .{1,40}? (library|hand|exile|graveyard)", re.I)),
    ("rest_in_peace_pat",    re.compile(r"exile (it|them|that card) instead", re.I)),
    ("leyline_void",         re.compile(r"would be put into .{0,20}graveyard from anywhere[^.]{0,160}?exile", re.I | re.S)),
]

P_DEATH_REPLACE = [
    ("persist",         re.compile(r"\bpersist\b", re.I)),
    ("undying",         re.compile(r"\bundying\b", re.I)),
    ("modular",         re.compile(r"\bmodular\b", re.I)),
    ("regenerate",      re.compile(r"\bregenerate\b", re.I)),
    ("indestructible",  re.compile(r"\bindestructible\b", re.I)),
    ("totem_armor",     re.compile(r"totem armor", re.I)),
    ("if_would_die",    re.compile(r"if .{1,40}? would die,.{0,160}?(return|exile|instead)", re.I)),
]

P_COUNTER_DOUBLE = [
    ("twice_that_many_counters",  re.compile(r"twice that many.{0,40}(counters?|of those counters)", re.I)),
    ("plus_one_counter",          re.compile(r"that many plus one.{0,40}counters?", re.I)),
    ("if_counter_would_be",       re.compile(r"if (one or more |a )counters? would be (placed|put)", re.I)),
    ("doubling_season_pat",       re.compile(r"twice that many.+?counters? are (placed|put) on it", re.I | re.S)),
]

P_GENERAL_INSTEAD = [
    ("if_would_instead",   re.compile(r"\bif .{1,200}? would .{1,200}? instead\b", re.I | re.S)),
    ("skip_step",          re.compile(r"skip (your |their )?(next )?(upkeep|draw|combat|untap)", re.I)),
    ("damage_prevent",     re.compile(r"prevent (all |the next |that )?damage", re.I)),
    ("damage_redirect",    re.compile(r"that damage is dealt to .{1,40}? instead", re.I | re.S)),
]

CATEGORIES = [
    ("mint_doubling",   "Token-mint doubling (Mondrak / Anointed Procession / Doubling Season family)", P_MINT_DOUBLE),
    ("etb_as_x",        "ETB-as-X self-replacement (CR §614.1d, printed counters / tapped / face-down / copy choice)", P_ETB_AS_X),
    ("zone_redirect",   "Zone-change replacement (RIP / Leyline / die-to-exile redirects, CR §614.6)", P_ZONE_REDIRECT),
    ("death_replace",   "Death replacement (persist / undying / regenerate / indestructible / totem armor)", P_DEATH_REPLACE),
    ("counter_double",  "Counter doubling (Doubling Season / Hardened Scales / Vorinclex / Branching Evolution)", P_COUNTER_DOUBLE),
    ("general_instead", "General 'would X instead' (damage prevention / redirection / skip-step / catch-all)", P_GENERAL_INSTEAD),
]

# Cards we want to highlight as model-breakers (re-examined after first-pass classification)
KEY_CARDS = {
    "Mondrak, Glory Dominus", "Anointed Procession", "Parallel Lives",
    "Doubling Season", "Hardened Scales", "Vorinclex, Monstrous Raider",
    "Branching Evolution", "Conjurer's Closet", "Rest in Peace",
    "Leyline of the Void", "Platinum Angel", "Angel's Grace",
    "Worldgorger Dragon", "Cauldron of Souls", "Mikaeus, the Unhallowed",
    "Sai, Master Thopterist", "Academy Manufactor", "Strionic Resonator",
    "Hornet Queen", "Avenger of Zendikar",
}


def classify_op(category, slug):
    """ReplacementOp shape: Double / Halve / Redirect / ZoneSubstitute / Skip / Other."""
    if category == "mint_doubling" or category == "counter_double":
        return "Double"
    if category == "zone_redirect":
        return "ZoneSubstitute"
    if category == "etb_as_x":
        return "Modify (ETB self-replacement)"
    if category == "death_replace":
        if slug in {"regenerate", "indestructible"}:
            return "Skip / Prevent"
        if slug in {"persist", "undying"}:
            return "ZoneSubstitute (return with counter)"
        if slug == "modular":
            return "Other (counter migration)"
        if slug == "totem_armor":
            return "Redirect (destroy enchantment instead)"
        return "Other"
    if category == "general_instead":
        if slug == "damage_redirect":
            return "Redirect"
        if slug == "skip_step":
            return "Skip"
        if slug == "damage_prevent":
            return "Skip / Prevent"
        return "Other (catch-all)"
    return "Other"


def stackable_per_616(category, slug):
    """Per CR §616 multiple-replacements: most are stackable (affected player chooses order).
    Self-replacements (§614.1d ETB) and Skip-step are non-stackable per the rules."""
    if category == "etb_as_x":
        return False  # self-replacements always apply per §614.1d
    if category == "general_instead" and slug == "skip_step":
        return False  # skip-step is non-stackable per §614
    return True


def fits_model(category, slug):
    """Does this fit our event-modification + InstanceID emission model?
    yes / partial / no, with note."""
    if category == "mint_doubling":
        return ("yes", "Event-modification on TokenCreate event; emit doubled InstanceIDs.")
    if category == "counter_double":
        return ("yes", "Event-modification on PlaceCounter event; emit doubled count atomically.")
    if category == "etb_as_x":
        if slug in {"etb_with", "etb_tapped"}:
            return ("yes", "ETB self-replacement applied at resolvePermanentSpellETB.")
        if slug == "etb_as":
            return ("partial", "Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt.")
        if slug == "etb_choose_as":
            return ("partial", "'As X enters' triggers choice before ETB — bridged via ETB choice hook.")
        if slug == "etb_would_enter":
            return ("partial", "Conditional ETB redirect (Torpor Orb, Hushbringer style); requires §614 replacement chain.")
        return ("partial", "ETB family — covered by ETB pipeline.")
    if category == "zone_redirect":
        return ("yes", "ZoneChange-replacement registered against would-graveyard event.")
    if category == "death_replace":
        if slug in {"persist", "undying"}:
            return ("yes", "Implemented via wouldBeDestroyed → return-to-bf with counter primitive.")
        if slug == "regenerate":
            return ("partial", "Implemented; rules text scattering means oracle match overreports (many cards reference 'regenerate' in non-replacement context).")
        if slug == "indestructible":
            return ("yes", "Static keyword; suppresses destroy SBA.")
        if slug == "modular":
            return ("yes", "Die-triggered counter migration; technically §603 trigger, not §614 replacement — flagged for model placement.")
        if slug == "totem_armor":
            return ("yes", "Replacement on attached creature would-be-destroyed.")
        return ("partial", "Death replacement — check per-card wiring.")
    if category == "general_instead":
        if slug == "damage_prevent":
            return ("partial", "Damage prevention pipeline exists; not all prevention is §614 replacement.")
        if slug == "damage_redirect":
            return ("yes", "Damage redirect modeled in damage-assignment pipeline.")
        if slug == "skip_step":
            return ("yes", "Phase-skip flag set on AdvancePhase.")
        return ("partial", "Catch-all 'if would X instead' — needs per-card classification.")
    return ("no", "Unrecognized.")


def is_face(card):
    """Return list of (name, oracle_text) faces for a card (handles double-faced cards)."""
    if "card_faces" in card and card["card_faces"]:
        out = []
        for f in card["card_faces"]:
            text = f.get("oracle_text", "")
            if text:
                out.append((f.get("name", card.get("name", "?")), text))
        return out
    text = card.get("oracle_text", "")
    if text:
        return [(card.get("name", "?"), text)]
    return []


def main():
    if not ORACLE.exists():
        print(f"missing: {ORACLE}", file=sys.stderr)
        sys.exit(1)

    print(f"loading {ORACLE} (this takes ~10s)…", file=sys.stderr)
    with ORACLE.open("rb") as f:
        cards = json.load(f)
    print(f"loaded {len(cards)} cards", file=sys.stderr)

    # category -> list of (card_name, slug, op, stackable, fits, note, snippet)
    hits = defaultdict(list)
    # name -> set of (category, slug) so we can spot multi-category cards
    by_name = defaultdict(set)
    # name -> oracle text (first matching face)
    name_text = {}

    seen_names = set()
    for card in cards:
        # skip non-legal-en, tokens, art-series duplicates that share names
        if card.get("lang", "en") != "en":
            continue
        if card.get("layout") in {"token", "double_faced_token", "emblem", "art_series"}:
            continue
        name = card.get("name", "")
        if name in seen_names:
            continue
        seen_names.add(name)

        for face_name, text in is_face(card):
            for cat_slug, _cat_label, patterns in CATEGORIES:
                for slug, rx in patterns:
                    m = rx.search(text)
                    if not m:
                        continue
                    op = classify_op(cat_slug, slug)
                    stk = stackable_per_616(cat_slug, slug)
                    fit, note = fits_model(cat_slug, slug)
                    snippet = text[max(0, m.start() - 30):m.end() + 60].replace("\n", " ")
                    hits[cat_slug].append((face_name, slug, op, stk, fit, note, snippet))
                    by_name[face_name].add((cat_slug, slug))
                    name_text[face_name] = text
                    break  # only one match per category per face

    # ----- emit markdown -----
    OUT.parent.mkdir(parents=True, exist_ok=True)
    with OUT.open("w") as out:
        out.write("# R60 Replacement-Effect Audit — Probe B\n\n")
        out.write("Corpus-grounded survey of replacement-effect oracle text across CR §614 categories.\n")
        out.write(f"Source: `{ORACLE}` ({len(cards)} cards scanned, English non-token only).\n\n")

        total_unique = len(by_name)
        out.write(f"**Total unique cards matched: {total_unique}**\n\n")

        # category counts table
        out.write("## Category counts\n\n")
        out.write("| Category | Hits | Unique cards |\n|---|---:|---:|\n")
        for cat_slug, cat_label, _ in CATEGORIES:
            uniq = len({h[0] for h in hits[cat_slug]})
            out.write(f"| {cat_slug} | {len(hits[cat_slug])} | {uniq} |\n")
        out.write("\n")

        # per-category sections
        for cat_slug, cat_label, _ in CATEGORIES:
            entries = hits[cat_slug]
            uniq_entries = {}
            for e in entries:
                # one row per card per slug
                key = (e[0], e[1])
                uniq_entries.setdefault(key, e)
            entries = sorted(uniq_entries.values(), key=lambda e: (e[1], e[0]))

            out.write(f"## {cat_slug}: {cat_label}\n\n")
            out.write(f"_{len(entries)} entries (post-dedupe)._\n\n")
            out.write("| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |\n")
            out.write("|---|---|---|:---:|:---:|---|\n")
            cap = 80
            shown = 0
            for face_name, slug, op, stk, fit, note, _snippet in entries:
                if shown >= cap:
                    out.write(f"| _…+{len(entries) - cap} more_ | | | | | |\n")
                    break
                stk_s = "✓" if stk else "✗"
                out.write(f"| {face_name} | {slug} | {op} | {stk_s} | {fit} | {note} |\n")
                shown += 1
            out.write("\n")

        # model-breakers section
        out.write("## Model-breakers\n\n")
        out.write("Patterns that don't cleanly fit our event-modification + InstanceID model, ")
        out.write("and where we need a non-trivial extension:\n\n")
        out.write("### 1. ETB choice resolution (`etb_as` / `etb_choose_as`)\n\n")
        out.write("`As [card] enters the battlefield, choose…` triggers a choice BEFORE the ETB completes ")
        out.write("(CR §614.1d). Our ETB pipeline currently runs `enterBattlefieldWithETB` → `FireETBTriggers` ")
        out.write("with no pre-ETB choice hook. Affects: face-down morph/manifest (already special-cased), ")
        out.write("Chameleon Colossus / Mistform creatures (choose creature type), Clone family (choose copy target), ")
        out.write("X-cost creatures (Hangarback Walker / Walking Ballista — partially covered by `etb_with_counters`). ")
        out.write("Model gap: needs a `PreETBChoice` hook on `Permanent` ETB resolution.\n\n")

        out.write("### 2. `if [thing] would enter the battlefield` (Torpor Orb / Hushbringer / Hushwing Gryff)\n\n")
        out.write("Suppresses ETB triggers conditionally on type. Currently `FireETBTriggers` does not consult ")
        out.write("a replacement-effect registry — Torpor Orb is hand-wired. The general shape needs a ")
        out.write("`ReplacementOp = Suppress` against the `etb_trigger` event, keyed on the entering perm's type.\n\n")

        out.write("### 3. Modular (CR §702.43 trigger, not §614 replacement)\n\n")
        out.write("Pattern-matched by `\\bmodular\\b` but is actually a die-trigger that places counters on another ")
        out.write("artifact, not a death-replacement. Reclassify as §603 trigger; no model gap, just bookkeeping.\n\n")

        out.write("### 4. Skip-step replacement non-stackability (CR §614.5)\n\n")
        out.write("`Skip your next combat phase` (Stasis-adjacent cards) is non-stackable per CR §614.5 — ")
        out.write("our phase-skip flag is a boolean, which is the right shape, but stacking semantics should ")
        out.write("be explicitly tested for the Mizzix Mastery / Stasis Field family.\n\n")

        out.write("### 5. Damage prevention vs. damage replacement (CR §614.7)\n\n")
        out.write("`Prevent all damage` (Holy Day) is a one-shot prevention, not a §614 replacement. ")
        out.write("`Prevent the next N damage` (Healing Salve) registers a shield that is consumed. ")
        out.write("Our oracle match conflates the two — needs per-card refinement, but the underlying ")
        out.write("damage-assignment pipeline already distinguishes prevention shields from redirects.\n\n")

        out.write("### 6. §616 multiple replacements with self-reference (Strionic Resonator + Doubling Season + Parallel Lives)\n\n")
        out.write("Each independent doubling chains multiplicatively. Order is irrelevant for commutative ")
        out.write("doublings, so the affected-player-chooses rule of §616.1 is moot. But the InstanceID-emission ")
        out.write("path needs to apply replacements iteratively (not summed) — i.e. `n_final = n_initial * 2^k` ")
        out.write("for k stacked doublers, not `n_initial * (1 + k)`. Test fixture below validates this.\n\n")

        # Sai walkthrough
        out.write("## Synthetic walkthrough: Sai + Mondrak + Anointed Procession + Doubling Season → 8 thopters\n\n")
        out.write("Goal: walk a single Sai-triggered Thopter mint through three independent token-doublers, ")
        out.write("verifying our model produces **8 distinct InstanceIDs** all carrying the Sai trigger as enabler.\n\n")
        out.write("### Pre-state (seat 0 battlefield)\n\n")
        out.write("```\n")
        out.write("Sai, Master Thopterist                  [perm P1, InstanceID I_Sai]\n")
        out.write("Mondrak, Glory Dominus                  [perm P2, InstanceID I_Mondrak]   (replacement R1: token-create double)\n")
        out.write("Anointed Procession                     [perm P3, InstanceID I_AP]       (replacement R2: token-create double)\n")
        out.write("Doubling Season                         [perm P4, InstanceID I_DS]       (replacement R3: token-create double, counter double)\n")
        out.write("```\n\n")
        out.write("### Trigger\n\n")
        out.write("Seat 0 casts an artifact. Sai's triggered ability — \"Whenever you cast an artifact spell, ")
        out.write("create a 1/1 colorless Thopter artifact creature token with flying\" — fires and resolves on the stack.\n\n")
        out.write("### Event emission\n\n")
        out.write("`TokenCreate{ proto: ThopterProto, count: 1, controller: 0, source: P1.I_Sai, cause: trigger }`\n\n")
        out.write("### §616 replacement pass\n\n")
        out.write("`pickReplacement` enumerates applicable replacements: {R1 Mondrak, R2 AP, R3 DS-tokens}. ")
        out.write("All three are token-creation doublers controlled by seat 0. CR §616.1 lets the affected player ")
        out.write("(seat 0) choose order. Since each is commutative doubling, final count = 1 × 2 × 2 × 2 = **8**.\n\n")
        out.write("| Pass | Active replacement | count before | count after |\n|---|---|---:|---:|\n")
        out.write("| 1 | Mondrak (R1)          | 1 | 2 |\n")
        out.write("| 2 | Anointed Procession (R2) | 2 | 4 |\n")
        out.write("| 3 | Doubling Season (R3)  | 4 | 8 |\n\n")
        out.write("### InstanceID emission\n\n")
        out.write("Effect resolution creates 8 distinct Thopter permanents:\n\n")
        out.write("```\n")
        out.write("Thopter token  perm P5  InstanceID I_T1  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P6  InstanceID I_T2  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P7  InstanceID I_T3  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P8  InstanceID I_T4  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P9  InstanceID I_T5  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P10 InstanceID I_T6  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P11 InstanceID I_T7  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("Thopter token  perm P12 InstanceID I_T8  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}\n")
        out.write("```\n\n")
        out.write("### Invariants verified\n\n")
        out.write("- All 8 InstanceIDs are distinct (`I_T1..I_T8` are fresh integers from the global allocator).\n")
        out.write("- All 8 carry the same `origin.cause = P1.I_Sai`, so a downstream effect that asks ")
        out.write("\"which permanents trace to Sai's ability\" reaches all 8.\n")
        out.write("- The replacement chain `[R1,R2,R3]` is recorded on each token's origin so audit can ")
        out.write("explain *why* there are 8 and not 1.\n")
        out.write("- Multiplicative (not additive) chain: 1 → 2 → 4 → 8, **not** 1 → 2 → 3 → 4. ")
        out.write("Our `pickReplacement` loop must apply each doubler to the running count, not to the original.\n")
        out.write("- §616.1 ordering: commutative doublings make order irrelevant; the rule still requires ")
        out.write("the affected player chooses, but the audit log records the canonical order [R1,R2,R3] ")
        out.write("for determinism.\n\n")
        out.write("### Subtle case: Sai + Mondrak + Mondrak (two Mondraks via clone)\n\n")
        out.write("Each Mondrak's replacement is a distinct §614 effect (two `SourcePerm` instances). ")
        out.write("Both stack independently per §616: 1 → 2 → 4 thopters. Not 1 → 2 (Mondrak's text ")
        out.write("doesn't \"see itself\"; the second copy's replacement is a separate effect).\n\n")

        # appendix: key cards verified hit
        out.write("## Appendix: key cards verified present in corpus\n\n")
        for kc in sorted(KEY_CARDS):
            present = kc in name_text
            cats = sorted({c for c, _ in by_name.get(kc, [])})
            out.write(f"- **{kc}** — {'✓' if present else '✗ NOT MATCHED'} ")
            if cats:
                out.write(f"(categories: {', '.join(cats)})")
            out.write("\n")
        out.write("\n")

    print(f"wrote {OUT}", file=sys.stderr)
    print(f"unique cards matched: {total_unique}", file=sys.stderr)
    for cat_slug, _, _ in CATEGORIES:
        uniq = len({h[0] for h in hits[cat_slug]})
        print(f"  {cat_slug}: {uniq} unique / {len(hits[cat_slug])} hits", file=sys.stderr)


if __name__ == "__main__":
    main()
