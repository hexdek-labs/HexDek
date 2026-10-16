# r135 — the exporter learns card faces (Adventurer / Omen)

CR §715 (Adventurer Cards), CR §720 (Omen Cards).

## The defect

`scripts/parser.py :: normalize()` pulled a card's oracle text like this:

```python
text = card.get("oracle_text") or ""
if not text and card.get("card_faces"):
    text = "\n".join(f.get("oracle_text") or "" for f in card["card_faces"])
```

Every two-faced card therefore had **both faces joined with `\n` and parsed as
one card**. Measured on the live corpus: 3,221 of 38,754 cards have
`card_faces`, and **none of them carries a top-level `oracle_text`**, so the
join always fired.

Two consequences, both measured rather than reasoned about:

**1. The half's text became the parent's abilities.** Faerie Guidemother is a
1/1 flier; its AST carried `target creature gets +2/+1 and gains flying until
end of turn` as a top-level ability of the creature. Twice Upon a Time carried
the Adventure's own timing restriction, `cast this spell only if you control
two or more doctors`, on the creature. **171 such abilities across 153 cards.**

**2. The adventure / omen keyword was never emitted — and could not be.** The
words "Adventure" and "Omen" appear **only in the half's type line**
(`Sorcery — Adventure`), never in oracle text. `normalize()` reads oracle text
and nothing else, so the signal was not merely missed, it was never in scope.

Count across the whole 32,417-entry dataset:

```
AST Keyword name=adventure : 0
AST Keyword name=omen      : 0
```

`gameengine.HasAdventure`, `HasOmen` and `OmenCost` all gate on exactly that
node. All three were **false for every card in the corpus**, which in turn
left `CastAdventure` and `CastOmen` with **zero production callers** — three
dead layers stacked on one another, each of which looks fine in isolation.

## What r135 changes

`half_face(card)` recognises `layout == "adventure"` (Scryfall's layout for
both Adventurer and Omen cards — 170 of them, 154 Adventure + 16 Omen). Face 0
is the card proper and face 1 is the half; **verified 170/170, the half is
never face 0.** For those cards and only those:

- the parent parses from **face 0's text alone**, with face 0's own name
  tilded rather than the combined `A // B` name;
- the half parses into its **own `CardAST`** under the new additive
  `CardAST.faces`, mirrored by `Faces []*CardAST` in Go and decoded in
  `internal/astload/loader.go`;
- the parent gains `Keyword(name="adventure"|"omen", args=(half_mana_cost,))`,
  taken from the half's **type line and mana cost**, which is where the
  information actually lives.

The dataset row carries the half's source text under `faces` too. That is not
decoration: the row's `oracle_text` → `ast` pair is a finetuning target, and a
text containing the half against an AST that does not would be a mislabelled
example.

### Deliberately out of scope

`transform` (401), `modal_dfc` (100), `split` (137), `prepare` (64) and `flip`
(26) still concatenate. Each swaps characteristics under different rules and
wants its own pass; folding them in here would be a silent half-fix. **Measured:
0 of 32,264 non-adventure cards change.**

## Measurement (live corpus, `python3 -I` on .206)

```
keyword emitted:                     {'adventure': 140, 'omen': 13}   (was 0 and 0)
parent abilities removed:            171
abilities relocated onto the halves: 177
non-adventure cards changed:         0 of 32,264
```

The control that matters: pristine-HEAD parser vs r135 parser over the same
38,754-card dump — **153 cards differ, 0 of them outside `layout="adventure"`.**

That control also caught something I would otherwise have mis-attributed. A
first comparison against the live `ast_dataset.jsonl` showed 26 *non*-adventure
cards changing, which looked like my change leaking. It was not: those 26 gain
`Keyword(storied)` or `Keyword(teamwork)` from grammar that landed in
`scripts/parser.py` and **was never exported**. The live dataset is stale by two
whole keyword mechanics. `scripts/check_ast_freshness.py` exists to catch
exactly this and is referenced only from comments — nothing runs it.

## A fabricated-class citation, corrected in passing

`keywords_combat.go` headed the Adventure block `CR §702.133` and emitted that
number as the `rule` field of the `adventure` event — **as event data**.
**§702.133 is Jump-Start**, cited correctly forty lines below in the same file.
Adventure has no §702 section at all; it is a card frame defined in §715. This
is the same error r99 corrected for Omen (§702.176 is Impending).

The exile-and-recast the function performs is **§715.3d**: *"Instead of putting
a spell that was cast as an Adventure into its owner's graveyard as it
resolves, its controller exiles it. For as long as that card remains exiled,
that player may play it."* Both events now cite it.

## Still open, named rather than stubbed

- **`CastAdventure` never resolves the half.** It pays, exiles and grants the
  recast — the Adventure's actual effect is not put on the stack. Before r135
  there was nowhere to read it from; `CardAST.Faces` is what that fix needs.
- **§715.3a/b and §720.2/§720.3b alternative characteristics** — the engine
  casts the card itself, not an object wearing the half's name, type line and
  text.
- **Neither `CastAdventure` nor `CastOmen` has a production caller.** The
  keyword now being true is a precondition for wiring them, not the wiring.

So r135 does not make Adventure playable. It makes it *representable*, and it
removes 171 abilities that were wrong on cards people are playing today.

## Verification

- `scripts/test_adventure_faces_r135.py` — inline fixtures, no corpus needed.
  **5 of its 10 assertions are red pre-change** (semantics-only revert: new
  schema, pristine `parser.py`). The rest are must-never guards: the parent
  keeps its own printed `flying`, and a transform DFC gains no face and no
  keyword.
- `internal/astload/adventure_faces_r135_test.go` — **1 of 3 red pre-change**
  (mutation: drop the `faces` decode block). The other two are vacuous against
  that mutation and are said to be: the keyword in those rows is hand-written
  JSON, so they pin the decode contract, not the emission. The emission's red
  test is the Python one.
- Gate green across gameengine, counters, instanceid, per_card, hat,
  tournament, judge, judge/outcome, judge/progression, crcite, paritycheck,
  astload, gameast. hexapi its three known reds.
