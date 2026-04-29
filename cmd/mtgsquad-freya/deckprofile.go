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
	Intent              string

	PrimaryWinLine    string
	WinLineCount      int
	BackupCount       int
	HasTutorAccess    bool
	SinglePointCount  int

	Strengths       []string
	Weaknesses      []string
	GameplanSummary string
}

type RoleCount struct {
	Role  RoleTag
	Count int
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

	dp.Strengths = deriveStrengths(report, dp)
	dp.Weaknesses = deriveWeaknesses(report, dp)
	dp.GameplanSummary = buildGameplanSummary(dp, report)

	return dp
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

	if report.TutorCount >= 8 {
		s = append(s, fmt.Sprintf("deep tutor package (%d tutors)", report.TutorCount))
	} else if report.TutorCount >= 5 {
		s = append(s, fmt.Sprintf("strong tutor package (%d tutors)", report.TutorCount))
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

	if report.TutorCount < 3 && report.TutorCount > 0 {
		w = append(w, fmt.Sprintf("thin tutor package (%d tutors)", report.TutorCount))
	} else if report.TutorCount == 0 {
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

	// Tutor density boosts combo proximity weight.
	if report.TutorCount >= 8 {
		w.ComboProximity += 0.3
	} else if report.TutorCount >= 5 {
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
	"combo": {
		BoardPresence: 0.4, CardAdvantage: 0.8, ManaAdvantage: 0.7,
		LifeResource: 0.3, ComboProximity: 2.0, ThreatExposure: 0.5,
		CommanderProgress: 0.6, GraveyardValue: 0.5,
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
	if dp.HasTutorAccess && report.TutorCount >= 5 {
		tutorNote = fmt.Sprintf(" Supported by %d tutors.", report.TutorCount)
	}

	bracket := fmt.Sprintf(" Plays at bracket %d/5 (%s).", dp.Bracket, dp.BracketLabel)

	return fmt.Sprintf("%s deck that wins via %s.%s%s%s",
		archetype, winMethod, backup, tutorNote, bracket)
}
