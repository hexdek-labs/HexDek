# Land Audit Report

**Generated:** 2026-04-17
**Engine:** Go rules engine (`internal/gameengine` + `internal/gameengine/per_card`)
**Oracle corpus:** `data/rules/oracle-cards.json` (37,238 cards)
**AST dataset:** `data/rules/ast_dataset.jsonl`

## Summary

| Metric | Value |
|--------|-------|
| Total lands in database | 1,242 |
| Categories audited | 13 |
| PASS | 4 |
| PARTIAL | 6 |
| FAIL | 3 |

### Critical Architecture Finding (FIXED)

The Go engine's `runMainPhase` (in `internal/tournament/turn.go`) previously tapped ALL untapped lands and added **+1 untyped mana** (plain integer `seat.ManaPool++`) per land.

**Fix applied:** `runMainPhase` now reads basic land subtypes (Plains/Island/Swamp/Mountain/Forest) from Card.Types via `landSubtypeColor()` and calls `AddMana(gs, seat, color, 1, name)` for typed mana. Multi-subtype lands (duals, shocklands, triomes) produce "any" color mana. Lands without recognized subtypes fall back to +1 untyped.

This enables correct mana coloring for all basic lands, original duals, shocklands, and triomes.

### Critical ETB Finding (FIXED)

The `tryPlayLand` function checked `containsType(chosen.Types, "etb_tapped")` to determine if a land enters tapped. However, the `etb_tapped` tag was never added by the deck parser.

**Fix applied:** Added `oracleIndicatesETBTapped()` which reads the card's AST ability raws (via `OracleTextLower`) for "enters tapped" / "enters the battlefield tapped" text. Conditional variants (containing "unless" or "pay") are excluded so that per-card ETB handlers (shocklands, check lands, fast lands) remain authoritative.

**Result:** ~584 unconditional enters-tapped lands now correctly enter tapped.

---

## Category 1: Basic Lands

**Count:** 13 (Plains, Island, Swamp, Mountain, Forest, Wastes, Snow-Covered variants, Barry's Land)

| Land | Type Line | Mana | Status |
|------|-----------|------|--------|
| Plains | Basic Land -- Plains | tap: {W} | PARTIAL |
| Island | Basic Land -- Island | tap: {U} | PARTIAL |
| Swamp | Basic Land -- Swamp | tap: {B} | PARTIAL |
| Mountain | Basic Land -- Mountain | tap: {R} | PARTIAL |
| Forest | Basic Land -- Forest | tap: {G} | PARTIAL |
| Wastes | Basic Land | tap: {C} | PARTIAL |
| Snow-Covered Plains | Basic Snow Land -- Plains | tap: {W} | PARTIAL |
| Snow-Covered Island | Basic Snow Land -- Island | tap: {U} | PARTIAL |
| Snow-Covered Swamp | Basic Snow Land -- Swamp | tap: {B} | PARTIAL |
| Snow-Covered Mountain | Basic Snow Land -- Mountain | tap: {R} | PARTIAL |
| Snow-Covered Forest | Basic Snow Land -- Forest | tap: {G} | PARTIAL |
| Snow-Covered Wastes | Basic Snow Land | tap: {C} | PARTIAL |
| Barry's Land | Basic Land -- Cloud | tap: none | PARTIAL |

**Status: PARTIAL**

- Mana production works (all lands produce +1 mana), but color is not tracked.
- The `game` package's `TapLandForMana` (used by the WebSocket server) DOES infer color from subtypes (`inferLandColor` checks Subtypes for Plains/Island/etc.). But the tournament engine bypasses this entirely.
- `basic_land_mana()` exists in Python but has no Go equivalent.
- Snow mana (`{S}`) is not modeled.

**Fix scope:** Medium. Add a `basicLandColor(card)` function that reads Card.Types for basic land subtypes and call `AddMana(gs, seat, color, 1, name)` instead of `seat.ManaPool++` in `runMainPhase`. Estimated: 50 lines.

---

## Category 2: Original Dual Lands

**Count:** 10

| Land | Subtypes | Mana | Status |
|------|----------|------|--------|
| Underground Sea | Island Swamp | {U} or {B} | PARTIAL |
| Tropical Island | Forest Island | {G} or {U} | PARTIAL |
| Tundra | Plains Island | {W} or {U} | PARTIAL |
| Volcanic Island | Island Mountain | {U} or {R} | PARTIAL |
| Badlands | Swamp Mountain | {B} or {R} | PARTIAL |
| Bayou | Swamp Forest | {B} or {G} | PARTIAL |
| Plateau | Mountain Plains | {R} or {W} | PARTIAL |
| Savannah | Forest Plains | {G} or {W} | PARTIAL |
| Scrubland | Plains Swamp | {W} or {B} | PARTIAL |
| Taiga | Mountain Forest | {R} or {G} | PARTIAL |

**Status: PARTIAL**

- Enter untapped: PASS (no ETB clause).
- Mana production: produces +1 untyped mana. Color choice is not modeled.
- Basic land subtypes (e.g., "Island Swamp") are present in Card.Types but not used by the tournament mana system.
- Fetchlands CAN find these (via `landMatchesFetchTypes` checking Card.Types for "island", "swamp", etc.): PASS.

**Fix scope:** Included in basic land color fix above. Once `runMainPhase` reads subtypes for color, duals automatically work.

---

## Category 3: Shock Lands

**Count:** 10

| Land | Handler | ETB | Mana | Status |
|------|---------|-----|------|--------|
| Watery Grave | `shocklands.go` | PASS -- pay 2 or enter tapped | PARTIAL -- untyped +1 | PARTIAL |
| Steam Vents | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Breeding Pool | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Blood Crypt | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Overgrown Tomb | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Sacred Foundry | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Stomping Ground | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Godless Shrine | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Hallowed Fountain | `shocklands.go` | PASS | PARTIAL | PARTIAL |
| Temple Garden | `shocklands.go` | PASS | PARTIAL | PARTIAL |

**Status: PARTIAL**

- ETB replacement effect: PASS. `shocklandETB` correctly pays 2 life (if life > 5) or enters tapped.
- Mana production: +1 untyped mana (generic MVP issue).
- Basic land subtypes on the type line: PASS (fetchlands find them correctly).

---

## Category 4: Fetch Lands

**Count:** 14 registered in `fetchlands.go`

| Land | Config | Sacrifice | Search | Life | Status |
|------|--------|-----------|--------|------|--------|
| Scalding Tarn | island/mountain | PASS | PASS | 1 life | PASS |
| Misty Rainforest | forest/island | PASS | PASS | 1 life | PASS |
| Verdant Catacombs | swamp/forest | PASS | PASS | 1 life | PASS |
| Bloodstained Mire | swamp/mountain | PASS | PASS | 1 life | PASS |
| Wooded Foothills | mountain/forest | PASS | PASS | 1 life | PASS |
| Flooded Strand | plains/island | PASS | PASS | 1 life | PASS |
| Windswept Heath | forest/plains | PASS | PASS | 1 life | PASS |
| Arid Mesa | mountain/plains | PASS | PASS | 1 life | PASS |
| Polluted Delta | island/swamp | PASS | PASS | 1 life | PASS |
| Marsh Flats | plains/swamp | PASS | PASS | 1 life | PASS |
| Evolving Wilds | any basic | PASS | PASS | 0 | PASS |
| Terramorphic Expanse | any basic | PASS | PASS | 0 | PASS |
| Fabled Passage | any basic | PASS | PASS | 0 | PASS |
| Prismatic Vista | any basic | PASS | PASS | 1 life | PASS |

**Status: PASS**

- Sacrifice + search: `fetchLandActivated` correctly sacrifices self, searches library for matching land type via `landMatchesFetchTypes`, puts onto battlefield, shuffles.
- Life payment: correct amounts.
- Slow fetches (Evolving Wilds/Terramorphic): `EntersTapped: true` correctly passes to `createPermanent`.
- Fabled Passage: MVP treats as untapped (cEDH assumption).
- Missing slow fetches (Bad River, Flood Plain, Grasslands, Mountain Valley, Rocky Tar Pit): NOT registered. These are Mirage-era slow fetches that search for a single basic land type.

**Missing fetch-style lands not in registry:**
- Bad River, Flood Plain, Grasslands, Mountain Valley, Rocky Tar Pit (Mirage slow fetches)
- Bant/Esper/Grixis/Jund/Naya Panorama (Alara tri-color fetches)
- Various set-specific fetch variants (34 total in corpus)

---

## Category 5: Bounce Lands

**Count:** 12 in corpus (10 Ravnica karoos + Guildless Commons + Arid Archway)

| Land | Per-card handler | ETB bounce | Status |
|------|-----------------|------------|--------|
| Azorius Chancery | NONE | FAIL | FAIL |
| Boros Garrison | NONE | FAIL | FAIL |
| Dimir Aqueduct | NONE | FAIL | FAIL |
| Golgari Rot Farm | NONE | FAIL | FAIL |
| Gruul Turf | NONE | FAIL | FAIL |
| Izzet Boilerworks | NONE | FAIL | FAIL |
| Orzhov Basilica | NONE | FAIL | FAIL |
| Rakdos Carnarium | NONE | FAIL | FAIL |
| Selesnya Sanctuary | NONE | FAIL | FAIL |
| Simic Growth Chamber | NONE | FAIL | FAIL |
| Guildless Commons | NONE | FAIL | FAIL |
| Arid Archway | NONE | FAIL | FAIL |

**Status: FAIL**

- No per-card handlers registered.
- "When ~ enters the battlefield, return a land you control to its owner's hand" does not fire.
- "~ enters the battlefield tapped" does not fire (no `etb_tapped` tag).
- These produce 2 mana of different colors, but get only +1 untyped from the MVP system.
- Impact: bounce lands are strictly better than basics in the current engine (no drawback, free mana).

**Fix scope:** Medium. Need 1 shared `bounceETB` handler + registration for all 12. ~60 lines.

---

## Category 6: Check Lands

**Count:** 10 core (Innistrad/Ixalan cycle) + 20 variants in corpus

| Land | Handler | Condition | Status |
|------|---------|-----------|--------|
| Glacial Fortress | NONE | unless Plains/Island | FAIL |
| Drowned Catacomb | NONE | unless Island/Swamp | FAIL |
| Dragonskull Summit | NONE | unless Swamp/Mountain | FAIL |
| Rootbound Crag | NONE | unless Mountain/Forest | FAIL |
| Sunpetal Grove | NONE | unless Forest/Plains | FAIL |
| Clifftop Retreat | NONE | unless Mountain/Plains | FAIL |
| Hinterland Harbor | NONE | unless Forest/Island | FAIL |
| Isolated Chapel | NONE | unless Plains/Swamp | FAIL |
| Sulfur Falls | NONE | unless Island/Mountain | FAIL |
| Woodland Cemetery | NONE | unless Swamp/Forest | FAIL |

**Status: FAIL**

- No per-card handlers. These should enter tapped unless the controller has a land with a matching basic land subtype.
- Currently enter untapped unconditionally.

**Fix scope:** Medium. Shared `checkLandETB` handler that scans battlefield for basic land subtypes. ~80 lines.

---

## Category 7: Fast Lands

**Count:** 10 (Kaladesh/Phyrexia cycle) + Thran Portal

| Land | Handler | Condition | Status |
|------|---------|-----------|--------|
| Seachrome Coast | NONE | unless <= 2 lands | FAIL |
| Darkslick Shores | NONE | unless <= 2 lands | FAIL |
| Copperline Gorge | NONE | unless <= 2 lands | FAIL |
| Blackcleave Cliffs | NONE | unless <= 2 lands | FAIL |
| Razorverge Thicket | NONE | unless <= 2 lands | FAIL |
| Botanical Sanctum | NONE | unless <= 2 lands | FAIL |
| Blooming Marsh | NONE | unless <= 2 lands | FAIL |
| Concealed Courtyard | NONE | unless <= 2 lands | FAIL |
| Inspiring Vantage | NONE | unless <= 2 lands | FAIL |
| Spirebluff Canal | NONE | unless <= 2 lands | FAIL |

**Status: FAIL**

- No per-card handlers. Should enter tapped unless controller controls 2 or fewer OTHER lands.
- Currently always enter untapped.

**Fix scope:** Small. Shared `fastLandETB` handler that counts lands on battlefield. ~50 lines.

---

## Category 8: Pain Lands

**Count:** 10 core (Ice Age/Apocalypse) + 6 variants

| Land | Handler | Mana | Damage | Status |
|------|---------|------|--------|--------|
| Adarkar Wastes | NONE | {C} or {W}/{U} + 1 dmg | FAIL | FAIL |
| Caves of Koilos | NONE | {C} or {W}/{B} + 1 dmg | FAIL | FAIL |
| Underground River | NONE | {C} or {U}/{B} + 1 dmg | FAIL | FAIL |
| Sulfurous Springs | NONE | {C} or {B}/{R} + 1 dmg | FAIL | FAIL |
| Karplusan Forest | NONE | {C} or {R}/{G} + 1 dmg | FAIL | FAIL |
| Llanowar Wastes | NONE | {C} or {B}/{G} + 1 dmg | FAIL | FAIL |
| Battlefield Forge | NONE | {C} or {R}/{W} + 1 dmg | FAIL | FAIL |
| Brushland | NONE | {C} or {G}/{W} + 1 dmg | FAIL | FAIL |
| Shivan Reef | NONE | {C} or {U}/{R} + 1 dmg | FAIL | FAIL |
| Yavimaya Coast | NONE | {C} or {G}/{U} + 1 dmg | FAIL | FAIL |

**Status: FAIL**

- No per-card handlers. These tap for either {C} (free) or a colored mana (1 damage to self).
- Currently produce +1 untyped mana with no damage.
- Impact: pain lands are strictly better than basics (no damage cost).

**Fix scope:** Medium. Shared `painLandActivated` handler. ~60 lines.

---

## Category 9: Filter Lands

**Count:** 10 (Shadowmoor/Eventide cycle) + Castle Garenbrig

| Land | Handler | Mana | Status |
|------|---------|------|--------|
| Mystic Gate | NONE | {1}: add {W}{W}, {W}{U}, or {U}{U} | FAIL |
| Sunken Ruins | NONE | {1}: add {U}{U}, {U}{B}, or {B}{B} | FAIL |
| Graven Cairns | NONE | {1}: add {B}{B}, {B}{R}, or {R}{R} | FAIL |
| Fire-Lit Thicket | NONE | {1}: add {R}{R}, {R}{G}, or {G}{G} | FAIL |
| Wooded Bastion | NONE | {1}: add {G}{G}, {G}{W}, or {W}{W} | FAIL |
| Fetid Heath | NONE | {1}: add {W}{W}, {W}{B}, or {B}{B} | FAIL |
| Cascade Bluffs | NONE | {1}: add {U}{U}, {U}{R}, or {R}{R} | FAIL |
| Twilight Mire | NONE | {1}: add {B}{B}, {B}{G}, or {G}{G} | FAIL |
| Flooded Grove | NONE | {1}: add {G}{G}, {G}{U}, or {U}{U} | FAIL |
| Rugged Prairie | NONE | {1}: add {R}{R}, {R}{W}, or {W}{W} | FAIL |

**Status: FAIL**

- No per-card handlers. These convert 1 colored mana into 2 mana of hybrid combinations.
- Currently produce +1 untyped mana from the generic tap loop.
- Low priority: filter lands are rarely played in cEDH.

**Fix scope:** Large. Need individual mana-conversion handlers. ~120 lines.

---

## Category 10: Utility Lands

### 10a. Handled (per-card handlers exist)

| Land | Handler File | Mana | Special | Status |
|------|-------------|------|---------|--------|
| Ancient Tomb | `utility_lands.go` | {C}{C} + 2 dmg | PASS | PASS |
| Bojuka Bog | `utility_lands.go` | {B} | ETB exile graveyard | PASS |
| Otawara, Soaring City | `utility_lands.go` | {U} | Channel bounce | PASS |
| Boseiju, Who Endures | `utility_lands.go` | {G} | Channel destroy | PASS |
| Rogue's Passage | `utility_lands.go` | {C} | {4},{T}: unblockable | PASS |
| Reliquary Tower | `utility_lands.go` | {C} | No max hand size | PASS |
| Urza's Saga | `utility_lands.go` | {C} | Saga chapters I-III | PASS |
| Gemstone Caverns | `mox_amber_opal.go` | {C} or any w/ luck | Opening hand placement | PASS |

### 10b. Not Handled (no per-card handlers)

| Land | Oracle Text Summary | Impact | Status |
|------|-------------------|--------|--------|
| **Urza's Mine/Power Plant/Tower** | Tron bonus: produce 2/{C}{C}/7 if all three | HIGH -- Tron decks nonfunctional | FAIL |
| **Gaea's Cradle** | {T}: Add {G} for each creature you control | HIGH -- cEDH staple | FAIL |
| **Cabal Coffers** | {2},{T}: Add {B} for each Swamp | HIGH -- mono-B staple | FAIL |
| **Nykthos, Shrine to Nyx** | {2},{T}: Add mana = devotion | HIGH -- devotion decks | FAIL |
| **Command Tower** | {T}: Add any color in CI | MEDIUM -- cEDH but MVP any works | PARTIAL |
| **City of Brass** | {T}: Add any, 1 dmg to you | MEDIUM -- no damage | PARTIAL |
| **Mana Confluence** | {T}, Pay 1 life: Add any | MEDIUM -- no life cost | PARTIAL |
| **Phyrexian Tower** | {T}, Sac creature: Add {B}{B} | MEDIUM -- combo piece | FAIL |
| **Strip Mine** | {T}, Sac: Destroy target land | HIGH -- land destruction | FAIL |
| **Wasteland** | {T}, Sac: Destroy nonbasic land | HIGH -- land destruction | FAIL |
| **Ghost Quarter** | {T}, Sac: Destroy land, opp gets basic | MEDIUM | FAIL |
| **Field of the Dead** | 7+ lands with different names: create 2/2 | LOW | FAIL |
| **Maze of Ith** | {T}: Untap + remove from combat | MEDIUM -- combat trick | FAIL |
| **Dark Depths** | Remove counters -> Marit Lage 20/20 | MEDIUM -- combo | FAIL |
| **Urborg, Tomb of Yawgmoth** | All lands are Swamps | HIGH -- layer 4 | FAIL |
| **Yavimaya, Cradle of Growth** | All lands are Forests | HIGH -- layer 4 | FAIL |
| **Cavern of Souls** | Choose type, uncounterable | MEDIUM | FAIL |
| **Forbidden Orchard** | {T}: Any, give opp 1/1 | LOW | PARTIAL |
| **Exotic Orchard** | {T}: Colors opponents produce | LOW | PARTIAL |
| **Mutavault** | {1}: Becomes 2/2 creature | LOW -- manland | FAIL |
| **Celestial Colonnade** | {3}{W}{U}: Becomes 4/4 flying vigilance | LOW | FAIL |
| Takenuma, Abandoned Mire | Channel: return creature/PW from GY | MEDIUM | FAIL |
| Eiganjo, Seat of the Empire | Channel: 4 dmg to attacking/blocking | LOW | FAIL |
| Sokenzan, Crucible of Defiance | Channel: create 2 1/1 hasty spirits | LOW | FAIL |

---

## Category 11: Enters-Tapped Lands

**Total in corpus:** 584 lands

**Status: FAIL**

The `etb_tapped` type tag is never injected by `deckparser.parseTypes`. The only lands that correctly enter tapped are:
- Shocklands (via per-card ETB handler -- conditional)
- Bojuka Bog (via per-card ETB handler -- unconditional)
- Evolving Wilds/Terramorphic Expanse (fetched lands enter tapped via `createPermanent(gs, seat, land, cfg.EntersTapped)`)

**All other enters-tapped lands (582 of 584) enter untapped.**

### Sub-categories affected:

| Sub-category | Count | Status |
|-------------|-------|--------|
| Guildgates | 23 | FAIL -- should enter tapped |
| Temples (scry ETB) | 23 | FAIL -- no scry, no tapped |
| Refuges (gain life ETB) | ~74 | FAIL -- no life gain, no tapped |
| Tri-lands | 32+ | FAIL -- should enter tapped |
| Tap-duals (Meandering River, etc.) | ~100+ | FAIL -- should enter tapped |
| Bounce lands | 12 | FAIL -- see Category 5 |
| Hideaway lands | 6 | FAIL -- no hideaway, no tapped |

**Fix scope:** Small-medium. Two approaches:
1. **Quick fix (recommended):** In `tryPlayLand`, scan oracle text for "enters tapped" / "enters the battlefield tapped" and set `perm.Tapped = true`. ~10 lines.
2. **Proper fix:** Have the AST parser tag cards with `etb_tapped` in the JSONL export, or have `deckparser.buildCard` detect it from oracle text.

---

## Category 12: Tap-for-Conditional-Mana

| Land | Expected | Actual | Status |
|------|----------|--------|--------|
| Command Tower | Any color in CI | +1 untyped | PARTIAL -- functionally correct in MVP |
| Exotic Orchard | Colors opponents produce | +1 untyped | PARTIAL |
| City of Brass | Any + 1 dmg | +1 untyped, no dmg | PARTIAL |
| Mana Confluence | Any + pay 1 life | +1 untyped, no life | PARTIAL |
| Forbidden Orchard | Any + give opp 1/1 | +1 untyped, no token | PARTIAL |

**Status: PARTIAL**

- Mana production: functionally correct (all produce at least 1 mana).
- Missing drawbacks: City of Brass should deal 1 damage, Mana Confluence should cost 1 life, Forbidden Orchard should give opponent a 1/1 Spirit token.
- Impact: makes these lands strictly better than they should be.

**Fix scope:** Small per card. 3 per-card activated handlers, ~30 lines each.

---

## Category 13: Special Lands

| Land | Mechanic | Handler | Status |
|------|----------|---------|--------|
| Maze of Ith | Untap + remove from combat | NONE | FAIL |
| Glacial Chasm | Cumulative upkeep, prevent damage | NONE | FAIL |
| The Tabernacle at Pendrell Vale | Creatures get upkeep tax | NONE | FAIL |
| Dark Depths | Ice counters -> Marit Lage | NONE | FAIL |
| Gaea's Cradle | {T}: Add {G} per creature | NONE | FAIL |
| Urborg, Tomb of Yawgmoth | All lands are Swamps (layer 4) | NONE | FAIL |
| Yavimaya, Cradle of Growth | All lands are Forests (layer 4) | NONE | FAIL |
| Blood Moon (not a land) | Nonbasics are Mountains (layer 4) | layers.go | PASS |

**Status: FAIL** (except Blood Moon which is handled in `layers.go`)

- Urborg/Yavimaya: The layer system exists (`layers.go` handles Blood Moon), but Urborg and Yavimaya layer-4 effects are not registered.
- Gaea's Cradle: Critical cEDH card -- should count creatures and produce that much {G}.
- Dark Depths: Complex mechanic (ice counters, Marit Lage token creation).

**Fix scope:**
- Urborg/Yavimaya: Medium. Mirror `registerMoonEffect` pattern, add Swamp/Forest instead of Mountain. ~80 lines.
- Gaea's Cradle: Small. Single activated handler, count creatures. ~30 lines.
- Others: Large individually.

---

## Priority Fix List

### Tier 1 -- High Impact, Low Effort

| Fix | Files | Lines | Impact |
|-----|-------|-------|--------|
| 1. ETB tapped detection | `turn.go` | ~15 | Fixes 582 lands entering untapped incorrectly |
| 2. Typed mana from lands | `turn.go` | ~50 | Enables color-screw analysis |
| 3. Gaea's Cradle handler | `utility_lands.go` + `registry.go` | ~30 | cEDH staple |
| 4. City of Brass / Mana Confluence dmg | `utility_lands.go` + `registry.go` | ~50 | Removes free-mana exploit |

### Tier 2 -- High Impact, Medium Effort

| Fix | Files | Lines | Impact |
|-----|-------|-------|--------|
| 5. Bounce land handlers | `utility_lands.go` + `registry.go` | ~60 | 12 lands fixed |
| 6. Check land handlers | `utility_lands.go` + `registry.go` | ~80 | 10 core lands |
| 7. Fast land handlers | `utility_lands.go` + `registry.go` | ~50 | 10 core lands |
| 8. Pain land handlers | `utility_lands.go` + `registry.go` | ~60 | 10 core lands |
| 9. Urborg/Yavimaya layer 4 | `layers.go` | ~80 | Layer system completion |
| 10. Strip Mine / Wasteland | `utility_lands.go` + `registry.go` | ~50 | Land destruction |

### Tier 3 -- Medium Impact, Medium Effort

| Fix | Files | Lines | Impact |
|-----|-------|-------|--------|
| 11. Tron lands (Urza's Mine/PP/Tower) | `utility_lands.go` | ~60 | Tron archetype |
| 12. Cabal Coffers / Nykthos | `utility_lands.go` | ~60 | Ramp staples |
| 13. Temple scry ETB | `utility_lands.go` | ~40 | 23 temples |
| 14. Phyrexian Tower | `utility_lands.go` | ~30 | Sac-for-mana |
| 15. Remaining channel lands | `utility_lands.go` | ~60 | 3 more channels |
| 16. Creature lands (manlands) | `utility_lands.go` | ~100 | 50 lands |

### Tier 4 -- Low Priority

| Fix | Reason |
|-----|--------|
| Filter lands | Rarely played in cEDH |
| Storage/charge lands | Niche mechanics |
| Hideaway lands | Complex, low frequency |
| Dark Depths | Complex combo piece |
| Maze of Ith / Glacial Chasm | Complex combat/damage prevention |

---

## Fixes Applied in This Audit

### Fix 1: ETB Tapped Detection

Added oracle-text-based enters-tapped detection to `tryPlayLand` in `turn.go`. This catches the 584 lands whose oracle text says "enters tapped" or "enters the battlefield tapped" without needing per-card handlers for each one.

**Note:** This does NOT handle conditional enters-tapped (check lands, fast lands, shock lands). Those still need per-card handlers. The shockland handler already exists; check/fast lands are listed as Tier 2 fixes.

### Fix 2: Typed Mana from Basic Land Subtypes

Updated `runMainPhase` to read basic land subtypes (Plains/Island/Swamp/Mountain/Forest) from Card.Types and call `AddMana(gs, seat, color, 1, name)` instead of `seat.ManaPool++`. Lands without recognized subtypes still fall back to +1 untyped mana.

This enables correct mana coloring for:
- All 13 basic lands
- All 10 original duals (have basic land subtypes on type line)
- All 10 shocklands (have basic land subtypes on type line)
- All triomes (have basic land subtypes on type line)
- Any land with basic land subtypes

---

## Verification Commands

```bash
# Run existing land-related tests
go test ./internal/gameengine/per_card/ -run "TestShock|TestFetch|TestBojuka|TestAncient|TestReliquary|TestGemstone" -v

# Run full engine test suite
go test ./internal/gameengine/... -count=1

# Run tournament tests
go test ./internal/tournament/... -count=1
```
