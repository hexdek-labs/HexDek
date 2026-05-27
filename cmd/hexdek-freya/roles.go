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
	// RolePseudoTutor — "soft" library/combo-piece access that consistently
	// surfaces a specific card via a mechanic other than search-library /
	// reveal-until. Cascade (Bloodbraid Elf), Transmute (Dimir House
	// Guard — also IsTutor via its "search your library" clause, so it
	// double-tags), Companion (Yorion, Lurrus — wish-style access from
	// outside the game), Unearth (Hellspark Elemental — graveyard-as-
	// hand reanimation). These aren't hard tutors but functionally feed
	// combo lines, so they deserve a separate tag from RoleTutor.
	RolePseudoTutor  RoleTag = "PseudoTutor"
	// RoleSuspendFinisher — cards whose entire identity is "suspend N,
	// then resolve a board-defining game-swing effect". Living End,
	// Restore Balance, Hypergenesis (suspend-only printing variants),
	// Wildfire / Burning of Xinye-style suspend sweepers. Distinct from
	// the pseudo-tutor flag (which the suspend card ALSO gets, since
	// the suspend timer functionally tutors the future cast), because
	// the strategic identity of a suspend-finisher is "the deck is
	// built around this resolving" — archetype classifiers and the buy-
	// guide want a separate signal from "Lotus Bloom = pseudo-tutor for
	// 3 mana on turn 4".
	RoleSuspendFinisher RoleTag = "SuspendFinisher"
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
	RoleTutor, RolePseudoTutor, RoleSuspendFinisher, RoleThreat, RoleCombo, RoleProtection, RoleStax,
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
	// SuspendFinisher renders above Threat / BoardWipe because a Living
	// End / Restore Balance card IS the deck's wincon — the suspend-
	// finisher framing communicates strategic identity better than the
	// "boardwipe" or "threat" labels would on their own.
	RoleSuspendFinisher: 2,
	RoleThreat:       3,
	RoleBoardWipe:    4,
	RoleCounterspell: 5,
	RoleTutor:        6,
	// PseudoTutor sits just below Tutor in the priority order — when a
	// card has both (transmute, modal "search library OR cascade"), Tutor
	// is the stronger characterization and should render first. Pseudo-
	// tutor still outranks Removal because the consistency engine angle
	// is the strategically important thing to surface.
	RolePseudoTutor:  7,
	RoleRemoval:      8,
	// Recursion sits between Removal and Protection — it's an
	// engine/value slot that often pairs with Threat (Sun Titan,
	// Karmic Guide, Reveillark) or stands alone (Eternal Witness,
	// Reanimate). Renders before Protection / Draw / Ramp so
	// multi-role recursion-creatures lead with the role that
	// explains their strategic purpose.
	RoleRecursion:    9,
	RoleProtection:   10,
	RoleDraw:         11,
	RoleRamp:         12,
	RoleUtility:      13,
	RoleLand:         14,
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

	if isPseudoTutor(oracleText) {
		roles = append(roles, RolePseudoTutor)
	}

	if isSuspendFinisher(oracleText) {
		roles = append(roles, RoleSuspendFinisher)
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

// isPseudoTutor returns true when the card carries one of the "soft" tutor
// mechanics — keyword machinery that consistently surfaces a specific card
// from library / graveyard / outside-the-game without being a literal
// "search your library" / "reveal until" tutor.
//
// Covered mechanics:
//   - Cascade (Bloodbraid Elf, Maelstrom Wanderer): exile cards from the
//     top of your library until you exile a nonland card that costs less,
//     then cast it for free. Functionally a tutor for cheaper combo
//     pieces (Bloodbraid → Living End, Maelstrom Wanderer → eot
//     stack-the-deck combos).
//   - Transmute (Dimir House Guard, Drift of Phantasms, Muddle the
//     Mixture): {cost}, Discard this card: Search library for a card
//     with the same mana value. Note transmute ALSO trips the canonical
//     "search your library" detector in classifyTutorInto, so these
//     cards double-tag (Tutor + PseudoTutor) — that's deliberate. The
//     hard-tutor flag captures the consistency engine angle; the
//     pseudo-tutor flag captures the mechanic family for downstream
//     consumers (combo-piece-density analysis, archetype fingerprint).
//   - Companion (Yorion, Lurrus, Jegantha): the companion mechanic lets
//     you pay {3} once per game to put the companion from outside the
//     game into your hand — a guaranteed wish-style tutor for that
//     specific card.
//   - Unearth (Hellspark Elemental, Anathemancer, Corpse Connoisseur):
//     {cost}: Return from graveyard with haste, exile at EOT. Treats
//     graveyard as an extension of hand for combo-piece access; loops
//     with Sun Titan / Reanimate / Loam-style recursion.
//   - Suspend (Lotus Bloom, Search for Tomorrow, Ancestral Vision,
//     Wheel of Fate): "Suspend N—{cost}" lets you exile the card from
//     hand for a reduced cost and cast it for free after N upkeeps.
//     Functionally tutors a future cast of the SAME card on a known
//     turn — Lotus Bloom on turn 1 is "tutored" mana on turn 4,
//     Search for Tomorrow on turn 1 is a "tutored" land drop on turn 3.
//     The Living End / Restore Balance family ALSO trips
//     isSuspendFinisher (RoleSuspendFinisher); the pseudo-tutor flag
//     here captures the "the suspend timer is the consistency engine"
//     angle separately.
//
// Detection runs against the raw (case-insensitive, reminder-stripped)
// oracle text on a per-line basis. The granted-keyword case (Yidris
// "...that spell has cascade", Maraxus-style "Equipped creature has
// unearth {cost}") is rejected by checking the line for a leading "has"
// / "have" / "with" / "gain" / "gains" framing before the keyword
// anchor, mirroring the hasSelfCyclingKeyword pattern in analysis.go.
func isPseudoTutor(rawOracleText string) bool {
	if rawOracleText == "" {
		return false
	}
	// NOTE: cannot use CleanForScan here — it collapses newlines to spaces,
	// which would defeat the per-line "line starts with the keyword" anchor
	// that distinguishes self-declared keywords from granted ones. Lowercase
	// directly; the keyword anchors all sit BEFORE any reminder text, so
	// HasPrefix matching is unaffected by the unstripped parens.
	clean := strings.ToLower(rawOracleText)
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Reject granted-keyword framing. "spell has cascade", "creatures
		// you control have unearth {1}{B}", "with cascade" — line is
		// granting the mechanic to OTHER cards, not declaring it on self.
		if containsAny(line,
			"has cascade", "have cascade", "with cascade", "gain cascade", "gains cascade",
			"has transmute", "have transmute", "with transmute",
			"has unearth", "have unearth", "with unearth", "gain unearth", "gains unearth",
			"has suspend", "have suspend", "with suspend", "gain suspend", "gains suspend") {
			continue
		}
		// Cascade — bare keyword, no cost ("cascade" or "cascade, cascade"
		// for Maelstrom Wanderer's double-cascade). Anchor at line start
		// so prose mentions ("...that spell has cascade") don't false-fire
		// even when the grant-prefix filter above doesn't catch them.
		if strings.HasPrefix(line, "cascade") {
			return true
		}
		// Transmute keyword: "transmute {cost}". The keyword always
		// includes a mana cost.
		if strings.HasPrefix(line, "transmute {") {
			return true
		}
		// Unearth keyword: "unearth {cost}".
		if strings.HasPrefix(line, "unearth {") {
			return true
		}
		// Companion clause: "companion — <condition>". Scryfall uses a
		// Unicode em-dash; the cleaned text preserves it. The companion
		// mechanic itself is the wish-tutor for the specific card.
		if strings.HasPrefix(line, "companion —") || strings.HasPrefix(line, "companion of ") {
			return true
		}
		// Suspend keyword: "suspend N—{cost}". Scryfall uses an em-dash
		// between the count and the cost; the cost always starts with
		// '{'. Anchoring at "suspend " plus the digit + dash pattern
		// rejects prose mentions ("...exile it as if it had suspend").
		if strings.HasPrefix(line, "suspend ") && strings.Contains(line, "—{") {
			return true
		}
	}
	return false
}

// isSuspendFinisher reports whether the card pairs the Suspend keyword with
// a game-swinging body — the Living End / Restore Balance / Hypergenesis
// archetype, where the deck's entire identity is "suspend this on turn 1,
// untap into a free wincon on turn N". Distinct from RolePseudoTutor
// (which the same card ALSO gets, since the suspend timer is itself a
// tutor for the future cast), because the finisher framing is what the
// archetype classifier and buy-guide need to surface as the deck's
// strategic anchor.
//
// Criteria: oracle has the suspend keyword AND the body includes one of
// the canonical "mass effect" markers:
//
//   - "all creature cards from all graveyards" (Living End mass reanimate)
//   - "exile all creatures" / "destroy all creatures" / "destroy all
//     permanents" / "exile all permanents" (mass removal)
//   - "sacrifices the rest" (Restore Balance pattern — equalizing sac)
//   - "each player sacrifices" + creatures/lands (Wildfire-style)
//   - "puts all creature cards from their graveyard onto the battlefield"
//     (Living End secondary phrasing)
//   - "deals X damage to each" (mass burn / sweep — Hypergenesis-adjacent
//     suspend wincons like Lava Burst variants would trip this)
//
// Vanilla suspend value cards (Lotus Bloom — adds mana; Search for
// Tomorrow — fetches a single land; Wheel of Fate — discard+draw 7;
// Ancestral Vision — draw 3) do NOT match because their body is a
// targeted value effect, not a board-defining swing.
func isSuspendFinisher(rawOracleText string) bool {
	if rawOracleText == "" {
		return false
	}
	clean := strings.ToLower(rawOracleText)
	hasSuspend := false
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		// Skip granted-suspend prose so an artifact granting suspend to
		// other cards doesn't auto-promote a non-finisher body.
		if containsAny(line,
			"has suspend", "have suspend", "with suspend", "gain suspend", "gains suspend") {
			continue
		}
		if strings.HasPrefix(line, "suspend ") && strings.Contains(line, "—{") {
			hasSuspend = true
			break
		}
	}
	if !hasSuspend {
		return false
	}
	// Body-side mass-effect markers. Tested against the full lowercased
	// text (not per-line) because the relevant clause can span lines or
	// follow the suspend declaration on a separate paragraph.
	if containsAny(clean,
		"all creature cards from all graveyards",
		"all creature cards from their graveyard",
		"exile all creatures",
		"exile all permanents",
		"exile all nonland",
		"destroy all creatures",
		"destroy all permanents",
		"destroy all nonland",
		"sacrifices the rest",
		"deals damage to each creature",
		"deals damage to each player",
		"deals damage to each opponent") {
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
