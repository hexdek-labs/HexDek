# Engine Confidence Matrix

**Purpose:** Transparent map of what the engine handles confidently, what's uncertain, and where 7174n1c should focus adversarial testing. Every gap is a target for the Judge CLI.

**Legend:**
- **SURGICAL** (95%+) — tested via nightmare list + fuzzer, confident under adversarial scrutiny
- **SOLID** (80-95%) — tested but edge cases may exist in complex board states
- **UNCERTAIN** (50-80%) — basic logic works but interactions with other systems untested
- **STUB** (< 50%) — minimal implementation, likely breaks on real-world use
- **MISSING** — not implemented

---

## Layer System (§613)

| Subsystem | Confidence | Known gaps for 7174n1c to probe |
|---|---|---|
| Layer 1: Copy effects | **SOLID** | Clone + Humility interaction tested. Cytoshape EOT revert tested. UNTESTED: copy of a copy (Clone copying Clone copying something). |
| Layer 2: Control change | **UNCERTAIN** | Perplexing Chimera stub exists. UNTESTED: Gilded Drake exchange, Bribery steal, Treachery control + LTB bounce. Multiple simultaneous control changes on same permanent. |
| Layer 3: Text change | **STUB** | No text-changing cards implemented (Magical Hack, Trait Doctoring). Not in any portfolio deck. |
| Layer 4: Type change | **SURGICAL** | Blood Moon, Mycosynth Lattice, Lignify all tested with interactions. |
| Layer 5: Color change | **SURGICAL** | Painter's Servant color-wash tested with dependency. |
| Layer 6: Ability add/remove | **SURGICAL** | Humility strips all abilities correctly. Lignify strips + Humility stacking tested. |
| Layer 7a: CDA | **SOLID** | Tarmogoyf-style CDAs work. UNTESTED: CDA in non-battlefield zones (Transguild Courier in graveyard). |
| Layer 7b: P/T set | **SURGICAL** | Lignify 0/4, Humility 1/1, correct sublayer ordering. |
| Layer 7c: P/T modify | **SURGICAL** | +1/+1 counters, buffs, prowess, anthems. |
| Layer 7d: P/T switch | **STUB** | About Face, Inside Out — not implemented. Rarely relevant. |

## Replacement Effects (§614)

| Subsystem | Confidence | Known gaps |
|---|---|---|
| "Instead" effects | **SURGICAL** | Rest in Peace, Leyline of the Void, Anafenza all route correctly. |
| Self-replacement | **SOLID** | UNTESTED: multiple self-replacements on same event. |
| ETB replacements (§614.6) | **SOLID** | UNTESTED: ETB replacement + copy interaction (Clone enters as copy with ETB replaced). |
| Draw replacements (§614.7) | **SOLID** | Chains of Mephistopheles handler exists. UNTESTED: Chains + Notion Thief + Alms Collector triple-replacement stack. |
| §616.1 ordering | **SURGICAL** | Hardened Scales + Doubling Season both orderings tested. APNAP tiebreak correct. |

## Combat (§506-§511)

| Subsystem | Confidence | Known gaps |
|---|---|---|
| Declare attackers | **SOLID** | UNTESTED: forced-attack effects (goad, Bident of Thassa). |
| Declare blockers | **SOLID** | Menace enforcement works. UNTESTED: banding (Soraya deck), "can't be blocked except by two or more" variants. |
| First strike / double strike | **SURGICAL** | Two-step damage tested, deathtouch per-step tested, triggers fire per-step. |
| Trample | **SURGICAL** | Excess damage over blockers, trample + deathtouch (1 to each blocker, rest to player). |
| Combat priority windows | **SURGICAL** | Priority at all 6 sub-step boundaries. |
| "Enters tapped and attacking" | **UNCERTAIN** | Token generation mid-combat creates attackers. UNTESTED: Najeela, Adeline, Hero of Bladehold — do they trigger "whenever attacks"? They shouldn't (CR §508.3a). |
| Additional combats | **SOLID** | Aggravated Assault + Sword of Feast and Famine infinite tested. UNTESTED: 3+ combat phases in one turn with tapped-creature tracking. |

## Stack & Casting (§601-§608)

| Subsystem | Confidence | Known gaps |
|---|---|---|
| Basic casting | **SURGICAL** | CMC, colored cost, X cost, Hat.ChooseX all work. |
| Alternative costs | **SURGICAL** | FoW, FoN, Fierce Guardianship, Deadly Rollick, evoke, flashback, escape. |
| Additional costs | **SOLID** | Sacrifice-as-cost (Culling the Weak, Natural Order). UNTESTED: discard-as-cost (Jump-Start), exile-from-hand-as-cost beyond pitch counters. |
| Cost modification | **SOLID** | Thalia, Trinisphere, Medallions. UNTESTED: stacked cost modifiers (Thalia + Trinisphere + Medallion simultaneously — order matters). |
| Mode choice | **UNCERTAIN** | Hat picks first mode or all modes. UNTESTED: "choose two" (Austere Command), "choose one that hasn't been chosen" (Charm cycle). |
| Target legality at resolution | **SURGICAL** | Fizzle when all targets illegal, partial resolution with remaining. |
| Split cards / MDFC | **UNCERTAIN** | MDFC lands work (play either side). UNTESTED: split cards (Fire // Ice, wear // tear) with fuse. |

## Triggered Abilities (§603)

| Subsystem | Confidence | Known gaps |
|---|---|---|
| ETB triggers | **SURGICAL** | Yarok doubling, face-down ETB, DFC ETB. |
| Dies triggers | **SURGICAL** | Blood Artist observer, Kokusho self-trigger, both tested. |
| LTB triggers | **SOLID** | Bounce fires LTB. UNTESTED: Oblivion Ring "when leaves" return-from-exile delayed. |
| Cast triggers | **SURGICAL** | Storm, Rhystic, Remora, Sentinel, Prowess, Young Pyromancer. |
| Draw triggers | **SOLID** | Smothering Tithe, Orcish Bowmasters. UNTESTED: Consecrated Sphinx (opponent draws → you draw two → if they have Sphinx too, infinite loop). |
| Upkeep triggers | **SOLID** | Cumulative upkeep (Remora), Pact cycle. UNTESTED: multiple upkeep triggers ordering (Braid of Fire + Mana Vault + Pact simultaneously). |
| Delayed triggers | **SOLID** | One-shot + condition-based. UNTESTED: delayed trigger + Sundial (does it fire NEXT turn? Yes per CR). |
| Intervening "if" | **UNCERTAIN** | Not explicitly checked. §603.4 says condition checked both at trigger AND resolution. UNTESTED: Felidar Sovereign "if you have 40+ life" — what if life drops between trigger and resolution? |
| Reflexive triggers | **STUB** | "When you do" pattern — not implemented. |

## State-Based Actions (§704)

| Subsystem | Confidence | Known gaps |
|---|---|---|
| Life ≤ 0 | **SURGICAL** | Tested extensively. |
| Library empty draw | **SURGICAL** | Tested. |
| Creature lethal damage | **SURGICAL** | Including deathtouch. |
| Creature 0 toughness | **SURGICAL** | Including Humility interaction. |
| Planeswalker 0 loyalty | **SOLID** | UNTESTED with actual planeswalker cards (most portfolio decks don't run PWs). |
| Legend rule | **SOLID** | UNTESTED: legend rule with mutate (does the mutate stack count as a new legend?). |
| +1/+1 vs -1/-1 annihilation | **SOLID** | UNTESTED: Hardened Scales + annihilation interaction timing. |
| Aura fall-off | **SOLID** | Orphan detection works. UNTESTED: aura on a creature that phases out (aura phases with it per CR 702.26d). |
| Commander damage 21+ | **SURGICAL** | Per-dealer tracking, partner separate, simultaneous with life-loss. |

## Keywords (§702)

| Confidence | Keywords |
|---|---|
| **SURGICAL** | Deathtouch, defender, double strike, first strike, flying, reach, haste, hexproof, indestructible, lifelink, menace, trample, vigilance, flash, storm, cascade, phasing, daybound/nightbound, morph/manifest/disguise/cloak, partner, companion |
| **SOLID** | Protection (color), prowess, ward, ninjutsu, evoke, flashback, escape, split second |
| **UNCERTAIN** | Equip (generic cost path), bestow, reconfigure, living weapon |
| **STUB** | Annihilator, afflict, bushido, absorb, affinity, convoke, delve, dredge, emerge, entwine, improvise, kicker/multikicker, madness, miracle, overload, spectacle, surge, undying, persist |
| **MISSING** | Banding, flanking, horsemanship, rampage, shadow, skulk, soulshift, suspend (partial), wither/infect, myriad, battle cry, exalted, modular, graft, transmute, replicate, forecast, hideaway, champion, changeling |

## Mana System

| Subsystem | Confidence | Known gaps |
|---|---|---|
| Typed colored pools | **SURGICAL** | W/U/B/R/G/C/any buckets. |
| Phase-drain (§106.4) | **SURGICAL** | Drains at every step boundary. |
| Artifact mana rocks | **SURGICAL** | Sol Ring through LED, all 50+ rocks. |
| Restriction-tagged mana | **SOLID** | Food Chain creature-only. UNTESTED: Powerstone noncreature-only + creature spell simultaneously on stack. |
| Omnath/Upwelling exemption | **SOLID** | Green retention tested. UNTESTED: Omnath + Blood Moon (does green mana survive if Omnath's on a Mountain?). |
| Snow/Phyrexian/Hybrid mana | **STUB** | Approximated as "any" or color-side. |

---

## How 7174n1c Should Use This

1. **Start with UNCERTAIN items.** These are the highest-probability dingleberry locations. Use the Judge CLI to construct the exact scenario described in the "Known gaps" column.

2. **Probe SOLID items at the interaction boundaries.** "Thalia + Trinisphere + Medallion simultaneously" — does the engine apply them in the right order? Construct it in the Judge CLI and verify.

3. **Skip SURGICAL items** unless you have a specific scenario you think breaks them. These have been nightmare-tested + fuzz-tested.

4. **Skip MISSING/STUB items** — they're known gaps, not bugs. They're on the roadmap but not in scope for the current presentation.

5. **If you find a bug:** note the Judge CLI commands that reproduce it. I'll add it to the nightmare list as a regression test + fix.

---

*Generated 2026-04-17. Updates as fixes land.*
