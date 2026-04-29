# §706 Copy Rules Audit Report

**Date:** 2026-04-25
**Scryfall cards analyzed:** 706
**AST corpus size:** 31965

## Summary

| Status | Count |
|--------|-------|
| PASS | 278 |
| FAIL | 0 |
| STUB | 0 |
| NO_HANDLER | 428 |
| **Total** | **706** |

## Category Breakdown

| Category | Count | Description |
|----------|-------|-------------|
| token_copy | 235 | create a token that's a copy (§706.10a) |
| becomes_copy | 131 | becomes a copy — layer 1 overwrite (§706.2) |
| spell_copy | 172 | copy target spell — stack copies (§707) |
| ability_copy | 18 | copy activated/triggered ability |
| exile_copy | 8 | exile-zone copy / cast a copy |
| other_copy | 82 | miscellaneous copy effect |
| incidental | 60 | copy in text but no copy mechanic |

## CR Compliance Check

| Rule | Check | Status |
|------|-------|--------|
| §706.2 | Token copies inherit MV from source | 113 PASS / 0 FAIL |
| §706.2 | Copy inherits color/types | 113 PASS / 0 FAIL |
| §603.6a | ETB triggers fire on token copies | 113 PASS / 0 FAIL |
| §614.1c | Replacement effects apply on entry | 113 PASS / 0 FAIL |
| §706.2+§202.3b | Token MV != 0 when copying | 113 PASS / 0 FAIL |
| §707 | Spell copies on stack | 0 PASS / 0 STUB (log-only) |
| §706.2/§613.1a | becomes-a-copy layer 1 | 2 correct |

## token_copy (235 cards)

### PASS (113)

| Card | AST Kind | Per-Card Handler |
|------|----------|------------------|
| A-Ocelot Pride | copy_permanent, as_token=true | - |
| Abyssal Harvester | copy_permanent, as_token=true | - |
| Adagia, Windswept Bastion | create_token(is_copy_of) | - |
| Anikthea, Hand of Erebos | copy_permanent, as_token=true | - |
| Applied Geometry | create_token(is_copy_of) | - |
| Arna Kennerüd, Skycaptain | copy_permanent, as_token=true | - |
| Artificer Class | create_token(is_copy_of) | - |
| Back from the Brink | copy_permanent, as_token=true | - |
| Benthic Anomaly | copy_permanent, as_token=true | - |
| Bloodforged Battle-Axe | copy_permanent, as_token=true | - |
| Cabaretti Confluence | create_token(is_copy_of) | - |
| Cackling Counterpart | create_token(is_copy_of) | - |
| Caller of the Untamed | copy_permanent, as_token=true | - |
| Caretaker's Talent | create_token(is_copy_of) | - |
| Chandra, Flameshaper | create_token(is_copy_of) | - |
| Coiling Rebirth | copy_permanent, as_token=true | - |
| Conclave Evangelist | copy_permanent, as_token=true | - |
| Croaking Counterpart | create_token(is_copy_of) | - |
| Dance of Many | create_token(is_copy_of) | - |
| Dino DNA | create_token(is_copy_of) | - |
| Dollhouse of Horrors | copy_permanent, as_token=true | - |
| Donatello, Gadget Master | create_token(is_copy_of) | - |
| Dutiful Replicator | create_token(is_copy_of) | - |
| Echo Chamber | copy_permanent, as_token=true | - |
| Echo Storm | create_token(is_copy_of) | - |
| Electroduplicate | create_token(is_copy_of) | - |
| Endless Evil | copy_permanent, as_token=true | - |
| Esika's Chariot | create_token(is_copy_of) | - |
| Espers to Magicite | copy_permanent, as_token=true | - |
| Extravagant Replication | copy_permanent, as_token=true | - |
| Faerie Artisans | create_token(is_copy_of) | - |
| Fated Infatuation | create_token(is_copy_of) | - |
| Fear of Ridicule | copy_permanent, as_token=true | - |
| Flamerush Rider | copy_permanent, as_token=true | - |
| Followed Footsteps | copy_permanent, as_token=true | - |
| Ghired, Mirror of the Wilds | create_token(is_copy_of) | - |
| Giant Adephage | copy_permanent, as_token=true | - |
| God-Pharaoh's Gift | copy_permanent, as_token=true | - |
| Gyrus, Waker of Corpses | copy_permanent, as_token=true | - |
| Hazel of the Rootbloom | create_token(is_copy_of) | - |
| Heat Shimmer | create_token(is_copy_of) | - |
| Helm of the Host | copy_permanent, as_token=true | - |
| Here Comes a New Hero! | copy_permanent, as_token=true | - |
| Impostor Syndrome | copy_permanent, as_token=true | - |
| Improvised Arsenal | copy_permanent, as_token=true | - |
| Inchblade Companion | copy_permanent, as_token=true | - |
| Irenicus's Vile Duplication | create_token(is_copy_of) | - |
| Joo Dee, One of Many | copy_permanent, as_token=true | - |
| Kindle the Inner Flame | create_token(is_copy_of) | - |
| Kinzu of the Bleak Coven | copy_permanent, as_token=true | - |
| Lazotep Quarry | copy_permanent, as_token=true | - |
| Littjara Mirrorlake | create_token(is_copy_of) | - |
| Mechanized Production | copy_permanent, as_token=true | - |
| Miirym, Sentinel Wyrm | copy_permanent, as_token=true | - |
| Mirrorpool | copy_spell, create_token(is_copy_of) | - |
| Mirrorworks | copy_permanent, as_token=true | - |
| Mishra's Self-Replicator | copy_permanent, as_token=true | - |
| Mist-Syndicate Naga | copy_permanent, as_token=true | - |
| Mordor on the March | copy_permanent, as_token=true | - |
| Myr Propagator | copy_permanent, as_token=true | - |
| Myrkul, Lord of Bones | copy_permanent, as_token=true | - |
| Mythos of Illuna | create_token(is_copy_of) | - |
| Nahiri, the Unforgiving | copy_permanent, as_token=true | - |
| Necroduality | copy_permanent, as_token=true | - |
| Nemesis Trap | copy_permanent, as_token=true | - |
| Nightmare Shepherd | copy_permanent, as_token=true | - |
| Ocelot Pride | copy_permanent, as_token=true | - |
| Octomancer | create_token(is_copy_of) | - |
| Offspring's Revenge | copy_permanent, as_token=true | - |
| Oko, the Ringleader | copy_permanent, as_token=true | - |
| Oltec Matterweaver | create_token(is_copy_of) | - |
| Orvar, the All-Form | create_token(is_copy_of) | - |
| Osgood, Operation Double | copy_permanent, as_token=true | - |
| Pack Rat | copy_permanent, as_token=true | - |
| Pool of Vigorous Growth | copy_permanent, as_token=true | - |
| Prototype Portal | copy_permanent, as_token=true | - |
| Quantum Misalignment | create_token(is_copy_of) | - |
| Quasiduplicate | create_token(is_copy_of) | - |
| Rally the Galadhrim | create_token(is_copy_of) | - |
| Ratadrabik of Urborg | copy_permanent, as_token=true | - |
| Relm's Sketching | create_token(is_copy_of) | - |
| Renewed Solidarity | copy_permanent, as_token=true | - |
| Replication Specialist | copy_permanent, as_token=true | - |
| Replication Technique | create_token(is_copy_of) | - |
| Rhys the Redeemed | copy_permanent, as_token=true | - |
| Riku of Two Reflections | copy_permanent, as_token=true | - |
| Rite of Replication | create_token(is_copy_of) | - |
| Rootcast Apprenticeship | create_token(is_copy_of) | - |
| Saheeli Rai | create_token(is_copy_of) | - |
| Saheeli, the Gifted | copy_permanent, as_token=true | - |
| Schema Thief | create_token(is_copy_of) | - |
| See Double | create_token(is_copy_of) | - |
| Self-Reflection | create_token(is_copy_of) | - |
| Shelob, Child of Ungoliant | copy_permanent, as_token=true | - |
| Shredder, Shadow Master | copy_permanent, as_token=true | - |
| Sorcerer's Broom | copy_permanent, as_token=true | - |
| Soul Foundry | copy_permanent, as_token=true | - |
| Soul Separator | copy_permanent, as_token=true | - |
| Spawnwrithe | copy_permanent, as_token=true | - |
| Specimen Collector | create_token(is_copy_of) | - |
| Spitting Image | create_token(is_copy_of) | - |
| Splitting Slime | copy_permanent, as_token=true | - |
| Springheart Nantuko | copy_permanent, as_token=true | - |
| Stolen Identity | create_token(is_copy_of) | - |
| Séance | copy_permanent, as_token=true | - |
| Tamiyo, Compleated Sage | copy_permanent, as_token=true | - |
| Tawnos, Solemn Survivor | copy_permanent, as_token=true | - |
| Tempestra, Dame of Games | copy_permanent, as_token=true | - |
| Urza, Prince of Kroog | create_token(is_copy_of) | - |
| Watchful Radstag | copy_permanent, as_token=true | - |
| Will of the Temur | create_token(is_copy_of) | - |
| Worldwalker Helm | create_token(is_copy_of) | - |
| Yenna, Redtooth Regent | copy_permanent, as_token=true | - |

### NO_HANDLER (122)

| Card | AST | Issues |
|------|-----|--------|
| A-Ochre Jelly | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| A-Thousand-Faced Shadow | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Arcane Artisan | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ardyn, the Usurper | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ashling's Command | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ashling, the Limitless | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Assemble from Parts | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Auton Soldier | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Baldur's Gate Wilderness | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler |
| Battle for Bretagard | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Biowaste Blob | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Blue Sun's Twilight | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Bramble Sovereign | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Brenard, Ginger Sculptor | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Brigid's Command | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Cadric, Soul Kindler | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Calix, Guided by Fate | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Chrome Dome | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| City of Death | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Clone Legion | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Cogwork Assembler | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Colorstorm Stallion | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Copy Catchers | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Court of Vantress | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Dedicated Dollmaker | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Dreadfeast Demon | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Dual Nature | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Echocasting Symposium | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Echoing Assault | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Elminster's Simulacrum | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Elvish Hydromancer | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ember Island Production | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Esoteric Duplicator | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Feldon of the Third Path | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Felhide Spiritbinder | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Firion, Wild Rose Warrior | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Flameshadow Conjuring | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Flash Photography | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Foggy Swamp Visions | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Fractured Identity | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Gallery of Legends | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler |
| Garth One-Eye | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Genestealer Patriarch | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Grub's Command | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Hate Mirage | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Haunting Imitation | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Hofri Ghostforge | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Homunculus Horde | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Hour of Eternity | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Hunted by The Family | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Imperial Mask | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Inalla, Archmage Ritualist | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Jace, Mirror Mage | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Jaxis, the Troublemaker | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Kaya, Intangible Slayer | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Kiki-Jiki, Mirror Breaker | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Kindred Charge | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Leitmotif Composer | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Leonardo da Vinci | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Life of the Party | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Lucy MacLean, Positively Armed | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Magar of the Magic Strings | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler |
| March of Progress | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mimic Vat | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Minion Reflector | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirage Mockery | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirage Phalanx | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirror March | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirror Match | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirror Mockery | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mirror-Sigil Sergeant | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Mishra, Eminent One | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Molten Duplication | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Molten Echoes | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Muddle, the Ever-Changing | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Nexus of Becoming | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ochre Jelly | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ondu Spiritdancer | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Orthion, Hero of Lavabrink | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Parallel Evolution | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Phoenix Fleet Airship | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Polyraptor | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Preston, the Vanisher | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Progenitor Mimic | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Ran and Shaw | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Red Sun's Twilight | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Saheeli's Artistry | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Saheeli, Radiant Creator | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Saheeli, the Sun's Brilliance | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Scute Swarm | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Season of Weaving | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Second Harvest | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Splinter Twin | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Sprouting Phytohydra | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Stangg, Echo Warrior | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Stormsplitter | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Sublime Epiphany | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Summon: Good King Mog XII | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Supplant Form | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Sygg's Command | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Tempt with Reflections | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Apprentice's Folly | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Cloning of Shredder | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Eleventh Hour | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Fire Crystal | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Jolly Balloon Man | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Master, Gallifrey's End | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| The Scarab God | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Theoretical Duplication | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Thousand-Faced Shadow | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Three Blind Mice | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Three Dog, Galaxy News DJ | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Three Steps Ahead | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Trickster's Talisman | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Trystan's Command | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Twilight Diviner | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Twinflame | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Uchuulon | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Vaultborn Tyrant | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Vesuvan Duplimancy | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Wedding Ring | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |
| Zndrsplt's Judgment | none | AST_MISSING_COPY: token copy card has no copy_permanent or is_copy_of in AST; NO_HANDLER: no AST copy node and no per-card handler |

## becomes_copy (131 cards)

### PASS (2)

| Card | AST Kind | Per-Card Handler |
|------|----------|------------------|
| Mirage Mirror | - | Yes |
| Phantasmal Image | - | Yes |

### NO_HANDLER (129)

| Card | AST | Issues |
|------|-----|--------|
| Activated Sleeper | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Altered Ego | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Artisan of Forms | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Assimilation Aegis | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Aurora Shifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Blade of Shared Souls | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Body Double | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Brudiclad, Telchor Engineer | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Cemetery Puca | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Cephalid Facetaker | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Chameleon, Master of Disguise | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Clever Impersonator | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Clone | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Copy Artifact | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Copy Enchantment | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Copy Land | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Copycrook | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Cryptoplasm | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Crystalline Resonance | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Curie, Emergent Intelligence | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Cursed Mirror | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Cytoshape | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Dack's Duplicate | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Deceptive Frostkite | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Deepfathom Echo | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Dermotaxi | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Dimir Doppelganger | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Duplication Device | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Essence of the Wild | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Estrid's Invocation | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Evil Twin | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Fleeting Reflection | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Flesh Duplicate | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Gigantoplasm | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Gogo, Mysterious Mime | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Hall of Mirrors | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Hulking Metamorph | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Identity Thief | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Imposter Mech | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Infinite Reflection | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Irma, Part-Time Mutant | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| It Came from Planet Glurg | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Jwari Shapeshifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Kaya, Spirits' Justice | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Killer Cosplay | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Kimahri, Valiant Guardian | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Lazav, Dimir Mastermind | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Lazav, Familiar Stranger | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Lazav, Wearer of Faces | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Lazav, the Multifarious | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Likeness Looter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Loose in the Park | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Machine God's Effigy | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Malleable Impostor | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| March from Velis Vel | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Masterful Replication | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Masterwork of Ingenuity | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mercurial Pretender | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mimeoplasm, Revered One | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mindlink Mech | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mirror Image | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mirror of the Forebears | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mirrorform | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mirrormade | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mirrorweave | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mizzium Transreliquat | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mocking Doppelganger | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Mockingbird | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Moritte of the Frost | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Naga Fleshcrafter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Nanogene Conversion | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Nascent Metamorph | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Oko, the Trickster | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Omni-Changeling | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Permeating Mass | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Phyrexian Metamorph | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Pirated Copy | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Polymorphous Rush | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Protean Raider | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Protean Thaumaturge | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Protean War Engine | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Quicksilver Gargantuan | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Reflection Net | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Renegade Doppelganger | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Saheeli, Sublime Artificer | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sakashima of a Thousand Faces | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sakashima the Impostor | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sakashima's Protege | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sakashima's Student | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sakashima's Will | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sarkhan, Soul Aflame | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Scion of the Ur-Dragon | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sculpting Steel | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Shadow Kin | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Shameless Charlatan | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Shapesharer | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Shapeshifter's Marrow | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Shifting Woodland | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Silent Hallcreeper | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Spark Double | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Spirit of Resilience | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Stunt Double | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Sunfrill Imitator | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Superior Spider-Man | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Surgical Metamorph | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Synth Infiltrator | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Animus | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Ever-Changing 'Dane | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Flood of Mars | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Fourteenth Doctor | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Master, Formed Anew | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Mimeoplasm | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| The Mycosynth Gardens | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Thespian's Stage | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Thunderbond Vanguard | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Tilonalli's Skinshifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| True Polymorph | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Undercover Operative | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Unstable Shapeshifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Vesuvan Doppelganger | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Vesuvan Drifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Vesuvan Shapeshifter | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Visage Bandit | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Vizier of Many Faces | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Volatile Chimera | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Volrath, the Shapestealer | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Wall of Stolen Identity | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Waxen Shapethief | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |
| Zygon Infiltrator | none | AST_MISSING_COPY: becomes-a-copy card has no copy_permanent in AST; NO_HANDLER: no AST copy node and no per-card handler for becomes-a-copy |

## spell_copy (172 cards)

### PASS (23)

| Card | AST Kind | Per-Card Handler |
|------|----------|------------------|
| Chancellor of Tales | copy_spell | - |
| Dualcaster Mage | copy_spell | - |
| Echo Mage | copy_spell | - |
| Flare of Duplication | copy_spell | - |
| Fury Storm | copy_spell | - |
| Geistblast | copy_spell | - |
| Gorion, Wise Mentor | copy_spell | - |
| Increasing Vengeance | copy_spell | - |
| Kitsa, Otterball Elite | copy_spell | - |
| Lithoform Engine | copy_spell | - |
| Mischievous Quanar | copy_spell | - |
| Mizzix, Replica Rider | copy_spell | - |
| Naru Meha, Master Wizard | copy_spell | - |
| Nivix Guildmage | copy_spell | - |
| Reiterate | copy_spell | - |
| Reverberate | copy_spell | - |
| Rootha, Mercurial Artist | copy_spell | - |
| Sigil Tracer | copy_spell | - |
| Stella Lee, Wild Card | copy_spell | - |
| Thousand-Year Storm | - | Yes |
| Twincast | copy_spell | - |
| Twinning Staff | copy_spell | - |
| Uyo, Silent Prophet | copy_spell | - |

### NO_HANDLER (149)

| Card | AST | Issues |
|------|-----|--------|
| A-Demilich | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| A-Leyline of Resonance | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| A-Mentor's Guidance | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Adaptive Training Post | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Alania, Divergent Storm | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Archmage of Echoes | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Aziza, Mage Tower Captain | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Banish into Fable | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Beamsplitter Mage | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Bill Potts | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Blue Mage's Cane | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Brass Knuckles | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Breeches, the Blastmaker | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Capricious Hellraiser | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Case of the Shifting Visage | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Chandra, Hope's Beacon | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Chandra, Pyromaster | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Chandra, the Firebrand | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Chef's Kiss | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Choreographed Sparks | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Cloven Casting | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Complete the Circuit | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Curse of Echoes | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Cursed Recording | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Demilich | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Donal, Herald of Wings | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Double Down | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Double Stroke | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: spell copy card has no implementation |
| Double Vision | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Doublecast | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Dual Casting | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Dual Strike | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Echoes of Eternity | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Echoing Boon | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: spell copy card has no implementation |
| Empyrial Storm | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Errant, Street Artist | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Ether | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Exterminator Magmarch | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Feather, Radiant Arbiter | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Fire Lord Azula | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Fork | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Founding the Third Path | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Frontline Heroism | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Gadwick's First Duel | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Galvanic Iteration | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Gandalf the Grey | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Gandalf, Westward Voyager | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Geist of Regret | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Genesis Storm | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Goblin Blastronauts | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: spell copy card has no implementation |
| Howl of the Horde | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Ink-Treader Nephilim | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Insidious Will | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Ivy, Gleeful Spellthief | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Izzet Guildmage | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jace Reawakened | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jackal, Genius Geneticist | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jacob Frye | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jaya, Fiery Negotiator | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jeong Jeong, the Deserter | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Jin-Gitaxias, Progress Tyrant | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Kaervek, the Punisher | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Kalamax, the Stormsire | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Krark, the Thumbless | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| League Guildmage | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Leyline of Resonance | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Lucky Clover | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Lumaret's Favor | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Lutri, the Spellchaser | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Magus Lucea Kane | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mathise, Surge Channeler | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Melek, Izzet Paragon | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mendicant Core, Guidelight | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mentor's Guidance | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mercurial Spelldancer | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mica, Reader of Ruins | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mirari | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mirror Sheen | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mizzix's Mastery | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Mysterious Stranger | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Najal, the Storm Runner | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Nalfeshnee | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Narset's Reversal | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Narset, Enlightened Exile | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Nashi, Moon's Legacy | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Ominous Lockbox | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Owlin Spiralmancer | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Parnesse, the Subtle Brush | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Pit Automaton | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Plate Spinning | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: spell copy card has no implementation |
| Providence of Night | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Psionic Ritual | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Psychic Rebuttal | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Pyromancer Ascension | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Pyromancer's Goggles | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Radiant Performer | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Radiate | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Ral, Storm Conduit | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Reenact the Crime | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Reflections of Littjara | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Reflective Golem | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Reflective Rimekin | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Renegade Bull | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Repeated Reverberation | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Resonance Technician | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Return the Favor | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Rimefire Torque | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Roving Actuator | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Rowan Kenrith | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Sala, Deck Boss | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Sea Gate Stormcaller | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Sevinne, the Chronoclasm | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Shiko and Narset, Unified | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Shiko, Paragon of the Way | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Show of Confidence | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Skull Storm | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Slick Imitator | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Spider-Verse | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Spinerock Tyrant | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Split Decision | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Storm King's Thunder | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Storm of Forms | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Storm of Saruman | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Summon: G.F. Cerberus | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Sunken Palace | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Swarm Intelligence | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Sword of Wealth and Power | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Taigam, Master Opportunist | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Tawnos, the Toymaker | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Teach by Example | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Tempt with Mayhem | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| The Clone Saga | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| The Mirari Conjecture | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| The Sixth Doctor | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| The Twelfth Doctor | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Thunderclap Drake | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Thurid, Mare of Destiny | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Tomb of Horrors Adventurer | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Twinferno | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Unbound Flourishing | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Verazol, the Split Current | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Very Cryptic Command | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: spell copy card has no implementation |
| Volo, Guide to Monsters | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Wild Ricochet | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Will Kenrith | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Wondrous Crucible | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Wyll's Reversal | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Zada, Hedron Grinder | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |
| Zevlor, Elturel Exile | none | AST_MISSING_COPY: spell copy card has no copy_spell in AST; NO_HANDLER: spell copy card has no implementation |

## ability_copy (18 cards)

### PASS (1)

| Card | AST Kind | Per-Card Handler |
|------|----------|------------------|
| Strionic Resonator | - | Yes |

### NO_HANDLER (17)

| Card | AST | Issues |
|------|-----|--------|
| Aboleth Spawn | none | NO_HANDLER: ability copy not supported in engine |
| Agrus Kos, Eternal Soldier | none | NO_HANDLER: ability copy not supported in engine |
| Ashnod the Uncaring | none | NO_HANDLER: ability copy not supported in engine |
| Battlemage's Bracers | none | NO_HANDLER: ability copy not supported in engine |
| Chandra's Regulator | none | NO_HANDLER: ability copy not supported in engine |
| Dynaheir, Invoker Adept | none | NO_HANDLER: ability copy not supported in engine |
| Ertha Jo, Frontier Mentor | none | NO_HANDLER: ability copy not supported in engine |
| Firebender Ascension | none | NO_HANDLER: ability copy not supported in engine |
| Illusionist's Bracers | none | NO_HANDLER: ability copy not supported in engine |
| Kirol, Attentive First-Year | none | NO_HANDLER: ability copy not supported in engine |
| Kurkesh, Onakke Ancient | none | NO_HANDLER: ability copy not supported in engine |
| Leori, Sparktouched Hunter | none | NO_HANDLER: ability copy not supported in engine |
| Mirror-Shield Hoplite | none | NO_HANDLER: ability copy not supported in engine |
| Mister Fantastic | none | NO_HANDLER: ability copy not supported in engine |
| Rings of Brighthearth | none | NO_HANDLER: ability copy not supported in engine |
| Rowan's Talent | none | NO_HANDLER: ability copy not supported in engine |
| Verrak, Warped Sengir | none | NO_HANDLER: ability copy not supported in engine |

## exile_copy (8 cards)

### NO_HANDLER (8)

| Card | AST | Issues |
|------|-----|--------|
| Altaïr Ibn-La'Ahad | none | NO_HANDLER: exile-zone copy has no implementation |
| Arcane Savant | none | NO_HANDLER: exile-zone copy has no implementation |
| Ertai's Meddling | none | NO_HANDLER: exile-zone copy has no implementation |
| Exit Through the Grift Shop | none | NO_AST: card not in ast_dataset.jsonl; NO_HANDLER: exile-zone copy has no implementation |
| Panoptic Mirror | none | NO_HANDLER: exile-zone copy has no implementation |
| Sauron, the Necromancer | none | NO_HANDLER: exile-zone copy has no implementation |
| Signature Spells | none | NO_HANDLER: exile-zone copy has no implementation |
| Sin, Spira's Punishment | none | NO_HANDLER: exile-zone copy has no implementation |

## other_copy (82 cards)

### PASS (79)

| Card | AST Kind | Per-Card Handler |
|------|----------|------------------|
| A-Spell Satchel | - | - |
| A-Symmetry Sage | - | - |
| Abstruse Archaic | - | - |
| Adric, Mathematical Genius | - | - |
| Arcane Bombardment | - | - |
| Arcane Proxy | - | - |
| Archmage Emeritus | - | - |
| Ashling, Flame Dancer | - | - |
| Bloodthirsty Adversary | - | - |
| Calamity, Galloping Inferno | - | - |
| Callidus Assassin | - | - |
| Clever Lumimancer | - | - |
| Compy Swarm | - | - |
| Deekah, Fractal Theorist | - | - |
| Delina, Wild Mage | - | - |
| Double Major | - | - |
| Drafna, Founder of Lat-Nam | - | - |
| Dragonsguard Elite | - | - |
| Eager First-Year | - | - |
| Echoing Deeps | - | - |
| Elemental Expressionist | - | - |
| Elite Arcanist | - | - |
| Finale of Promise | - | - |
| Flamehold Grappler | - | - |
| Flawless Forgery | - | - |
| God-Eternal Kefnet | - | - |
| Gogo, Master of Mimicry | - | - |
| Hashaton, Scarab's Fist | - | - |
| Isochron Scepter | - | Yes |
| Jaya's Phoenix | - | - |
| Kambal, Profiteering Mayor | - | - |
| Karok Wrangler | - | - |
| Leonin Lightscribe | - | - |
| Lorehold Apprentice | - | - |
| Lorehold Pledgemage | - | - |
| Mardu Siegebreaker | - | - |
| Metamorphic Alteration | - | - |
| Mirror-Style Master | - | - |
| Mnemonic Deluge | - | - |
| Myojin of Cryptic Dreams | - | - |
| Octavia, Living Thesis | - | - |
| Peter Parker's Camera | - | - |
| Phantom Steed | - | - |
| Prismari Apprentice | - | - |
| Prismari Pledgemage | - | - |
| Professor Onyx | - | - |
| Quandrix Apprentice | - | - |
| Quandrix Pledgemage | - | - |
| Redoubled Stormsinger | - | - |
| Reversal of Fortune | - | - |
| Romana II | - | - |
| Sandstorm Crasher | - | - |
| Saruman of Many Colors | - | - |
| Satya, Aetherflux Genius | - | Yes |
| Scouring Swarm | - | - |
| Sedgemoor Witch | - | - |
| Shaun, Father of Synths | - | - |
| Silverquill Apprentice | - | - |
| Silverquill Pledgemage | - | - |
| Spell Satchel | - | - |
| Spellbinder | - | - |
| Spellweaver Helix | - | - |
| Spellweaver Volute | - | - |
| Storm-Kiln Artist | - | - |
| Surge to Victory | - | - |
| Symmetry Sage | - | - |
| Tawnos, Urza's Apprentice | - | - |
| The Peregrine Dynamo | - | - |
| The Tale of Tamiyo | - | - |
| Tzaangor Shaman | - | - |
| Vesuva | - | - |
| Veyran, Voice of Duality | - | - |
| Weaver of Harmony | - | - |
| Wildfire Devils | - | - |
| Witherbloom Apprentice | - | - |
| Witherbloom Pledgemage | - | - |
| Wizard's Spellbook | - | - |
| Zaffai, Thunder Conductor | - | - |
| Zethi, Arcane Blademaster | - | - |

### NO_HANDLER (3)

| Card | AST | Issues |
|------|-----|--------|
| Mobile Clone | none | NO_AST: card not in ast_dataset.jsonl |
| Myra the Magnificent | none | NO_AST: card not in ast_dataset.jsonl |
| Snazzy Aether Homunculus | none | NO_AST: card not in ast_dataset.jsonl |

## incidental (60 cards)

These cards mention 'copy' in oracle text but have no copy mechanic (e.g., reminder text, flavor text within oracle, or references to copying that don't create actual game copies).

<details><summary>Card list</summary>

- A-Rowan, Scholar of Sparks // A-Will, Scholar of Frost
- Augmenter Pugilist // Echoing Equation
- Baron Helmut Zemo
- Barroom Brawl
- Bonus Round
- Bygone Marvels
- Chain Lightning
- Chain Stasis
- Chain of Acid
- Chain of Plasma
- Chain of Silence
- Chain of Smog
- Chain of Vapor
- Display of Power
- Expansion // Explosion
- Extus, Oriq Overlord // Awaken the Blood Avatar
- Eye of the Storm
- Fable of the Mirror-Breaker // Reflection of Kiki-Jiki
- Glasspool Mimic // Glasspool Shore
- Grist, Voracious Larva // Grist, the Plague Swarm
- Grub, Storied Matriarch // Grub, Notorious Auntie
- Hive Mind
- Inspired Skypainter // Maestro's Gift
- Invasion of Alara // Awaken the Maelstrom
- Invasion of Amonkhet // Lazotep Convert
- Invasion of Arcavios // Invocation of the Founders
- Invasion of Vryn // Overloaded Mage-Ring
- Jadzi, Oracle of Arcavios // Journey to the Oracle
- Lorehold Archivist // Restore Relic
- Ludevic, Necrogenius // Olag, Ludevic's Hubris
- Malicious Affliction
- Meletis Charlatan
- Mirror Room // Fractured Realm
- Mirrorhall Mimic // Ghastly Mimicry
- Mirrorwing Dragon
- Ob Nixilis, the Adversary
- Odds // Ends
- Paleontologist's Pick-Axe // Dinosaur Headdress
- Pigment Wrangler // Striking Palette
- Plumb the Forbidden
- Precursor Golem
- Primal Amulet // Primal Wellspring
- Refuse // Cooperate
- Repudiate // Replicate
- Rowan, Scholar of Sparks // Will, Scholar of Frost
- Runo Stromkirk // Krothuss, Lord of the Deep
- Sage of the Skies
- Sevinne's Reclamation
- Social Snub
- Soundwave, Sonic Spy // Soundwave, Superior Captain
- Spelltwine
- String of Disappearances
- Sun-Blessed Guardian // Furnace-Blessed Conqueror
- Terra, Magical Adept // Esper Terra
- The Enigma Jewel // Locus of Enlightenment
- The Everflowing Well // The Myriad Pools
- Ulalek, Fused Atrocity
- Valki, God of Lies // Tibalt, Cosmic Impostor
- Virtue of Knowledge // Vantress Visions
- Wandering Archaic // Explore the Vastlands
</details>

## Engine Gap Analysis

### Critical Gaps

**1. `resolveCreateToken` ignores `IsCopyOf` field**

The `CreateToken` AST node has an `IsCopyOf *Filter` field that is populated by the parser for 48 cards, but `resolveCreateToken()` in resolve.go:1261 never reads it. These cards create generic tokens instead of proper copies, violating:
- §706.2: copy should inherit all copiable characteristics
- §202.3b: token MV should match copied card's MV
- §603.6a: ETB triggers should fire (resolveCreateToken skips InvokeETBHook)
- §614.1c: replacement effects should apply (skips RegisterReplacementsForPermanent)

**2. `resolveCopySpell` is log-only stub**

All 0 spell-copy cards that route through `CopySpell` AST nodes resolve to a log event only. No stack copy is created. Phase 5 implementation pending.

**3. `resolveCreateToken` skips ETB dispatch for ALL tokens**

Even non-copy tokens created via `resolveCreateToken` don't get `InvokeETBHook` / `RegisterReplacementsForPermanent` / `fireObserverETBTriggers`. This means token-ETB interactions (Panharmonicon + token ETBs, Doubling Season replacement on +1/+1 counter ETBs) are broken for AST-routed tokens.

**4. `resolveCopyPermanent` — correct path**

Cards that parse to `CopyPermanent` with `as_token=true` correctly:
- Deep-copy the source card (inherits MV, color, types, abilities)
- Add 'token' type
- Call `RegisterReplacementsForPermanent` + `InvokeETBHook` + `fireObserverETBTriggers`
- Log with §706.10a reference

**5. `CopyPermanentLayered` — correct path**

Cards that parse to `CopyPermanent` with `as_token=false` correctly:
- Snapshot source's printed characteristics via `BaseCharacteristics`
- Register layer-1 continuous effect that overwrites name, P/T, types, colors, abilities, CMC
- For permanent duration: also deep-copies Card pointer
- Proper §706.2 / §613.1a compliance

### Recommended Fixes (Priority Order)

1. **Fix `resolveCreateToken` to handle `IsCopyOf`**: When `e.IsCopyOf != nil`, find the target permanent via `PickTarget`, deep-copy its card, add 'token' type, and call the full ETB cascade. Fixes 48 cards.

2. **Add ETB dispatch to `resolveCreateToken`**: Even for non-copy tokens, call `RegisterReplacementsForPermanent` + `InvokeETBHook` + `fireObserverETBTriggers`. This is the same pattern used in `resolveCopyPermanent` and `enterBattlefieldWithETB`.

3. **Implement `resolveCopySpell`**: Create actual stack copies with new target selection.

4. **Add per-card handlers for high-impact copy cards without AST coverage** (see NO_HANDLER lists above).

