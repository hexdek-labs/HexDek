package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCabalRitual wires Cabal Ritual.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Cabal%20Ritual):
//
//	Add {B}{B}{B}.
//	Threshold — Add {B}{B}{B}{B}{B} instead if you have seven or more
//	cards in your graveyard.
//
// {1}{B} Instant. The threshold sibling to Dark Ritual — 2-mana cast
// for 3-mana add (net +1), 5-mana add (net +3) at threshold. cEDH
// storm-pile staple alongside Dark Ritual / Songs of the Damned for
// chaining into a 7+ mana finisher (Yawgmoth's Will, Bolas's Citadel,
// X-cost burn).
//
// Implementation:
//   - OnResolve. Read graveyard size at resolve time; >=7 → +5,
//     else +3. ManaPool += amount; AddMana log event mirrors Dark
//     Ritual's shape so storm-pile auditors see the same trail.
//   - Note: threshold count is "in your graveyard" at the moment the
//     spell resolves, NOT at cast time. Cards milled / discarded
//     between cast and resolution count.
func registerCabalRitual(r *Registry) {
	r.OnResolve("Cabal Ritual", cabalRitualResolve)
}

func cabalRitualResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cabal_ritual"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	amount := 3
	threshold := len(s.Graveyard) >= 7
	if threshold {
		amount = 5
	}

	s.ManaPool += amount
	gameengine.SyncManaAfterAdd(s, amount)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: "Cabal Ritual",
		Amount: amount,
		Details: map[string]interface{}{
			"reason":    "cabal_ritual",
			"threshold": threshold,
			"gy_size":   len(s.Graveyard),
		},
	})
	emit(gs, slug, "Cabal Ritual", map[string]interface{}{
		"seat":      seat,
		"added":     amount,
		"threshold": threshold,
		"gy_size":   len(s.Graveyard),
	})
}
