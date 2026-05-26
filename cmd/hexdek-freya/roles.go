package main

import (
	"fmt"
	"sort"
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
	RoleRecursion    RoleTag = "Recursion"
	RoleUtility      RoleTag = "Utility"
	RoleLand         RoleTag = "Land"
)

var AllRoles = []RoleTag{
	RoleRamp, RoleDraw, RoleRemoval, RoleBoardWipe, RoleCounterspell,
	RoleTutor, RoleThreat, RoleCombo, RoleProtection, RoleStax,
	RoleRecursion, RoleUtility, RoleLand,
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

var rolePriority = map[RoleTag]int{
	RoleCombo:        0,
	RoleStax:         1,
	RoleThreat:       2,
	RoleBoardWipe:    3,
	RoleCounterspell: 4,
	RoleTutor:        5,
	RoleRemoval:      6,
	// Recursion sits between Removal and Protection — it's an
	// engine/value slot that often pairs with Threat (Sun Titan,
	// Karmic Guide, Reveillark) or stands alone (Eternal Witness,
	// Reanimate). Renders before Protection / Draw / Ramp so
	// multi-role recursion-creatures lead with the role that
	// explains their strategic purpose.
	RoleRecursion:    7,
	RoleProtection:   8,
	RoleDraw:         9,
	RoleRamp:         10,
	RoleUtility:      11,
	RoleLand:         12,
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

	if isRecursion(profile, ot, tl) {
		roles = append(roles, RoleRecursion)
	}

	if len(roles) == 0 && !profile.IsLand {
		roles = append(roles, RoleUtility)
	}

	sort.Slice(roles, func(i, j int) bool {
		return rolePriority[roles[i]] < rolePriority[roles[j]]
	})

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
		// Combat-keyword / attack-trigger gate. Sun Titan hits this
		// arm via "whenever this creature attacks" — that's correct
		// because Sun Titan IS a clock once it's down, AND a recursion
		// engine; multi-role tagging (Threat + Recursion) handles it.
		if containsAny(ot, "trample", "flying", "double strike", "menace",
			"commander damage", "annihilator", "infect",
			"deals combat damage to a player",
			"whenever this creature attacks",
			"whenever this creature deals combat damage") {
			return true
		}
		// cmc>=6 catch-all. Pre-r60 this swept every mid-CMC creature
		// into Threat, mis-tagging pure recursion bodies that have no
		// combat keywords (Karmic Guide as a 2/2 protection-from-black
		// reanimator, mid-CMC ETB-recursion bodies without flying).
		// Exempt cmc∈[6,7] recursion-engine creatures so they get the
		// Recursion-only tag and don't pollute Threat-ratio counts in
		// archetype.go's fingerprint match. cmc≥8 still falls through
		// to Threat because by that point the creature IS a finisher
		// regardless of secondary text (Worldspine Wurm, Emrakul, etc).
		if cmc >= 6 {
			if p.IsRecursion && cmc <= 7 {
				// Recursion piece in the 6-7 CMC band — let isRecursion
				// own the role. Combat-keyword cards above already
				// returned true on their own merits.
				return false
			}
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

// isRecursion reports whether a card should carry the Recursion role
// tag. Reads the pre-existing p.IsRecursion flag (populated by
// analysis.go from the "return ... from ... graveyard ..." oracle
// pattern) and adds a few oracle-text patterns that the analysis-
// layer detector misses:
//
//   - "exile target creature card from a graveyard" + "put it onto the
//     battlefield" (Reanimate variants that route through exile, e.g.
//     Bringer of the Last Gift, Necromancy at one moment in its life)
//   - "you may cast that card from your graveyard" (Past in Flames,
//     Snapcaster Mage's effect — though Snapcaster also fires the
//     analysis-layer detector via "gains flashback")
//   - "from your graveyard to the battlefield" (catches a couple of
//     activations that don't use the "return" verb, e.g. Yawgmoth's
//     Will, Bone Shards-class effects)
//
// Lands are excluded because Crucible of Worlds / Ramunap Excavator
// land-recursion is a different value pattern (it's ramp-coded, not
// engine-coded — handled by the Ramp tag already).
func isRecursion(p CardProfile, ot, tl string) bool {
	if p.IsLand {
		return false
	}
	if p.IsRecursion {
		return true
	}
	// Exile-and-reanimate routes (Bringer of the Last Gift, certain
	// reanimator-via-exile spells) skip the "return from graveyard"
	// pattern the analysis layer keys on.
	if strings.Contains(ot, "exile target creature card from") &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "put it onto the battlefield") ||
			strings.Contains(ot, "create a token that's a copy")) {
		return true
	}
	// "Cast from your graveyard" payoffs (Past in Flames,
	// Yawgmoth's Will, Underworld Breach).
	if strings.Contains(ot, "may cast") && strings.Contains(ot, "from your graveyard") {
		return true
	}
	// Catch-all for activations that put a card from graveyard onto
	// the battlefield without using the "return" verb.
	if strings.Contains(ot, "from your graveyard to the battlefield") ||
		strings.Contains(ot, "from a graveyard to the battlefield") {
		return true
	}
	// Grant-flashback effects (Past in Flames, Snapcaster Mage's
	// effect, Iroh, Lier Disciple of the Drowned). The grant turns
	// every i/s in graveyard into a castable recursion target —
	// strictly graveyard-value play.
	if containsAny(ot,
		"gains flashback", "gain flashback",
		"has flashback", "have flashback") {
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
