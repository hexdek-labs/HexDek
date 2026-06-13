package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// traumatize_r60.go — per_card handler for Traumatize.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Target player mills half their library, rounded down.
//
// {3}{U}{U} Sorcery. THE mill-deck centerpiece — instantly halves a
// library, the enabler for self-mill graveyard combos (cast on yourself
// to fuel a reanimator / dredge engine) and the classic two-card mill
// kill alongside a second Traumatize or a finisher. Parses to a
// `parsed_effect_residual` raw-text node ("target player mills half
// their library, rounded down") with no structured Mill node, so the
// generic dispatch logged it inert and milled ZERO cards.
//
// Implementation:
//   - OnResolve. "Target player" — harmful default is an opponent. Hat
//     policy picks the opponent with the LARGEST library (maximum cards
//     denied / closest to a deck-out). If somehow no living opponent is
//     available the spell fizzles (logged), matching the §608.2b
//     "all targets illegal" outcome.
//   - Mills floor(library/2) via gs.millOne — the same per-card mill
//     primitive resolveMill uses, so every mill fires its zone-change
//     triggers (e.g. dredge prompts, "whenever a card is put into a
//     graveyard from a library" payoffs).
func init() {
	registerTraumatizeR60(Global())
	AddResetHook(registerTraumatizeR60)
}

func registerTraumatizeR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Traumatize", traumatizeResolve)
}

func traumatizeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "traumatize"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Pick the opponent with the largest library (maximum mill).
	target := -1
	best := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if n := len(os.Library); n > best {
			best = n
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, "Traumatize", "no_target", nil)
		return
	}

	count := len(gs.Seats[target].Library) / 2
	milled := 0
	for i := 0; i < count; i++ {
		ts := gs.Seats[target]
		if ts == nil || len(ts.Library) == 0 {
			break
		}
		top := ts.Library[0]
		gameengine.MoveCard(gs, top, target, "library", "graveyard", "mill")
		milled++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Target: target,
		Source: "Traumatize",
		Amount: milled,
	})
	emit(gs, slug, "Traumatize", map[string]interface{}{
		"seat":        seat,
		"target_seat": target,
		"milled":      milled,
	})
}
