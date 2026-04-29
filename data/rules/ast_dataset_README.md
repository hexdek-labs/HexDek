---
license: mit
task_categories:
- text2text-generation
- text-classification
language:
- en
tags:
- magic-the-gathering
- mtg
- abstract-syntax-tree
- ast
- games
- structured-generation
size_categories:
- 10K<n<100K
pretty_name: MTG Typed AST Dataset
---

# MTG Oracle-Text Typed AST Dataset

A finetuning-grade corpus of every printed Magic: The Gathering card
(post-filtering: 31,963 cards) paired with its **typed abstract syntax tree**
as produced by the [mtgsquad](https://github.com/) oracle-text parser.

Each AST is emitted per the MTG Comprehensive Rules §113 ability taxonomy:

- `Static` — always-on rules text (e.g. *"Flying"*, *"Creatures you control get +1/+1"*)
- `Activated` — cost/effect abilities (e.g. *"{T}: Add {G}"*)
- `Triggered` — When/Whenever/At-the-beginning-of events
- `Keyword` — named shorthand (Flying, Trample, Flashback, ...)

...and ~40 typed effect nodes (`Damage`, `Draw`, `Destroy`, `Exile`, `Bounce`,
`Buff`, `CreateToken`, `AddMana`, `Tutor`, `Reanimate`, `GainLife`, `LoseLife`,
`Discard`, `Mill`, `CounterMod`, `Sequence`, `Choice`, `Optional_`,
`Conditional`, `UnknownEffect`, and more).

Parser syntactic coverage: **100% GREEN** on all 31,963 real cards.
A majority of cards carry at least one `Modification(kind="custom", ...)`
stub intended for the runtime layer; that information is preserved in this
dataset so downstream consumers can filter on it. See
`data/rules/coverage_honest.md` in the mtgsquad repository for live
structural / mixed / stub / vanilla breakdown.

## File

- `ast_dataset.jsonl` — one JSON object per card, newline-delimited.

## Schema

```jsonc
{
  "name": "Lightning Bolt",
  "oracle_text": "Lightning Bolt deals 3 damage to any target.",
  "type_line": "Instant",
  "mana_cost": "{R}",
  "cmc": 1,
  "colors": ["R"],
  "ast": {
    "__ast_type__": "CardAST",
    "name": "Lightning Bolt",
    "abilities": [ /* typed ability nodes */ ],
    "parse_errors": [],
    "fully_parsed": true
  }
}
```

Every dataclass-backed AST node carries an `__ast_type__` field naming its
Python class (e.g. `Static`, `Activated`, `Triggered`, `Keyword`, `Damage`,
`Sequence`, `Choice`, ...) so consumers can discriminate structurally-similar
nodes without hand-rolled schema inference.

See `scripts/mtg_ast.py` in the mtgsquad repository for the full type
definitions (frozen dataclasses).

## Example row (verbatim first line of the JSONL)

```json
{"name": "Nissa, Worldsoul Speaker", "oracle_text": "Landfall — Whenever a land you control enters, you get {E}{E} (two energy counters).\nYou may pay eight {E} rather than pay the mana cost for permanent spells you cast.", "type_line": "Legendary Creature — Elf Druid", "mana_cost": "{3}{G}", "cmc": 4.0, "colors": ["G"], "ast": {"__ast_type__": "CardAST", "name": "Nissa, Worldsoul Speaker", "abilities": [{"__ast_type__": "Static", "condition": null, "modification": {"__ast_type__": "Modification", "kind": "ability_word", "args": ["landfall", "triggered", {"__ast_type__": "Triggered", "trigger": {"__ast_type__": "Trigger", "event": "tribe_you_control_etb", "actor": null, "target_filter": null, "phase": null, "controller": null, "condition": null}, "effect": {"__ast_type__": "Static", "condition": null, "modification": {"__ast_type__": "Modification", "kind": "gain_energy", "args": [2], "layer": null}, "raw": ""}, "intervening_if": null, "raw": "whenever a land you control enters, you get {e}{e}"}], "layer": null}, "raw": "landfall - whenever a land you control enters, you get {e}{e}"}, {"__ast_type__": "Static", "condition": null, "modification": {"__ast_type__": "Modification", "kind": "rather_than_pay", "args": ["eight {e}", "the mana cost for permanent spells you cast"], "layer": null}, "raw": "you may pay eight {e} rather than pay the mana cost for permanent spells you cast"}], "parse_errors": [], "fully_parsed": true, "morph_cost": null, "disguise_cost": null, "manifest_token": false, "has_morph": false, "has_megamorph": false, "has_disguise": false}}
```

## Use cases

- **LoRA / full finetuning** of an LLM to emit a typed AST from oracle text
  (use `finetune_pairs.jsonl` in this folder for the instruction-format
  variant).
- **Retrieval augmentation** — cluster cards by AST signature.
- **Rules-engine validation** — round-trip AST → text → AST to measure
  semantic stability of a generated model.
- **Ability classification** — supervised targets are the `__ast_type__`
  tags on each ability node.

## License

Code (parser, AST schema, serializer): **MIT**.

Oracle text: Magic: The Gathering oracle text is a property of Wizards of the
Coast. This dataset is distributed for research / educational purposes under
Wizards' Fan Content Policy. Card text originates from the [Scryfall bulk
data API](https://scryfall.com/docs/api/bulk-data) (oracle-cards dump).
Please credit Scryfall when redistributing:

> Unofficial Fan Content permitted under the Fan Content Policy. Not
> approved/endorsed by Wizards. Portions of the materials used are property of
> Wizards of the Coast. ©Wizards of the Coast LLC.

## Citation

```bibtex
@misc{mtgsquad_ast_dataset,
  title  = {MTG Oracle-Text Typed AST Dataset},
  author = {mtgsquad contributors},
  year   = {2026},
  note   = {Typed AST per MTG Comprehensive Rules §113,
            sourced from Scryfall oracle-cards bulk data.}
}
```

Regenerate with:

```bash
python3 scripts/export_ast_dataset.py
```
