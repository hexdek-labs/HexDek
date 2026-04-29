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
	Signals           []string
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
	equipAuraCount int
	spellCopyCount int
	profiles       []CardProfile
	qtyProfiles    []CardProfileQty
	oracle         *oracleDB
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
		Name: "Aristocrats",
		Ratios: map[RoleTag]float64{
			RoleThreat: 0.10, RoleCombo: 0.06, RoleDraw: 0.10, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.sacrificeCount >= 5 && ctx.deathTriggers >= 3
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
		Name: "Reanimator",
		Ratios: map[RoleTag]float64{
			RoleDraw: 0.10, RoleTutor: 0.08, RoleThreat: 0.10, RoleRamp: 0.08,
		},
		Require: func(ctx *classifyContext) bool {
			return ctx.graveyardCount >= 6 && ctx.selfMillCount >= 2
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
	ac.Bracket, ac.BracketLabel = estimateBracket(ctx, report)

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
	creatureTypes := map[string]int{}

	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
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

func estimateBracket(ctx *classifyContext, report *FreyaReport) (int, string) {
	score := 0

	if ctx.tutorDensity >= 0.12 {
		score += 3
	} else if ctx.tutorDensity >= 0.08 {
		score += 2
	} else if ctx.tutorDensity >= 0.04 {
		score += 1
	}

	if ctx.comboCount >= 5 {
		score += 3
	} else if ctx.comboCount >= 2 {
		score += 2
	} else if ctx.comboCount >= 1 {
		score += 1
	}

	if ctx.avgCMC < 2.0 {
		score += 2
	} else if ctx.avgCMC < 2.5 {
		score += 1
	} else if ctx.avgCMC > 3.5 {
		score -= 1
	}

	if ctx.fastManaCount >= 10 {
		score += 3
	} else if ctx.fastManaCount >= 6 {
		score += 2
	} else if ctx.fastManaCount >= 3 {
		score += 1
	}

	if ctx.roleRatios[RoleCounterspell] >= 0.06 {
		score += 1
	}

	if report.Roles != nil {
		landRatio := ctx.roleRatios[RoleLand]
		if landRatio < 0.30 {
			score += 1
		}
	}

	var bracket int
	var label string
	switch {
	case score >= 10:
		bracket = 5
		label = "cEDH"
	case score >= 7:
		bracket = 4
		label = "High Power"
	case score >= 4:
		bracket = 3
		label = "Focused"
	case score >= 2:
		bracket = 2
		label = "Casual+"
	default:
		bracket = 1
		label = "Casual"
	}

	return bracket, label
}
