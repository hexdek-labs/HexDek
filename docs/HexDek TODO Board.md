---

kanban-plugin: board

---

## High Priority — Parser (100% Coverage Push)

*Empty — 100% coverage achieved (0 UnknownEffect across 31,963 cards)*


## High Priority — Engine

- [ ] **Remaining 276 commander handlers** — coverage at 447/652 files (681 registered names). Most remaining are 1-2 deck count. Template generator (`cmd/gen-handlers/main.go`) handles simple patterns. #engine #per_card


## High Priority — Platform

- [ ] Negative ELO shame badges — "MID" stamp at 0, escalating tiers for deep negative. Leaderboard bottom-10 wall of shame section #ui #fun
- [ ] Operator platform page/tab (operator profile, deck management, analytics dashboard) #ui #platform
- [ ] Friends system + player profiles — add friends, view each other's profiles/decks #ui #social
- [ ] Bracket-stratified leaderboard tabs — filter by B1-B5, separate rankings per bracket #ui
- [ ] Game Changer cards list on deck page — show which specific GC cards the deck runs #ui


## Medium Priority — Engine

- [ ] **Temporal Pincer** — anon UUID cookie → session tracking → on login stitch all anon device UUIDs to authenticated profile. No PII, all UUIDs. Powers P&R via GraphQL. #infra #platform


## Medium Priority — Platform

- [ ] BOINC-style distributed compute (desktop client → contribute games → earn credits) #distributed
- [ ] Deterministic replay anti-cheat (cryptographic seed, spot-check 2-5%, auto-cauterize bad actors) #anticheat
- [ ] Statistical anomaly detection (per-contributor distribution tracking, 3σ flagging) #anticheat
- [ ] Credit economy (contribute compute → earn credits → spend on own deck testing) #economy
- [ ] Stream/narrator layer (game state → visual renderer → Twitch/OBS output) #stream


## Low Priority

- [ ] **i18n** — internationalize hexdek.dev for global audience #platform
- [ ] Multi-format support beyond Commander (Modern, Legacy deck ratings) #engine
- [ ] Mobile-friendly leaderboard #ui
- [ ] Donations page BOINC/ads buttons — "COMING SOON" placeholders (`Donations.jsx:109,119`) #ui
- [ ] Report analysis placeholder (`Report.jsx:332`) — feature not fully wired #ui


## Done

- [x] **Jhoira of the Ghitu suspend** — proper `OnActivated` handler, picks highest-CMC nonland from hand, calls `SuspendCard(gs, seat, card, 4)`. Removed stub (2026-05-01) #engine
- [x] **Lich's Mastery life observers** — `OnTrigger("life_gained")` draws cards, `OnTrigger("life_lost")` exiles permanents/hand/graveyard. Added `FireCardTrigger("life_lost")` to `resolveLoseLife` (2026-05-01) #engine
- [x] **Ulrich transform events** — `FireCardTrigger("transform")` added to `TransformPermanent()`. Back-face fight trigger on transform to Uncontested Alpha, front-face +4/+4 on transform back (2026-05-01) #engine
- [x] **Wayward Servant ETB observer** — `OnTrigger("token_created")`+`OnTrigger("permanent_etb")`: Zombie ETB drains opponents 1, gains controller 1 (2026-05-01) #engine
- [x] **Coat of Arms layer 7** — `RegisterContinuousEffect` layer 7c: each creature +N/+N where N = other creatures sharing a creature type, all battlefields (2026-05-01) #engine
- [x] **Concession diagnostics** — `ConcessionRecord` in Muninn: commander, turn, board power, life, hand size, opponents alive. `PersistConcessions()`, `SortedConcessions()`, `hexdek-muninn --concessions` flag (2026-05-01) #rating #analytics
- [x] **Dungeon tracking (704.5t)** — enhanced SBA infers completion from `dungeon_level` vs max rooms (7/4/4 for standard dungeons), cleans up flags (2026-05-01) #engine
- [x] **Battle/Siege protectors (704.5w/x)** — full SBA: auto-assigns first living opponent as protector, reassigns on protector death, sacrifices if no opponents. Siege controller=protector reset (2026-05-01) #engine
- [x] **Speed mechanic (704.5z)** — SBA checks permanent types + `start_your_engines` flag, sets `speed=1` on seats without it (2026-05-01) #engine
- [x] **Layer 3 text-changing effects** — STUB: LayerText=3 slot exists in pipeline (layers.go:429), no effects register. Intentional no-op per CONFIDENCE_MATRIX — no portfolio deck uses Magical Hack / Trait Doctoring. Deferred until meta demand. (2026-05-01) #engine #layers
- [x] **opponentLikelyHasWrath expansion** — replaced boolean with `wrathProbability()` returning graded float64. Factors: hand size (0.04/card), mana availability, opponent colors (W+0.15, B+0.10, R+0.05), archetype (Control+0.15), cast cadence (nothing cast + full hand + mana = +0.10), prior wrath history from 3rd Eye cardsSeen (+0.20). Cap 0.95 (2026-05-01) #engine #evaluator
- [x] **Partner-aware mulligan adjustment** — detects partner pair (CommanderNames >= 2), collects hand colors from lands, counts enablers per commander's colors. Mulligans if one commander has 0 enablers and hand lacks star cards / combo pieces (2026-05-01) #engine
- [x] **Transform recognition in cardHeuristic** — IsMDFC() flexibility bonus +0.10, back-face-is-land +0.10, back-face cheaper and castable +0.08. Scores both faces of DFC cards (2026-05-01) #engine
- [x] **Tournament runner post-game stat emission** — SeatStats struct (15 fields: life, board, hand, graveyard, mana sources, spells, creatures, conceded, etc). Emitted per-game in PostGameStats without event log. `CountManaRocksAndLands` exported for cross-package use (2026-05-01) #engine #analytics
- [x] **PartnerSynergy evaluator dimension** — partner pair on-field bonus, 4-color coverage scoring, complementary role detection (draw/attack/tutor/removal), tax penalty for repeated deaths. 13 archetype weight profiles. Tests (2026-05-01) #engine #evaluator
- [x] **ActivationTempo evaluator dimension** — non-mana activated ability scoring, untapped vs tapped weighting, repeatable (no-tap) engines boosted, high-impact activation bonus, opponent-relative comparison. Tests (2026-05-01) #engine #evaluator
- [x] **ToolboxBreadth evaluator dimension** — tutors in hand, modal spells, MDFC flexibility, non-mana board activations, tutor-target-aware bonus from Freya profile. Tests (2026-05-01) #engine #evaluator
- [x] **Threat trajectory prediction** — forward-looking opponent power projection: board power + deployment potential (hand × mana) + spell cadence bonus. Clamps to [-2, 0]. Tests (2026-05-01) #engine #evaluator
- [x] **UCB1 exploration factor per archetype/turn** — `refreshExplorationFactor()` replaces hardcoded √2. Aggro/Tribal=1.0, Combo=1.8, Control=1.6, Stax=1.2. Early game +0.3, late game -0.3 decay. Floor 0.5. Cached per turn (2026-05-01) #engine #evaluator
- [x] **Dynamic evaluator weight rescaling** — `rescaleWeights()` adjusts all 16 dimension weights by game stage (early boosts mana/card, late boosts combo/threat/board) and position (behind boosts combo/toolbox, ahead boosts card/mana/life). Tests (2026-05-01) #engine #evaluator
- [x] **Light mode toggle** — `[data-theme="light"]` CSS vars, toggle button on all pages (drilldown, leaderboard, import, game), localStorage persistence (2026-05-01) #ui
- [x] **Curve analysis UI** — mana curve bar chart + color balance demand/supply visualization in Freya Analysis tab. ManaCurve + ColorBalance added to strategy.json (2026-05-01) #ui #analytics
- [x] **Deck page auto-refresh on Freya push** — SSE endpoint `/api/decks/{owner}/{id}/events` broadcasts `freya_complete` event when analysis finishes. Drilldown page auto-reloads data via EventSource (2026-05-01) #ui #infra
- [x] **Exile-cast pattern** — Dauthi Voidwalker (replacement effect via graveyard_enter trigger), Emry (ZoneCastGrant from graveyard + ExileOnResolve), Urza (free cast from exile + EOT cleanup) (2026-05-01) #engine
- [x] **Clone/copy handlers** — already implemented: Phantasmal Image (DeepCopy + sac-on-target flag), Riku (creature token copy + spell stack copy). Stale TODO removed (2026-05-01) #engine
- [x] **ELO-bracket correlation re-check** — 485K games, rho=-0.49 (inverted). B1 avg 1649, B5 avg 1315 in standard ELO. HexELO preserves bracket ordering via seeded starts. Drift outliers: 921/1196 decks (2026-05-01) #engine #rating
- [x] **Soulshift keyword** — CR §702.46 implemented: CheckSoulshift fires on creature death, returns highest-CMC Spirit with mana value ≤ N from graveyard to hand. CONFIDENCE_MATRIX updated to SOLID. 0 STUB keywords remain (2026-05-01) #engine #keywords
- [x] **Opponent graveyard threat tracking** — 12th evaluator dimension `scoreOpponentGraveyard`: reanimation targets, flashback/escape spells, enablers on battlefield, known GY-abuse commanders. Weighted per archetype (2026-05-01) #engine #evaluator
- [x] **Color-aware mana sequencing** — ChooseLandToPlay now scans hand spells for color pip demand, boosts lands producing needed colors with deficit-aware weighting. Near-castable spells (CMC ≤ available+1) get 2x priority (2026-05-01) #engine #evaluator
- [x] **Jeska's Will** — exile-play permission via RegisterZoneCastGrant for each exiled card + end-of-turn DelayedTrigger cleanup. Last GC clause gap closed (2026-05-01) #engine
- [x] **Panoptic Mirror** — imprint persistence via sync.Map (Isochron Scepter pattern), upkeep creates StackItem{IsCopy: true} + InvokeResolveHook. Last GC clause gap closed (2026-05-01) #engine
- [x] **P1: 100% parser coverage** — 0 UnknownEffect across 31,963 cards. `final_13_cards.py` extension covers forced-attack, variable P/T, as-enters, monstrous triggers, composite ETB, search-library, card-put-into-zone (2026-05-01) #parser
- [x] **Graveyard-leave observer hook** — `graveyard_leave` event fires from MoveCard when fromZone=="graveyard". Tormod creates 2/2 Zombie, Imotekh creates 2x 2/2 Necron Warriors on artifact leave (2026-05-01) #engine
- [x] **Land-tap hook** — `land_tapped_for_mana` event fires from AddManaFromPermanent when source is land. Caged Sun doubles controller's mana, Gauntlet of Power doubles all players' mana (2026-05-01) #engine
- [x] **Cast observer → Door of Destinies** — `spell_cast` already fires; wired Door of Destinies charge counter increment on controller cast (2026-05-01) #engine
- [x] **Bracket-aware tournament grinder** — switched AssemblePod → AssembleBracketPod in showmatch, populates Bracket field from ELO cache (2026-05-01) #engine #matchmaking
- [x] **Keyword stubs audit** — CONFIDENCE_MATRIX reconciled 2026-04-30, all 30+ keywords now SOLID. Only Soulshift remains (rare). Stale TODO cleaned (2026-05-01) #engine #keywords
- [x] **Cleanup: internal/rules/** — removed empty package, test goldens item stale (no files exist) (2026-05-01) #cleanup
- [x] **P12: TurnFaceUp effect handler** — 44th effect type wired in resolve.go, calls existing dfc.go:TurnFaceUp, megamorph +1/+1 counter support, 3 tests (2026-05-01) #engine
- [x] **P6: Saga ETB + chapter tagging** — saga_chapter detection moved before extension loop (all chapters tagged correctly), initSagaLoreCounters on ETB sets saga_final_chapter + first lore counter, 2 tests (2026-05-01) #parser #engine
- [x] **P2: Ability word extension** — already wired via load_extensions() STATIC_PATTERNS → EXT_STATIC_PATTERNS. Stale TODO: 820 cards resolved, coverage at 99.96% (2026-05-01) #parser
- [x] **SBA 704.5r counter limit enforcement** — parses "can't have more than N counters" from AST raw text, trims excess, 3 tests (2026-05-01) #engine
- [x] **AI autopilot behavior policy** — plays lands, taps mana, casts highest-CMC affordable spells, alpha-strikes in combat (2026-05-01) #engine #ai
- [x] **ELO reset #2** — cleared 1196 entries + 367 games + 4585 card stats, grinder resampling with 447 handlers active (2026-05-01) #rating
- [x] **Handler coverage push** — 66→447 handler files across waves 1-14, 6 dev hexes. Template generator `cmd/gen-handlers/main.go` (201 auto-gen + 228 manual) (2026-05-01) #engine #per_card
- [x] **53/53 GC per-card handlers** — all Game Changers have registered handlers (2026-04-30) #engine #per_card
- [x] **Spectator UI scroll fix** — overflow-anchor + CSS containment + rAF log scroll (2026-05-01) #ui
- [x] **Turn bar redesign** — left-anchor commander, right-anchor perms, kill blinker, ellipsis overflow (2026-05-01) #ui
- [x] **Stax lock wiring** — Null Rod/Ouphe/Totem via StaxCheck, Grand Abolisher cast block (2026-04-30) #engine
- [x] **CreateToken hook** — token_created trigger fires, Chatterfang + Anointed Procession + Pitiless Plunderer (2026-04-30) #engine
- [x] **Dual-track ELO** — standard + HexELO (bracket-weighted) computed every game (2026-04-30) #rating
- [x] **HexELO drift detection** — /api/live/elo/drift endpoint, outlier tagging (2026-04-30) #rating
- [x] **Loss reason display** — spectator UI shows GG reason (cmdr dmg, life, poison, etc) (2026-05-01) #ui
- [x] **Partner commander casting priority** — +0.20 base, +0.45 when partner on board (2026-05-01) #engine
- [x] **Sacrifice-as-value overhaul** — drain/draw/ramp payoffs, 1.5x fodder multiplier (2026-05-01) #engine
- [x] **Sandbagging exemption** — aristocrats/combo/enchantress/artifacts at 30% penalty (2026-05-01) #engine
- [x] **Reanimate activation awareness** — activationHeuristic graveyard-to-battlefield scoring (2026-05-01) #engine
- [x] Fix Tergrid recursive trigger crash (depth guard + total trigger cap) #engine
- [x] Fix Obeka wrong ability resolution #engine
- [x] Fix DFC commander name mismatch #engine
- [x] Fix compound type filter for cast triggers #engine
- [x] Giga Quorum v1 (30,597 games, 18 decks, 7m11s) #tournament
- [x] Giga Quorum v2 (30,595 games, 18 decks, 12m, trigger-capped) #tournament
- [x] Universal zone-change system (MoveCard) — 0 regressions across 64K tests #engine
- [x] Trigger dispatch audit — 8 dead triggers found, 7 fixed #engine
- [x] ELO rating system (standard K=32, multiplayer pairwise) #rating
- [x] Tune trigger cap — tested 2000/5000, converged at depth 15 (2000 is optimal) #engine
- [x] Giga Quorum v3 (30,595 games, 5000 cap, identical to v2 — confirms convergence) #tournament
- [x] Crash investigation: all 5 "crashes" are 90s wall-clock timeouts, not panics. Eshki in 5/5 pods. #engine
- [x] TrueSkill rating system — multiplayer Bayesian (μ, σ), pairwise decomposition, wired into tournament + round-robin #rating
- [x] Content-addressable deck hashing (SHA256 sorted card list) #rating
- [x] Giga Quorum v4 (30,595 games, TrueSkill enabled) — Yuriko #1, Coram #2, Oloro drops to #5 #tournament
- [x] YggdrasilHat political AI — unified hat with 8-dim evaluator, threat scoring, grudge tracking, budget system #hat
- [x] Giga Quorum v5 (30,598 games, Yggdrasil budget=50) — Shalai #1 win, Varina #1 TS, Soraya #2→#16, politics reshuffles meta #tournament
- [x] Rivalry tracker (per-deck matchup W/L, canonical key pairing, cross-run merging) #analytics
- [x] Killer-victim elimination tracking (threat graph, backward event log scan, kingmaker detection) #analytics
- [x] Bayesian prior inheritance for deck versioning (μ carries, σ inflates by card delta) #rating
- [x] Deck versioning DAG (content-addressable SHA256, lineage, HEAD leaderboard) #schema
- [x] Matchmaking scheduler (rating-aware pod assembly, info gain scoring) #matchmaking
- [x] Deck import from Moxfield URL (auto-register, auto-hash, auto-rate) #ui
- [x] Bug/Suggestion report (red footer button → form → JSON flat-file persistence) #ui #platform
- [x] Footer statusbar (bug report link, donate link, user status) #ui
- [x] About page (project overview, philosophy, tech stack, no-ads stance) #ui #platform
- [x] Donations page (monthly COGs breakdown, donation tracker bar, philosophy) #ui #platform
- [x] User profile page (display name, owner name for deck filtering) #ui #platform
- [x] Splash page GitHub + docs links #ui
- [x] W/L color fix (wins green, losses red across DeckList, DeckArchive, gauntlet) #ui
- [x] Parser Wave 4: 73% → 86.42% (+4,193 cards, 125 new rules) #parser
- [x] Engine resolve stubs: 163/163 mod handlers promoted (0 remaining) #engine
- [x] Per-card handlers: 17 commander staples (Sol Ring, Force of Will, Smothering Tithe, etc.) #engine
- [x] Per-card handlers: Necrotic Ooze, Bolas's Citadel, Food Chain, Underworld Breach improved #engine
- [x] Spell-copy tracking — `Card.IsCopy` bool + SBA 704.5e copy cleanup #engine #copy
- [x] Layer 7d P/T switching — RegisterPTSwitch, RegisterDoranSiegeTower #engine #layers
- [x] Reflexive triggers — QueueReflexiveTrigger via DelayedTrigger system #engine #triggers
- [x] Damage distribution (601.2d) — distributeDamage + DamageDistributor interface #engine
- [x] Monarch system — BecomeMonarch wired via court cards #engine
- [x] Annihilator/Afflict/Rampage/Bushido keywords — AST-aware N extraction #engine #keywords
- [x] Elimination logging — `>>>` death entries with loss reason (life/poison/cmdr/mill) #engine #spectator
- [x] Custom brutalist sliders — 3px track, 12x12 square thumb, var(--ok) fill #ui #design
- [x] Web leaderboard — sortable table, search, mobile sort bar, clickable rows, confidence dots #ui
- [x] ELO confidence badges on deck list + drilldown #ui #rating
- [x] Deck drilldown mana curve/color pie (client-side fallback when no Freya data) #ui
- [x] Wire Forge gauntlet backend — game count selector, RUN button, progress bar, results #ui
- [x] Scryfall card art prefetcher (`cmd/hexdek-artfetch/`) + RAM cache warming at startup #infra
- [x] Bracket System v2 — WotC-aligned 5-tier (Exhibition/Core/Upgraded/Optimized/cEDH) + 53 Game Changers scoring #engine #rating
- [x] Bracket-aware matchmaking — AssembleBracketPod with soft ±1 weighting #matchmaking



%% kanban:settings
```
{"kanban-plugin":"board","list-collapse":[false,false,false,false,false]}
```
%%
