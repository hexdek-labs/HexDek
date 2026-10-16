# r144 — printed keywords swallowed by long-tail customs

**Status: measured and filed. No code.** The fix is an exporter change and
needs its own round with a full corpus count and a Python regression
(the r135/r140 recipe).

## What I set out to do

The work order's next item was "the rest of the new set", with Titanic
Mistake's description: *"A few new global statics, like a card that says
all instants and sorceries you cast have split second."* The standing
instruction is to grep the corpus for the class **before** building.

That grep answered the question and then opened a much larger one.

## The new-set item: do not build

**There are exactly two global split-second granters in the oracle dump:**

| card | text | status |
|---|---|---|
| Shadow the Hedgehog | "Each spell you cast has split second if mana from an artifact was spent" | in the dataset, **inert** — needs mana provenance, tracked nowhere (r137) |
| The Misty Stepper | "Spells you cast have split second" | **NOT in the AST dataset** — filtered by `is_real_card`; set `unk` / "Unknown Event" |

`SplitSecondActive` has three arms and all three are scoped to items **on
the stack**: the source permanent's keyword, the spell card's keyword, and
r137's conditional `itemGrantsSplitSecond`. **There is no arm that scans
the battlefield for a permanent granting split second to its controller's
spells.** That gap is real — and building it today would light up **zero**
live cards, because the only unconditional member of the class is not in
the dataset and the only dataset member is blocked on something else
entirely. **Do not build it until a card in the class is actually
playable.**

Worth recording: the oracle dump is **not** stale — it carries sets up to
Star Trek (`trk`, 2026-11-13), including Reality Fracture (`fra`/`frc`,
2026-10). The new set is present. The described card simply is not in it.

## The larger finding: 102 cards whose printed keyword has no node

While checking how split second parses, I found Sudden Substitution — a
genuine printed Split-second card — whose **entire AST is one Static**:

```
Static  custom  '<long-tail:sudden_substitution>'
```

No `Keyword` node. `cardHasSplitSecond` walks for `*gameast.Keyword`, so
it returns false and **Sudden Substitution's split second is inert.**

That is not one card. **102 cards have an AST consisting ENTIRELY of
`<long-tail:...>` custom Statics while their oracle text BEGINS with a
printed keyword:**

```
flying 40   trample 11   flash 10   vigilance 9   haste 5   menace 5
convoke 3   deathtouch 3   reach 3   hexproof 2   ward 2   indestructible 2
```

### Why that matters: there is no oracle-text fallback

`(*Permanent).HasKeyword` (combat.go:54) reads exactly four sources:

1. AST `*gameast.Keyword` nodes
2. `GrantedAbilities`
3. `Flags["kw:"+name]`
4. keyword counters (CR §122.1b)

**None of them is the oracle text.** A card with no Keyword node and no
handler setting a flag does not have the keyword, as far as the engine is
concerned.

### How much of it is actually broken

Verified by reading, three of three:

- **Kykar, Zephyr Awakener** — handler exists and grants flying to the
  **tokens it creates**, not to Kykar. Kykar does not fly.
- **Skullbriar, the Walking Grave** — handler exists and manages counters.
  Nothing grants haste.
- **Trazyn the Infinite** — **no handler file at all.** Deathtouch has no
  possible source.

A name-token proxy over the per_card directory says **57 of the 102 have
no handler file whose name matches the card at all** — 21 of those are
flying. Samples: Magmatic Hellkite, Ochre Jelly, Keranos God of Storms,
Zilortha Strength Incarnate, Voracious Fell Beast, Doorkeeper Thrull,
Salvation Swan, Imoen Mystic Trickster.

**That 57 is a PROXY, not a count.** A handler could exist under a
different filename. The honest statement is: the class is 102, three are
confirmed broken by reading, and the proxy suggests roughly half have no
handler at all. **Someone must do the real per-card check before the
number 57 is quoted as fact.**

## Where the fix goes

**The exporter, not the engine.** The long-tail handler
(`scripts/extensions/long_tail_handlers.py`) replaces a card's whole
ability list with one custom Static. It should emit the printed leading
keywords as real `Keyword` nodes **alongside** the long-tail, exactly as
the ordinary path does. That is additive, costs the engine nothing, and
fixes every member of the class at once rather than 102 per-card patches.

Doing it in the engine instead — an oracle-text fallback in `HasKeyword` —
would be the wrong place: it would make a text scan load-bearing in the
hottest keyword path in the engine.

### Required verification for that round

Per the r135/r140 Python recipe, and non-negotiable given r140's lesson
that an "exporter-only" change moved play in 858 of 1,727 decks:

1. Run the exporter over the real corpus on .206 and **count** what
   changed — Keyword nodes added, by name, and confirm nothing else moves.
2. A `scripts/test_*.py` regression with inline Scryfall-shaped fixtures
   that **calls `P.load_extensions()`** (r140).
3. **Count live-deck exposure before shipping.** Granting flying to 40
   cards changes combat across the pool and is another ELO
   incomparability point.
4. Check for the near-miss: a card whose text merely *mentions* a keyword
   must not acquire it. Only a leading printed keyword line counts.
