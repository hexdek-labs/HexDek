package main

import (
	"fmt"
	"io"
	"sort"
)

// ---------------------------------------------------------------------------
// Interaction floor — minimum interaction the opposing pod must resolve
// to shut down each detected combo. Bridges the per-combo vulnerability
// profile (ComboMetaVulnerability, PR #852) with the deck's defensive
// layer (InteractionPackage's Counterspells + Protection lists) into
// a single integer "how much interaction does it take" estimate per
// combo + deck-level Min / Max / Median rollup for deckbuild advice.
//
// Per-axis answer cost (the cheapest single way to neutralize the combo):
//
//   - removal: RemovalRequiredToBreak from ComboMetaVulnerability (1
//     for any unprotected permanent piece, 2 when every piece is
//     protected and needs strip+remove, 0 when the combo has no
//     permanent pieces at all — spot removal cannot break it).
//
//   - stax: 1 if StaxScore >= 1 (a stax hoser exists that hits this
//     combo's shape), else 0 = "this axis can't break this combo".
//
//   - graveyard: 1 if GraveyardScore >= 1, else 0.
//
//   - counter: always 1 — every combo has at least one keystone cast
//     moment the opponent can counter.
//
// CheapestAnswerCost = min over feasible axes (skipping 0 = "can't").
//
// Defensive layer tax — the extra interaction the opponent must
// resolve because the deck packs counters / protection of its own:
//
//   - CounterspellTax = floor(InteractionPackage.Counterspells / 3),
//     capped at 2. Applied to stack-based answers (stax piece resolve,
//     graveyard hate resolve, opponent counter on the keystone — all
//     stack interactions the deck's counters can answer).
//
//   - ProtectionTax = floor(InteractionPackage.Protection / 3), capped
//     at 2. Applied to removal-based answers (Veil of Summer / Heroic
//     Intervention / Lightning Greaves shield permanents from spot
//     removal, forcing the opponent to either commit a 2nd removal or
//     pivot to an exile / non-targeted answer).
//
//   - /3 floor is the rough empirical estimate: 6 counterspells in a
//     deck means the opponent should expect to fight through ~2 of
//     them on average to land their interaction; 3 means ~1. Cap at 2
//     because past that the tax doesn't compound — the opponent only
//     has ~3 interaction slots in a typical opening hand window
//     regardless of how much the defender has stacked up.
//
// InteractionFloor per combo = adjusted CheapestAnswerCost (axis
// cost + tax applied to that axis). Deck-level rollup surfaces:
//
//   - MinFloor / MaxFloor / MedianFloor across all combos
//   - CheapestComboLabel (opponent picks this line to attack first)
//   - HardestComboLabel (deck's most-resilient backup line)
//   - DeckbuildAdvice headline matching the floor shape — e.g. "all
//     combos fold to a single removal — pack 2-3 protection slots"
//     vs "graveyard hate is the only feasible answer — opponent must
//     find specific cards"
//
// Wiring: built in BuildDeckProfile right after
// BuildComboMetaInteraction (which produces the per-combo vulnerability
// inputs). Surfaced under unified_profile.interaction_floor in JSON
// and as a short "[CYA] INTERACTION FLOOR" text section.
// ---------------------------------------------------------------------------

// ComboInteractionFloor is the per-combo interaction-needed estimate.
type ComboInteractionFloor struct {
	ComboIndex int
	Label      string
	Source     string

	// Per-axis answer cost. 0 = this axis cannot break this combo
	// (cardinal "infeasible" — skipped in min computation).
	RemovalAnswerCost      int
	StaxAnswerCost         int
	GraveyardAnswerCost    int
	CounterspellAnswerCost int

	// CheapestAxis names the axis chosen for the floor. One of
	// "removal" / "stax" / "graveyard" / "counter". When multiple
	// axes tie at the same cost, the priority is removal > counter >
	// stax > graveyard (removal is the most common/available
	// interaction in EDH; graveyard hate is the most situational).
	CheapestAxis string

	// CheapestAnswerCost is the raw axis cost before defensive tax.
	CheapestAnswerCost int

	// DefensiveLayerTax is the extra interaction the opponent must
	// resolve due to the deck's own counterspell + protection layer.
	// Capped at 2 (see file header).
	DefensiveLayerTax int

	// InteractionFloor = CheapestAnswerCost + DefensiveLayerTax. The
	// total number of interaction cards the opposing pod must land
	// (resolve and stick) to shut this combo down.
	InteractionFloor int
}

// ComboInteractionFloorReport is the deck-level rollup.
type ComboInteractionFloorReport struct {
	PerCombo []ComboInteractionFloor

	// Min / Max / Median across all per-combo floors.
	MinFloor    int
	MaxFloor    int
	MedianFloor int

	// Deck defensive layer sizes (denormalized for display so JSON
	// consumers don't have to cross-reference InteractionPackage).
	CounterspellCount int
	ProtectionCount   int

	// Cheapest combo — the line opponents will attack first.
	CheapestComboLabel string
	CheapestComboIndex int
	CheapestFloor      int

	// Hardest combo — the resilient backup line.
	HardestComboLabel string
	HardestComboIndex int
	HardestFloor      int

	// DeckbuildAdvice is a one-line callout suggesting how the deck
	// could raise its floor (or noting that the floor is already
	// healthy). Always non-empty when PerCombo is non-empty.
	DeckbuildAdvice string
}

// BuildComboInteractionFloor produces the per-combo + deck-level
// interaction-floor analysis. Inputs: the per-combo vulnerability
// profile (cma) and the deck's interaction package (pkg, used for
// the defensive tax). Returns nil when there are no combos to score.
func BuildComboInteractionFloor(cma *ComboMetaInteraction, pkg InteractionPackage) *ComboInteractionFloorReport {
	if cma == nil || len(cma.PerCombo) == 0 {
		return nil
	}

	// Tax per defensive layer category. /3 with cap-at-2 — see file
	// header for rationale. Computed once for the whole report; same
	// defensive layer applies to every combo.
	counterspellTax := len(pkg.Counterspells) / 3
	if counterspellTax > 2 {
		counterspellTax = 2
	}
	protectionTax := len(pkg.Protection) / 3
	if protectionTax > 2 {
		protectionTax = 2
	}

	out := &ComboInteractionFloorReport{
		CounterspellCount: len(pkg.Counterspells),
		ProtectionCount:   len(pkg.Protection),
	}

	for _, v := range cma.PerCombo {
		f := ComboInteractionFloor{
			ComboIndex:             v.ComboIndex,
			Label:                  v.Label,
			Source:                 v.Source,
			RemovalAnswerCost:      v.RemovalRequiredToBreak,
			CounterspellAnswerCost: 1, // every combo has a keystone cast
		}
		if v.StaxScore >= 1 {
			f.StaxAnswerCost = 1
		}
		if v.GraveyardScore >= 1 {
			f.GraveyardAnswerCost = 1
		}

		// Pick cheapest feasible axis. Tie-breaks: removal > counter >
		// stax > graveyard (most-common interaction first).
		type axisOpt struct {
			name string
			cost int
			tax  int
		}
		opts := []axisOpt{
			{"removal", f.RemovalAnswerCost, protectionTax},
			{"counter", f.CounterspellAnswerCost, counterspellTax},
			{"stax", f.StaxAnswerCost, counterspellTax},
			{"graveyard", f.GraveyardAnswerCost, counterspellTax},
		}
		bestIdx := -1
		bestTotal := 0
		for i, o := range opts {
			if o.cost == 0 {
				continue // infeasible
			}
			total := o.cost + o.tax
			if bestIdx == -1 || total < bestTotal {
				bestIdx = i
				bestTotal = total
			}
		}
		// Every combo has at least the counter axis (CounterspellAnswerCost==1),
		// so bestIdx is always set. Defensive nil-check kept for safety.
		if bestIdx == -1 {
			// Should not happen — counter axis is always 1. Fall back
			// to counter with no tax.
			bestIdx = 1
			bestTotal = 1
		}
		f.CheapestAxis = opts[bestIdx].name
		f.CheapestAnswerCost = opts[bestIdx].cost
		f.DefensiveLayerTax = opts[bestIdx].tax
		f.InteractionFloor = bestTotal

		out.PerCombo = append(out.PerCombo, f)
	}

	// Deck-level rollup: min / max / median.
	floors := make([]int, len(out.PerCombo))
	for i, p := range out.PerCombo {
		floors[i] = p.InteractionFloor
	}
	sort.Ints(floors)
	out.MinFloor = floors[0]
	out.MaxFloor = floors[len(floors)-1]
	out.MedianFloor = floors[len(floors)/2]

	// Cheapest / hardest combo identification. Tie-break by index
	// ascending so output is deterministic across runs.
	for i, p := range out.PerCombo {
		if i == 0 || p.InteractionFloor < out.CheapestFloor {
			out.CheapestComboIndex = i
			out.CheapestComboLabel = p.Label
			out.CheapestFloor = p.InteractionFloor
		}
		if i == 0 || p.InteractionFloor > out.HardestFloor {
			out.HardestComboIndex = i
			out.HardestComboLabel = p.Label
			out.HardestFloor = p.InteractionFloor
		}
	}

	out.DeckbuildAdvice = renderDeckbuildAdvice(out)
	return out
}

// renderDeckbuildAdvice picks a one-line headline based on the floor
// shape. Rules:
//
//   - MinFloor == 1: the cheapest combo folds to a single interaction
//     spell. Advise adding protection / counterspell slots to raise the
//     floor.
//   - MinFloor >= 3: the deck is robust — interaction-heavy decks need
//     to commit multiple cards. Note this as a strength.
//   - Otherwise (MinFloor == 2): middle ground — note the cheapest line
//     and what raises it.
//
// Tail picks the most-impactful single change: counterspells when
// CounterspellCount is low, protection when ProtectionCount is low.
func renderDeckbuildAdvice(r *ComboInteractionFloorReport) string {
	if len(r.PerCombo) == 0 {
		return ""
	}
	tail := ""
	switch {
	case r.CounterspellCount < 3 && r.ProtectionCount < 3:
		tail = "add 3-4 counterspells AND 2-3 protection slots (Veil / Heroic Intervention)"
	case r.CounterspellCount < 3:
		tail = "add 3-4 counterspells to raise the stack-interaction tax"
	case r.ProtectionCount < 3:
		tail = "add 2-3 protection slots (Veil of Summer / Heroic Intervention / Lightning Greaves)"
	default:
		tail = "defensive layer is already healthy"
	}
	switch {
	case r.MinFloor == 1:
		return fmt.Sprintf("cheapest combo folds to 1 interaction (%s) — %s",
			r.CheapestComboLabel, tail)
	case r.MinFloor >= 3:
		return fmt.Sprintf("robust — every combo needs 3+ interaction to break (%s)",
			tail)
	default:
		return fmt.Sprintf("middling — cheapest combo costs %d interaction (%s); %s",
			r.MinFloor, r.CheapestComboLabel, tail)
	}
}

// printComboInteractionFloor renders the human-readable section. No-op
// when r is nil or has no combos.
func printComboInteractionFloor(w io.Writer, r *ComboInteractionFloorReport) {
	if r == nil || len(r.PerCombo) == 0 {
		return
	}
	fmt.Fprintf(w, "[CYA] INTERACTION FLOOR -- min %d, max %d, median %d (defenders: %d counters / %d protection)\n",
		r.MinFloor, r.MaxFloor, r.MedianFloor,
		r.CounterspellCount, r.ProtectionCount)
	fmt.Fprintf(w, "  Cheapest line to break: [%d] %s (floor %d)\n",
		r.CheapestComboIndex, r.CheapestComboLabel, r.CheapestFloor)
	if r.HardestComboIndex != r.CheapestComboIndex {
		fmt.Fprintf(w, "  Most resilient line: [%d] %s (floor %d)\n",
			r.HardestComboIndex, r.HardestComboLabel, r.HardestFloor)
	}
	for _, p := range r.PerCombo {
		taxNote := ""
		if p.DefensiveLayerTax > 0 {
			taxNote = fmt.Sprintf(" (+%d defensive tax)", p.DefensiveLayerTax)
		}
		fmt.Fprintf(w, "  [%d] %s — floor %d via %s axis (cost %d%s)\n",
			p.ComboIndex, p.Label, p.InteractionFloor,
			p.CheapestAxis, p.CheapestAnswerCost, taxNote)
	}
	fmt.Fprintf(w, "  Advice: %s\n\n", r.DeckbuildAdvice)
}

// comboInteractionFloorToJSON projects the report into the JSON shape
// (defined in report.go). Returns nil when the input is nil so the
// JSON field is omitted via omitempty.
func comboInteractionFloorToJSON(r *ComboInteractionFloorReport) *jsonComboInteractionFloor {
	if r == nil {
		return nil
	}
	perCombo := make([]jsonComboInteractionFloorEntry, len(r.PerCombo))
	for i, p := range r.PerCombo {
		perCombo[i] = jsonComboInteractionFloorEntry{
			ComboIndex:             p.ComboIndex,
			Label:                  p.Label,
			Source:                 p.Source,
			RemovalAnswerCost:      p.RemovalAnswerCost,
			StaxAnswerCost:         p.StaxAnswerCost,
			GraveyardAnswerCost:    p.GraveyardAnswerCost,
			CounterspellAnswerCost: p.CounterspellAnswerCost,
			CheapestAxis:           p.CheapestAxis,
			CheapestAnswerCost:     p.CheapestAnswerCost,
			DefensiveLayerTax:      p.DefensiveLayerTax,
			InteractionFloor:       p.InteractionFloor,
		}
	}
	return &jsonComboInteractionFloor{
		PerCombo:           perCombo,
		MinFloor:           r.MinFloor,
		MaxFloor:           r.MaxFloor,
		MedianFloor:        r.MedianFloor,
		CounterspellCount:  r.CounterspellCount,
		ProtectionCount:    r.ProtectionCount,
		CheapestComboLabel: r.CheapestComboLabel,
		CheapestComboIndex: r.CheapestComboIndex,
		CheapestFloor:      r.CheapestFloor,
		HardestComboLabel:  r.HardestComboLabel,
		HardestComboIndex:  r.HardestComboIndex,
		HardestFloor:       r.HardestFloor,
		DeckbuildAdvice:    r.DeckbuildAdvice,
	}
}

