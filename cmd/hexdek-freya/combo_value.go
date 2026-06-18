package main

import (
	"fmt"
	"strings"
)

// Phase 2 — VALUE axis = generic, deck-independent advantage.
//
// VALUE types a card's (or interaction's) baseline advantage:
//   - CARD  — draw / tutor / recursion (refills the hand)
//   - MANA  — ramp / rituals / mana rocks / land tutors (more mana)
//   - BOARD — tokens / removal / one-sided wipes (board swing in your favor)
//
// SYMMETRY discriminator (7174n1c's sacrifice example): an ASYMMETRICAL effect
// (one-sided sacrifice, one-sided board wipe, you-benefit) is VALUE; a
// SYMMETRICAL effect (each player sacrifices, group hug) is NOT value here — it
// routes to SYNERGY in Phase 3. A board wipe is treated as ASYMMETRICAL: you
// cast it from ahead, so it's a one-sided swing.
//
// Output lands in FreyaReport.ValueEngines (additive; COMBO entries from
// Phase 1 are excluded so the axes don't double-list — VALUE and SYNERGY will
// co-tag in Phase 3, but COMBO stays its own axis). Deck-independent: this
// pass never reads the commander.

// detectSymmetricEffect reports whether oracle text describes a table-wide
// effect that hits every player equally ("each player ...", "all players ...").
// Operates on reminder-stripped, lowercased text. Deliberately narrow: it does
// NOT fire on "each opponent" (one-sided, good for you) or "destroy all
// creatures" (a board wipe you cast from ahead) — both stay asymmetric VALUE.
func detectSymmetricEffect(ot string) bool {
	return containsAny(ot,
		"each player", "all players", "every player", "each player's",
		"each of you", "players sacrifice", "players draw", "players discard")
}

// buildValueProfile types a card's deck-independent advantage baseline and its
// symmetry. Advantage is derived from the existing oracle-classified flags /
// resource model — no new oracle parsing.
func buildValueProfile(p CardProfile) ValueProfile {
	var adv []string

	// CARD advantage — draw, tutor, recursion all refill the hand / restock.
	if containsRes(p.Produces, ResCard) || p.IsTutor || p.IsRecursion ||
		containsRes(p.Produces, ResGraveyard) || containsRes(p.Produces, ResReanimate) {
		adv = append(adv, "card")
	}
	// MANA advantage — direct mana production or land ramp.
	if containsRes(p.Produces, ResMana) || p.IsLandTutor {
		adv = append(adv, "mana")
	}
	// BOARD advantage — tokens, single-target removal, mass removal.
	if containsRes(p.Produces, ResToken) || p.IsRemoval || p.IsMassWipe {
		adv = append(adv, "board")
	}

	sym := "n/a"
	if len(adv) > 0 {
		if p.Symmetric {
			sym = "symmetric"
		} else {
			sym = "asymmetric"
		}
	}

	tl := strings.ToLower(p.TypeLine)
	oneShot := strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery")

	return ValueProfile{Advantage: adv, Symmetry: sym, OneShot: oneShot}
}

// isValue reports whether a ValueProfile qualifies as a VALUE baseline:
// non-empty advantage that is one-sided (asymmetric). Symmetric table-wide
// effects are excluded (they route to SYNERGY in Phase 3).
func (vp ValueProfile) isValue() bool {
	return len(vp.Advantage) > 0 && vp.Symmetry == "asymmetric"
}

// populateValueEngines fills report.ValueEngines with the deck's VALUE
// baseline: every card whose ValueProfile is asymmetric advantage, plus any
// multi-card interaction (from the legacy detectors) that provides value.
// Entries whose exact card set is already tagged COMBO are skipped — COMBO is
// its own axis. comboKeys is the set of canonical card keys already in
// report.Combos. Idempotent; called from CategorizeReportCombos.
func populateValueEngines(report *FreyaReport, comboKeys map[string]bool, profByName map[string]CardProfile) {
	report.ValueEngines = nil

	seen := map[string]bool{}
	add := func(c ComboResult) {
		key := comboCardKey(c.Cards)
		if comboKeys[key] || seen[key] {
			return
		}
		seen[key] = true
		c.Categories = []string{CategoryValue}
		report.ValueEngines = append(report.ValueEngines, c)
	}

	// 1. Per-card value baseline.
	for _, p := range report.Profiles {
		vp := buildValueProfile(p)
		if !vp.isValue() {
			continue
		}
		add(ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "value",
			Resources:   strings.Join(vp.Advantage, "+"),
			Description: fmt.Sprintf("%s — %s advantage", p.Name, strings.Join(vp.Advantage, "/")),
		})
	}

	// 2. Multi-card value interactions surfaced by the legacy detectors (the
	//    downgraded reciprocal pairs, graveyard / land value synergies, etc.).
	//    A 2+ card interaction is VALUE when at least one piece carries
	//    asymmetric advantage. Single-card bucket entries are already covered
	//    by the per-card pass above.
	buckets := [][]ComboResult{
		report.Determined, report.Synergies,
		report.LandCycleSynergies, report.GraveyardLoops,
	}
	for _, bucket := range buckets {
		for _, c := range bucket {
			if len(c.Cards) < 2 {
				continue
			}
			if !interactionHasValue(c.Cards, profByName) {
				continue
			}
			vc := c
			vc.Annotation = nil
			add(vc)
		}
	}
}

// interactionHasValue reports whether any card in the interaction contributes
// asymmetric advantage (the interaction nets value for you).
func interactionHasValue(names []string, profByName map[string]CardProfile) bool {
	for _, n := range names {
		if p, ok := profByName[n]; ok && buildValueProfile(p).isValue() {
			return true
		}
	}
	return false
}
