package main

import (
	"fmt"
	"strings"
)

type RoleTag string

const (
	RoleRamp         RoleTag = "Ramp"
	RoleDraw         RoleTag = "Draw"
	RoleRemoval      RoleTag = "Removal"
	RoleBoardWipe    RoleTag = "BoardWipe"
	RoleCounterspell RoleTag = "Counterspell"
	RoleTutor        RoleTag = "Tutor"
	RoleThreat       RoleTag = "Threat"
	RoleCombo        RoleTag = "Combo"
	RoleProtection   RoleTag = "Protection"
	RoleStax         RoleTag = "Stax"
	RoleUtility      RoleTag = "Utility"
	RoleLand         RoleTag = "Land"
)

var AllRoles = []RoleTag{
	RoleRamp, RoleDraw, RoleRemoval, RoleBoardWipe, RoleCounterspell,
	RoleTutor, RoleThreat, RoleCombo, RoleProtection, RoleStax,
	RoleUtility, RoleLand,
}

type CardRoleAssignment struct {
	Name  string
	Roles []RoleTag
}

type RoleAnalysis struct {
	Assignments []CardRoleAssignment
	RoleCounts  map[RoleTag]int
	TotalCards  int
	Warnings    []string
}

type archetypeTemplate struct {
	Name       string
	MinRatios  map[RoleTag]float64
	MaxRatios  map[RoleTag]float64
}

var defaultTemplate = archetypeTemplate{
	Name: "Generic EDH",
	MinRatios: map[RoleTag]float64{
		RoleRamp:      0.08,
		RoleDraw:      0.08,
		RoleRemoval:   0.05,
		RoleBoardWipe: 0.02,
		RoleLand:      0.33,
	},
	MaxRatios: map[RoleTag]float64{
		RoleLand: 0.45,
	},
}

func TagCardRole(name, oracleText, typeLine, manaCost string, cmc int, profile CardProfile) []RoleTag {
	var roles []RoleTag
	ot := strings.ToLower(oracleText)
	tl := strings.ToLower(typeLine)

	if profile.IsLand {
		roles = append(roles, RoleLand)
	}

	if isRamp(profile, ot, tl) {
		roles = append(roles, RoleRamp)
	}

	if isDraw(profile, ot) {
		roles = append(roles, RoleDraw)
	}

	isBounceAll := isMassBounce(ot)
	isBounceTarget := isTargetBounce(ot)

	if profile.IsMassWipe || isBounceAll {
		roles = append(roles, RoleBoardWipe)
	}
	if (profile.IsRemoval || isBounceTarget) && !profile.IsMassWipe && !isBounceAll {
		roles = append(roles, RoleRemoval)
	}

	if isCounterspell(ot, tl) {
		roles = append(roles, RoleCounterspell)
	}

	if profile.IsTutor {
		roles = append(roles, RoleTutor)
	}

	if isThreat(profile, ot, tl, cmc) {
		roles = append(roles, RoleThreat)
	}

	if isCombo(profile) {
		roles = append(roles, RoleCombo)
	}

	if isProtection(ot, tl) {
		roles = append(roles, RoleProtection)
	}

	if isStax(profile, ot) {
		roles = append(roles, RoleStax)
	}

	if len(roles) == 0 && !profile.IsLand {
		roles = append(roles, RoleUtility)
	}

	return roles
}

func isTargetBounce(ot string) bool {
	return containsAny(ot,
		"return target creature",
		"return target nonland permanent",
		"return target permanent",
		"return target artifact",
		"return target enchantment",
		"put target creature on top",
		"put target nonland permanent on top") &&
		!strings.Contains(ot, "return target creature card from your graveyard")
}

func isMassBounce(ot string) bool {
	return containsAny(ot,
		"return all creatures",
		"return all nonland permanents",
		"return all permanents",
		"return each nonland permanent",
		"return each creature",
		"return each permanent") ||
		(strings.Contains(ot, "overload") && isTargetBounce(ot))
}

func isRamp(p CardProfile, ot, tl string) bool {
	if p.IsLand {
		return false
	}
	for _, r := range p.Produces {
		if r == ResMana {
			return true
		}
	}
	for _, e := range p.Effects {
		if e == "land_fetch" {
			return true
		}
	}
	return false
}

func isDraw(p CardProfile, ot string) bool {
	if p.IsLand {
		return false
	}
	for _, r := range p.Produces {
		if r == ResCard {
			return true
		}
	}
	return false
}

func isCounterspell(ot, tl string) bool {
	if strings.Contains(ot, "counter target spell") ||
		strings.Contains(ot, "counter target activated") ||
		strings.Contains(ot, "counter target triggered") ||
		strings.Contains(ot, "counter that spell") {
		return true
	}
	if strings.Contains(ot, "counter it") &&
		!strings.Contains(ot, "ward") &&
		!strings.Contains(ot, "unless that player pays") {
		return true
	}
	if strings.Contains(tl, "instant") &&
		strings.Contains(ot, "counter target") {
		return true
	}
	return false
}

func isThreat(p CardProfile, ot, tl string, cmc int) bool {
	if p.IsLand {
		return false
	}
	if p.IsWinCon || p.IsManaPayoff {
		return true
	}
	if p.HasETBDamage || p.HasDeathDrain || p.MakesInfiniteTokens {
		return true
	}
	isCreature := strings.Contains(tl, "creature")
	isPW := strings.Contains(tl, "planeswalker")
	if isCreature && cmc >= 4 && !p.IsTutor && !p.IsRemoval && !p.IsMassWipe {
		if containsAny(ot, "trample", "flying", "double strike", "menace",
			"commander damage", "annihilator", "infect",
			"deals combat damage to a player",
			"whenever this creature attacks",
			"whenever this creature deals combat damage") {
			return true
		}
		if cmc >= 6 {
			return true
		}
	}
	if isPW {
		return true
	}
	if strings.Contains(ot, "each opponent loses") && !strings.Contains(ot, "whenever") {
		return true
	}
	return false
}

func isCombo(p CardProfile) bool {
	if p.WinsWithEmptyLib || p.EmptiesLibrary || p.UntapsAll {
		return true
	}
	if p.LifegainToDrain || p.LifelossToPump {
		return true
	}
	if p.IsOutlet && len(p.Triggers) > 0 {
		return true
	}
	hasRepeatable := false
	for _, t := range p.Triggers {
		if t == "etb" || t == "dies" || t == "sacrifice" || t == "cast" || t == "lifegain" || t == "lifeloss" {
			hasRepeatable = true
			break
		}
	}
	if hasRepeatable && (len(p.Produces) >= 2 || p.MandatoryTriggers) {
		return true
	}
	return false
}

func isProtection(ot, tl string) bool {
	if containsAny(ot,
		"hexproof", "shroud", "indestructible",
		"ward", "protection from",
		"can't be the target",
		"can't be countered",
		"can't be destroyed") {
		return true
	}
	if containsAny(ot,
		"your permanents have hexproof",
		"you have hexproof",
		"creatures you control have hexproof",
		"creatures you control have indestructible",
		"creatures you control gain indestructible",
		"creatures you control gain hexproof") {
		return true
	}
	if strings.Contains(ot, "phase out") || strings.Contains(ot, "phases out") {
		return true
	}
	return false
}

func isStax(p CardProfile, ot string) bool {
	if p.IsLand {
		return false
	}
	for _, e := range p.Effects {
		if e == "tax" || e == "lock" || e == "symmetric_pain" {
			return true
		}
	}
	if containsAny(ot,
		"can't untap", "don't untap",
		"skip", "can't draw",
		"can't search", "can't cast noncreature",
		"each player can't",
		"players can't") {
		return true
	}
	return false
}

func ComputeRoleAnalysis(qtyProfiles []CardProfileQty, oracle *oracleDB) *RoleAnalysis {
	ra := &RoleAnalysis{
		RoleCounts: make(map[RoleTag]int),
	}
	for _, role := range AllRoles {
		ra.RoleCounts[role] = 0
	}

	for _, qp := range qtyProfiles {
		var oracleText string
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				oracleText = entry.OracleText
				if oracleText == "" && len(entry.CardFaces) > 0 {
					oracleText = entry.CardFaces[0].OracleText
				}
			}
		}

		roles := TagCardRole(
			qp.Profile.Name,
			oracleText,
			qp.Profile.TypeLine,
			qp.Profile.ManaCost,
			qp.Profile.CMC,
			qp.Profile,
		)

		ra.Assignments = append(ra.Assignments, CardRoleAssignment{
			Name:  qp.Profile.Name,
			Roles: roles,
		})

		for _, role := range roles {
			ra.RoleCounts[role] += qp.Qty
		}
	}

	for _, qp := range qtyProfiles {
		ra.TotalCards += qp.Qty
	}
	computeRoleWarnings(ra, qtyProfiles)
	return ra
}

func computeRoleWarnings(ra *RoleAnalysis, qtyProfiles []CardProfileQty) {
	totalCards := 0
	for _, qp := range qtyProfiles {
		totalCards += qp.Qty
	}
	if totalCards == 0 {
		return
	}

	tmpl := defaultTemplate
	for role, minRatio := range tmpl.MinRatios {
		actual := float64(ra.RoleCounts[role]) / float64(totalCards)
		if actual < minRatio-0.005 {
			ra.Warnings = append(ra.Warnings,
				fmt.Sprintf("%s: %.0f%% of deck (%d cards) — %s template recommends at least %.0f%%",
					role, actual*100, ra.RoleCounts[role], tmpl.Name, minRatio*100))
		}
	}
	for role, maxRatio := range tmpl.MaxRatios {
		actual := float64(ra.RoleCounts[role]) / float64(totalCards)
		if actual > maxRatio+0.005 {
			ra.Warnings = append(ra.Warnings,
				fmt.Sprintf("%s: %.0f%% of deck (%d cards) — %s template recommends at most %.0f%%",
					role, actual*100, ra.RoleCounts[role], tmpl.Name, maxRatio*100))
		}
	}
}
