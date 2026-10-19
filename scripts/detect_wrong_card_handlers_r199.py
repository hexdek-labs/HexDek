#!/usr/bin/env python3
"""r199 — detect per_card handlers that implement the WRONG CARD, by
scoring the HANDLER BODY against the real oracle text.

This supersedes the r178 detector, which scored a handler's
`// Oracle text:` COMMENT. A comment can be wrong about a handler that is
right (Jace, Wielder of Mysteries), and a handler with no comment at all
is invisible to a comment-based method.

Direction matters and is the whole design:

    handler -> oracle   (PRECISION: is everything the handler does
                         something the card actually says?)

NOT oracle -> handler (recall / completeness). An INCOMPLETE handler
makes few claims and all of them are supported, so it scores clean —
which is exactly the Jace discrimination r178 failed.

A CLAIM is a mechanical assertion extracted from the handler source:
a trigger-event subscription, an engine primitive the body invokes, a
token subtype it mints, a colour it mints, or the presence of an
activated-ability hook. Each claim carries a predicate over the real
oracle text. A claim is UNSUPPORTED when the predicate fails.

`r.OnETB` is NON-CLAIMING on its own: it is the engine's universal
"install this card" hook, where statics and replacement effects
register. Claims from an ETB handler come only from what its body does.

Usage:
    python3 -I scripts/detect_wrong_card_handlers_r199.py \
        internal/gameengine/per_card /path/to/ast_dataset.jsonl [--json out.json]

The corpus is gitignored; see CLAUDE.md for where to fetch it.
"""

import json
import os
import re
import sys

# ---------------------------------------------------------------------------
# claim vocabulary
#
# Grounded in what the handler corpus ACTUALLY uses, not invented: the
# trigger-event list below was produced by counting every `r.OnTrigger`
# event string in internal/gameengine/per_card, and the primitive list by
# counting every `gameengine.X` call there.
# ---------------------------------------------------------------------------

# trigger event -> regex that must match the real oracle text
TRIGGER_EVIDENCE = {
    "combat_begin": r"beginning of combat",
    "begin_combat": r"beginning of combat",
    "begin_combat_controller": r"beginning of combat",
    "creature_attacks": r"attack",
    "attacks": r"attack",
    "attack": r"attack",
    "attack_declared": r"attack",
    "declare_attackers": r"attack",
    "combat_attackers_declared": r"attack",
    "end_of_combat": r"end of combat",
    "permanent_etb": r"enters|entering|put onto the battlefield",
    "nonland_permanent_etb": r"enters|entering|put onto the battlefield",
    "creature_enters_battlefield": r"enters|entering",
    "land_entered_battlefield": r"land.{0,30}enters|landfall",
    "outlaw_etb": r"enters|outlaw",
    "permanent_ltb": r"leaves the battlefield|dies|put into .{0,20}graveyard|exiled",
    "creature_dies": r"dies|put into .{0,20}graveyard",
    "dies": r"dies|put into .{0,20}graveyard",
    "upkeep": r"upkeep",
    "upkeep_controller": r"upkeep",
    "upkeep_opponent": r"upkeep",
    "upkeep_start": r"upkeep",
    "end_step": r"end step",
    "end_step_controller": r"end step",
    "untap_step": r"untap step",
    "draw_step_controller": r"draw step",
    "postcombat_main_controller": r"main phase",
    "spell_cast": r"cast|spell",
    "instant_or_sorcery_cast": r"instant or sorcery|cast",
    "noncreature_spell_cast": r"noncreature|cast",
    "creature_spell_cast": r"creature spell|cast",
    "spell_cast_by_opponent": r"opponent casts|cast",
    "magecraft": r"magecraft|instant or sorcery",
    "spell_copied": r"cop(y|ies)",
    "combat_damage_player": r"combat damage|deals damage",
    "combat_damage_to_player": r"combat damage|deals damage",
    "creature_combat_damage_to_player": r"combat damage|deals damage",
    "combat_damage_dealt": r"combat damage|deals damage",
    "combat_damage_creature": r"combat damage|deals damage",
    "creature_dealt_damage": r"deals damage|dealt damage",
    "noncombat_damage_to_player": r"damage",
    "noncombat_damage_to_creature": r"damage",
    "damage_taken": r"damage",
    "damaged": r"damage",
    "life_gained": r"gain.{0,12}life|lifelink",
    "life_lost": r"lose.{0,12}life|loses life",
    "life_change": r"life",
    "card_discarded": r"discard",
    "card_drawn": r"draw",
    "draw": r"draw",
    "player_would_draw": r"draw",
    "opponent_draws": r"draw",
    "card_cycled": r"cycl",
    "card_exiled": r"exile",
    "zone_change": r"graveyard|exile|library|hand|battlefield",
    "graveyard_leave": r"graveyard",
    "land_to_graveyard": r"land|graveyard",
    "permanent_sacrificed": r"sacrific",
    "creature_sacrificed": r"sacrific",
    "artifact_sacrificed": r"sacrific",
    "food_sacrificed": r"food|sacrific",
    "counter_placed": r"counter",
    "proliferate": r"proliferate",
    "lore_counter_added": r"__TYPE_SAGA__",
    "class_level_up": r"__TYPE_CLASS__",
    "token_created": r"token",
    "investigate": r"investigat|clue",
    "permanent_tapped": r"tap",
    "becomes_tapped": r"tap",
    "tap_event": r"tap",
    "land_tapped_for_mana": r"land|mana",
    "artifact_tapped_for_mana": r"artifact|mana",
    "mana_added_from_permanent": r"mana",
    "transform": r"transform|daybound|nightbound|disguise",
    "permanent_phased_out": r"phase",
    "seat_eliminated": r"lose|player",
    "creature_targeted": r"target",
    "became_target": r"target",
    "heroic": r"heroic|target",
    "inspired": r"inspired|untap",
    "eerie": r"eerie",
    "expend": r"expend",
    "tribute_resolved": r"tribute",
    "creature_mutated": r"mutate",
    "ninjutsu_activated": r"ninjutsu",
    "mount_saddled": r"saddle",
    "commit_crime": r"crime",
    "ring_tempt": r"ring",
    "scry": r"scry",
    "roll_dice": r"roll",
    "die_rolled": r"roll",
}

# engine primitive -> regex that must match the real oracle text.
# Helpers local to the package (drawOne, ...) are included.
PRIMITIVE_EVIDENCE = {
    "DrawN": r"draw",
    "drawOne": r"draw",
    "DrawCard": r"draw",
    "DrawCards": r"draw",
    "LoseLife": r"lose.{0,15}life|loses life|damage|drain|blight",
    "GainLife": r"gain.{0,15}life|lifelink",
    "DealDamage": r"damage",
    "CreateTreasureToken": r"treasure",
    "CreateFoodToken": r"food",
    "CreateClueToken": r"clue|investigat",
    "CreateBloodToken": r"blood",
    "SacrificePermanent": r"sacrific",
    "DestroyPermanent": r"destroy",
    "ExilePermanent": r"exile",
    "BouncePermanent": r"return.{0,40}hand|owner's hand",
    "DiscardCard": r"discard",
    "DiscardN": r"discard",
    "MillCards": r"mill",
    "MillN": r"mill",
    "Surveil": r"surveil",
    "Scry": r"scry",
    "Proliferate": r"proliferate",
    "PerformExplore": r"explore",
    "AddMana": r"add.{0,20}mana|\{[wubrgc]\}",
    "ApplyControlChange": r"gain control|exchange control",
    "TransformPermanent": r"transform|daybound|nightbound|disguise|meld",
    "CounterSpell": r"counter target|counter that spell|counter it",
    "GoadCreature": r"goad",
    "Amass": r"amass",
    "VentureIntoDungeon": r"venture",
    "TheRingTemptsYou": r"ring tempts",
    "MintSpellCopy": r"cop(y|ies)",
    "MintTokenAsCopyOf": r"cop(y|ies)",
    "CreateDoubledTokens": r"token",
    "CreateCreatureToken": r"token",
    "CreateToken": r"token",
    "FightCreatures": r"fight",
    "Monstrosity": r"monstros",
    "Bolster": r"bolster",
    "Support": r"support",
    "Populate": r"populate",
    "Connive": r"connive",
}

# Keyword grants, mapped to the regex that must match the oracle text.
# The engine's internal spelling is not always the printed wording:
# "unblockable" is printed as "can't be blocked".
GRANTABLE_KEYWORDS = {
    "flying": r"flying", "trample": r"trample", "deathtouch": r"deathtouch",
    "lifelink": r"lifelink", "first strike": r"first strike",
    "double strike": r"double strike", "vigilance": r"vigilance",
    "haste": r"haste", "menace": r"menace", "reach": r"reach",
    "defender": r"defender", "hexproof": r"hexproof",
    "indestructible": r"indestructible", "shroud": r"shroud",
    "flash": r"flash", "infect": r"infect", "fear": r"fear",
    "intimidate": r"intimidate", "protection": r"protection",
    "skulk": r"skulk", "horsemanship": r"horsemanship",
    "unblockable": r"can't be blocked|unblockable",
}

# Only string literals in these call/struct contexts are keyword GRANTS.
# A bare scan is wrong: Lightning, Army of One reads ctx["defender"], a
# map key, which a flat scan scores as granting defender.
KEYWORD_CONTEXT_RE = re.compile(
    r"(?:SetKeywordFlags|GrantKeywordFlag|RevokeKeywordFlag|ClearKeywordFlags|"
    r"HasKeywordFlagGrant|GrantedAbilities|AddKeyword|gameast\.Keyword\{)"
    r"[^\n]{0,200}")

# token subtypes we recognise inside Types/token-name strings
TOKEN_SUBTYPES = {
    "angel", "ape", "army", "assassin", "beast", "bird", "boar", "cat",
    "cleric", "construct", "demon", "dinosaur", "dragon", "drake", "druid",
    "dwarf", "elemental", "elephant", "elf", "elk", "eldrazi", "faerie",
    "fish", "fractal", "frog", "fungus", "germ", "giant", "gnome", "goat",
    "goblin", "golem", "griffin", "horror", "horse", "human", "hydra",
    "illusion", "insect", "knight", "kobold", "kor", "lizard", "merfolk",
    "monk", "myr", "ninja", "nightmare", "orc", "ooze", "pegasus", "pest",
    "phyrexian", "pirate", "plant", "rabbit", "rat", "rogue", "saproling",
    "satyr", "scarecrow", "scion", "serf", "servo", "shapeshifter", "sheep",
    "skeleton", "slith", "sliver", "snake", "soldier", "spider", "spirit",
    "squid", "squirrel", "tentacle", "thopter", "thrull", "treefolk",
    "vampire", "wall", "warrior", "whale", "wizard", "wolf", "wraith",
    "wurm", "zombie",
}

COLOR_WORD = {"W": "white", "U": "blue", "B": "black", "R": "red", "G": "green"}

# oracle-text shapes that constitute an activated ability
ACTIVATION_RE = re.compile(
    r"(\{[^}]+\}[^\n:]*:)"            # a mana/tap cost then a colon
    r"|(^|\n)[^\n:]{0,80}:"            # any "cost: effect" line
    # a card that GRANTS activated abilities legitimately needs the hook
    # even though it prints no cost line of its own (Necrotic Ooze).
    r"|activated abilit"
    r"|\b(equip|cycling|channel|unearth|level up|crew|adapt|monstrosity|"
    r"embalm|eternalize|scavenge|reconfigure|outlast|forecast|transmute|"
    r"morph|megamorph|disguise|flashback|escape|madness|bestow|ninjutsu|"
    r"evoke|prototype|suspend|plot|boast|exhaust|craft|freerunning)\b",
    re.I,
)

LOYALTY_RE = re.compile(r"^\s*[+−\-]?\s*[0-9X]+\s*:", re.M)

REG_RE = re.compile(
    r"\br\.On([A-Za-z]+)\(\s*"
    r'"((?:[^"\\]|\\.)*)"'                       # card name
    r'(?:\s*,\s*"([a-z_0-9]+)")?'                # optional event
    r"\s*,\s*([A-Za-z_][A-Za-z0-9_]*)"           # handler func
)

FUNC_RE = re.compile(
    r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.M)
IDENT_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\b")
STRLIT_RE = re.compile(r'"((?:[^"\\]|\\.)*)"')
ORACLE_BLOCK_RE = re.compile(r"^//\s*Oracle text:\s*$", re.M)


def strip_comments(src):
    """Remove // and /* */ comments. Comments are NOT evidence: several in
    this repo are provably fabricated (see docs/wrong-card-handlers-r179)."""
    out = []
    i, n = 0, len(src)
    in_str = in_ch = in_raw = False
    while i < n:
        c = src[i]
        nxt = src[i + 1] if i + 1 < n else ""
        if in_str:
            if c == "\\":
                out.append("  "); i += 2; continue
            if c == '"':
                in_str = False
            out.append(c); i += 1; continue
        if in_raw:
            if c == "`":
                in_raw = False
            out.append(c); i += 1; continue
        if in_ch:
            if c == "\\":
                out.append("  "); i += 2; continue
            if c == "'":
                in_ch = False
            out.append(c); i += 1; continue
        if c == "/" and nxt == "/":
            while i < n and src[i] != "\n":
                i += 1
            continue
        if c == "/" and nxt == "*":
            i += 2
            while i + 1 < n and not (src[i] == "*" and src[i + 1] == "/"):
                if src[i] == "\n":
                    out.append("\n")
                i += 1
            i += 2
            continue
        if c == '"':
            in_str = True
        elif c == "`":
            in_raw = True
        elif c == "'":
            in_ch = True
        out.append(c); i += 1
    return "".join(out)


def func_bodies(src):
    """Map func name -> body text, by brace matching on comment-free source.

    The opening brace must be found as the LAST character on its line.
    Taking the first `{` after the signature is wrong and silently yields
    an empty body for every handler, because Go handler signatures carry
    `ctx map[string]interface{}` — that inner brace closes immediately.
    gofmt guarantees the body brace ends its line, so anchor on that.
    """
    bodies = {}
    for m in FUNC_RE.finditer(src):
        name = m.group(1)
        brace = -1
        j = m.end()
        while True:
            j = src.find("{", j)
            if j < 0:
                break
            k = j + 1
            while k < len(src) and src[k] in " \t":
                k += 1
            if k < len(src) and src[k] == "\n":
                brace = j
                break
            j += 1
        if brace < 0:
            continue
        depth, i, n = 0, brace, len(src)
        while i < n:
            if src[i] == "{":
                depth += 1
            elif src[i] == "}":
                depth -= 1
                if depth == 0:
                    break
            i += 1
        bodies[name] = src[brace:i + 1]
    return bodies


def reachable(entry, bodies, max_depth=3):
    """Union of entry's body with locally-defined functions it calls."""
    seen, frontier, text = set(), [entry], []
    for _ in range(max_depth):
        nxt = []
        for fn in frontier:
            if fn in seen or fn not in bodies:
                continue
            seen.add(fn)
            body = bodies[fn]
            text.append(body)
            for ident in set(IDENT_RE.findall(body)):
                if ident in bodies and ident not in seen:
                    nxt.append(ident)
        frontier = nxt
        if not frontier:
            break
    return "\n".join(text)


# Game-visible actions. A trigger handler that contains none of these is
# a BOOKKEEPING hook (register/unregister/cleanup/diagnostics), not a
# claim that the card has an ability on that event. Lightning, Army of
# One subscribes to permanent_ltb purely to unregister its own damage
# replacements; scoring that as a leave-the-battlefield ability was the
# single largest false-positive cluster in this detector.
ACTION_RE = re.compile(
    r"\b(Draw\w*|drawOne|LoseLife|GainLife|DealDamage|Create\w*Token|"
    r"CreateCreatureToken|SacrificePermanent|DestroyPermanent|"
    r"ExilePermanent|BouncePermanent|Discard\w*|Mill\w*|Surveil|Scry|"
    r"Proliferate|AddMana|AddCounter|PutCounters\w*|ApplyControlChange|"
    r"TransformPermanent|CounterSpell|MoveCard|PushStackItem|"
    r"FightCreatures|Modifications\s*=|Modification\{|enterBattlefieldWithETB|"
    r"MintSpellCopy|MintTokenAsCopyOf|GoadCreature|Amass|TapPermanent|"
    r"UntapPermanent|SearchLibrary|Shuffle\w*)\b")

# Calls that are pure state management, never an ability's effect.
BOOKKEEPING_ONLY_RE = re.compile(
    r"\b(Unregister\w*|Revoke\w*|Clear\w*|delete|emit|emitFail|emitPartial|"
    r"Register\w*|LogEvent)\b")


def is_bookkeeping(fn_body):
    """True when a trigger handler performs no game-visible action."""
    if not fn_body.strip():
        return True
    return not ACTION_RE.search(fn_body)


def mint_regions(body):
    """Source spans where a TOKEN IS CONSTRUCTED.

    Colour and subtype strings must be read only from here. Scanning the
    whole body is wrong: Mother of Runes switches on "pip:W".."pip:G" to
    COUNT an opponent's colours for its protection choice, and a flat
    scan reads that as minting four off-identity colours.
    """
    spans = []
    for m in re.finditer(r"gameengine\.Card\{", body):
        i, depth = m.end() - 1, 0
        while i < len(body):
            if body[i] == "{":
                depth += 1
            elif body[i] == "}":
                depth -= 1
                if depth == 0:
                    break
            i += 1
        spans.append(body[m.start():i + 1])
    for m in re.finditer(
            r"\b(?:CreateCreatureToken|CreateToken|CreateDoubledTokens|"
            r"MintTokenAsCopyOf)\s*\(", body):
        i, depth = m.end() - 1, 0
        while i < len(body):
            if body[i] == "(":
                depth += 1
            elif body[i] == ")":
                depth -= 1
                if depth == 0:
                    break
            i += 1
        spans.append(body[m.start():i + 1])
    return "\n".join(spans)


def card_colors(entry):
    cols = set(entry.get("colors") or [])
    for sym in re.findall(r"\{([^}]+)\}", entry.get("mana_cost") or ""):
        for ch in sym:
            if ch in COLOR_WORD:
                cols.add(ch)
    # colour identity also picks up mana symbols printed in the rules text
    for sym in re.findall(r"\{([^}]+)\}", entry.get("oracle_text") or ""):
        for ch in sym:
            if ch in COLOR_WORD:
                cols.add(ch)
    return cols


def extract_claims(card, entry, regs, bodies):
    """Return a list of (kind, detail, supported: bool)."""
    oracle = (entry.get("oracle_text") or "").lower()
    tline = (entry.get("type_line") or "").lower()
    cols = card_colors(entry)
    claims = []

    def ev(pattern):
        if pattern == "__TYPE_SAGA__":
            return "saga" in tline
        if pattern == "__TYPE_CLASS__":
            return "class" in tline
        return re.search(pattern, oracle) is not None

    # ---- A. trigger subscriptions -----------------------------------
    for kind, event, fn in regs:
        if kind != "Trigger" or not event:
            continue
        pat = TRIGGER_EVIDENCE.get(event)
        if pat is None:
            continue                      # unknown event: abstain, never flag
        if is_bookkeeping(reachable(fn, bodies)):
            continue                      # cleanup hook, not an ability
        claims.append(("trigger", event, ev(pat)))

    # ---- body of every registered handler ----------------------------
    body = "\n".join(reachable(fn, bodies) for _, _, fn in regs)

    # ---- B. engine primitives ---------------------------------------
    for prim, pat in PRIMITIVE_EVIDENCE.items():
        if re.search(r"\b" + prim + r"\s*\(", body):
            claims.append(("primitive", prim, ev(pat)))

    # ---- E. activated-ability hook ----------------------------------
    if any(k == "Activated" for k, _, _ in regs):
        raw = entry.get("oracle_text") or ""
        has_act = bool(ACTIVATION_RE.search(raw)) or bool(LOYALTY_RE.search(raw))
        claims.append(("activated_hook", "OnActivated", has_act))

    lits = {s.lower() for s in STRLIT_RE.findall(body)}
    mint = mint_regions(body)
    mint_lits = {s.lower() for s in STRLIT_RE.findall(mint)}

    # ---- C. token subtypes ------------------------------------------
    subs = set()
    for s in mint_lits:
        for w in re.split(r"[^a-z]+", s):
            if w in TOKEN_SUBTYPES:
                subs.add(w)
    for w in sorted(subs):
        claims.append(("token_subtype", w, w in oracle))

    # ---- D. colours minted ------------------------------------------
    minted = set()
    for s in mint_lits:
        m = re.fullmatch(r"pip:([wubrg])", s)
        if m:
            minted.add(m.group(1).upper())
    for m in re.finditer(r"Colors:\s*\[\]string\{([^}]*)\}", mint):
        for s in STRLIT_RE.findall(m.group(1)):
            if s.upper() in COLOR_WORD:
                minted.add(s.upper())
    # A card may legitimately mint a token OUTSIDE its own colour
    # identity — Sand Scout is mono-white and prints a "1/1 red, green,
    # and white Sand Warrior". Colour words in the rules text are not
    # mana symbols and add no identity, so the colour WORD appearing in
    # the oracle text supports the claim just as identity membership does.
    for c in sorted(minted):
        ok = (c in cols) or (COLOR_WORD[c] in oracle)
        claims.append(("color", COLOR_WORD[c], ok))

    # ---- F. keyword grants ------------------------------------------
    kw_ctx = "\n".join(KEYWORD_CONTEXT_RE.findall(body))
    kw_lits = {s.lower() for s in STRLIT_RE.findall(kw_ctx)}
    for s in sorted(kw_lits):
        if s in GRANTABLE_KEYWORDS:
            claims.append(("keyword", s, bool(re.search(GRANTABLE_KEYWORDS[s], oracle))))

    # ---- G. power/toughness pump ------------------------------------
    # A Modification carrying a nonzero Power/Toughness delta is an
    # anthem or pump the card must actually print.
    if re.search(r"Modification\{[^}]*\b(Power|Toughness):\s*-?[1-9]", body):
        claims.append(("pump", "+N/+N", bool(re.search(r"\+\d+/\+\d+|gets? \+|get \+", oracle))))

    return claims


# Claim kinds that name a concrete, checkable PARTICULAR rather than a
# generic verb. Measured base rates in the live corpus (n=32,417):
# creature subtypes 0.001-0.003, lifelink 0.021 — against generic verbs
# "token" 0.131, "draw" 0.137, "enters" 0.232. A failed particular is
# therefore 50-200x more discriminating than a passed generic is
# reassuring, which is why a FLAT unsupported-fraction under-ranks a
# handler that is wrong about specifics while using common verbs.
HARD_KINDS = {"token_subtype", "color", "keyword", "activated_hook"}


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(2)
    pc_dir, corpus_path = sys.argv[1], sys.argv[2]
    out_json = None
    if "--json" in sys.argv:
        out_json = sys.argv[sys.argv.index("--json") + 1]

    corpus = {}
    for line in open(corpus_path, encoding="utf-8"):
        try:
            d = json.loads(line)
        except Exception:
            continue
        if d.get("name"):
            corpus[d["name"]] = d

    files = [f for f in sorted(os.listdir(pc_dir))
             if f.endswith(".go") and not f.endswith("_test.go")]

    rows = []
    buckets = {"no_registration": [], "no_corpus_name": [], "scored": []}
    for f in files:
        raw = open(os.path.join(pc_dir, f), encoding="utf-8", errors="replace").read()
        src = strip_comments(raw)
        bodies = func_bodies(src)
        per_card = {}
        for m in REG_RE.finditer(src):
            kind, name, event, fn = m.group(1), m.group(2), m.group(3), m.group(4)
            name = name.replace('\\"', '"')
            per_card.setdefault(name, []).append((kind, event, fn))
        if not per_card:
            buckets["no_registration"].append(f)
            continue
        has_block = len(ORACLE_BLOCK_RE.findall(raw)) > 0
        multi = len([n for n in per_card if n in corpus]) > 1
        for name, regs in per_card.items():
            entry = corpus.get(name)
            if entry is None:
                buckets["no_corpus_name"].append((f, name))
                continue
            claims = extract_claims(name, entry, regs, bodies)
            bad = [c for c in claims if not c[2]]
            frac = (len(bad) / len(claims)) if claims else 0.0
            hard = sum(1 for k, _, s in claims if not s and k in HARD_KINDS)
            rows.append({
                "card": name, "file": f, "multi_card_file": multi,
                "has_oracle_comment": has_block,
                "claims": len(claims), "unsupported": len(bad),
                "hard": hard, "frac": round(frac, 3),
                "unsupported_detail": [f"{k}:{d}" for k, d, _ in bad],
                "all_detail": [f"{k}:{d}={'ok' if s else 'NO'}"
                               for k, d, s in claims],
            })
            buckets["scored"].append(name)

    # Two disjoint ways to be a suspect:
    #   RULE A (pre-registered): >= MIN_CLAIMS claims and >= THRESH of
    #           them contradicted.
    #   RULE B (added after calibration, see the r199 writeup): >= 2
    #           contradicted HARD claims. Rule A alone is a ratio whose
    #           denominator includes near-tautological matches, so a
    #           handler wrong about specifics but phrased in common verbs
    #           dilutes below threshold. Rule B is scale-free.
    MIN_CLAIMS, THRESH, HARD_MIN = 3, 0.60, 2
    ruleA = [r for r in rows if r["claims"] >= MIN_CLAIMS and r["frac"] >= THRESH]
    ruleB = [r for r in rows if r["hard"] >= HARD_MIN]
    flagged = {id(r): r for r in ruleA + ruleB}.values()
    flagged = sorted(flagged, key=lambda r: (-r["hard"], -r["frac"],
                                             -r["unsupported"], r["card"]))

    print(f"corpus entries            : {len(corpus)}")
    print(f"files scanned             : {len(files)}")
    print(f"  no registration found   : {len(buckets['no_registration'])}")
    print(f"  name not in corpus      : {len(buckets['no_corpus_name'])}")
    print(f"card-handler attributions : {len(rows)}")
    print(f"  in multi-card files     : {sum(1 for r in rows if r['multi_card_file'])}")
    print(f"  with // Oracle text:    : {sum(1 for r in rows if r['has_oracle_comment'])}")
    print(f"  WITHOUT that comment    : {sum(1 for r in rows if not r['has_oracle_comment'])}")
    print(f"  scorable (>= {MIN_CLAIMS} claims): {sum(1 for r in rows if r['claims'] >= MIN_CLAIMS)}")
    print(f"  rule A (frac >= {THRESH})    : {len(ruleA)}")
    print(f"  rule B (hard >= {HARD_MIN})       : {len(ruleB)}")
    print(f"FLAGGED (A or B)          : {len(flagged)}")
    print()
    for r in flagged:
        rule = ("A" if r in ruleA else "") + ("B" if r in ruleB else "")
        print(f"[{rule:2s}] hard={r['hard']} frac={r['frac']:.2f} "
              f"{r['unsupported']}/{r['claims']}  {r['card']}  [{r['file']}]")
        print(f"        unsupported: {', '.join(r['unsupported_detail'])}")

    if out_json:
        json.dump(rows, open(out_json, "w"), indent=1)
        print(f"\nfull rows -> {out_json}")


if __name__ == "__main__":
    main()
