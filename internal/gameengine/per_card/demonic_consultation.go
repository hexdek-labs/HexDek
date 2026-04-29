package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDemonicConsultation wires up Demonic Consultation.
//
// Oracle text:
//
//	Name a card. Exile the top six cards of your library, then reveal
//	cards from the top of your library until you reveal the named
//	card. Put that card into your hand and exile all other cards
//	revealed this way.
//
// Thoracle-combo usage: name a card NOT in the deck. The reveal loop
// never finds a match, so the whole library gets exiled — triggering
// Thassa's Oracle's "library ≤ devotion" win.
//
// Context support: ctx["named_card"] optionally provides the named
// card. When absent or empty, we default to the combo-line behavior
// (library-empty).
func registerDemonicConsultation(r *Registry) {
	r.OnResolve("Demonic Consultation", demonicConsultationResolve)
}

func demonicConsultationResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "demonic_consultation"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Default to combo line: no named card → exile entire library.
	named := ""
	if v, ok := gs.Flags["_consultation_named_"+intToStr(seat)]; ok && v != 0 {
		// Numeric flag is a future extension; not used in MVP.
		_ = v
	}
	if item.Card != nil {
		// Look for a "named_card" hint attached to the card's Flags.
		// Tests can set this via card.Types as "named:Foo"; extract here.
		named = extractNamedCardHint(item.Card)
	}

	// Step 1: exile top six.
	exiledSix := 0
	for i := 0; i < 6 && len(s.Library) > 0; i++ {
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "exile", "exile-from-library")
		exiledSix++
	}

	// Step 2: reveal until named card (or library empty).
	revealed := 0
	found := false
	for len(s.Library) > 0 {
		c := s.Library[0]
		revealed++
		if named != "" && strings.EqualFold(c.DisplayName(), named) {
			gameengine.MoveCard(gs, c, seat, "library", "hand", "tutor-to-hand")
			found = true
			break
		}
		gameengine.MoveCard(gs, c, seat, "library", "exile", "exile-from-library")
	}

	emit(gs, slug, "Demonic Consultation", map[string]interface{}{
		"seat":             seat,
		"exiled_top_six":   exiledSix,
		"revealed_count":   revealed,
		"found_named":      found,
		"named_card":       named,
		"library_remaining": len(s.Library),
	})

	// Note: demonic consultation does NOT itself check Thassa's Oracle
	// or Laboratory Maniac. Those trigger as replacement effects on draws
	// (Labman) or ETB triggers (Thoracle when she's already in play, she
	// would re-evaluate on... no actually Thoracle only checks at ETB).
	// The combo pattern is: play Thoracle FIRST, THEN cast Consultation.
	// Consultation empties library → next SBA pass makes any draw fail,
	// and Thoracle's ETB has already won the game. In our model we
	// evaluate Thoracle's ETB condition once; an already-in-play Oracle
	// does NOT re-trigger on library changes.
	//
	// For test fixtures that cast Consultation first, then Oracle, the
	// Oracle ETB handler will see the empty library and win immediately.
}

// extractNamedCardHint pulls a "named:CardName" marker out of card.Types.
// This is a test-friendliness hook — there's no UI choice yet.
func extractNamedCardHint(card *gameengine.Card) string {
	if card == nil {
		return ""
	}
	for _, t := range card.Types {
		if strings.HasPrefix(t, "named:") {
			return strings.TrimPrefix(t, "named:")
		}
	}
	return ""
}

func intToStr(n int) string {
	// Tiny, allocation-free int-to-string for internal flag keys.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [16]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
