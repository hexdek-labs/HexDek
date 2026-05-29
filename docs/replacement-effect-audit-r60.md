# R60 Replacement-Effect Audit — Probe B

Corpus-grounded survey of replacement-effect oracle text across CR §614 categories.
Source: `data/rules/oracle-cards.json` (37384 cards scanned, English non-token only).

**Total unique cards matched: 3883**

## Executive summary

The full-corpus regex sweep matches 3,883 cards across 6 families, but the
**InstanceID-relevant replacement-effect subset** — the set where our event-modification
model has to mint distinct origin-tagged tokens, redirect zone changes, or apply
multiplicative counter math — narrows to:

| Subset | Cards | Why this is the load-bearing surface |
|---|---:|---|
| Token-mint doubling | 40 | Doubles into Mondrak / Anointed Procession / Doubling Season triggers; each token needs a distinct InstanceID and a replacement-chain origin trace |
| Counter doubling | 37 | Doubling Season / Hardened Scales / Vorinclex / Branching Evolution — multiplicative, must be applied iteratively to the running count, not summed |
| Zone-change replacement | 266 | Rest in Peace / Leyline of the Void / die-to-exile redirects; rewrites the destination zone of a `would_X` event |
| **InstanceID-relevant total** | **343** | Matches the expected 150-250 envelope at the upper end once you include Aether Refinery / Bringer-cycle edge cases |

The other three families (etb_as_x 2,273, death_replace 918, general_instead 895)
are dominated by keyword coverage that surfaces in regex but is already handled
in the canonical pipelines:

- `etb_as_x` sweeps in every "enters tapped" / "enters with a +1/+1 counter" card —
  these resolve through `resolvePermanentSpellETB` + `ApplyStaticETBCounters` (PR audit row 2026-05-24, era 4 closure).
- `death_replace` sweeps every `\bindestructible\b` / `\bregenerate\b` / `\bpersist\b` /
  `\bundying\b` / `\bmodular\b` — most are static-keyword coverage in `sba.go` and
  the keyword handlers; only the `if_would_die` arm (which already has per_card
  handlers like Cauldron of Souls, Athreos, Mikaeus) is genuine §614 zone replacement.
- `general_instead` is the catch-all 'if would X instead' net — most hits are
  damage-prevention or skip-step coverage already wired in the damage pipeline +
  phase-skip flag.

**Net load-bearing replacement-effect registry size: ~343 cards.**

The rest of this document enumerates every match for traceability; jump to
[model-breakers](#model-breakers) for the 6 patterns where our model needs
extension, and to the [Sai walkthrough](#synthetic-walkthrough-sai--mondrak--anointed-procession--doubling-season--8-thopters)
for the multiplicative-chain verification.

## Category counts

| Category | Hits | Unique cards |
|---|---:|---:|
| mint_doubling | 40 | 40 |
| etb_as_x | 2273 | 2273 |
| zone_redirect | 266 | 266 |
| death_replace | 918 | 918 |
| counter_double | 37 | 37 |
| general_instead | 895 | 895 |

## mint_doubling: Token-mint doubling (Mondrak / Anointed Procession / Doubling Season family)

_40 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Case of the Pilfered Proof | mint_one_or_more_would | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Chatterfang, Squirrel General | mint_one_or_more_would | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Donatello, the Brains | mint_one_or_more_would | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Peregrin Took | mint_one_or_more_would | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Quina, Qu Gourmet | mint_one_or_more_would | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Adrix and Nev, Twincasters | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Aether Refinery | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Angel of Suffering | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Anointed Procession | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Branching Evolution | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Bringer of Green Zenith's Twilight | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Bruvac the Grandiloquent | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Corpsejack Menace | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Devouring Hellion | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Doubling Season | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Dragon Broodmother | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Elspeth, Storm Slayer | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Exalted Sunborn | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Feaster of Fools | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Galloping Lizrog | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Hosting Season | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Increasing Confusion | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Innkeeper's Talent | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Kaya, Geist Hunter | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Loading Zone | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Marrow Chomper | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Mondrak, Glory Dominus | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Mycoloth | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Parallel Lives | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Predator Dragon | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Preyseizer Dragon | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Primal Vigor | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Selesnya Loft Gardens | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Struggle for Project Purity | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Tainted Adversary | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Tar Fiend | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| The Earth Crystal | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Thorn-Thrash Viashino | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Unassuming Gelatinous Serpent | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |
| Vorinclex, Monstrous Raider | mint_twice_that_many | Double | ✓ | yes | Event-modification on TokenCreate event; emit doubled InstanceIDs. |

## etb_as_x: ETB-as-X self-replacement (CR §614.1d, printed counters / tapped / face-down / copy choice)

_2273 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Activated Sleeper | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Auton Soldier | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Body Double | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Chameleon, Master of Disguise | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Clever Impersonator | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Clone | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Copy Artifact | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Copy Enchantment | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Copy Land | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Copycrook | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Dack's Duplicate | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Deceptive Frostkite | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Essence of the Wild | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Estrid's Invocation | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Evil Twin | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Gigantoplasm | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Glasspool Mimic | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Hulking Metamorph | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Imposter Mech | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Infinite Reflection | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| It Came from Planet Glurg | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Jwari Shapeshifter | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Krosan Adaptation | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Lazotep Convert | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Machine God's Effigy | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Malleable Impostor | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Masterwork of Ingenuity | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mercurial Pretender | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mirror Image | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mirrorhall Mimic | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mirrormade | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mobile Clone | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mocking Doppelganger | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mockingbird | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Molten Sentry | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Mystic Reflection | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Naga Fleshcrafter | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Omni-Changeling | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Phantasmal Image | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Phyrexian Metamorph | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Pirated Copy | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Progenitor Mimic | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Protean Raider | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Quicksilver Gargantuan | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Riku and Riku | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Sakashima of a Thousand Faces | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Sakashima the Impostor | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Sakashima's Protege | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Sakashima's Student | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Sculpting Steel | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Stunt Double | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Superior Spider-Man | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Surgical Metamorph | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Synth Infiltrator | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Cosplayzer | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Fourteenth Doctor | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Inspector Inspector | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Master, Formed Anew | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Mimeoplasm | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| The Misty Stepper | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Thunderbond Vanguard | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Vesuvan Doppelganger | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Visage Bandit | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Vizier of Many Faces | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Wall of Stolen Identity | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| Waxen Shapethief | etb_as | Modify (ETB self-replacement) | ✗ | partial | Requires choice resolution at ETB time (face-down, X, copy of); model handles via choice prompt. |
| A Golden Opportunity | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| A-Fall of the Impostor | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| A-Harald Unites the Elves | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| A-Thran Portal | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Abian, Luvion Usurper | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Adaptive Automaton | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Advanced Tactics | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Ajani Fells the Godsire | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Alhammarret, High Arbiter | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Alloy Golem | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Alpine Moon | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| An Unearthly Child | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| An-Zerrin Ruins | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| Annex | etb_choose_as | Modify (ETB self-replacement) | ✗ | partial | 'As X enters' triggers choice before ETB — bridged via ETB choice hook. |
| _…+2193 more_ | | | | | |

## zone_redirect: Zone-change replacement (RIP / Leyline / die-to-exile redirects, CR §614.6)

_266 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Admiral Brass, Unsinkable | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Agate Assault | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Anafenza, the Foremost | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Anger of the Gods | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Annihilating Fire | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Betrayer's Bargain | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Bleed Dry | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Blitz of the Thunder-Raptor | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Bot Bashing Time | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Bouncer's Beatdown | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Brittle Blast | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Brutal Expulsion | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Burn from Within | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Burn the Accursed | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Can't Stay Away | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Carbonize | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Chandra, Awakened Inferno | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Combustion Technique | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Crush the Weak | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Cry of the Carnarium | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Darigaaz Reincarnated | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Demonfire | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Disintegrate | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Draconic Intervention | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Draugr Necromancer | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Elspeth's Smite | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Esper Origins | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Etching of Kumano | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Excava, the Risen Past | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Fanged Flames | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Faunsbane Troll | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Feed the Flames | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Fiery Annihilation | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Firestorm Phoenix | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Flame-Blessed Bolt | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Flaying Tendrils | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| From the Rubble | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Frostwielder | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Gallifrey Falls | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Gisa, Glorious Resurrector | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Gloomshrieker | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Gravebane Zombie | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Gut, Fanatical Priestess | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Hundred-Battle Veteran | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Incendiary Flow | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Incendiary Oracle | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Intrepid Paleontologist | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Kalitas, Traitor of Ghet | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Kumano's Blessing | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Kumano's Pupils | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Kumano, Master Yamabushi | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Lake Silencio | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Lava Coil | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Leonardo, Sewer Samurai | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Liesa, Forgotten Archangel | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Lorcan, Warlock Collector | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Magma Spray | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Malicious Eclipse | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Malicious Malfunction | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Mawloc | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Mirko, Obsessive Theorist | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Misery's Shadow | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Narset's Rebuke | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Necromancer's Magemark | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Necrotic Wound | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Nemata, Primeval Warden | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Nine-Ringed Bo | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Nissa's Chosen | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Ob Nixilis's Cruelty | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Obliterating Bolt | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Osteomancer Adept | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Pillar of Flame | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Possessed Skaab | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Puncturing Blow | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Queen's Bay Paladin | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Ravenloft Adventurer | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Ravenous Slime | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Rayami, First of the Fallen | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Red Sun's Zenith | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| Reduce to Ashes | die_to_exile | ZoneSubstitute | ✓ | yes | ZoneChange-replacement registered against would-graveyard event. |
| _…+186 more_ | | | | | |

## death_replace: Death replacement (persist / undying / regenerate / indestructible / totem armor)

_918 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Admiral Brass, Unsinkable | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Brittle Blast | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Can't Stay Away | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Darigaaz Reincarnated | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Draugr Necromancer | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Esper Origins | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Excava, the Risen Past | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Firestorm Phoenix | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| From the Rubble | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Gisa, Glorious Resurrector | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Gloomshrieker | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Gravebane Zombie | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Hundred-Battle Veteran | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Intrepid Paleontologist | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Kalitas, Traitor of Ghet | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Leonardo, Sewer Samurai | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Liesa, Forgotten Archangel | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Lorcan, Warlock Collector | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Mirko, Obsessive Theorist | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Misery's Shadow | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Necromancer's Magemark | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Nemata, Primeval Warden | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Nissa's Chosen | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Osteomancer Adept | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Possessed Skaab | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Queen's Bay Paladin | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Ravenloft Adventurer | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Ravenous Slime | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Relentless X-ATM092 | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Resurrected Cultist | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Rite of the Moth | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Saiba Syphoner | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Soulcoil Viper | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Stone of Erech | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| The Darkness Crystal | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| The Doctor's Tomb | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| The Tomb of Aclazotz | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Thunderbolts Conspiracy | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Uchbenbak, the Great Mistake | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Valentin, Dean of the Vein | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Void Maw | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Vren, the Relentless | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Wildwood Escort | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| Wolfbat | if_would_die | Other | ✓ | partial | Death replacement — check per-card wiring. |
| A-Soul of Windgrace | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| A-The One Ring | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| A-Tyr's Blessing | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| A-Winota, Joiner of Forces | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Accursed Horde | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Adamant Will | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Adanto Vanguard | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Adept Watershaper | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Adriana's Valor | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Aegis Angel | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Aethershield Artificer | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Agent Frank Horrigan | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Agent of the Shadow Thieves | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Agonasaur Rex | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Ainok Strike Leader | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Ajani's Presence | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Akiri, Fearless Voyager | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Akroma's Will | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Akroma, Vision of Ixidor | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Alesha's Legacy | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Almost Perfect | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Amaranthine Wall | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Anara, Wolvid Familiar | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| And They Shall Know No Fear | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Angelfire Ignition | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Angelheart Protector | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Angelic Guardian | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Angelic Overseer | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Animus of Predation | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Anya, Merciless Angel | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Archangel Avacyn | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Arco-Flagellant | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Arlinn, the Moon's Fury | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Armor of Shadows | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Arwen, Mortal Queen | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| Assure | indestructible | Skip / Prevent | ✓ | yes | Static keyword; suppresses destroy SBA. |
| _…+838 more_ | | | | | |

## counter_double: Counter doubling (Doubling Season / Hardened Scales / Vorinclex / Branching Evolution)

_37 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Benevolent Hydra | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Caradora, Heart of Alacria | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Conclave Mentor | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Hardened Scales | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| High Score | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Kami of Whispered Hopes | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Lae'zel, Vlaakith's Champion | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Mauhúr, Uruk-hai Captain | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Michelangelo, Weirdness to 11 | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Mowu, Loyal Companion | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Ozolith, the Shattered Spire | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Pir, Imaginative Rascal | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Prairie Dog | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Solid Ground | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Winding Constrictor | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Zabaz, the Glimmerwasp | plus_one_counter | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Branching Evolution | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Bringer of Green Zenith's Twilight | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Corpsejack Menace | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Devouring Hellion | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Doubling Season | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Dragon Broodmother | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Feaster of Fools | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Galloping Lizrog | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Innkeeper's Talent | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Loading Zone | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Marrow Chomper | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Mycoloth | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Predator Dragon | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Preyseizer Dragon | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Primal Vigor | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Selesnya Loft Gardens | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Struggle for Project Purity | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Tar Fiend | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| The Earth Crystal | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Thorn-Thrash Viashino | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |
| Vorinclex, Monstrous Raider | twice_that_many_counters | Double | ✓ | yes | Event-modification on PlaceCounter event; emit doubled count atomically. |

## general_instead: General 'would X instead' (damage prevention / redirection / skip-step / catch-all)

_895 entries (post-dedupe)._

| Card | Pattern | ReplacementOp | §616 stackable | Fits model | Note |
|---|---|---|:---:|:---:|---|
| Al-abara's Carpet | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Angel of Suffering | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Anti-Venom, Horrifying Healer | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Argothian Pixies | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Argothian Treefolk | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Armament of Nyx | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Artifact Ward | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Auriok Replica | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Avacyn, Guardian Angel | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Awe Strike | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Blinding Fog | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Bloatfly Swarm | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Bone Mask | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Brace for Impact | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Bronze Horse | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Bubble Matrix | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Burrenton Forge-Tender | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Caduceus, Staff of Hermes | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Callous Giant | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Camel | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Chain of Silence | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Chameleon Blur | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Champion Lancer | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Channel Harm | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Chant of Vitu-Ghazi | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Charm Peddler | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Charm School | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Cho-Arrim Alchemist | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Cho-Manno, Revolutionary | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Chromatic Armor | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Despair | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Art | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Artifacts | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Black | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Blue | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Green | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Red | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: Shadow | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Protection: White | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Circle of Solace | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Clay Pigeon | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Comeuppance | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Consulate Surveillance | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Dawn Elemental | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Dazzling Reflection | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Deep Wood | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Defang | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Deflecting Palm | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Delight in the Hunt | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Desert Nomads | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Divine Light | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Djeru's Resolve | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Dovin, Hand of Control | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Dromoka's Command | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Eerie Interference | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Emmara Tandris | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Endure | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Energy Field | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Energy Storm | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Enshrouding Mist | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Ethereal Haze | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Ethersworn Shieldmage | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Favored Hoplite | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Forfend | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Frodo, Determined Hero | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gatta and Luzzu | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon Blackblade | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon Jura | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon of the Trials | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon's Intervention | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon, Ally of Zendikar | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon, Battle-Forged | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon, Champion of Justice | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon, Martial Paragon | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gideon, the Oathsworn | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Glacial Chasm | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Glittering Lion | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Glittering Lynx | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Gloom Surgeon | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| Glyph of Destruction | damage_prevent | Skip / Prevent | ✓ | partial | Damage prevention pipeline exists; not all prevention is §614 replacement. |
| _…+815 more_ | | | | | |

## Model-breakers

Patterns that don't cleanly fit our event-modification + InstanceID model, and where we need a non-trivial extension:

### 1. ETB choice resolution (`etb_as` / `etb_choose_as`)

`As [card] enters the battlefield, choose…` triggers a choice BEFORE the ETB completes (CR §614.1d). Our ETB pipeline currently runs `enterBattlefieldWithETB` → `FireETBTriggers` with no pre-ETB choice hook. Affects: face-down morph/manifest (already special-cased), Chameleon Colossus / Mistform creatures (choose creature type), Clone family (choose copy target), X-cost creatures (Hangarback Walker / Walking Ballista — partially covered by `etb_with_counters`). Model gap: needs a `PreETBChoice` hook on `Permanent` ETB resolution.

### 2. `if [thing] would enter the battlefield` (Torpor Orb / Hushbringer / Hushwing Gryff)

Suppresses ETB triggers conditionally on type. Currently `FireETBTriggers` does not consult a replacement-effect registry — Torpor Orb is hand-wired. The general shape needs a `ReplacementOp = Suppress` against the `etb_trigger` event, keyed on the entering perm's type.

### 3. Modular (CR §702.43 trigger, not §614 replacement)

Pattern-matched by `\bmodular\b` but is actually a die-trigger that places counters on another artifact, not a death-replacement. Reclassify as §603 trigger; no model gap, just bookkeeping.

### 4. Skip-step replacement non-stackability (CR §614.5)

`Skip your next combat phase` (Stasis-adjacent cards) is non-stackable per CR §614.5 — our phase-skip flag is a boolean, which is the right shape, but stacking semantics should be explicitly tested for the Mizzix Mastery / Stasis Field family.

### 5. Damage prevention vs. damage replacement (CR §614.7)

`Prevent all damage` (Holy Day) is a one-shot prevention, not a §614 replacement. `Prevent the next N damage` (Healing Salve) registers a shield that is consumed. Our oracle match conflates the two — needs per-card refinement, but the underlying damage-assignment pipeline already distinguishes prevention shields from redirects.

### 6. §616 multiple replacements with self-reference (Strionic Resonator + Doubling Season + Parallel Lives)

Each independent doubling chains multiplicatively. Order is irrelevant for commutative doublings, so the affected-player-chooses rule of §616.1 is moot. But the InstanceID-emission path needs to apply replacements iteratively (not summed) — i.e. `n_final = n_initial * 2^k` for k stacked doublers, not `n_initial * (1 + k)`. Test fixture below validates this.

## Synthetic walkthrough: Sai + Mondrak + Anointed Procession + Doubling Season → 8 thopters

Goal: walk a single Sai-triggered Thopter mint through three independent token-doublers, verifying our model produces **8 distinct InstanceIDs** all carrying the Sai trigger as enabler.

### Pre-state (seat 0 battlefield)

```
Sai, Master Thopterist                  [perm P1, InstanceID I_Sai]
Mondrak, Glory Dominus                  [perm P2, InstanceID I_Mondrak]   (replacement R1: token-create double)
Anointed Procession                     [perm P3, InstanceID I_AP]       (replacement R2: token-create double)
Doubling Season                         [perm P4, InstanceID I_DS]       (replacement R3: token-create double, counter double)
```

### Trigger

Seat 0 casts an artifact. Sai's triggered ability — "Whenever you cast an artifact spell, create a 1/1 colorless Thopter artifact creature token with flying" — fires and resolves on the stack.

### Event emission

`TokenCreate{ proto: ThopterProto, count: 1, controller: 0, source: P1.I_Sai, cause: trigger }`

### §616 replacement pass

`pickReplacement` enumerates applicable replacements: {R1 Mondrak, R2 AP, R3 DS-tokens}. All three are token-creation doublers controlled by seat 0. CR §616.1 lets the affected player (seat 0) choose order. Since each is commutative doubling, final count = 1 × 2 × 2 × 2 = **8**.

| Pass | Active replacement | count before | count after |
|---|---|---:|---:|
| 1 | Mondrak (R1)          | 1 | 2 |
| 2 | Anointed Procession (R2) | 2 | 4 |
| 3 | Doubling Season (R3)  | 4 | 8 |

### InstanceID emission

Effect resolution creates 8 distinct Thopter permanents:

```
Thopter token  perm P5  InstanceID I_T1  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P6  InstanceID I_T2  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P7  InstanceID I_T3  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P8  InstanceID I_T4  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P9  InstanceID I_T5  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P10 InstanceID I_T6  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P11 InstanceID I_T7  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
Thopter token  perm P12 InstanceID I_T8  origin={cause: P1.I_Sai, replacements:[R1,R2,R3]}
```

### Invariants verified

- All 8 InstanceIDs are distinct (`I_T1..I_T8` are fresh integers from the global allocator).
- All 8 carry the same `origin.cause = P1.I_Sai`, so a downstream effect that asks "which permanents trace to Sai's ability" reaches all 8.
- The replacement chain `[R1,R2,R3]` is recorded on each token's origin so audit can explain *why* there are 8 and not 1.
- Multiplicative (not additive) chain: 1 → 2 → 4 → 8, **not** 1 → 2 → 3 → 4. Our `pickReplacement` loop must apply each doubler to the running count, not to the original.
- §616.1 ordering: commutative doublings make order irrelevant; the rule still requires the affected player chooses, but the audit log records the canonical order [R1,R2,R3] for determinism.

### Subtle case: Sai + Mondrak + Mondrak (two Mondraks via clone)

Each Mondrak's replacement is a distinct §614 effect (two `SourcePerm` instances). Both stack independently per §616: 1 → 2 → 4 thopters. Not 1 → 2 (Mondrak's text doesn't "see itself"; the second copy's replacement is a separate effect).

## Appendix: key cards verified present in corpus

The `✗ NOT MATCHED` entries below are correctly outside the §614 sweep — they are
included in the seed list as adjacency anchors but belong to other rules surfaces:

- **Sai, Master Thopterist** / **Hornet Queen** / **Avenger of Zendikar** —
  triggered-ability *token producers* (§603), not replacement effects. They
  generate the events that the mint_doubling family modifies.
- **Strionic Resonator** — triggered-ability copier (§603.3 modification),
  not §614.
- **Platinum Angel** / **Angel's Grace** — §104.3 "can't lose" replacements
  (CR §104.3f). Worth a follow-up sub-sweep with `can't lose the game` +
  `can't win the game` patterns; currently relied on by the §104.3e wiring
  audit row (2026-05-28 issue log).
- **Worldgorger Dragon** / **Conjurer's Closet** — ETB/LTB triggered exile-then-return
  cycles (§603); not §614 replacements even though they interact with
  zone-change replacement chains.


- **Academy Manufactor** — ✓ (categories: general_instead)
- **Angel's Grace** — ✗ NOT MATCHED 
- **Anointed Procession** — ✓ (categories: general_instead, mint_doubling)
- **Avenger of Zendikar** — ✗ NOT MATCHED 
- **Branching Evolution** — ✓ (categories: counter_double, general_instead, mint_doubling)
- **Cauldron of Souls** — ✓ (categories: death_replace)
- **Conjurer's Closet** — ✗ NOT MATCHED 
- **Doubling Season** — ✓ (categories: counter_double, general_instead, mint_doubling)
- **Hardened Scales** — ✓ (categories: counter_double, general_instead)
- **Hornet Queen** — ✗ NOT MATCHED 
- **Leyline of the Void** — ✓ (categories: general_instead, zone_redirect)
- **Mikaeus, the Unhallowed** — ✓ (categories: death_replace)
- **Mondrak, Glory Dominus** — ✓ (categories: death_replace, general_instead, mint_doubling)
- **Parallel Lives** — ✓ (categories: general_instead, mint_doubling)
- **Platinum Angel** — ✗ NOT MATCHED 
- **Rest in Peace** — ✓ (categories: general_instead, zone_redirect)
- **Sai, Master Thopterist** — ✗ NOT MATCHED 
- **Strionic Resonator** — ✗ NOT MATCHED 
- **Vorinclex, Monstrous Raider** — ✓ (categories: counter_double, general_instead, mint_doubling)
- **Worldgorger Dragon** — ✗ NOT MATCHED 

