package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ComboSequencer evaluates whether a seat can execute a combo win
// this turn. It reads Freya's combo packages (via StrategyProfile)
// and checks the player's zones for pieces and mana feasibility.
type ComboSequencer struct {
	Lines []ComboConstraint // loaded from Freya's combo packages
}

// NewComboSequencer builds a sequencer from a StrategyProfile's combo
// data. Returns nil when the profile has no combo plans, which callers
// treat as "no combos to evaluate."
func NewComboSequencer(sp *StrategyProfile) *ComboSequencer {
	if sp == nil || len(sp.ComboPieces) == 0 {
		return nil
	}
	lines := make([]ComboConstraint, 0, len(sp.ComboPieces))
	for _, cp := range sp.ComboPieces {
		if len(cp.Pieces) == 0 {
			continue
		}
		cc := ComboConstraint{
			Name:          comboName(cp),
			PiecesNeeded:  cp.Pieces,
			ZonesAccepted: buildZonesAccepted(cp),
			ManaRequired:  estimateManaRequired(cp),
			SequenceOrder: buildSequenceOrder(cp),
		}
		// Infinite combos often need protection to stick.
		if cp.Type == "infinite" {
			cc.NeedsProtection = true
		}
		lines = append(lines, cc)
	}
	if len(lines) == 0 {
		return nil
	}
	return &ComboSequencer{Lines: lines}
}

// comboName generates a readable name from the combo plan.
func comboName(cp ComboPlan) string {
	if len(cp.Pieces) == 0 {
		return "unknown"
	}
	parts := make([]string, len(cp.Pieces))
	copy(parts, cp.Pieces)
	name := strings.Join(parts, " + ")
	if cp.Type != "" {
		name += " (" + cp.Type + ")"
	}
	return name
}

// buildZonesAccepted creates the zone acceptance map. Most combo
// pieces need to be either in hand (castable), on the battlefield
// (already resolved), in the graveyard (reanimation), or in the
// command zone (the commander itself is part of the win line —
// Kinnan + Basalt Monolith, Najeela combats, Korvold sac, Esika +
// Bridge cheat). Without "command_zone" the sequencer treats every
// commander-engine deck as one piece short until the commander has
// been cast, so it never reports Executable on the assembly turn
// and the commander's cast never gets the combo-priority signal.
func buildZonesAccepted(cp ComboPlan) map[string][]string {
	zones := make(map[string][]string, len(cp.Pieces))
	for _, piece := range cp.Pieces {
		zones[piece] = []string{"hand", "battlefield", "graveyard", "command_zone"}
	}
	return zones
}

// buildSequenceOrder returns the cast order for the combo, falling
// back to piece order if no explicit CastOrder is provided.
func buildSequenceOrder(cp ComboPlan) []string {
	if len(cp.CastOrder) > 0 {
		return cp.CastOrder
	}
	return cp.Pieces
}

// estimateManaRequired estimates the total mana needed to execute a
// combo from hand. We sum the costs of pieces that aren't already on
// the battlefield; since we don't have board state at construction
// time, we use len(pieces) * 2 as a rough heuristic (most combo
// pieces are 2-4 CMC). The Evaluate method does the real check.
func estimateManaRequired(cp ComboPlan) int {
	// Rough estimate: 2 mana per piece. Real check happens at eval time.
	return len(cp.Pieces) * 2
}

// Evaluate checks all combo lines against the current game state for
// a seat. Returns a ComboAssessment describing the best available
// combo line and whether it can be executed this turn.
func (cs *ComboSequencer) Evaluate(gs *gameengine.GameState, seatIdx int) ComboAssessment {
	if cs == nil || gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return ComboAssessment{}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return ComboAssessment{}
	}

	// Build zone indexes: card name -> set of zones it appears in.
	zoneIndex := buildZoneIndex(seat)
	availMana := gameengine.AvailableManaEstimate(gs, seat)

	var best *comboEval
	for i := range cs.Lines {
		ev := evaluateLine(&cs.Lines[i], zoneIndex, seat, availMana)
		if best == nil || ev.betterThan(best) {
			best = ev
		}
	}

	if best == nil {
		return ComboAssessment{}
	}

	result := ComboAssessment{
		BestLine:    best.line,
		PiecesFound: best.found,
		PiecesTotal: len(best.line.PiecesNeeded),
	}

	if best.executable {
		result.Executable = true
		result.NextAction = best.nextAction
	} else if best.found >= 1 && best.missing >= 1 {
		// r60-cedh-sequencer: broaden the Assembling gate to cover the
		// canonical cEDH multi-tutor reach pattern. The pre-tuning gate
		// required missing==1 AND any tutor in hand, which captured only
		// the "one missing piece, one tutor" boundary case. The cEDH
		// signal we want to surface is broader: when the hand contains
		// >=1 anchoring real piece AND tutorsInHand >= missing, the
		// combo is functionally one cast-tutor-then-cast-piece sequence
		// from complete, regardless of how many slots remain. PlanState
		// switches into PlanAssemble on that signal, which then biases
		// the cast queue toward tutors and combo pieces (see
		// cardHeuristic combo-priority block in yggdrasil.go).
		tutors := seatTutorsInHand(seat)
		if tutors >= best.missing {
			result.Assembling = true
			result.MissingPiece = best.missingPiece
		}
	}

	return result
}

// comboEval is the internal evaluation result for a single combo line.
type comboEval struct {
	line         *ComboConstraint
	found        int    // pieces found in acceptable zones
	missing      int    // pieces not found
	executable   bool   // all pieces + mana available
	nextAction   string // first piece to cast from hand
	missingPiece string // name of the first missing piece (for tutor)
	manaOK       bool   // enough mana to cast remaining pieces from hand
}

// betterThan returns true if this eval is strictly better than other.
// Priority: executable > more pieces found > fewer pieces needed.
func (e *comboEval) betterThan(other *comboEval) bool {
	if e.executable != other.executable {
		return e.executable
	}
	// Higher completion ratio is better.
	eRatio := float64(e.found) / float64(max(len(e.line.PiecesNeeded), 1))
	oRatio := float64(other.found) / float64(max(len(other.line.PiecesNeeded), 1))
	if eRatio != oRatio {
		return eRatio > oRatio
	}
	// Tie-break: fewer total pieces (simpler combo) wins.
	return len(e.line.PiecesNeeded) < len(other.line.PiecesNeeded)
}

// zoneSet tracks which zones a card name appears in.
type zoneSet struct {
	hand        bool
	battlefield bool
	graveyard   bool
	commandZone bool
}

// buildZoneIndex scans a seat's hand, battlefield, graveyard, and
// command zone and returns a map of card name -> which zones that
// card appears in.
//
// DFC-aware: each Card is indexed under EVERY name Freya might emit
// for it — the runtime Card.Name plus, for MDFCs, the synthetic
// "Front // Back" full DFC string and each individual face name.
// Without this, an MDFC commander whose back face is a combo piece
// (Esika → Bridge) was unrecognized after a back-face cast mutated
// Card.Name to the back-face-only form, even though Freya emits
// pieces with the full "Front // Back" oracle name.
func buildZoneIndex(seat *gameengine.Seat) map[string]*zoneSet {
	idx := make(map[string]*zoneSet)

	ensure := func(name string) *zoneSet {
		if name == "" {
			return nil
		}
		if z, ok := idx[name]; ok {
			return z
		}
		z := &zoneSet{}
		idx[name] = z
		return z
	}

	mark := func(c *gameengine.Card, set func(*zoneSet)) {
		if c == nil {
			return
		}
		for _, alias := range cardAliases(c) {
			z := ensure(alias)
			if z != nil {
				set(z)
			}
		}
	}

	for _, c := range seat.Hand {
		mark(c, func(z *zoneSet) { z.hand = true })
	}
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		mark(p.Card, func(z *zoneSet) { z.battlefield = true })
	}
	for _, c := range seat.Graveyard {
		mark(c, func(z *zoneSet) { z.graveyard = true })
	}
	for _, c := range seat.CommandZone {
		mark(c, func(z *zoneSet) { z.commandZone = true })
	}
	return idx
}

// cardAliases returns every name Freya might emit for a card. Single-
// faced cards yield just c.Name. MDFCs yield c.Name + the full
// "Front // Back" string (synthesized when c.Name is just one face)
// and both face names individually. Aliasing on every face costs an
// extra map insert per zone scan but matches Freya's name conventions
// (which use the full DFC string in win lines and tutor paths).
func cardAliases(c *gameengine.Card) []string {
	if c == nil || c.Name == "" {
		return nil
	}
	out := []string{c.Name}
	back := c.BackFaceName
	if back == "" || !c.IsMDFC() {
		return out
	}
	if strings.Contains(c.Name, " // ") {
		// c.Name is the canonical "Front // Back" oracle string. Add
		// each face individually so a Freya piece keyed off only one
		// face still matches.
		parts := strings.SplitN(c.Name, " // ", 2)
		front := strings.TrimSpace(parts[0])
		face := strings.TrimSpace(parts[1])
		if front != "" && front != c.Name {
			out = append(out, front)
		}
		if face != "" && face != c.Name {
			out = append(out, face)
		}
		return out
	}
	// c.Name has been swapped to a single face (back-face cast or
	// transform on a TDFC). The back face is c.Name; we still know
	// c.BackFaceName. Without a stored FrontFaceName we can't
	// reconstruct the full "Front // Back" string from this side
	// alone, but we can still index the back face and any other
	// known alias.
	if back != c.Name {
		out = append(out, back)
	}
	return out
}

// evaluateLine checks a single combo line against the zone index and
// available mana.
func evaluateLine(line *ComboConstraint, zoneIndex map[string]*zoneSet, seat *gameengine.Seat, availMana int) *comboEval {
	ev := &comboEval{
		line: line,
	}

	// Track mana needed to cast pieces that are in hand (not yet on battlefield).
	manaCost := 0

	for _, piece := range line.PiecesNeeded {
		z, exists := zoneIndex[piece]
		if !exists {
			ev.missing++
			if ev.missingPiece == "" {
				ev.missingPiece = piece
			}
			continue
		}

		// Check if the piece is in an acceptable zone for this combo.
		// Battlefield is the ideal source (no cast required); when a
		// piece appears in multiple zones, we always prefer battlefield
		// so we don't double-count its cast cost.
		accepted := line.ZonesAccepted[piece]
		inAcceptableZone := false
		needsCast := false
		castFromCommandZone := false

		// Linear scan over accepted zones — accepted is small (1-3
		// entries in practice), so avoiding a map allocation pays off
		// in the hot evaluation path.
		var acceptsBattlefield, acceptsHand, acceptsCommand, acceptsGraveyard bool
		for _, zone := range accepted {
			switch zone {
			case "battlefield":
				acceptsBattlefield = true
			case "hand":
				acceptsHand = true
			case "command_zone":
				acceptsCommand = true
			case "graveyard":
				acceptsGraveyard = true
			}
		}

		switch {
		case acceptsBattlefield && z.battlefield:
			inAcceptableZone = true
			// already resolved — no mana required.
		case acceptsHand && z.hand:
			inAcceptableZone = true
			needsCast = true
		case acceptsCommand && z.commandZone:
			inAcceptableZone = true
			needsCast = true
			castFromCommandZone = true
		case acceptsGraveyard && z.graveyard:
			inAcceptableZone = true
			needsCast = true
		}

		if inAcceptableZone {
			ev.found++
			if needsCast {
				manaCost += pieceCastCost(seat, piece, castFromCommandZone)
			}
		} else {
			ev.missing++
			if ev.missingPiece == "" {
				ev.missingPiece = piece
			}
		}
	}

	ev.manaOK = availMana >= manaCost
	ev.executable = ev.missing == 0 && ev.manaOK

	// Determine next action: first piece in sequence order that needs
	// to be cast — either from hand or from the command zone. The
	// command-zone branch routes through tryCastCommander; the hand
	// branch is consumed directly by ChooseSpellToCast.
	if ev.executable {
		for _, name := range line.SequenceOrder {
			z, ok := zoneIndex[name]
			if !ok {
				continue
			}
			if z.battlefield {
				continue
			}
			if z.hand || z.commandZone {
				ev.nextAction = name
				break
			}
		}
		// If all pieces are on battlefield (activated combo), the next
		// action is the first piece in sequence order.
		if ev.nextAction == "" && len(line.SequenceOrder) > 0 {
			ev.nextAction = line.SequenceOrder[0]
		}
	}

	return ev
}

// cardManaCost looks up the mana cost for a card by name in the
// seat's hand. Returns 0 if not found (tokens, free spells).
func cardManaCost(seat *gameengine.Seat, name string) int {
	for _, c := range seat.Hand {
		if c != nil && cardMatchesAlias(c, name) {
			return gameengine.ManaCostOf(c)
		}
	}
	return 0
}

// pieceCastCost returns the mana the seat actually needs to cast
// `name` from the appropriate source zone. Hand pieces use the
// printed front-face cost (or back-face cost if the AI has decided
// to cast the back face — same convention as castableCards). Command-
// zone pieces add the §903.8 tax. Pieces not located in either zone
// (e.g. graveyard reanimation) return 0 because the reanimation cost
// is paid by the recursion spell itself, which is tracked separately.
func pieceCastCost(seat *gameengine.Seat, name string, fromCommandZone bool) int {
	if fromCommandZone {
		for _, c := range seat.CommandZone {
			if c == nil || !cardMatchesAlias(c, name) {
				continue
			}
			base := gameengine.ManaCostOf(c)
			// MDFC commanders whose back face is non-creature should be
			// quoted at the back-face cost — that's the cast tryCastCommander
			// will actually make. Match the heuristic in tournament/turn.go.
			if c.IsMDFC() && c.BackFaceCMC > 0 && mdfcBackFaceIsNonCreatureCombo(c) {
				base = c.BackFaceCMC
			}
			tax := 0
			if seat.CommanderTax != nil {
				tax = seat.CommanderTax[name]
				// Tax may be keyed off the canonical full DFC name even
				// when the piece is one face. Try both lookups.
				if tax == 0 && c.Name != name {
					tax = seat.CommanderTax[c.Name]
				}
			}
			return base + 2*tax
		}
		return 0
	}
	return cardManaCost(seat, name)
}

// cardMatchesAlias reports whether `name` is one of `c`'s known
// aliases (runtime Name, full DFC string, either face). Mirrors the
// indexing in cardAliases so consumers like cardManaCost can find a
// piece by any name Freya might emit.
func cardMatchesAlias(c *gameengine.Card, name string) bool {
	if c == nil || name == "" {
		return false
	}
	for _, a := range cardAliases(c) {
		if a == name {
			return true
		}
	}
	return false
}

// mdfcBackFaceIsNonCreatureCombo mirrors mdfcPreferBackFace from
// internal/tournament/turn.go: an MDFC whose front face is a creature
// and back face is not is preferentially cast as the back face. We
// duplicate the predicate here rather than import-cycling through
// the tournament package.
func mdfcBackFaceIsNonCreatureCombo(c *gameengine.Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	frontIsCreature := false
	for _, t := range c.Types {
		if t == "creature" {
			frontIsCreature = true
			break
		}
	}
	if !frontIsCreature {
		return false
	}
	for _, t := range c.BackFaceTypes {
		if t == "creature" {
			return false
		}
	}
	return true
}
