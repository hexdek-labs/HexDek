package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type ArchetypeClassification struct {
	Primary           string
	PrimaryConfidence float64
	Secondary         string
	SecondaryDistance  float64
	Intent            string
	// Bracket is the rubber-stamp / declared bracket — the value the
	// deck identifies AS (precons claim B2, user-built decks claim
	// whatever the owner sets). At the ArchetypeClassification layer
	// this defaults to the measured value; DeckProfile overrides it for
	// known-declared sources (e.g. WotC precons under data/decks/wizards/
	// auto-stamp to B2).
	Bracket           int
	BracketLabel      string
	// MeasuredBracket is Freya's signal-computed bracket — the canonical
	// felt-power measurement (surfaced in the UI as "Estimated Bracket").
	// Derived from density / card-list signals. Diverges from Bracket
	// when a declared override is applied (e.g. precons that Freya
	// thinks play hotter than B2).
	MeasuredBracket      int
	MeasuredBracketLabel string
	GameChangerCount  int
	GameChangerCards  []string
	Signals           []string
	// BracketRationale documents how the bracket was derived: which
	// signals contributed (and how many points), what cards backed each
	// scoring tier, and which ceilings/floors/gates fired to adjust the
	// raw score. Surfaces the WHY behind the bracket call so deck
	// builders can see exactly which density / card-list signals pushed
	// them across a threshold.
	BracketRationale *BracketRationale
}

// BracketSignal records a single contributing observation in the bracket
// calculation. Kind distinguishes scoring contributions from post-score
// adjustments (ceiling caps, floor lifts, B5 gate demotions) — for the
// latter, Contribution is 0 and the explanation lives in Note.
type BracketSignal struct {
	Name         string   // signal label ("Game Changers", "Free interaction", "B5 gate")
	Kind         string   // "score" | "ceiling" | "floor" | "gate"
	Tier         string   // matched threshold band ("8+", "12%+", "lean curve <2.0")
	Measurement  string   // actual measurement ("10 pieces", "13% of nonlands")
	Evidence     []string // card names backing the signal, where curated lists exist
	Contribution int      // points added (positive) or subtracted (negative); 0 for adjustments
	Note         string   // free-form why-line, used heavily by adjustment kinds
}

// BracketRationale is the assembled explanation for a single bracket
// call. RawScore is the sum of Contribution across Kind="score"
// signals before any adjustments fire. FinalBracket / FinalLabel are
// the post-adjustment outcome.
type BracketRationale struct {
	FinalBracket int
	FinalLabel   string
	RawScore     int
	Signals      []BracketSignal
}

type archetypeFingerprint struct {
	Name    string
	Ratios  map[RoleTag]float64
	Require func(ctx *classifyContext) bool
}

type classifyContext struct {
	roleRatios     map[RoleTag]float64
	avgCMC         float64
	comboCount     int
	tutorDensity   float64
	fastManaCount  int
	// tappedManaCount holds CMC-≤2 mana sources that ETB tapped
	// (Coldsteel Heart, Diamond cycle, Star Compass, Coalition Relic).
	// They're real ramp but a turn slower than untapped rocks of the same
	// CMC, so they're excluded from fastManaCount (which feeds the B4
	// bracket signal) and surfaced separately in the rationale. See
	// isTappedETBRock + the Fast mana note in estimateMeasuredBracket.
	tappedManaCount int
	tappedManaNames []string
	// creatureCount is the raw qty-weighted count of creatures in the
	// deck (paired with creaturePct as its proportional companion).
	// Drives the absolute-count Control detection arm where a
	// creature-light deck (<15 creatures) loaded with interaction
	// reliably signals the Control archetype shape regardless of the
	// proportional role-ratio gate.
	creatureCount int
	// interactionCount sums the qty-weighted role counts for Removal +
	// BoardWipe + Counterspell — the three "stop opponents from
	// winning" role buckets. Distinct from the per-role roleRatios
	// (proportional, sums to 1.0 across all roles) because the
	// absolute count is the signal: a deck with 15+ interaction
	// cards is meaningfully Control-shaped even when the proportional
	// gate is diluted by a deep draw/ramp package pushing each
	// individual role ratio down.
	interactionCount int
	instantSorcPct float64
	// instantSorceryCount is the raw count of instant + sorcery cards
	// (qty-weighted) in the deck. Paired with spellTriggerPermanentCount
	// to drive the absolute-count Spellslinger detection arm — a 60%
	// density gate misses I/S-heavy hybrid builds (Niv-Mizzet Reborn,
	// Veyran, mid-density Storm) where ramp + creature payoffs pull the
	// percentage below 60% but the spell-trigger payoff cluster makes
	// the deck unmistakably Spellslinger-shaped.
	instantSorceryCount int
	// spellTriggerPermanentCount counts non-instant-sorcery permanents
	// whose oracle text triggers on spell casts: "whenever you cast a
	// spell" (Aetherflux Reservoir, Sentinel Tower), "whenever you cast
	// an instant or sorcery" (Guttersnipe, Talrand, Young Pyromancer,
	// Murmuring Mystic, Archmage Emeritus), "magecraft" (Strixhaven
	// family), Stormwing-style ETB-cast checks ("if you've cast another
	// instant or sorcery"), and per-cast scaling payoffs ("for each
	// spell you've cast"). Distinct from spellCopyCount which conflates
	// copy-effect engines (Thousand-Year Storm, Pyromancer's Goggles)
	// with cast-trigger creatures — this field is the cleaner "do we
	// have a spell-trigger payoff cluster" signal.
	spellTriggerPermanentCount int
	creaturePct    float64
	topCreatureTypePct float64
	sacrificeCount int
	deathTriggers  int
	graveyardCount int
	selfMillCount  int
	// discardOutletCount: cards that ENABLE discard as a cost or
	// trigger — Faithless Looting / Wild Mongrel / Putrid Imp /
	// Bone Miser / Liliana of the Veil / Burning Inquiry. Distinct
	// from random opponent-discard cards (Mind Rot, Hymn to Tourach)
	// because those don't fill YOUR graveyard. Paired with
	// selfMillCount as the "fill your graveyard" combined signal.
	discardOutletCount int
	// reanimationCount: cards that put a creature card from a
	// graveyard onto the battlefield — the LOAD-BEARING signal that
	// distinguishes Reanimator (cheat fatties from grave) from
	// generic recursion (Eternal Witness returns to HAND, not
	// battlefield). Detected via curated names AND oracle-text
	// pattern + the existing IsRecursion + RecursionDest=battlefield
	// classification.
	reanimationCount int
	// graveyardSizePayoffCount: cards that scale a measured value with
	// the size or contents of YOUR graveyard. Distinct from
	// graveyardCount (which conflates recursion + self-mill enablers)
	// and selfMillCount (which only counts mill/dredge/surveil
	// enablers). Drives the Selfmill fingerprint Require gate — a
	// deck with 8 self-mill enablers but zero scaling payoffs (a pure
	// "fill your graveyard then do nothing with it" deck) isn't
	// Selfmill, it's confused. Examples: Splinterfright / Lhurgoyf /
	// Mortivore (oracle "X equal to ... in your graveyard"), Jarad /
	// Sutured Ghoul (creature stat scaling), Bonehoard (equipment
	// scaling), Genesis Wave / Crucible-class graveyard-replay
	// engines.
	graveyardSizePayoffCount int
	equipAuraCount int
	// equipmentCount and auraCount split equipAuraCount by subtype so
	// Equipment-Voltron and Aura-Voltron can be distinguished from
	// generic suit-up Voltron. The sub-archetypes diverge sharply in
	// gameplay shape (Equipment carries an extra deck-building tax for
	// equip costs but survives bounce, while Aura-Voltron is faster but
	// vulnerable to single-target removal) so the strategy bridge needs
	// the distinction to weight protection vs board-wipe risk.
	equipmentCount int
	auraCount      int
	// equipTriggerPayoffCount tracks cards that BOOST the equipment
	// engine itself — Puresteel Paladin's "Whenever an Equipment enters,
	// draw a card", Sigarda's Aid's "Auras and Equipment you control
	// have flash + attach on ETB", Sram's "Whenever you cast an Aura,
	// Equipment, or Vehicle, draw a card", Stoneforge Mystic's tutor +
	// free-cast trigger. A pile of 10 equipment with zero payoffs is a
	// midrange-with-toolbox shape; 8 equipment + 3 payoffs is the
	// committed Equipment-Voltron archetype.
	equipTriggerPayoffCount int
	spellCopyCount int
	landfallCount  int
	counterCount   int // +1/+1 counter / proliferate cards
	// proliferateCount counts cards that specifically PROLIFERATE — a
	// narrower signal than counterCount (which also includes +1/+1
	// counter anthems and "number of counters" payoffs). Drives the
	// Atraxa-style Superfriends detection arm where a 4-6 planeswalker
	// shell leans on proliferate engines (Atraxa Praetors' Voice as
	// commander, Karn's Bastion, Inexorable Tide, Tezzeret's Gambit,
	// Contagion Engine / Clasp, Flux Channeler, Evolution Sage,
	// Plaguemaw Beast) to make even a smaller walker cluster
	// threatening. Distinguishing this from counters-matter (Hardened
	// Scales / Marchesa) requires gating on BOTH proliferate density
	// AND a planeswalker floor — counters-matter decks pack lots of
	// +1/+1 payoffs but typically run 0-2 planeswalkers.
	proliferateCount int
	enchantmentPct float64
	lifegainCount  int
	blinkCount     int
	// etbValueCreatureCount counts CREATURE cards whose ETB produces
	// something worth blinking for (HasValueETB && type-line contains
	// "creature"). The Blink/Flicker archetype fingerprint requires
	// this in addition to a meaningful blink-effect count — a control
	// deck happens to run Ghostly Flicker + Cyclonic Rift, but isn't
	// a Blink deck unless it ALSO packs enough Mulldrifter / Reclamation
	// Sage / Eternal Witness-class ETB payoffs to justify the engine.
	etbValueCreatureCount int
	artifactCount  int
	extraCombatCount int
	// extraTurnCount counts cards that grant a literal extra TURN
	// ("take an extra/another/additional turn after this one") — Time
	// Walk / Time Warp / Nexus of Fate / Sage of Hours / Beacon of
	// Tomorrows family. Distinct from extraCombatCount (Aggravated
	// Assault / Hellkite Charger grant an additional COMBAT PHASE,
	// not a turn). Adventure-style multi-face cards are deduped at the
	// CARD level so a single Adventure with extra-turn text on both
	// halves counts as 1, not 2. Drives a B4+ bracket-lift signal
	// because a deck packing 4+ extra-turn spells is an extra-turns
	// archetype — the WotC framework treats repeatable extra-turn
	// generation as a B4 marker (chains 3-4 turns in a row, sets up
	// uncontestable wins).
	extraTurnCount int
	planeswalkerCount int
	millOppCount   int // opponent-targeting mill
	discardForceCount int
	// R60 new-archetype counters
	pillowfortCount     int // attack-tax / damage-prevention cards (Propaganda, Sphere of Safety, Solitary Confinement)
	groupSlugCount      int // passive damage-to-opponents triggers (Manabarbs, Pyrostatic Pillar, Underworld Dreams)
	damageRedirectCount int // "dealt damage, it deals" reflectors + redirect effects (Stuffy Doll, Boros Reckoner, Pariah)
	// R60 (post-precon-corpus-audit) — 4 new archetype counters surfaced
	// by docs/precon-shape-scans/group-{a,b,c}.md where stock precons
	// fell through to Midrange/Artifacts because no fingerprint matched.
	groupHugCount    int // "each player draws/gains/searches", Phelddagrif/Kynaios shells, Howling Mine cluster
	cyclingCount     int // cards with the cycling keyword cost ("cycling {")
	cyclingPayoffCount int // cards that trigger on cycling (Astral Drift, Drake Haven, New Perspectives, Fluctuator)
	toxicInfectCount int // "infect", "toxic N", "poison counter" — distinct from Counters Matter's +1/+1 axis
	vehicleCount     int // Vehicle (and Spacecraft) typeline + crew-payoff cards
	// R60 Tokens archetype: distinct from generic "Aggro / Go Wide"
	// because the structural signature is the SPECIFIC pairing of
	// token-creation density + anthem-stacking density. A Krenko /
	// Adeline / Rhys the Redeemed / Ghave-tokens shell wants the
	// MCTS evaluator to weight BoardPresence heavily and to treat
	// board wipes as catastrophic (HIGH ThreatExposure) — different
	// from raw Aggro where any creature-pressure plan triggers the
	// fingerprint regardless of token-vs-permanent shape.
	//
	// tokenCreatorCount: cards whose oracle text creates tokens via
	// any of the canonical phrasings (see tokenCreationPhrases).
	// Includes token-doubler replacements (Anointed Procession,
	// Parallel Lives, Doubling Season) — they CREATE tokens by
	// replacement so they count as creators for the structural
	// signal even though they don't generate the first token
	// themselves; a decklist with 8 token-creator cards that all
	// double-up is still a tokens deck.
	tokenCreatorCount int
	// anthemCount: cards that buff "creatures you control" or
	// "[subtype] creatures you control" (tribal anthems like Goblin
	// King apply since they buff every token of the matching tribe).
	// Excludes single-target buffs ("target creature gets +1/+1")
	// because those don't scale with board-wide token presence.
	anthemCount int
	// tribalLordCount: subset of anthemCount that names a specific
	// creature subtype ("Goblin creatures you control get +1/+1",
	// "Other Elves you control get +1/+1"). A pile of generic-anthem
	// "creatures you control get +1/+1" doesn't signal tribal — it
	// signals go-wide or aggro. But 2+ cards whose buff is gated on a
	// SHARED CREATURE TYPE is the structural signature of a tribal
	// deck even when the deck's top-creature-type concentration is
	// only marginally above noise (a deck running 18 elves + 6 utility
	// non-elf creatures may sit at ~75% elf in the creature slot but
	// the lord package is the load-bearing signal regardless).
	//
	// tribalLordTribe records the most-mentioned tribe across the
	// detected lords, so the buildSignals output can name it. Empty
	// when tribalLordCount < 1.
	tribalLordCount int
	tribalLordTribe string
	bannedCount      int
	gameChangerCount int
	gameChangerNames []string
	// freeInteractionCount tracks pitch-counter / phyrexian-mana / pact /
	// commander-free / evoke spells (see cedhFreeInteractionList). The
	// strongest single-deck-shape signal that separates true cEDH (B5)
	// from merely-optimized B4.
	freeInteractionCount int
	freeInteractionNames []string
	profiles       []CardProfile
	qtyProfiles    []CardProfileQty
	oracle         *oracleDB
}

// cEDHFreeInteractionList tracks the cEDH-defining "free" interaction
// suite — spells castable for {0} via alternative casting cost (pitch a
// blue card, return a land, commander on battlefield, evoke, phyrexian
// life). The presence of multiple free-interaction pieces is the
// strongest deck-shape signal that separates true cEDH (B5) from
// merely-optimized B4 decks. B4 players use Counterspell at 2 mana;
// B5 players hold up Force of Will / Fierce Guardianship / Mental
// Misstep so the counter doesn't trade their tempo against winning
// faster. We intentionally do NOT include "cheap" interaction
// (Counterspell, Swan Song, An Offer You Can't Refuse) — those are
// strong B4 interaction but not the discriminating B5 signal.
var cedhFreeInteractionList = map[string]bool{
	// Pitch counters (exile a card of matching color)
	"force of will": true, "force of negation": true, "force of vigor": true,
	"force of despair": true, "force of rage": true, "force of virtue": true,
	"misdirection": true, "foil": true, "commandeer": true,
	"disrupting shoal": true, "thwart": true, "daze": true,
	// Phyrexian-mana free spells
	"mental misstep": true, "gut shot": true, "snuff out": true,
	// Pact cycle (free now, pay next turn)
	"pact of negation": true, "slaughter pact": true, "intervention pact": true,
	"summoner's pact": true, "pact of the titan": true,
	// Commander-on-battlefield free spells (cEDH partner-rich decks lean on these)
	"fierce guardianship": true, "deflecting swat": true, "deadly rollick": true,
	"fierce retribution": true, "obscuring haze": true, "tribute to the world tree": true,
	// Evoke elementals (free with tempo cost)
	"subtlety": true, "endurance": true, "solitude": true, "grief": true, "fury": true,
	// Other 0-mana / free alternative-cost interaction
	"snapback": true, "unmask": true, "chain of vapor": true,
}

// tappedETBRocks is the curated set of CMC-≤2 artifact ramp pieces that
// enter the battlefield tapped, so they don't tap-for-mana on the turn
// they're cast. They're a turn slower than untapped CMC-equivalent
// rocks (Sol Ring, Arcane Signet, signet/talisman cycles), which is the
// material difference for the B4 bracket signal — "T1 fast rock → T2
// 4-drop wincon" tempo doesn't exist with these. Used as a backup
// against the oracle-text check in isTappedETBRock so detection still
// fires on cards whose oracle data is missing from the local cache.
var tappedETBRocks = map[string]bool{
	"coldsteel heart":  true,
	"charcoal diamond": true,
	"marble diamond":   true,
	"sky diamond":      true,
	"fire diamond":     true,
	"moss diamond":     true,
	"star compass":     true,
	"guardian idol":    true,
}

// isTappedETBRock reports whether a CMC-≤2 mana producer enters tapped
// and should therefore be excluded from fastManaCount. Two-pronged
// detection: the curated tappedETBRocks set handles cards whose oracle
// text isn't available (offline classify runs); the oracle-text scan
// catches anything else printed with the universal "enters tapped" /
// "enters the battlefield tapped" phrasing. Matches the lowercased
// name + lowercased oracle text the caller already has.
func isTappedETBRock(name, ot string) bool {
	if tappedETBRocks[strings.ToLower(name)] {
		return true
	}
	// Scryfall's modern simplified oracle uses "enters tapped"; legacy
	// printings use "enters the battlefield tapped". Substring match
	// covers both since the shorter form is a strict prefix of the
	// longer one when split by the same anchor word "enters".
	if strings.Contains(ot, "enters tapped") ||
		strings.Contains(ot, "enters the battlefield tapped") {
		return true
	}
	return false
}

// WotC Commander Game Changers list (53 cards, Feb 2026 update).
// Presence of these cards is the primary WotC bracket classification signal.
var gameChangersList = map[string]bool{
	// White
	"drannith magistrate": true, "enlightened tutor": true, "farewell": true,
	"humility": true, "teferi's protection": true, "smothering tithe": true,
	// Blue
	"consecrated sphinx": true, "cyclonic rift": true, "force of will": true,
	"fierce guardianship": true, "gifts ungiven": true, "intuition": true,
	"mystical tutor": true, "narset, parter of veils": true, "rhystic study": true,
	"thassa's oracle": true,
	// Black
	"ad nauseam": true, "bolas's citadel": true, "braids, cabal minion": true,
	"demonic tutor": true, "imperial seal": true, "necropotence": true,
	"opposition agent": true, "orcish bowmasters": true,
	"tergrid, god of fright": true, "vampiric tutor": true,
	// Red
	"gamble": true, "jeska's will": true, "underworld breach": true,
	// Green
	"biorhythm": true, "crop rotation": true, "natural order": true,
	"seedborn muse": true, "survival of the fittest": true, "worldly tutor": true,
	// Multicolor
	"aura shards": true, "coalition victory": true,
	"grand arbiter augustin iv": true, "notion thief": true,
	// Colorless
	"ancient tomb": true, "chrome mox": true, "field of the dead": true,
	"gaea's cradle": true, "glacial chasm": true, "grim monolith": true,
	"lion's eye diamond": true, "mana vault": true, "mishra's workshop": true,
	"mox diamond": true, "panoptic mirror": true, "serra's sanctum": true,
	"the one ring": true, "the tabernacle at pendrell vale": true,
}

var archetypeFingerprints = []archetypeFingerprint{
	{
		Name: "Combo",
		Ratios: map[RoleTag]float64{
			RoleCombo: 0.06, RoleTutor: 0.10, RoleDraw: 0.12, RoleRamp: 0.10,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.comboCount >= 2 && ctx.tutorDensity >= 0.04
		},
	},
	{
		Name: "Stax",
		Ratios: map[RoleTag]float64{
			RoleStax: 0.10, RoleRemoval: 0.08, RoleThreat: 0.08, RoleDraw: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.roleRatios[RoleStax] >= 0.06
		},
	},
	{
		Name: "Control",
		Ratios: map[RoleTag]float64{
			RoleRemoval: 0.15, RoleDraw: 0.14, RoleCounterspell: 0.08, RoleThreat: 0.06,
			RoleBoardWipe: 0.04, RoleRamp: 0.08,
		},
		// Two-arm detection:
		//   (1) Proportional gate: interaction roles (Removal +
		//       BoardWipe + Counterspell) total ≥15% AND draw ≥10%.
		//       Catches the canonical UW / UBx control shape where
		//       the deck is structurally proportional-heavy on
		//       interaction.
		//   (2) Absolute-count gate: ≥15 interaction cards AND <15
		//       creatures. Picks up creature-light Talrand /
		//       Baral / Kess / Sen Triplets-style control builds where
		//       a deep draw + ramp package dilutes each individual
		//       role ratio below 15% even though the deck packs
		//       Counterspell + Swan Song + Negate + Path + Swords +
		//       Cyclonic Rift + Toxic Deluge + Wrath of God + …
		//       The <15 creature ceiling keeps generic Bant /
		//       Esper goodstuff midrange (which routinely runs 15
		//       interaction cards alongside 18+ creatures) from
		//       poaching into Control.
		Require: func(ctx *classifyContext) bool {
			if ctx.roleRatios[RoleRemoval]+ctx.roleRatios[RoleBoardWipe]+ctx.roleRatios[RoleCounterspell] >= 0.15 &&
				ctx.roleRatios[RoleDraw] >= 0.10 {
				return true
			}
			if ctx.interactionCount >= 15 && ctx.creatureCount < 15 {
				return true
			}
			return false
		},
	},
	{
		// R60: Equipment-Voltron sub-archetype. Distinct from generic
		// Voltron in gameplay shape:
		//   - Equipment carries an extra deck-building tax (equip
		//     cost mana per turn) but survives single-target bounce
		//     since the Equipment stays on the battlefield.
		//   - The engine REQUIRES at least one equip-payoff piece
		//     (Puresteel Paladin / Sigarda's Aid / Stoneforge Mystic
		//     / Sram / Halvar) to be tractable; without payoffs the
		//     equip-cost tax stalls the clock past commander-tax
		//     escalation. 3+ payoffs is the load-bearing signal.
		//
		// Gates: 8+ Equipment cards AND 3+ equip-trigger payoffs.
		// Falls through to generic Voltron if either gate fails.
		// Listed BEFORE generic Voltron in the fingerprint slice so
		// the Euclidean-distance sort sees both candidates and the
		// sub-archetype's tighter template (RoleArtifact-heavy) edges
		// it ahead when the deck shape matches.
		Name: "Equipment-Voltron",
		Ratios: map[RoleTag]float64{
			RoleProtection: 0.12, RoleThreat: 0.10, RoleRamp: 0.10, RoleRemoval: 0.05,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.equipmentCount >= 8 &&
				ctx.equipTriggerPayoffCount >= 3 &&
				ctx.roleRatios[RoleProtection] >= 0.06
		},
	},
	{
		Name: "Voltron",
		Ratios: map[RoleTag]float64{
			RoleProtection: 0.12, RoleThreat: 0.10, RoleRamp: 0.10, RoleRemoval: 0.05,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.equipAuraCount >= 8 && ctx.roleRatios[RoleProtection] >= 0.06
		},
	},
	{
		// R60-13: broadened the Aristocrats fingerprint after a tournament
		// pass surfaced sac-themed decks landing on the Midrange fallback.
		// The original AND (sacOutlets≥5 AND deathTriggers≥3) caught the
		// canonical Sidisi / Meren / Korvold builds but missed:
		//   - token-flood + drain decks with light sac outlets (Elenda the
		//     Dusk Rose, Teysa Karlov, Marchesa the Black Rose) where the
		//     payoff cards outnumber the outlets. The deck still wants the
		//     Aristocrats weight profile (BoardPresence + DrainEngine), but
		//     sacrificeCount=2-3 with 6+ death triggers fell out of the AND.
		//   - persist-recursion shells (Reassembling Skeleton / Bloodghast /
		//     Gravecrawler) where the GY bodies ARE the sac fuel and the
		//     strict outlet count missed them.
		// The disjunction below keeps the strict shape as the strongest
		// signal, then adds two looser shapes that still require either
		// a meaningful drain payoff cluster OR a real graveyard presence
		// so it doesn't poach generic creature midrange decks.
		Name: "Aristocrats",
		// R60 (post-#469): RoleRecursion enters the Ratios at a
		// modest 0.06 — Aristocrats decks do run recursion (persist
		// creatures, Reassembling Skeleton, Bloodghast, Gravecrawler,
		// Phyrexian Reclamation, Victimize) but the engine center of
		// gravity is the sac outlet + death-trigger payoff cluster,
		// not the recursion density itself. 0.06 reflects ~6 of 99
		// cards with the Recursion tag in a Korvold/Meren-as-aristo/
		// Teysa shell.
		Ratios: map[RoleTag]float64{
			RoleThreat:    0.10,
			RoleCombo:     0.06,
			RoleDraw:      0.10,
			RoleRamp:      0.08,
			RoleRecursion: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			if ctx.sacrificeCount >= 5 && ctx.deathTriggers >= 3 {
				return true
			}
			// Drain-heavy / payoff-heavy shape: fewer outlets, more
			// death-trigger payoffs (Blood Artist / Zulaport Cutthroat /
			// Cruel Celebrant / Bastion of Remembrance / Marionette Master).
			if ctx.sacrificeCount >= 2 && ctx.deathTriggers >= 6 {
				return true
			}
			// Persist / GY-recursion shape: bodies in the GY power the
			// sac engine (Reassembling Skeleton + Phyrexian Altar +
			// Pitiless Plunderer). R60 (post-#469): adds an explicit
			// RoleRecursion >= 0.04 requirement so this arm is
			// genuinely recursion-shape-aware — pre-r60 it accepted
			// any deck with sacrificeCount>=3 + deathTriggers>=3 +
			// graveyardCount>=4, where graveyardCount's conflation
			// with self_mill effects let pure-mill decks slip in.
			if ctx.sacrificeCount >= 3 && ctx.deathTriggers >= 3 &&
				ctx.graveyardCount >= 4 && ctx.roleRatios[RoleRecursion] >= 0.04 {
				return true
			}
			return false
		},
	},
	{
		Name: "Spellslinger",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.14, RoleRamp: 0.10, RoleCounterspell: 0.04, RoleThreat: 0.05,
		},
		// Two-arm detection:
		//   (1) Density gate: ≥60% instant/sorcery + at least one copy/
		//       cast-trigger effect. Catches the canonical mono-blue /
		//       Mizzix / Niv-Mizzet I/S-pile shape.
		//   (2) Absolute-count gate: ≥25 instant/sorcery cards AND ≥4
		//       spell-trigger payoff permanents. Picks up Niv-Mizzet
		//       Reborn / Veyran / Stormwing-style hybrids that pack a
		//       full I/S+payoff cluster but include enough ramp/creatures
		//       that the percentage falls below 60. The 25-card floor on
		//       I/S keeps generic Izzet midrange (which sits in the 15-22
		//       range) from poaching; the 4-permanent floor on the
		//       payoff cluster (Aetherflux Reservoir + Guttersnipe +
		//       Talrand + Young Pyromancer + Stormwing Entity + Archmage
		//       Emeritus + Murmuring Mystic and similar) ensures we're
		//       seeing the deliberate "I/S triggers payoffs" structure
		//       and not just an instant-heavy goodstuff pile.
		Require: func(ctx *classifyContext) bool {
			if ctx.instantSorcPct >= 0.60 && ctx.spellCopyCount >= 1 {
				return true
			}
			if ctx.instantSorceryCount >= 25 && ctx.spellTriggerPermanentCount >= 4 {
				return true
			}
			return false
		},
	},
	{
		Name: "Tribal",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.12, RoleDraw: 0.08, RoleRamp: 0.08, RoleRemoval: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			// Baseline gate — high creature density + a clear top tribe.
			if ctx.creaturePct >= 0.35 && ctx.topCreatureTypePct >= 0.30 {
				return true
			}
			// Lord-package carveout — 2+ tribal lords (cards whose
			// anthem is gated on a specific creature subtype) means
			// the deck is committed to a tribe even if the raw
			// type-concentration count is below the baseline gate.
			// Drops the topCreatureTypePct requirement to 0.20.
			if ctx.tribalLordCount >= 2 && ctx.creaturePct >= 0.30 && ctx.topCreatureTypePct >= 0.20 {
				return true
			}
			return false
		},
	},
	{
		// R60 (post-#466): RoleRecursion is now a first-class role-tag,
		// so the Reanimator fingerprint scores directly against the
		// recursion-role density rather than relying entirely on
		// bag-of-words graveyardCount + selfMillCount. The 0.12 target
		// reflects a healthy Meren/Muldrotha/Karador shell: ~12 of 99
		// cards carry the Recursion tag (Eternal Witness, Reanimate,
		// Animate Dead, Regrowth, Sun Titan, Karmic Guide, Bone Shards,
		// the commander itself when it has graveyard text, etc).
		// RoleThreat lowered 0.10→0.08 — reanimator threats are
		// concentrated in 4-6 big bombs that target-reanimate, not the
		// 10+ midrange threats a Voltron / Aggro shell carries.
		Name: "Reanimator",
		Ratios: map[RoleTag]float64{
			RoleRecursion: 0.12,
			RoleDraw:      0.10,
			RoleTutor:     0.08,
			RoleThreat:    0.08,
			RoleRamp:      0.08,
		},
		// Require gate has TWO qualifying branches:
		//
		// (1) Legacy gate (pre-r60): graveyardCount + selfMillCount
		//     + RoleRecursion floor. The graveyardCount counter is
		//     conflated (IsRecursion + self_mill effects + mass_
		//     reanimate effects), which over-fires on broad-graveyard
		//     value decks (Muldrotha goodstuff) but also under-fires
		//     on tight Animate-Dead-style shells.
		//
		// (2) Refined gate (r60 reanimator audit): the canonical
		//     "6+ self-mill/discard enablers + 4+ reanimation
		//     effects" shape. Reanimation effects are counted via
		//     curated name list (Animate Dead, Reanimate, Necromancy,
		//     Persist creatures, etc.) + RecursionDest=="battlefield"
		//     classification + oracle-text pattern scan. Discard
		//     outlets supplement self-mill so Faithless Looting /
		//     Wild Mongrel / Putrid Imp shells aren't penalized for
		//     filling the graveyard via discard rather than mill.
		//
		// EITHER branch qualifies a deck as Reanimator. The refined
		// gate catches tight Animate-Dead / Reanimate / Unburial Rites
		// shells that the legacy graveyardCount gate underestimates;
		// the legacy gate catches broader Muldrotha / Karador value
		// piles whose reanimation count is below 4 but graveyard
		// presence is otherwise unmistakable.
		Require: func(ctx *classifyContext) bool {
			legacy := ctx.graveyardCount >= 6 &&
				ctx.selfMillCount >= 2 &&
				ctx.roleRatios[RoleRecursion] >= 0.05
			// Refined branch intentionally OMITS the RoleRecursion
			// floor: the explicit count gates (6+ fill + 4+
			// reanimation) are already a tighter structural signal
			// than the role-density floor. A deck running only 4-5
			// reanimation cards in a 99-card list necessarily has
			// RoleRecursion below 5% but is still unmistakably
			// Reanimator-shaped (tight Animate-Dead shells often
			// look this way — small reanimation core + big targets).
			refined := (ctx.selfMillCount+ctx.discardOutletCount) >= 6 &&
				ctx.reanimationCount >= 4
			return legacy || refined
		},
	},
	{
		// R60 (post-#469): new Selfmill fingerprint. ArchetypeSelfmill
		// already exists as a downstream consumer tag in
		// internal/hat/poker.go (driving distinct MCTS weight profiles
		// + plan state machines vs Reanimator), but freya never
		// produced this classification — Sidisi-Brood-Tyrant /
		// Bruvac / Phenax / Splinterfright / Tasigur decks fell
		// through to Reanimator or Midrange. The new fingerprint
		// targets the "fill graveyard, scale a payoff" archetype:
		// heavy self-mill enablers + at least one graveyard-size
		// scaling payoff (Splinterfright's "X equal to creature
		// cards in your graveyard", Jarad's commander ability,
		// Sidisi's "create a zombie when you mill a creature"
		// embedded payoff via the canonical-commander match).
		//
		// Distinguishes from Reanimator via RoleRecursion target:
		// Selfmill runs SOME recursion (Eternal Witness, Splendid
		// Reclamation) but the engine center of gravity is the mill-
		// then-scale path, not the reanimate-then-attack path. 0.06
		// recursion target vs Reanimator's 0.12 means a deck with
		// ~5% recursion lands closer to Selfmill via Euclidean
		// distance, while a Meren-style 12% recursion deck stays in
		// Reanimator.
		Name: "Selfmill",
		Ratios: map[RoleTag]float64{
			RoleRecursion: 0.06,
			RoleDraw:      0.10,
			RoleRamp:      0.08,
			RoleThreat:    0.08,
		},
		Require: func(ctx *classifyContext) bool {
			// Strong self-mill enabler density AND at least one
			// graveyard-size scaling payoff. Without the payoff a
			// deck is just self-milling for value (which is the
			// Reanimator shape — graveyard is fuel for casting
			// from); WITH the payoff the graveyard size is a
			// resource being measured, which is the Selfmill shape.
			return ctx.selfMillCount >= 6 && ctx.graveyardSizePayoffCount >= 1
		},
	},
	{
		// R60 Tokens archetype — structural pairing of token-creation
		// density + anthem-stacking density. Distinct from generic
		// Aggro / Aggro / Go Wide because the Require gate is a specific
		// 8+ creators / 3+ anthems structural shape, not just "low CMC
		// with a lot of creatures." Canonical example shells: Krenko
		// Mob Boss (goblin tokens + Goblin King / Goblin Chieftain
		// anthems), Adeline Resplendent Cathar (human tokens + anthem
		// support), Rhys the Redeemed (elf tokens + Elvish Champion),
		// Ghave Guru of Spores (saproling tokens + anthem). RoleThreat
		// 0.22 nudges this slightly ahead of generic Aggro (0.20) so a
		// tokens-shape deck that ALSO trips Aggro's avgCMC<3 gate wins
		// the euclidean-distance tiebreak.
		Name: "Tokens",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.22, RoleRamp: 0.06, RoleDraw: 0.08, RoleRemoval: 0.05,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.tokenCreatorCount >= 8 && ctx.anthemCount >= 3
		},
	},
	{
		Name: "Aggro",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.20, RoleRamp: 0.10, RoleDraw: 0.06, RoleRemoval: 0.05,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.roleRatios[RoleThreat] >= 0.15 && ctx.avgCMC < 3.0
		},
	},
	{
		Name: "Lands Matter",
		Ratios: map[RoleTag]float64{
			RoleRamp: 0.15, RoleThreat: 0.10, RoleDraw: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.landfallCount >= 5
		},
	},
	{
		Name: "Enchantress",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.12, RoleThreat: 0.10, RoleRamp: 0.08, RoleProtection: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.enchantmentPct >= 0.30
		},
	},
	{
		Name: "Counters Matter",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.15, RoleDraw: 0.08, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.counterCount >= 8
		},
	},
	{
		Name: "Storm",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.14, RoleRamp: 0.12, RoleCombo: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.instantSorcPct >= 0.50 && ctx.spellCopyCount >= 3
		},
	},
	{
		Name: "Lifegain",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleDraw: 0.10, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.lifegainCount >= 8
		},
	},
	{
		Name: "Blink",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleDraw: 0.10, RoleRamp: 0.08, RoleRemoval: 0.06,
		},
		// r60 Blink gate — two-pronged: 5+ blink-effect cards
		// (Conjurer's Closet, Cloudshift, Ghostly Flicker, Eldrazi
		// Displacer, Restoration Angel, Brago, Aminatou, Yorion,
		// Deadeye Navigator, Thassa Deep-Dwelling, etc.) AND 8+
		// creatures whose ETB is worth re-triggering (Mulldrifter,
		// Reclamation Sage, Eternal Witness, Wood Elves, Sun Titan,
		// Cavalier of Gales, Solemn Simulacrum, ...). Pre-r60 the gate
		// was just blinkCount>=6, which over-classified control decks
		// that ran a handful of bounce/exile-and-return effects without
		// the ETB-payoff density that makes blink actually function as
		// a strategy.
		Require: func(ctx *classifyContext) bool {
			return ctx.blinkCount >= 5 && ctx.etbValueCreatureCount >= 8
		},
	},
	{
		Name: "Artifacts",
		Ratios: map[RoleTag]float64{
			RoleRamp: 0.12, RoleThreat: 0.10, RoleDraw: 0.10, RoleCombo: 0.04,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.artifactCount >= 20
		},
	},
	{
		Name: "Extra Combats",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.18, RoleRamp: 0.10, RoleDraw: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.extraCombatCount >= 3
		},
	},
	{
		Name: "Superfriends",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleRemoval: 0.08, RoleDraw: 0.08, RoleBoardWipe: 0.04,
		},
		// Two-arm detection:
		//   (1) PW-dense shell: ≥8 planeswalkers. Catches dedicated
		//       walker-tribal Estrid / Sliver Overlord-as-walker /
		//       cEDH Tezzeret stax-walker piles where the deck is
		//       structurally walker-heavy regardless of proliferate
		//       support.
		//   (2) Atraxa-style proliferate shell: ≥4 planeswalkers AND
		//       ≥3 proliferate effects. The proliferate cluster makes
		//       even a smaller 4-7 walker count threatening because
		//       every walker ticks up each upkeep / each combat /
		//       each spell cast. The 3-proliferate floor distinguishes
		//       a real Atraxa shell (Atraxa as commander + Karn's
		//       Bastion + Tezzeret's Gambit + Contagion Engine +
		//       Evolution Sage + Flux Channeler + Inexorable Tide)
		//       from generic counters-matter decks (Hardened Scales /
		//       Marchesa / Animar) that pack +1/+1 payoffs but few
		//       proliferate engines. The 4-walker floor distinguishes
		//       it from generic proliferate-matters decks (Skullbriar,
		//       Pir Imaginative Rascal, Ezuri Renegade Leader) that
		//       proliferate +1/+1 counters on creatures rather than
		//       loyalty.
		Require: func(ctx *classifyContext) bool {
			if ctx.planeswalkerCount >= 8 {
				return true
			}
			if ctx.planeswalkerCount >= 4 && ctx.proliferateCount >= 3 {
				return true
			}
			return false
		},
	},
	{
		Name: "Mill",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.12, RoleRemoval: 0.08, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.millOppCount >= 6
		},
	},
	{
		Name: "Discard",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleDraw: 0.10, RoleRemoval: 0.08, RoleStax: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.discardForceCount >= 6
		},
	},
	// ── R60: Three new archetype detectors ──
	{
		// Pillowfort: attack-tax + damage-prevention shell that wins
		// slowly while opponents are deflected elsewhere. Often runs
		// enchantments (Ghostly Prison, Sphere of Safety) so the
		// fingerprint can land alongside Enchantress — the classifier
		// picks the closer-distance match. Threshold of 5 pillowfort
		// pieces avoids false positives from incidental Ghostly Prison
		// inclusions in unrelated decks.
		Name: "Pillowfort",
		Ratios: map[RoleTag]float64{
			RoleProtection: 0.10, RoleRemoval: 0.10, RoleDraw: 0.10,
			RoleRamp: 0.08, RoleStax: 0.04, RoleThreat: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.pillowfortCount >= 5
		},
	},
	{
		// Group Slug: passive damage triggers (Manabarbs / Pyrostatic
		// Pillar / Underworld Dreams) that punish opponents for normal
		// actions. Distinct from Aristocrats (no sac engine) and
		// Spellslinger (no active spell-chain). Threshold of 5 to
		// require a real seed — incidental Eidolon of the Great Revel
		// in a burn deck shouldn't poach it.
		Name: "Group Slug",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.08, RoleRemoval: 0.10, RoleDraw: 0.10,
			RoleRamp: 0.08, RoleStax: 0.04,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.groupSlugCount >= 5
		},
	},
	{
		// Damage Redirect: Stuffy Doll / Boros Reckoner shell —
		// punisher creatures that reflect damage at opponents, often
		// supported by damage doublers (Furnace of Rath, Gisela).
		// Smaller archetype so the threshold is 4 (not 5); the cards
		// are distinctive enough that 4 pieces signals real intent.
		Name: "Damage Redirect",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleProtection: 0.08, RoleRemoval: 0.10,
			RoleDraw: 0.08, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.damageRedirectCount >= 4
		},
	},
	// ── R60 (post-precon-audit): four archetypes surfaced by
	// docs/precon-shape-scans/group-{a,b,c}.md where stock precons fell
	// through to Midrange/Artifacts because no fingerprint matched. ──
	{
		// Group Hug: gives cards/life/lands to all players, expects to
		// pivot to a win via Triskaidekaphile / Approach of the Second
		// Sun / Smothering Tithe-style asymmetric payoff. Distinct from
		// Pillowfort (no attack-tax shell required) and Group Slug
		// (gives, doesn't punish). Threshold 5 keeps a stray Howling
		// Mine in a draw-engine deck from poaching the classification.
		Name: "Group Hug",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.16, RoleRamp: 0.12, RoleProtection: 0.08,
			RoleRemoval: 0.06, RoleThreat: 0.04,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.groupHugCount >= 5
		},
	},
	{
		// Cycling: both cycling-cost density AND a real payoff are
		// required. A deck with 15 Lonely Sandbar-style cycling lands
		// but no Astral Drift / Drake Haven is just a midrange deck
		// with cycling utility lands, not the cycling-matters
		// archetype. Gates: >=2 payoff cards AND >=10 cycling-cost
		// cards. Gavi Nest Warden C20 hits ~3 payoffs + 18 cyclers.
		Name: "Cycling",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.16, RoleRamp: 0.08, RoleRemoval: 0.08,
			RoleThreat: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.cyclingPayoffCount >= 2 && ctx.cyclingCount >= 10
		},
	},
	{
		// Toxic / Infect: poison-axis win condition (CR 704.5c). The
		// poison counters are an alt-win in the same way Approach of
		// the Second Sun is for control. Distinct from Counters
		// Matter — those decks scale +1/+1 counters; this one drips
		// poison. Threshold 6 keeps a Triumph of the Hordes splash
		// from poaching a generic Naya tokens deck.
		Name: "Toxic",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.14, RoleRemoval: 0.08, RoleDraw: 0.08,
			RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.toxicInfectCount >= 6
		},
	},
	{
		// Vehicles: the Vehicle (or Spacecraft) typeline plus crew
		// payoffs. Kotori Buckle Up and Inspirit Counter Intelligence
		// both currently misclassify as Artifacts despite shipping
		// 10+ vehicles each; this fingerprint pulls them into the
		// right bucket. Threshold 6 — typical vehicle precon ships
		// 10-15 vehicles, so 6 is a comfortable floor while still
		// excluding decks with a few utility vehicles (Smuggler's
		// Copter splash).
		Name: "Vehicles",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.16, RoleRamp: 0.10, RoleDraw: 0.08,
			RoleRemoval: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.vehicleCount >= 6
		},
	},
	{
		Name: "Midrange",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.12, RoleRemoval: 0.10, RoleDraw: 0.10, RoleRamp: 0.10,
		},
		Require: nil,
	},
}

func ClassifyArchetype(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *ArchetypeClassification {
	if report.Roles == nil || report.Roles.TotalCards == 0 {
		return nil
	}

	ctx := buildClassifyContext(report, qtyProfiles, oracle)

	type scored struct {
		name     string
		distance float64
	}
	var results []scored

	for _, fp := range archetypeFingerprints {
		if fp.Require != nil && !fp.Require(ctx) {
			continue
		}
		d := euclideanDistance(ctx.roleRatios, fp.Ratios)
		// Tribal lord-package bonus: a deck running 2+ tribal lords
		// (cards whose anthem is gated on a specific creature
		// subtype) gets a 15% Tribal-distance discount. The shrink
		// edges Tribal ahead of neighbouring archetypes (Aggro,
		// Midrange, Counters Matter) when the role-ratio signal is
		// genuinely ambiguous — a lord-heavy elf-go-wide deck would
		// otherwise sit close to Aggro on the role axis but the lord
		// commitment is the load-bearing call.
		if fp.Name == "Tribal" && ctx.tribalLordCount >= 2 {
			d *= 0.85
		}
		// Equipment-Voltron sub-archetype tie-break: generic Voltron and
		// Equipment-Voltron share an identical role-ratio template, so
		// with both gates passing they would land at the same Euclidean
		// distance and the unstable sort.Slice ordering would
		// non-deterministically pick the winner. Apply a small 5%
		// discount to Equipment-Voltron when it qualifies so the more
		// specific sub-archetype wins deterministically.
		if fp.Name == "Equipment-Voltron" &&
			ctx.equipmentCount >= 8 && ctx.equipTriggerPayoffCount >= 3 {
			d *= 0.95
		}
		results = append(results, scored{name: fp.Name, distance: d})
	}

	if len(results) == 0 {
		results = append(results, scored{name: "Midrange", distance: 0.5})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].distance < results[j].distance
	})

	ac := &ArchetypeClassification{
		Primary: results[0].name,
	}

	if len(results) >= 2 {
		best := results[0].distance
		second := results[1].distance
		if second > 0 {
			ac.PrimaryConfidence = math.Max(0, math.Min(1, 1-(best/second)))
		} else {
			ac.PrimaryConfidence = 0
		}
		threshold := best * 1.25
		if best < 0.01 {
			threshold = 0.05
		}
		if second <= threshold {
			ac.Secondary = results[1].name
			ac.SecondaryDistance = second
		}
	} else {
		ac.PrimaryConfidence = 1.0
	}

	ac.Signals = buildSignals(ctx, ac)
	ac.Intent = buildIntent(ac, report, ctx)
	ac.MeasuredBracket, ac.MeasuredBracketLabel, ac.BracketRationale = estimateMeasuredBracket(ctx, report, ac.Primary)
	// At the ArchetypeClassification layer, Bracket defaults to the
	// measured value. DeckProfile.BuildDeckProfile applies any declared
	// override (e.g. wizards/ precons auto-stamp to B2).
	ac.Bracket, ac.BracketLabel = ac.MeasuredBracket, ac.MeasuredBracketLabel
	ac.GameChangerCount = ctx.gameChangerCount
	ac.GameChangerCards = ctx.gameChangerNames

	return ac
}

func buildClassifyContext(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *classifyContext {
	ra := report.Roles
	total := float64(ra.TotalCards)

	ctx := &classifyContext{
		roleRatios:  make(map[RoleTag]float64),
		avgCMC:      report.AvgCMC,
		comboCount:  len(report.TrueInfinites) + len(report.Determined),
		profiles:    report.Profiles,
		qtyProfiles: qtyProfiles,
		oracle:      oracle,
	}

	for _, role := range AllRoles {
		ctx.roleRatios[role] = float64(ra.RoleCounts[role]) / total
	}
	ctx.tutorDensity = ctx.roleRatios[RoleTutor]
	ctx.interactionCount = ra.RoleCounts[RoleRemoval] + ra.RoleCounts[RoleBoardWipe] + ra.RoleCounts[RoleCounterspell]

	nonlandTotal := 0
	instantSorcCount := 0
	creatureCount := 0
	enchantmentCount := 0
	creatureTypes := map[string]int{}
	tribeCounts := map[string]int{}

	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
		}
		nameLower := strings.ToLower(qp.Profile.Name)
		if commanderBannedList[nameLower] {
			ctx.bannedCount += qp.Qty
			continue
		}
		if gameChangersList[nameLower] {
			ctx.gameChangerCount += qp.Qty
			ctx.gameChangerNames = append(ctx.gameChangerNames, qp.Profile.Name)
		}
		if cedhFreeInteractionList[nameLower] {
			ctx.freeInteractionCount += qp.Qty
			ctx.freeInteractionNames = append(ctx.freeInteractionNames, qp.Profile.Name)
		}
		nonlandTotal += qp.Qty
		tl := strings.ToLower(qp.Profile.TypeLine)

		if strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery") {
			instantSorcCount += qp.Qty
		}
		if strings.Contains(tl, "creature") {
			creatureCount += qp.Qty
			for _, ct := range qp.Profile.CreatureTypes {
				creatureTypes[ct] += qp.Qty
			}
		}

		if strings.Contains(tl, "equipment") || strings.Contains(tl, "aura") {
			ctx.equipAuraCount += qp.Qty
		}
		// Subtype split for the Equipment-Voltron / Aura-Voltron
		// distinction. "Equipment" and "Aura" are subtypes of
		// artifact and enchantment respectively; the TypeLine
		// substring check is reliable because Scryfall normalizes
		// the printed line ("Artifact — Equipment" / "Enchantment —
		// Aura"). MDFC and split cards have their front-face
		// TypeLine here, which is the equip-relevant face anyway.
		if strings.Contains(tl, "equipment") {
			ctx.equipmentCount += qp.Qty
		}
		if strings.Contains(tl, "aura") {
			ctx.auraCount += qp.Qty
		}
		if strings.Contains(tl, "enchantment") {
			enchantmentCount += qp.Qty
		}
		if strings.Contains(tl, "artifact") {
			ctx.artifactCount += qp.Qty
		}
		if strings.Contains(tl, "planeswalker") {
			ctx.planeswalkerCount += qp.Qty
		}

		var ot string
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
				if ot == "" && len(entry.CardFaces) > 0 {
					ot = strings.ToLower(entry.CardFaces[0].OracleText)
				}
			}
		}

		if containsAny(ot,
			"copy target instant", "copy target sorcery",
			"copy that spell", "copy it",
			"magecraft", "storm",
			"whenever you cast an instant or sorcery",
			"whenever you cast or copy") {
			ctx.spellCopyCount += qp.Qty
		}

		// Spell-trigger permanent detection. Only fires on non-instant-
		// sorcery cards (a permanent is the trigger BEARER; the I/S spell
		// is the trigger SOURCE). Captures the Aetherflux / Stormwing /
		// Guttersnipe / Talrand / Archmage Emeritus payoff cluster used by
		// the absolute-count Spellslinger detection arm.
		isInstantOrSorcery := strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery")
		if !isInstantOrSorcery && containsAny(ot,
			"whenever you cast a spell",
			"whenever you cast an instant or sorcery",
			"magecraft",
			"if you've cast another instant or sorcery",
			"if you've cast an instant or sorcery this turn",
			"for each spell you've cast",
			"for each instant and sorcery spell you've cast",
			"whenever you cast your") {
			ctx.spellTriggerPermanentCount += qp.Qty
		}

		if qp.Profile.IsOutlet {
			ctx.sacrificeCount += qp.Qty
		}
		for _, t := range qp.Profile.Triggers {
			if t == "dies" || t == "sacrifice" {
				ctx.deathTriggers += qp.Qty
				break
			}
		}

		if qp.Profile.IsRecursion {
			ctx.graveyardCount += qp.Qty
		}
		for _, e := range qp.Profile.Effects {
			if e == "self_mill" || e == "mass_reanimate" || e == "land_reanimate" {
				ctx.graveyardCount += qp.Qty
				break
			}
		}
		if containsAny(ot, "mill", "dredge", "surveil") && !strings.Contains(ot, "opponent") {
			ctx.selfMillCount += qp.Qty
		}

		// Graveyard-size payoff detection. "equal to the number of
		// [...] in your graveyard" / "for each [...] in your
		// graveyard" / "[...] cards in your graveyard" + a stat-
		// scaling verb. Excludes "opponent's graveyard" (graveyard-
		// hate from the other side) and excludes the "from your
		// graveyard" recursion phrasing which is already counted via
		// IsRecursion. Sidisi/Bruvac-class self-mill commanders also
		// register via a name match below.
		hasGYSizeScale := (strings.Contains(ot, "equal to") || strings.Contains(ot, "for each")) &&
			strings.Contains(ot, "in your graveyard")
		isCanonicalSelfMillCommander := containsAny(strings.ToLower(qp.Profile.Name),
			"sidisi", "bruvac", "phenax", "splinterfright", "lhurgoyf",
			"tasigur", "jarad", "mortivore", "sutured ghoul")
		if hasGYSizeScale || isCanonicalSelfMillCommander {
			ctx.graveyardSizePayoffCount += qp.Qty
		}

		for _, t := range qp.Profile.Triggers {
			if t == "landfall" {
				ctx.landfallCount += qp.Qty
				break
			}
		}
		if containsAny(ot, "+1/+1 counter", "proliferate", "number of counters", "modified") {
			ctx.counterCount += qp.Qty
		}
		// Narrower proliferate signal: only count cards whose oracle text
		// names the proliferate keyword/action directly. Excludes generic
		// "+1/+1 counter" anthems and "number of counters" payoffs that
		// counterCount conflates. Counters-matter decks (Hardened Scales,
		// Marchesa the Black Rose, Animar) will bump counterCount but not
		// proliferateCount — that's the precision lever the Superfriends
		// disjunctive arm uses to avoid poaching them.
		if containsAny(ot, "proliferate") {
			ctx.proliferateCount += qp.Qty
		}
		if containsAny(ot, "gain life", "whenever you gain life", "lifelink") {
			ctx.lifegainCount += qp.Qty
		}
		if qp.Profile.IsBlinker || containsAny(ot, "exile, then return", "flicker", "exile target creature you control, then return") {
			ctx.blinkCount += qp.Qty
		}
		// ETB-value creature counter — the second prong of the Blink
		// archetype gate. Restricted to creatures because non-creature
		// ETB triggers (artifacts like Solemn Simulacrum, enchantments
		// like Smothering Tithe) are typically one-shots that don't
		// benefit from blink. Brago / Yorion / Aminatou decks live on
		// Mulldrifter / Reclamation Sage / Eternal Witness / Wood Elves
		// / Sun Titan / Cavalier of Gales — all creatures.
		if qp.Profile.HasValueETB && strings.Contains(tl, "creature") {
			ctx.etbValueCreatureCount += qp.Qty
		}
		if qp.Profile.IsExtraCombat || containsAny(ot, "additional combat", "extra combat") {
			ctx.extraCombatCount += qp.Qty
		}
		// Extra-TURN detection (distinct from extra-combat). Match the
		// granting phrasings only — "take an extra turn" / "takes an
		// additional turn" / "take another turn" and their plural-noun
		// variants. The "an" / "another" / "additional" qualifier
		// excludes Stranglehold-style denial wording ("your opponents
		// can't take extra turns" — plural, no qualifier). Adventure
		// cards have per-face oracle text in CardFaces; we count the
		// card once even when multiple halves grant a turn (e.g. a
		// hypothetical Adventure with extra-turn text on both halves).
		if cardHasExtraTurnGrant(ot, qp.Profile.Name, oracle) {
			ctx.extraTurnCount += qp.Qty
		}
		if containsAny(ot, "mills", "put the top", "into their graveyard", "each opponent mills") && strings.Contains(ot, "opponent") {
			ctx.millOppCount += qp.Qty
		}
		if containsAny(ot, "each opponent discards", "target opponent discards", "target player discards", "whenever an opponent discards") {
			ctx.discardForceCount += qp.Qty
		}

		// R60 new-archetype detectors.
		//
		// Pillowfort: combat-tax + damage-prevention shell. Canonical
		// signatures are Propaganda/Ghostly Prison ("can't attack you
		// unless their controller pays"), No Mercy ("whenever a creature
		// deals damage to you, destroy it"), Crawlspace ("no more than
		// two creatures can attack you each combat"), and Solitary
		// Confinement ("prevent all damage that would be dealt to you").
		// "can't attack you" alone covers Propaganda / Ghostly Prison /
		// Sphere of Safety / Norn's Annex / Windborn Muse.
		if containsAny(ot,
			"can't attack you", "to attack you, pay",
			"creatures attacking you have",
			"prevent all damage that would be dealt to you",
			"whenever a creature deals damage to you") ||
			(strings.Contains(ot, "no more than") && strings.Contains(ot, "attack you")) {
			ctx.pillowfortCount += qp.Qty
		}
		// Group Slug: passive damage triggers vs opponents. "each
		// opponent loses" covers Underworld Dreams / Liliana's Caress
		// / Polluted Bonds. "deals damage to each opponent" covers
		// Manabarbs / Pyrostatic Pillar / Eidolon of the Great Revel /
		// Sulfuric Vortex (which actually phrases it as "deals damage
		// to each player" — caught by the second clause).
		if containsAny(ot,
			"each opponent loses",
			"deals damage to each opponent",
			"deals damage to each player",
			"whenever an opponent casts",
			"whenever an opponent draws a card") {
			ctx.groupSlugCount += qp.Qty
		}
		// Damage Redirect: the signature core phrase across the cluster
		// is "deals that much damage" — Boros Reckoner / Spitemare /
		// Truefire Captain / Brash Taunter use the active "is dealt
		// damage, it deals that much damage" wording; Stuffy Doll uses
		// the passive "damage is dealt to {this}, it deals that much
		// damage"; Repercussion uses the spread "damage is dealt to a
		// creature, it deals that much damage to that creature's
		// controller". Pariah's "all damage that would be dealt to you
		// is dealt to enchanted creature instead" + Toralf's "redirect"
		// wording round out the cluster.
		if containsAny(ot,
			"deals that much damage",
			"would be dealt to you is dealt to",
			"redirect that damage") {
			ctx.damageRedirectCount += qp.Qty
		}

		// R60 (post-precon-audit) detectors. Group Hug, Cycling, Toxic,
		// Vehicles — added after the group-{a,b,c} precon shape-scan
		// pass showed these archetypes had no fingerprint and fell
		// through to Midrange/Artifacts/Tribal incorrectly.
		//
		// Group Hug: give cards/lands/life to ALL players (not just
		// yourself). Canonical signatures are Phelddagrif / Kynaios
		// and Tiro / Howling Mine / Font of Mythos / Heartbeat of
		// Spring / Veteran Explorer / Tempting Wurm. The phrasing
		// pattern is "each player [verb]" or "each opponent [verb]"
		// where the verb is a beneficial action (draw / search / gain
		// life / put a land). Distinct from Group Slug ("each
		// opponent LOSES") and from generic symmetric effects.
		if containsAny(ot,
			"each player draws", "each opponent draws",
			"target opponent draws",
			"each player gains", "each opponent gains",
			"each player may search", "each opponent may search",
			"each player puts the top",
			"each player may put a land",
			"each player creates",
			"all players draw") {
			ctx.groupHugCount += qp.Qty
		}
		// Cycling: TWO counters — the keyword density (how many cards
		// have the cycling cost) and the payoff density (cards that
		// trigger when you cycle). A real cycling deck needs BOTH —
		// 15 cycling lands without Astral Drift / Drake Haven is just
		// a deck full of cycling lands, not a cycling-MATTERS deck.
		// "cycling {" catches the keyword-cost text (cycling {2}, etc.).
		if strings.Contains(ot, "cycling {") || strings.Contains(ot, "cycling—") {
			ctx.cyclingCount += qp.Qty
		}
		if containsAny(ot,
			"whenever you cycle", "when you cycle",
			"whenever you discard or cycle",
			"cards with cycling", "spells with cycling",
			"if you've cycled",
			"cycling abilities you activate cost") {
			ctx.cyclingPayoffCount += qp.Qty
		}
		// Toxic / Infect: "infect" keyword text, "toxic N" keyword,
		// "poison counter" payoffs. Distinct from Counters Matter
		// (that's the +1/+1 axis); poison is a separate counter type
		// and an alt-win condition (CR 704.5c — 10 poison = loss).
		// "infect" as a substring also catches "infectious"-type false
		// positives — so we anchor against more specific phrasings.
		if containsAny(ot,
			"deal damage in the form of poison",
			"poison counter", "poisoned opponent",
			"gets infect", "has infect",
			"toxic 1", "toxic 2", "toxic 3", "toxic 4",
			"creatures you control have infect",
			"creatures you control have toxic") ||
			// Standalone "infect" creature ability — appears as a
			// reminder-textless keyword on most modern printings, so
			// we match the word in the keywords list rather than the
			// oracle text body. CardProfile doesn't expose Keywords
			// directly, but oracle text for older infect creatures
			// includes "infect (this creature deals damage to..."
			strings.Contains(ot, "infect (this creature deals damage") {
			ctx.toxicInfectCount += qp.Qty
		}
		// Vehicles: Vehicle subtype on the typeline (modern vehicles)
		// or Spacecraft subtype (EOE). Crew payoffs (Depala / Veteran
		// Motorist) ALSO count via the oracle-text "crew" phrasing
		// because some non-vehicle artifacts/creatures care about
		// crewing. Important: we don't count crew triggers from
		// vehicle-INTERIOR text against this counter (a vehicle's own
		// "Crew 3" line is the cost, not a payoff). The vehicle
		// typeline alone is enough — actual vehicles are the bulk of
		// a vehicle deck.
		if strings.Contains(tl, "vehicle") || strings.Contains(tl, "spacecraft") {
			ctx.vehicleCount += qp.Qty
		} else if containsAny(ot,
			"whenever a vehicle", "whenever a crewed vehicle",
			"vehicles you control",
			"crew costs you pay cost",
			"vehicle creature tokens",
			"crewed by") {
			ctx.vehicleCount += qp.Qty
		}

		if qp.Profile.CMC <= 2 {
			for _, r := range qp.Profile.Produces {
				if r != ResMana {
					continue
				}
				// Tapped-ETB rocks (Coldsteel Heart, Diamond cycle, Star
				// Compass, Coalition Relic) produce mana but don't tap-
				// for-mana the turn they're cast — they're a turn slower
				// than untapped rocks of the same CMC. The B4 bracket
				// signal cares about "T1 Sol Ring → T2 4-drop" tempo, not
				// raw mana-source count, so these are split into a
				// separate counter and surfaced in the rationale.
				if isTappedETBRock(qp.Profile.Name, ot) {
					ctx.tappedManaCount += qp.Qty
					ctx.tappedManaNames = append(ctx.tappedManaNames, qp.Profile.Name)
				} else {
					ctx.fastManaCount += qp.Qty
				}
				break
			}
		}

		// R60 Tokens archetype detection. cardCreatesTokens checks the
		// canonical oracle phrasings ("create [N] [type] token",
		// "creates [N] [type] token", "puts [N] [type] tokens onto the
		// battlefield"), plus token-doubler replacements (Anointed
		// Procession / Parallel Lives / Doubling Season / Mondrak)
		// which create tokens by replacement and earn the creator
		// classification for the structural signal.
		if cardCreatesTokens(ot, qp.Profile.Name) {
			ctx.tokenCreatorCount += qp.Qty
		}
		// cardHasAnthem checks for "creatures you control get +X/+Y"
		// patterns plus tribal anthems "[subtype] creatures you
		// control get +X/+Y". Single-target buffs are excluded — they
		// don't scale with board-wide token presence.
		if cardHasAnthem(ot) {
			ctx.anthemCount += qp.Qty
		}
		// Tribal-lord detection: the subset of anthems that name a
		// specific creature subtype. Tracked separately because 2+
		// lords gated on the same tribe is the structural signature
		// of tribal regardless of whether the deck hits the bare
		// topCreatureTypePct threshold (a 24-elf-creature deck might
		// only be 65% elf-type by raw count if half its creatures are
		// utility role plays, yet the Lord package commits the
		// archetype unambiguously).
		if tribe := cardTribalLordTribe(ot); tribe != "" {
			ctx.tribalLordCount += qp.Qty
			tribeCounts[tribe] += qp.Qty
		}
		// Equipment-trigger payoff detection — cards that boost the
		// equipment engine rather than just being an Equipment piece.
		// Counted separately so the Equipment-Voltron fingerprint can
		// gate on payoff density (3+ payoffs is the load-bearing
		// signal that distinguishes a committed Equipment archetype
		// from a midrange shell with a few equipment in the toolbox).
		if cardIsEquipPayoff(qp.Profile.Name, ot) {
			ctx.equipTriggerPayoffCount += qp.Qty
		}
		// Reanimator-archetype refinement: split "fill graveyard"
		// from "reanimate from graveyard" so the Reanimator gate can
		// require BOTH signals in load-bearing density.
		if cardIsDiscardOutlet(qp.Profile.Name, ot) {
			ctx.discardOutletCount += qp.Qty
		}
		if cardIsReanimationEffect(qp.Profile.Name, ot, qp.Profile.IsRecursion, qp.Profile.RecursionDest) {
			ctx.reanimationCount += qp.Qty
		}
	}
	if len(tribeCounts) > 0 {
		topTribe := ""
		topCount := 0
		for tribe, n := range tribeCounts {
			if n > topCount {
				topTribe = tribe
				topCount = n
			}
		}
		ctx.tribalLordTribe = topTribe
	}

	ctx.instantSorceryCount = instantSorcCount
	ctx.creatureCount = creatureCount
	if nonlandTotal > 0 {
		ctx.instantSorcPct = float64(instantSorcCount) / float64(nonlandTotal)
		ctx.creaturePct = float64(creatureCount) / float64(nonlandTotal)
		ctx.enchantmentPct = float64(enchantmentCount) / float64(nonlandTotal)
	}

	if creatureCount > 0 {
		topCount := 0
		for _, cnt := range creatureTypes {
			if cnt > topCount {
				topCount = cnt
			}
		}
		ctx.topCreatureTypePct = float64(topCount) / float64(creatureCount)
	}

	return ctx
}

// twoCardCategoricalWinClasses lists the combo classes that
// CATEGORICALLY win the game when assembled — no additional outlet
// piece needed. Infinite mana / ETB / blink_engine / etb_payoff /
// etb_doubler / mana_sink are deliberately excluded: they're
// accelerators or value engines that require a separate kill piece
// the deck may or may not have. Lockdown is excluded because it
// doesn't end the game, it just prevents opponents from playing.
var twoCardCategoricalWinClasses = map[string]bool{
	ComboClassInfiniteDamage:  true,
	ComboClassInfiniteDrain:   true,
	ComboClassInfiniteMill:    true,
	ComboClassLibraryExileWin: true,
	ComboClassCombatFinisher:  true,
	ComboClassStormFinisher:   true,
	ComboClassInfiniteTokens:  true, // hasty token engines (Kiki lines) close on the next swing
}

// hasTwoCardCategoricalWin returns true if any combo in the list is a
// 2-card categorical-win combo per twoCardCategoricalWinClasses. Used
// to broaden the B4+ "winning combo" carveout beyond the narrow
// Type=="true_infinite" check — many determined 2-card combos
// (Kiki+Conscripts, Thoracle+Consultation, Hellkite Charger+Bear
// Umbra) win the game just as decisively and warrant the same
// WotC-bracket-framework lift.
func hasTwoCardCategoricalWin(combos []ComboResult) bool {
	for _, c := range combos {
		if len(c.Cards) == 2 && twoCardCategoricalWinClasses[c.Class] {
			return true
		}
	}
	return false
}

// extraTurnGrantPhrases is the canonical phrase set for cards that
// grant a literal extra TURN. The "an"/"another"/"additional" qualifier
// is mandatory so we don't match Stranglehold's "your opponents can't
// take extra turns" (plural noun, denial wording). Listed as separate
// "take" and "takes" variants because Scryfall oracle text uses both
// the imperative ("Take an extra turn after this one." — Nexus of Fate)
// and the targeted-player phrasing ("Target player takes an extra turn
// after this one." — Time Walk).
var extraTurnGrantPhrases = []string{
	"take an extra turn",
	"takes an extra turn",
	"take another turn",
	"takes another turn",
	"take an additional turn",
	"takes an additional turn",
}

// cardHasExtraTurnGrant returns true if the card grants an extra TURN
// (not combat phase / upkeep / draw / etc.). primaryOracle is the
// already-computed oracle text for the card; for Adventure-style
// multi-face cards we also walk CardFaces so a Adventure half on the
// non-primary face still triggers detection. The card-level boolean
// guarantees a single match increments the count once regardless of
// how many phrase occurrences appear across faces.
func cardHasExtraTurnGrant(primaryOracle, cardName string, oracle *oracleDB) bool {
	if containsAny(primaryOracle, extraTurnGrantPhrases...) {
		return true
	}
	if oracle == nil {
		return false
	}
	entry := oracle.lookup(cardName)
	if entry == nil {
		return false
	}
	for _, face := range entry.CardFaces {
		if containsAny(strings.ToLower(face.OracleText), extraTurnGrantPhrases...) {
			return true
		}
	}
	return false
}

// tokenCreationPhrases is the canonical set of oracle text fragments
// indicating a card creates one or more tokens. Covers the modern
// printing wording ("create [a/an/N] X token") plus older / template
// variants ("creates", "puts X tokens onto the battlefield"). The "X"
// in "puts X tokens" is intentionally not bounded by a digit because
// many older cards say "puts a [type] token" or "puts the indicated
// number of [type] tokens" — anchoring on "puts" + "token" + "onto
// the battlefield" is the safest substring-match shape.
var tokenCreationPhrases = []string{
	"create a ", // e.g. "Create a 1/1 white Soldier creature token."
	"create two ",
	"create three ",
	"create four ",
	"create five ",
	"create six ",
	"create seven ",
	"create eight ",
	"create x ",
	"creates a ", // 3rd-person variant
	"creates two ",
	"creates three ",
	"creates x ",
	"create that many ",   // Selvala's Stampede / many "X tokens" cards
	"create one ",         // older phrasing
	"put a ",              // "put a [type] token onto the battlefield"
	"puts a ",
	"puts onto the battlefield", // mass-token effects use this combined form
}

// tokenDoublerNames are token-replacement permanents that don't
// CREATE the first token themselves but DOUBLE every subsequent
// token. They count as creators for the structural Tokens-archetype
// signal because a Krenko / Adeline shell stuffed with 5 actual
// creators + 3 doublers IS a tokens deck — the deck's gameplan
// hinges on the doubled output, not the count of source cards alone.
var tokenDoublerNames = map[string]bool{
	"anointed procession":         true,
	"parallel lives":              true,
	"doubling season":             true,
	"mondrak, glory dominus":      true,
	"primal vigor":                true, // doubles tokens AND counters
	"adrix and nev, twincasters":  true, // doubles tokens (limited to specific token types but still a structural creator signal)
	"second harvest":              true, // one-shot but still a creator
}

// cardCreatesTokens returns true if the lowercased oracle text or
// canonical card name matches a token-creation pattern. False
// positives like "create a copy of target spell" (Spell Copy) are
// filtered out by the "token" anchor requirement — every entry in
// tokenCreationPhrases except the doubler-name fallback requires
// the literal substring "token" to appear elsewhere in the oracle.
// "create a copy" / "put a counter" / "puts a card" pass the
// per-phrase match but fail the "token" anchor.
func cardCreatesTokens(ot, cardName string) bool {
	if ot == "" && cardName == "" {
		return false
	}
	if tokenDoublerNames[strings.ToLower(cardName)] {
		return true
	}
	if !strings.Contains(ot, "token") {
		return false
	}
	for _, phrase := range tokenCreationPhrases {
		if strings.Contains(ot, phrase) {
			return true
		}
	}
	return false
}

// anthemPhrases lists oracle-text shapes that buff "creatures you
// control" (broad anthem like Glorious Anthem / Honor of the Pure)
// OR "[subtype] creatures you control" (tribal anthem like Goblin
// King / Elvish Champion). The "+1/+1"-shape is the canonical
// detection target; "+2/+2" and counter-based +N/+N variants pick
// up the same archetype role. Single-target buffs ("target creature
// you control gets +X/+Y") are NOT included — they don't scale with
// board-wide token presence and would false-positive on every Giant
// Growth / Boros Charm in the deck.
var anthemPhrases = []string{
	"creatures you control get +",
	"other creatures you control get +",
	"creatures you control have +",
	"creature tokens you control get +",
	"creature tokens you control have +",
}

// reanimationEffectNames is the curated list of canonical
// reanimation cards — spells / permanents that put a creature card
// from a graveyard onto the battlefield. Includes the Animate Dead
// Aura family, single-target reanimate sorceries, mass reanimation,
// and the persist / undying mechanics (which read as in-place
// reanimation when a creature dies).
//
// Cards that return to HAND (Eternal Witness, Regrowth, Phyrexian
// Reclamation) are NOT reanimation — they're recursion. The
// distinction is load-bearing: Reanimator uses "from graveyard
// directly onto battlefield" cheats; pure recursion plays the card
// at full cost from hand on a subsequent turn.
var reanimationEffectNames = map[string]bool{
	// Single-target Auras.
	"animate dead":         true,
	"dance of the dead":    true,
	"necromancy":           true,
	"dread return":         true, // not aura but single-target
	"reanimate":            true,
	"exhume":               true,
	"unburial rites":       true,
	"footsteps of the goryo": true,
	"goryo's vengeance":    true,
	"shallow grave":        true,
	"corpse dance":         true,
	"victimize":            true,
	"beacon of unrest":     true,
	"chainer's edict":      true, // not reanim, removing
	"bring back":           true,
	"sevinne's reclamation": true,
	"phantasmagorian":      true,
	// Mass reanimation.
	"living death":          true,
	"twilight's call":       true,
	"patriarch's bidding":   true,
	"rise of the dark realms": true,
	"balthor the defiled":   true,
	"living end":            true,
	"command the dreadhorde": true,
	"finale of devastation": true,
	// Persist creatures (return to battlefield via persist trigger).
	"woodfall primus":         true,
	"murderous redcap":        true,
	"kitchen finks":            true,
	"puppeteer clique":         true,
	"twilight shepherd":        true,
	// Reanimator-recursion creatures (return TO BATTLEFIELD).
	"karmic guide":             true,
	"sun titan":                true,
	"reveillark":               true,
	"angel of glory's rise":    true,
	"apprentice necromancer":   true,
	"corpse connoisseur":       true,
	"doomed necromancer":       true,
	"hell's caretaker":         true,
	"lord of extinction":       true, // not reanim
	"meren of clan nel toth":   true,
	"karador, ghost chieftain": true,
	"chainer, nightmare adept": true,
	"chainer, dementia master": true,
	"the scarab god":           true,
	"alesha, who smiles at death": true,
	"sun-stained scrap":        true, // placeholder, not real
	"recurring nightmare":      true,
	"oversold cemetery":        true,
}

// discardOutletPhrases recognises oracle-text shapes for cards that
// enable discard as a cost or trigger — the engine pieces of the
// Reanimator self-discard pile. Patterns are scoped to phrases that
// signal active discard control (cost-position or looter-style draw-
// then-discard); random opponent-discard effects (Mind Rot, Hymn to
// Tourach) are excluded because those don't fill the controller's
// graveyard.
var discardOutletPhrases = []string{
	"discard a card:",              // activated-ability cost
	"discard a card, then",         // looter pattern (Faithless Looting "draw two, then discard two")
	"discard two cards",            // multi-discard outlets
	"discard your hand",            // Burning Inquiry / Wheel of Fortune family
	"discard the rest",             // looter clauses
	", then discard a card",        // suffix variant
	", then discard two cards",     // suffix variant
	"draw a card, then discard",    // explicit looter
	"draw two cards, then discard", // wheel-light
	"as an additional cost",        // Reanimate-style "exile X from your graveyard" isn't discard but covers cards with discard additional costs
	"discard target",               // self-discard-target shells (Buried Alive variants)
}

// curated discard-outlet names (oracle-pattern misses on some printings).
var discardOutletNames = map[string]bool{
	"faithless looting":   true,
	"wild mongrel":        true,
	"putrid imp":          true,
	"olivia's wrath":      true,
	"frantic search":      true,
	"compulsive research": true,
	"merfolk looter":      true,
	"looter il-kor":       true,
	"key to the city":     true,
	"bone miser":          true,
	"tortured existence":  true,
	"oblivion crown":      true,
	"squee, dubious monarch": true,
	"liliana of the veil": true,
	"liliana vess":        true,
	"burning inquiry":     true,
	"wheel of fortune":    true,
	"windfall":            true,
	"jace, vryn's prodigy": true,
	"smuggler's copter":   true,
	"thrill of possibility": true,
	"cathartic reunion":   true,
}

// cardIsReanimationEffect returns true if the card is a reanimation
// effect — either by curated name OR by matching the "return target
// creature card from [...] graveyard to/onto the battlefield" oracle
// pattern. The pattern matches the canonical Animate Dead / Reanimate
// / Sevinne's Reclamation phrasing.
//
// Also catches cards where IsRecursion is set AND RecursionDest is
// "battlefield" — defense-in-depth for cards the curated list and
// oracle pattern miss (parser-tagged via IsRecursion but with a non-
// canonical printed phrasing).
func cardIsReanimationEffect(name, ot string, isRecursion bool, recursionDest string) bool {
	if reanimationEffectNames[strings.ToLower(strings.TrimSpace(name))] {
		return true
	}
	if isRecursion && recursionDest == "battlefield" {
		return true
	}
	if ot == "" {
		return false
	}
	// Canonical printed shapes.
	if strings.Contains(ot, "return target creature card") &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "to the battlefield") || strings.Contains(ot, "onto the battlefield")) {
		return true
	}
	if strings.Contains(ot, "put target creature card") &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "onto the battlefield") || strings.Contains(ot, "into play")) {
		return true
	}
	// Mass reanimation: "return all creature cards from [...] graveyard[s]"
	if strings.Contains(ot, "return all creature cards") &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "to the battlefield") || strings.Contains(ot, "onto the battlefield")) {
		return true
	}
	// Persist / undying — in-place reanimation on death.
	if strings.Contains(ot, "persist (when this creature dies") ||
		strings.Contains(ot, "undying (when this creature dies") ||
		(strings.Contains(ot, "persist") && strings.Contains(ot, "-1/-1 counter")) ||
		(strings.Contains(ot, "undying") && strings.Contains(ot, "+1/+1 counter")) {
		return true
	}
	return false
}

// cardIsDiscardOutlet returns true if the card is a discard outlet —
// either by curated name OR by an oracle-text phrase scan. Random
// opponent-discard cards (Mind Rot, Hymn to Tourach) are excluded
// because they don't fill the controller's graveyard.
func cardIsDiscardOutlet(name, ot string) bool {
	if discardOutletNames[strings.ToLower(strings.TrimSpace(name))] {
		return true
	}
	if ot == "" {
		return false
	}
	// Exclude opponent-targeted discard.
	if strings.Contains(ot, "target opponent discards") ||
		strings.Contains(ot, "each opponent discards") ||
		strings.Contains(ot, "target player discards") {
		return false
	}
	for _, phrase := range discardOutletPhrases {
		if strings.Contains(ot, phrase) {
			return true
		}
	}
	return false
}

// equipmentTriggerPayoffNames is the curated list of canonical
// equipment-trigger payoff cards. A card matches as an
// equipTriggerPayoff if it's in this list OR its oracle text contains
// one of the equipmentTriggerPayoffPhrases below.
//
// Curated entries cover the cards whose payoff text is parser-
// resilient but rarely catches via the substring scan (Stoneforge
// Mystic's two-step tutor-then-free-cast doesn't read as "equipment
// enters" / "cast an equipment"; Halvar's static buff lives behind a
// double-faced front-back layout).
var equipmentTriggerPayoffNames = map[string]bool{
	"puresteel paladin":              true,
	"sigarda's aid":                  true,
	"sram, senior edificer":          true,
	"stoneforge mystic":              true,
	"stonehewer giant":               true,
	"kemba, kha regent":              true,
	"kemba, kha enduring":            true,
	"akiri, line-slinger":            true,
	"akiri, fearless voyager":        true,
	"halvar, god of battle":          true,
	"nahiri, forged in fury":         true,
	"bruenor battlehammer":           true,
	"valduk, keeper of the flame":    true,
	"tiana, ship's caretaker":        true,
	"toggo, goblin weaponsmith":      true,
	"auriok steelshaper":             true,
	"stone haven outfitter":          true,
	"open the armory":                true,
	"steelshaper's gift":             true,
	"steelshaper apprentice":         true,
	"ardenn, intrepid archaeologist": true,
	"hammer of nazahn":               true,
	"nazahn, revered bladesmith":     true,
	"danitha capashen, paragon":      true,
	"balan, wandering knight":        true,
	"forging the tyrite sword":       true,
}

// equipmentTriggerPayoffPhrases recognises oracle-text shapes for
// equip-payoff cards that aren't in the curated list. Patterns are
// scoped to phrases that ONLY appear on equip-archetype payoffs —
// generic "creatures you control" anthems are excluded, and the
// bare-Equipment template "equipped creature gets +X/+Y" is
// deliberately NOT included because every Equipment piece carries
// that shape in its own buff text (Hammer of Nazahn, Bonesplitter,
// Whispersilk Cloak all match "equipped creature gets/has..." for
// their OWN buff — those are pieces, not payoffs). The phrases below
// are the engine-buffing shapes: plural "equipped creatures you
// control", board-wide "equipment you control have/get", count-up
// "for each equipment", and the ETB / cast / attach trigger shapes.
var equipmentTriggerPayoffPhrases = []string{
	"whenever an equipment enters",
	"whenever you cast an equipment",
	"whenever you attach",
	"equipment spells you cast cost",
	"equipped creatures you control",
	"equipment you control have ",
	"equipment you control get ",
	"for each equipment you control",
	"equipment abilities you activate cost",
	"search your library for an equipment",
}

// cardIsEquipPayoff returns true if the card buffs the equipment
// engine itself — either by name (curated list) or by oracle-text
// shape. A bare Equipment with no oracle-text payoff (Bonesplitter,
// Whispersilk Cloak) is NOT a payoff — it's a piece. The distinction
// is the LOAD-BEARING signal that separates Equipment-Voltron from a
// midrange-with-equipment-toolbox shape.
func cardIsEquipPayoff(name, ot string) bool {
	if equipmentTriggerPayoffNames[strings.ToLower(strings.TrimSpace(name))] {
		return true
	}
	if ot == "" {
		return false
	}
	for _, phrase := range equipmentTriggerPayoffPhrases {
		if strings.Contains(ot, phrase) {
			return true
		}
	}
	return false
}

// cardHasAnthem returns true if the lowercased oracle text matches
// an anthem shape. Tribal anthems (Goblin King's "other Goblin
// creatures you control get +1/+1") flow through the
// "creatures you control get +" substring match — the subtype prefix
// is preserved in the matched substring but doesn't break detection.
func cardHasAnthem(ot string) bool {
	if ot == "" {
		return false
	}
	for _, phrase := range anthemPhrases {
		if strings.Contains(ot, phrase) {
			return true
		}
	}
	return false
}

// cardTribalLordTribe returns the (lowercased) creature subtype this
// card "lords for" — i.e. buffs all instances of that tribe under the
// controller. Returns "" if the card isn't a tribal lord.
//
// Detection anchors on two clause shapes:
//
//	"<tribe> creatures you control [...] (get|have) +"
//	"<tribe>s you control [...] (get|have) +"
//
// The intervening "[...]" window allows real-card phrasings like
// Goblin Chieftain's "Goblin creatures you control HAVE HASTE AND
// GET +1/+1" — "get +" doesn't immediately follow "you control"
// because the keyword grant ("have haste") lives in the middle of
// the clause. The window is scoped to the same sentence (up to the
// next period) so unrelated body text can't bleed in.
//
// The "other <tribe>" variant (Goblin King: "Other Goblin creatures
// you control get +1/+1") flows through the first shape because the
// "other " prefix doesn't break the substring match. Single-target
// buffs ("target Goblin you control gets +1/+1") are excluded because
// the pattern demands plural "creatures" or the "<tribe>s" form.
//
// Returns the FIRST matched tribe; if a single card buffs multiple
// tribes (rare — see Door of Destinies / Coat of Arms which are
// changeling-style and not tribe-specific so they don't match here),
// the first tribe in knownCreatureTypes order wins. That's acceptable
// for the count-based signal — the lord still earns its place in
// tribalLordCount; the tribe label is just for the human-readable
// signal string.
func cardTribalLordTribe(ot string) string {
	if ot == "" {
		return ""
	}
	for _, tribe := range knownCreatureTypes {
		if tribe == "" {
			continue
		}
		// Plural-suffix tribal anthems like "Goblins you control get +1/+1".
		// "Creature"/"creatures" never appears as a tribe in
		// knownCreatureTypes (defense-in-depth against future list edits).
		if tribe == "creature" || tribe == "creatures" {
			continue
		}
		if clauseHasBuffAnchor(ot, tribe+" creatures you control") {
			return tribe
		}
		if clauseHasBuffAnchor(ot, tribe+"s you control") {
			return tribe
		}
	}
	return ""
}

// clauseHasBuffAnchor returns true if `anchor` appears in `ot` AND
// the clause that follows (up to the next period or newline) contains
// "get +" or "have +" — the canonical anthem-shape buff. Used by
// cardTribalLordTribe to tolerate intervening keyword grants between
// "<tribe> creatures you control" and the "+" stat bump (Goblin
// Chieftain's "have haste and get +1/+1" pattern).
func clauseHasBuffAnchor(ot, anchor string) bool {
	start := 0
	for {
		idx := strings.Index(ot[start:], anchor)
		if idx < 0 {
			return false
		}
		// Walk the clause starting right after the anchor up to the
		// next sentence boundary.
		rest := ot[start+idx+len(anchor):]
		if end := strings.IndexAny(rest, ".\n"); end >= 0 {
			rest = rest[:end]
		}
		if strings.Contains(rest, "get +") || strings.Contains(rest, "have +") {
			return true
		}
		// No buff anchor in THIS occurrence's clause — keep scanning
		// further into the oracle text in case the anchor appears
		// twice (rare, but defensive).
		start += idx + len(anchor)
		if start >= len(ot) {
			return false
		}
	}
}

func euclideanDistance(actual map[RoleTag]float64, template map[RoleTag]float64) float64 {
	sum := 0.0
	for role, target := range template {
		diff := actual[role] - target
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

func buildSignals(ctx *classifyContext, ac *ArchetypeClassification) []string {
	var signals []string

	if ctx.comboCount >= 5 {
		signals = append(signals, fmt.Sprintf("heavy combo density (%d lines)", ctx.comboCount))
	} else if ctx.comboCount >= 2 {
		signals = append(signals, fmt.Sprintf("combo present (%d lines)", ctx.comboCount))
	}

	if ctx.tutorDensity >= 0.12 {
		signals = append(signals, fmt.Sprintf("tutor-heavy (%.0f%%)", ctx.tutorDensity*100))
	}

	if ctx.fastManaCount >= 8 {
		signals = append(signals, fmt.Sprintf("fast mana dense (%d pieces)", ctx.fastManaCount))
	}

	if ctx.avgCMC < 2.0 {
		signals = append(signals, fmt.Sprintf("extremely lean curve (%.1f avg)", ctx.avgCMC))
	} else if ctx.avgCMC < 2.5 {
		signals = append(signals, fmt.Sprintf("lean curve (%.1f avg)", ctx.avgCMC))
	} else if ctx.avgCMC > 3.5 {
		signals = append(signals, fmt.Sprintf("heavy curve (%.1f avg)", ctx.avgCMC))
	}

	if ctx.roleRatios[RoleStax] >= 0.06 {
		signals = append(signals, fmt.Sprintf("stax presence (%.0f%%)", ctx.roleRatios[RoleStax]*100))
	}

	if ctx.roleRatios[RoleCounterspell] >= 0.06 {
		signals = append(signals, fmt.Sprintf("counter-heavy (%.0f%%)", ctx.roleRatios[RoleCounterspell]*100))
	}

	if ctx.instantSorcPct >= 0.55 {
		signals = append(signals, fmt.Sprintf("spell-heavy (%.0f%% instants/sorceries)", ctx.instantSorcPct*100))
	}

	if ctx.topCreatureTypePct >= 0.40 && ctx.creaturePct >= 0.35 {
		signals = append(signals, "strong tribal core")
	}

	if ctx.tribalLordCount >= 2 {
		tribeLabel := ctx.tribalLordTribe
		if tribeLabel == "" {
			tribeLabel = "tribal"
		}
		signals = append(signals, fmt.Sprintf("tribal lord package (%d %s lords)", ctx.tribalLordCount, tribeLabel))
	}

	if ctx.equipmentCount >= 8 && ctx.equipTriggerPayoffCount >= 3 {
		signals = append(signals, fmt.Sprintf("equipment engine (%d equipment + %d payoffs)",
			ctx.equipmentCount, ctx.equipTriggerPayoffCount))
	}

	if (ctx.selfMillCount+ctx.discardOutletCount) >= 6 && ctx.reanimationCount >= 4 {
		signals = append(signals, fmt.Sprintf("reanimation engine (%d fill + %d reanimate)",
			ctx.selfMillCount+ctx.discardOutletCount, ctx.reanimationCount))
	}

	if ctx.freeInteractionCount >= 2 {
		signals = append(signals, fmt.Sprintf("free interaction suite (%d pieces)", ctx.freeInteractionCount))
	}

	if ctx.gameChangerCount > 0 {
		signals = append(signals, fmt.Sprintf("%d Game Changer(s)", ctx.gameChangerCount))
	}

	if ctx.bannedCount > 0 {
		signals = append(signals, fmt.Sprintf("%d banned card(s) excluded from bracket scoring", ctx.bannedCount))
	}

	return signals
}

func buildIntent(ac *ArchetypeClassification, report *FreyaReport, ctx *classifyContext) string {
	primary := ac.Primary
	secondary := ac.Secondary

	var label string
	if secondary != "" && ac.PrimaryConfidence < 0.40 {
		label = primary + "-" + secondary + " hybrid"
	} else {
		label = strings.ToLower(primary) + " deck"
	}

	var gameplan string
	switch primary {
	case "Combo":
		if ctx.comboCount > 0 {
			gameplan = fmt.Sprintf("assemble one of %d combo lines while controlling the board", ctx.comboCount)
		} else {
			gameplan = "assemble a combo win"
		}
	case "Control":
		gameplan = "answer threats and win in the late game with card advantage"
	case "Stax":
		gameplan = "deploy lock pieces to deny opponents resources while advancing its own position"
	case "Aggro":
		gameplan = "deploy threats early and close before opponents stabilize"
	case "Midrange":
		gameplan = "trade efficiently and grind value until it can close"
	case "Voltron":
		gameplan = "suit up the commander and eliminate players through commander damage"
	case "Equipment-Voltron":
		gameplan = "tutor and stack equipment on a single threat (commander), leverage equip-payoff triggers (free attaches, card draw on equipment ETB) for tempo, and close through commander damage"
	case "Aristocrats":
		gameplan = "sacrifice creatures for incremental drain and value"
	case "Spellslinger":
		gameplan = "chain instants and sorceries for cumulative payoffs"
	case "Tribal":
		gameplan = "build a critical mass of synergistic creatures"
	case "Reanimator":
		gameplan = "fill the graveyard and cheat high-value threats into play"
	case "Selfmill":
		gameplan = "mill itself aggressively and scale graveyard-size payoffs"
	case "Lands Matter":
		gameplan = "abuse land drops and landfall triggers for cumulative value"
	case "Enchantress":
		gameplan = "chain enchantments for card advantage while building a pillowfort"
	case "Counters Matter":
		gameplan = "distribute and multiply +1/+1 counters across its board"
	case "Storm":
		gameplan = "chain cheap spells in a single explosive turn for a lethal storm count"
	case "Lifegain":
		gameplan = "gain life for incremental value and convert life total into a win condition"
	case "Blink":
		gameplan = "flicker permanents to re-trigger ETB abilities for repeatable value"
	case "Artifacts":
		gameplan = "build an artifact engine that generates mana and card advantage"
	case "Extra Combats":
		gameplan = "take additional combat phases to multiply damage output"
	case "Superfriends":
		gameplan = "deploy planeswalkers and protect them while ticking toward ultimates"
	case "Mill":
		gameplan = "empty opponent libraries through mill effects"
	case "Discard":
		gameplan = "strip opponents' hands and profit from discard triggers"
	case "Pillowfort":
		gameplan = "tax and deflect attacks against itself while grinding to a slow inevitability"
	case "Group Slug":
		gameplan = "punish opponents passively — every spell, draw, and untap chips away at their life total"
	case "Damage Redirect":
		gameplan = "absorb damage onto a Stuffy Doll-style redirector and reflect it back at opponents"
	case "Group Hug":
		gameplan = "give every player resources to keep the table happy, then pivot to an asymmetric payoff that only rewards you"
	case "Cycling":
		gameplan = "cycle the library to fuel triggered payoffs and assemble a tokens/burn finish from the graveyard"
	case "Toxic":
		gameplan = "drip poison counters onto opponents until they hit ten and lose to the alt-win condition"
	case "Vehicles":
		gameplan = "crew vehicles into the red zone and chain pilot triggers for resilient combat pressure"
	default:
		gameplan = "execute its game plan through incremental advantage"
	}

	var disguise string
	if secondary != "" && ac.PrimaryConfidence < 0.40 {
		disguise = fmt.Sprintf(" It looks like %s but pivots to %s when the window opens.", strings.ToLower(secondary), strings.ToLower(primary))
	}

	var speed string
	if ctx.avgCMC < 2.2 && ctx.fastManaCount >= 6 {
		speed = " Expects to threaten a win by turn 4-5."
	} else if ctx.avgCMC < 2.8 {
		speed = " Aims to establish its position by turn 5-6."
	} else if ctx.avgCMC > 3.5 {
		speed = " Plans to operate in the mid-to-late game."
	}

	return fmt.Sprintf("This is a %s that wants to %s.%s%s", label, gameplan, disguise, speed)
}

// landCycleSynergyArchetypes lists primary archetypes where a
// dual-cycle land pair (Scattered Groves + Irrigated Farmland, etc.)
// is a deliberate wincon component — Lands Matter strats use the
// cycle as a land-into-graveyard pipeline that Crucible / Excavator /
// Splendid Reclamation cash in; Reanimator uses the cycle as a
// targeted-discard outlet for the reanimate target; Selfmill uses
// the discard cost as a self-mill enabler. In every other archetype
// the cycle is incidental fixing and must NOT lift bracket.
var landCycleSynergyArchetypes = map[string]bool{
	"Lands Matter": true,
	"Reanimator":   true,
	"Selfmill":     true,
}

// estimateMeasuredBracket computes Freya's signal-driven bracket call —
// the "measured" bracket. The result populates ArchetypeClassification's
// MeasuredBracket / MeasuredBracketLabel pair. The user-visible Bracket
// field (the rubber-stamp / declared identity) defaults to this value
// but can be overridden at the DeckProfile layer (e.g. WotC precons
// under data/decks/wizards/ auto-stamp to B2 regardless of measurement).
func estimateMeasuredBracket(ctx *classifyContext, report *FreyaReport, primaryArchetype string) (int, string, *BracketRationale) {
	rationale := &BracketRationale{}
	addScore := func(name, tier, measurement string, evidence []string, points int) {
		rationale.RawScore += points
		rationale.Signals = append(rationale.Signals, BracketSignal{
			Name:         name,
			Kind:         "score",
			Tier:         tier,
			Measurement:  measurement,
			Evidence:     evidence,
			Contribution: points,
		})
	}

	// WotC Game Changers — heaviest signal, aligns with official bracket axis
	switch {
	case ctx.gameChangerCount >= 8:
		addScore("Game Changers", "8+", fmt.Sprintf("%d on WotC list", ctx.gameChangerCount),
			ctx.gameChangerNames, 4)
	case ctx.gameChangerCount >= 4:
		addScore("Game Changers", "4-7", fmt.Sprintf("%d on WotC list", ctx.gameChangerCount),
			ctx.gameChangerNames, 3)
	case ctx.gameChangerCount >= 2:
		addScore("Game Changers", "2-3", fmt.Sprintf("%d on WotC list", ctx.gameChangerCount),
			ctx.gameChangerNames, 2)
	case ctx.gameChangerCount >= 1:
		addScore("Game Changers", "1", fmt.Sprintf("%d on WotC list", ctx.gameChangerCount),
			ctx.gameChangerNames, 1)
	}

	switch {
	case ctx.tutorDensity >= 0.12:
		addScore("Tutor density", "12%+", fmt.Sprintf("%.0f%% of nonlands", ctx.tutorDensity*100), nil, 3)
	case ctx.tutorDensity >= 0.08:
		addScore("Tutor density", "8-11%", fmt.Sprintf("%.0f%% of nonlands", ctx.tutorDensity*100), nil, 2)
	case ctx.tutorDensity >= 0.04:
		addScore("Tutor density", "4-7%", fmt.Sprintf("%.0f%% of nonlands", ctx.tutorDensity*100), nil, 1)
	}

	// Combo line scoring. Land-cycle synergies are reclassified out of
	// Determined upstream (see analysis.go extractLandCyclePairs), so
	// ctx.comboCount already excludes them. In the LandsMatter /
	// Reanimator / Selfmill archetypes the cycle IS the wincon
	// component, so we add it back; everywhere else it stays
	// out of the bracket-lifting count.
	effectiveComboCount := ctx.comboCount
	lcs := len(report.LandCycleSynergies)
	if lcs > 0 && landCycleSynergyArchetypes[primaryArchetype] {
		effectiveComboCount += lcs
	}
	switch {
	case effectiveComboCount >= 5:
		addScore("Combo lines", "5+", fmt.Sprintf("%d true-infinite/determined loops", effectiveComboCount), nil, 3)
	case effectiveComboCount >= 2:
		addScore("Combo lines", "2-4", fmt.Sprintf("%d true-infinite/determined loops", effectiveComboCount), nil, 2)
	case effectiveComboCount >= 1:
		addScore("Combo lines", "1", fmt.Sprintf("%d true-infinite/determined loop", effectiveComboCount), nil, 1)
	}
	if lcs > 0 {
		kind := "note"
		note := fmt.Sprintf("%d land-cycle synergy pair(s) excluded from combo count (incidental fixing outside Lands Matter / Reanimator / Selfmill)", lcs)
		if landCycleSynergyArchetypes[primaryArchetype] {
			note = fmt.Sprintf("%d land-cycle synergy pair(s) counted toward combo lines (archetype %q uses land-cycles as wincon component)", lcs, primaryArchetype)
		}
		rationale.Signals = append(rationale.Signals, BracketSignal{
			Name: "Land-cycle synergy",
			Kind: kind,
			Note: note,
		})
	}

	switch {
	case ctx.avgCMC < 2.0:
		addScore("Average CMC", "lean (<2.0)", fmt.Sprintf("%.1f avg", ctx.avgCMC), nil, 2)
	case ctx.avgCMC < 2.5:
		addScore("Average CMC", "moderate (<2.5)", fmt.Sprintf("%.1f avg", ctx.avgCMC), nil, 1)
	case ctx.avgCMC > 3.5:
		addScore("Average CMC", "heavy (>3.5)", fmt.Sprintf("%.1f avg", ctx.avgCMC), nil, -1)
	}

	switch {
	case ctx.fastManaCount >= 10:
		addScore("Fast mana", "10+", fmt.Sprintf("%d sub-2-CMC mana producers", ctx.fastManaCount), nil, 3)
	case ctx.fastManaCount >= 6:
		addScore("Fast mana", "6-9", fmt.Sprintf("%d sub-2-CMC mana producers", ctx.fastManaCount), nil, 2)
	case ctx.fastManaCount >= 3:
		addScore("Fast mana", "3-5", fmt.Sprintf("%d sub-2-CMC mana producers", ctx.fastManaCount), nil, 1)
	}
	if ctx.tappedManaCount > 0 {
		names := ctx.tappedManaNames
		if len(names) > 4 {
			names = append([]string{}, names[:4]...)
			names = append(names, fmt.Sprintf("+%d more", len(ctx.tappedManaNames)-4))
		}
		rationale.Signals = append(rationale.Signals, BracketSignal{
			Name: "Fast mana",
			Kind: "note",
			Note: fmt.Sprintf("%d tapped-ETB rock(s) excluded from fast-mana count (%s) — they don't tap-for-mana the turn cast, so they don't drive the T2/T3 wincon tempo the B4 signal measures",
				ctx.tappedManaCount, strings.Join(names, ", ")),
		})
	}

	if ctx.roleRatios[RoleCounterspell] >= 0.06 {
		addScore("Counterspell density", "6%+",
			fmt.Sprintf("%.0f%% of nonlands", ctx.roleRatios[RoleCounterspell]*100), nil, 1)
	}

	if report.Roles != nil {
		landRatio := ctx.roleRatios[RoleLand]
		if landRatio < 0.30 {
			addScore("Land ratio", "<30%",
				fmt.Sprintf("%.0f%% lands (spell-dense)", landRatio*100), nil, 1)
		}
	}

	// Extra-turn density — a deck packing 4+ extra-turn spells is
	// playing the extra-turns archetype, which the WotC bracket
	// framework treats as a B4 marker (the closer is "chain 3-4 turns
	// in a row → uncontestable win"). Counts only literal extra-turn
	// grants, NOT extra combats (Aggravated Assault / Hellkite Charger
	// are scored separately through the combo / finisher signals).
	switch {
	case ctx.extraTurnCount >= 7:
		addScore("Extra-turn density", "7+",
			fmt.Sprintf("%d extra-turn spells", ctx.extraTurnCount), nil, 3)
	case ctx.extraTurnCount >= 4:
		addScore("Extra-turn density", "4-6",
			fmt.Sprintf("%d extra-turn spells", ctx.extraTurnCount), nil, 2)
	}

	// Graveyard-loop combos — per WotC bracket framework, "value
	// engines that can loop with a graveyard enabler" are a B3-tier
	// marker (softer than a 2-card categorical-win infinite, which is
	// B4). Detected as a 2-card combo whose play sequence buries a
	// piece, paired with a deck-level recursion enabler (Sun Titan /
	// Muldrotha / Karador / Meren / Sheoldred / Reya / Karmic Guide).
	// Single detection contributes +2 (B3 marker on its own); 2+
	// distinct (combo × enabler) pairs signal a deliberately redundant
	// graveyard-loop deck, contributing +3 to reflect the
	// commitment. Evidence cards are the three-card tuple per entry,
	// flattened and deduplicated for display.
	gyLoopCount := len(report.GraveyardLoops)
	if gyLoopCount > 0 {
		var evidence []string
		seenEv := map[string]bool{}
		for _, gl := range report.GraveyardLoops {
			for _, name := range gl.Cards {
				if seenEv[name] {
					continue
				}
				seenEv[name] = true
				evidence = append(evidence, name)
			}
		}
		switch {
		case gyLoopCount >= 2:
			addScore("Graveyard loop combo", "2+ pairs",
				fmt.Sprintf("%d (combo × enabler) loops with graveyard backup", gyLoopCount),
				evidence, 3)
		default:
			addScore("Graveyard loop combo", "1 pair",
				fmt.Sprintf("%d (combo × enabler) loop with graveyard backup", gyLoopCount),
				evidence, 2)
		}
	}

	// Finisher density — distinct win-condition lines signal tuned optimization.
	finisherCount := len(report.Finishers)
	switch {
	case finisherCount >= 8:
		addScore("Finisher density", "8+",
			fmt.Sprintf("%d distinct finisher lines", finisherCount), nil, 2)
	case finisherCount >= 4:
		addScore("Finisher density", "4-7",
			fmt.Sprintf("%d distinct finisher lines", finisherCount), nil, 1)
	}

	// Free interaction — the deck-shape signal that separates true cEDH
	// from merely-optimized B4. Pitch counters, phyrexian-mana spells,
	// pact cycle, commander-free spells, evoke elementals.
	switch {
	case ctx.freeInteractionCount >= 4:
		addScore("Free interaction", "4+",
			fmt.Sprintf("%d pitch/phyrexian/commander-free pieces", ctx.freeInteractionCount),
			ctx.freeInteractionNames, 3)
	case ctx.freeInteractionCount >= 2:
		addScore("Free interaction", "2-3",
			fmt.Sprintf("%d pitch/phyrexian/commander-free pieces", ctx.freeInteractionCount),
			ctx.freeInteractionNames, 2)
	case ctx.freeInteractionCount >= 1:
		addScore("Free interaction", "1",
			fmt.Sprintf("%d pitch/phyrexian/commander-free piece", ctx.freeInteractionCount),
			ctx.freeInteractionNames, 1)
	}
	score := rationale.RawScore

	var bracket int
	var label string
	switch {
	case score >= 12:
		bracket = 5
		label = "cEDH"
	case score >= 8:
		bracket = 4
		label = "Optimized"
	case score >= 5:
		bracket = 3
		label = "Upgraded"
	case score >= 2:
		bracket = 2
		label = "Core"
	default:
		bracket = 1
		label = "Exhibition"
	}

	addAdjustment := func(name, kind, note string) {
		rationale.Signals = append(rationale.Signals, BracketSignal{
			Name: name,
			Kind: kind,
			Note: note,
		})
	}

	// B5 confirmation gate. The raw score threshold of 12+ catches tuned
	// decks but can be reached by stacking GCs + tutors + fast mana even
	// when the deck doesn't have the deck-shape signature of cEDH (free
	// interaction, multi-tutor density, low CMC). Decks that reach B5
	// score-wise but lack ALL of these markers are demoted to B4 — they're
	// optimized but not tournament-tuned. The gate requires AT LEAST ONE
	// hard cEDH marker:
	//   - 2+ free interaction pieces (the primary signal)
	//   - 12%+ tutor density (consistency required for tournament play)
	//   - 8+ Game Changers (heavy cEDH-card density)
	// AND avgCMC < 2.8 (cEDH decks are lean; a 3.0+ CMC pile is too slow
	// to win on turn 3-4 regardless of how many GCs it stacks).
	if bracket == 5 {
		hasCEDHMarker := ctx.freeInteractionCount >= 2 ||
			ctx.tutorDensity >= 0.12 ||
			ctx.gameChangerCount >= 8
		switch {
		case !hasCEDHMarker:
			bracket = 4
			label = "Optimized"
			addAdjustment("B5 gate", "gate",
				fmt.Sprintf("demoted to B4: no cEDH marker (free interaction %d<2, tutors %.0f%%<12, GCs %d<8)",
					ctx.freeInteractionCount, ctx.tutorDensity*100, ctx.gameChangerCount))
		case ctx.avgCMC >= 2.8:
			bracket = 4
			label = "Optimized"
			addAdjustment("B5 gate", "gate",
				fmt.Sprintf("demoted to B4: avg CMC %.1f >= 2.8 (cEDH curves are leaner)", ctx.avgCMC))
		default:
			addAdjustment("B5 gate", "gate",
				fmt.Sprintf("confirmed: cEDH marker present (free interaction %d / tutors %.0f%% / GCs %d) and avg CMC %.1f < 2.8",
					ctx.freeInteractionCount, ctx.tutorDensity*100, ctx.gameChangerCount, ctx.avgCMC))
		}
	}

	// Ceilings — WotC GC caps, modulated by combo presence.
	// Per WotC bracket framework: a 2-card combo that wins the game is itself
	// a B4 marker (Bracket 3 explicitly disallows winning 2-card infinites;
	// Bracket 4 explicitly allows them). So combo presence lifts the GC ceiling.
	// Determined-loop counts are intentionally NOT used here — heuristic combo
	// detection over-classifies value engines (e.g. tribal Werewolf with 4
	// "determined loops" is really value, not B4 combo).
	trueInfCount := len(report.TrueInfinites)
	// Per WotC: "winning 2-card combo" is itself a B4 marker (B3
	// explicitly disallows, B4 explicitly allows). Original predicate
	// only counted Type=="true_infinite" entries — too narrow,
	// since determined 2-card categorical wins (Kiki+Conscripts,
	// Thoracle+Consultation, Walking Ballista+Heliod, Hellkite
	// Charger+Bear Umbra) all win the game when assembled and should
	// also trip the carveout. Broadened predicate counts ANY 2-card
	// combo (TrueInfinites or Determined) whose Class is a categorical
	// win shape (damage/drain/mill/library-exile/combat/storm/tokens).
	// Mana-into-win combos are intentionally NOT in the win-class set
	// — infinite mana without a sink is just acceleration, not a win.
	hasWinningCombo := trueInfCount >= 1 ||
		hasTwoCardCategoricalWin(report.TrueInfinites) ||
		hasTwoCardCategoricalWin(report.Determined)
	// Finisher-density override: many distinct closer lines + accelerator
	// density signals tuned optimization even without an explicit infinite.
	tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6

	preCeilingBracket := bracket
	if ctx.gameChangerCount == 0 {
		// No Game Changers: B2 cap by default, but a true winning 2-card combo
		// is itself a B4 marker per WotC's combo carveout.
		if hasWinningCombo {
			if bracket > 4 {
				bracket = 4
				label = "Optimized"
				addAdjustment("GC=0 ceiling", "ceiling",
					"capped at B4: true-infinite combo present (WotC combo carveout)")
			}
		} else if bracket > 2 {
			bracket = 2
			label = "Core"
			addAdjustment("GC=0 ceiling", "ceiling",
				fmt.Sprintf("held at Bracket 2 — this deck has no Game Changers and no game-winning infinite combo, so it can't sit at Bracket 3 or higher even though smaller signals added up to a raw score of B%d. Adding a Game Changer or a real 2-card combo would unlock the higher bracket.", preCeilingBracket))
		}
	} else if ctx.gameChangerCount <= 3 {
		// Light GC presence: B3 cap by default; lifts to B4 when a real
		// winning combo or tuned-redundancy signal is present.
		if hasWinningCombo || tunedRedundancy {
			if bracket > 4 {
				bracket = 4
				label = "Optimized"
				addAdjustment("GC=1-3 ceiling", "ceiling",
					"capped at B4: winning combo or tuned-redundancy signal present")
			}
		} else if bracket > 3 {
			bracket = 3
			label = "Upgraded"
			addAdjustment("GC=1-3 ceiling", "ceiling",
				fmt.Sprintf("capped at B3: %d GC and no winning combo (was B%d on raw score)",
					ctx.gameChangerCount, preCeilingBracket))
		}
	}

	// Floors — GC presence guarantees minimum bracket.
	// 5+ Game Changers is a deliberate optimization choice — floor at B4.
	preFloorBracket := bracket
	if ctx.gameChangerCount >= 1 && ctx.gameChangerCount <= 3 && bracket < 2 {
		bracket = 2
		label = "Core"
		addAdjustment("GC=1-3 floor", "floor",
			fmt.Sprintf("lifted to B2: %d GC present (was B%d on raw score)",
				ctx.gameChangerCount, preFloorBracket))
	}
	if ctx.gameChangerCount >= 4 && bracket < 3 {
		bracket = 3
		label = "Upgraded"
		addAdjustment("GC=4+ floor", "floor",
			fmt.Sprintf("lifted to B3: %d GC present (was B%d on raw score)",
				ctx.gameChangerCount, preFloorBracket))
	}
	if ctx.gameChangerCount >= 5 && bracket < 4 {
		bracket = 4
		label = "Optimized"
		addAdjustment("GC=5+ floor", "floor",
			fmt.Sprintf("lifted to B4: %d GC = deliberate optimization (was B%d)",
				ctx.gameChangerCount, preFloorBracket))
	}
	// Categorical-win-combo floor: per WotC bracket framework, the
	// presence of a 2-card combo that wins the game is itself a B4
	// marker (B3 explicitly disallows winning 2-card infinites; B4
	// explicitly allows them). hasWinningCombo is the broadened
	// predicate covering both true_infinites and determined 2-card
	// categorical wins (damage / drain / mill / library-exile / combat
	// / storm / tokens). This is a FLOOR, not just a ceiling lift —
	// the combo is a categorical bracket marker regardless of the
	// raw-score path. Restricted to decks with at least one GC OR
	// the combo carveout itself so a precon-shape deck with zero
	// game-changers and no real combo support doesn't get lifted on
	// a false-positive curated-combo match.
	if hasWinningCombo && bracket < 4 {
		bracket = 4
		label = "Optimized"
		addAdjustment("Winning-combo floor", "floor",
			fmt.Sprintf("lifted to B4: 2-card categorical-win combo present (was B%d) — WotC carveout",
				preFloorBracket))
	}
	// Tuned-redundancy floor: many distinct finisher lines + fast-mana density
	// is operationally B4 regardless of GC count.
	if tunedRedundancy && bracket < 4 {
		bracket = 4
		label = "Optimized"
		addAdjustment("Tuned-redundancy floor", "floor",
			fmt.Sprintf("lifted to Bracket 4 — %d different ways to close the game backed by %d cheap mana producers (CMC 2 or less) means this deck reliably draws both a finisher and the mana to cast it on the same turn. That \"finisher + mana on curve\" reliability is what makes a deck play at Bracket 4, even without a single standout card (raw score was B%d before this floor).",
				finisherCount, ctx.fastManaCount, preFloorBracket))
	}

	rationale.FinalBracket = bracket
	rationale.FinalLabel = label
	return bracket, label, rationale
}

