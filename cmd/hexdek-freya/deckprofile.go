package main

import (
	"fmt"
	"sort"
	"strings"
)

type DeckProfile struct {
	DeckName      string
	Commander     string
	ColorIdentity []string
	CardCount     int

	AvgCMC         float64
	LandCount      int
	RecommendedLands int
	LandVerdict    string
	RampCount      int
	DrawCount      int

	RoleCounts map[RoleTag]int
	TopRoles   []RoleCount

	PrimaryArchetype    string
	SecondaryArchetype  string
	ArchetypeConfidence float64
	Bracket             int
	BracketLabel        string
	PlaysLike           int
	PlaysLikeLabel      string
	GameChangerCount    int
	GameChangerCards    []string
	Intent              string

	PrimaryWinLine    string
	WinLineCount      int
	BackupCount       int
	HasTutorAccess    bool
	SinglePointCount  int

	Strengths       []string
	Weaknesses      []string
	GameplanSummary string

	CommanderSynergy    float64  // 0.0-1.0 ratio of cards synergizing with commander
	CommanderThemes     []string // detected themes from commander oracle text
	SynergyCount        int      // cards in 99 that match commander themes

	InteractionQuality  float64 // avg CMC of interaction spells (lower = faster)
	CheapInteraction    int     // interaction at CMC 0-2
	ExpensiveInteraction int    // interaction at CMC 4+
	ProtectedKeyPieces  int     // key pieces with built-in protection
	UnprotectedKeyPieces int    // key pieces without built-in protection

	// Mana base grading
	ManaBaseGrade    string // A/B/C/D/F
	ManaBaseNotes    []string
	TaplandCount     int
	FetchCount       int
	UtilityLandCount int

	// Threat assessment
	VulnerableTo []string // specific hosers this deck fears

	// Opening hand simulation
	KeepableHandPct  float64 // % of hands with 2-5 lands + action
	AvgTurnToFourMana float64

	// Commander-centric hand evaluation: for decks where the gameplan IS the
	// commander (Voltron, high commander-synergy engines), the standard
	// keepable heuristic over-penalizes hands that lack standalone threats.
	// KeepableHandPctAdjusted relaxes the action requirement when the
	// commander itself supplies the engine.
	IsCommanderCentric       bool
	CommanderCentricReason   string
	KeepableHandPctAdjusted  float64
	AvgTurnToCommander       float64
	CommanderCMC             int

	// Synergy clusters
	SynergyClusters []SynergyCluster

	// Alt-build suggestions — when ≥2 clusters are above the pivot
	// threshold, surface each non-primary as a "you could refocus
	// around X" candidate so the Decks screen can show "this deck is
	// trying to do two things; pick one." See computeAltBuildSuggestions.
	AltBuildSuggestions []AltBuildSuggestion

	// Meta positioning
	MetaMatchups []MetaMatchup

	// "Favored against" reverse-lookup — archetypes this deck reliably
	// beats, combined from both forward (X's own favored matchups) and
	// reverse (other archetypes that list themselves unfavored vs X)
	// directions of the matchup DB. See MetaStrongAgainst for details.
	// Surfaced to the Decks screen as the "good against" summary.
	StrongAgainst []MetaAdvantage

	// Card quality tiers — star = clear keepers tied to wincon / value
	// chain; solid = okay-but-replaceable, scoring above the cut floor
	// but below the star threshold (worth flagging for builders shopping
	// upgrades); cuttable = bottom of the deck on the synergy evaluator.
	CuttableCards []CardQuality
	SolidCards    []CardQuality
	StarCards     []CardQuality

	// Color weight suggestions
	LandSwapSuggestions []string

	// Deck personality
	PersonalityBlurb string

	// Power ranking
	PowerPercentile int    // 0-100 estimated percentile within archetype
	PowerFactors    []string
}

type RoleCount struct {
	Role  RoleTag
	Count int
}

type SynergyCluster struct {
	Name        string
	Cards       []string // capped at 8 for display
	Theme       string
	Score       int // number of pairwise synergies within the cluster
	MemberCount int // full deduped count (uncapped) — used by alt-build threshold
}

// AltBuildSuggestion is a "you could re-focus the deck around X" hint
// surfaced when the deck has multiple synergy clusters above the
// pivotable-size threshold — the deckbuilder is splitting slot
// priority across two engines and could commit to one. See
// computeAltBuildSuggestions.
type AltBuildSuggestion struct {
	Cluster     string // theme key (e.g. "tokens", "recursion")
	ClusterName string // display name ("Token Engine")
	MemberCount int    // cards already aligned with the theme
	Score       int    // pair-weighted score from the cluster
	Pivot       string // "You could refocus around X — ..."
	Trade       string // "Currently splits with Y — picking one frees N slots"
}

type MetaMatchup struct {
	Archetype string
	Rating    string // "favored", "neutral", "unfavored"
	Reason    string
}

// MetaAdvantage is the per-entry shape of the "favored against" list
// produced by MetaStrongAgainst. Reason is the headline why-line
// (forward perspective if available — "X beats Y because Z"); when
// Source is "both", OpponentReason carries the corroborating reverse
// perspective ("Y can't answer X because W").
type MetaAdvantage struct {
	Archetype      string
	Reason         string
	OpponentReason string // populated only when Source == "both"
	Source         string // "forward" | "reverse" | "both"
}

type CardQuality struct {
	Name   string
	Tier   string // "star" | "solid" | "cuttable"
	Reason string
	// Rationale fields (populated for cuttable tier).
	Detected  string   // what stat/pattern triggered the recommendation
	WhyCut    string   // why cutting it is recommended
	Effect    string   // resulting effect on the deck if cut
	Suggested []string // suggested swap candidates
}

func BuildDeckProfile(report *FreyaReport, oracle *oracleDB) *DeckProfile {
	dp := &DeckProfile{
		DeckName:  report.DeckName,
		Commander: report.Commander,
		CardCount: report.TotalCards,
		AvgCMC:    report.AvgCMC,
		LandCount: report.LandCount,
	}

	if oracle != nil && report.Commander != "" {
		entry := oracle.lookup(report.Commander)
		if entry != nil {
			dp.ColorIdentity = entry.ColorIdentity
		}
	}

	if report.Stats != nil {
		dp.RecommendedLands = report.Stats.RecommendedLands
		dp.LandVerdict = report.Stats.LandVerdict
		dp.RampCount = report.Stats.RampCount
		dp.DrawCount = report.Stats.DrawSourceCount
	}

	if report.Roles != nil {
		dp.RoleCounts = report.Roles.RoleCounts
		dp.TopRoles = topNRoles(report.Roles.RoleCounts, 3)
	}

	if report.Archetype != nil {
		dp.PrimaryArchetype = report.Archetype.Primary
		dp.SecondaryArchetype = report.Archetype.Secondary
		dp.ArchetypeConfidence = report.Archetype.PrimaryConfidence
		dp.Bracket = report.Archetype.Bracket
		dp.BracketLabel = report.Archetype.BracketLabel
		dp.PlaysLike = report.Archetype.PlaysLike
		dp.PlaysLikeLabel = report.Archetype.PlaysLikeLabel
		dp.GameChangerCount = report.Archetype.GameChangerCount
		dp.GameChangerCards = report.Archetype.GameChangerCards
		dp.Intent = report.Archetype.Intent
	}

	if report.WinLines != nil {
		dp.WinLineCount = len(report.WinLines.WinLines)
		dp.BackupCount = len(report.WinLines.BackupPlans)
		dp.SinglePointCount = len(report.WinLines.SinglePoints)
		if len(report.WinLines.WinLines) > 0 {
			wl := report.WinLines.WinLines[0]
			dp.PrimaryWinLine = strings.Join(wl.Pieces, " + ")
			dp.HasTutorAccess = len(wl.TutorPaths) > 0
		}
	}

	if oracle != nil && report.Commander != "" {
		computeCommanderSynergy(dp, report, oracle)
	}

	computeInteractionQuality(dp, report, oracle)
	computeProtectionDensity(dp, report, oracle)
	computeManaBaseGrade(dp, report, oracle)
	// Opening hand sim runs first so dp.IsCommanderCentric is populated
	// before threat assessment reads it (commander-centric decks fear
	// commander-targeting hosers like Imprisoned in the Moon).
	computeOpeningHandSim(dp, report, oracle)
	computeThreatAssessment(dp, report)
	computeSynergyClusters(dp, report, oracle)
	computeAltBuildSuggestions(dp)
	computeMetaPositioning(dp)
	computeCardQualityTiers(dp, report, oracle)
	computeLandSwapSuggestions(dp, report)
	dp.PersonalityBlurb = buildPersonalityBlurb(dp, report)
	dp.PowerPercentile, dp.PowerFactors = estimatePowerPercentile(dp, report)

	dp.Strengths = deriveStrengths(report, dp)
	dp.Weaknesses = deriveWeaknesses(report, dp)
	dp.GameplanSummary = buildGameplanSummary(dp, report)

	return dp
}

func computeInteractionQuality(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.Roles == nil {
		return
	}

	totalCMC := 0
	count := 0
	for _, a := range report.Roles.Assignments {
		isInteraction := false
		for _, r := range a.Roles {
			if r == RoleRemoval || r == RoleCounterspell || r == RoleBoardWipe {
				isInteraction = true
				break
			}
		}
		if !isInteraction {
			continue
		}

		var cmc int
		for _, p := range report.Profiles {
			if p.Name == a.Name {
				cmc = p.CMC
				break
			}
		}

		count++
		totalCMC += cmc
		if cmc <= 2 {
			dp.CheapInteraction++
		} else if cmc >= 4 {
			dp.ExpensiveInteraction++
		}
	}

	if count > 0 {
		dp.InteractionQuality = float64(totalCMC) / float64(count)
	}
}

func computeProtectionDensity(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.Roles == nil || oracle == nil {
		return
	}

	keyRoles := map[RoleTag]bool{RoleCombo: true, RoleThreat: true}
	protectionWords := []string{
		"hexproof", "shroud", "indestructible", "ward",
		"protection from", "can't be the target",
		"can't be countered", "can't be destroyed",
		"phase out", "phases out",
	}

	for _, a := range report.Roles.Assignments {
		isKey := false
		for _, r := range a.Roles {
			if keyRoles[r] {
				isKey = true
				break
			}
		}
		if !isKey {
			continue
		}

		entry := oracle.lookup(a.Name)
		if entry == nil {
			dp.UnprotectedKeyPieces++
			continue
		}

		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}

		hasProtection := false
		for _, word := range protectionWords {
			if strings.Contains(ot, word) {
				hasProtection = true
				break
			}
		}

		if hasProtection {
			dp.ProtectedKeyPieces++
		} else {
			dp.UnprotectedKeyPieces++
		}
	}
}

var commanderThemePatterns = []struct {
	Theme    string
	Patterns []string
}{
	{"sacrifice", []string{"sacrifice", "sacrificed", "sac a"}},
	{"tokens", []string{"create", "token", "populate", "embalm", "encore"}},
	{"counters", []string{"+1/+1 counter", "proliferate", "counter on"}},
	{"graveyard", []string{"graveyard", "mill", "dredge", "surveil", "reanimate", "return from your graveyard"}},
	{"lifegain", []string{"gain life", "lifelink", "whenever you gain life"}},
	{"spellcasting", []string{"instant", "sorcery", "whenever you cast", "magecraft", "prowess", "storm"}},
	{"combat", []string{"attacks", "combat damage", "combat phase", "first strike", "double strike"}},
	{"artifacts", []string{"artifact", "treasure", "equipment", "equip"}},
	{"enchantments", []string{"enchantment", "aura", "constellation", "enchanted"}},
	{"lands", []string{"landfall", "land enters", "search your library for a land"}},
	{"blink", []string{"exile", "return", "flicker", "exile target creature you control"}},
	{"discard", []string{"discard", "each opponent discards", "madness"}},
	{"tribal", []string{"creature type", "all creatures of the chosen type", "creatures you control get"}},
	{"drawing", []string{"draw", "draws a card", "whenever you draw"}},
}

func computeCommanderSynergy(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	cmdrEntry := oracle.lookup(report.Commander)
	if cmdrEntry == nil {
		return
	}

	cmdrOT := strings.ToLower(cmdrEntry.OracleText)
	if cmdrOT == "" && len(cmdrEntry.CardFaces) > 0 {
		cmdrOT = strings.ToLower(cmdrEntry.CardFaces[0].OracleText)
		if len(cmdrEntry.CardFaces) > 1 {
			cmdrOT += " " + strings.ToLower(cmdrEntry.CardFaces[1].OracleText)
		}
	}

	var themes []string
	for _, tp := range commanderThemePatterns {
		for _, pat := range tp.Patterns {
			if strings.Contains(cmdrOT, pat) {
				themes = append(themes, tp.Theme)
				break
			}
		}
	}
	dp.CommanderThemes = themes

	if len(themes) == 0 {
		return
	}

	synergyCount := 0
	nonlandCount := 0
	for _, p := range report.Profiles {
		if p.IsLand || p.Name == report.Commander {
			continue
		}
		nonlandCount++

		entry := oracle.lookup(p.Name)
		if entry == nil {
			continue
		}
		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		tl := strings.ToLower(p.TypeLine)

		for _, theme := range themes {
			matched := false
			for _, tp := range commanderThemePatterns {
				if tp.Theme != theme {
					continue
				}
				for _, pat := range tp.Patterns {
					if strings.Contains(ot, pat) || strings.Contains(tl, pat) {
						matched = true
						break
					}
				}
				break
			}
			if matched {
				synergyCount++
				break
			}
		}
	}

	dp.SynergyCount = synergyCount
	if nonlandCount > 0 {
		dp.CommanderSynergy = float64(synergyCount) / float64(nonlandCount)
	}
}

func topNRoles(counts map[RoleTag]int, n int) []RoleCount {
	var rcs []RoleCount
	for role, count := range counts {
		if count > 0 && role != RoleLand && role != RoleUtility {
			rcs = append(rcs, RoleCount{Role: role, Count: count})
		}
	}
	sort.Slice(rcs, func(i, j int) bool {
		return rcs[i].Count > rcs[j].Count
	})
	if len(rcs) > n {
		rcs = rcs[:n]
	}
	return rcs
}

func deriveStrengths(report *FreyaReport, dp *DeckProfile) []string {
	var s []string

	// Use NonLandTutorCount — basic-land ramp should not inflate the
	// "tutor package" headline. A 100-card deck with 10 ramp pieces and
	// 0 real tutors is not a tutor deck.
	if report.NonLandTutorCount >= 8 {
		s = append(s, fmt.Sprintf("deep tutor package (%d tutors)", report.NonLandTutorCount))
	} else if report.NonLandTutorCount >= 5 {
		s = append(s, fmt.Sprintf("strong tutor package (%d tutors)", report.NonLandTutorCount))
	}

	if dp.DrawCount >= 12 {
		s = append(s, fmt.Sprintf("excellent draw density (%d sources)", dp.DrawCount))
	} else if dp.DrawCount >= 8 {
		s = append(s, fmt.Sprintf("good draw density (%d sources)", dp.DrawCount))
	}

	if dp.RampCount >= 14 {
		s = append(s, fmt.Sprintf("heavy ramp package (%d pieces)", dp.RampCount))
	} else if dp.RampCount >= 10 {
		s = append(s, fmt.Sprintf("solid ramp package (%d pieces)", dp.RampCount))
	}

	if dp.WinLineCount >= 5 {
		s = append(s, fmt.Sprintf("diverse win lines (%d paths to victory)", dp.WinLineCount))
	} else if dp.WinLineCount >= 3 {
		s = append(s, fmt.Sprintf("multiple win lines (%d paths)", dp.WinLineCount))
	}

	if dp.SinglePointCount == 0 && dp.WinLineCount > 0 {
		s = append(s, "no single points of failure")
	}

	if dp.LandVerdict == "ok" {
		s = append(s, "land count on target")
	}

	if report.Stats != nil && len(report.Stats.ColorGaps) == 0 {
		s = append(s, "balanced mana base")
	}

	if dp.CommanderSynergy >= 0.60 {
		s = append(s, fmt.Sprintf("strong commander synergy (%.0f%% of cards match themes)", dp.CommanderSynergy*100))
	} else if dp.CommanderSynergy >= 0.40 {
		s = append(s, fmt.Sprintf("good commander synergy (%.0f%%)", dp.CommanderSynergy*100))
	}

	if dp.InteractionQuality > 0 && dp.InteractionQuality <= 2.0 {
		s = append(s, fmt.Sprintf("fast interaction (avg CMC %.1f, %d pieces at CMC ≤2)", dp.InteractionQuality, dp.CheapInteraction))
	}

	if dp.ProtectedKeyPieces > 0 {
		total := dp.ProtectedKeyPieces + dp.UnprotectedKeyPieces
		if total > 0 && float64(dp.ProtectedKeyPieces)/float64(total) >= 0.40 {
			s = append(s, fmt.Sprintf("well-protected key pieces (%d/%d have built-in protection)", dp.ProtectedKeyPieces, total))
		}
	}

	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction >= 12 {
		s = append(s, fmt.Sprintf("heavy interaction suite (%d pieces)", interaction))
	} else if interaction >= 8 {
		s = append(s, fmt.Sprintf("good interaction density (%d pieces)", interaction))
	}

	return s
}

func deriveWeaknesses(report *FreyaReport, dp *DeckProfile) []string {
	var w []string

	if report.NonLandTutorCount < 3 && report.NonLandTutorCount > 0 {
		w = append(w, fmt.Sprintf("thin tutor package (%d tutors)", report.NonLandTutorCount))
	} else if report.NonLandTutorCount == 0 {
		w = append(w, "no tutors")
	}

	if dp.DrawCount < 5 && dp.DrawCount > 0 {
		w = append(w, fmt.Sprintf("low draw density (%d sources)", dp.DrawCount))
	} else if dp.DrawCount == 0 {
		w = append(w, "no dedicated draw sources")
	}

	if dp.RampCount < 7 && dp.RampCount > 0 {
		w = append(w, fmt.Sprintf("light ramp (%d pieces)", dp.RampCount))
	}

	if dp.WinLineCount <= 1 {
		w = append(w, "limited win conditions")
	}

	if dp.SinglePointCount > 0 {
		w = append(w, fmt.Sprintf("%d single point(s) of failure", dp.SinglePointCount))
	}

	if dp.LandVerdict == "too_few" {
		w = append(w, fmt.Sprintf("low land count (%d vs %d recommended)", dp.LandCount, dp.RecommendedLands))
	} else if dp.LandVerdict == "too_many" {
		w = append(w, fmt.Sprintf("high land count (%d vs %d recommended)", dp.LandCount, dp.RecommendedLands))
	}

	if report.Stats != nil && len(report.Stats.ColorGaps) > 0 {
		w = append(w, fmt.Sprintf("%d color gap(s) in mana base", len(report.Stats.ColorGaps)))
	}

	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction < 5 {
		w = append(w, fmt.Sprintf("low interaction (%d removal + counterspells)", interaction))
	}

	if report.Roles != nil && report.Roles.RoleCounts[RoleBoardWipe] == 0 {
		w = append(w, "no board wipes")
	}

	if dp.CommanderSynergy > 0 && dp.CommanderSynergy < 0.25 {
		w = append(w, fmt.Sprintf("low commander synergy (%.0f%%) — many cards don't align with commander themes", dp.CommanderSynergy*100))
	}

	if dp.InteractionQuality > 3.5 {
		w = append(w, fmt.Sprintf("slow interaction (avg CMC %.1f) — may not answer fast threats in time", dp.InteractionQuality))
	}

	if dp.UnprotectedKeyPieces > 0 {
		total := dp.ProtectedKeyPieces + dp.UnprotectedKeyPieces
		if total >= 3 && float64(dp.UnprotectedKeyPieces)/float64(total) >= 0.80 {
			w = append(w, fmt.Sprintf("key pieces exposed (%d/%d combo/threat pieces lack built-in protection)", dp.UnprotectedKeyPieces, total))
		}
	}

	return w
}

// ComputeEvalWeights derives MCTS evaluator weights from the deck profile.
// Starts from archetype defaults and adjusts based on deck-specific signals
// (tutor density, graveyard recursion, ramp count, win line structure).
func ComputeEvalWeights(dp *DeckProfile, report *FreyaReport) *jsonEvalWeights {
	arch := strings.ToLower(dp.PrimaryArchetype)
	if arch == "" {
		arch = "midrange"
	}

	defaults := defaultWeights[arch]
	if defaults == nil {
		defaults = defaultWeights["midrange"]
	}

	w := *defaults

	// Tutor density boosts combo proximity weight. NonLandTutorCount only —
	// fetchlands/ramp don't help find combo pieces.
	if report.NonLandTutorCount >= 8 {
		w.ComboProximity += 0.3
	} else if report.NonLandTutorCount >= 5 {
		w.ComboProximity += 0.15
	}

	// Recursion-heavy decks get graveyard value boost.
	recursionCount := 0
	for _, p := range report.Profiles {
		if p.IsRecursion {
			recursionCount++
		}
	}
	if recursionCount >= 5 {
		w.GraveyardValue += 0.4
	} else if recursionCount >= 3 {
		w.GraveyardValue += 0.2
	}

	// Heavy ramp package boosts mana advantage weight.
	if dp.RampCount >= 14 {
		w.ManaAdvantage += 0.3
	} else if dp.RampCount >= 10 {
		w.ManaAdvantage += 0.15
	}

	// Multiple win lines with tutor access boosts combo proximity.
	if dp.WinLineCount >= 3 && dp.HasTutorAccess {
		w.ComboProximity += 0.2
	}

	// Low interaction decks should weight threat exposure higher.
	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction < 5 {
		w.ThreatExposure += 0.3
	}

	return &w
}

var defaultWeights = map[string]*jsonEvalWeights{
	"aggro": {
		BoardPresence: 1.5, CardAdvantage: 0.4, ManaAdvantage: 0.3,
		LifeResource: 0.8, ComboProximity: 0.1, ThreatExposure: 0.6,
		CommanderProgress: 0.9, GraveyardValue: 0.2,
	},
	"aggro / go wide": {
		BoardPresence: 1.8, CardAdvantage: 0.4, ManaAdvantage: 0.3,
		LifeResource: 0.7, ComboProximity: 0.1, ThreatExposure: 0.5,
		CommanderProgress: 0.6, GraveyardValue: 0.2,
	},
	"combo": {
		BoardPresence: 0.4, CardAdvantage: 0.8, ManaAdvantage: 0.7,
		LifeResource: 0.3, ComboProximity: 2.0, ThreatExposure: 0.5,
		CommanderProgress: 0.6, GraveyardValue: 0.5,
	},
	"combo / infinite": {
		BoardPresence: 0.3, CardAdvantage: 0.9, ManaAdvantage: 0.8,
		LifeResource: 0.2, ComboProximity: 2.2, ThreatExposure: 0.5,
		CommanderProgress: 0.5, GraveyardValue: 0.6,
	},
	"control": {
		BoardPresence: 0.5, CardAdvantage: 1.5, ManaAdvantage: 0.8,
		LifeResource: 0.6, ComboProximity: 0.4, ThreatExposure: 1.2,
		CommanderProgress: 0.5, GraveyardValue: 0.4,
	},
	"midrange": {
		BoardPresence: 1.0, CardAdvantage: 1.0, ManaAdvantage: 0.8,
		LifeResource: 0.7, ComboProximity: 0.5, ThreatExposure: 0.8,
		CommanderProgress: 0.7, GraveyardValue: 0.5,
	},
	"ramp": {
		BoardPresence: 0.6, CardAdvantage: 0.7, ManaAdvantage: 1.8,
		LifeResource: 0.5, ComboProximity: 0.3, ThreatExposure: 0.6,
		CommanderProgress: 0.8, GraveyardValue: 0.3,
	},
	"voltron": {
		BoardPresence: 0.8, CardAdvantage: 0.5, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.2, ThreatExposure: 0.9,
		CommanderProgress: 2.0, GraveyardValue: 0.3,
	},
	"storm": {
		BoardPresence: 0.2, CardAdvantage: 1.3, ManaAdvantage: 1.5,
		LifeResource: 0.2, ComboProximity: 1.8, ThreatExposure: 0.3,
		CommanderProgress: 0.4, GraveyardValue: 0.3,
	},
	"aristocrats": {
		BoardPresence: 1.0, CardAdvantage: 0.8, ManaAdvantage: 0.5,
		LifeResource: 0.9, ComboProximity: 1.2, ThreatExposure: 0.7,
		CommanderProgress: 0.5, GraveyardValue: 1.2,
	},
	"artifacts": {
		BoardPresence: 1.2, CardAdvantage: 0.8, ManaAdvantage: 1.0,
		LifeResource: 0.4, ComboProximity: 1.0, ThreatExposure: 0.6,
		CommanderProgress: 0.6, GraveyardValue: 0.8,
	},
	"enchantress": {
		BoardPresence: 0.8, CardAdvantage: 1.4, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 0.8, ThreatExposure: 0.7,
		CommanderProgress: 0.6, GraveyardValue: 0.3,
	},
	"reanimator": {
		BoardPresence: 0.7, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.4, ComboProximity: 0.8, ThreatExposure: 0.5,
		CommanderProgress: 0.5, GraveyardValue: 2.0,
	},
	"lands matter": {
		BoardPresence: 0.7, CardAdvantage: 0.7, ManaAdvantage: 1.8,
		LifeResource: 0.6, ComboProximity: 0.4, ThreatExposure: 0.5,
		CommanderProgress: 0.7, GraveyardValue: 0.8,
	},
	"tribal": {
		BoardPresence: 1.6, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.7, ComboProximity: 0.3, ThreatExposure: 0.7,
		CommanderProgress: 0.8, GraveyardValue: 0.3,
	},
	"superfriends": {
		BoardPresence: 0.9, CardAdvantage: 1.2, ManaAdvantage: 0.7,
		LifeResource: 0.8, ComboProximity: 0.4, ThreatExposure: 1.0,
		CommanderProgress: 0.4, GraveyardValue: 0.3,
	},
	"mill": {
		BoardPresence: 0.4, CardAdvantage: 0.8, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 1.4, ThreatExposure: 0.8,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"lifegain": {
		BoardPresence: 0.9, CardAdvantage: 0.7, ManaAdvantage: 0.5,
		LifeResource: 1.8, ComboProximity: 0.8, ThreatExposure: 0.6,
		CommanderProgress: 0.6, GraveyardValue: 0.3,
	},
	"discard / hand attack": {
		BoardPresence: 0.6, CardAdvantage: 1.4, ManaAdvantage: 0.5,
		LifeResource: 0.5, ComboProximity: 0.6, ThreatExposure: 1.0,
		CommanderProgress: 0.7, GraveyardValue: 0.4,
	},
	"blink / flicker": {
		BoardPresence: 1.1, CardAdvantage: 1.0, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 0.9, ThreatExposure: 0.6,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"spellslinger": {
		BoardPresence: 0.3, CardAdvantage: 1.4, ManaAdvantage: 1.0,
		LifeResource: 0.3, ComboProximity: 1.2, ThreatExposure: 0.4,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"counters matter": {
		BoardPresence: 1.4, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.8, ThreatExposure: 0.6,
		CommanderProgress: 0.7, GraveyardValue: 0.3,
	},
	"stax": {
		BoardPresence: 0.5, CardAdvantage: 1.2, ManaAdvantage: 0.8,
		LifeResource: 0.4, ComboProximity: 0.6, ThreatExposure: 1.4,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"extra combats": {
		BoardPresence: 1.4, CardAdvantage: 0.5, ManaAdvantage: 0.5,
		LifeResource: 0.7, ComboProximity: 0.6, ThreatExposure: 0.6,
		CommanderProgress: 1.5, GraveyardValue: 0.2,
	},
	"theft / clone": {
		BoardPresence: 0.8, CardAdvantage: 1.0, ManaAdvantage: 0.7,
		LifeResource: 0.5, ComboProximity: 0.5, ThreatExposure: 1.0,
		CommanderProgress: 0.5, GraveyardValue: 0.4,
	},
	"ninjutsu / evasion": {
		BoardPresence: 1.0, CardAdvantage: 1.2, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.3, ThreatExposure: 0.5,
		CommanderProgress: 0.8, GraveyardValue: 0.2,
	},
}

type jsonEvalWeights struct {
	BoardPresence     float64 `json:"board_presence"`
	CardAdvantage     float64 `json:"card_advantage"`
	ManaAdvantage     float64 `json:"mana_advantage"`
	LifeResource      float64 `json:"life_resource"`
	ComboProximity    float64 `json:"combo_proximity"`
	ThreatExposure    float64 `json:"threat_exposure"`
	CommanderProgress float64 `json:"commander_progress"`
	GraveyardValue    float64 `json:"graveyard_value"`
}

func buildGameplanSummary(dp *DeckProfile, report *FreyaReport) string {
	archetype := dp.PrimaryArchetype
	if archetype == "" {
		archetype = "Midrange"
	}

	var winMethod string
	if dp.WinLineCount > 0 && dp.PrimaryWinLine != "" {
		first := report.WinLines.WinLines[0]
		switch first.Type {
		case "infinite", "determined":
			winMethod = dp.PrimaryWinLine + " combo"
		case "finisher":
			winMethod = dp.PrimaryWinLine
		case "combat":
			winMethod = "combat damage"
		case "commander_damage":
			winMethod = "commander damage"
		default:
			winMethod = dp.PrimaryWinLine
		}
	} else {
		winMethod = "combat damage"
	}

	var backup string
	comboLines := 0
	if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			if wl.Type == "infinite" || wl.Type == "determined" || wl.Type == "finisher" {
				comboLines++
			}
		}
	}
	if comboLines > 1 {
		backup = fmt.Sprintf(" %d backup lines available.", comboLines-1)
	} else if dp.BackupCount > 0 {
		backup = fmt.Sprintf(" %d backup plan(s).", dp.BackupCount)
	}

	var tutorNote string
	if dp.HasTutorAccess && report.NonLandTutorCount >= 5 {
		tutorNote = fmt.Sprintf(" Supported by %d tutors.", report.NonLandTutorCount)
	}

	var gcNote string
	if dp.GameChangerCount > 0 {
		gcNote = fmt.Sprintf(", %d GC", dp.GameChangerCount)
	}
	var playsLikeNote string
	if dp.PlaysLike != dp.Bracket {
		playsLikeNote = fmt.Sprintf(" Plays like B%d (%s).", dp.PlaysLike, dp.PlaysLikeLabel)
	}
	bracket := fmt.Sprintf(" Strict B%d (%s%s).%s", dp.Bracket, dp.BracketLabel, gcNote, playsLikeNote)

	return fmt.Sprintf("%s deck that wins via %s.%s%s%s",
		archetype, winMethod, backup, tutorNote, bracket)
}
