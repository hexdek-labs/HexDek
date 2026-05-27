package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type WinLine struct {
	Pieces     []string
	Type       string // "infinite", "determined", "finisher", "combat", "commander_damage", "alt_wincon"
	Class      string // ComboClass* — what the combo produces (sourced from ComboResult); empty for non-combo lines like commander_damage.
	Desc       string
	TutorPaths []TutorChain
	Rationale  *WinLineRationale
	// Weight is a tiered quality score (~1-17 typical range, higher is
	// better) computed by computeWinLineWeight. It captures three things a
	// flat per-deck WinLineCount blurs together: (a) what the line wins
	// with (true infinite > determined > combat), (b) how many pieces it
	// needs (2-card combos are dramatically easier to assemble than 4-card
	// grindy loops), and (c) tutor coverage of those pieces. Downstream
	// consumers sum these into WinLineAnalysis.TotalWeightedScore.
	Weight int

	// Confidence is a 0.0-1.0 backend-only score measuring how reliably the
	// deck can actually assemble and resolve this win line. Computed from
	// three signals: (a) tutor support — share of pieces a deck tutor can
	// fetch; (b) piece redundancy — alternative cards in the deck that
	// could substitute for each piece (e.g. Vito subs for Sanguine Bond in
	// the Bond + Exquisite Blood line); (c) protection density — deck-
	// level counterspell/hexproof/blink coverage that can defend the
	// assembly turn. Weighted sum (0.45/0.30/0.25) clamped to [0, 1].
	//
	// Used by the downstream search: lines with Confidence < 0.3 are
	// dropped from the bracket-scoring finisher count; lines with
	// Confidence >= 0.7 weight 1.5x in the archetype-detection combo
	// signal. BACKEND-ONLY — intentionally absent from Rationale, JSON
	// export, and text/markdown reports.
	Confidence float64
}

// WinLineRationale explains how this win line is detected and resolves.
type WinLineRationale struct {
	Forms      []string // cards that form the line (with optional role annotation)
	Conditions []string // setup/state required before the line goes live
	Resolves   []string // step-by-step description of how the line wins
}

type TutorChain struct {
	Tutor      string
	Finds      string
	Delivery   string // "hand", "top", "battlefield", "graveyard"
	Restricted string // "creature", "artifact", "enchantment", "instant_sorcery", "land", "any"
}

type WinLineAnalysis struct {
	WinLines             []WinLine
	TutorMap             []TutorInfo
	BackupPlans          []string
	SinglePoints         []string
	RedundancyMap        map[string]int
	// TotalWeightedScore is the sum of WinLine.Weight across every win
	// line in this analysis. Surfaces a single deck-level signal that
	// distinguishes "one 2-card Thoracle line" from "three 5-card grindy
	// assemblies" — both report WinLineCount=1 vs 3 respectively, but the
	// weighted score puts the Thoracle deck ahead.
	TotalWeightedScore   int
}

type TutorInfo struct {
	Name       string
	Restricted string
	Delivery   string
}

type tutorSpec struct {
	restricted string
	delivery   string
}

var knownTutors = map[string]tutorSpec{
	"demonic tutor":            {"any", "hand"},
	"vampiric tutor":           {"any", "top"},
	"imperial seal":            {"any", "top"},
	"grim tutor":               {"any", "hand"},
	"diabolic tutor":           {"any", "hand"},
	"diabolic intent":          {"any", "hand"},
	"beseech the mirror":       {"any", "hand"},
	"wishclaw talisman":        {"any", "hand"},
	"scheming symmetry":        {"any", "top"},
	"final parting":            {"any", "hand"},
	"razaketh, the foulblooded": {"any", "hand"},

	"enlightened tutor":        {"artifact_enchantment", "top"},
	"idyllic tutor":            {"enchantment", "hand"},
	"academy rector":           {"enchantment", "battlefield"},
	"zur the enchanter":        {"enchantment", "battlefield"},

	"worldly tutor":            {"creature", "top"},
	"sylvan tutor":             {"creature", "top"},
	"green sun's zenith":       {"creature", "battlefield"},
	"chord of calling":         {"creature", "battlefield"},
	"eldritch evolution":       {"creature", "battlefield"},
	"neoform":                  {"creature", "battlefield"},
	"birthing pod":             {"creature", "battlefield"},
	"survival of the fittest":  {"creature", "hand"},
	"fierce empath":            {"creature", "hand"},
	"fauna shaman":             {"creature", "hand"},
	"eladamri's call":          {"creature", "hand"},
	"finale of devastation":    {"creature", "battlefield"},
	"natural order":            {"creature", "battlefield"},
	"yisan, the wanderer bard": {"creature", "battlefield"},

	"mystical tutor":           {"instant_sorcery", "top"},
	"personal tutor":           {"sorcery", "top"},
	"merchant scroll":          {"instant", "hand"},
	"spellseeker":              {"instant_sorcery", "hand"},
	"solve the equation":       {"instant_sorcery", "hand"},

	"fabricate":                {"artifact", "hand"},
	"trinket mage":             {"artifact", "hand"},
	"trophy mage":              {"artifact", "hand"},
	"tribute mage":             {"artifact", "hand"},
	"treasure mage":            {"artifact", "hand"},
	"whir of invention":        {"artifact", "battlefield"},
	"tezzeret the seeker":      {"artifact", "battlefield"},
	"urza's saga":              {"artifact", "hand"},
	"inventors' fair":          {"artifact", "hand"},
	"transmute artifact":       {"artifact", "hand"},
	"muddle the mixture":       {"cmc2", "hand"},
	"drift of phantasms":       {"cmc3", "hand"},
	"perplex":                  {"cmc3", "hand"},
	"dimir infiltrator":        {"cmc2", "hand"},
	"shred memory":             {"cmc2", "hand"},
	"clutch of the undercity":  {"cmc4", "hand"},
	"brainspoil":               {"cmc5", "hand"},
	"netherborn altar":         {"cmc2", "hand"},
	"tolaria west":             {"cmc0", "hand"},

	"crop rotation":            {"land", "battlefield"},
	"expedition map":           {"land", "hand"},
	"sylvan scrying":           {"land", "hand"},
	"knight of the reliquary":  {"land", "battlefield"},
	"tempt with discovery":     {"land", "battlefield"},
	"hour of promise":          {"land", "battlefield"},
	"reap and sow":             {"land", "battlefield"},

	"entomb":                   {"any", "graveyard"},
	"buried alive":             {"creature", "graveyard"},
	"intuition":                {"any", "hand"},
	"gifts ungiven":            {"any", "hand"},
	"gamble":                   {"any", "hand"},

	"cultivate":                {"basic_land", "hand"},
	"kodama's reach":           {"basic_land", "hand"},
	"farseek":                  {"basic_land", "battlefield"},
	"nature's lore":            {"basic_land", "battlefield"},
	"three visits":             {"basic_land", "battlefield"},
	"rampant growth":           {"basic_land", "battlefield"},
	"sakura-tribe elder":       {"basic_land", "battlefield"},
	"wood elves":               {"basic_land", "battlefield"},
	"farhaven elf":             {"basic_land", "battlefield"},
	"solemn simulacrum":        {"basic_land", "battlefield"},
	"burnished hart":           {"basic_land", "battlefield"},
	"explosive vegetation":     {"basic_land", "battlefield"},
	"skyshroud claim":          {"basic_land", "battlefield"},
	"migration path":           {"basic_land", "battlefield"},
	"circuitous route":         {"basic_land", "battlefield"},
	"primeval titan":           {"land", "battlefield"},
	"oracle of mul daya":       {"land", "battlefield"},
	"springbloom druid":        {"basic_land", "battlefield"},

	// Wish-style / outside-the-game tutors. Modeled as Restricted="wish"
	// because Freya doesn't track sideboards; treated as "any" for win-
	// line coverage purposes (see tutorCanFind).
	"burning wish":              {"wish", "hand"},
	"cunning wish":              {"wish", "hand"},
	"living wish":               {"wish", "hand"},
	"glittering wish":           {"wish", "hand"},
	"mastermind's acquisition":  {"wish", "hand"},
	"karn, the great creator":   {"wish", "battlefield"},
	"spawnsire of ulamog":       {"wish", "battlefield"},
	"research // development":   {"wish", "hand"},

	// Tribal subtype tutors — restricted="tribe:<subtype>" routes through
	// the type-line check in tutorCanFind.
	"goblin matron":            {"tribe:goblin", "hand"},
	"goblin recruiter":         {"tribe:goblin", "top"},
	"imperial recruiter":       {"cmcle2", "hand"}, // "creature ... power 2 or less"
	"recruiter of the guard":   {"cmcle1", "hand"}, // "toughness 1 or less"
	"merfolk searcher":         {"tribe:merfolk", "hand"},
	"diregraf colossus":        {"tribe:zombie", "battlefield"},
	"sliver overlord":          {"tribe:sliver", "hand"},
	"sliver legion":            {"tribe:sliver", "hand"},
	"patriarch's bidding":      {"tribe:any", "battlefield"}, // mass reanimate by tribe

	// Conditional CMC tutors.
	"bring to light":            {"cmcle5", "battlefield"},
	"diabolic revelation":       {"any", "hand"}, // unbounded X, treat as any
	"increasing ambition":       {"any", "hand"},
	"demonic collusion":         {"any", "hand"},
	"insidious dreams":          {"any", "top"},

	"flooded strand":           {"land", "battlefield"},
	"polluted delta":           {"land", "battlefield"},
	"windswept heath":          {"land", "battlefield"},
	"wooded foothills":         {"land", "battlefield"},
	"bloodstained mire":        {"land", "battlefield"},
	"scalding tarn":            {"land", "battlefield"},
	"verdant catacombs":        {"land", "battlefield"},
	"misty rainforest":         {"land", "battlefield"},
	"arid mesa":                {"land", "battlefield"},
	"marsh flats":              {"land", "battlefield"},
	"prismatic vista":          {"land", "battlefield"},
	"fabled passage":           {"land", "battlefield"},
	"terramorphic expanse":     {"land", "battlefield"},
	"evolving wilds":           {"land", "battlefield"},
}

func ComputeWinLines(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *WinLineAnalysis {
	wla := &WinLineAnalysis{
		RedundancyMap: map[string]int{},
	}

	profileByName := map[string]CardProfile{}
	typeByName := map[string]string{}
	for _, qp := range qtyProfiles {
		profileByName[qp.Profile.Name] = qp.Profile
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				typeByName[qp.Profile.Name] = strings.ToLower(entry.TypeLine)
			}
		}
	}

	allTutors := buildTutorMap(qtyProfiles, oracle)
	wla.TutorMap = allTutors

	var winTutors []TutorInfo
	for _, t := range allTutors {
		// Land/basic-land tutors don't find combo pieces; they belong to
		// the ramp package. A nonbasic-land tutor (Crop Rotation, Expedition
		// Map) might fetch a combo-relevant utility land — those still
		// surface here when they're not classified "land"/"basic_land"
		// (e.g. Expedition Map is registered as "land" — that's fine: if
		// a deck's win line names a land card, tutorCanFind will pick it
		// up from a separate explicit entry).
		if t.Restricted != "land" && t.Restricted != "basic_land" {
			winTutors = append(winTutors, t)
		}
	}

	addComboWinLines(wla, report.TrueInfinites, "infinite", winTutors, typeByName, oracle)
	addComboWinLines(wla, report.Determined, "determined", winTutors, typeByName, oracle)
	addComboWinLines(wla, report.Finishers, "finisher", winTutors, typeByName, oracle)
	addNonComboWinLines(wla, report, qtyProfiles, oracle, profileByName)

	sort.Slice(wla.WinLines, func(i, j int) bool {
		return winLinePriority(wla.WinLines[i].Type) < winLinePriority(wla.WinLines[j].Type)
	})

	// Tiered weight pass — must run after TutorPaths are populated so the
	// tutor-coverage bonus reflects the post-Compute state.
	for i := range wla.WinLines {
		wla.WinLines[i].Weight = computeWinLineWeight(&wla.WinLines[i])
		wla.TotalWeightedScore += wla.WinLines[i].Weight
	}

	computeBackupPlans(wla)
	computeRedundancy(wla, report, profileByName)
	computeSinglePoints(wla, profileByName)

	// Confidence pass — backend-only quality signal feeding archetype
	// detection (>=0.7 weights 1.5x) and bracket scoring (<0.3 excluded
	// from finisher count). Must run AFTER TutorPaths population.
	protection := computeWinLineProtectionDensity(report, profileByName)
	for i := range wla.WinLines {
		wla.WinLines[i].Confidence = computeWinLineConfidence(&wla.WinLines[i], profileByName, protection)
	}

	return wla
}

// computeWinLineConfidence scores a single WinLine on the [0, 1] interval
// reflecting how reliably the deck can assemble and resolve it. Weighted
// sum: 0.45*tutor + 0.30*redundancy + 0.25*protection. See WinLine.Confidence
// for the contract.
func computeWinLineConfidence(wl *WinLine, profileByName map[string]CardProfile, protection float64) float64 {
	if wl == nil {
		return 0
	}
	pieceCount := len(wl.Pieces)

	// Non-combo lines don't model tutor / piece-redundancy in the same
	// way — pieces are descriptive ("12 threats + 2 pumps") or a single
	// commander, not card slots a tutor fetches. Use protection alone
	// plus a neutral 0.5 on the other two axes so these lines land near
	// the middle of the distribution and the <0.3 / >=0.7 thresholds
	// don't either uniformly demote or promote them.
	if wl.Type == "combat" || wl.Type == "commander_damage" {
		return clamp01(0.45*0.5 + 0.30*0.5 + 0.25*protection)
	}

	if pieceCount == 0 {
		return 0
	}

	// (a) Tutor support — share of pieces with at least one tutor path.
	covered := map[string]bool{}
	for _, tp := range wl.TutorPaths {
		covered[tp.Finds] = true
	}
	tutorScore := float64(len(covered)) / float64(pieceCount)
	if tutorScore > 1 {
		tutorScore = 1
	}

	// (b) Piece redundancy — for each piece, how many cards in the deck
	// share at least one characteristic combo-flag (alternative pieces
	// that could substitute, like Vito for Sanguine Bond). Saturate at
	// 2 alternatives per piece.
	totalRedundancy := 0.0
	for _, piece := range wl.Pieces {
		alts := countPieceAlternatives(piece, profileByName)
		score := float64(alts) / 2.0
		if score > 1 {
			score = 1
		}
		totalRedundancy += score
	}
	redundancyScore := totalRedundancy / float64(pieceCount)

	return clamp01(0.45*tutorScore + 0.30*redundancyScore + 0.25*protection)
}

// computeWinLineProtectionDensity is a deck-level [0, 1] score: how much defensive
// coverage (counterspells, hexproof/indestructible grants, blink/flicker as
// a "save the combo piece" tool) the deck packs. Density saturates at 15%
// of total cards — at that level the deck is meaningfully protecting its
// assembly turn.
func computeWinLineProtectionDensity(report *FreyaReport, profileByName map[string]CardProfile) float64 {
	if report == nil || report.Roles == nil {
		return 0
	}
	total := report.Roles.TotalCards
	if total == 0 {
		return 0
	}
	count := report.Roles.RoleCounts[RoleCounterspell] + report.Roles.RoleCounts[RoleProtection]
	for _, p := range profileByName {
		if p.IsBlinker {
			count++
		}
	}
	density := float64(count) / float64(total)
	score := density / 0.15
	if score > 1 {
		score = 1
	}
	return score
}

// countPieceAlternatives returns the count of cards in the deck (excluding
// the piece itself) that share at least one characteristic combo flag with
// the named piece. A non-zero result means the deck has functional
// substitutes — losing the named piece doesn't break the line.
func countPieceAlternatives(pieceName string, profileByName map[string]CardProfile) int {
	pp, ok := profileByName[pieceName]
	if !ok {
		return 0
	}
	alts := 0
	for n, p := range profileByName {
		if n == pieceName {
			continue
		}
		if profileFunctionalMatch(pp, p) {
			alts++
		}
	}
	return alts
}

// profileFunctionalMatch reports whether two CardProfiles fill the same
// characteristic combo role — i.e. one could substitute for the other in
// a win line. Matched on a curated set of combo-piece flags (outlet,
// wincon, Sanguine/Exquisite drain loops, ETB-damage, mana-payoff, etc.)
// rather than every CardProfile bool, to avoid false matches on incidental
// shared properties (e.g. both being creatures).
func profileFunctionalMatch(a, b CardProfile) bool {
	switch {
	case a.IsOutlet && b.IsOutlet:
		return true
	case a.IsWinCon && b.IsWinCon:
		return true
	case a.LifegainToDrain && b.LifegainToDrain:
		return true
	case a.LifelossToPump && b.LifelossToPump:
		return true
	case a.CounterToDamage && b.CounterToDamage:
		return true
	case a.DamageToCounter && b.DamageToCounter:
		return true
	case a.HasDeathDrain && b.HasDeathDrain:
		return true
	case a.HasETBDamage && b.HasETBDamage:
		return true
	case a.WinsWithEmptyLib && b.WinsWithEmptyLib:
		return true
	case a.EmptiesLibrary && b.EmptiesLibrary:
		return true
	case a.UntapsAll && b.UntapsAll:
		return true
	case a.IsManaPayoff && b.IsManaPayoff:
		return true
	case a.MakesInfiniteTokens && b.MakesInfiniteTokens:
		return true
	case a.IsBlinker && b.IsBlinker:
		return true
	case a.IsExtraCombat && b.IsExtraCombat:
		return true
	case a.IsStormFinisher && b.IsStormFinisher:
		return true
	case a.IsXBurn && b.IsXBurn:
		return true
	}
	return false
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// computeWinLineWeight assigns a tiered numeric quality score to a single
// WinLine. The factors a flat WinLineCount blurs together:
//
//   - WHAT the line wins with: a true infinite kills the table the turn it
//     assembles, while a combat-damage line needs another rotation of
//     interaction to fail before it resolves.
//   - HOW MANY pieces: a 2-card combo is dramatically easier to find, keep
//     alive, and protect than a 4-card grindy loop.
//   - TUTOR COVERAGE: a tutored line is reliably reachable; an untutored
//     one waits on raw draw.
//
// Formula:
//
//	weight = round( (typeBase + classBonus) * pieceMultiplier ) + tutorBonus
//
// Ranges:
//
//	typeBase: 10 infinite | 7 determined | 5 finisher | 3 alt_wincon |
//	          2 combat | 1 commander_damage
//	classBonus: +5 library_exile_win / infinite_drain / infinite_damage
//	            +4 infinite_mana / infinite_mill / lockdown
//	            +3 storm_finisher / mana_sink
//	            +2 infinite_etb / infinite_tokens / etb_payoff / etb_doubler
//	            +1 blink_engine / combat_finisher / graveyard_loop
//	            +0 unknown / land_cycle_synergy / empty
//	pieceMultiplier: 1.00 (≤2 pieces) | 0.85 (3) | 0.70 (4) | 0.55 (≥5)
//	tutorBonus: 0..2 — proportion of pieces with at least one tutor path × 2.
//
// A 2-card Thoracle line: (10 + 5) * 1.00 + 2 = 17. A 4-card grindy infinite
// ETB pile with no tutors: round((10 + 2) * 0.70) + 0 = 8. A combat line: 2.
func computeWinLineWeight(wl *WinLine) int {
	if wl == nil {
		return 0
	}

	typeBase := 0
	switch wl.Type {
	case "infinite":
		typeBase = 10
	case "determined":
		typeBase = 7
	case "finisher":
		typeBase = 5
	case "alt_wincon":
		typeBase = 3
	case "combat":
		typeBase = 2
	case "commander_damage":
		typeBase = 1
	}

	classBonus := 0
	switch wl.Class {
	case ComboClassLibraryExileWin, ComboClassInfiniteDrain, ComboClassInfiniteDamage:
		classBonus = 5
	case ComboClassInfiniteMana, ComboClassInfiniteMill, ComboClassLockdown:
		classBonus = 4
	case ComboClassStormFinisher, ComboClassManaSink:
		classBonus = 3
	case ComboClassInfiniteETB, ComboClassInfiniteTokens, ComboClassETBPayoff, ComboClassETBDoubler:
		classBonus = 2
	case ComboClassBlinkEngine, ComboClassCombatFinisher, ComboClassGraveyardLoop:
		classBonus = 1
	}

	pieceCount := len(wl.Pieces)
	var pieceMult float64
	switch {
	case pieceCount <= 2:
		pieceMult = 1.00
	case pieceCount == 3:
		pieceMult = 0.85
	case pieceCount == 4:
		pieceMult = 0.70
	default:
		pieceMult = 0.55
	}

	weighted := float64(typeBase+classBonus) * pieceMult
	// round half-up
	rounded := int(weighted + 0.5)

	tutorBonus := 0
	if pieceCount > 0 && len(wl.TutorPaths) > 0 {
		coveredPieces := map[string]bool{}
		for _, tp := range wl.TutorPaths {
			coveredPieces[tp.Finds] = true
		}
		// Coverage is the share of declared pieces with at least one tutor
		// path; multiply by 2 and round so the bonus tops out at +2 when
		// every piece is tutorable. Non-combo lines (combat,
		// commander_damage) carry no TutorPaths, so this branch is skipped
		// for them — their pieces aren't card names a tutor could find.
		coverage := float64(len(coveredPieces)) / float64(pieceCount)
		if coverage > 1.0 {
			coverage = 1.0
		}
		tutorBonus = int(coverage*2.0 + 0.5)
	}

	return rounded + tutorBonus
}

func winLinePriority(t string) int {
	switch t {
	case "infinite":
		return 0
	case "determined":
		return 1
	case "finisher":
		return 2
	case "alt_wincon":
		return 3
	case "combat":
		return 4
	case "commander_damage":
		return 5
	default:
		return 6
	}
}

func buildTutorMap(qtyProfiles []CardProfileQty, oracle *oracleDB) []TutorInfo {
	var tutors []TutorInfo
	seen := map[string]bool{}
	for _, qp := range qtyProfiles {
		if !qp.Profile.IsTutor || qp.Profile.IsLand {
			continue
		}
		name := qp.Profile.Name
		if seen[name] {
			continue
		}
		seen[name] = true

		nameLower := strings.ToLower(name)
		if spec, ok := knownTutors[nameLower]; ok {
			tutors = append(tutors, TutorInfo{
				Name:       name,
				Restricted: spec.restricted,
				Delivery:   spec.delivery,
			})
			continue
		}

		var ot, tl string
		if oracle != nil {
			entry := oracle.lookup(name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
				tl = strings.ToLower(entry.TypeLine)
				if ot == "" && len(entry.CardFaces) > 0 {
					ot = entry.CardFaces[0].OracleText
					tl = entry.CardFaces[0].TypeLine
					ot = strings.ToLower(ot)
					tl = strings.ToLower(tl)
				}
			}
		}

		restricted, delivery := inferTutorRestriction(ot, tl)
		tutors = append(tutors, TutorInfo{
			Name:       name,
			Restricted: restricted,
			Delivery:   delivery,
		})
	}
	return tutors
}

// inferTutorRestriction reads oracle text and infers what kind of card the
// tutor can fetch, plus where it ends up. The output Restricted strings are
// consumed by tutorCanFind below.
//
// Recognized restriction keys:
//
//	any                        — unrestricted (Demonic Tutor)
//	creature                   — creature-only (Worldly Tutor, Birthing Pod)
//	artifact                   — artifact-only (Fabricate, Trinket Mage)
//	enchantment                — enchantment-only (Idyllic Tutor)
//	artifact_enchantment       — either (Enlightened Tutor)
//	instant                    — instant-only (Merchant Scroll)
//	sorcery                    — sorcery-only (Personal Tutor)
//	instant_sorcery            — either (Mystical Tutor, Spellseeker)
//	planeswalker               — planeswalker-only
//	land                       — any land
//	basic_land                 — basic lands only (Rampant Growth, Cultivate)
//	cmcN                       — exact mana value N (transmute cards)
//	cmcleN                     — mana value N or less (Bring to Light: cmcle5)
//	cmcgeN                     — mana value N or greater
//	multicolored               — multicolored card (Bring to Light)
//	monocolored                — monocolored card
//	tribe:<subtype>            — creature subtype tutor (Goblin Matron → tribe:goblin)
//	wish                       — fetches a card from outside the game / sideboard
//
// Recognized delivery values: hand (default), top, battlefield, graveyard,
// exile.
func inferTutorRestriction(ot, tl string) (string, string) {
	restricted := "any"

	// Wish-style: outside the game / sideboard. Detected first so the
	// downstream "search your library" branches don't override.
	if containsAny(ot, "from outside the game", "from your sideboard") &&
		containsAny(ot, "choose a ", "choose an ",
			"reveal a ", "reveal an ", "reveal any number",
			"may put", "may cast", "you own from outside",
			"you own in exile") {
		return "wish", inferTutorDelivery(ot)
	}

	// Transmute / pure CMC-equal tutors.
	if strings.Contains(ot, "transmute") {
		for cmc := 0; cmc <= 9; cmc++ {
			if strings.Contains(ot, fmt.Sprintf("mana value %d", cmc)) ||
				strings.Contains(ot, fmt.Sprintf("converted mana cost %d", cmc)) {
				return fmt.Sprintf("cmc%d", cmc), "hand"
			}
		}
	}

	// Land tutors. Basic-land restricted gets a separate key so a
	// non-basic land tutor (Crop Rotation, Expedition Map) can still feed
	// utility lands into combo lines.
	if isBasicLandSearch(ot) {
		restricted = "basic_land"
	} else if isLandSearch(ot) {
		restricted = "land"
	} else if strings.Contains(ot, "search your library for a creature") ||
		strings.Contains(ot, "search your library for up to one creature") ||
		strings.Contains(ot, "search your library for up to two creature") ||
		strings.Contains(ot, "search your library for up to three creature") {
		restricted = "creature"
	} else if strings.Contains(ot, "search your library for an artifact") ||
		strings.Contains(ot, "search your library for up to one artifact") ||
		strings.Contains(ot, "search your library for up to two artifact") {
		restricted = "artifact"
	} else if strings.Contains(ot, "search your library for an enchantment") {
		restricted = "enchantment"
	} else if strings.Contains(ot, "search your library for an instant") &&
		strings.Contains(ot, "or sorcery") {
		restricted = "instant_sorcery"
	} else if strings.Contains(ot, "search your library for an instant") {
		restricted = "instant"
	} else if strings.Contains(ot, "search your library for a sorcery") {
		restricted = "sorcery"
	} else if strings.Contains(ot, "search your library for a planeswalker") {
		restricted = "planeswalker"
	} else if tribe := extractTribalSearch(ot); tribe != "" {
		restricted = "tribe:" + tribe
	}

	// CMC-bound restrictions stack on the type bucket. "Bring to Light"
	// reads "search your library for a multicolored card with converted
	// mana cost 5 or less" — that's both multicolored and cmcle5. When
	// both apply we prefer the CMC restriction (it's the tighter, more
	// useful filter for combo win-line coverage).
	//
	// Power/toughness bounds (Imperial Recruiter, Recruiter of the Guard,
	// Ranger of Eos) override the plain "creature" bucket because they're
	// strictly tighter — power_le_2 finds Dockside Extortionist and Dark
	// Confidant but not Craterhoof, so a deck that runs Imperial Recruiter
	// can reach a 2-power combo piece but NOT a 6-power finisher.
	//
	// Variable-X CMC bounds (Chord of Calling, Birthing Pod, Eldritch
	// Evolution) collapse to a single "cmc_variable" key — the bound
	// exists but depends on game state we can't model statically.
	if cmcle := extractCmcLeBound(ot); cmcle >= 0 {
		restricted = fmt.Sprintf("cmcle%d", cmcle)
	} else if cmcge := extractCmcGeBound(ot); cmcge >= 0 {
		restricted = fmt.Sprintf("cmcge%d", cmcge)
	} else if pwle := extractPowerLeBound(ot); pwle >= 0 {
		restricted = fmt.Sprintf("power_le_%d", pwle)
	} else if tghle := extractToughnessLeBound(ot); tghle >= 0 {
		restricted = fmt.Sprintf("toughness_le_%d", tghle)
	} else if detectVariableCmcSearch(ot) && restricted == "creature" {
		// Only override "creature" — preserving narrower buckets (tribe,
		// land) that may also have variable-CMC clauses (Genesis Wave
		// variants).
		restricted = "cmc_variable"
	} else if restricted == "any" {
		// Only fall back to color restriction when no other bucket fits.
		if strings.Contains(ot, "for a multicolored card") ||
			strings.Contains(ot, "for a multicolored creature") {
			restricted = "multicolored"
		} else if strings.Contains(ot, "for a monocolored card") {
			restricted = "monocolored"
		}
	}

	delivery := inferTutorDelivery(ot)
	return restricted, delivery
}

func inferTutorDelivery(ot string) string {
	// "on top" in oracle text overwhelmingly refers to library top in a
	// tutor context. Vampiric Tutor says "shuffle and put that card on top
	// of it" — "of it" refers back to the library. Cover the common
	// phrasings explicitly so a stray "on top of each creature" elsewhere
	// in the oracle text doesn't false-trigger.
	if containsAny(ot,
		"on top of your library",
		"on top of their library",
		"put it on top",
		"put that card on top",
		"puts it on top",
		"put them on top") {
		return "top"
	}
	if strings.Contains(ot, "onto the battlefield") ||
		strings.Contains(ot, "put it onto the battlefield") ||
		strings.Contains(ot, "put that card onto the battlefield") {
		return "battlefield"
	}
	if strings.Contains(ot, "into your graveyard") ||
		strings.Contains(ot, "put it into your graveyard") {
		return "graveyard"
	}
	if strings.Contains(ot, "exile it") && strings.Contains(ot, "you may cast") {
		return "exile"
	}
	return "hand"
}

// isLandSearch covers any land tutor — basic, nonbasic, or unrestricted.
func isLandSearch(ot string) bool {
	return containsAny(ot,
		"search your library for a land",
		"search your library for up to two land",
		"search your library for up to three land",
		"search your library for up to four land",
		"search your library for up to one land") ||
		(strings.Contains(ot, "search your library") && strings.Contains(ot, "land card") &&
			!strings.Contains(ot, "nonland"))
}

// isBasicLandSearch is the tighter form: only basic land types.
func isBasicLandSearch(ot string) bool {
	return containsAny(ot,
		"search your library for a basic land",
		"search your library for up to two basic land",
		"search your library for up to three basic land",
		"search your library for up to four basic land",
		"search your library for a plains",
		"search your library for an island",
		"search your library for a swamp",
		"search your library for a mountain",
		"search your library for a forest",
		"search your library for a card with a basic land type",
		"search your library for a basic plains",
		"search your library for a basic island",
		"search your library for a basic swamp",
		"search your library for a basic mountain",
		"search your library for a basic forest")
}

// extractTribalSearch detects "search your library for a <Subtype> card"
// patterns and returns the lowercase subtype name, or "" if none. Covers
// the common Commander tribes; the exact list is intentional rather than
// exhaustive — adding a new tribe later is a one-liner.
func extractTribalSearch(ot string) string {
	tribes := []string{
		"goblin", "elf", "merfolk", "zombie", "vampire", "dragon",
		"sliver", "wizard", "soldier", "knight", "human", "angel",
		"demon", "spirit", "dwarf", "elemental", "beast", "cat",
		"warrior", "rogue", "cleric", "shaman", "druid", "rat",
		"sphinx", "hydra", "treefolk", "elder", "samurai", "ninja",
		"giant", "minotaur", "centaur", "satyr", "horror", "eldrazi",
		"phyrexian", "construct",
	}
	for _, t := range tribes {
		if strings.Contains(ot, "search your library for a "+t+" card") ||
			strings.Contains(ot, "search your library for an "+t+" card") ||
			strings.Contains(ot, "search your library for a "+t+" creature") {
			return t
		}
	}
	return ""
}

// extractCmcLeBound parses "with mana value N or less" / "with converted
// mana cost N or less" phrases when they appear inside a search clause.
// Returns -1 when no such bound is present.
func extractCmcLeBound(ot string) int {
	if !strings.Contains(ot, "search your library") {
		return -1
	}
	for n := 0; n <= 12; n++ {
		needles := []string{
			fmt.Sprintf("mana value %d or less", n),
			fmt.Sprintf("converted mana cost %d or less", n),
			fmt.Sprintf("mana value of %d or less", n),
			fmt.Sprintf("converted mana cost of %d or less", n),
		}
		if containsAny(ot, needles...) {
			return n
		}
	}
	return -1
}

func extractCmcGeBound(ot string) int {
	if !strings.Contains(ot, "search your library") {
		return -1
	}
	for n := 12; n >= 0; n-- {
		needles := []string{
			fmt.Sprintf("mana value %d or greater", n),
			fmt.Sprintf("converted mana cost %d or greater", n),
		}
		if containsAny(ot, needles...) {
			return n
		}
	}
	return -1
}

// extractPowerLeBound parses "with power N or less" inside a search clause.
// Imperial Recruiter ("creature card with power 2 or less"), Ranger of Eos
// ("creature card with mana value 1 or less" — different axis; this helper
// targets POWER specifically), Recruiter of the Wheel-style cards.
// Returns -1 when no power bound is present.
func extractPowerLeBound(ot string) int {
	if !strings.Contains(ot, "search your library") {
		return -1
	}
	for n := 0; n <= 12; n++ {
		needles := []string{
			fmt.Sprintf("power %d or less", n),
			fmt.Sprintf("power of %d or less", n),
		}
		if containsAny(ot, needles...) {
			return n
		}
	}
	return -1
}

// extractToughnessLeBound parses "with toughness N or less" inside a search
// clause. Recruiter of the Guard ("creature card with toughness 2 or less"),
// Skullclamp-adjacent tutors.
func extractToughnessLeBound(ot string) int {
	if !strings.Contains(ot, "search your library") {
		return -1
	}
	for n := 0; n <= 12; n++ {
		needles := []string{
			fmt.Sprintf("toughness %d or less", n),
			fmt.Sprintf("toughness of %d or less", n),
		}
		if containsAny(ot, needles...) {
			return n
		}
	}
	return -1
}

// detectVariableCmcSearch returns true when a "search your library" clause
// has a CMC bound that depends on game state rather than a literal digit:
//
//   - Chord of Calling: "with mana value X or less" (X from convoke)
//   - Birthing Pod: "with mana value equal to [sacrificed creature]'s mana
//     value plus 1"
//   - Eldritch Evolution: "with converted mana cost equal to or less than 2
//     plus the sacrificed creature's converted mana cost"
//   - Natural Order family: variants on "plus 1" / "plus 2" / "plus N"
//
// We collapse all of these into a single "cmc_variable" restriction key —
// downstream `tutorCanFind` treats it as a soft restriction (returns true
// for any creature/card on the assumption that the variable will exceed
// the target's CMC often enough to count as coverage). This is more honest
// than the pre-fix behavior, which dropped the bound entirely and treated
// these as plain "creature" or "any" tutors.
func detectVariableCmcSearch(ot string) bool {
	if !strings.Contains(ot, "search your library") {
		return false
	}
	return containsAny(ot,
		"mana value x or less",
		"converted mana cost x or less",
		"with mana value equal to",
		"with converted mana cost equal to",
		"equal to or less than",
		"mana value of the sacrificed",
		"converted mana cost of the sacrificed")
}


func tutorCanFind(tutor TutorInfo, cardName string, typeByName map[string]string, oracle *oracleDB) bool {
	if tutor.Restricted == "any" {
		return true
	}
	// Wish tutors can fetch anything the pilot has access to in their
	// sideboard / outside-game pool. Freya doesn't model sideboards, so we
	// treat wishes as "any" for win-line coverage purposes — they're a
	// real combo enabler in cEDH (Burning Wish → Tendrils, etc.).
	if tutor.Restricted == "wish" {
		return true
	}

	tl := typeByName[cardName]
	if tl == "" && oracle != nil {
		entry := oracle.lookup(cardName)
		if entry != nil {
			tl = strings.ToLower(entry.TypeLine)
		}
	}

	switch tutor.Restricted {
	case "creature":
		return strings.Contains(tl, "creature")
	case "artifact":
		return strings.Contains(tl, "artifact")
	case "enchantment":
		return strings.Contains(tl, "enchantment")
	case "artifact_enchantment":
		return strings.Contains(tl, "artifact") || strings.Contains(tl, "enchantment")
	case "instant_sorcery":
		return strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery")
	case "instant":
		return strings.Contains(tl, "instant")
	case "sorcery":
		return strings.Contains(tl, "sorcery")
	case "planeswalker":
		return strings.Contains(tl, "planeswalker")
	case "land":
		return strings.Contains(tl, "land")
	case "basic_land":
		return strings.Contains(tl, "basic") && strings.Contains(tl, "land")
	}

	// CMC-bound buckets.
	if strings.HasPrefix(tutor.Restricted, "cmcle") {
		target, _ := strconv.Atoi(tutor.Restricted[5:])
		if oracle != nil {
			if entry := oracle.lookup(cardName); entry != nil {
				return int(entry.CMC) <= target
			}
		}
		return false
	}
	if strings.HasPrefix(tutor.Restricted, "cmcge") {
		target, _ := strconv.Atoi(tutor.Restricted[5:])
		if oracle != nil {
			if entry := oracle.lookup(cardName); entry != nil {
				return int(entry.CMC) >= target
			}
		}
		return false
	}
	if strings.HasPrefix(tutor.Restricted, "cmc") && !strings.HasPrefix(tutor.Restricted, "cmc_variable") {
		target, _ := strconv.Atoi(tutor.Restricted[3:])
		if oracle != nil {
			if entry := oracle.lookup(cardName); entry != nil {
				return int(entry.CMC) == target
			}
		}
		return false
	}

	// Power/toughness-restricted creature tutors. The candidate MUST be a
	// creature AND have power/toughness within the bound. Falls back to
	// "no oracle data → assume in-range" when the oracle DB is absent
	// (mirrors how tribal tutors handle the no-DB case).
	if strings.HasPrefix(tutor.Restricted, "power_le_") {
		if !strings.Contains(tl, "creature") {
			return false
		}
		target, _ := strconv.Atoi(tutor.Restricted[len("power_le_"):])
		if oracle != nil {
			if entry := oracle.lookup(cardName); entry != nil {
				pw, err := strconv.Atoi(entry.Power)
				if err == nil {
					return pw <= target
				}
				// Non-integer power ("*", "1+*") — assume coverable.
			}
		}
		return true
	}
	if strings.HasPrefix(tutor.Restricted, "toughness_le_") {
		if !strings.Contains(tl, "creature") {
			return false
		}
		target, _ := strconv.Atoi(tutor.Restricted[len("toughness_le_"):])
		if oracle != nil {
			if entry := oracle.lookup(cardName); entry != nil {
				tg, err := strconv.Atoi(entry.Toughness)
				if err == nil {
					return tg <= target
				}
			}
		}
		return true
	}

	// Variable-CMC tutors (Chord of Calling, Birthing Pod, Eldritch
	// Evolution) — the bound exists but depends on game state we can't
	// model statically. Treat as a soft restriction: any creature passes,
	// non-creatures don't (these tutors are all creature-typed in practice).
	if tutor.Restricted == "cmc_variable" {
		return strings.Contains(tl, "creature")
	}

	// Tribal subtype tutors (Goblin Matron → tribe:goblin).
	if strings.HasPrefix(tutor.Restricted, "tribe:") {
		tribe := tutor.Restricted[len("tribe:"):]
		return strings.Contains(tl, tribe)
	}

	// Color-bound restrictions. Without parsing mana cost colors we can
	// only do this when oracle data is present.
	if tutor.Restricted == "multicolored" || tutor.Restricted == "monocolored" {
		if oracle == nil {
			return false
		}
		entry := oracle.lookup(cardName)
		if entry == nil {
			return false
		}
		nColors := len(entry.ColorIdentity)
		if tutor.Restricted == "multicolored" {
			return nColors >= 2
		}
		return nColors == 1
	}

	return false
}

func addComboWinLines(wla *WinLineAnalysis, combos []ComboResult, lineType string, tutors []TutorInfo, typeByName map[string]string, oracle *oracleDB) {
	for _, combo := range combos {
		wl := WinLine{
			Pieces: combo.Cards,
			Type:   lineType,
			Class:  combo.Class,
			Desc:   combo.Description,
		}

		for _, piece := range combo.Cards {
			for _, tutor := range tutors {
				if tutorCanFind(tutor, piece, typeByName, oracle) {
					wl.TutorPaths = append(wl.TutorPaths, TutorChain{
						Tutor:      tutor.Name,
						Finds:      piece,
						Delivery:   tutor.Delivery,
						Restricted: tutor.Restricted,
					})
				}
			}
		}

		wl.Rationale = buildComboWinLineRationale(&wl)
		wla.WinLines = append(wla.WinLines, wl)
	}
}

// buildComboWinLineRationale extracts the combo's how-it-works detail from the
// description string emitted by the combo detector and surfaces tutor coverage.
func buildComboWinLineRationale(wl *WinLine) *WinLineRationale {
	r := &WinLineRationale{
		Forms: append([]string(nil), wl.Pieces...),
	}

	desc := strings.TrimSpace(wl.Desc)
	// The combo detector concatenates structured tags after a ' | ' separator
	// (OUTLETS, MANA SINK, NO OUTLET, etc.). Split them so the resolution and
	// the conditions render on distinct lines.
	if desc != "" {
		parts := strings.Split(desc, " | ")
		// First sentence is the canonical mechanism; subsequent ' | ' fragments
		// are conditions or warnings.
		mech := strings.TrimSpace(parts[0])
		if mech != "" {
			r.Resolves = append(r.Resolves, mech)
		}
		for _, part := range parts[1:] {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			r.Conditions = append(r.Conditions, part)
		}
	}

	switch wl.Type {
	case "infinite":
		r.Conditions = append(r.Conditions,
			fmt.Sprintf("All %d pieces must be present (battlefield/graveyard/hand depending on combo)", len(wl.Pieces)))
	case "determined":
		r.Conditions = append(r.Conditions, "Reach a board state where the combo is forced/inevitable")
	case "finisher":
		r.Conditions = append(r.Conditions, "Survive until enough mana/setup is available to cast the finisher")
	}

	if len(wl.TutorPaths) > 0 {
		// Group tutor coverage by piece for a compact summary.
		coverage := map[string][]string{}
		for _, tp := range wl.TutorPaths {
			coverage[tp.Finds] = append(coverage[tp.Finds], tp.Tutor)
		}
		var lines []string
		for _, piece := range wl.Pieces {
			if tutors, ok := coverage[piece]; ok {
				deduped := uniqueStrings(tutors)
				lines = append(lines, fmt.Sprintf("%s: %d tutor%s (%s)",
					piece, len(deduped), pluralS(len(deduped)),
					strings.Join(deduped, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("%s: must draw naturally — no tutor in deck can fetch", piece))
			}
		}
		r.Conditions = append(r.Conditions, "Tutor access:")
		r.Conditions = append(r.Conditions, lines...)
	} else if wl.Type == "infinite" || wl.Type == "determined" {
		r.Conditions = append(r.Conditions, "No tutor support — every piece must be drawn naturally")
	}

	return r
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func addNonComboWinLines(wla *WinLineAnalysis, report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB, profileByName map[string]CardProfile) {
	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
		}
		p := qp.Profile

		var ot string
		if oracle != nil {
			entry := oracle.lookup(p.Name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
				if ot == "" && len(entry.CardFaces) > 0 {
					ot = strings.ToLower(entry.CardFaces[0].OracleText)
				}
			}
		}

		if p.IsWinCon && !alreadyInWinLines(wla, p.Name) {
			wla.WinLines = append(wla.WinLines, WinLine{
				Pieces: []string{p.Name},
				Type:   "alt_wincon",
				Desc:   "alternate win condition",
				Rationale: &WinLineRationale{
					Forms:      []string{p.Name},
					Conditions: []string{"Card is in play / castable", "No interaction stops the win-state effect"},
					Resolves:   []string{altWinconResolves(p.Name, ot)},
				},
			})
		}
	}

	if report.Commander != "" {
		wla.WinLines = append(wla.WinLines, WinLine{
			Pieces: []string{report.Commander},
			Type:   "commander_damage",
			Desc:   "21 commander damage",
			Rationale: &WinLineRationale{
				Forms:      []string{report.Commander},
				Conditions: []string{"Cast commander from the command zone", "Connect with combat damage repeatedly until target accumulates 21+ commander damage"},
				Resolves:   []string{"Each opponent loses individually once they reach 21 commander damage from this commander."},
			},
		})
	}

	threatCount := 0
	pumpCount := 0
	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
		}
		tl := strings.ToLower(qp.Profile.TypeLine)
		if strings.Contains(tl, "creature") && qp.Profile.CMC >= 3 {
			threatCount += qp.Qty
		}
		var ot string
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
			}
		}
		if containsAny(ot, "creatures you control get", "all creatures get", "overrun", "trample") {
			pumpCount += qp.Qty
		}
	}
	if threatCount >= 10 || pumpCount >= 2 {
		wla.WinLines = append(wla.WinLines, WinLine{
			Pieces: []string{fmt.Sprintf("%d threats + %d pumps", threatCount, pumpCount)},
			Type:   "combat",
			Desc:   "combat damage with creature pressure",
			Rationale: &WinLineRationale{
				Forms: []string{
					fmt.Sprintf("%d creatures CMC 3+ in deck", threatCount),
					fmt.Sprintf("%d anthem/overrun-style pump effects", pumpCount),
				},
				Conditions: []string{
					"Maintain a board through opposing removal",
					"Stick at least one pump effect to convert chip damage into lethal",
				},
				Resolves: []string{"Develop a wide or tall board, alpha-strike with a pump in play, and split combat damage across opponents until they're knocked out."},
			},
		})
	}
}

func altWinconResolves(name, ot string) string {
	low := strings.ToLower(ot)
	switch {
	case strings.Contains(low, "wins the game"):
		return name + " has a static or triggered ability that explicitly says 'wins the game' once a condition is met."
	case strings.Contains(low, "loses the game"):
		return name + " has a static or triggered ability that makes opponents lose the game once a condition is met."
	case strings.Contains(low, "each opponent") &&
		(strings.Contains(low, "loses life") || strings.Contains(low, "loses ") && strings.Contains(low, " life")):
		return name + " drains each opponent's life total — chain triggers or copies to push them to 0."
	}
	return "Card is tagged as an alternate win condition by oracle text. Resolve its triggered/activated/static win clause."
}

func alreadyInWinLines(wla *WinLineAnalysis, name string) bool {
	for _, wl := range wla.WinLines {
		for _, p := range wl.Pieces {
			if p == name {
				return true
			}
		}
	}
	return false
}

func computeBackupPlans(wla *WinLineAnalysis) {
	if len(wla.WinLines) <= 1 {
		return
	}

	tutorCoverage := map[int]int{}
	for i, wl := range wla.WinLines {
		tutorCoverage[i] = len(wl.TutorPaths)
	}

	bestIdx := 0
	bestCount := 0
	for i, count := range tutorCoverage {
		if count > bestCount {
			bestCount = count
			bestIdx = i
		}
	}

	for i, wl := range wla.WinLines {
		if i == bestIdx {
			continue
		}
		label := strings.Join(wl.Pieces, " + ")
		if wl.Type == "combat" || wl.Type == "commander_damage" {
			wla.BackupPlans = append(wla.BackupPlans,
				fmt.Sprintf("Backup: %s (%s)", label, wl.Type))
		} else {
			paths := tutorCoverage[i]
			if paths > 0 {
				wla.BackupPlans = append(wla.BackupPlans,
					fmt.Sprintf("Backup: %s (%d tutor paths)", label, paths))
			} else {
				wla.BackupPlans = append(wla.BackupPlans,
					fmt.Sprintf("Backup: %s (no tutor access — must draw naturally)", label))
			}
		}
	}
}

func computeRedundancy(wla *WinLineAnalysis, report *FreyaReport, profileByName map[string]CardProfile) {
	for _, p := range profileByName {
		if p.IsLand {
			continue
		}
		if p.IsWinCon {
			wla.RedundancyMap["win_condition"]++
		}
		if p.IsOutlet {
			wla.RedundancyMap["sacrifice_outlet"]++
		}
		if p.IsTutor {
			wla.RedundancyMap["tutor"]++
		}
		if p.IsMassWipe {
			wla.RedundancyMap["board_wipe"]++
		}
		for _, r := range p.Produces {
			if r == ResCard {
				wla.RedundancyMap["draw_engine"]++
				break
			}
		}
		for _, r := range p.Produces {
			if r == ResMana {
				wla.RedundancyMap["mana_source"]++
				break
			}
		}
	}
}

func computeSinglePoints(wla *WinLineAnalysis, profileByName map[string]CardProfile) {
	for _, wl := range wla.WinLines {
		if wl.Type == "combat" || wl.Type == "commander_damage" {
			continue
		}
		for _, piece := range wl.Pieces {
			if len(wl.TutorPaths) == 0 {
				continue
			}
			hasTutor := false
			for _, tp := range wl.TutorPaths {
				if tp.Finds == piece {
					hasTutor = true
					break
				}
			}
			if !hasTutor {
				wla.SinglePoints = append(wla.SinglePoints,
					fmt.Sprintf("%s is needed for [%s] but no tutor in the deck can find it",
						piece, strings.Join(wl.Pieces, " + ")))
			}
		}
	}

	wla.SinglePoints = uniqueStrings(wla.SinglePoints)

	if wla.RedundancyMap["win_condition"] == 1 {
		wla.SinglePoints = append(wla.SinglePoints,
			"only 1 card tagged as win condition — if exiled, no backup win con")
	}
	if wla.RedundancyMap["sacrifice_outlet"] == 1 {
		wla.SinglePoints = append(wla.SinglePoints,
			"only 1 sacrifice outlet — if removed, combo lines that need sac are dead")
	}
}
