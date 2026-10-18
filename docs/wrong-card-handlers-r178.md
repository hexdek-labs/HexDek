# r178 — handlers implementing the wrong card

**Status:** measurement. No production code changed. Findings are a FLOOR
over a subset, not a census.

## Why this exists

`Golbez, Crystal Collector` was found by reading, not by any detector.
Its handler carries its own `// Oracle text:` comment block, that block
faithfully describes the handler's behaviour, and **the text is not
Golbez's**. Nothing in the repo cross-checks a `per_card` handler against
the real corpus, so a handler written against wrong oracle text looks
healthy forever: it compiles, it registers, its tests pass against the
text its author believed.

This is worse than an inert card. An inert card is weak. A card with
fabricated abilities **changes game outcomes and corrupts ELO** — Golbez
grants an activated ability the real card does not have, which the AI
can and does use.

## Method

For each file in `internal/gameengine/per_card/`:

1. Extract every quoted string in an `On*(...)` registration, then keep
   only those that are **real card names in the live corpus**
   (`~/hexdek/data/rules/ast_dataset.jsonl` on .206, 32,417 rows).
2. Require **exactly one** real card name per file, so the oracle block's
   attribution is unambiguous.
3. Require **exactly one** `// Oracle text:` block, and extract it.
4. Compare its content words against the corpus `oracle_text` by
   coverage = |claimed ∩ real| / |claimed|, after stripping mana symbols
   and MTG stop-words.

Result: **415 handlers compared, 0 card names unresolved.**

```
coverage 0.00-0.20   no overlap — wrong card      5
coverage 0.20-0.35   very low                     2
coverage 0.35-0.50   low                          1
coverage >=0.50      plausible match            407
```

## TWO EARLIER VERSIONS OF THIS DETECTOR WERE WRONG, AND HOW

Recorded because both failures are the kind that produce a confident
wrong number.

**Attempt 1 — compare handler trigger events against corpus AST trigger
events.** Produced **455 flags, almost all false**. Handler events and
parser events are DIFFERENT VOCABULARIES: a handler registers the
engine's canonical `spell_cast` while the corpus carries the parser's
generic `cast_filtered`. Same trigger, different granularity, and the
alias table does not bridge them. Adeliz, the Cinder Wind was flagged
while being entirely correct. Abandoned.

**Attempt 2 — oracle-comment comparison without the single-card
filter.** Produced **55 flags, mostly false**. Multi-card files like
`game_changers.go` hold one oracle block; my extractor attributed it to
every card registered in the file, so eight unrelated cards all "claimed"
*All creatures lose all abilities and have base power and toughness 1/1*.

**Attempt 3 caught its own false negative.** The first single-card
version **excluded Golbez** — the one case known to be real — because
the name extractor treated the event string `"spell_cast"` as a second
card name, making the file look multi-card. Validating names against the
corpus fixed it, and Golbez now ranks worst at coverage 0.00.

**The lesson generalises: a detector that does not find the case you
already know about is not yet a detector.** Attempts 1 and 2 would both
have been reported as findings if the known case had not been there to
check them against.

## The eight flags

Two are MY EXTRACTOR'S artifact, not wrong cards: `Tam, Observant
Sequencer // Deep Sight` (0.12) and `Lluwen, Exchange Student // Pest
Friend` (0.14) are DFCs whose comment begins `Front face — ...`, a
format the extractor truncates to a header line. **Not bugs.**

The remaining six are genuine mismatches between a handler's own
documented text and the real card:

    
    ### Golbez, Crystal Collector   [golbez.go]  coverage=0.00
      handler claims: Whenever you cast a spell from your graveyard or from exile, create a Treasure token. {2}{B}: Exile target card from a graveyard. You may cast it from
    ### Lluwen, Exchange Student // Pest Friend   [gen_lluwen_exchange_student_pest_friend.go]  coverage=0.14
      handler claims: Front face — Lluwen, Exchange Student ({2}{B}{G}, Legendary Creature — Human Student, 2/4):
      REAL card     : Lluwen enters prepared. (While it's prepared, you may cast a copy of its spell. Doing so unprepares it.) / Exile a creature card from your graveyard: Ll
    
    ### Teval, the Balanced Scale   [teval.go]  coverage=0.14
      handler claims: At the beginning of combat on your turn, if an opponent controls the most creatures, create a 3/3 green Beast creature token. If you control the most 
      REAL card     : Flying / Whenever Teval attacks, mill three cards. Then you may return a land card from your graveyard to the battlefield tapped. / Whenever one or more c
    
    ### Rootha, Mastering the Moment   [rootha.go]  coverage=0.15
      handler claims: {1}{U}{R}: Copy target instant or sorcery you control. You may choose new targets for the copy. Return Rootha, Mastering the Moment to its owner's han
      REAL card     : At the beginning of combat on your turn, if you've cast an instant or sorcery spell this turn, create an X/X blue and red Elemental creature token wit
    
    ### Berta, Wise Extrapolator   [berta_wise_extrapolator.go]  coverage=0.23
      handler claims: At the beginning of your end step, if you gained 3 or more life this turn, draw a card. {2}{W}{B}: Create a 1/1 white Cleric creature token with lifel
      REAL card     : Increment (Whenever you cast a spell, if the amount of mana you spent is greater than this creature's power or toughness, put a +1/+1 counter on this 
    
    ### Jace, Wielder of Mysteries   [jace_wielder_of_mysteries.go]  coverage=0.33
      handler claims: +1: Draw a card. -8: Exile the top ten cards of your library. You may cast any number of nonland cards from among them without paying their mana costs
      REAL card     : If you would draw a card while your library has no cards in it, you win the game instead. / +1: Target player mills two cards. Draw a card. / −8: Draw sev
    
    ### Ms. Bumbleflower   [ms_bumbleflower.go]  coverage=0.38

**Only Golbez is confirmed by a code read** — its handler implements the
text in its comment, so the comment is not a stale note, it is what the
code does. The other five are strong suspects whose handler bodies have
NOT been read; a handler could in principle carry a wrong comment while
implementing the right card.

## What this does NOT cover

- **71 multi-card files** excluded for ambiguous attribution.
- **426 files** with no corpus-resolvable single card name or no usable
  oracle block.
- Handlers with **no `// Oracle text:` comment at all** are invisible to
  this method — a handler cannot be caught contradicting a claim it
  never makes.
- Coverage ≥ 0.50 is treated as plausible **without inspection**. A
  handler could implement two of a card's three abilities and score
  well. This finds wrong cards, not incomplete ones.

So **six** is a floor over roughly a third of the handler corpus.

## Suggested follow-up

The cheap durable win is to require an `// Oracle text:` block on every
single-card handler and gate it against the corpus in CI. The corpus is
not in CI today (it is a 46MB gitignored file), so this needs either a
fetch step or a checked-in digest of name → oracle hash.

That is an owner call, not a drive-by.
