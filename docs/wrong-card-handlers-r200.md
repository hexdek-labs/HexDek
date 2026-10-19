# r199 — scoring the handler BODY against the real oracle text

**Status:** measurement. **No production code changed.** Deliverable is
`scripts/detect_wrong_card_handlers_r199.py` plus this document.

Supersedes the detector in `docs/wrong-card-handlers-r178.md`; builds on
the hand reads in `docs/wrong-card-handlers-r179.md`.

---

## 0. Two corrections to the brief, up front

**(a) The "426 unresolvable files" figure in r178 is wrong, and it is not
drift.** The per_card directory held **1184** non-test `.go` files at
r178's own commit (`a8851e80`) and holds 1184 today — identical. But
r178's partition is 415 + 71 + 426 = **912**, which leaves **272 files
unaccounted for in its own arithmetic**. Measured on current origin/main,
the population r178 did not cover is **767 files, not 426**. r178's prose
claim that it covered "roughly a third" is correct (417/1184 = 35.2%);
its residual count is not. Bucket breakdown in §4.

**(b) The 71 multi-card figure is essentially accurate** — I measure
**72**. See §4.

Everything else in the brief held up. The premise that r178 scores the
comment and therefore cannot order handlers by actual wrongness is
correct, and Jace is exactly the discriminating case the brief says it is.

---

## 1. The design, and why it fixes r178

r178 compared a handler's `// Oracle text:` comment to the corpus. That
measures the comment. A comment can be wrong about a handler that is
right — r178 ranked Jace (0.33) as a worse suspect than Ms. Bumbleflower
(0.38), and Jace is the correct one.

r199 extracts **claims** from the handler source and asks of each whether
the **real oracle text supports it**. The direction is the whole fix:

> **handler → oracle (precision):** is everything this handler does
> something the card actually says?

and explicitly **not** oracle → handler (recall). An *incomplete* handler
makes few claims and all of them are supported, so it scores clean. That
is the Jace discrimination, by construction rather than by threshold.

A claim is one of: a trigger-event subscription; an engine primitive the
body invokes; a token subtype it mints; a colour it mints; a granted
keyword; a power/toughness pump; or the presence of an `OnActivated`
hook. Each carries a predicate over the real oracle text.

Two deliberate abstentions:

- **`r.OnETB` is non-claiming on its own.** It is the engine's universal
  "install this card" hook — statics and replacement effects register
  there. Claims from an ETB handler come only from what its body does.
  This is what keeps Jace at zero claims.
- **Unknown trigger events abstain.** An event not in the vocabulary is
  never scored, never flagged.

The handler bodies are read **with comments stripped**, because r179
established that several `// Oracle text:` blocks in this repo are not
stale notes but fabrications. Comments are not evidence here.

**Per-card attribution.** Registrations are parsed as
`(card, kind, event, handler-func)`, and each card is scored only against
the bodies of *its own* registered functions (plus locally-defined
functions those call, depth 3). Multi-card files are therefore scored
per registered card, not per file — which is the r178 exclusion the brief
asked to lift.

**Corpus vs API.** Bulk scoring uses the 32,417-row
`ast_dataset.jsonl` fetched from `.206`. **Every card named as a finding
or a false positive in §3 had its oracle text re-fetched live from
`https://hexdek.dev/api/oracle/card/{name}`**, and all quoted text below
is from that fetch, never from a repo comment.

---

## 2. MUTATION CONTROL — predicted vs actual

Predictions were written before the first run
(`PREDICTIONS.md`, reproduced verbatim in §2.1). The adjudicated seven
from r179 are the calibration set.

### 2.1 What I predicted, before running

| Card | predicted frac | predicted verdict |
|---|---|---|
| Golbez, Crystal Collector | ~1.00 | FLAG |
| Teval, the Balanced Scale | ~0.75 | FLAG |
| Rootha, Mastering the Moment | ~1.00 | FLAG |
| Berta, Wise Extrapolator | ~0.83 | FLAG |
| High Perfect Morcant | ~1.00 | FLAG |
| Ms. Bumbleflower | ~0.40 | **MISS (predicted)** |
| Jace, Wielder of Mysteries | n/a, 0 claims | ABSTAIN (correct) |

**Predicted score: 5/6, with Ms. Bumbleflower predicted to escape** —
because three of its claims are satisfied by coincidence of vocabulary
(the real card does say "draw"; white is inside its GWU identity), so a
flat unsupported-fraction dilutes the two damning ones. I recorded that
predicted failure rather than pre-weighting to avoid it, on the grounds
that a weighting chosen to make a calibration pass is not a calibration.

I also predicted the 426 would not reconcile, and that ~272 files were
missing from r178's partition. Both held (§0a).

### 2.2 What actually happened

| Run | Golbez | Teval | Rootha | Berta | Bumble | Morcant | Jace | **score** | flags |
|---|---|---|---|---|---|---|---|---|---|
| v1 (as designed) | 1.00 ✔ | 1.00 ✔ | 1.00 ✔ | 0.67 ✔ | 0.50 ✘ | 1.00 ✔ | 0 ✔ | *see note* | 18 |
| v2 (after bug fix) | 1.00 ✔ | 0.50 ✘ | 1.00 ✔ | 0.86 ✔ | 0.40 ✘ | 0.67 ✔ | 0 ✔ | **4/6** | 27 |
| v5 final | 1.00 ✔ | 0.60 ✔ | 1.00 ✔ | 0.83 ✔ | 0.40→B ✔ | 0.67 ✔ | 0 ✔ | **6/6** | **12** |

Note on v1: the fractions look right but **four of the six had only 1–2
claims** and fell under the `claims >= 3` gate, so only Berta flagged.
v1's real score was **1/6**.

**v1 → v2 was a bug fix, not tuning.** `func_bodies` brace-matched from
the first `{` after the signature — which in every handler is the one
inside `ctx map[string]interface{}`. It closes immediately, so **every
handler body came back empty** and no body-derived claim was ever
extracted. Anchoring on the brace that ends its line (gofmt guarantees
this) fixed it. This is the single most important thing in this document
for anyone reusing the script: the detector looked like it was working
and was scoring nothing.

**v2 → v5 contains one genuine tuning step, declared:**

- **Rule B added.** `hard >= 2`, where a *hard* claim is one naming a
  concrete particular (token subtype, minted colour, granted keyword,
  activated-ability hook) rather than a generic verb. This was added
  after seeing the v2 result, so it is post-hoc, and the pre-tuning
  number (4/6) is stated above rather than buried.

  It is not arbitrary. Measured base rates over the live corpus
  (n=32,417): creature subtypes `beast` 0.003, `cleric` 0.003,
  `rabbit` 0.001, `lifelink` 0.021 — against generic verbs `token`
  0.131, `draw` 0.137, `enters` 0.232. A *failed* particular is 50–200×
  more discriminating than a *passed* generic is reassuring, so a flat
  ratio whose denominator is full of near-tautological matches
  systematically under-ranks a handler that is wrong about specifics
  while phrased in common verbs. Rule B is scale-free and does not
  touch Rule A.

- **The pump claim was implemented.** `Modification{Power/Toughness:N}`
  → the oracle must print `+N/+N`. This was in my written prediction for
  Teval and simply had not been coded; adding it is closing a gap
  between the stated design and the implementation, and it is what moves
  Teval from 0.50 to 0.60.

### 2.3 Chasing the two that stayed green

The brief says chase a green mutation rather than note it. Both had the
same cause and it is worth stating as a finding in its own right:

> **The flat unsupported-fraction is a ratio, and its denominator is
> contaminated by vacuous agreement.** Teval scored `CreateCreatureToken
> = ok` (the card does make *a* token — just a 2/2 black Zombie Druid,
> not a 3/3 green Beast) and `color:green = ok` (green is in its BGU
> identity). Bumbleflower scored `drawOne = ok` and `color:white = ok`
> the same way. In both cases the handler is wrong about every
> *particular* and right about every *generality*, and the generalities
> outvote.

Teval is recoverable inside Rule A once the pump claim exists (3/5 =
0.60). Bumbleflower is not recoverable by any threshold on a flat ratio
— it is 2/5 and its two failures are both hard. It needs Rule B, which
is why Rule B exists.

### 2.4 Jace — the negative control held throughout

Jace scored **0 claims in every version**. Its `OnETB` body calls
`RegisterJaceWielderOfMysteries` and `emit`/`emitPartial` and performs no
game-visible action, so there is nothing to contradict. It was never
flagged, including in v1. The incompleteness r179 found (two unimplemented
loyalty abilities) is invisible to this detector *by design* — this method
measures precision, not completeness, and those are different audits.

---

## 3. Results — 12 flags, and what READING says about each

**A score is a suspect, not a verdict.** The table separates what was
scored from what was read. Every row marked READ had its handler body
read in full and its oracle text fetched live from the API.

| # | Card | File | frac / hard | Scored | **READ verdict** |
|---|------|------|---|---|---|
| 1 | Berta, Wise Extrapolator | `berta_wise_extrapolator.go` | 0.83 / 3 | suspect | WRONG CARD *(r179, known)* |
| 2 | Golbez, Crystal Collector | `golbez.go` | 1.00 / 1 | suspect | WRONG CARD *(r178/r179, known)* |
| 3 | Rootha, Mastering the Moment | `rootha.go` | 1.00 / 1 | suspect | WRONG CARD *(r179, known)* |
| 4 | Teval, the Balanced Scale | `teval.go` | 0.60 / 1 | suspect | WRONG CARD *(r179, known)* |
| 5 | Ms. Bumbleflower | `ms_bumbleflower.go` | 0.40 / 2 | suspect | WRONG CARD *(r179, known)* |
| 6 | High Perfect Morcant | `morcant.go` | 0.67 / 0 | suspect | WRONG CARD *(r179, known)* |
| 7 | **Kilo, Apogee Mind** | `kilo_apogee_mind.go` | 1.00 / 0 | suspect | **READ — WRONG CARD (new)** |
| 8 | **Emiel the Blessed** | `emiel.go` | 0.67 / 1 | suspect | **READ — FABRICATED ABILITY (new)** |
| 9 | **Magus Lucea Kane** | `magus_lucea_kane.go` | 0.67 / 0 | suspect | **READ — FABRICATED ABILITY (new)** |
| 10 | **Zopandrel, Hunger Dominus** | `custom_zopandrel_hunger_dominus.go` | 0.60 / 0 | suspect | **READ — FABRICATED ABILITY (new)** |
| 11 | Anointed Procession | `batch17_sweep.go` | 0.67 / 0 | suspect | **READ — FALSE POSITIVE** |
| 12 | Phylactery Lich | `chaos_cascade.go` | 0.67 / 0 | suspect | **READ — FALSE POSITIVE** |

**Precision on the new (non-calibration) flags: 4 true / 6 read = 67%.**
Overall 10/12 = 83%, but that includes the six it was calibrated on, so
67% is the honest number.

### 3.1 Kilo, Apogee Mind — WRONG CARD, and it says so itself

**Oracle (fetched):** `{U}{R}{W}` Legendary Artifact Creature — Robot
Artificer
> Haste
> Whenever Kilo becomes tapped, proliferate.

**Handler:** `OnETB` → draw one card per legendary creature you control.
`OnTrigger("spell_cast")` → scry 2 on casting a legendary spell.

No haste, no becomes-tapped trigger, no proliferate. Two fabricated
abilities in their place.

What makes this case worth the whole exercise is its comment:

> *"Note: The current Scryfall printing reads 'Haste / Whenever Kilo
> becomes tapped, proliferate.' The simplified spec above is what this
> engine implements; if the printed version is later wired up, replace
> this handler."*

The comment is **honest and accurate about the handler**. r178's
detector scores comment-against-corpus, so a handler whose comment
faithfully documents the wrong card is invisible to it — the comment and
the code agree, and both are wrong. Only a body-based method finds this
shape. I expect it is not the only one.

### 3.2 Emiel the Blessed — right card, second ability fabricated

**Oracle (fetched):** `{2}{W}{W}` Legendary Creature — Unicorn
> {3}: Exile another target creature you control, then return it to the
> battlefield under its owner's control.
> Whenever another creature you control enters, you may pay {G/W}. If you
> do, put a +1/+1 counter on it. If it's a Unicorn, put two +1/+1
> counters on it instead.

**Handler:** the `{3}` flicker is implemented and looks right. The rider
is not: on the flicker returning a creature, if the controller has ≥2
mana it spends 2 and calls
`CreateCreatureToken(..., "Beast Token", 3/3)`.

Three divergences: the payoff is a **3/3 Beast token instead of +1/+1
counters**; the Unicorn double-counter clause is absent; and the trigger
is bolted onto the flicker path only, where the real one fires on *any*
other creature you control entering. The `// Oracle text:` block in
`emiel.go` states the Beast-token version — another fabricated comment,
and the reason r178 scored this file as plausible.

### 3.3 Magus Lucea Kane — fabricated upkeep scry

**Oracle (fetched):** `{1}{G}{U}{R}` Legendary Creature — Human Tyranid
Wizard
> Spiritual Leader — At the beginning of combat on your turn, put a +1/+1
> counter on target creature.
> Psychic Stimulus — {T}: Add {C}{C}. When you next cast a spell with {X}
> in its mana cost or activate an ability with {X} in its activation cost
> this turn, copy that spell or ability. …

**Handler:** `OnTrigger("upkeep_controller")` → `Scry 1`, plus an
`OnActivated`. The card has **no upkeep trigger and no scry anywhere**.
The printed begin-combat +1/+1 counter ability is absent. The activated
half is at least the right shape (the card does have `{T}:`).

### 3.4 Zopandrel, Hunger Dominus — fabricated combat-damage payoff

**Oracle (fetched):** `{5}{G}{G}` Legendary Creature — Phyrexian Horror
> Reach
> At the beginning of each combat, double the power and toughness of each
> creature you control until end of turn.
> {G/P}{G/P}, Sacrifice two other creatures: Put an indestructible counter
> on Zopandrel.

**Handler:** `OnTrigger("combat_damage_to_player")` → draw a card and
gain life equal to the damage. That is wholly fabricated. The activated
ability (indestructible counter) is correct; the begin-combat P/T
doubling and reach are absent.

### 3.5 The two false positives, and the classes they represent

**Anointed Procession** (`batch17_sweep.go`) — oracle: *"If an effect
would create one or more tokens under your control, it creates twice that
many of those tokens instead."* The handler is a correct doubler that
enumerates every predefined token type to re-mint it, so
`CreateTreasureToken` / `Food` / `Clue` / `Blood` each fail a predicate
the generic oracle text cannot satisfy. **Class: generic effect doublers.**

**Phylactery Lich** (`chaos_cascade.go`) — oracle: *"When you control no
permanents with phylactery counters on them, sacrifice this creature."*
That is a state trigger. The handler polls it by subscribing to
`permanent_ltb` and `creature_dies` and re-checking the condition. Those
events are the *observation mechanism*, not claimed abilities. **Class:
state-trigger polling.** My `is_bookkeeping` guard does not catch it
because the handler does take a real action (`SacrificePermanent`).

Neither is worth hacking around; both are documented limitations.

### 3.6 Four false-positive classes killed by reading, not by tuning

Flags fell **35 → 12** across v3→v5 with the calibration set fully
intact. Every one of these was found by reading a flagged body and was an
extractor defect, not a threshold:

1. **Colour strings read outside a mint context.** Mother of Runes
   switches on `"pip:W".."pip:G"` to *count* an opponent's colours for its
   protection choice; a flat literal scan read that as minting four
   off-identity colours. Fixed by scoping colour and subtype extraction to
   `gameengine.Card{…}` literals and `Create*Token(…)` argument spans.
   Killed 2 flags (Mother of Runes, Preston the Vanisher).
2. **A card may legitimately mint a token outside its own colour
   identity.** Sand Scout is mono-white and prints *"create a 1/1 red,
   green, and white Sand Warrior"* — colour words in rules text are not
   mana symbols and add no identity. Predicate now accepts the colour
   *word* appearing in the oracle text as support.
3. **Map-key literals scored as keyword grants.** Lightning, Army of One
   reads `ctx["defender"]`; `"defender"` is in the grantable-keyword set.
   Fixed by only reading keyword literals from
   `SetKeywordFlags` / `GrantKeywordFlag` / `gameast.Keyword{…}` contexts.
   Also: the engine spells "can't be blocked" as `unblockable`, which made
   Bria, Riptide Rogue a false positive until the predicate learned both.
4. **Bookkeeping hooks scored as abilities.** This was the largest
   cluster. Lightning subscribes to `permanent_ltb` purely to unregister
   its own damage replacements; roughly a third of v3's flags were a
   `permanent_ltb` or `permanent_etb` cleanup hook. A trigger handler
   whose body contains no game-visible action is now not a claim.

One more predicate bug, caught on **Sefris of the Hidden Ways**: the
oracle reads *"put into **your** graveyard"* and my regex demanded *"put
into **a** graveyard"*. Sefris is correct — creature cards dying *and*
being discarded both satisfy its printed trigger — and cleared once the
wording was generalised.

### 3.7 The confession grep — two more wrong cards, and a blind spot
### this detector structurally cannot see

§3.1 noted that Kilo's comment openly declares it implements a different
card, and suggested grepping for that admission. I ran it rather than
leaving it as a proposal. Five files match; three are unrelated
(`not the printed value`, about power/toughness layers). Two are real,
and **one carries boilerplate identical to Kilo's**, which confirms a
batch rather than a one-off:

**Caesar, Legion's Emperor** (`caesar_legions_emperor.go`) — same closing
sentence as Kilo, word for word: *"The simplified spec above is what this
engine implements; if the printed version is later wired up, replace this
handler."*

> **Oracle (fetched):** Whenever you attack, you may sacrifice another
> creature. When you do, choose two — • Create two 1/1 red and white
> Soldier creature tokens with haste that are tapped and attacking. •
> You draw a card and you lose 1 life. • Caesar deals damage equal to the
> number of creature tokens you control to target opponent.

The handler drops the sacrifice cost and the modal choose-two entirely,
makes **one** 1/1 **red** Soldier instead of two 1/1 **red and white**
ones, and adds a fabricated *"whenever a nontoken creature dies during
combat, draw a card"*. The comment also misquotes the third mode as a
flat 3 damage.

**Aragorn, the Uniter** (`aragorn_the_uniter.go`) — *"Implementation
simplification (per task spec)"*.

> **Oracle (fetched):** Whenever you cast a white spell, create a 1/1
> white Human Soldier creature token. / Whenever you cast a blue spell,
> scry 2. / Whenever you cast a red spell, Aragorn deals 3 damage to
> target opponent. / Whenever you cast a green spell, target creature
> gets +4/+4 until end of turn.

Four independent per-colour triggers. The handler instead gates on the
spell being **multicoloured** (`len(Colors) >= 2`), so it **never fires
on a mono-coloured spell at all** and fires several modes at once on a
multicoloured one. Of the four modes it does implement, the white one is
a fabricated *gain 5 life* (really: a white Soldier token) and the green
one is a *green* Human Soldier token (really: +4/+4 to target creature).
Its `// Oracle text:` block states the fabricated version.

**Now the uncomfortable part. My detector scores both of these clean:**

| Card | claims | unsupported | frac | hard |
|---|---:|---:|---:|---:|
| Caesar, Legion's Emperor | 4 | 1 | **0.25** | 0 |
| Aragorn, the Uniter | 8 | 1 | **0.125** | 0 |

Aragorn at 0.125 is cleaner than the median flagged handler. The reason
is structural and not fixable by tuning:

> **A precision detector cannot see a handler that is wrong while reusing
> the real card's vocabulary.** Aragorn really does scry, really does
> deal damage, really does make a Human Soldier token, and green really
> is in its identity — so seven of eight claims are literally supported.
> What is wrong is the *binding*: which trigger drives which effect, and
> under what condition. Claim-level support is blind to binding.

This is the mirror image of r178's flaw. r178 could not see a handler
whose comment was honest; r199 cannot see a handler whose vocabulary is
honest. **The two instruments are complementary, and the five-second
confession grep found two cards that 1479 scored attributions did not.**

### 3.8 A correction to r179's Rootha proposal

r179 proposed re-registering `rootha.go` under `Rootha, Mercurial Artist`
and explicitly flagged that it had **not** fetched that card. I fetched it:

> `{1}{U}{R}` Legendary Creature — Orc Shaman
> **{2}, Return Rootha to its owner's hand:** Copy target instant or
> sorcery spell you control. You may choose new targets for the copy.

The handler copies the spell and *then* bounces Rootha as an **effect**.
On the real card the bounce is part of the **activation cost**, and the
mana is `{2}`, not the `{1}{U}{R}` r179's comment quoted. So the re-filing
proposal is **not** a free win: it would ship a card that is still wrong
about both its cost and its ordering. r179's lead is downgraded.

---

## 4. Coverage — the 71 and the 426, measured

Measured on current origin/main, `internal/gameengine/per_card`,
non-test `.go` files only.

| Bucket | Files | r178 | r199 |
|---|---:|---|---|
| **A** no registration at all (shared helper / family file) | 64 | — | — |
| **B** registers names, none corpus-resolvable | 12 | — | — |
| **C** multi-card file (ambiguous attribution for r178) | **72** | excluded | **covered, per card** |
| **D** single card, **no** `// Oracle text:` block | **619** | invisible | **covered** |
| **F** single card + one oracle block (r178's set) | 417 | covered | covered |
| **total** | **1184** | | |

- **"71 multi-card files" → 72.** Accurate.
- **"426 unresolvable" → 767.** r178's partition sums to 912 against a
  directory of 1184 at its own commit, so 272 files were simply missing
  from it. The r199 detector scores **1479 card-handler attributions**
  across buckets C, D and F — including **444 attributions inside the 72
  multi-card files** and **947 attributions with no `// Oracle text:`
  comment at all**, both of which were wholly invisible to r178.

**Why bucket B does not resolve (12 files, 78 attributions):**

| Reason | Files |
|---|---:|
| one **face** of a DFC; the corpus keys the combined `A // B` name | 11 |
| genuinely absent from the 32,417-row corpus | 5 |

(Counts exceed 12 because several files register more than one such
name.) The DFC-face class is a cheap future win: index each face of every
`A // B` corpus entry under its own name and 11 of the 12 resolve. The
remaining handful are cards the corpus does not carry — a corpus
freshness question, not a handler question.

**Scorable fraction.** Of 1479 attributions, **359 produce ≥3 claims**.
The rest are handlers too thin to judge: a single trigger, or an ETB that
only registers a static. That is not a gap this method can close — a
handler that asserts almost nothing cannot be caught contradicting
anything. It is the precision/completeness boundary again.

---

## 5. What this does NOT establish

- **12 flags is a floor, not a census.** 359 of 1479 attributions are
  scorable; the other 1120 are below the claim floor and were not judged.
- **No runtime verification.** Everything here is source reading plus
  fetched oracle text. Whether a fabricated ability actually fires in a
  game — and r179's analysis of the ETB-suppression vs additive seams —
  was not re-confirmed by running anything.
- **Rule B is post-hoc.** Its base-rate justification was measured, but
  it was added after seeing that v2 scored 4/6. The pre-tuning number is
  in §2.2 and the v1 number (1/6) is there too.
- **Six of the twelve flags are the calibration set.** Quoting 83%
  precision would be circular; the figure that means anything is **4
  true of 6 newly read**.
- **The two documented false-positive classes** (generic doublers,
  state-trigger polling) are open. Neither was worked around.
- **A clean score is not a clearance.** Caesar (0.25) and Aragorn
  (0.125) are both genuinely wrong and both score cleaner than most of
  the corpus (§3.7). Nothing in this document licenses treating a low
  `frac` as evidence a handler is correct.
- **Counts of zero are not claimed.** No handler outside the 12 flags
  and the 2 grep hits was examined; "none observed" is the strongest
  statement available about the rest.
- **The confession grep matched on wording I guessed.** Five files for
  one set of phrases. It is a lower bound on that batch, not its size.

## 6. Suggested follow-up (owner call, nothing applied)

1. **Deal with the "per task spec" batch first.** Kilo, Caesar and
   Aragorn were all written against a supplied spec that was not the
   card, and Kilo and Caesar share a verbatim boilerplate sentence. Three
   found from one grep means the batch is probably larger than the
   wording I happened to guess. Whoever owns this should find the batch
   by provenance — the commit(s) that introduced that boilerplate — not
   by more string matching. **Two of the three are invisible to the r199
   detector (§3.7), so this cannot be delegated to the score.**
2. The r178 CI proposal still stands and is better motivated, but it
   should gate the **body**, not the comment — a comment gate would have
   passed Kilo, and a body gate would still have passed Aragorn. Neither
   instrument alone is sufficient; the honest framing is that these find
   different, partly disjoint populations.
3. Index DFC faces in the corpus loader; 11 of the 12 bucket-B files
   resolve for free.
4. r179's Rootha re-filing proposal should be withdrawn or reworked
   (§3.8).
5. **A binding-level check is the real next instrument.** Both classes
   this audit cannot see are binding errors: which trigger drives which
   effect. That needs the handler's control flow compared against the
   card's AST structure, not a bag of claims — a materially harder build,
   and worth scoping before committing to.

---

## Verification pass by the commissioning session (r200)

The agent was briefed to argue with its brief and did. I re-checked its
load-bearing claims myself rather than accept them. Results below.

### VERIFIED — the "426 unresolvable files" figure is wrong

```
per_card non-test .go files at r178's own commit a8851e80 : 1184
per_card non-test .go files today                         : 1184
r178's own partition  415 + 71 + 426                      :  912
unaccounted for in r178's arithmetic                      :  272
```

Identical file counts then and now, so **this is not corpus drift — it
is a gap in r178's own accounting.** r178's prose claim ("six is a
floor over roughly a third of the handler corpus") survives; its
residual number does not. Anywhere "426" has been quoted, including in
my own tick briefs, should read "not reliably known; r178 covered 417
of 1184".

### VERIFIED — r179's Rootha entry needs correcting, in a direction that makes it worse

Fetched live:

```
Rootha, Mercurial Artist   {1}{U}{R}
  "{2}, Return Rootha to its owner's hand: Copy target instant or
   sorcery spell you control. You may choose new targets for the copy."
Rootha, Mastering the Moment  {2}{U}{R}
  begin-combat X/X blue-and-red Elemental with flying and haste
```

The handler's own comment claims an ability that reads
`{1}{U}{R}: Copy target instant or sorcery you control... Return Rootha
to its owner's hand.`

**That text is on neither card.** On the real Mercurial Artist the
bounce is part of the ACTIVATION COST and the cost is `{2}`, not
`{1}{U}{R}` (which is that card's *mana cost*, a different thing).

So r179's verdict **WRONG CARD stands and is unchanged** — Mastering
the Moment's real begin-combat trigger is entirely absent. What must be
withdrawn is the narrower identification: this is **not** "the other
Rootha implemented by mistake". It is a third ability that exists on no
printed card, assembled out of one card's effect and a fabricated cost.
The comment is the source of the fabrication, which is the standing
rule about repo comments holding exactly.

I am recording this as a partial withdrawal rather than accepting the
agent's "withdraw the re-filing proposal" in full, because the agent's
phrasing could be read as softening the WRONG CARD verdict. It does not.

### VERIFIED — the three self-confessing handlers, and the batch is bounded at 3

Matched after joining wrapped comment lines (a single-line grep finds
only one of them and undercounts — the boilerplate wraps):

```
simplified-spec boilerplate  2   caesar_legions_emperor.go, kilo_apogee_mind.go
"per task spec"              3   + aragorn_the_uniter.go
"current Scryfall printing"  2   caesar_legions_emperor.go, kilo_apogee_mind.go
```

Caesar and Kilo carry the same boilerplate: *"The current Scryfall
printing reads … The simplified spec above is what this engine
implements; if the printed version is later wired up, replace this
handler."* Aragorn says *"Implementation simplification (per task
spec)"*.

**The batch is exactly 3, not open-ended.** Worth stating, because
"it's a batch" invites an unbounded reading.

**The selection effect must be stated with the result:** this grep
finds precisely the handlers whose author was honest enough to document
the divergence. It is a lower bound on confessed divergence and says
**nothing** about silent divergence. It is not a cheaper substitute for
the detector; it is a different population.

### NOT VERIFIED — the bucket table does not reconcile with my own count

The agent's buckets sum to 1184 exactly (64+12+72+619+417), which is
good internal arithmetic. But an independent count disagrees:

```
non-test files                               1184
files mentioning an oracle-text comment       993
files with none                               191
```

The agent reports **619** single-card files with **no** `// Oracle
text:` block. I find only **191** files with no oracle-text comment of
any kind. Those are not the same predicate — mine is looser, and
multi-card or multi-block files could account for part of the gap — but
993 − 417 − 72 = 504 files remain unexplained under either reading.

**Do not quote 619, 417, 1479, or 947 until this reconciles.** The
partition may be right and my predicate too loose; I have not
established which. Flagged, not resolved.

### The result I would actually act on

The agent ran the "grep for comments confessing divergence" idea
instead of merely filing it, and found two genuine wrong cards
(Caesar, Aragorn) that **its own detector scored 0.25 and 0.125 —
cleaner than most of the corpus.**

The reason is structural and generalises past this audit:

- r178 could not see a handler carrying an **honest comment**.
- r199's detector cannot see a handler that is wrong while reusing the
  real card's **vocabulary**. Aragorn genuinely scries, genuinely deals
  damage, genuinely makes a Human Soldier token. What is wrong is the
  **binding** — which trigger drives which effect, and the fact that
  the real card's "choose one per colour, no repeats" structure is
  absent entirely.

**A low divergence score is not a clearance.** Both detectors measure
vocabulary overlap; neither measures structure. Any future detector for
this class has to compare the trigger→effect graph, not the bag of
primitives.

### The v1 bug is the most reusable thing in this report

The agent's first detector scored **1 of 6** on the calibration set.
The cause was not a threshold: `func_bodies` brace-matched from the
first `{` after the signature, which in every Go handler lands inside
`ctx map[string]interface{}`. That brace closes immediately, so **every
handler body came back empty and no body-derived claim was ever
extracted.** The detector emitted plausible per-card scores throughout.

This is the inert-detector pattern again, in its worst form: not a
detector that fails loudly, but one that **scores nothing while
appearing to work**. The calibration set is the only reason it was
caught — which is precisely the argument for keeping one.

### Mutation control — predicted before running, as required

| | Golbez | Teval | Rootha | Berta | Bumble | Morcant | Jace | score |
|---|---|---|---|---|---|---|---|---|
| predicted | FLAG | FLAG | FLAG | FLAG | **MISS** | FLAG | abstain | **5/6** |
| v1 as designed | ✔ | ✔ | ✔ | ✔ | ✘ | ✔ | ✔ | **1/6** |
| v2 after bug fix | ✔ | ✘ | ✔ | ✔ | ✘ | ✔ | ✔ | **4/6** |
| final | ✔ | ✔ | ✔ | ✔ | ✔ | ✔ | ✔ | **6/6** |

The agent predicted the Bumbleflower miss **and its reason** in advance
rather than pre-weighting to avoid it, and declared its one post-hoc
tuning step with the pre-tuning number (4/6) left in the document.
Jace — the discriminating case — scored 0 claims in **every** version,
including the broken one.

**Chasing the greens, as required:** Teval and Bumbleflower both escaped
for one reason. The score's denominator is contaminated by *vacuous
agreement* — `color:green=ok` because green is in Teval's identity,
`CreateCreatureToken=ok` because the card does make *a* token. They are
wrong about every particular and right about every generality, and the
generalities outvote. That is a denominator defect, not a threshold
defect, and it is the same shape as the vocabulary blindness above.

### Status

**No production code changed.** Four new wrong-card candidates were
READ and oracle-verified (Kilo, Emiel the Blessed, Magus Lucea Kane,
Zopandrel) plus Caesar and Aragorn from the grep — **six new, on top of
r179's five.** Two READ and cleared as false positives (Anointed
Procession, Phylactery Lich). Precision on newly-read flags 4/6; the
unread 10/12 is circular and is not quoted.

All of it remains **filed, awaiting the owner gate.**

---

## Bucket table RECONCILED at r202 — bucket D is wrong by ~3x

The flag raised in the r200 verification section ("619 vs my 191, ~504
unexplained") is now resolved. **The agent's detector keyed on the literal
comment prefix `// Oracle text:` and the corpus overwhelmingly does not use
it.** Measured on `internal/gameengine/per_card`, non-test `.go` only,
current origin/main:

```
total non-test .go files                                  1184
files carrying an oracle block under ANY '// Oracle...:'   984
  ...using the literal '// Oracle text:'                   439
  ...using only a parenthesised or short variant           545
files with NO oracle block marker of any kind              200
```

The 545 are not undocumented. They carry full, sourced oracle blocks under
headers the detector did not match — e.g. `// Oracle text (Scryfall, verified
2026-05-17 via hexdek.dev oracle):`, `// Oracle text (Instant, {1}{R}):`,
`// Oracle:`. Spot-checked `abrade.go`, `abzan_falconer.go`,
`acererak_the_archlich.go`: all three hold complete oracle text.

**Corrections to the r200 report:**

- **Bucket D "single card, no `// Oracle text:` block" = 619 is WITHDRAWN.**
  The honest number for "no oracle block of any kind" is **200**. Bucket D
  as published counts ~420 files that are properly documented.
- **"947 attributions with no `// Oracle text:` comment at all" is
  WITHDRAWN** for the same reason. Most of those attributions do have oracle
  text; the detector could not see the header.
- The partition is still internally consistent (417 + 619 = 1036 =
  1184 − 64 − 12 − 72), which is exactly why it looked sound. **An
  arithmetically consistent partition built on a wrong predicate is still
  wrong — summing to the population checks the bookkeeping, not the test.**
- My own "191" in the r200 verification section was approximately right but
  not exact; the correct figure under the `//[[:space:]]*Oracle[^:]*:`
  regex is **200**. I had used a looser case-insensitive match.
- **Unaffected:** every card-level verdict. The eleven confirmed wrong-card
  handlers were each read and oracle-checked individually and do not depend
  on this table. The 1184 file count, the 72 multi-card files, and the
  withdrawal of r178's "426" all stand.

**The transferable lesson: a detector that keys on a comment marker is
measuring the project's comment conventions, not its code.** The r200 report
already recorded that a detector scoring the comment is not scoring the code;
this is the same error one level up — the *presence* test for the comment was
itself wrong, and it produced a number three times too large that reconciled
perfectly against the population.
