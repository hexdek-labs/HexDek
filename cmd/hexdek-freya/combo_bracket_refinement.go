package main

import "fmt"

// ---------------------------------------------------------------------------
// Timing+floor B5 gate — tightens the cEDH bracket boundary using the
// new combo timing (PR #891) and InteractionFloor (PR #872) data.
//
// Currently the B5 gate in estimateMeasuredBracket uses card-list signals
// (free-interaction pieces, tutor density, GC count, avgCMC). Those catch
// the "tuned-deck shape" but can over-promote a deck that stacked the
// signals without actually having a fast-and-cheap-to-execute combo.
//
// cEDH play pattern: turn 3-4 wins through assembled 2-3 card combos that
// the opposing pod must answer with a SINGLE interaction spell (combos
// are fast and answerable, but you have to find the answer in the
// turn-3 window). Concretely:
//
//   - At least one combo with EarliestTurn ≤ 4 (turn-3 or turn-4 kill)
//   - That same combo's InteractionFloor ≤ 2 (opposing pod needs ≤2
//     interaction spells to neutralize — usually 1, occasionally 2
//     for protected pieces or moderate defender layer)
//
// If MeasuredBracket == 5 AND no combo meets this profile, demote to B4.
// The card-list shape may have ticked all the cEDH boxes, but without a
// timing-validated cheap-to-answer combo the deck doesn't actually play
// like cEDH — it's an optimized B4 that's slow to close.
//
// Demotion is recorded as a BracketSignal of kind "gate" on
// BracketRationale.Signals so the rationale shows the timing-gate
// reasoning alongside the existing card-list gates.
//
// Reverse direction (B4 → B5 promotion) is NOT applied. The card-list
// gate already requires multiple signals; lacking those, a B4 deck
// with a fast combo line is still B4 by design (a single fast combo
// doesn't make a deck cEDH if the consistency engine isn't there).
//
// Wiring: called from BuildDeckProfile at the very end (after
// ComboTiming + InteractionFloor + all other dp.* fields are populated).
// Mutates dp.MeasuredBracket / dp.MeasuredBracketLabel, syncs dp.Bracket
// when no rubber-stamp override is in play (Bracket == MeasuredBracket
// pre-mutation), and appends to dp.BracketRationale.Signals.
// ---------------------------------------------------------------------------

const (
	cedhTimingFloorMaxTurn = 4 // EarliestTurn ≤ 4 to qualify as fast-enough
	cedhTimingFloorMaxCost = 2 // InteractionFloor ≤ 2 to qualify as cheap-to-answer
)

// applyTimingFloorB5Gate runs the timing+floor B5 confirmation gate.
// No-op when dp is nil, not at B5, or has no combo data. Sees only
// inputs already attached to dp — does not re-derive timing/floor.
func applyTimingFloorB5Gate(dp *DeckProfile) {
	if dp == nil || dp.MeasuredBracket != 5 {
		return
	}

	// If ANY combo qualifies (fast AND cheap), the gate confirms cEDH.
	if hasCEDHTimingFloor(dp) {
		if dp.BracketRationale != nil {
			dp.BracketRationale.Signals = append(dp.BracketRationale.Signals, BracketSignal{
				Name: "Timing+floor gate",
				Kind: "gate",
				Note: cedhConfirmNote(dp),
			})
		}
		return
	}

	// Demote — no combo meets the cEDH profile.
	prevBracket := dp.MeasuredBracket
	prevLabel := dp.MeasuredBracketLabel
	dp.MeasuredBracket = 4
	dp.MeasuredBracketLabel = "Optimized"
	// Sync the user-visible Bracket only when no rubber-stamp override
	// is in play — i.e. it currently mirrors the pre-mutation measured
	// values. Preserves the WotC-precon B2 declared bracket and similar.
	if dp.Bracket == prevBracket && dp.BracketLabel == prevLabel {
		dp.Bracket = dp.MeasuredBracket
		dp.BracketLabel = dp.MeasuredBracketLabel
	}
	if dp.BracketRationale != nil {
		dp.BracketRationale.Signals = append(dp.BracketRationale.Signals, BracketSignal{
			Name: "Timing+floor gate",
			Kind: "gate",
			Note: cedhDemoteNote(dp),
		})
	}
}

// hasCEDHTimingFloor returns true if at least one combo has BOTH
// EarliestTurn ≤ cedhTimingFloorMaxTurn AND its matching
// InteractionFloor entry's InteractionFloor ≤ cedhTimingFloorMaxCost.
//
// Matching is by ComboIndex — both reports use the same source-asc /
// label-asc dedup ordering, so indices align.
func hasCEDHTimingFloor(dp *DeckProfile) bool {
	if dp == nil || dp.ComboTiming == nil || dp.InteractionFloor == nil {
		return false
	}
	if len(dp.ComboTiming.PerCombo) == 0 || len(dp.InteractionFloor.PerCombo) == 0 {
		return false
	}
	floorByIndex := map[int]int{}
	for _, f := range dp.InteractionFloor.PerCombo {
		floorByIndex[f.ComboIndex] = f.InteractionFloor
	}
	for _, t := range dp.ComboTiming.PerCombo {
		if t.EarliestTurn > cedhTimingFloorMaxTurn {
			continue
		}
		floor, ok := floorByIndex[t.ComboIndex]
		if !ok {
			continue
		}
		if floor <= cedhTimingFloorMaxCost {
			return true
		}
	}
	return false
}

// cedhConfirmNote builds the rationale line for the confirming case.
// Picks the fastest qualifying combo for evidence.
func cedhConfirmNote(dp *DeckProfile) string {
	if dp == nil || dp.ComboTiming == nil || dp.InteractionFloor == nil {
		return "confirmed: timing+floor data unavailable"
	}
	floorByIndex := map[int]int{}
	for _, f := range dp.InteractionFloor.PerCombo {
		floorByIndex[f.ComboIndex] = f.InteractionFloor
	}
	bestIdx := -1
	bestTurn := 99
	for _, t := range dp.ComboTiming.PerCombo {
		if t.EarliestTurn > cedhTimingFloorMaxTurn {
			continue
		}
		floor, ok := floorByIndex[t.ComboIndex]
		if !ok || floor > cedhTimingFloorMaxCost {
			continue
		}
		if t.EarliestTurn < bestTurn {
			bestTurn = t.EarliestTurn
			bestIdx = t.ComboIndex
		}
	}
	if bestIdx == -1 {
		return "confirmed: at least one combo meets cEDH timing+floor profile"
	}
	label := ""
	for _, t := range dp.ComboTiming.PerCombo {
		if t.ComboIndex == bestIdx {
			label = t.Label
			break
		}
	}
	return fmt.Sprintf(
		"confirmed: combo %q assembles by turn %d at interaction floor %d (cEDH profile: turn ≤%d AND floor ≤%d)",
		label, bestTurn, floorByIndex[bestIdx],
		cedhTimingFloorMaxTurn, cedhTimingFloorMaxCost,
	)
}

// cedhDemoteNote builds the rationale line for the demotion case.
// Calls out why no combo qualified.
func cedhDemoteNote(dp *DeckProfile) string {
	if dp == nil || dp.ComboTiming == nil || dp.InteractionFloor == nil {
		return fmt.Sprintf(
			"demoted to B4: no combo timing+floor data available to confirm cEDH profile (need turn ≤%d AND floor ≤%d)",
			cedhTimingFloorMaxTurn, cedhTimingFloorMaxCost,
		)
	}
	if len(dp.ComboTiming.PerCombo) == 0 {
		return fmt.Sprintf(
			"demoted to B4: no combos detected (cEDH requires ≥1 combo meeting turn ≤%d AND floor ≤%d)",
			cedhTimingFloorMaxTurn, cedhTimingFloorMaxCost,
		)
	}
	// Diagnose: was the issue timing or floor?
	floorByIndex := map[int]int{}
	for _, f := range dp.InteractionFloor.PerCombo {
		floorByIndex[f.ComboIndex] = f.InteractionFloor
	}
	fastCount := 0
	cheapCount := 0
	for _, t := range dp.ComboTiming.PerCombo {
		if t.EarliestTurn <= cedhTimingFloorMaxTurn {
			fastCount++
		}
		if f, ok := floorByIndex[t.ComboIndex]; ok && f <= cedhTimingFloorMaxCost {
			cheapCount++
		}
	}
	return fmt.Sprintf(
		"demoted to B4: no combo meets cEDH timing+floor profile (%d/%d combos turn ≤%d, %d/%d combos floor ≤%d — need a combo with BOTH)",
		fastCount, len(dp.ComboTiming.PerCombo), cedhTimingFloorMaxTurn,
		cheapCount, len(dp.ComboTiming.PerCombo), cedhTimingFloorMaxCost,
	)
}
