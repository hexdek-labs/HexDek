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
	Bracket           int
	BracketLabel      string
	PlaysLike         int
	PlaysLikeLabel    string
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
	instantSorcPct float64
	creaturePct    float64
	topCreatureTypePct float64
	sacrificeCount int
	deathTriggers  int
	graveyardCount int
	selfMillCount  int
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
	spellCopyCount int
	landfallCount  int
	counterCount   int // +1/+1 counter / proliferate cards
	enchantmentPct float64
	lifegainCount  int
	blinkCount     int
	artifactCount  int
	extraCombatCount int
	planeswalkerCount int
	millOppCount   int // opponent-targeting mill
	discardForceCount int
	// R60 new-archetype counters
	pillowfortCount     int // attack-tax / damage-prevention cards (Propaganda, Sphere of Safety, Solitary Confinement)
	groupSlugCount      int // passive damage-to-opponents triggers (Manabarbs, Pyrostatic Pillar, Underworld Dreams)
	damageRedirectCount int // "dealt damage, it deals" reflectors + redirect effects (Stuffy Doll, Boros Reckoner, Pariah)
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
		Require: func(ctx *classifyContext) bool {
			return ctx.roleRatios[RoleRemoval]+ctx.roleRatios[RoleBoardWipe]+ctx.roleRatios[RoleCounterspell] >= 0.15 &&
				ctx.roleRatios[RoleDraw] >= 0.10
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
		Require: func(ctx *classifyContext) bool {
			return ctx.instantSorcPct >= 0.60 && ctx.spellCopyCount >= 1
		},
	},
	{
		Name: "Tribal",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.12, RoleDraw: 0.08, RoleRamp: 0.08, RoleRemoval: 0.06,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.creaturePct >= 0.35 && ctx.topCreatureTypePct >= 0.30
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
		// Require gate strengthened: pre-r60 a Bruvac-style pure-mill
		// deck (8+ mill payoffs, 0 reanimate spells) would pass the
		// graveyardCount + selfMillCount gates and false-positive into
		// Reanimator. Adding a 5% RoleRecursion floor (≈5 recursion
		// pieces in a 99-card deck) makes the gate genuinely
		// reanimator-shape-aware. Mill decks without recursion bodies
		// now fall through to Selfmill / Midrange.
		Require: func(ctx *classifyContext) bool {
			return ctx.graveyardCount >= 6 &&
				ctx.selfMillCount >= 2 &&
				ctx.roleRatios[RoleRecursion] >= 0.05
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
		Require: func(ctx *classifyContext) bool {
			return ctx.blinkCount >= 6
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
		Require: func(ctx *classifyContext) bool {
			return ctx.planeswalkerCount >= 8
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
	ac.Bracket, ac.BracketLabel, ac.BracketRationale = estimateBracket(ctx, report, ac.Primary)
	ac.PlaysLike, ac.PlaysLikeLabel = estimatePlaysLike(ctx, report)
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

	nonlandTotal := 0
	instantSorcCount := 0
	creatureCount := 0
	enchantmentCount := 0
	creatureTypes := map[string]int{}

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
		if containsAny(ot, "gain life", "whenever you gain life", "lifelink") {
			ctx.lifegainCount += qp.Qty
		}
		if qp.Profile.IsBlinker || containsAny(ot, "exile, then return", "flicker", "exile target creature you control, then return") {
			ctx.blinkCount += qp.Qty
		}
		if qp.Profile.IsExtraCombat || containsAny(ot, "additional combat", "extra combat") {
			ctx.extraCombatCount += qp.Qty
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

		if qp.Profile.CMC <= 2 {
			for _, r := range qp.Profile.Produces {
				if r == ResMana {
					ctx.fastManaCount += qp.Qty
					break
				}
			}
		}
	}

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

func estimateBracket(ctx *classifyContext, report *FreyaReport, primaryArchetype string) (int, string, *BracketRationale) {
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
	hasWinningCombo := trueInfCount >= 1
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
				fmt.Sprintf("capped at B2: no Game Changers and no true-infinite combo (was B%d on raw score)", preCeilingBracket))
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
	// Tuned-redundancy floor: many distinct finisher lines + fast-mana density
	// is operationally B4 regardless of GC count.
	if tunedRedundancy && bracket < 4 {
		bracket = 4
		label = "Optimized"
		addAdjustment("Tuned-redundancy floor", "floor",
			fmt.Sprintf("lifted to B4: %d finishers + %d fast-mana pieces (was B%d)",
				finisherCount, ctx.fastManaCount, preFloorBracket))
	}

	rationale.FinalBracket = bracket
	rationale.FinalLabel = label
	return bracket, label, rationale
}

// estimatePlaysLike determines what bracket a deck PERFORMS at based on
// mechanical signals: win condition consistency, speed, redundancy, and
// strategy coherence. This ignores card pedigree (Game Changers) and
// focuses on how the deck actually plays.
func estimatePlaysLike(ctx *classifyContext, report *FreyaReport) (int, string) {
	score := 0

	// Win line density — more lines = more consistent closing power
	winLines := 0
	if report.WinLines != nil {
		winLines = len(report.WinLines.WinLines)
	}
	if winLines >= 5 {
		score += 3
	} else if winLines >= 2 {
		score += 2
	} else if winLines >= 1 {
		score += 1
	}

	// Combo density — true infinites are the strongest signal
	trueInf := len(report.TrueInfinites)
	if trueInf >= 3 {
		score += 3
	} else if trueInf >= 1 {
		score += 2
	}
	if ctx.comboCount >= 5 {
		score += 2
	} else if ctx.comboCount >= 2 {
		score += 1
	}

	// Speed — low CMC decks execute faster
	if ctx.avgCMC < 2.0 {
		score += 3
	} else if ctx.avgCMC < 2.5 {
		score += 2
	} else if ctx.avgCMC < 3.0 {
		score += 1
	} else if ctx.avgCMC > 4.0 {
		score -= 1
	}

	// Tutor consistency — ability to find win conditions
	if ctx.tutorDensity >= 0.12 {
		score += 3
	} else if ctx.tutorDensity >= 0.08 {
		score += 2
	} else if ctx.tutorDensity >= 0.04 {
		score += 1
	}

	// Fast mana — acceleration matters for "plays like"
	if ctx.fastManaCount >= 8 {
		score += 2
	} else if ctx.fastManaCount >= 4 {
		score += 1
	}

	// Interaction density — counterspells + removal
	if ctx.roleRatios[RoleCounterspell] >= 0.08 {
		score += 2
	} else if ctx.roleRatios[RoleCounterspell] >= 0.04 {
		score += 1
	}

	// Alternate win conditions that bypass normal combat
	// (poison, infect, commander damage voltron, mill, etc.)
	hasAltWin := false
	// Check commander oracle text first
	if ctx.oracle != nil && report.Commander != "" {
		entry := ctx.oracle.lookup(report.Commander)
		if entry != nil {
			ot := strings.ToLower(entry.OracleText)
			if ot == "" && len(entry.CardFaces) > 0 {
				ot = strings.ToLower(entry.CardFaces[0].OracleText)
			}
			if strings.Contains(ot, "poison counter") ||
				strings.Contains(ot, "infect") ||
				strings.Contains(ot, "you win the game") ||
				strings.Contains(ot, "loses the game") ||
				strings.Contains(ot, "commander damage") {
				hasAltWin = true
			}
		}
	}
	// Check cards in the 99
	if !hasAltWin {
		for _, qp := range ctx.qtyProfiles {
			if ctx.oracle != nil {
				entry := ctx.oracle.lookup(qp.Profile.Name)
				if entry != nil {
					ot := strings.ToLower(entry.OracleText)
					if strings.Contains(ot, "poison counter") ||
						strings.Contains(ot, "infect") ||
						strings.Contains(ot, "you win the game") ||
						strings.Contains(ot, "loses the game") {
						hasAltWin = true
						break
					}
				}
			}
		}
	}
	if hasAltWin {
		score += 2
	}

	var bracket int
	var label string
	switch {
	case score >= 14:
		bracket = 5
		label = "cEDH"
	case score >= 10:
		bracket = 4
		label = "Optimized"
	case score >= 6:
		bracket = 3
		label = "Upgraded"
	case score >= 3:
		bracket = 2
		label = "Core"
	default:
		bracket = 1
		label = "Exhibition"
	}

	return bracket, label
}
