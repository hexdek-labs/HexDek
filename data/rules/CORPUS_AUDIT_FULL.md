# Thor — Per-Card Interaction Stress Test Report

**Date:** 2026-04-26 12:01:15
**Cards tested:** 36083
**Total tests:** 31965
**Failures:** 116
**Time:** 0s
**Rate:** 90592 tests/s

## Invariant Violations (116)

| Card | Interaction | Invariant | Message |
|------|-------------|-----------|--------|
| Unscrupulous Contractor | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Lore Weaver | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Mind Spiral | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Scholar of Stars | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Nefarox, Overlord of Grixis | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Marina Vendrell's Grimoire | corpus_audit_discard |  | [discard] TurnStructure: TurnStructure: active seat 0 is Lost but life is 20 with no LossReason |
| Stroke of Genius | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Zurgo Stormrender | corpus_audit_lose_life |  | lose_life: expected life-1 (seat 1), got delta=0 |
| Boundary Lands Ranger | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Electroduplicate | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| See Double | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Fell Stinger | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Quantum Misalignment | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Will Kenrith | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Primal Empathy | corpus_audit_counter_mod |  | counter_mod: expected 1 +1/+1 counters, none found |
| Emeritus of Ideation // Ancestral Recall | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Dino DNA | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Damnable Pact | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Flubs, the Fool | corpus_audit_discard |  | discard: expected hand-1 (seat 1), got delta=0 |
| Combat Tutorial | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Stolen Identity | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Krav, the Unredeemed | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Gutsplitter Gang | corpus_audit_lose_life |  | lose_life: expected life-3 (seat 1), got delta=0 |
| Council's Deliberation | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Foreboding Fruit | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Ghired, Mirror of the Wilds | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Littjara Mirrorlake | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Scheming Silvertongue // Sign in Blood | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Ceta Sanctuary | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=-1 lib delta=0 events=0 |
| Dutiful Replicator | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Walking Ballista | corpus_audit_damage |  | damage: expected 1 damage dealt, no matching life/damage/destroy observed |
| Wandering Champion | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Ancestral Recall | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Deep Analysis | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Mirrorpool | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Comparative Analysis | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Self-Reflection | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Heat Shimmer | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| A-Cosmos Elixir | corpus_audit_gain_life |  | gain_life: expected life+2 (seat 0), got delta=0 |
| Aetherflux Reservoir | corpus_audit_damage |  | damage: expected 50 damage dealt, no matching life/damage/destroy observed |
| Saheeli Rai | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Blue Sun's Zenith | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Savra, Queen of the Golgari | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Dirgur Focusmage // Braingeyser | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Spiked Ripsaw | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Hakbal of the Surging Soul | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Kiora, the Tide's Fury | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Urza, Prince of Kroog | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Painful Lesson | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Artificer Class | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Adagia, Windswept Bastion | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Harrowing Journey | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Tribute to the World Tree | corpus_audit_counter_mod |  | counter_mod: expected 2 +1/+1 counters, none found |
| Dance of Many | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Quasiduplicate | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Roiling Waters | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Donnie & April, Adorkable Duo | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Faerie Artisans | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Bloodcrazed Socialite | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Etched Oracle | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Chandra, Flameshaper | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Rhox Meditant | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Hazel of the Rootbloom | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Rally the Galadhrim | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Historian of Zhalfir | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Faramir, Prince of Ithilien | corpus_audit_create_token |  | create_token: expected battlefield+3 (seat 0), got delta=0 events=0 |
| Esika's Chariot | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Inspiration | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Invasion of Alara // Awaken the Maelstrom | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Archfiend of the Dross | corpus_audit_counter_mod |  | [counter_mod] TurnStructure: TurnStructure: active seat 0 is Lost but life is 20 with no LossReason |
| Echo Storm | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Ancestral Vision | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Toil // Trouble | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Miles "Tails" Prower | corpus_audit_counter_mod |  | counter_mod: expected 1 flying counters, none found |
| Braingeyser | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Cabaretti Confluence | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Cursed Recording | corpus_audit_damage |  | [damage] ResourceConservation: ResourceConservation: seat 0 is Lost but has ManaPool=10 |
| Gisa's Favorite Shovel | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Thraximundar | corpus_audit_sacrifice |  | sacrifice: no permanent was sacrificed |
| Cackling Counterpart | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Carrion Feeder | corpus_audit_counter_mod |  | counter_mod: expected 1 +1/+1 counters, none found |
| Spitting Image | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Irenicus's Vile Duplication | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Repudiate // Replicate | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Penregon Strongbull | corpus_audit_buff |  | buff: expected P/T change (+1/+1), no modification observed |
| Penregon Strongbull | corpus_audit_damage |  | damage: expected 1 damage dealt, no matching life/damage/destroy observed |
| Scouring Swarm | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Sazacap's Brew | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Fated Infatuation | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Expansion // Explosion | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Cost of Brilliance | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Furgul, Quag Nurturer | corpus_audit_counter_mod |  | counter_mod: expected 1 +1/+1 counters, none found |
| Here Comes a New Hero! | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Sign in Blood | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Kindle the Inner Flame | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Caretaker's Talent | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Blood Pact | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Etched Monstrosity | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Drown in Dreams | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Relm's Sketching | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Channeled Force | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Warrior's Resolve | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Can't Quite Recall | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Mirror Room // Fractured Realm | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Urza, Lord High Artificer | corpus_audit_add_mana |  | add_mana: mana pool did not increase |
| Knight of the Mists | corpus_audit_destroy |  | destroy: no permanent left battlefield or entered graveyard |
| Flame of Anor | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Cosmos Elixir | corpus_audit_gain_life |  | gain_life: expected life+2 (seat 0), got delta=0 |
| Homesickness | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Decorum Dissertation | corpus_audit_draw |  | draw: expected hand+2 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Donatello, Turtle Techie | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Compulsive Research | corpus_audit_draw |  | draw: expected hand+3 (seat 0), got hand delta=0 lib delta=0 events=1 |
| Settlement Blacksmith | corpus_audit_draw |  | draw: expected hand+1 (seat 0), got hand delta=0 lib delta=0 events=0 |
| Rite of Replication | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Specimen Collector | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |
| Rootcast Apprenticeship | corpus_audit_create_token |  | create_token: expected battlefield+1 (seat 0), got delta=0 events=0 |

## Failures by Interaction

| Interaction | Failures |
|-------------|----------|
| corpus_audit_draw | 52 |
| corpus_audit_create_token | 38 |
| corpus_audit_sacrifice | 7 |
| corpus_audit_counter_mod | 6 |
| corpus_audit_damage | 4 |
| corpus_audit_gain_life | 2 |
| corpus_audit_discard | 2 |
| corpus_audit_lose_life | 2 |
| corpus_audit_buff | 1 |
| corpus_audit_add_mana | 1 |
| corpus_audit_destroy | 1 |
