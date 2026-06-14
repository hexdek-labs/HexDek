package gameengine

// Cascade keyword (CR §702.84).
//
//   702.84a "Cascade is a triggered ability. 'Cascade' means 'When you
//           cast this spell, exile cards from the top of your library
//           until you exile a nonland card whose mana value is less than
//           this spell's mana value. You may cast that card without paying
//           its mana cost. Then put all exiled cards on the bottom of
//           your library in a random order.'"
//
// Implementation: when CastSpell pushes a cascade-bearing stack item,
// call ApplyCascade. It exiles cards from the top of the caster's library
// until it finds a nonland card with CMC < the cast spell's CMC, offers
// to cast it for free, then puts remaining exiled cards on the bottom in
// random order.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// HasCascadeKeyword returns true if the card carries Cascade (CR §702.84).
func HasCascadeKeyword(card *Card) bool {
	return CascadeCount(card) > 0
}

// CascadeCount returns the number of Cascade instances on a card. CR
// §702.85b: cascade is a separate triggered ability per instance, so a
// card printed "Cascade, cascade" (Maelstrom Wanderer = 2, Apex Devastator
// = 4) triggers cascade that many times. The cast path loops this many
// times; counting only the bool would cascade exactly once for those cards.
func CascadeCount(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	n := 0
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok || kw.Name == "" {
			continue
		}
		if equalFoldSimple(kw.Name, "cascade") {
			n++
		}
	}
	return n
}

// ApplyCascade resolves the Cascade trigger for a spell cast by `controller`.
// Returns true if a card was found and cast for free.
//
// Procedure (CR §702.84a):
//  1. Exile cards from the top of the library one at a time.
//  2. Stop when a nonland card with CMC < spellCMC is found.
//  3. The controller may cast that card without paying its mana cost.
//  4. Put all exiled cards (not the cast one) on the bottom of the
//     library in a random order.
func ApplyCascade(gs *GameState, controller int, spellCMC int, spellName string) bool {
	if gs == nil || controller < 0 || controller >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[controller]
	if seat == nil {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "cascade_trigger",
		Seat:   controller,
		Source: spellName,
		Amount: spellCMC,
		Details: map[string]interface{}{
			"rule": "702.84a",
		},
	})

	var exiled []*Card
	var found *Card

	// Exile cards from the top of the library until we find a nonland
	// with CMC < spellCMC.
	for len(seat.Library) > 0 {
		c := seat.Library[0]
		seat.Library = seat.Library[1:]
		exiled = append(exiled, c)

		if c == nil {
			continue
		}

		// Check if nonland.
		isLand := false
		for _, t := range c.Types {
			if strings.ToLower(t) == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}

		// Check CMC < spellCMC.
		cardCMC := manaCostOf(c)
		if cardCMC < spellCMC {
			found = c
			break
		}
	}

	castSucceeded := false
	if found != nil {
		gs.LogEvent(Event{
			Kind:   "cascade_hit",
			Seat:   controller,
			Source: found.DisplayName(),
			Amount: manaCostOf(found),
			Details: map[string]interface{}{
				"spell_cmc":  spellCMC,
				"found_cmc":  manaCostOf(found),
				"cards_exiled": len(exiled),
				"rule":       "702.84a",
			},
		})

		// Remove the found card from the exiled pile (it will be cast).
		for i, c := range exiled {
			if c == found {
				exiled = append(exiled[:i], exiled[i+1:]...)
				break
			}
		}

		// Cast for free. Build a stack item and resolve it.
		// The Hat decides whether to cast; MVP: always cast.
		shouldCast := true
		if seat.Hat != nil {
			// Hat doesn't have a "should cast cascade" method yet —
			// default to always casting (it's free, rarely wrong).
			shouldCast = true
		}

		if shouldCast {
			eff := collectSpellEffect(found)
			cascadeItem := &StackItem{
				Controller: controller,
				Card:       found,
				Effect:     eff,
				IsCopy:     false,
				CastZone:   "exile", // cascade casts the card from exile (CR §702.85a)
			}
			// NOTE: do NOT mutate found.Name here. The cascaded card is a
			// real card object; appending "(cascade)" permanently corrupts
			// its name, which then leaks onto the battlefield permanent /
			// graveyard card and breaks name-based matching (legend rule,
			// "creatures named X", tutors) and display. The cascade_hit /
			// stack_push events already attribute the cast.
			PushStackItem(gs, cascadeItem)

			// CR §702.85a — the cascaded card is CAST. A cast spell must do
			// the full cast-time bookkeeping: count toward storm / cast
			// count, fire "whenever you cast a spell" triggers (magecraft,
			// prowess, Rhystic Study, …), and trigger its OWN cast keywords
			// (storm copies, a chained cascade, discover). The previous
			// raw-PushStackItem path skipped all of this. Mirror CastSpell's
			// cast-trigger block, BEFORE the priority window so the triggers
			// land on the stack above the cascaded spell (CR §603.3).
			IncrementCastCount(gs, controller)
			RecordCast(gs, controller, found, 0)
			fireCastTriggers(gs, controller, found)
			if HasStormKeyword(found) {
				ApplyStormCopies(gs, cascadeItem, controller)
			}
			// CR §702.85b — cascade is a separate triggered ability PER
			// instance. When a cascade spell cascades INTO a multi-cascade
			// card (e.g. Maelstrom Wanderer ×2, Apex Devastator ×4), the
			// cast of that card fires cascade once per instance, exactly as
			// the CastSpell entry path does. The old single ApplyCascade call
			// here chained only ONE instance, so a cascaded-into Maelstrom
			// Wanderer cascaded once instead of twice.
			if cc := CascadeCount(found); cc > 0 {
				foundMV := manaCostOf(found)
				for i := 0; i < cc; i++ {
					ApplyCascade(gs, controller, foundMV, found.DisplayName())
				}
			}
			if cardHasKeyword(found, "discover") {
				PerformDiscover(gs, controller, manaCostOf(found))
			}
			InvokeCastHook(gs, cascadeItem)

			// CR §117.3 / §117.4 — once a spell is on the stack, opponents
			// receive priority before it resolves. Without this window the
			// cascaded spell can't be countered (Mindbreak Trap, Counterspell,
			// Mana Drain, etc.) — a real misplay surface, not just a rules
			// nicety. PriorityRound polls APNAP and pushes a response on
			// top if any seat chooses to counter; the guarded ResolveStackTop
			// below intentionally skips resolution if the stack changed
			// (a counter is on top instead) so the outer DrainStack can
			// resolve the response and any chained counterwar.
			PriorityRound(gs)
			if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == cascadeItem {
				ResolveStackTop(gs)
				StateBasedActions(gs)
			}
			castSucceeded = true
		} else {
			// Player chose not to cast — put it with the rest.
			exiled = append(exiled, found)
		}
	} else {
		gs.LogEvent(Event{
			Kind:   "cascade_whiff",
			Seat:   controller,
			Source: spellName,
			Amount: len(exiled),
			Details: map[string]interface{}{
				"spell_cmc":  spellCMC,
				"cards_exiled": len(exiled),
				"rule":       "702.84a",
			},
		})
	}

	// Put remaining exiled cards on bottom of library in random order.
	// Per CR §702.84a these cards were exiled during the cascade; route
	// each through MoveCard so §614 replacements and exile-leave triggers
	// fire as they return to the library.
	if len(exiled) > 0 && gs.Rng != nil {
		gs.Rng.Shuffle(len(exiled), func(i, j int) {
			exiled[i], exiled[j] = exiled[j], exiled[i]
		})
	}
	pile := exiled
	exiled = nil
	for _, c := range pile {
		MoveCard(gs, c, controller, "exile", "library_bottom", "cascade-miss-return")
	}

	return castSucceeded
}
