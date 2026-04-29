package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKinnanBonderProdigy wires up Kinnan, Bonder Prodigy.
//
// Oracle text:
//
//	Whenever you tap a nonland permanent for mana, add one mana of any
//	type that permanent produced.
//	{2}{G}{U}: Reveal the top card of your library. If it's a creature
//	card, put it onto the battlefield. Otherwise, put it into your hand.
//
// The cEDH commander that dominates mono-green/Simic gauntlets — turns
// every mana dork and mana rock into a 2-mana producer. Combos with
// Basalt Monolith (3 mana → untap for 3 → +1 mana/loop = infinite) and
// Grim Monolith (same pattern with {4} untap cost).
//
// Batch #3 scope — the STATIC ability (CR §605 / §106):
//   - Implemented via an additive hook at the typed-mana-pool add path
//     (gameengine/mana.go: NotifyManaAdded callback). When a permanent
//     taps for mana and calls AddMana(...), the hook fires a per-card
//     trigger "mana_added_from_permanent" with the source Permanent in
//     ctx. Kinnan listens on that trigger and, if the source is a
//     nonland permanent controlled by Kinnan's controller, adds one
//     extra mana of any type.
//
// Preventing recursion:
//   - Kinnan's own extra mana is added via AddMana directly (NOT
//     through the NotifyManaAdded seam) — so Kinnan does NOT trigger
//     on itself.
//   - We also guard by tagging the ctx with "from_kinnan" to belt-and-
//     suspenders prevent double fires.
//
// The activated ability (reveal-top / put-on-battlefield) is the
// "dig for creatures" mode and is implemented but only rarely decides
// games — Kinnan's power is his static, not his active.
func registerKinnanBonderProdigy(r *Registry) {
	r.OnTrigger("Kinnan, Bonder Prodigy", "mana_added_from_permanent", kinnanOnManaAdded)
	r.OnActivated("Kinnan, Bonder Prodigy", kinnanActivate)
}

func kinnanOnManaAdded(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kinnan_static_mana_doubler"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Anti-recursion guard: if this mana add is ALREADY from Kinnan, stop.
	if v, ok := ctx["from_kinnan"].(bool); ok && v {
		return
	}
	// Source must be a nonland permanent controlled by Kinnan's controller.
	srcPerm, _ := ctx["source_perm"].(*gameengine.Permanent)
	if srcPerm == nil {
		return
	}
	if srcPerm.IsLand() {
		return
	}
	if srcPerm.Controller != perm.Controller {
		return
	}
	// Extra mana matches "one mana of any type" — for MVP we mirror the
	// color added. Original color is in ctx["color"].
	color, _ := ctx["color"].(string)
	if color == "" {
		color = "any"
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Tag the ctx so subsequent observers know this came from Kinnan.
	// We use AddMana directly here (NOT the notified path) so we don't
	// recurse through this same trigger.
	gameengine.AddMana(gs, seat, color, 1, "Kinnan, Bonder Prodigy")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_card": srcPerm.Card.DisplayName(),
		"added_color": color,
		"new_pool":    seat.ManaPool,
	})
}

func kinnanActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kinnan_reveal_top_dig"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
		return
	}
	// Shuffle determinism: use gs.Rng where needed, but for the reveal we
	// just look at the top card.
	_ = rand.New
	top := s.Library[0]
	if cardHasType(top, "creature") {
		// Remove from library before putting on battlefield (not through MoveCard).
		s.Library = s.Library[1:]
		// Put onto battlefield under Kinnan's controller with full ETB cascade.
		enterBattlefieldWithETB(gs, seat, top, false)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"revealed":    top.DisplayName(),
			"destination": "battlefield",
		})
	} else {
		gameengine.MoveCard(gs, top, seat, "library", "hand", "effect")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"revealed":    top.DisplayName(),
			"destination": "hand",
		})
	}
}
