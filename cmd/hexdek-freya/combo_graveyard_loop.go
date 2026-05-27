package main

import (
	"fmt"
	"sort"
	"strings"
)

// graveyardRecursionEnabler is a load-bearing card whose presence in
// the deck transforms a 2-card combo that buries a piece into a
// multi-turn loop. The Recurs predicate returns true when the enabler
// can plausibly rebuy the given piece — Sun Titan rebuys nonland
// permanent CMC ≤ 3, Muldrotha rebuys any permanent type once per
// turn from grave, Karador / Meren / Sheoldred / Reya rebuy creatures.
// The enabler list is intentionally narrow — commander-tier engines
// that the rest of the deck builds around, not generic reanimate
// spells (which are tracked as RoleRecursion at the role-tagging
// layer, not as combo-class promoters).
type graveyardRecursionEnabler struct {
	Name   string
	Recurs func(p CardProfile) bool
}

// graveyardRecursionEnablers is the curated table. Order is not
// significant — detection walks all enablers present in the deck
// and emits one synthesized loop per (combo × enabler) pair. The
// recurrence predicates intentionally accept the SAME piece via
// multiple enablers; the dedup pass in detectGraveyardLoopCombos
// collapses the (cardsKey, enabler) tuples so a deck with both
// Karador and Muldrotha doesn't double-count the Karmic Guide +
// Reveillark loop.
var graveyardRecursionEnablers = []graveyardRecursionEnabler{
	{
		Name: "Sun Titan",
		// Sun Titan attack trigger: "return target permanent card with
		// mana value 3 or less from your graveyard to the battlefield."
		// Restricted to nonland permanents (lands are returned but rarely
		// the combo piece).
		Recurs: func(p CardProfile) bool {
			return p.CMC <= 3 && isCombatRecurablePermanent(p)
		},
	},
	{
		Name: "Karmic Guide",
		// "When Karmic Guide enters the battlefield, return target
		// creature card from your graveyard to the battlefield." Single-
		// shot but reliable creature-only recursion; pairs with itself in
		// the Karmic + Reveillark loop, so we DO list it as an enabler
		// even though it's also a combo piece.
		Recurs: func(p CardProfile) bool {
			return strings.Contains(strings.ToLower(p.TypeLine), "creature")
		},
	},
	{
		Name: "Karador, Ghost Chieftain",
		// "During each of your turns, you may cast one creature card
		// from your graveyard." Creature-only, any CMC.
		Recurs: func(p CardProfile) bool {
			return strings.Contains(strings.ToLower(p.TypeLine), "creature")
		},
	},
	{
		Name: "Muldrotha, the Gravetide",
		// "During each of your turns, you may play a land and cast a
		// permanent spell of each permanent type from your graveyard."
		// Any permanent type, any CMC.
		Recurs: func(p CardProfile) bool {
			return isPermanentType(p.TypeLine)
		},
	},
	{
		Name: "Meren of Clan Nel Toth",
		// EOT XP-counter creature recursion. Any creature CMC ≤ XP, but
		// XP scales over time so we treat the CMC ceiling as soft.
		Recurs: func(p CardProfile) bool {
			return strings.Contains(strings.ToLower(p.TypeLine), "creature")
		},
	},
	{
		Name: "Sheoldred, Whispering One",
		// Upkeep mandatory creature return. Any CMC.
		Recurs: func(p CardProfile) bool {
			return strings.Contains(strings.ToLower(p.TypeLine), "creature")
		},
	},
	{
		Name: "Reya Dawnbringer",
		// Upkeep creature return, any CMC. Slower than Sheoldred (only
		// your upkeep, and Reya herself is CMC 9) but a real enabler in
		// decks built around it.
		Recurs: func(p CardProfile) bool {
			return strings.Contains(strings.ToLower(p.TypeLine), "creature")
		},
	},
}

// isCombatRecurablePermanent returns true for permanent-type cards
// Sun Titan can return from graveyard at attack: nonland permanents
// (Sun Titan's printed wording explicitly says "permanent card",
// including lands, but lands are rarely the combo piece we care
// about and including them produces a flood of false positives on
// fetch-land + Crucible-style "combos" the detector picks up).
func isCombatRecurablePermanent(p CardProfile) bool {
	tl := strings.ToLower(p.TypeLine)
	if strings.Contains(tl, "land") {
		return false
	}
	return isPermanentType(tl)
}

// isPermanentType returns true when the type line contains any of the
// six permanent supertypes. Instants and sorceries are explicitly NOT
// permanent types and can never be rebought by Muldrotha-style
// recursion (Muldrotha says "permanent spell," which excludes them).
func isPermanentType(typeLine string) bool {
	tl := strings.ToLower(typeLine)
	for _, perm := range []string{"creature", "artifact", "enchantment", "planeswalker", "battle", "land"} {
		if strings.Contains(tl, perm) {
			return true
		}
	}
	return false
}

// isBuriableInCombo returns true when the piece would naturally hit
// the graveyard during the combo's play sequence — either because it's
// a creature (dies to removal or sacrifice mid-loop), because it's an
// Aura that falls off when the enchanted creature dies (Animate Dead),
// or because the combo class itself is one that routes through a
// sacrifice outlet (infinite_etb, infinite_drain) where the loop piece
// repeatedly dies. The signal is intentionally permissive — false
// positives surface as "you have backup via your enabler" suggestions,
// which is correct framing even when the combo doesn't strictly require
// burial.
func isBuriableInCombo(p CardProfile, comboClass string) bool {
	tl := strings.ToLower(p.TypeLine)
	if strings.Contains(tl, "creature") {
		return true
	}
	if strings.Contains(tl, "aura") || strings.Contains(tl, "enchantment - aura") {
		return true
	}
	// Sacrifice-cost outlet pieces self-bury when they activate.
	if p.IsOutlet {
		return true
	}
	// Combos where the LOOP shape involves death triggers — pieces here
	// naturally hit the graveyard even when not creatures (e.g. Goblin
	// Bombardment as a sac outlet in an ETB loop).
	switch comboClass {
	case ComboClassInfiniteETB, ComboClassInfiniteDrain, ComboClassInfiniteDamage,
		ComboClassInfiniteTokens, ComboClassBlinkEngine:
		return true
	}
	return false
}

// detectGraveyardLoopCombos walks TrueInfinites + Determined for
// 2-card combos and emits a ComboResult per (combo × enabler) where
// the deck contains the enabler AND the enabler can recur at least
// one piece of the combo AND that piece would naturally hit the
// graveyard during the combo's play. Returns nil when no enabler is
// in the deck (the dominant case — non-graveyard decks shouldn't pay
// the per-combo scan cost), making the function safe to call
// unconditionally from FindCombos.
//
// Each output entry is a SYNTHETIC finisher annotation, not a
// duplicate of the underlying combo — the Cards list is [combo piece
// A, combo piece B, enabler]; LoopType is "synergy" (it's a loop made
// possible by the enabler, not a true infinite); Class is
// ComboClassGraveyardLoop so the bracket scorer can recognize the
// B3-tier marker without re-running the detection.
func detectGraveyardLoopCombos(profiles []CardProfile, combos []ComboResult) []ComboResult {
	if len(profiles) == 0 || len(combos) == 0 {
		return nil
	}
	profileByName := make(map[string]CardProfile, len(profiles))
	for _, p := range profiles {
		profileByName[p.Name] = p
	}

	// Filter enablers to those actually in the deck.
	type liveEnabler struct {
		name   string
		recurs func(p CardProfile) bool
	}
	var live []liveEnabler
	for _, e := range graveyardRecursionEnablers {
		if _, ok := profileByName[e.Name]; ok {
			live = append(live, liveEnabler{name: e.Name, recurs: e.Recurs})
		}
	}
	if len(live) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var out []ComboResult
	for _, c := range combos {
		if len(c.Cards) != 2 {
			continue
		}
		// Skip combos where the enabler IS one of the pieces — we want
		// to surface "your loop has a graveyard backup," not "Karmic
		// Guide combos with itself."
		for _, e := range live {
			if c.Cards[0] == e.name || c.Cards[1] == e.name {
				continue
			}
			pieceA, okA := profileByName[c.Cards[0]]
			pieceB, okB := profileByName[c.Cards[1]]
			if !okA || !okB {
				continue
			}
			// At least one piece must be recurable AND naturally
			// buriable during the combo. We don't require BOTH to be
			// recurable — most graveyard-loop scenarios survive on a
			// single rebought piece (rebuy the dead Karmic Guide;
			// Reveillark is still on the battlefield).
			candidates := []CardProfile{pieceA, pieceB}
			var recuredPiece string
			for _, piece := range candidates {
				if e.recurs(piece) && isBuriableInCombo(piece, c.Class) {
					recuredPiece = piece.Name
					break
				}
			}
			if recuredPiece == "" {
				continue
			}
			// Stable dedup key — combo cards sorted, paired with the
			// enabler name. Prevents the same (combo, enabler) showing
			// up twice when the same combo surfaces in both
			// TrueInfinites and Determined.
			sortedPair := []string{c.Cards[0], c.Cards[1]}
			sort.Strings(sortedPair)
			key := sortedPair[0] + "|" + sortedPair[1] + "|" + e.name
			if seen[key] {
				continue
			}
			seen[key] = true

			desc := fmt.Sprintf(
				"%s + %s loops via %s — when %s hits the graveyard during the play sequence, %s rebuys it. "+
					"B3-tier finisher: the 2-card combo has graveyard backup, but execution still costs a turn cycle.",
				c.Cards[0], c.Cards[1], e.name, recuredPiece, e.name)

			out = append(out, ComboResult{
				Cards:       []string{c.Cards[0], c.Cards[1], e.name},
				LoopType:    "synergy",
				Class:       ComboClassGraveyardLoop,
				Resources:   "graveyard recursion",
				Description: desc,
			})
		}
	}
	return out
}
