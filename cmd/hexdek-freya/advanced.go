package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// 1. Mana base grading — taplands, fetches, utility lands, overall grade.
// ---------------------------------------------------------------------------

func computeManaBaseGrade(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if oracle == nil {
		return
	}

	score := 100 // start at A+, deduct for issues

	for _, p := range report.Profiles {
		if !p.IsLand {
			continue
		}

		entry := oracle.lookup(p.Name)
		if entry == nil {
			continue
		}
		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		tl := strings.ToLower(p.TypeLine)

		if containsAny(ot, "enters tapped", "enters the battlefield tapped") &&
			!strings.Contains(ot, "you may pay") &&
			!strings.Contains(ot, "unless") {
			dp.TaplandCount++
		}

		if strings.Contains(ot, "search your library") && strings.Contains(tl, "land") &&
			!strings.Contains(tl, "creature") {
			dp.FetchCount++
		}

		if len(p.LandColors) == 0 || (len(p.LandColors) == 1 && p.LandColors[0] == "C") {
			if !containsAny(strings.ToLower(p.Name), "plains", "island", "swamp", "mountain", "forest", "wastes") {
				dp.UtilityLandCount++
			}
		}
	}

	// Deductions
	if dp.TaplandCount > 5 {
		penalty := (dp.TaplandCount - 5) * 5
		score -= penalty
		dp.ManaBaseNotes = append(dp.ManaBaseNotes,
			fmt.Sprintf("%d taplands slowing you down — consider upgrading %d to untapped duals",
				dp.TaplandCount, dp.TaplandCount-5))
	} else if dp.TaplandCount > 3 {
		score -= 5
	}

	if dp.UtilityLandCount > 5 {
		penalty := (dp.UtilityLandCount - 5) * 3
		score -= penalty
		dp.ManaBaseNotes = append(dp.ManaBaseNotes,
			fmt.Sprintf("%d utility lands eating color slots — may cause color screw", dp.UtilityLandCount))
	}

	if dp.FetchCount >= 5 {
		score += 10
		dp.ManaBaseNotes = append(dp.ManaBaseNotes,
			fmt.Sprintf("%d fetchlands providing excellent color fixing", dp.FetchCount))
	}

	if report.Stats != nil && len(report.Stats.ColorGaps) > 0 {
		score -= len(report.Stats.ColorGaps) * 10
	}

	switch {
	case score >= 90:
		dp.ManaBaseGrade = "A"
	case score >= 75:
		dp.ManaBaseGrade = "B"
	case score >= 60:
		dp.ManaBaseGrade = "C"
	case score >= 45:
		dp.ManaBaseGrade = "D"
	default:
		dp.ManaBaseGrade = "F"
	}
}

// ---------------------------------------------------------------------------
// 2. Threat assessment — what specific hosers does this deck fear?
// ---------------------------------------------------------------------------

type hoserMapping struct {
	Condition string
	Hoser     string
	Reason    string
	// Severity grades the hoser-vs-condition matchup, not the hoser
	// itself. 3=critical (engine-killing or hard lock), 2=major
	// (one-shot wipe / tax / static drag), 1=minor (chip damage,
	// avoidable). Used to sort VulnerableTo descending so the most
	// dangerous matchups surface first and to tag critical entries
	// with a "(critical)" prefix in the report.
	Severity int
}

// Severity constants. Names align with the comment on hoserMapping.Severity
// so future entries don't have to puzzle out which int means what.
const (
	hoserSeverityMinor    = 1
	hoserSeverityMajor    = 2
	hoserSeverityCritical = 3
)

var hoserDB = []hoserMapping{
	// ── Existing conditions (R59 and earlier) ──
	{"graveyard_heavy", "Rest in Peace", "exiles your graveyard engine", hoserSeverityCritical},
	{"graveyard_heavy", "Leyline of the Void", "prevents your graveyard from filling", hoserSeverityCritical},
	{"graveyard_heavy", "Dauthi Voidwalker", "exiles your dying creatures", hoserSeverityMajor},
	{"artifact_heavy", "Collector Ouphe", "shuts down your artifact mana and combo pieces", hoserSeverityCritical},
	{"artifact_heavy", "Stony Silence", "locks your artifact activations", hoserSeverityCritical},
	{"artifact_heavy", "Vandalblast", "one-sided artifact wipe", hoserSeverityMajor},
	{"creature_heavy", "Cyclonic Rift", "bounces your entire board", hoserSeverityMajor},
	{"creature_heavy", "Toxic Deluge", "uncounterable creature wipe", hoserSeverityMajor},
	{"creature_heavy", "Elesh Norn, Grand Cenobite", "shrinks your board while pumping theirs", hoserSeverityMajor},
	{"combo_heavy", "Rule of Law", "locks you to one spell per turn", hoserSeverityCritical},
	{"combo_heavy", "Drannith Magistrate", "prevents casting from non-hand zones", hoserSeverityCritical},
	{"combo_heavy", "Stifle", "counters your critical triggers", hoserSeverityMajor},
	{"token_heavy", "Massacre Wurm", "kills tokens and drains you", hoserSeverityCritical},
	{"token_heavy", "Rakdos Charm", "each creature deals 1 to you", hoserSeverityMinor},
	{"enchantment_heavy", "Aura Shards", "destroys enchantments on creature ETB", hoserSeverityCritical},
	{"enchantment_heavy", "Back to Nature", "instant-speed enchantment wipe", hoserSeverityMajor},
	{"tutor_heavy", "Opposition Agent", "steals your tutored cards", hoserSeverityCritical},
	{"tutor_heavy", "Aven Mindcensor", "limits your searches to top 4", hoserSeverityMajor},
	{"lifegain", "Erebos, God of the Dead", "prevents your lifegain", hoserSeverityMajor},
	{"lifegain", "Sulfuric Vortex", "prevents lifegain and pressures life total", hoserSeverityMajor},
	{"etb_heavy", "Torpor Orb", "shuts down all your ETB triggers", hoserSeverityCritical},
	{"etb_heavy", "Hushbringer", "prevents ETB and death triggers", hoserSeverityCritical},
	{"spellslinger", "Deafening Silence", "limits noncreature spells to one per turn", hoserSeverityCritical},
	{"spellslinger", "Thalia, Guardian of Thraben", "taxes your noncreature spells", hoserSeverityMajor},
	{"land_ramp", "Blood Moon", "turns your nonbasics into Mountains", hoserSeverityMajor},
	{"land_ramp", "Back to Basics", "taps your nonbasics", hoserSeverityMajor},

	// ── R60 expansion: 5 new conditions, 11 new entries ──
	// Reanimator: graveyard_heavy fires for any self-mill or recursion,
	// but reanimator decks specifically fear instant-speed graveyard
	// exile that can deny a single key target mid-cast. Bojuka Bog +
	// Faerie Macabre are colorless / free and slot into any deck.
	{"reanimator", "Faerie Macabre", "instant-speed graveyard exile of two cards for free", hoserSeverityCritical},
	{"reanimator", "Bojuka Bog", "colorless one-sided graveyard wipe — slots into any mana base", hoserSeverityCritical},
	{"reanimator", "Soul-Guide Lantern", "exiles your graveyard for {1} and replaces itself", hoserSeverityMajor},
	// Storm-explicit: the existing spellslinger condition covers
	// generic noncreature-spell decks. Storm finishers have unique
	// hosers that lock the cast chain itself, not just per-turn count.
	{"storm_explicit", "Eidolon of Rhetoric", "1 spell per turn — kills storm cast chain", hoserSeverityCritical},
	{"storm_explicit", "Damping Sphere", "additional cost {1} per spell after the first, snowballs storm cost", hoserSeverityCritical},
	// Commander-centric: dp.IsCommanderCentric is detected for Voltron /
	// commander-engine decks (e.g. Uril, Yuriko, Sythis). They survive
	// removal because they can recast — but commander-altering enchantments
	// neutralize without sending the commander to the zone where it can be
	// recast cheaply.
	{"commander_centric", "Imprisoned in the Moon", "turns commander into a colorless land — survives commander tax", hoserSeverityCritical},
	{"commander_centric", "Song of the Dryads", "turns commander into a forest — same shape, removed-by-enchantment-only", hoserSeverityCritical},
	{"commander_centric", "Darksteel Mutation", "turns commander into an indestructible 0/1 — can't even die to commander tax", hoserSeverityCritical},
	// Counters-matter: +1/+1 counters / proliferate strategies.
	// Solemnity is the hard lock (counters can't enter); Hex Parasite
	// is a flexible removal piece that scales with counter count.
	{"counters_matter", "Solemnity", "no +1/+1 counters can enter — turns off the entire deck", hoserSeverityCritical},
	{"counters_matter", "Hex Parasite", "removes counters at instant speed for {1} each", hoserSeverityMajor},
	// Wheels: Wheel of Fortune / Windfall / Time Spiral / Echo of Eons
	// decks. Notion Thief and Hullbreacher convert every wheel into a
	// gift to the deck's opponents — catastrophic when the deck's
	// whole engine is "draw 7s".
	{"wheels", "Notion Thief", "wheel hands replace the wheeler's draws — your wheels become opponent tutors", hoserSeverityCritical},
	{"wheels", "Hullbreacher", "same shape — your wheels generate Treasures for the opponent instead of cards for you", hoserSeverityCritical},

	// ── R60 round 2: 10 new entries across 6 existing conditions + 1 new condition ──
	// Enchantment-heavy: the existing Aura Shards + Back to Nature pair
	// covered creature-ETB and instant wipes. Force of Vigor adds the
	// FREE 2-for-1 angle (mass enchantress decks fold to free hate);
	// Pernicious Deed adds the scaling-X wipe that doubles as artifact
	// removal so Enchantress decks running artifact mana eat it twice.
	{"enchantment_heavy", "Force of Vigor", "free instant 2-for-1 — destroys two artifacts/enchantments at no mana cost", hoserSeverityCritical},
	{"enchantment_heavy", "Pernicious Deed", "scaling X-cost one-sided wipe of all artifacts and enchantments", hoserSeverityMajor},
	// Artifact-heavy: Collector Ouphe / Stony Silence are the green/white
	// hatebears; missing was the colorless cheap-artifact equivalent
	// (Null Rod slots into any mana base), the static-tutor planeswalker
	// lock (Karn, the Great Creator), and the green wipe that also
	// punishes enchantment-heavy artifact decks (Bane of Progress).
	{"artifact_heavy", "Null Rod", "colorless 1-cost Stony Silence — slots into any deck and shuts down artifact mana", hoserSeverityCritical},
	{"artifact_heavy", "Karn, the Great Creator", "static lock on opp artifact activations + tutors more stax pieces from sideboard", hoserSeverityCritical},
	{"artifact_heavy", "Bane of Progress", "ETB wipe scaling with permanents you control — kills artifacts AND enchantments", hoserSeverityMajor},
	// Lifegain: Erebos / Sulfuric Vortex prevent the gain. Tainted Remedy
	// FLIPS it — every "you gain N life" becomes "you lose N life",
	// which against a lifegain deck reads as a one-card kill.
	{"lifegain", "Tainted Remedy", "inverts every lifegain trigger into life loss — turns the deck's engine against itself", hoserSeverityCritical},
	// Token-heavy: existing Massacre Wurm (drains) and Rakdos Charm
	// (ping). Pyroclasm covers the cheap 2-mana mass-2-damage sweep
	// that kills the X/1 token board in a single card.
	{"token_heavy", "Pyroclasm", "2-mana 2-damage to each creature — one-card token board wipe", hoserSeverityMajor},
	// Combo-heavy: Rule of Law / Drannith Magistrate / Stifle cover the
	// per-turn limit + cast-zone lock + trigger counter angles. Missing
	// were the graveyard-AND-library cast-source lock (Grafdigger's Cage,
	// which kills Underworld Breach, Yawgmoth's Will, Eldritch Evolution,
	// Birthing Pod, AND Cascade-style combos in a single 1-cost artifact)
	// and the activated-ability-key lock (Pithing Needle on a Thassa's
	// Oracle / Walking Ballista / Isochron Scepter).
	{"combo_heavy", "Grafdigger's Cage", "shuts down graveyard recursion AND library tutors-to-cast — kills Breach, Birthing Pod, Eldritch Evolution in one card", hoserSeverityCritical},
	{"combo_heavy", "Pithing Needle", "names a key activated combo piece — Walking Ballista, Isochron Scepter, Thassa's Oracle activations all stop", hoserSeverityMajor},
	// Wheels: Notion Thief + Hullbreacher flipped wheels for the
	// wheel-controller. Narset, Parter of Veils does the same against
	// the wheel-CASTER from the opposite angle — wheels "draw 7" become
	// "discard hand, draw 1", which catastrophically asymmetric.
	{"wheels", "Narset, Parter of Veils", "limits each opponent's draws to 1 per turn — your wheels turn into mass-discard for one card replacement", hoserSeverityCritical},

	// ── R60 round 2 new condition: extra_turns ──
	// Time Walk / Time Warp / Nexus / Capture of Jingzhou decks (Yennett,
	// Yuriko-extra-turns, Narset Enlightened Master, Wanderwine Prophets).
	// Stranglehold is the canonical hard counter: "If an opponent would
	// take an extra turn, instead they don't" — and as a bonus also
	// shuts down opponent tutors.
	{"extra_turns", "Stranglehold", "extra turns simply don't happen — also locks opponent searches as collateral", hoserSeverityCritical},
}

func computeThreatAssessment(dp *DeckProfile, report *FreyaReport) {
	conditions := map[string]bool{}

	if report.Roles != nil {
		rolePct := func(r RoleTag) float64 {
			if report.Roles.TotalCards == 0 {
				return 0
			}
			return float64(report.Roles.RoleCounts[r]) / float64(report.Roles.TotalCards)
		}
		if rolePct(RoleCombo) >= 0.08 || len(report.TrueInfinites)+len(report.Determined) >= 3 {
			conditions["combo_heavy"] = true
		}
	}

	graveyardCards := 0
	artifactCards := 0
	creatureCards := 0
	tokenCards := 0
	enchantmentCards := 0
	etbCards := 0
	rampLandCards := 0
	for _, p := range report.Profiles {
		if p.IsLand {
			continue
		}
		tl := strings.ToLower(p.TypeLine)
		if p.IsRecursion || containsAny(strings.Join(p.Effects, ","), "self_mill", "mass_reanimate") {
			graveyardCards++
		}
		if strings.Contains(tl, "artifact") {
			artifactCards++
		}
		if strings.Contains(tl, "creature") {
			creatureCards++
		}
		if strings.Contains(tl, "enchantment") {
			enchantmentCards++
		}
		if p.HasValueETB || profileHasTrigger(p, "etb") {
			etbCards++
		}
		for _, e := range p.Effects {
			if e == "land_fetch" {
				rampLandCards++
				break
			}
		}
		for _, r := range p.Produces {
			if r == ResToken {
				tokenCards++
				break
			}
		}
	}

	if graveyardCards >= 8 {
		conditions["graveyard_heavy"] = true
	}
	if artifactCards >= 12 {
		conditions["artifact_heavy"] = true
	}
	if creatureCards >= 25 {
		conditions["creature_heavy"] = true
	}
	if tokenCards >= 8 {
		conditions["token_heavy"] = true
	}
	if enchantmentCards >= 10 {
		conditions["enchantment_heavy"] = true
	}
	if etbCards >= 10 {
		conditions["etb_heavy"] = true
	}
	if rampLandCards >= 6 {
		conditions["land_ramp"] = true
	}

	if report.NonLandTutorCount >= 6 {
		conditions["tutor_heavy"] = true
	}

	arch := strings.ToLower(dp.PrimaryArchetype)
	if containsAny(arch, "lifegain") {
		conditions["lifegain"] = true
	}
	if containsAny(arch, "spellslinger", "storm") {
		conditions["spellslinger"] = true
	}

	// ── R60: 5 new condition detectors ──
	// reanimator: explicit reanimator archetype OR graveyard_heavy with
	// enough mass-reanimate spells (Living Death / Patriarch's Bidding /
	// Balthor the Defiled) to credibly cast multiple reanimate turns.
	if containsAny(arch, "reanimator") {
		conditions["reanimator"] = true
	} else if conditions["graveyard_heavy"] {
		massReanimate := 0
		for _, p := range report.Profiles {
			if containsAny(strings.Join(p.Effects, ","), "mass_reanimate") {
				massReanimate++
			}
		}
		if massReanimate >= 2 {
			conditions["reanimator"] = true
		}
	}
	// storm_explicit: storm archetype, OR has a storm_finisher win-line
	// class. Note this fires IN ADDITION to spellslinger, not instead —
	// storm decks are still spellslinger and want both layers of hosers
	// surfaced.
	if containsAny(arch, "storm") {
		conditions["storm_explicit"] = true
	} else if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			if wl.Class == ComboClassStormFinisher {
				conditions["storm_explicit"] = true
				break
			}
		}
	}
	// commander_centric: flag set by computeOpeningHandSim →
	// detectCommanderCentric. Voltron / engine-commander decks.
	if dp.IsCommanderCentric {
		conditions["commander_centric"] = true
	}
	// counters_matter: archetype name OR commander themes name "counters"
	// OR a notable fraction of profiles produce +1/+1 counters. The
	// fraction threshold is loose (5+ counter-producers) since most
	// decks running counters revolve around them, not dabble.
	if containsAny(arch, "counter") {
		conditions["counters_matter"] = true
	} else if dp.CommanderThemes != nil {
		for _, theme := range dp.CommanderThemes {
			if strings.Contains(strings.ToLower(theme), "counter") {
				conditions["counters_matter"] = true
				break
			}
		}
	}
	if !conditions["counters_matter"] {
		counterProducers := 0
		for _, p := range report.Profiles {
			if containsAny(strings.Join(p.Effects, ","), "counter_add", "counter_move", "proliferate") {
				counterProducers++
			}
		}
		if counterProducers >= 5 {
			conditions["counters_matter"] = true
		}
	}
	// wheels: archetype name OR presence of 2+ canonical wheel spells.
	// Hosers are catastrophic enough (Notion Thief flips the engine)
	// that even a "soft wheel" sub-theme deserves the warning.
	if containsAny(arch, "wheel") {
		conditions["wheels"] = true
	} else {
		wheelHits := 0
		wheelCards := map[string]bool{
			"Wheel of Fortune": true, "Windfall": true, "Time Spiral": true,
			"Echo of Eons": true, "Wheel of Misfortune": true,
			"Magus of the Wheel": true, "Whispering Madness": true,
			"Day's Undoing": true, "Timetwister": true, "Memory Jar": true,
		}
		for _, p := range report.Profiles {
			if wheelCards[p.Name] {
				wheelHits++
			}
		}
		if wheelHits >= 2 {
			conditions["wheels"] = true
		}
	}
	// extra_turns: archetype name OR 2+ canonical Time-Walk-family spells.
	// Threshold matches wheels (2) — a single Temporal Manipulation in
	// an otherwise-blue deck is incidental; two or more is a deliberate
	// engine choice. Stranglehold deserves the warning because it's a
	// total lock on the deck's central play pattern.
	if containsAny(arch, "extra turn") || containsAny(arch, "extra_turn") {
		conditions["extra_turns"] = true
	} else {
		turnHits := 0
		turnCards := map[string]bool{
			"Time Walk": true, "Time Warp": true, "Temporal Manipulation": true,
			"Capture of Jingzhou": true, "Walk the Aeons": true,
			"Temporal Trespass": true, "Time Stretch": true,
			"Nexus of Fate": true, "Karn's Temporal Sundering": true,
			"Expropriate": true, "Final Fortune": true, "Last Chance": true,
			"Notorious Throng": true, "Stitch in Time": true,
			"Temporal Mastery": true, "Beacon of Tomorrows": true,
			"Sage of Hours": true,
		}
		for _, p := range report.Profiles {
			if turnCards[p.Name] {
				turnHits++
			}
		}
		if turnHits >= 2 {
			conditions["extra_turns"] = true
		}
	}

	// Collect matches preserving Severity so we can sort
	// most-dangerous-first and tag critical entries inline. The dedupe
	// pass keeps an earlier match's Severity if the same hoser appears
	// under multiple conditions (e.g. Rest in Peace could conceivably
	// be added to both graveyard_heavy AND reanimator in the future) —
	// hoserDB ordering decides the canonical condition.
	type matched struct {
		Hoser    string
		Reason   string
		Severity int
	}
	seen := map[string]bool{}
	var hits []matched
	for _, h := range hoserDB {
		if !conditions[h.Condition] || seen[h.Hoser] {
			continue
		}
		seen[h.Hoser] = true
		hits = append(hits, matched{Hoser: h.Hoser, Reason: h.Reason, Severity: h.Severity})
	}
	// Stable sort by severity desc — entries of equal severity keep
	// their hoserDB ordering, which is grouped by condition for
	// readability.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Severity > hits[j].Severity })
	for _, h := range hits {
		if h.Severity >= hoserSeverityCritical {
			dp.VulnerableTo = append(dp.VulnerableTo,
				fmt.Sprintf("%s (critical) — %s", h.Hoser, h.Reason))
		} else {
			dp.VulnerableTo = append(dp.VulnerableTo,
				fmt.Sprintf("%s — %s", h.Hoser, h.Reason))
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Opening hand simulation — Monte Carlo mulligan analysis.
// ---------------------------------------------------------------------------

func computeOpeningHandSim(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.TotalCards < 40 {
		return
	}

	detectCommanderCentric(dp, report, oracle)

	rng := rand.New(rand.NewSource(42))
	// mullRng is a separate, independently-seeded RNG used ONLY for
	// the Vancouver free-mulligan re-shuffles. Keeping the mulligan
	// stream isolated from `rng` preserves bit-stability of the
	// no-mulligan KeepableHandPct / AvgTurnToFourMana metrics — the
	// first-hand and turns-to-N draws still consume `rng` in the
	// same sequence as pre-r60, regardless of how often the
	// conditional re-shuffle fires. The mulligan seed is offset from
	// the primary so the two streams don't correlate (which would
	// bias the per-trial pair toward identical hands and depress
	// the observed mulligan uplift below the 1-(1-p)^2 expectation).
	mullRng := rand.New(rand.NewSource(43))
	trials := 10000
	keepable := 0
	keepableAdjusted := 0
	keepableFreeMull := 0
	keepableAdjustedFreeMull := 0
	totalTurnsToFour := 0.0
	totalTurnsToCmdr := 0.0
	validTrials := 0
	validCmdrTrials := 0

	const (
		flagLand    = 1 << 0
		flagRamp    = 1 << 1
		flagSynergy = 1 << 2
		flagAction  = 1 << 3
	)

	// Build a flag deck with one entry per real card slot (so basic lands
	// count toward lands-in-hand). report.Profiles only carries unique
	// nonbasic cards, so we infer the basic-land slots from the gap
	// between report.LandCount and the nonbasic land count in Profiles.
	synergySet := buildSynergyNameSet(dp, report, oracle)
	actionSet := buildActionNameSet(report)
	nonbasicLandSlots := 0
	deckFlags := make([]uint8, 0, report.TotalCards)
	for _, p := range report.Profiles {
		f := uint8(0)
		if p.IsLand {
			f |= flagLand
			nonbasicLandSlots++
		} else {
			for _, r := range p.Produces {
				if r == ResMana {
					f |= flagRamp
					break
				}
			}
			lname := strings.ToLower(p.Name)
			if synergySet[lname] {
				f |= flagSynergy
			}
			if actionSet[lname] {
				f |= flagAction
			}
		}
		deckFlags = append(deckFlags, f)
	}
	// Pad with basic land slots so total slot count equals deck size.
	basicLandSlots := report.LandCount - nonbasicLandSlots
	if basicLandSlots < 0 {
		basicLandSlots = 0
	}
	for i := 0; i < basicLandSlots; i++ {
		deckFlags = append(deckFlags, flagLand)
	}
	// Pad any remaining gap with neutral non-action slots (unresolved cards).
	for len(deckFlags) < report.TotalCards {
		deckFlags = append(deckFlags, 0)
	}

	cmdrCMC := dp.CommanderCMC
	if cmdrCMC <= 0 {
		cmdrCMC = 4
	}

	// Separate slice for the mulligan re-shuffles so the primary
	// deckFlags slice's permutation trajectory across trials stays
	// identical to pre-r60. Fisher-Yates produces a state-dependent
	// permutation given a fixed swap sequence — if the mulligan
	// helper mutated the shared slice, the NEXT trial's first-hand
	// shuffle would start from a different permutation and produce
	// a different keepability outcome, drifting KeepableHandPct
	// away from its pre-r60 value. Copy once up front.
	mullDeckFlags := make([]uint8, len(deckFlags))

	// shuffleAndEvalHand shuffles the supplied slice using the
	// supplied RNG, inspects the top 7, and returns
	// (standardKeepable, adjustedKeepable) plus the landsInHand +
	// rampInHand counts. The counts are returned so the downstream
	// turns-to-N estimation can use the first-hand state from the
	// FIRST call without re-counting; the helper is invoked up to
	// twice per trial for the Vancouver free-mulligan path with
	// separate RNGs AND separate slices to keep the no-mulligan
	// stream bit-stable.
	shuffleAndEvalHand := func(slice []uint8, r *rand.Rand) (stdKeep, adjKeep bool, landsInHand, rampInHand int) {
		for i := len(slice) - 1; i > 0; i-- {
			j := r.Intn(i + 1)
			slice[i], slice[j] = slice[j], slice[i]
		}
		var synergyInHand, actionInHand int
		for i := 0; i < 7; i++ {
			f := slice[i]
			if f&flagLand != 0 {
				landsInHand++
			}
			if f&flagRamp != 0 {
				rampInHand++
			}
			if f&flagSynergy != 0 {
				synergyInHand++
			}
			if f&flagAction != 0 {
				actionInHand++
			}
		}
		landsOK := landsInHand >= 2 && landsInHand <= 5
		// Standard keepable: 2-5 lands AND at least one threat /
		// interaction / draw / combo piece.
		stdKeep = landsOK && actionInHand >= 1
		// Commander-adjusted keepable: 2-5 lands plus EITHER an
		// action card, a ramp piece, a commander-synergy enabler,
		// or enough lands to hit commander CMC purely by land
		// drops.
		if landsOK {
			naturalReach := landsInHand >= cmdrCMC
			adjKeep = actionInHand >= 1 || rampInHand >= 1 || synergyInHand >= 1 || naturalReach
		}
		return
	}

	for t := 0; t < trials; t++ {
		stdKeep1, adjKeep1, landsInHand, rampInHand := shuffleAndEvalHand(deckFlags, rng)

		if stdKeep1 {
			keepable++
		}
		if adjKeep1 {
			keepableAdjusted++
		}

		// Vancouver free-mulligan: if the first hand isn't keepable,
		// Commander rules allow a single penalty-free re-draw of 7.
		// The trial counts as keepable under the free-mulligan
		// variant if EITHER the initial hand OR the free-mulligan
		// hand passes. We only re-shuffle when the first hand fails,
		// matching real-table behavior (a player who's keeping the
		// first hand doesn't burn the free mulligan).
		//
		// IMPORTANT: the post-mulligan hand reuses the same RNG
		// stream so the simulation stays seed-deterministic. The
		// re-shuffle advances the RNG state past the initial draw's
		// state, so KeepableHandPct (which only cares about the
		// first-shuffle hand) remains bit-stable — the first-hand
		// classification is captured BEFORE the conditional
		// re-shuffle below. Tested in TestOpeningHandSim_NoMulligan
		// FieldIsBitStable.
		stdKeepMull := stdKeep1
		adjKeepMull := adjKeep1
		if !stdKeep1 || !adjKeep1 {
			// Sync the mulligan slice from the primary slice's
			// post-first-shuffle state. The mulligan helper then
			// re-shuffles its own copy, leaving the primary slice
			// untouched for the next trial's first-hand shuffle.
			copy(mullDeckFlags, deckFlags)
			stdKeep2, adjKeep2, _, _ := shuffleAndEvalHand(mullDeckFlags, mullRng)
			if !stdKeepMull && stdKeep2 {
				stdKeepMull = true
			}
			if !adjKeepMull && adjKeep2 {
				adjKeepMull = true
			}
		}
		if stdKeepMull {
			keepableFreeMull++
		}
		if adjKeepMull {
			keepableAdjustedFreeMull++
		}

		// Estimate turns to 4 mana and turns to commander CMC.
		// Always uses the FIRST hand's lands/ramp counts so the
		// estimation reflects the typical accept-or-mulligan
		// player's starting curve, independent of the free-mulligan
		// keepability tracking above.
		if landsInHand >= 2 {
			validTrials++
			mana := 0
			turn := 0
			landDropsLeft := landsInHand
			rampLeft := rampInHand
			drawIdx := 7
			turnToFour := 0
			turnToCmdr := 0

			for (turnToFour == 0 || turnToCmdr == 0) && turn < 12 {
				turn++
				if turn > 1 && drawIdx < len(deckFlags) {
					f := deckFlags[drawIdx]
					drawIdx++
					if f&flagLand != 0 {
						landDropsLeft++
					}
					if f&flagRamp != 0 {
						rampLeft++
					}
				}

				if landDropsLeft > 0 {
					mana++
					landDropsLeft--
				}
				// Play ramp if we have mana for it (assume CMC 2 ramp).
				if rampLeft > 0 && mana >= 2 {
					mana++
					rampLeft--
				}

				if turnToFour == 0 && mana >= 4 {
					turnToFour = turn
				}
				if turnToCmdr == 0 && mana >= cmdrCMC {
					turnToCmdr = turn
				}
			}
			if turnToFour == 0 {
				turnToFour = turn
			}
			totalTurnsToFour += float64(turnToFour)
			if turnToCmdr > 0 {
				totalTurnsToCmdr += float64(turnToCmdr)
				validCmdrTrials++
			}
		}
	}

	dp.KeepableHandPct = float64(keepable) / float64(trials) * 100
	dp.KeepableHandPctAdjusted = float64(keepableAdjusted) / float64(trials) * 100
	dp.KeepableHandPctFreeMull = float64(keepableFreeMull) / float64(trials) * 100
	dp.KeepableHandPctAdjustedFreeMull = float64(keepableAdjustedFreeMull) / float64(trials) * 100
	if validTrials > 0 {
		dp.AvgTurnToFourMana = totalTurnsToFour / float64(validTrials)
	}
	if validCmdrTrials > 0 {
		dp.AvgTurnToCommander = totalTurnsToCmdr / float64(validCmdrTrials)
	}
}

// detectCommanderCentric flags decks whose primary gameplan is the commander
// itself, so the keepable-hand heuristic can be relaxed accordingly.
func detectCommanderCentric(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if oracle == nil || report.Commander == "" {
		return
	}
	cmdr := oracle.lookup(report.Commander)
	if cmdr == nil {
		return
	}
	dp.CommanderCMC = int(cmdr.CMC)

	var reasons []string

	if dp.PrimaryArchetype == "Voltron" {
		reasons = append(reasons, "Voltron archetype")
	}
	if dp.CommanderSynergy >= 0.45 {
		reasons = append(reasons, fmt.Sprintf("%.0f%% commander synergy", dp.CommanderSynergy*100))
	}

	cmdrOT := strings.ToLower(cmdr.OracleText)
	if cmdrOT == "" && len(cmdr.CardFaces) > 0 {
		cmdrOT = strings.ToLower(cmdr.CardFaces[0].OracleText)
	}
	enginePhrases := []string{
		"draw a card", "draw cards", "draw two", "draw three",
		"create a token", "create two", "create x",
		"return target", "return it to the battlefield", "from your graveyard to the battlefield",
		"deals damage to any target", "deals damage equal",
		"add {", "add one mana", "add two mana",
		"search your library",
	}
	engineHits := 0
	for _, phrase := range enginePhrases {
		if strings.Contains(cmdrOT, phrase) {
			engineHits++
		}
	}
	if engineHits >= 2 {
		reasons = append(reasons, "commander supplies core engine")
	}

	if len(reasons) > 0 {
		dp.IsCommanderCentric = true
		dp.CommanderCentricReason = strings.Join(reasons, "; ")
	}
}

// buildActionNameSet returns the set of card names (lowercased) that
// count as "do something this turn" pieces — threats, removal,
// counterspells, board wipes, draw, combo pieces, and tutors.
func buildActionNameSet(report *FreyaReport) map[string]bool {
	out := map[string]bool{}
	if report.Roles == nil {
		return out
	}
	actionRoles := map[RoleTag]bool{
		RoleThreat:       true,
		RoleRemoval:      true,
		RoleBoardWipe:    true,
		RoleCounterspell: true,
		RoleDraw:         true,
		RoleCombo:        true,
		RoleTutor:        true,
	}
	for _, a := range report.Roles.Assignments {
		for _, r := range a.Roles {
			if actionRoles[r] {
				out[strings.ToLower(a.Name)] = true
				break
			}
		}
	}
	return out
}

// buildSynergyNameSet returns the set of card names (lowercased) that
// synergize with the commander's themes — used to count "commander
// enablers" in opening hands.
func buildSynergyNameSet(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) map[string]bool {
	out := map[string]bool{}
	if oracle == nil || len(dp.CommanderThemes) == 0 {
		return out
	}
	themeSet := map[string]bool{}
	for _, t := range dp.CommanderThemes {
		themeSet[t] = true
	}
	for _, p := range report.Profiles {
		if p.IsLand || p.Name == report.Commander {
			continue
		}
		entry := oracle.lookup(p.Name)
		if entry == nil {
			continue
		}
		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		tl := strings.ToLower(p.TypeLine)
		for _, tp := range commanderThemePatterns {
			if !themeSet[tp.Theme] {
				continue
			}
			matched := false
			for _, pat := range tp.Patterns {
				if strings.Contains(ot, pat) || strings.Contains(tl, pat) {
					matched = true
					break
				}
			}
			if matched {
				out[strings.ToLower(p.Name)] = true
				break
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 4. Synergy clusters — groups of cards that amplify each other.
// ---------------------------------------------------------------------------

// clusterRole encodes a card's structural role within a theme. Producers
// create the resource the theme cares about (a token-maker for the
// tokens theme, a sac outlet for the death_value theme), payoffs reward
// the resource appearing (Purphoros for tokens, Blood Artist for
// death_value). A card that satisfies both predicates ("both") slots
// into either side of a pair. "unknown" is the fallback used for
// themes where the producer/payoff dichotomy doesn't apply (landfall,
// spellcast, lifegain — the producer side is too broad to tag cleanly).
//
// R60 round 2 adds Amplifier: the chain-middle role that transforms the
// producer's resource into payoff-triggering events (sac outlet between
// token-maker and dies-trigger; blinker between value-ETB creature and
// etb-rewards-others trigger). Themes that define an amplifier earn a
// chain bonus when all three roles are present — see computeSynergyClusters.
type clusterRole int

const (
	clusterRoleUnknown clusterRole = iota
	clusterRoleProducer
	clusterRolePayoff
	clusterRoleBoth
	clusterRoleAmplifier
)

// classifyClusterRole returns the producer/payoff role for a card
// within a given theme. Themes without a clean dichotomy return
// clusterRoleUnknown, which falls back to plain pair-counting.
func classifyClusterRole(p CardProfile, theme string) clusterRole {
	prod := false
	payoff := false
	switch theme {
	case "tokens":
		for _, r := range p.Produces {
			if r == ResToken {
				prod = true
			}
		}
		// Token payoffs: token_created trigger directly, OR the
		// Purphoros-shape (creature-ETB trigger + opponent-pain trigger
		// — i.e. "whenever a creature ETBs, opponents lose / take
		// damage"). The narrower etb+opponent_pain pair distinguishes
		// the payoff card from generic etb-trigger value engines like
		// Solemn Simulacrum.
		if profileHasTrigger(p, "token_created") ||
			(profileHasTrigger(p, "etb") && profileHasTrigger(p, "opponent_pain")) {
			payoff = true
		}
	case "counters":
		for _, r := range p.Produces {
			if r == ResCounter {
				prod = true
			}
		}
		for _, e := range p.Effects {
			if e == "counter_add" || e == "counter_move" || e == "proliferate" {
				prod = true
			}
		}
		if profileHasTrigger(p, "counter_placed") || profileHasTrigger(p, "counter_matters") {
			payoff = true
		}
	case "death_value":
		// Three-step chain: token-maker (bodies) → sac outlet (converts
		// bodies to death events) → dies/sacrifice trigger (rewards
		// the events). The outlet is the central amplifier and short-
		// circuits the producer/payoff resolution below.
		if p.IsOutlet {
			return clusterRoleAmplifier
		}
		for _, r := range p.Produces {
			if r == ResToken {
				prod = true
			}
		}
		if profileHasTrigger(p, "dies") || profileHasTrigger(p, "sacrifice") {
			payoff = true
		}
	case "etb_value":
		// Three-step chain: HasValueETB creature (worth re-triggering)
		// → blinker (amplifier that re-triggers the ETB) → triggers-on-
		// other-ETB payoff (Soul Warden / Impact Tremors / Purphoros).
		// Note Triggers="etb" only fires for OTHER creatures' ETBs (see
		// analysis.go:386); a card's own ETB is HasValueETB.
		if p.IsBlinker {
			return clusterRoleAmplifier
		}
		if p.HasValueETB {
			prod = true
		}
		if profileHasTrigger(p, "etb") {
			payoff = true
		}
	default:
		// landfall / spellcast / lifegain etc. — no clean dichotomy
		// available from existing tags. Caller falls back to flat
		// pair-count for these themes.
		return clusterRoleUnknown
	}
	switch {
	case prod && payoff:
		return clusterRoleBoth
	case prod:
		return clusterRoleProducer
	case payoff:
		return clusterRolePayoff
	default:
		return clusterRoleUnknown
	}
}

// rolesPairScore returns the weighted pair score between two roles in a
// cluster. Mixed pairs (any two different non-unknown roles) score 2
// because they represent complementary engine sides; same-role pairs
// score 1 because they're redundant copies of the same side. "Both"-
// tagged cards are treated as mixed-with-anything since they satisfy
// either side of the producer/payoff pair. Amplifier pairs with any
// other non-unknown role at 2, since the amplifier is the chain middle
// and bridges either side. "Unknown" pairs (themes without a dichotomy)
// score 1 — the caller is responsible for falling back to flat counting
// when *every* pair is unknown, so this only matters at cluster
// boundaries.
func rolesPairScore(a, b clusterRole) int {
	if a == clusterRoleUnknown || b == clusterRoleUnknown {
		return 1
	}
	if a == clusterRoleBoth || b == clusterRoleBoth {
		return 2
	}
	if a == b {
		return 1
	}
	// Any two distinct non-unknown roles (producer/amplifier/payoff)
	// form a chain pair.
	return 2
}

func computeSynergyClusters(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if oracle == nil || len(report.Profiles) < 10 {
		return
	}

	type cardThemes struct {
		name    string
		profile CardProfile
		themes  map[string]bool
	}

	var cards []cardThemes
	for _, p := range report.Profiles {
		if p.IsLand {
			continue
		}
		themes := map[string]bool{}

		if p.IsOutlet {
			themes["sacrifice"] = true
			// Sac outlets are the chain-amplifier for death_value (R60
			// round 2). Without this the death_value cluster only saw
			// the dies-trigger payoffs, never the conversion engine.
			themes["death_value"] = true
		}
		if profileHasTrigger(p, "dies") || profileHasTrigger(p, "sacrifice") {
			themes["death_value"] = true
		}
		if profileHasTrigger(p, "etb") || p.HasValueETB {
			themes["etb_value"] = true
		}
		if p.IsBlinker {
			themes["blink"] = true
			themes["etb_value"] = true
		}
		for _, r := range p.Produces {
			switch r {
			case ResToken:
				themes["tokens"] = true
			case ResCounter:
				themes["counters"] = true
			case ResCard:
				themes["draw"] = true
			case ResMana:
				themes["mana"] = true
			}
		}
		// Tokens-payoff detection (R60 refinement): cards whose primary
		// shape is "rewarding tokens / creature ETBs" weren't landing in
		// the tokens cluster because they don't Produce ResToken —
		// they only Trigger on it. Purphoros, Impact Tremors, Reckless
		// Fireweaver are the canonical examples; without this, a
		// Krenko-shape token deck's actual win-condition cards stayed
		// invisible to the tokens cluster, only surfacing in etb_value.
		if profileHasTrigger(p, "token_created") ||
			(profileHasTrigger(p, "etb") && profileHasTrigger(p, "opponent_pain")) {
			themes["tokens"] = true
		}
		if p.IsRecursion {
			themes["recursion"] = true
		}
		if profileHasTrigger(p, "cast") {
			themes["spellcast"] = true
		}
		if profileHasTrigger(p, "landfall") {
			themes["landfall"] = true
		}
		if profileHasTrigger(p, "lifegain") || profileHasTrigger(p, "lifeloss") {
			themes["lifegain"] = true
		}
		if profileHasTrigger(p, "counter_matters") || profileHasTrigger(p, "counter_placed") {
			themes["counters"] = true
		}

		if len(themes) > 0 {
			cards = append(cards, cardThemes{name: p.Name, profile: p, themes: themes})
		}
	}

	// R60 round 2: death_value chain-context promotion. The chain is
	// token-maker (producer) → sac outlet (amplifier) → dies-trigger
	// (payoff). The first-pass tagging only put outlets and dies-triggers
	// into death_value; pulling token-makers in too is contextual —
	// only when the rest of the chain is actually present in the deck.
	// Without this guard, every token deck would pollute death_value
	// even with zero sac outlets and zero dies-triggers.
	hasOutlet, hasDiesTrigger := false, false
	for i := range cards {
		if cards[i].profile.IsOutlet {
			hasOutlet = true
		}
		if profileHasTrigger(cards[i].profile, "dies") ||
			profileHasTrigger(cards[i].profile, "sacrifice") {
			hasDiesTrigger = true
		}
	}
	if hasOutlet && hasDiesTrigger {
		for i := range cards {
			for _, r := range cards[i].profile.Produces {
				if r == ResToken {
					cards[i].themes["death_value"] = true
					break
				}
			}
		}
	}

	// Find clusters by theme overlap. We need the per-card profile at
	// pair-scoring time (for role classification) so the cluster member
	// list carries cardThemes refs rather than plain names.
	clusterMembers := map[string][]cardThemes{
		"death_value": {},
		"etb_value":   {},
		"tokens":      {},
		"counters":    {},
		"landfall":    {},
		"spellcast":   {},
		"lifegain":    {},
		// recursion: reanimator / dredge / flashback cards already get
		// themes["recursion"] tagged but had no bucket — surfacing it
		// here lets alt-build suggestions catch "you could lean fully
		// into reanimator" when the rest of the deck is split across
		// engines.
		"recursion": {},
	}

	for _, c := range cards {
		for theme := range c.themes {
			if _, ok := clusterMembers[theme]; ok {
				clusterMembers[theme] = append(clusterMembers[theme], c)
			}
		}
	}

	clusterNames := map[string]string{
		"death_value": "Death Value Package",
		"etb_value":   "ETB Value Package",
		"tokens":      "Token Engine",
		"counters":    "Counters Synergy",
		"landfall":    "Landfall Package",
		"spellcast":   "Spellslinger Package",
		"lifegain":    "Lifegain Engine",
		"recursion":   "Reanimator / Recursion Package",
	}

	for theme, members := range clusterMembers {
		if len(members) < 4 {
			continue
		}

		// Deduplicate by card name. The first occurrence wins (preserves
		// theme-order priority for the display list).
		seen := map[string]bool{}
		deduped := make([]cardThemes, 0, len(members))
		for _, m := range members {
			if seen[m.name] {
				continue
			}
			seen[m.name] = true
			deduped = append(deduped, m)
		}
		if len(deduped) < 4 {
			continue
		}

		// Weighted pair scoring (R60 round 1): producer × payoff pairs
		// count 2 each; same-role or unknown pairs count 1. A balanced
		// engine outscores a same-role pile of the same size. For themes
		// without a clean dichotomy (landfall, spellcast, lifegain —
		// classifyClusterRole returns Unknown), every pair scores 1 and
		// the result reduces to the previous n*(n-1)/2 shape.
		roles := make([]clusterRole, len(deduped))
		for i, m := range deduped {
			roles[i] = classifyClusterRole(m.profile, theme)
		}
		score := 0
		for i := 0; i < len(deduped); i++ {
			for j := i + 1; j < len(deduped); j++ {
				score += rolesPairScore(roles[i], roles[j])
			}
		}

		// Chain-depth bonus (R60 round 2): for themes with a 3-step
		// chain (death_value, etb_value — see classifyClusterRole),
		// reward each complete producer→amplifier→payoff triangle with
		// +3. Counts each card in exactly one role bucket (Both counts
		// as both producer and payoff for chain math). The chain bonus
		// is 0 when no amplifier exists in the cluster, so adding the
		// missing-link card to a 2-role pile gives a discontinuous jump
		// — which matches the deckbuilding reality that a sac outlet
		// finally makes a token+drain pile actually win.
		producers, amps, payoffs := 0, 0, 0
		for _, r := range roles {
			switch r {
			case clusterRoleProducer:
				producers++
			case clusterRoleAmplifier:
				amps++
			case clusterRolePayoff:
				payoffs++
			case clusterRoleBoth:
				producers++
				payoffs++
			}
		}
		chainCount := producers
		if amps < chainCount {
			chainCount = amps
		}
		if payoffs < chainCount {
			chainCount = payoffs
		}
		score += chainCount * 3

		// Build the full member list once, then cap a separate slice
		// for display. AllMembers feeds the structured cluster export
		// (cluster_export.go) so downstream consumers get the complete
		// membership, not just the 8-card preview.
		allMembers := make([]string, 0, len(deduped))
		for _, m := range deduped {
			allMembers = append(allMembers, m.name)
		}
		displayed := allMembers
		if len(displayed) > 8 {
			displayed = displayed[:8]
		}

		dp.SynergyClusters = append(dp.SynergyClusters, SynergyCluster{
			Name:        clusterNames[theme],
			Cards:       displayed,
			Theme:       theme,
			Score:       score,
			MemberCount: len(deduped),
			AllMembers:  allMembers,
		})
	}

	// R60: land_value cluster. The main pass skips lands at line 928,
	// so cycling/slow-fetch lands and the land-payoff package never
	// surface as a deck-shape signal. land_value is computed separately
	// across BOTH lands and nonlands: producer = cycle-land or self-
	// sacrificing/self-mill land (cycle-lands feed the graveyard for
	// the rest of the package); amplifier = a Crucible-style replay
	// engine (play-lands-from-graveyard or extra-land-drop), which is
	// the chain center that turns "cycled land in graveyard" into
	// recurring tempo; payoff = landfall trigger or lands-in-
	// graveyard scaling effect (Tatyova, Aesi, Omnath, Splendid
	// Reclamation, Field of the Dead, Lord Windgrace).
	if cluster := computeLandValueCluster(report); cluster != nil {
		dp.SynergyClusters = append(dp.SynergyClusters, *cluster)
	}

	finalizeClusters(dp)
}

// finalizeClusters prunes sub-MinimumClusterMembers entries, stamps the
// HighDensity flag on clusters at HighDensityClusterFloor (5) or larger,
// then sorts by score desc and caps at 5. Extracted from
// computeSynergyClusters so tests can exercise the prune + flag pass in
// isolation without rebuilding the full theme-tagging pipeline.
func finalizeClusters(dp *DeckProfile) {
	pruned := dp.SynergyClusters[:0]
	for _, sc := range dp.SynergyClusters {
		if sc.MemberCount < MinimumClusterMembers {
			continue
		}
		if sc.MemberCount >= HighDensityClusterFloor {
			sc.HighDensity = true
		}
		pruned = append(pruned, sc)
	}
	dp.SynergyClusters = pruned

	sort.Slice(dp.SynergyClusters, func(i, j int) bool {
		return dp.SynergyClusters[i].Score > dp.SynergyClusters[j].Score
	})

	if len(dp.SynergyClusters) > 5 {
		dp.SynergyClusters = dp.SynergyClusters[:5]
	}
}

// landValuePayoffNames is the curated card-name set of canonical
// lands-matter payoffs that the substring detector below may miss
// when oracle text doesn't include the exact "landfall" / "land
// from your graveyard" anchors. These are name-match anchors, not a
// blocklist — the substring scan still runs and adds cards
// independently.
var landValuePayoffNames = map[string]bool{
	"tatyova, benthic druid":          true,
	"aesi, tyrant of gyre strait":     true,
	"omnath, locus of creation":       true,
	"omnath, locus of rage":           true,
	"lord windgrace":                  true,
	"field of the dead":               true,
	"splendid reclamation":            true,
	"scapeshift":                      true,
	"the gitrog monster":              true,
	"the gitrog, ravenous ride":       true,
	"lotus cobra":                     true,
	"avenger of zendikar":             true,
	"world shaper":                    true,
	"titania, protector of argoth":    true,
	"titania, voice of gaea":          true,
	"emeria, the sky ruin":            true,
	"valakut, the molten pinnacle":    true,
}

// landValueAmplifierNames are the curated replay/extra-drop engines
// that make a graveyard-bound cycled land come back. Substring scan
// still runs in parallel — these catch the cards whose oracle text
// doesn't have the obvious "play lands from your graveyard" anchor.
var landValueAmplifierNames = map[string]bool{
	"crucible of worlds":                  true,
	"ramunap excavator":                   true,
	"the gitrog monster":                  true,
	"the gitrog, ravenous ride":           true,
	"oracle of mul daya":                  true,
	"azusa, lost but seeking":             true,
	"dryad of the ilysian grove":          true,
	"exploration":                         true,
	"burgeoning":                          true,
	"wayward swordtooth":                  true,
	"lord windgrace":                      true,
	"world shaper":                        true,
	"life from the loam":                  true,
	"splendid reclamation":                true,
	"scapeshift":                          true,
}

// computeLandValueCluster surfaces the lands-matter package as a
// single synergy cluster. Returns nil when the cluster doesn't reach
// the 4-card floor or doesn't have at least one payoff (a pile of
// cycle-lands without a payoff isn't a wincon — see the bracket-
// gating note in archetype.go). Scoring follows the same producer →
// amplifier → payoff chain model as the other R60-round-2 clusters
// in computeSynergyClusters: pair bonus 2 between distinct roles,
// chain bonus +3 per complete producer×amplifier×payoff triangle.
func computeLandValueCluster(report *FreyaReport) *SynergyCluster {
	if report == nil || len(report.Profiles) < 10 {
		return nil
	}

	type roleEntry struct {
		name string
		role string // "producer" / "amplifier" / "payoff"
	}
	var entries []roleEntry
	seen := map[string]bool{}

	add := func(name, role string) {
		key := name + "|" + role
		if seen[key] {
			return
		}
		seen[key] = true
		entries = append(entries, roleEntry{name: name, role: role})
	}

	for _, p := range report.Profiles {
		nameLower := strings.ToLower(p.Name)

		// Producer: cycle-lands (the graveyard-feeder). Slow-fetches
		// and dual-cycle lands are the canonical producers because
		// their discard cost sends the land card to the graveyard
		// where the amplifier can replay it.
		if p.IsLand && p.HasCycling {
			add(p.Name, "producer")
		}

		// Amplifier: replay engines + extra-drop enablers. Crucible
		// of Worlds / Ramunap Excavator literally play lands from
		// the graveyard; Azusa / Exploration / Burgeoning enable
		// extra drops that consume the producer's output faster.
		if landValueAmplifierNames[nameLower] {
			add(p.Name, "amplifier")
		}

		// Payoff: landfall triggers + lands-in-graveyard scaling.
		isPayoff := false
		if profileHasTrigger(p, "landfall") {
			isPayoff = true
		}
		if landValuePayoffNames[nameLower] {
			isPayoff = true
		}
		if isPayoff {
			add(p.Name, "payoff")
		}
	}

	producers, amps, payoffs := 0, 0, 0
	cardSeen := map[string]bool{}
	var allMembers []string
	for _, e := range entries {
		switch e.role {
		case "producer":
			producers++
		case "amplifier":
			amps++
		case "payoff":
			payoffs++
		}
		if !cardSeen[e.name] {
			cardSeen[e.name] = true
			allMembers = append(allMembers, e.name)
		}
	}

	// Floor: 4 unique cards AND at least one payoff. A pile of
	// cycle-lands without a payoff is just fixing — see the bracket
	// gating in archetype.go. The payoff requirement is what
	// distinguishes a wincon package from incidental land density.
	if len(allMembers) < 4 || payoffs == 0 {
		return nil
	}

	// Pair-score across the three role buckets. Distinct-role pairs
	// score 2 (chain pair); same-role pairs score 1 (redundant
	// copies). Plus chain-completion bonus: each producer×amplifier×
	// payoff triangle adds +3. Matches the rolesPairScore /
	// chain-depth model in computeSynergyClusters.
	score := 0
	pair := func(aCount, bCount, weight int) {
		if aCount == 0 || bCount == 0 {
			return
		}
		score += aCount * bCount * weight
	}
	pair(producers, amps, 2)
	pair(producers, payoffs, 2)
	pair(amps, payoffs, 2)
	// Same-role pairs score 1 each.
	score += (producers * (producers - 1)) / 2
	score += (amps * (amps - 1)) / 2
	score += (payoffs * (payoffs - 1)) / 2

	chainCount := producers
	if amps < chainCount {
		chainCount = amps
	}
	if payoffs < chainCount {
		chainCount = payoffs
	}
	score += chainCount * 3

	displayed := allMembers
	if len(displayed) > 8 {
		displayed = displayed[:8]
	}

	return &SynergyCluster{
		Name:        "Land Value Package",
		Cards:       displayed,
		Theme:       "land_value",
		Score:       score,
		MemberCount: len(allMembers),
		AllMembers:  allMembers,
	}
}

// altBuildPivotThreshold is the minimum cluster MemberCount that
// qualifies as a "pivotable seed" — enough aligned cards that the
// deckbuilder could commit to the theme without rebuilding from
// scratch. 8 is the lower bar; the user's headline example was a
// "12 vs 8" split so anything below 8 wouldn't be worth a suggestion.
const altBuildPivotThreshold = 8

// computeAltBuildSuggestions detects when the deck has ≥2 synergy
// clusters above the pivot threshold and emits a suggestion for each
// non-primary cluster. The Decks screen renders these as "this deck
// is trying to do two things — you could focus on X or Y" hints.
//
// Requires the primary cluster to also clear the threshold — if the
// top cluster has only 5 aligned cards, the deck doesn't have a real
// spine yet and "alt-build" framing is misleading.
//
// Capped at 3 alt-builds to keep the Decks screen output bounded;
// past that, the deck is so unfocused that the suggestion list is
// itself noise.
func computeAltBuildSuggestions(dp *DeckProfile) {
	if len(dp.SynergyClusters) < 2 {
		return
	}
	primary := dp.SynergyClusters[0]
	if primary.MemberCount < altBuildPivotThreshold {
		return
	}
	for i := 1; i < len(dp.SynergyClusters) && len(dp.AltBuildSuggestions) < 3; i++ {
		sec := dp.SynergyClusters[i]
		if sec.MemberCount < altBuildPivotThreshold {
			continue
		}
		dp.AltBuildSuggestions = append(dp.AltBuildSuggestions, AltBuildSuggestion{
			Cluster:     sec.Theme,
			ClusterName: sec.Name,
			MemberCount: sec.MemberCount,
			Score:       sec.Score,
			Pivot: fmt.Sprintf(
				"You could refocus around %s — %d cards already lean that way (cluster score %d).",
				sec.Name, sec.MemberCount, sec.Score),
			Trade: fmt.Sprintf(
				"Currently splits slot priority with %s (%d cards). Picking one frees ~%d slots for deeper payoffs.",
				primary.Name, primary.MemberCount, sec.MemberCount/2),
		})
	}
}

// ---------------------------------------------------------------------------
// 5. Meta positioning — predicted matchup spread by archetype.
// ---------------------------------------------------------------------------

type matchupEntry struct {
	vsArchetype string
	rating      string
	reason      string
	// strength captures the MAGNITUDE of the favored/unfavored verdict.
	// Empty string is normalized to "moderate" by metaMatchupStrengthOrDefault.
	//
	//   "strong"   — mechanical hard-lock or near-unwinnable matchup.
	//                Drannith Magistrate denying every combo cast, Rest in
	//                Peace deleting reanimator's engine, Rule of Law / Eidolon
	//                of Rhetoric breaking storm. Roughly a 70-80% expected
	//                win rate for the favored side; the unfavored side is
	//                playing for a topdeck.
	//   "moderate" — the default. Real matchup edge from speed, density,
	//                or value differential. Roughly 60-70%. Covers most
	//                race-style entries ("fast clock punishes setup turns").
	//   "slight"   — narrow edge that flips on draw quality or pod context.
	//                "depends on draw", "race depends on", "neutral" entries
	//                with a directional lean. Roughly 52-58%.
	//
	// Neutral-rated matchups MUST carry "" (normalized to "moderate" for
	// rendering) — the strength scale is only meaningful for the
	// favored/unfavored arms. See TestMetaMatchups_StrengthOnlyOnDirectional.
	strength string
}

// metaMatchupStrengthOrDefault returns the entry's strength, defaulting
// to "moderate" when unset. Neutral-rated entries always return ""
// (strength is a directional-only signal — a neutral matchup has no
// magnitude to report).
func metaMatchupStrengthOrDefault(e matchupEntry) string {
	if e.rating == "neutral" {
		return ""
	}
	if e.strength == "" {
		return "moderate"
	}
	return e.strength
}

// metaMatchupStrengthOverrides annotates clear-cut matchups where the
// default "moderate" strength is misleading. Keyed on (fromArch, vsArch)
// where both sides are the canonical lowercase keys from metaMatchupDB.
//
// "strong": mechanical hard-lock — the unfavored side is materially
// unable to execute its plan even with optimal draws. The bar is high
// to keep this list short and defensible.
//
// "slight": draw- or curve-dependent — the directional lean is real but
// frequently flips on a single mulligan / topdeck. Applied to entries
// whose reason text already contains "depends on" / "race depends" /
// "draw-dependent" hedge language, since those are the cases where the
// "favored" verdict was already qualified.
//
// Annotating in a separate map (rather than inline on every matchupEntry)
// keeps the existing 80+ entries diff-free and makes the strength
// signal an additive, auditable layer.
var metaMatchupStrengthOverrides = map[[2]string]string{
	// --- "strong" — mechanical hard-locks ---
	{"stax", "Combo"}:        "strong", // Drannith Magistrate / Rule of Law / Cursed Totem
	{"stax", "Storm"}:        "strong", // Rule of Law / Eidolon of Rhetoric / Damping Sphere
	{"stax", "Reanimator"}:   "strong", // Drannith Magistrate denies the reanimation cast
	{"stax", "Aristocrats"}:  "strong", // Cursed Totem disables creature sac outlets
	{"stax", "Enchantress"}:  "strong", // taxes prevent enchantment engine setup
	{"reanimator", "Graveyard Hate"}: "strong", // Rest in Peace / Leyline of the Void
	{"aristocrats", "Graveyard Hate"}: "strong", // RIP / Leyline exile the recursion
	{"enchantress", "Enchantment Hate"}: "strong", // Aura Shards mass removal
	{"voltron", "Stax"}:       "strong", // commander-tax + Cursed Totem prevents recasting
	{"voltron", "Control"}:    "strong", // single threat folds to every removal spell
	{"storm", "Stax"}:         "strong", // Rule of Law locks the cast chain — game over
	{"storm", "Control"}:      "strong", // one Counterspell breaks the whole turn

	// --- Reciprocal entries — the other half of each hard-lock pair ---
	// The reciprocity invariant says A favored-strong vs B should pair
	// with B unfavored-strong vs A (both perspectives agree the matchup
	// is lopsided). Annotating both sides keeps deck-recommendation
	// surfaces consistent regardless of which deck's profile is being
	// viewed.
	{"combo", "Stax"}:         "strong",
	{"reanimator", "Stax"}:    "strong",
	{"aristocrats", "Stax"}:   "strong",
	{"enchantress", "Stax"}:   "strong",
	{"control", "Voltron"}:    "strong",
	{"control", "Storm"}:      "strong",

	// --- "slight" — draw/curve-dependent leans ---
	// (The matching entries are RATED favored/unfavored but the reason
	// text explicitly hedges. Tagging them as "slight" so callers can
	// down-weight these in deck recommendations.)
	{"aggro", "Voltron"}:      "slight", // wide board provides chumps; works only if you go wide
	{"midrange", "Aggro"}:     "slight", // The matching entry rated neutral; no override needed but
	// listed here as a placeholder for future tuning.
}

// init applies the strength overrides into metaMatchupDB so all readers
// (computeMetaPositioning, MetaStrongAgainst) see consistent values
// without each one re-doing the lookup. Runs at package init, after the
// var-declaration block above.
func init() {
	for fromKey, entries := range metaMatchupDB {
		for i, e := range entries {
			if s, ok := metaMatchupStrengthOverrides[[2]string{fromKey, e.vsArchetype}]; ok {
				entries[i].strength = s
			}
		}
		// Range over entries gave a copy; write back the modified slice.
		metaMatchupDB[fromKey] = entries
	}
}

// metaMatchupDB maps the deck's primary archetype to a list of
// vs-archetype predictions. The keys are normalized lowercase
// archetype tokens (see computeMetaPositioning for the canonicalisation
// step); the vsArchetype field carries the human-facing label.
//
// R60 expansion: 15 new entries filling reciprocity gaps + 6 sharpened
// reasoning strings naming specific mechanics or canonical cards.
// Reciprocity invariant: if A→B is favored, B→A should be unfavored
// (and vice versa); same rating only when both are neutral. The
// TestMetaMatchups_ReciprocityInvariant test pins this.
var metaMatchupDB = map[string][]matchupEntry{
	"aggro": {
		{vsArchetype: "Control", rating: "unfavored", reason: "Cyclonic Rift / Toxic Deluge wipes plus card-draw engines grind the board out by turn 8-10"},
		{vsArchetype: "Combo", rating: "favored", reason: "fast clock forces combo to assemble through removal before turn 6"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "Thalia / Vryn Wingmare taxes and Winter Orb-style locks slow the creature plan to a crawl"},
		{vsArchetype: "Reanimator", rating: "unfavored", reason: "early fatties out-stat your creatures the moment they hit the table"},
		{vsArchetype: "Aristocrats", rating: "unfavored", reason: "chump blockers and recursive bodies stall the clock; drain math closes faster than you can hit"},
		{vsArchetype: "Storm", rating: "favored", reason: "fast clock punishes storm setup turns before the kill chain assembles"},
		{vsArchetype: "Voltron", rating: "favored", reason: "wide board provides chump blockers that stall the 21-damage clock past lethal"},
		{vsArchetype: "Enchantress", rating: "neutral", reason: "pillow fort effects slow the race; depends on how early Ghostly Prison lands"},
		{vsArchetype: "Midrange", rating: "neutral", reason: "race depends on draw quality and curve smoothness"},
	},
	"combo": {
		{vsArchetype: "Aggro", rating: "unfavored", reason: "aggro often clocks the table before turn-5 combo assembly"},
		{vsArchetype: "Control", rating: "neutral", reason: "counterspells vs speed — depends on holding mana up for the right resolve"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "Drannith Magistrate / Rule of Law / Cursed Totem deny multiple combo cast lines"},
		{vsArchetype: "Voltron", rating: "favored", reason: "voltron's commander-tax assembly is slower than the goldfish combo turn"},
		{vsArchetype: "Reanimator", rating: "favored", reason: "goldfish kill lands before reanimator can stabilize on a haymaker"},
		{vsArchetype: "Aristocrats", rating: "favored", reason: "aristocrats' incremental drain is too slow vs the combo kill turn"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "enchantment engine ticks slower than combo piece assembly"},
		{vsArchetype: "Storm", rating: "neutral", reason: "race depends on which combo assembles first — both are goldfish strategies"},
		{vsArchetype: "Midrange", rating: "favored", reason: "goldfish speed outraces midrange value"},
	},
	"control": {
		{vsArchetype: "Aggro", rating: "favored", reason: "early board wipes (Toxic Deluge, Cyclonic Rift, Damnation) plus spot removal stabilize the board"},
		{vsArchetype: "Combo", rating: "neutral", reason: "need to hold counterspells for the right moment — combo can fade an answer with a tutor"},
		{vsArchetype: "Midrange", rating: "favored", reason: "card advantage wins the long game once threats are answered"},
		{vsArchetype: "Stax", rating: "neutral", reason: "both play long game, but stax constraints (Rule of Law, Winter Orb) tax your answer suite"},
		{vsArchetype: "Voltron", rating: "favored", reason: "single threat folds to Path to Exile / Pongify / Imprisoned in the Moon — each answer is a tempo win"},
		{vsArchetype: "Storm", rating: "favored", reason: "counterspells dismantle the storm chain mid-cast; one Counterspell breaks the whole turn"},
		{vsArchetype: "Reanimator", rating: "neutral", reason: "counterspells stop the reanimation spell but recursive enablers (Unburial Rites, Yawgmoth's Will) reset the line"},
		{vsArchetype: "Aristocrats", rating: "neutral", reason: "recursive threats are hard to answer one-at-a-time, but the clock is slow enough that wipes catch up"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "counterspells dismantle the enchantment engine before Argothian Enchantress / Sythis stack triggers, and enchantments can't be answered post-resolution as cheaply as Disenchant lets a control deck pick them apart"},
	},
	"aristocrats": {
		{vsArchetype: "Aggro", rating: "favored", reason: "resilient to removal, drain math bypasses combat damage"},
		{vsArchetype: "Control", rating: "neutral", reason: "recursive threats are hard to answer but the clock is slow"},
		{vsArchetype: "Combo", rating: "unfavored", reason: "incremental drain is too slow to race dedicated combo"},
		{vsArchetype: "Voltron", rating: "favored", reason: "sac engines + drain triggers ignore the commander-damage clock entirely"},
		{vsArchetype: "Reanimator", rating: "unfavored", reason: "reanimator fatties out-pressure the chump-and-drain plan; you can't sac fast enough"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "Drannith Magistrate denies recursion casts; Cursed Totem disables creature sac outlets"},
		{vsArchetype: "Storm", rating: "unfavored", reason: "storm kill lands before incremental drain closes the game"},
		{vsArchetype: "Midrange", rating: "favored", reason: "recursive drain out-grinds midrange value over 10+ turns"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "Blood Artist / Zulaport Cutthroat drain math bypasses Ghostly Prison / Propaganda entirely — pillow fort doesn't stop death triggers, and the aristocrat clock closes before the enchantment engine reaches lethal value"},
		{vsArchetype: "Graveyard Hate", rating: "unfavored", reason: "Rest in Peace / Leyline of the Void exile the recursion before it loops"},
	},
	"voltron": {
		{vsArchetype: "Control", rating: "unfavored", reason: "single threat folds to Path to Exile / Pongify / Imprisoned in the Moon — every answer is a full reset"},
		{vsArchetype: "Token/Go Wide", rating: "unfavored", reason: "chump blockers stall the 21-damage commander-damage clock"},
		{vsArchetype: "Combo", rating: "unfavored", reason: "combo wins faster than the commander-tax assembly can build a lethal threat"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "commander-tax escalation prevents recasting after each removal — stax exploits the single-threat plan"},
		{vsArchetype: "Aristocrats", rating: "unfavored", reason: "chump blockers and lifegain triggers stall the clock past lethal"},
		{vsArchetype: "Reanimator", rating: "unfavored", reason: "reanimator fatties block the commander and reset the damage count"},
		{vsArchetype: "Storm", rating: "unfavored", reason: "storm kill lands before commander damage assembles"},
		{vsArchetype: "Enchantress", rating: "unfavored", reason: "pillow fort effects (Ghostly Prison, Propaganda) deflect the commander-damage plan turn after turn"},
		{vsArchetype: "Midrange", rating: "favored", reason: "evasive commander outpaces midrange's slower blockers and removal cadence"},
		{vsArchetype: "Aggro", rating: "unfavored", reason: "wide aggro boards present chump blockers that stall the 21-damage clock past lethal"},
	},
	"stax": {
		{vsArchetype: "Combo", rating: "favored", reason: "Drannith Magistrate / Rule of Law / Cursed Totem deny combo cast lines"},
		{vsArchetype: "Aggro", rating: "favored", reason: "Thalia / Vryn Wingmare taxes and Winter Orb / Static Orb locks slow aggro to a crawl"},
		{vsArchetype: "Control", rating: "neutral", reason: "both play long game but stax constraints hurt both sides — Winter Orb is symmetrical"},
		{vsArchetype: "Midrange", rating: "favored", reason: "value engines need resources stax denies — Smokestack / Tangle Wire choke the curve"},
		{vsArchetype: "Storm", rating: "favored", reason: "Rule of Law / Eidolon of Rhetoric / Damping Sphere lock the cast chain"},
		{vsArchetype: "Reanimator", rating: "favored", reason: "Drannith Magistrate denies reanimation casts; Stranglehold prevents tutoring the enabler"},
		{vsArchetype: "Voltron", rating: "favored", reason: "commander-tax escalation under Drannith Magistrate / Cursed Totem prevents recasting the single threat"},
		{vsArchetype: "Aristocrats", rating: "favored", reason: "Drannith Magistrate + Cursed Totem disable creature sac outlets and recursion casts"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "taxes prevent enchantment engine setup; can't pay for Argothian Enchantress turns"},
	},
	"reanimator": {
		{vsArchetype: "Aggro", rating: "favored", reason: "early fatties outclass aggro creatures the turn they reanimate"},
		{vsArchetype: "Control", rating: "neutral", reason: "counterspells stop reanimation, but recursive enablers (Unburial Rites, Yawgmoth's Will) keep the line live"},
		{vsArchetype: "Graveyard Hate", rating: "unfavored", reason: "Rest in Peace / Leyline of the Void / Bojuka Bog turn off the entire engine"},
		{vsArchetype: "Combo", rating: "unfavored", reason: "combo kill lands before reanimator stabilizes on a haymaker"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "Drannith Magistrate denies the reanimation spell cast; Stranglehold prevents tutoring it"},
		{vsArchetype: "Aristocrats", rating: "favored", reason: "reanimated haymakers swing through chumps; aristocrats can't race the size differential"},
		{vsArchetype: "Voltron", rating: "favored", reason: "fatties out-stat the commander and reset the damage clock turn over turn"},
		{vsArchetype: "Midrange", rating: "favored", reason: "haymakers out-stat midrange creatures; the size gap is unwinnable"},
		{vsArchetype: "Storm", rating: "unfavored", reason: "storm wins before the big creature lands"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "fatties out-class enchantment engines; pillow fort doesn't stop combat damage from a 9/9"},
	},
	"storm": {
		{vsArchetype: "Stax", rating: "unfavored", reason: "Rule of Law / Eidolon of Rhetoric / Damping Sphere lock the cast chain — game over"},
		{vsArchetype: "Aggro", rating: "unfavored", reason: "fast clock punishes setup turns before the kill chain assembles"},
		{vsArchetype: "Control", rating: "unfavored", reason: "counterspells disrupt the chain mid-cast; one Counterspell breaks the whole turn"},
		{vsArchetype: "Midrange", rating: "favored", reason: "combo kill lands before midrange value matters"},
		{vsArchetype: "Combo", rating: "neutral", reason: "race depends on which combo assembles first — both are goldfish strategies"},
		{vsArchetype: "Aristocrats", rating: "favored", reason: "storm kill lands before aristocrats grind out"},
		{vsArchetype: "Reanimator", rating: "favored", reason: "storm kill lands before fatty reanimation stabilizes"},
		{vsArchetype: "Enchantress", rating: "favored", reason: "storm kill lands before enchantment engine pays off"},
		{vsArchetype: "Voltron", rating: "favored", reason: "storm kill lands before commander damage assembles"},
		{vsArchetype: "Graveyard Hate", rating: "unfavored", reason: "Underworld Breach / Past in Flames / Yawgmoth's Will storm variants are crippled by Rest in Peace / Leyline of the Void — graveyard-pivoting kill chains lose their pivot, and Bojuka Bog one-shots the recursion piece"},
	},
	"enchantress": {
		{vsArchetype: "Aggro", rating: "neutral", reason: "pillow fort effects (Ghostly Prison, Propaganda) can stabilize if drawn early"},
		{vsArchetype: "Enchantment Hate", rating: "unfavored", reason: "Aura Shards / Back to Nature / Calming Verse mass enchantment removal is devastating"},
		{vsArchetype: "Combo", rating: "unfavored", reason: "engine too slow to race dedicated combo"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "taxes prevent enchantment engine setup; can't pay for Argothian Enchantress turns"},
		{vsArchetype: "Reanimator", rating: "unfavored", reason: "reanimator fatties end the game before pillow fort stabilizes"},
		{vsArchetype: "Voltron", rating: "favored", reason: "pillow fort effects deflect the commander-damage plan"},
		{vsArchetype: "Storm", rating: "unfavored", reason: "storm kill lands before enchantment engine pays off"},
		{vsArchetype: "Control", rating: "unfavored", reason: "counterspells dismantle the enchantment engine before Argothian Enchantress / Sythis can stack triggers; control's Disenchant-style answers pick off the engine pieces faster than the engine pays for replacements"},
		{vsArchetype: "Aristocrats", rating: "unfavored", reason: "Blood Artist / Zulaport drain bypasses pillow fort entirely — Ghostly Prison only deflects combat, and the death-trigger clock closes before the enchantment engine reaches lethal value"},
		{vsArchetype: "Midrange", rating: "neutral", reason: "both grind for value, draw-dependent"},
	},
	"midrange": {
		{vsArchetype: "Aggro", rating: "neutral", reason: "bigger creatures but slower start — race depends on tempo"},
		{vsArchetype: "Control", rating: "unfavored", reason: "outgrinded in long games by superior card advantage"},
		{vsArchetype: "Combo", rating: "unfavored", reason: "too fair to race combo"},
		{vsArchetype: "Storm", rating: "unfavored", reason: "midrange clock is too slow to outrun storm cast turns"},
		{vsArchetype: "Reanimator", rating: "unfavored", reason: "reanimator fatties out-stat midrange creatures — the size gap is unwinnable"},
		{vsArchetype: "Stax", rating: "unfavored", reason: "value engines need resources stax denies — Smokestack / Tangle Wire choke the curve"},
		{vsArchetype: "Voltron", rating: "unfavored", reason: "evasive commander outpaces midrange's blockers and removal cadence"},
		{vsArchetype: "Aristocrats", rating: "unfavored", reason: "aristocrats' recursive drain out-grinds midrange value over 10+ turns"},
		{vsArchetype: "Enchantress", rating: "neutral", reason: "both grind for value, draw-dependent"},
	},
}

// canonicaliseMetaArchetypeKey maps a free-form archetype string (the
// classifier's output, e.g. "Go Wide Tokens" / "Infinite Combo") to the
// lowercase key used by metaMatchupDB. Shared by computeMetaPositioning
// (forward lookup) and MetaStrongAgainst (reverse lookup) so the two
// stay in lockstep — a new alias added here lights up both directions.
func canonicaliseMetaArchetypeKey(archetype string) string {
	arch := strings.ToLower(archetype)
	switch {
	case containsAny(arch, "aggro", "go wide", "token", "tribal", "extra combats"):
		return "aggro"
	case containsAny(arch, "combo", "infinite"):
		return "combo"
	case containsAny(arch, "stax"):
		return "stax"
	case containsAny(arch, "aristocrats"):
		return "aristocrats"
	case containsAny(arch, "voltron"):
		return "voltron"
	case containsAny(arch, "reanimator"):
		return "reanimator"
	case containsAny(arch, "storm", "spellslinger"):
		return "storm"
	case containsAny(arch, "enchantress"):
		return "enchantress"
	case containsAny(arch, "control", "mill", "discard"):
		return "control"
	default:
		return "midrange"
	}
}

// archetypeKeyToLabels returns the human-facing labels that appear as
// vsArchetype values in metaMatchupDB and resolve back to the given
// canonical key. Used by MetaStrongAgainst to do reverse-direction
// matching — e.g. "aggro" matches both "Aggro" and "Token/Go Wide" so
// a Tokens deck picks up Voltron's "Token/Go Wide: unfavored" entry.
func archetypeKeyToLabels(key string) []string {
	switch key {
	case "aggro":
		return []string{"Aggro", "Token/Go Wide"}
	case "combo":
		return []string{"Combo"}
	case "control":
		return []string{"Control"}
	case "aristocrats":
		return []string{"Aristocrats"}
	case "voltron":
		return []string{"Voltron"}
	case "stax":
		return []string{"Stax"}
	case "reanimator":
		return []string{"Reanimator"}
	case "storm":
		return []string{"Storm"}
	case "enchantress":
		return []string{"Enchantress"}
	case "midrange":
		return []string{"Midrange"}
	}
	return nil
}

// MetaStrongAgainst answers "which archetypes does X reliably beat?"
// by combining BOTH directions of the matchup DB:
//
//   - Forward: entries in metaMatchupDB[X] with rating=favored — X's
//     own claim that it beats Y.
//   - Reverse: entries elsewhere in the DB where vsArchetype resolves
//     to X (via archetypeKeyToLabels) and rating=unfavored — Y's own
//     claim that it loses to X.
//
// The reciprocity invariant (TestMetaMatchups_ReciprocityInvariant)
// guarantees the two directions mostly agree, but the reverse pass
// adds two distinct values: (1) it surfaces matchups against meta-call
// labels that aren't DB keys (Voltron's "Token/Go Wide: unfavored" is
// invisible to a forward-only Tokens lookup); (2) the reverse reason
// reads from the opponent's perspective ("Y can't tutor its enabler
// against X") which complements the forward reason ("X denies Y's
// cast lines") on the same matchup.
//
// Output is sorted: matchups corroborated by BOTH directions first,
// then forward-only, then reverse-only — each tier sorted by archetype
// name for stability. The Decks screen consumes this as the
// "favored against" line under a deck profile.
func MetaStrongAgainst(archetype string) []MetaAdvantage {
	key := canonicaliseMetaArchetypeKey(archetype)
	selfLabels := map[string]bool{}
	for _, lbl := range archetypeKeyToLabels(key) {
		selfLabels[lbl] = true
	}

	// Forward pass: X's own favored entries.
	out := []MetaAdvantage{}
	idx := map[string]int{}
	for _, m := range metaMatchupDB[key] {
		if m.rating != "favored" {
			continue
		}
		idx[m.vsArchetype] = len(out)
		out = append(out, MetaAdvantage{
			Archetype: m.vsArchetype,
			Reason:    m.reason,
			Source:    "forward",
		})
	}

	// Reverse pass: other archetypes' unfavored-vs-X entries. The
	// `inverseLabels` step is the key trick — Tokens' "aggro" key
	// matches both "Aggro" and "Token/Go Wide" as opponent-side labels.
	for otherKey, matchups := range metaMatchupDB {
		if otherKey == key {
			continue
		}
		otherLabels := archetypeKeyToLabels(otherKey)
		if len(otherLabels) == 0 {
			continue
		}
		otherLabel := otherLabels[0] // the canonical display label
		for _, m := range matchups {
			if m.rating != "unfavored" {
				continue
			}
			if !selfLabels[m.vsArchetype] {
				continue
			}
			if i, ok := idx[otherLabel]; ok {
				// Already on the list from forward direction —
				// upgrade to "both" and attach the opponent's reason.
				if out[i].OpponentReason == "" {
					out[i].OpponentReason = m.reason
					out[i].Source = "both"
				}
				continue
			}
			idx[otherLabel] = len(out)
			out = append(out, MetaAdvantage{
				Archetype: otherLabel,
				Reason:    m.reason,
				Source:    "reverse",
			})
			break // one entry per other-archetype is enough
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		rank := func(src string) int {
			switch src {
			case "both":
				return 0
			case "forward":
				return 1
			case "reverse":
				return 2
			}
			return 3
		}
		if ri, rj := rank(out[i].Source), rank(out[j].Source); ri != rj {
			return ri < rj
		}
		return out[i].Archetype < out[j].Archetype
	})
	return out
}

func computeMetaPositioning(dp *DeckProfile) {
	arch := canonicaliseMetaArchetypeKey(dp.PrimaryArchetype)

	matchups, ok := metaMatchupDB[arch]
	if !ok {
		return
	}

	for _, m := range matchups {
		dp.MetaMatchups = append(dp.MetaMatchups, MetaMatchup{
			Archetype: m.vsArchetype,
			Rating:    m.rating,
			Reason:    m.reason,
			Strength:  metaMatchupStrengthOrDefault(m),
		})
	}

	dp.StrongAgainst = MetaStrongAgainst(dp.PrimaryArchetype)
}

// ---------------------------------------------------------------------------
// 6. Card quality tiers — identify star performers and cuttable cards.
// ---------------------------------------------------------------------------

// powerExplanationInputs is the parameter bundle for
// buildPowerExplanation. Grouped so the caller doesn't have to pass
// 13 positional arguments and so adding new signals is a one-field
// change in two places instead of three.
type powerExplanationInputs struct {
	tier             string   // "S" / "A" / "B" / "C" / "D"
	cmc              int
	roles            []RoleTag
	primaryArchetype string   // e.g. "Combo"; "" when deck didn't match a fingerprint
	matchedFitRoles  []string // card roles that landed in the deck's fingerprint
	fitFloorHit      bool     // true when archetype-fit was lifted to the 10-point floor
	multiLowCMC      bool     // CMC<=2 with 2+ role tags (efficiency-sweet-spot)
	isWincon         bool
	isBridge         bool
	isStep           bool
	isFinisher       bool
	isCluster        bool
	isRedundantTutor bool
	isDeadSlot       bool
}

// buildPowerExplanation assembles a one-line human-readable why for a
// card's power-tier grade. Format:
//
//	TIER — signal1 + signal2 + signal3
//
// Signals are listed in priority order (highest-impact first) so the
// reader sees the dominant contributor up front. Per-component
// coverage:
//
//   - Synergy: wincon piece / value-chain bridge / finisher / chain
//     step / cluster member (only the highest-tier hit, to keep the
//     line scannable)
//   - CMC efficiency: multi-role at low CMC, or curve placement when
//     it's load-bearing for the verdict
//   - Archetype fit: matched-role list, "off-archetype" for tagged
//     non-matches, "untagged" when no roles at all
//   - Penalties: redundant tutor / dead slot, appended at the end so
//     they read as the explanation for low grades
//
// The signal set is exhaustive — every card produces at least one
// signal (curve/fit/untagged always emit) so explanations are never
// empty.
func buildPowerExplanation(in powerExplanationInputs) string {
	var parts []string

	// 1. Top synergy signal — the single highest-impact membership.
	switch {
	case in.isWincon:
		parts = append(parts, "wincon piece")
	case in.isBridge:
		parts = append(parts, "value-chain bridge")
	case in.isFinisher:
		parts = append(parts, "finisher")
	case in.isStep:
		parts = append(parts, "value-chain step")
	case in.isCluster:
		parts = append(parts, "cluster member")
	}

	// 2. CMC + role-count framing.
	switch {
	case in.multiLowCMC:
		parts = append(parts, fmt.Sprintf("%d-role at CMC %d", len(in.roles), in.cmc))
	case in.cmc <= 2 && len(in.roles) == 1:
		parts = append(parts, fmt.Sprintf("%s at CMC %d", in.roles[0], in.cmc))
	case in.cmc >= 5 && len(in.roles) <= 1:
		parts = append(parts, fmt.Sprintf("CMC %d (curve heavy)", in.cmc))
	case len(in.roles) == 1:
		parts = append(parts, fmt.Sprintf("%s at CMC %d", in.roles[0], in.cmc))
	case len(in.roles) >= 2:
		parts = append(parts, fmt.Sprintf("%d-role at CMC %d", len(in.roles), in.cmc))
	}

	// 3. Archetype fit framing.
	switch {
	case len(in.matchedFitRoles) > 0:
		archLabel := in.primaryArchetype
		if archLabel == "" {
			archLabel = "archetype"
		}
		parts = append(parts, fmt.Sprintf("%s fit (%s)", archLabel, strings.Join(in.matchedFitRoles, "/")))
	case in.fitFloorHit:
		parts = append(parts, "off-archetype")
	case len(in.roles) == 0:
		parts = append(parts, "untagged")
	}

	// 4. Penalties — surfaced at the tail so the why-line for a D-tier
	// card explains the demotion ("D — CMC 6 (curve heavy) + untagged
	// + dead slot").
	if in.isRedundantTutor {
		parts = append(parts, "redundant tutor")
	}
	if in.isDeadSlot {
		parts = append(parts, "dead slot")
	}

	if len(parts) == 0 {
		// Defensive — every code path above should emit at least one
		// signal, but if a future refactor empties parts the line is
		// still readable as just "TIER".
		return in.tier
	}
	return fmt.Sprintf("%s — %s", in.tier, strings.Join(parts, " + "))
}

// computePetCards detects "pet cards" — low-tier creatures the
// deckbuilder kept despite their off-archetype fit. A pet card signals
// that personal taste / flavor matters more to the builder than raw
// optimization for the deck's strategy, so the upgrade coaching
// shouldn't hammer "cut this" against them.
//
// Detection requires ALL of:
//
//  1. PowerTier is C or D (low-tier). High-power creatures don't need
//     a "keep for flavor" defense — they're already pulling weight.
//  2. TypeLine contains "creature". Builders form attachments to
//     creatures (lore, art, signature plays) far more than to generic
//     noncreature utility. A bad spell is just a bad spell; a bad
//     creature is often deliberate.
//  3. Card has at least one role tag. Pure-filler / untagged cards
//     aren't pet cards — they're cards the builder didn't realize
//     were bad. A tagged card playing SOME role was an intentional
//     pick.
//  4. NONE of the card's roles match the deck's primary-archetype
//     fingerprint. If the card's role is what the deck wants, it's
//     just a low-power play of the right type — not a pet card.
//     Off-archetype creatures are the canonical flavor-pick shape:
//     "I know my Tribal deck doesn't want this Dragon, but I love it."
//  5. NOT a dead slot (CMC 5+ Utility-only). The existing dead-slot
//     penalty path already flags these as obvious cuts; bundling them
//     under "pet card" would muddy both signals.
//
// Legendary creatures get a slightly different reason string
// ("signature flavor pick") since legendaries are usually the
// strongest pet-card signal — characters players genuinely care about.
//
// Cap at 8 display entries (sorted by Power descending so the
// "highest-power flavor picks" lead — those are the most defensible
// keeps), since a flavor deck can legitimately have many pet cards
// and a longer list stops being useful guidance.
func computePetCards(dp *DeckProfile, report *FreyaReport) {
	if report.Roles == nil || len(dp.CardPowerLevels) == 0 {
		return
	}
	profileByName := map[string]CardProfile{}
	for _, p := range report.Profiles {
		profileByName[p.Name] = p
	}
	roleMap := map[string][]RoleTag{}
	for _, a := range report.Roles.Assignments {
		roleMap[a.Name] = a.Roles
	}
	var fpRatios map[RoleTag]float64
	for _, fp := range archetypeFingerprints {
		if fp.Name == dp.PrimaryArchetype {
			fpRatios = fp.Ratios
			break
		}
	}

	for _, pl := range dp.CardPowerLevels {
		if pl.PowerTier != "C" && pl.PowerTier != "D" {
			continue
		}
		p, ok := profileByName[pl.Name]
		if !ok {
			continue
		}
		tl := strings.ToLower(p.TypeLine)
		if !strings.Contains(tl, "creature") {
			continue
		}
		roles := roleMap[pl.Name]
		if len(roles) == 0 {
			continue // untagged pure-filler — not a pet
		}
		matched := false
		for _, r := range roles {
			if _, ok := fpRatios[r]; ok {
				matched = true
				break
			}
		}
		if matched {
			continue // role fits the archetype — not a pet
		}
		// Dead slot — the existing cuttable path already owns this
		// signal; pet card would double-flag.
		if p.CMC >= 5 && len(roles) == 1 && roles[0] == RoleUtility {
			continue
		}

		reason := "off-archetype creature — likely a personal-taste pick (keep if you love it)"
		if strings.Contains(tl, "legendary") {
			reason = "off-archetype legendary creature — signature flavor pick (keep if you love it)"
		}

		dp.PetCards = append(dp.PetCards, PetCard{
			Name:      pl.Name,
			CMC:       p.CMC,
			Roles:     pl.Roles,
			Power:     pl.Power,
			PowerTier: pl.PowerTier,
			Reason:    reason,
		})
	}

	// CardPowerLevels is already sorted Power desc, so PetCards inherit
	// that ordering for free. Cap at 8 for display scannability.
	if len(dp.PetCards) > 8 {
		dp.PetCards = dp.PetCards[:8]
	}
}

// computeCardPower populates dp.CardPowerLevels with a 0-100 power
// rating for every non-land card in the deck. Power is the clamped sum
// of three explicit components:
//
//	ArchetypeFit (0-40)         — card roles aligned with the deck's
//	                              primary-archetype fingerprint ratios.
//	                              Tagged cards get a small floor (10)
//	                              so role-less cards aren't lumped with
//	                              filler that earned nothing.
//	CMCEfficiency (0-20)        — curve placement with a low-CMC bias.
//	                              Multi-role at CMC<=2 earns a bonus.
//	SynergyContribution (0-40)  — wincon piece (+25), value-chain bridge
//	                              (+20) / step (+10), finisher (+12),
//	                              cluster member (+6), per-role floor
//	                              (+2). Penalties for expensive-tutor
//	                              redundancy (-8) and CMC5+ utility-only
//	                              dead slots (-10) subtract within the
//	                              component (clamped to [0, 40]).
//
// Final Power = sum, clamped [0, 100]. Sorted high → low.
func computeCardPower(dp *DeckProfile, report *FreyaReport) {
	if report.Roles == nil {
		return
	}
	// Role assignment lookup.
	roleMap := map[string][]RoleTag{}
	for _, a := range report.Roles.Assignments {
		roleMap[a.Name] = a.Roles
	}
	// Primary-archetype fingerprint for the fit component. Falls back
	// to an empty map when the deck didn't match a fingerprint, so the
	// fit component reduces to the tagged-card floor.
	var fpRatios map[RoleTag]float64
	for _, fp := range archetypeFingerprints {
		if fp.Name == dp.PrimaryArchetype {
			fpRatios = fp.Ratios
			break
		}
	}

	// Win-line / value-chain / finisher / cluster membership sets.
	winLinePieces := map[string]bool{}
	if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			for _, piece := range wl.Pieces {
				winLinePieces[piece] = true
			}
		}
	}
	chainSteps := map[string]bool{}
	chainBridges := map[string]bool{}
	for _, vc := range report.ValueChains {
		for _, step := range vc.Steps {
			for _, card := range step.Cards {
				chainSteps[card] = true
			}
		}
		for _, b := range vc.BridgeCards {
			chainBridges[b] = true
		}
	}
	finisherPieces := map[string]bool{}
	for _, c := range report.Finishers {
		for _, name := range c.Cards {
			finisherPieces[name] = true
		}
	}
	clusterMembers := map[string]bool{}
	for _, cl := range dp.SynergyClusters {
		for _, name := range cl.Cards {
			clusterMembers[name] = true
		}
	}

	// Tutor density for the redundant-tutor penalty.
	cheaperTutorsThan := map[int]int{} // cmc → count of cheaper non-land tutors
	for _, p := range report.Profiles {
		if !p.IsTutor || p.IsLandTutor {
			continue
		}
		for cmc := p.CMC + 1; cmc <= 10; cmc++ {
			cheaperTutorsThan[cmc]++
		}
	}

	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}

	for _, p := range report.Profiles {
		if p.IsLand {
			continue
		}
		roles := roleMap[p.Name]

		// --- Archetype fit (0-40) ---
		archFit := 0
		var matchedFitRoles []string
		for _, r := range roles {
			if ratio, ok := fpRatios[r]; ok {
				archFit += int(ratio * 200) // 0.10 → 20; 0.20 → 40
				matchedFitRoles = append(matchedFitRoles, string(r))
			}
		}
		fitFloorHit := false
		// Tagged cards that didn't match the fingerprint still get a small
		// fit credit — they're playing SOME role, just not the deck's
		// signature one (e.g. a Removal card in a Combo deck).
		if len(roles) > 0 && archFit < 10 {
			archFit = 10
			fitFloorHit = true
		}
		archFit = clamp(archFit, 0, 40)

		// --- CMC efficiency (0-20) ---
		var cmcEff int
		switch {
		case p.CMC <= 1:
			cmcEff = 20
		case p.CMC == 2:
			cmcEff = 18
		case p.CMC == 3:
			cmcEff = 14
		case p.CMC == 4:
			cmcEff = 10
		case p.CMC == 5:
			cmcEff = 6
		default:
			cmcEff = 2
		}
		// Multi-role at low CMC is the efficiency sweet spot.
		multiLowCMC := p.CMC <= 2 && len(roles) >= 2
		if multiLowCMC {
			cmcEff += 2
		}
		cmcEff = clamp(cmcEff, 0, 20)

		// --- Synergy contribution (0-40) ---
		syn := 0
		isWincon := winLinePieces[p.Name]
		if isWincon {
			syn += 25
		}
		isBridge := chainBridges[p.Name]
		isStep := !isBridge && chainSteps[p.Name]
		if isBridge {
			syn += 20
		} else if isStep {
			syn += 10
		}
		isFinisher := finisherPieces[p.Name]
		if isFinisher {
			syn += 12
		}
		isCluster := clusterMembers[p.Name]
		if isCluster {
			syn += 6
		}
		// Per-role floor — generic tagged cards land at modest synergy
		// rather than zero.
		syn += len(roles) * 2

		// Penalty: redundant expensive tutor when 3+ cheaper non-land
		// tutors already exist in the deck.
		isRedundantTutor := p.IsTutor && !p.IsLandTutor && p.CMC >= 4 && cheaperTutorsThan[p.CMC] >= 3
		if isRedundantTutor {
			syn -= 8
		}
		// Penalty: CMC 5+ with only RoleUtility tag is a dead slot.
		isDeadSlot := p.CMC >= 5 && len(roles) == 1 && roles[0] == RoleUtility
		if isDeadSlot {
			syn -= 10
		}
		syn = clamp(syn, 0, 40)

		power := clamp(archFit+cmcEff+syn, 0, 100)
		tier := PowerTierFor(power)

		roleStrs := make([]string, len(roles))
		for i, r := range roles {
			roleStrs[i] = string(r)
		}

		explanation := buildPowerExplanation(powerExplanationInputs{
			tier:             tier,
			cmc:              p.CMC,
			roles:            roles,
			primaryArchetype: dp.PrimaryArchetype,
			matchedFitRoles:  matchedFitRoles,
			fitFloorHit:      fitFloorHit,
			multiLowCMC:      multiLowCMC,
			isWincon:         isWincon,
			isBridge:         isBridge,
			isStep:           isStep,
			isFinisher:       isFinisher,
			isCluster:        isCluster,
			isRedundantTutor: isRedundantTutor,
			isDeadSlot:       isDeadSlot,
		})

		dp.CardPowerLevels = append(dp.CardPowerLevels, CardPowerLevel{
			Name:                p.Name,
			CMC:                 p.CMC,
			Roles:               roleStrs,
			Power:               power,
			PowerTier:           tier,
			Explanation:         explanation,
			ArchetypeFit:        archFit,
			CMCEfficiency:       cmcEff,
			SynergyContribution: syn,
		})
	}

	sort.Slice(dp.CardPowerLevels, func(i, j int) bool {
		if dp.CardPowerLevels[i].Power != dp.CardPowerLevels[j].Power {
			return dp.CardPowerLevels[i].Power > dp.CardPowerLevels[j].Power
		}
		return dp.CardPowerLevels[i].Name < dp.CardPowerLevels[j].Name
	})

	// Aggregate the tier histogram. Pre-seed all 5 tiers with 0 so
	// downstream rendering always reads S/A/B/C/D in stable order even
	// when a deck has zero S-tier cards.
	dp.PowerTierCounts = map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0}
	for _, pl := range dp.CardPowerLevels {
		dp.PowerTierCounts[pl.PowerTier]++
	}
}

func computeCardQualityTiers(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.Roles == nil {
		return
	}

	type cardScore struct {
		name      string
		score     float64
		roles     []RoleTag
		cmc       int
		reason    string
		detected  string
		whyCut    string
		effect    string
		suggested []string
	}

	var scores []cardScore

	// Build role assignment lookup
	roleMap := map[string][]RoleTag{}
	for _, a := range report.Roles.Assignments {
		roleMap[a.Name] = a.Roles
	}

	// Power-level lookup (populated by computeCardPower upstream).
	powerByName := map[string]int{}
	explanationByName := map[string]string{}
	for _, pl := range dp.CardPowerLevels {
		powerByName[pl.Name] = pl.Power
		explanationByName[pl.Name] = pl.Explanation
	}

	// Score combo pieces mentioned in win lines
	winLinePieces := map[string]bool{}
	if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			for _, piece := range wl.Pieces {
				winLinePieces[piece] = true
			}
		}
	}

	// Score value chain pieces
	chainPieces := map[string]bool{}
	bridgePieces := map[string]bool{}
	for _, vc := range report.ValueChains {
		for _, step := range vc.Steps {
			for _, card := range step.Cards {
				chainPieces[card] = true
			}
		}
		for _, b := range vc.BridgeCards {
			bridgePieces[b] = true
		}
	}

	for _, p := range report.Profiles {
		if p.IsLand {
			continue
		}

		s := cardScore{
			name:  p.Name,
			cmc:   p.CMC,
			roles: roleMap[p.Name],
		}

		// Multi-role cards score higher
		s.score += float64(len(s.roles)) * 1.0

		// Win line pieces are stars
		if winLinePieces[p.Name] {
			s.score += 3.0
			s.reason = "win condition piece"
		}

		// Bridge cards in value chains are highly valuable
		if bridgePieces[p.Name] {
			s.score += 2.5
			if s.reason == "" {
				s.reason = "value chain bridge card"
			}
		} else if chainPieces[p.Name] {
			s.score += 1.0
		}

		// CMC efficiency: low CMC with multiple roles is efficient
		if p.CMC <= 2 && len(s.roles) >= 2 {
			s.score += 1.5
			if s.reason == "" {
				s.reason = "efficient multi-role at low CMC"
			}
		}

		// High CMC with only utility role is likely cuttable
		if p.CMC >= 5 && len(s.roles) == 1 && s.roles[0] == RoleUtility {
			s.score -= 2.0
			s.reason = "high CMC with no clear role"
			s.detected = fmt.Sprintf("CMC %d, single role: utility", p.CMC)
			s.whyCut = "Pays full price but contributes neither pressure nor synergy. Top-end slots should accelerate the gameplan."
			s.effect = "Frees a top-end slot for a payoff threat, draw engine, or finisher tied to the deck's value chain."
		}

		// Cards with only Utility role and high CMC
		if p.CMC >= 4 && len(s.roles) == 1 && s.roles[0] == RoleUtility {
			s.score -= 1.0
			if s.reason == "" {
				s.reason = "filler — no synergy role at CMC " + fmt.Sprint(p.CMC)
				s.detected = fmt.Sprintf("CMC %d, single role: utility", p.CMC)
				s.whyCut = "Mid-range slot consumed by a card with no synergy tag — likely a generic value piece duplicated by stronger options in the same colors."
				s.effect = "Opens a CMC " + fmt.Sprint(p.CMC) + " slot for a synergy-tagged replacement."
			}
		}

		// CMC >= 4 with ZERO role tags AND not part of a detected win
		// line / value chain = priority cuttable. The pre-r60 detector
		// only fired on (CMC>=4 && len==1 && Utility), missing the
		// classic vanilla-creature case (Hill Giant, Craw Wurm) where
		// the card has no role tags at all because no role-detector
		// fires on it. Such cards have neither raw stats worth the
		// mana nor synergy with the deck's gameplan; they're the
		// clearest possible cuts.
		//
		// Win-line / chain-piece override: a card might be tagged as
		// a combo piece without earning a separate role (e.g. a
		// vanilla creature that's the target of a Birthing Pod chain).
		// Skip the penalty in those cases — the win-line membership
		// is the role.
		//
		// Threshold note: CMC 4 chosen rather than CMC 5 because the
		// 4-mana slot is dense with high-quality alternatives in every
		// archetype; a 4-mana card paying nothing for its mana cost is
		// already a quality miss. CMC 3 still gets a pass — there are
		// real 3-mana cards with no role tags that earn their slot via
		// raw stats (Watchwolf, Tarmogoyf in some shells).
		if p.CMC >= 4 && len(s.roles) == 0 &&
			!winLinePieces[p.Name] && !chainPieces[p.Name] && !bridgePieces[p.Name] {
			s.score -= 2.0
			isCreature := strings.Contains(strings.ToLower(p.TypeLine), "creature")
			if isCreature {
				s.reason = fmt.Sprintf("vanilla creature at CMC %d — no role tags", p.CMC)
			} else {
				s.reason = fmt.Sprintf("no role tags at CMC %d — pays full mana for no synergy", p.CMC)
			}
			s.detected = fmt.Sprintf("CMC %d, zero role tags assigned", p.CMC)
			if isCreature {
				s.whyCut = "Vanilla body — no abilities matter for this deck's gameplan, no role-detector fires. " +
					"Pays full mana for raw P/T at a CMC where every other archetype's payoffs are denser."
			} else {
				s.whyCut = "No roles fire on this card and it's not a piece of any detected win line or value chain. " +
					"Spending mana on it actively crowds out role-tagged alternatives."
			}
			s.effect = fmt.Sprintf("Frees a CMC %d slot for a role-tagged replacement "+
				"(threat, removal, draw, tutor, or combo piece in the deck's colors).", p.CMC)
		}

		// Tutors that are worse versions of other tutors in the deck.
		// Compare like-with-like: land tutors against land tutors, real
		// tutors against real tutors. A CMC-3 Cultivate is not "strictly
		// worse than CMC-2 Vampiric Tutor" because they fetch different
		// things.
		if p.IsTutor && !p.IsLandTutor && p.CMC >= 4 {
			cheaperTutors := 0
			for _, other := range report.Profiles {
				if other.IsTutor && !other.IsLandTutor && other.CMC < p.CMC && other.Name != p.Name {
					cheaperTutors++
				}
			}
			if cheaperTutors >= 3 {
				s.score -= 1.5
				s.reason = fmt.Sprintf("expensive tutor (CMC %d) with %d cheaper alternatives", p.CMC, cheaperTutors)
				s.detected = fmt.Sprintf("CMC %d tutor, %d cheaper tutors already in deck", p.CMC, cheaperTutors)
				s.whyCut = "Strictly slower than the existing tutor suite. The deck already has cheaper, equivalent search options that beat this card on tempo."
				s.effect = "Removes a redundant tutor slot — replace with interaction, draw, or a missing combo piece you currently can't fetch."
			}
		}

		scores = append(scores, s)
	}

	// Sort by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Top scorers (score >= 3) = stars. Cap at 5 to keep the list
	// signal-dense — past the top 5 the per-card score gap is small
	// enough that calling them "star" pollutes the recommendation.
	starCount := 0
	starredNames := map[string]bool{}
	for i := 0; i < len(scores) && starCount < 5; i++ {
		if scores[i].score < 3.0 {
			break
		}
		reason := scores[i].reason
		if reason == "" {
			reason = "high synergy density"
		}
		power := powerByName[scores[i].name]
		dp.StarCards = append(dp.StarCards, CardQuality{
			Name:             scores[i].name,
			Tier:             "star",
			Reason:           reason,
			Power:            power,
			PowerTier:        PowerTierFor(power),
			PowerExplanation: explanationByName[scores[i].name],
		})
		starredNames[scores[i].name] = true
		starCount++
	}

	// Flex Slot tier (R60): the FOURTH tier sitting between Solid and
	// Cuttable. Cards that have a single generic utility role tag, are
	// not part of any win line or value chain, and score in the
	// lower-positive band. These are the slots builders swap for
	// metagame tech (Grafdigger's Cage when GY decks rise, Pithing
	// Needle when planeswalker decks rise, etc) — the role they fill
	// is replaceable by another card serving the same generic purpose.
	//
	// Disjoint from Solid: flex picks are removed from the Solid
	// candidate pool below (via flexNames) so a card never appears in
	// both lists. Disjoint from Star/Cuttable via score bands.
	//
	// Criteria (all required):
	//   - score in (0.0, 2.0) — strictly positive so cuttable retains
	//     ownership of score==0 cards (typically Utility-only at CMC>=4
	//     where the CMC penalty zeroed the role bonus — those are
	//     genuine cuts, not flex slots)
	//   - exactly one role tag, and that role is in genericFlexRoles
	//   - not a win-line piece, value-chain piece, or bridge
	//   - not already starred
	// Cap at 5 to mirror the other tiers.
	genericFlexRoles := map[RoleTag]bool{
		RoleRemoval:    true,
		RoleDraw:       true,
		RoleRamp:       true,
		RoleProtection: true,
		RoleUtility:    true,
	}
	const flexMin, flexMax = 0.0, 2.0
	flexNames := map[string]bool{}
	flexCount := 0
	for i := 0; i < len(scores) && flexCount < 5; i++ {
		s := scores[i]
		if starredNames[s.name] {
			continue
		}
		if s.score <= flexMin || s.score >= flexMax {
			continue
		}
		if winLinePieces[s.name] || chainPieces[s.name] || bridgePieces[s.name] {
			continue
		}
		if len(s.roles) != 1 || !genericFlexRoles[s.roles[0]] {
			continue
		}
		roleStr := string(s.roles[0])
		reason := fmt.Sprintf("generic %s at CMC %d — flex slot, swap for situational meta tech (graveyard hate, PW hate, etc) without breaking the gameplan",
			strings.ToLower(roleStr), s.cmc)
		power := powerByName[s.name]
		dp.FlexSlots = append(dp.FlexSlots, CardQuality{
			Name:             s.name,
			Tier:             "flex",
			Reason:           reason,
			Power:            power,
			PowerTier:        PowerTierFor(power),
			PowerExplanation: explanationByName[s.name],
		})
		flexNames[s.name] = true
		flexCount++
	}

	// Solid Pick tier (R60): the middle slice — cards that scored above
	// the cut floor but below star threshold. Surfaces "okay-but-
	// replaceable" cards so builders shopping upgrades know which slots
	// are flex, not load-bearing. Excludes stars and any card already
	// scheduled for the cuttable list (handled by score floor).
	//
	// Window: 0.5 <= score < 3.0. The lower bound trims pure-filler that
	// only earned points from a single role tag; the upper bound matches
	// the star threshold so a card is never both. Cap at 5 to mirror the
	// star list and keep the section scannable.
	const solidMin, solidMax = 0.5, 3.0
	solidCount := 0
	for i := 0; i < len(scores) && solidCount < 5; i++ {
		s := scores[i]
		if starredNames[s.name] {
			continue
		}
		// R60: flex picks are listed in dp.FlexSlots; skip them here so
		// the same card never appears in both Solid and Flex.
		if flexNames[s.name] {
			continue
		}
		if s.score < solidMin || s.score >= solidMax {
			continue
		}
		reason := s.reason
		if reason == "" {
			// Build a default reason from the card's role mix and CMC
			// since solid picks usually lack a dramatic standout signal.
			roleNames := make([]string, 0, len(s.roles))
			for _, r := range s.roles {
				roleNames = append(roleNames, string(r))
			}
			switch {
			case len(roleNames) >= 2:
				reason = fmt.Sprintf("functional %s at CMC %d — okay but upgrade-shoppable",
					strings.Join(roleNames, "/"), s.cmc)
			case len(roleNames) == 1:
				reason = fmt.Sprintf("functional %s at CMC %d — flex slot",
					roleNames[0], s.cmc)
			default:
				reason = "fills a slot but not load-bearing"
			}
		}
		power := powerByName[s.name]
		dp.SolidCards = append(dp.SolidCards, CardQuality{
			Name:             s.name,
			Tier:             "solid",
			Reason:           reason,
			Power:            power,
			PowerTier:        PowerTierFor(power),
			PowerExplanation: explanationByName[s.name],
		})
		solidCount++
	}

	// Bottom cards with low scores = cuttable
	suggestedSwaps := suggestCuttableSwaps(dp, report)
	for i := len(scores) - 1; i >= 0 && i > len(scores)-6; i-- {
		s := scores[i]
		if s.score > 0 {
			continue
		}
		reason := s.reason
		if reason == "" {
			reason = "low synergy with deck strategy"
		}
		detected := s.detected
		whyCut := s.whyCut
		effect := s.effect
		if detected == "" {
			roleNames := make([]string, len(s.roles))
			for ri, r := range s.roles {
				roleNames[ri] = string(r)
			}
			roleStr := "no roles"
			if len(roleNames) > 0 {
				roleStr = strings.Join(roleNames, ", ")
			}
			detected = fmt.Sprintf("CMC %d, roles: %s, score %.1f", s.cmc, roleStr, s.score)
		}
		if whyCut == "" {
			whyCut = "Card scored at the bottom of the deck on the role/synergy/win-line evaluator. It contributes little to the detected gameplan."
		}
		if effect == "" {
			effect = "Removing it costs the deck almost nothing on offense or defense and clears a slot for a synergy-tagged replacement."
		}
		power := powerByName[s.name]
		dp.CuttableCards = append(dp.CuttableCards, CardQuality{
			Name:             s.name,
			Tier:             "cuttable",
			Reason:           reason,
			Power:            power,
			PowerTier:        PowerTierFor(power),
			PowerExplanation: explanationByName[s.name],
			Detected:         detected,
			WhyCut:           whyCut,
			Effect:           effect,
			Suggested:        suggestedSwaps,
		})
	}
}

// suggestCuttableSwaps returns a short, archetype-flavoured list of swap
// candidates. We keep this generic on purpose — the per-card "perfect upgrade"
// problem is too card-pool dependent to solve heuristically; the goal is to
// prompt the deckbuilder with categories worth shopping for.
func suggestCuttableSwaps(dp *DeckProfile, report *FreyaReport) []string {
	var out []string
	if dp == nil {
		return out
	}
	if dp.WinLineCount > 0 && len(dp.PrimaryWinLine) > 0 {
		out = append(out, "redundant copy of a missing combo piece in "+dp.PrimaryWinLine)
	}
	if report != nil && report.Stats != nil {
		if report.Stats.RampCount < 10 {
			out = append(out, fmt.Sprintf("ramp piece (deck has %d ramp sources, target ≥10)", report.Stats.RampCount))
		}
		if report.Stats.DrawSourceCount < 10 {
			out = append(out, fmt.Sprintf("draw engine (deck has %d draw sources, target ≥10)", report.Stats.DrawSourceCount))
		}
	}
	if dp.PrimaryArchetype != "" {
		out = append(out, "tagged synergy piece for "+dp.PrimaryArchetype)
	}
	out = append(out, "additional removal/interaction at low CMC")
	return out
}

// ---------------------------------------------------------------------------
// 8. Color weight optimization — suggest specific land swaps.
// ---------------------------------------------------------------------------

// dualLandRecommendations: a representative dual land per 2-color pair,
// used as the recommended swap-IN target when both colors are in the
// deck's identity. Picked from the Battlebond / "Crowded" cycle (Sea
// of Clouds / Bountiful Promenade / etc.) because those lands are
// budget-friendly, untapped in multiplayer, and don't punish basic-
// land budgets the way fetch / shock / OG dual cycles do. Key is the
// alphabetically-sorted 2-letter pair (e.g. "GW", not "WG") so a
// single lookup serves either argument order.
var dualLandRecommendations = map[string]string{
	"UW": "Sea of Clouds",
	"BW": "Vault of Champions",
	"RW": "Spectator Seating",
	"GW": "Bountiful Promenade",
	"BU": "Morphic Pool",
	"RU": "Training Center",
	"GU": "Rejuvenating Springs",
	"BR": "Luxury Suite",
	"BG": "Undergrowth Stadium",
	"GR": "Spire Garden",
}

// colorPairKey returns the alphabetically-sorted 2-letter key for a
// color pair (e.g. colorPairKey("G", "W") == "GW"). Used to look up
// dualLandRecommendations regardless of argument order.
func colorPairKey(a, b string) string {
	if a <= b {
		return a + b
	}
	return b + a
}

func computeLandSwapSuggestions(dp *DeckProfile, report *FreyaReport) {
	if report.Stats == nil {
		return
	}

	totalDemand := 0
	totalSupply := 0
	demand := map[string]int{}
	supply := map[string]int{}

	for _, c := range []string{"W", "U", "B", "R", "G"} {
		d := report.ColorDemand[c]
		s := report.ColorSupply[c]
		demand[c] = d
		supply[c] = s
		totalDemand += d
		totalSupply += s
	}

	if totalDemand == 0 || totalSupply == 0 {
		return
	}

	type colorImbalance struct {
		color     string
		demandPct float64
		supplyPct float64
		gap       float64
	}

	var imbalances []colorImbalance
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if demand[c] == 0 && supply[c] == 0 {
			continue
		}
		dPct := float64(demand[c]) / float64(totalDemand)
		sPct := float64(supply[c]) / float64(totalSupply)
		gap := dPct - sPct
		if math.Abs(gap) > 0.05 {
			imbalances = append(imbalances, colorImbalance{
				color: c, demandPct: dPct, supplyPct: sPct, gap: gap,
			})
		}
	}

	// Sort: biggest undersupply first.
	sort.Slice(imbalances, func(i, j int) bool {
		return imbalances[i].gap > imbalances[j].gap
	})

	basicNames := map[string]string{"W": "Plains", "U": "Island", "B": "Swamp", "R": "Mountain", "G": "Forest"}

	// Collect the deck's actual colors (any color with non-zero demand
	// OR non-zero supply). Used to pick a dual replacement whose second
	// color is something the deck actually wants.
	deckColors := map[string]bool{}
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if demand[c] > 0 || supply[c] > 0 {
			deckColors[c] = true
		}
	}

	// Names of cards already in the deck (lowercased) — used to avoid
	// recommending a dual the player already runs.
	inDeck := map[string]bool{}
	for _, p := range report.Profiles {
		inDeck[strings.ToLower(p.Name)] = true
	}

	for _, ib := range imbalances {
		if ib.gap <= 0.05 {
			continue
		}
		// Find an oversupplied color to swap from.
		var over *colorImbalance
		for i := range imbalances {
			if imbalances[i].gap < -0.05 {
				over = &imbalances[i]
				break
			}
		}
		if over == nil {
			continue
		}

		// Find a specific nonbasic land in the deck that produces the
		// oversupplied color but does NOT also produce the undersupplied
		// color (those duals are already pulling their weight). Prefer
		// lands that produce ONLY the oversupplied color (single-color
		// nonbasics, e.g. Mishra's Factory in a R deck) over lands that
		// produce the oversupplied color alongside other healthy colors —
		// the more "stranded" the land is on the wrong color, the more
		// actionable the swap.
		swapOut := pickLandSwapCandidate(report, over.color, ib.color)

		// Pick a replacement: dual covering the undersupplied color +
		// any other in-deck color (preferring the most-demanded peer),
		// falling back to the basic of the undersupplied color when no
		// curated dual matches the deck's color identity.
		swapIn := pickReplacementDual(ib.color, deckColors, demand, inDeck)
		if swapIn == "" {
			swapIn = basicNames[ib.color]
		}

		if swapOut != "" {
			dp.LandSwapSuggestions = append(dp.LandSwapSuggestions,
				fmt.Sprintf("Recommend swap: %s → %s (%s demand high, %s demand low)",
					swapOut, swapIn, ib.color, over.color))
		} else {
			// Fall back to the legacy generic suggestion — useful when
			// the deck's only oversupplied source is its basics (which
			// aren't in report.Profiles), since the player can act on
			// "Replace 1 Mountain with 1 Forest" themselves.
			dp.LandSwapSuggestions = append(dp.LandSwapSuggestions,
				fmt.Sprintf("Replace 1 %s with 1 %s: %s has %.0f%% demand but only %.0f%% sources",
					basicNames[over.color], swapIn,
					ib.color, ib.demandPct*100, ib.supplyPct*100))
		}
	}

	if len(dp.LandSwapSuggestions) > 3 {
		dp.LandSwapSuggestions = dp.LandSwapSuggestions[:3]
	}
}

// pickLandSwapCandidate scans the deck's nonbasic lands for one that
// produces overColor but NOT underColor. Returns the most-actionable
// candidate name (single-color nonbasics first, then multi-color lands
// that don't include underColor), or "" if no candidate exists.
func pickLandSwapCandidate(report *FreyaReport, overColor, underColor string) string {
	type cand struct {
		name        string
		colorsCount int
	}
	var cands []cand
	for _, p := range report.Profiles {
		if !p.IsLand {
			continue
		}
		hasOver := false
		hasUnder := false
		for _, c := range p.LandColors {
			if c == overColor {
				hasOver = true
			}
			if c == underColor {
				hasUnder = true
			}
		}
		if !hasOver || hasUnder {
			continue
		}
		cands = append(cands, cand{name: p.Name, colorsCount: len(p.LandColors)})
	}
	if len(cands) == 0 {
		return ""
	}
	// Prefer the most-stranded candidate (fewest colors produced — a
	// mono-color nonbasic like Mishra's Factory is more swap-worthy
	// than a 3-color rainbow land that just happens not to make the
	// undersupplied color). Stable-sort by colorsCount asc, then name
	// asc to keep output deterministic.
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].colorsCount != cands[j].colorsCount {
			return cands[i].colorsCount < cands[j].colorsCount
		}
		return cands[i].name < cands[j].name
	})
	return cands[0].name
}

// pickReplacementDual picks a curated dual land that produces
// underColor + the in-deck color with the highest demand (excluding
// underColor itself, since the second-color slot already produces it).
// Skips duals the deck already runs. Returns "" if no suitable dual
// exists (mono-color deck, or every viable dual is already in-deck).
func pickReplacementDual(underColor string, deckColors map[string]bool, demand map[string]int, inDeck map[string]bool) string {
	// Order peer colors by demand desc so we recommend a dual whose
	// second color the deck actually wants. WUBRG iteration with a
	// stable demand-desc sort.
	type peer struct {
		color  string
		demand int
	}
	var peers []peer
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if c == underColor || !deckColors[c] {
			continue
		}
		peers = append(peers, peer{color: c, demand: demand[c]})
	}
	sort.SliceStable(peers, func(i, j int) bool {
		if peers[i].demand != peers[j].demand {
			return peers[i].demand > peers[j].demand
		}
		return peers[i].color < peers[j].color
	})
	for _, p := range peers {
		key := colorPairKey(underColor, p.color)
		name, ok := dualLandRecommendations[key]
		if !ok {
			continue
		}
		if inDeck[strings.ToLower(name)] {
			continue
		}
		return name
	}
	return ""
}

// ---------------------------------------------------------------------------
// 9. Deck personality blurb — 4-6 sentence narrative paragraph that names
//    specific cards driving the personality call (commander, marquee star
//    cards, finisher pieces, pet picks), plus a flavor-text-style tagline.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Curve vs. archetype fit — warn when AvgCMC doesn't match the
// archetype's expected curve band.
// ---------------------------------------------------------------------------

// archetypeCurveExpectation pairs each archetype with the avg-CMC band
// that the archetype's gameplan demands. Values are drawn from
// observed corpus distributions + archetype theory:
//
//   - Fast / cheap-cost shells (Combo / Storm / Aggro / Stax / Spellslinger
//     / Cycling / Toxic / Vehicles / Group Slug / Voltron / Discard)
//     need to assemble or pressure on turns 3-5; high CMC means the
//     deck telegraphs without the cheap-piece density to back it up.
//   - Midweight grinders (Tribal / Aristocrats / Lifegain / Enchantress
//     / Counters Matter / Selfmill / Blink / Mill / Damage Redirect /
//     Theft) sit in the 2.4-3.5 sweet spot — too lean misses payoffs,
//     too heavy stalls.
//   - Top-end shells (Reanimator / Lands Matter / Control / Pillowfort
//     / Group Hug / Extra Combats / Artifacts / Superfriends) want
//     2.8-5.0; under-curved versions look like midrange shells without
//     finishers, over-curved versions can't cast anything pre-ramp.
//
// Midrange has no expectation entry — it's the "anything" fallback,
// and the existing curve-warning pass in analysis.go already covers
// shape signals (bimodal / top-heavy / bottom-light) independent of
// archetype. Toleranced by ±0.2 in computeCurveArchetypeFit so
// borderline curves don't trip false warnings.
var archetypeCurveExpectation = map[string]struct {
	MinAvgCMC float64
	MaxAvgCMC float64
	// Band is a human-readable shorthand ("lean" / "midweight" /
	// "top-heavy") used in the warning text so builders see the
	// expected curve flavor at a glance.
	Band string
}{
	"Combo":           {0.0, 2.7, "lean"},
	"Storm":           {0.0, 2.5, "lean"},
	"Aggro":           {0.0, 2.8, "lean"},
	"Voltron":         {0.0, 3.0, "lean"},
	"Spellslinger":    {0.0, 2.8, "lean"},
	"Cycling":         {0.0, 2.8, "lean"},
	"Toxic":           {0.0, 2.8, "lean"},
	"Vehicles":        {0.0, 3.0, "lean"},
	"Group Slug":      {0.0, 2.8, "lean"},
	"Stax":            {0.0, 2.8, "lean"},
	"Discard":         {2.0, 3.2, "midweight"},
	"Tribal":          {2.0, 3.5, "midweight"},
	"Aristocrats":     {2.0, 3.2, "midweight"},
	"Lifegain":        {2.5, 3.5, "midweight"},
	"Enchantress":     {2.5, 3.5, "midweight"},
	"Selfmill":        {2.5, 3.5, "midweight"},
	"Counters Matter": {2.2, 3.4, "midweight"},
	"Blink":           {2.5, 3.5, "midweight"},
	"Damage Redirect": {2.5, 3.5, "midweight"},
	"Mill":            {2.5, 3.5, "midweight"},
	"Theft / Clone":   {2.5, 3.8, "midweight"},
	"Reanimator":      {2.8, 4.2, "top-heavy"},
	"Lands Matter":    {2.8, 4.0, "top-heavy"},
	"Control":         {3.0, 5.0, "top-heavy"},
	"Pillowfort":      {2.8, 4.0, "top-heavy"},
	"Group Hug":       {2.8, 4.0, "top-heavy"},
	"Extra Combats":   {2.8, 4.0, "top-heavy"},
	"Artifacts":       {2.5, 4.0, "top-heavy"},
	"Superfriends":    {3.0, 5.0, "top-heavy"},
}

// curveArchetypeTolerance is the slack we allow before flagging an
// AvgCMC as outside the archetype's expected band. 0.2 lets a Combo
// deck with avgCMC 2.85 (slightly over the 2.7 max) pass without a
// warning — most decks have at least one finisher that bumps the
// average, and we don't want to false-positive borderline shells.
const curveArchetypeTolerance = 0.2

// computeCurveArchetypeFit checks whether dp.AvgCMC falls within the
// expected curve band for dp.PrimaryArchetype. Mismatches that exceed
// the ±0.2 tolerance produce a one-line warning in
// dp.CurveArchetypeWarnings. Archetypes not in the expectation map
// (notably Midrange, the generic fallback) are skipped — the existing
// shape-based curve warnings in report.CurveWarnings cover those
// independently of archetype.
func computeCurveArchetypeFit(dp *DeckProfile, report *FreyaReport) {
	if dp == nil || dp.PrimaryArchetype == "" {
		return
	}
	exp, ok := archetypeCurveExpectation[dp.PrimaryArchetype]
	if !ok {
		return
	}
	// Avoid false-firing on tiny decks where AvgCMC is unstable
	// (commander-only test fixtures, partial parses).
	if report == nil || report.NonlandCount < 20 {
		return
	}
	avg := dp.AvgCMC
	if avg < exp.MinAvgCMC-curveArchetypeTolerance {
		dp.CurveArchetypeWarnings = append(dp.CurveArchetypeWarnings,
			fmt.Sprintf("%s archetype expects a %s curve (avg %.1f-%.1f) but deck avg CMC is %.2f — curve plays faster than the archetype gameplan needs; consider adding heavier payoffs",
				dp.PrimaryArchetype, exp.Band, exp.MinAvgCMC, exp.MaxAvgCMC, avg))
		return
	}
	if avg > exp.MaxAvgCMC+curveArchetypeTolerance {
		dp.CurveArchetypeWarnings = append(dp.CurveArchetypeWarnings,
			fmt.Sprintf("%s archetype expects a %s curve (avg %.1f-%.1f) but deck avg CMC is %.2f — curve is too heavy for the archetype's tempo; consider cutting top-end cards or adding ramp",
				dp.PrimaryArchetype, exp.Band, exp.MinAvgCMC, exp.MaxAvgCMC, avg))
	}
}

// buildPersonalityBlurb produces the 4-6 sentence narrative paragraph.
// Structure:
//
//	1. Opening — speed + archetype + commander framing (always emits)
//	2. Approach — what the deck does in play (existing describeApproach)
//	3. Engine — names 2-3 specific star/power-tier cards anchoring the plan
//	4. Closer — names finisher pieces from primary win line (existing
//	            describeCloser already does the naming for combo/finisher/
//	            commander_damage/alt_wincon types)
//	5. Texture — pet picks if present, else mana base / protection /
//	             bracket-flavored signature line
//	6. (Optional) Final tag — bracket-aware closing thought, only when
//	            distinct from sentences 1-5 to keep us inside the 6-cap
//
// Every sentence has a fallback so the function always emits at least
// 4 sentences on a minimally-populated DeckProfile.
func buildPersonalityBlurb(dp *DeckProfile, report *FreyaReport) string {
	// Sections are ordered by importance. The first four (opening,
	// approach, engine, closer) always emit — they form the 4-sentence
	// floor. The last two (texture, final tag) are optional and get
	// trimmed if the closer used its backup-line bonus sentence and we
	// would otherwise blow past the 6-sentence ceiling. describeCloser
	// can return 1-2 sentences depending on the presence of backup win
	// lines, which is why we count after each append rather than just
	// taking the first N.
	core := []string{
		openingSentence(dp),
		describeApproach(dp, report),
		engineSentence(dp, report),
		describeCloser(dp, report),
	}
	optional := []string{
		textureSentence(dp, report),
		finalTagSentence(dp, report, core),
	}

	emit := func(out []string, s string) []string {
		s = strings.TrimSpace(s)
		if s == "" {
			return out
		}
		if !endsInSentencePunctuation(s) {
			s += "."
		}
		return append(out, s)
	}

	out := make([]string, 0, len(core)+len(optional))
	for _, s := range core {
		out = emit(out, s)
	}
	// Append optional sections only while we stay within the 6-sentence
	// ceiling. A section that itself contains multiple sentences (like
	// describeCloser with a backup line) counts toward the cap as a
	// single appended string, but countSentences inspects punctuation
	// across the joined blurb so the trim decision is correct.
	for _, s := range optional {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !endsInSentencePunctuation(s) {
			s += "."
		}
		candidate := append(out, s)
		if blurbSentenceCount(strings.Join(candidate, " ")) > 6 {
			continue
		}
		out = candidate
	}
	return strings.Join(out, " ")
}

// blurbSentenceCount counts how many `[.!?] ` (final-punct + space)
// boundaries the blurb contains, plus 1 if the string ends with
// sentence punctuation. MTG card names contain commas and apostrophes
// but NOT internal periods, so the heuristic does not false-fire on
// embedded card names. Local to advanced.go so the production trim
// path doesn't depend on a test helper.
func blurbSentenceCount(s string) int {
	n := 0
	for i := 0; i < len(s)-1; i++ {
		c := s[i]
		if (c == '.' || c == '!' || c == '?') && s[i+1] == ' ' {
			n++
		}
	}
	if s != "" {
		last := s[len(s)-1]
		if last == '.' || last == '!' || last == '?' {
			n++
		}
	}
	return n
}

// openingSentence names the commander whenever one is set; the commander
// is the deck's flagship reference point and naming it grounds the rest
// of the blurb. Falls back to a commanderless opener for partial deck
// fixtures so the function stays robust.
func openingSentence(dp *DeckProfile) string {
	speed := describeSpeed(dp)
	arch := dp.PrimaryArchetype
	if arch == "" {
		arch = "Commander"
	}
	if dp.Commander != "" {
		return fmt.Sprintf("This is a %s %s deck led by %s.", speed, arch, dp.Commander)
	}
	return fmt.Sprintf("This is a %s %s deck.", speed, arch)
}

// engineSentence names 2-3 marquee cards that anchor the deck's engine,
// drawn from StarCards (the synergy-tier classifier) and falling back
// through CardPowerLevels (top-power sort) and GameChangerCards. Every
// branch produces a sentence — readers should always learn at least one
// card name beyond the commander.
func engineSentence(dp *DeckProfile, report *FreyaReport) string {
	commander := dp.Commander
	named := topNamedCards(dp, 3, commander)
	if len(named) == 0 {
		return engineSentenceFallback(dp, report)
	}
	role := dominantEngineRole(dp)
	joined := joinWithSerialAnd(named)
	switch {
	case len(named) >= 3:
		return fmt.Sprintf("%s anchor the %s package, each pulling double duty across the gameplan.", joined, role)
	case len(named) == 2:
		return fmt.Sprintf("%s carry the %s package between them, shaping how every turn unfolds.", joined, role)
	default:
		return fmt.Sprintf("%s is the marquee piece — when it lands, the %s package starts firing.", joined, role)
	}
}

// engineSentenceFallback runs when no star cards survived the synergy
// classifier (very low-power decks, partial parses). Falls back to
// commander-synergy framing or a generic gameplan note so the blurb
// still gets a third sentence.
func engineSentenceFallback(dp *DeckProfile, report *FreyaReport) string {
	if dp.CommanderSynergy >= 0.40 && dp.Commander != "" {
		pct := int(dp.CommanderSynergy * 100)
		return fmt.Sprintf("Roughly %d%% of the deck synergizes directly with %s — the engine IS the commander.", pct, dp.Commander)
	}
	if report != nil && report.NonLandTutorCount >= 5 {
		return fmt.Sprintf("A deep tutor package (%d searchers) keeps the right card in hand at the right moment.", report.NonLandTutorCount)
	}
	if dp.WinLineCount >= 3 {
		return fmt.Sprintf("With %d distinct win lines, the deck stays flexible against whatever the table presents.", dp.WinLineCount)
	}
	return "The engine runs on synergy more than marquee pieces — each card pulls a small share of the load."
}

// textureSentence is the 5th-sentence "personality texture" line.
// Pet cards (flavor picks the deckbuilder kept despite off-archetype
// fit) are the highest-signal texture; we always lead with them when
// they exist. Otherwise we fall through to mana-base / protection /
// bracket signatures.
func textureSentence(dp *DeckProfile, report *FreyaReport) string {
	if pets := selectPetNames(dp, 2); len(pets) > 0 {
		joined := joinWithSerialAnd(pets)
		if len(pets) == 1 {
			return fmt.Sprintf("%s is the builder's flavor pick — kept on the list despite the optimization cost.", joined)
		}
		return fmt.Sprintf("%s are the builder's flavor picks, kept on the list despite the optimization cost — personality earns its slot here.", joined)
	}
	if dp.ProtectedKeyPieces >= 4 && dp.UnprotectedKeyPieces <= dp.ProtectedKeyPieces/2 {
		return fmt.Sprintf("%d key pieces carry their own protection — the plan is built to survive interaction, not just to outpace it.", dp.ProtectedKeyPieces)
	}
	if dp.ManaBaseGrade == "A" {
		return fmt.Sprintf("The mana base is grade A — the deck rarely stumbles on color, and every land drop is intentional.")
	}
	if dp.ManaBaseGrade == "D" || dp.ManaBaseGrade == "F" {
		return fmt.Sprintf("The mana base is the weakest link (grade %s) — color screw and tapland tempo loss are the real opponents here.", dp.ManaBaseGrade)
	}
	if report != nil && report.NonlandCount > 0 && dp.RampCount >= 12 {
		return fmt.Sprintf("Ramp-heavy at %d ramp pieces — the deck wants to be two turns ahead before the real plan starts.", dp.RampCount)
	}
	if dp.CommanderSynergy >= 0.55 {
		return fmt.Sprintf("Commander synergy is unusually tight at %d%% — the deck has one voice, not many.", int(dp.CommanderSynergy*100))
	}
	return "The texture is balanced — no marquee weaknesses, no signature flourishes, just a deck that knows what it wants to do."
}

// finalTagSentence is the optional 6th sentence. Only emits when it adds
// something distinct from sentences 1-5 (avoid restating bracket if the
// closer already named the bracket). Keeps the blurb inside the 6-cap.
func finalTagSentence(dp *DeckProfile, report *FreyaReport, prior []string) string {
	priorText := strings.Join(prior, " ")
	mentionedBracket := strings.Contains(priorText, "bracket") || strings.Contains(priorText, "Bracket")
	if !mentionedBracket && dp.Bracket >= 4 {
		return fmt.Sprintf("Built at bracket %d (%s) — this is a deck that demands respect from turn one.", dp.Bracket, dp.BracketLabel)
	}
	if !mentionedBracket && dp.Bracket == 1 {
		return fmt.Sprintf("A bracket 1 (%s) build — casual, social, and built to enjoy the long game.", dp.BracketLabel)
	}
	if dp.PowerPercentile >= 85 {
		return fmt.Sprintf("Power lands in the %dth percentile of its archetype — the deck plays above its weight class.", dp.PowerPercentile)
	}
	return ""
}

// topNamedCards returns up to `max` star-card names (CardQuality at
// "star" tier), falling back through CardPowerLevels and
// GameChangerCards, deduped and excluding the commander (we already
// name it in sentence 1).
func topNamedCards(dp *DeckProfile, max int, excludeCommander string) []string {
	seen := map[string]bool{}
	if excludeCommander != "" {
		seen[strings.ToLower(excludeCommander)] = true
	}
	out := []string{}
	push := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	for _, c := range dp.StarCards {
		if len(out) >= max {
			break
		}
		push(c.Name)
	}
	for _, c := range dp.CardPowerLevels {
		if len(out) >= max {
			break
		}
		if c.PowerTier == "S" || c.PowerTier == "A" {
			push(c.Name)
		}
	}
	for _, name := range dp.GameChangerCards {
		if len(out) >= max {
			break
		}
		push(name)
	}
	return out
}

// selectPetNames returns up to `max` pet-card names. Legendaries first
// (the signature-flavor reason carries the strongest personality
// signal), then nonlegendaries in detection order.
func selectPetNames(dp *DeckProfile, max int) []string {
	if max <= 0 || len(dp.PetCards) == 0 {
		return nil
	}
	legendaries := []string{}
	others := []string{}
	for _, p := range dp.PetCards {
		if strings.Contains(strings.ToLower(p.Reason), "signature flavor") {
			legendaries = append(legendaries, p.Name)
		} else {
			others = append(others, p.Name)
		}
	}
	out := append(legendaries, others...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// dominantEngineRole names what kind of package the engine cards
// anchor. Archetype-keyed so the blurb's vocabulary matches the
// deck's identity — Combo decks anchor a "kill package", Aristocrats
// anchor a "drain engine", and so on.
func dominantEngineRole(dp *DeckProfile) string {
	switch strings.ToLower(dp.PrimaryArchetype) {
	case "combo":
		return "kill"
	case "storm":
		return "burst"
	case "aristocrats":
		return "drain"
	case "reanimator":
		return "graveyard"
	case "control":
		return "answer"
	case "stax":
		return "lock"
	case "voltron":
		return "commander"
	case "tribal":
		return "tribal"
	case "lifegain":
		return "lifegain"
	case "enchantress":
		return "enchantment"
	case "artifacts":
		return "artifact"
	case "lands matter":
		return "landfall"
	case "blink", "flicker":
		return "ETB"
	case "mill":
		return "mill"
	case "spellslinger":
		return "spellslinger"
	case "counters matter":
		return "counters"
	case "superfriends":
		return "planeswalker"
	case "aggro", "go wide":
		return "pressure"
	case "extra combats":
		return "combat"
	case "pillowfort":
		return "defensive"
	case "group hug":
		return "value"
	case "group slug":
		return "burn"
	case "toxic":
		return "toxic"
	default:
		return "value"
	}
}

// joinWithSerialAnd renders a slice as "A", "A and B", or
// "A, B, and C" — the Oxford-comma form, picked because the blurb
// register leans literary.
func joinWithSerialAnd(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func endsInSentencePunctuation(s string) bool {
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	return last == '.' || last == '!' || last == '?'
}

// buildPersonalityTagline produces the flavor-text-style one-liner.
// Templates are archetype-keyed; each leaves a single optional slot
// for a deck-specific noun (top star card, finisher piece, or
// commander) so the same archetype on two different decks reads
// differently. Always non-empty — the default falls back to a
// generic "play to win" register.
func buildPersonalityTagline(dp *DeckProfile, report *FreyaReport) string {
	piece := taglineSlotCard(dp, report)
	commander := dp.Commander
	arch := strings.ToLower(dp.PrimaryArchetype)
	switch {
	case strings.Contains(arch, "storm"):
		return "One spell. Then ten. Then the silence between them."
	case strings.Contains(arch, "combo"):
		if piece != "" {
			return fmt.Sprintf("All the time in the world — and only %s left to draw.", piece)
		}
		return "All the time in the world. All the answers already on the table."
	case strings.Contains(arch, "aristocrats"):
		return "Death is the engine. Sacrifice is the fuel."
	case strings.Contains(arch, "reanimator"):
		if commander != "" {
			return fmt.Sprintf("%s remembers what was buried.", commander)
		}
		return "The grave does not forget what it was given."
	case strings.Contains(arch, "voltron"):
		if commander != "" {
			return fmt.Sprintf("Hand %s the blade — the rest is arithmetic.", commander)
		}
		return "One sword. One swing. One winner."
	case strings.Contains(arch, "stax"):
		return "Take a turn. Try a spell. Discover what is permitted."
	case strings.Contains(arch, "control"):
		return "Every threat answered. Every door closed. Every game ours."
	case strings.Contains(arch, "tribal"):
		return "The tribe gathers. The tribe remembers. The tribe wins."
	case strings.Contains(arch, "lifegain"):
		return "The body endures. The body remembers. The body collects."
	case strings.Contains(arch, "lands"):
		return "Every land a door. Every door a doom."
	case strings.Contains(arch, "enchantress"):
		return "Each parchment a small geometry. Together — gravity."
	case strings.Contains(arch, "artifact"):
		return "The forge speaks last. The forge speaks loudest."
	case strings.Contains(arch, "counters matter"):
		return "One counter at a time. Then ten thousand."
	case strings.Contains(arch, "toxic"):
		return "The wound is small. The wound is final."
	case strings.Contains(arch, "blink"), strings.Contains(arch, "flicker"):
		return "Step out of the world. Step back. Profit."
	case strings.Contains(arch, "mill"):
		return "Twenty cards. Then twenty more. Then nothing."
	case strings.Contains(arch, "spellslinger"):
		return "The page burns. The ink burns. The reader endures."
	case strings.Contains(arch, "extra combats"):
		return "Once more, with feeling — and again, until it ends."
	case strings.Contains(arch, "superfriends"):
		return "Each loyalty tick a verdict the table cannot appeal."
	case strings.Contains(arch, "pillowfort"):
		return "The walls remember every wound they refused."
	case strings.Contains(arch, "group hug"):
		return "Take the gifts. Take the cards. Take the loss."
	case strings.Contains(arch, "aggro"), strings.Contains(arch, "go wide"):
		return "Faster than fear. Louder than warning."
	case strings.Contains(arch, "theft"), strings.Contains(arch, "clone"):
		return "Your threat. My turn. Same outcome."
	case strings.Contains(arch, "discard"):
		return "Empty their hand. Then empty the table."
	case strings.Contains(arch, "ramp"):
		return "Two turns ahead. One conversation behind."
	default:
		if piece != "" {
			return fmt.Sprintf("Played carefully. Played patiently. Played until %s lands.", piece)
		}
		return "Played carefully. Played patiently. Played to win."
	}
}

// taglineSlotCard picks a deck-specific noun for the tagline templates
// that accept one. Prefers the primary win-line's first piece (the
// actual finisher reader most wants named), then the top star card.
// Returns empty when no card name is available.
func taglineSlotCard(dp *DeckProfile, report *FreyaReport) string {
	if report != nil && report.WinLines != nil && len(report.WinLines.WinLines) > 0 {
		wl := report.WinLines.WinLines[0]
		if len(wl.Pieces) > 0 {
			name := strings.TrimSpace(wl.Pieces[0])
			if name != "" && !strings.EqualFold(name, dp.Commander) {
				return name
			}
		}
	}
	for _, c := range dp.StarCards {
		name := strings.TrimSpace(c.Name)
		if name != "" && !strings.EqualFold(name, dp.Commander) {
			return name
		}
	}
	return ""
}

// describeSpeed factors in both curve and ramp density. A 4.0 avg CMC with
// 14 ramp pieces is "explosive" (the ramp is part of the plan), not "slow
// but devastating" — and a 2.2 avg with no ramp is "lightning-fast"
// regardless. Pure CMC alone misreads ramp decks.
func describeSpeed(dp *DeckProfile) string {
	switch {
	case dp.AvgCMC < 2.5:
		return "lightning-fast"
	case dp.AvgCMC < 3.0:
		return "agile"
	case dp.AvgCMC > 3.8 && dp.RampCount >= 12:
		return "explosive"
	case dp.AvgCMC > 3.8 && dp.RampCount < 8:
		return "lumbering"
	case dp.AvgCMC > 3.8:
		return "slow but devastating"
	case dp.RampCount >= 12:
		return "ramp-fueled"
	default:
		return "methodical"
	}
}

func describeApproach(dp *DeckProfile, report *FreyaReport) string {
	arch := strings.ToLower(dp.PrimaryArchetype)
	switch {
	case containsAny(arch, "storm"):
		return "It chains rituals and cantrips into a single explosive turn, riding the storm count to a one-shot kill."
	case containsAny(arch, "combo", "infinite"):
		if dp.HasTutorAccess && report.NonLandTutorCount >= 5 {
			return "It assembles its kill with surgical precision, tutoring combo pieces while holding up protection."
		}
		return "It digs aggressively for its combo pieces, racing to assemble a kill before opponents can disrupt it."
	case containsAny(arch, "stax"):
		return "It locks the table down with asymmetric resource denial, taxing every spell and untap until opponents have no moves left."
	case containsAny(arch, "control"):
		return "It answers threats one by one, drawing extra cards off the exchange until the table runs out of pressure."
	case containsAny(arch, "voltron"):
		return "It suits up its commander and swings for lethal, protecting its investment with shields and counters."
	case containsAny(arch, "aristocrats"):
		return "It feeds the death machine — sacrificing and recurring creatures in a loop of incremental drains that bypass combat entirely."
	case containsAny(arch, "reanimator"):
		return "It cheats massive threats into play from the graveyard, bypassing mana costs for devastating early haymakers."
	case containsAny(arch, "enchantress"):
		return "It weaves a web of enchantments, drawing cards off each one until the value engine becomes unstoppable."
	case containsAny(arch, "artifact"):
		return "It snowballs an artifact board into mana, draw, and lethal payoffs that win on overwhelming density."
	case containsAny(arch, "lands"):
		return "It turns land drops into a resource engine, triggering landfall chains that generate exponential value."
	case containsAny(arch, "blink", "flicker"):
		return "It blinks creatures in and out of existence, squeezing maximum value from every ETB trigger."
	case containsAny(arch, "mill"):
		return "It attacks libraries instead of life totals, grinding opponents out card by card until they draw from nothing."
	case containsAny(arch, "superfriends"):
		return "It deploys an army of planeswalkers, ticking up loyalty counters behind a wall of protection until ultimates end the game."
	case containsAny(arch, "lifegain"):
		return "It builds an unkillable life buffer and turns each gain trigger into incremental advantage, draining the table once the engine clicks."
	case containsAny(arch, "spellslinger"):
		return "It chains instants and sorceries to fuel magecraft and prowess payoffs, snowballing each spell into the next."
	case containsAny(arch, "counters matter"):
		return "It stacks +1/+1 counters and proliferates them across its board, turning a single threat into a lethal one in a turn cycle."
	case containsAny(arch, "extra combats"):
		return "It chains attack steps to swing for unblockable lethal in a single turn, weaponizing haste and double strike."
	case containsAny(arch, "theft", "clone"):
		return "It hijacks the strongest pieces on the table, turning opponents' threats and engines into its own win condition."
	case containsAny(arch, "ninjutsu", "evasion"):
		return "It tempos in cheap evasive creatures and ninjutsus bigger threats onto an unprotected board, racing damage past blockers."
	case containsAny(arch, "discard", "hand attack"):
		return "It strips opponents' hands before they can act, leaving the table topdecking while it deploys threats into empty boards."
	case containsAny(arch, "tribal"):
		return "It builds a tribal swarm — every creature pumps the next, and the lord effects compound until the board overwhelms."
	case containsAny(arch, "aggro", "go wide"):
		return "It floods the board and turns sideways, overwhelming opponents before they can stabilize."
	case containsAny(arch, "ramp"):
		return "It powers out mana well ahead of curve, then drops haymakers two turns before the table can answer them."
	case containsAny(arch, "group hug"):
		return "It hands out cards and mana to keep the table happy, then quietly tips the politics — and the win — its own way."
	case containsAny(arch, "midrange"):
		return "It trades resources efficiently, leaning on card-for-card value until its threats outclass whatever opponents have left."
	default:
		return "It plays a flexible game, adapting its strategy based on the table and finding the right moment to strike."
	}
}

// describeCloser prefers naming the actual primary win line over a generic
// "X paths to victory" count — readers want to know HOW the deck wins, not
// just that it can. Falls back to bracket / synergy framing when the win
// line is generic combat damage with no named finisher.
func describeCloser(dp *DeckProfile, report *FreyaReport) string {
	if note := finisherNote(dp, report); note != "" {
		return note
	}
	if dp.Bracket >= 4 {
		return fmt.Sprintf("Built at bracket %d (%s), this is a deck that demands respect from turn one.", dp.Bracket, dp.BracketLabel)
	}
	if dp.WinLineCount >= 3 {
		return fmt.Sprintf("With %d paths to victory, it always has a plan B.", dp.WinLineCount)
	}
	if dp.CommanderSynergy >= 0.50 {
		return "Tightly built around its commander's strengths, every card pulls its weight."
	}
	if dp.Bracket <= 2 {
		return fmt.Sprintf("A casual %s build geared for table presence over speed.", dp.BracketLabel)
	}
	return fmt.Sprintf("A solid %s build that rewards patient piloting.", dp.BracketLabel)
}

// finisherNote names the primary win line when it points at a specific
// combo or finisher card. Returns empty for generic combat damage with no
// named card, so the caller can fall through to bracket/synergy framing.
func finisherNote(dp *DeckProfile, report *FreyaReport) string {
	if report == nil || report.WinLines == nil || len(report.WinLines.WinLines) == 0 {
		return ""
	}
	wl := report.WinLines.WinLines[0]
	pieces := strings.Join(wl.Pieces, " + ")
	if pieces == "" {
		return ""
	}
	backup := ""
	if dp.WinLineCount >= 3 {
		backup = fmt.Sprintf(" %d backup lines stand behind it.", dp.WinLineCount-1)
	} else if dp.WinLineCount == 2 {
		backup = " A backup line waits if the primary gets answered."
	}
	switch wl.Type {
	case "infinite":
		return fmt.Sprintf("The kill is %s — an infinite loop that ends the game outright.%s", pieces, backup)
	case "determined":
		return fmt.Sprintf("It closes through %s, a deterministic line that wins on resolution.%s", pieces, backup)
	case "finisher":
		return fmt.Sprintf("%s is the trigger that ends games, dropping the table from a stable board state to zero in one turn.%s", pieces, backup)
	case "commander_damage":
		if dp.Bracket >= 4 {
			return fmt.Sprintf("21 commander damage from %s is the primary close — and the clock starts the turn it lands.", dp.Commander)
		}
		return fmt.Sprintf("21 commander damage from %s is the primary close.", dp.Commander)
	case "alt_wincon":
		return fmt.Sprintf("%s is an alternate-win condition the table has to answer specifically.%s", pieces, backup)
	}
	// "combat" and unknown types: don't name a generic Pieces string —
	// caller falls back to bracket/synergy framing.
	return ""
}

// ---------------------------------------------------------------------------
// 10. Power percentile — estimated ranking within archetype.
// ---------------------------------------------------------------------------

func estimatePowerPercentile(dp *DeckProfile, report *FreyaReport) (int, []string) {
	score := 50 // start at median
	var factors []string

	// Tutor density — NonLandTutorCount only. Land tutors are ramp and are
	// scored separately under the ramp/manabase factors.
	if report.NonLandTutorCount >= 8 {
		score += 15
		factors = append(factors, fmt.Sprintf("deep tutor package (%d) → +15", report.NonLandTutorCount))
	} else if report.NonLandTutorCount >= 5 {
		score += 8
		factors = append(factors, fmt.Sprintf("solid tutor access (%d) → +8", report.NonLandTutorCount))
	} else if report.NonLandTutorCount <= 1 {
		score -= 10
		factors = append(factors, fmt.Sprintf("minimal tutors (%d) → -10", report.NonLandTutorCount))
	}

	// Win line count
	if dp.WinLineCount >= 5 {
		score += 10
		factors = append(factors, fmt.Sprintf("diverse win lines (%d) → +10", dp.WinLineCount))
	} else if dp.WinLineCount >= 3 {
		score += 5
	} else if dp.WinLineCount <= 1 {
		score -= 10
		factors = append(factors, "limited win conditions → -10")
	}

	// Win line QUALITY — separate signal from raw count. A deck with one
	// 2-card Thoracle line (Weight ~17) scores above a deck with three
	// 5-card grindy assemblies (~24 total but each piece is fragile + slow
	// to assemble). Bands tuned against representative decks: cEDH
	// Thassa's Oracle pile lands ~30-50 weighted, Bracket-4 combo decks
	// ~15-25, midrange goodstuff ~5-12, casual battlecruiser ~2-6.
	if dp.WeightedWinLineScore >= 30 {
		score += 8
		factors = append(factors, fmt.Sprintf("elite win-line quality (weighted %d) → +8", dp.WeightedWinLineScore))
	} else if dp.WeightedWinLineScore >= 15 {
		score += 4
		factors = append(factors, fmt.Sprintf("strong win-line quality (weighted %d) → +4", dp.WeightedWinLineScore))
	} else if dp.WeightedWinLineScore > 0 && dp.WeightedWinLineScore < 5 {
		score -= 4
		factors = append(factors, fmt.Sprintf("low win-line quality (weighted %d) → -4", dp.WeightedWinLineScore))
	}

	// Mana base quality. r60 rebalance: pre-r60 the table was
	//   A → +10 (logged), B → +5 (SILENT), C → 0, D → -10, F → -10
	// which had three concrete bugs:
	//   (1) B-grade silently credited +5 with no factor message —
	//       users couldn't see that their mana base was contributing,
	//       since factors[] is the user-facing explanation. Audit on
	//       data/decks/test (16-deck corpus) showed 4 of 16 decks
	//       received a silent +5 with zero attribution.
	//   (2) C-grade ignored — a mediocre mana base (passable but
	//       not great, score 60-74) was scored identically to a
	//       perfect-but-not-A one. C represents real signal: usually
	//       too many taplands or 1-2 color gaps. Worth a small -3
	//       (between the B credit and the D penalty).
	//   (3) D and F collapsed at the same -10 — squashed two
	//       distinct grades together. F represents a catastrophic
	//       mana base (4+ color gaps, 8+ taplands, no fixing); D is
	//       "rough but functional." Splitting to D=-8 / F=-15
	//       preserves the spread and makes the F penalty bite.
	//
	// New table (all rungs now log a factor for transparency):
	//   A → +10, B → +5, C → -3, D → -8, F → -15
	switch dp.ManaBaseGrade {
	case "A":
		score += 10
		factors = append(factors, "A-grade mana base → +10")
	case "B":
		score += 5
		factors = append(factors, "B-grade mana base → +5")
	case "C":
		score -= 3
		factors = append(factors, "C-grade mana base → -3")
	case "D":
		score -= 8
		factors = append(factors, "D-grade mana base → -8")
	case "F":
		score -= 15
		factors = append(factors, "F-grade mana base → -15")
	}

	// Interaction quality
	if dp.InteractionQuality > 0 && dp.InteractionQuality <= 2.0 {
		score += 8
		factors = append(factors, fmt.Sprintf("fast interaction (avg CMC %.1f) → +8", dp.InteractionQuality))
	} else if dp.InteractionQuality > 3.5 {
		score -= 5
		factors = append(factors, fmt.Sprintf("slow interaction (avg CMC %.1f) → -5", dp.InteractionQuality))
	}

	// Draw density
	if dp.DrawCount >= 12 {
		score += 8
		factors = append(factors, fmt.Sprintf("excellent draw (%d sources) → +8", dp.DrawCount))
	} else if dp.DrawCount < 5 {
		score -= 8
		factors = append(factors, fmt.Sprintf("low draw (%d sources) → -8", dp.DrawCount))
	}

	// Commander synergy
	if dp.CommanderSynergy >= 0.60 {
		score += 5
		factors = append(factors, fmt.Sprintf("strong commander synergy (%.0f%%) → +5", dp.CommanderSynergy*100))
	} else if dp.CommanderSynergy > 0 && dp.CommanderSynergy < 0.25 {
		score -= 5
		factors = append(factors, "low commander synergy → -5")
	}

	// Average CMC (lean curves are generally better)
	if dp.AvgCMC < 2.5 {
		score += 5
		factors = append(factors, fmt.Sprintf("lean curve (%.1f avg) → +5", dp.AvgCMC))
	} else if dp.AvgCMC > 3.8 {
		score -= 5
		factors = append(factors, fmt.Sprintf("heavy curve (%.1f avg) → -5", dp.AvgCMC))
	}

	// Keepable hand rate
	if dp.KeepableHandPct >= 85 {
		score += 5
		factors = append(factors, fmt.Sprintf("consistent opening hands (%.0f%% keepable) → +5", dp.KeepableHandPct))
	} else if dp.KeepableHandPct > 0 && dp.KeepableHandPct < 70 {
		score -= 5
		factors = append(factors, fmt.Sprintf("inconsistent opening hands (%.0f%% keepable) → -5", dp.KeepableHandPct))
	}

	// Clamp to 1-99
	if score < 1 {
		score = 1
	}
	if score > 99 {
		score = 99
	}

	return score, factors
}
