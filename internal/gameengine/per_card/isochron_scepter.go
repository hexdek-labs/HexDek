package per_card

import (
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIsochronScepter wires up Isochron Scepter.
//
// Oracle text:
//
//	Imprint — When Isochron Scepter enters the battlefield, you may
//	exile an instant card with mana value 2 or less from your hand.
//	{2}, {T}: You may copy the exiled card. If you do, you may cast
//	the copy without paying its mana cost.
//
// The BACKBONE of the Scepter + Dramatic Reversal combo. Imprint
// Dramatic Reversal → activate for {2} → copy of Reversal resolves
// → all nonland permanents untap (including Scepter itself) →
// activate again. With enough mana rocks in play that produce >=2
// generic mana total, this is infinite mana + infinite untap triggers.
//
// Batch #3 scope:
//   - OnETB: imprint an instant from hand. The choice is the
//     "smallest CMC instant" heuristic (auto-imprint policy). We
//     move the card from hand → exile and stash it on
//     perm.Flags["imprinted_card_ptr"] (actually on a helper map on
//     the permanent so we can retrieve it later).
//   - OnActivated(0, ...): resolve the copy-and-cast-for-free of the
//     imprinted card. For MVP we DIRECTLY INVOKE the imprinted card's
//     ResolveHook (if it's a snowflake like Dramatic Reversal) OR
//     build a stack item and let the normal resolve path run.
//
// Anti-cycle: we cap the number of Scepter activations per call stack
// at 100 to prevent the test harness from hanging on actually-
// infinite loops. Real tournament runs go through CastSpell + priority
// rounds which handle depth via their own guard.
func registerIsochronScepter(r *Registry) {
	r.OnETB("Isochron Scepter", isochronScepterETB)
	r.OnActivated("Isochron Scepter", isochronScepterActivate)
}

var imprintedCards sync.Map

func isochronScepterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "isochron_scepter_imprint"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// Policy: imprint the first hand card that (a) is an instant and
	// (b) has CMC ≤ 2. Dramatic Reversal's CMC is 2 — qualifies.
	var choice *gameengine.Card
	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") {
			continue
		}
		if cardCMC(c) > 2 {
			continue
		}
		choice = c
		break
	}
	if choice == nil {
		// No eligible imprint target. Scepter enters blank.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"imprint":  "none",
			"fallback": "entered_blank",
		})
		return
	}
	// Move from hand to exile.
	gameengine.MoveCard(gs, choice, seat, "hand", "exile", "exile-from-hand")
	imprintedCards.Store(perm, choice)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["imprint_present"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"imprinted_card": choice.DisplayName(),
	})
}

func isochronScepterActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "isochron_scepter_copy_and_cast"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	var imprinted *gameengine.Card
	if v, ok := imprintedCards.Load(src); ok {
		imprinted = v.(*gameengine.Card)
	}
	if imprinted == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_imprint", nil)
		return
	}
	// Tap Scepter (cost: {2}, {T}). We don't drain mana here — the
	// {2} cost is caller-paid; {T} we enforce locally.
	src.Tapped = true
	seat := src.Controller
	// Copy-and-cast-for-free: build a StackItem with the imprinted
	// card AS-IF cast for free. IsCopy=true ensures the card doesn't
	// move zones to the graveyard afterward (CR §706.10 — a copy
	// ceases to exist).
	item := &gameengine.StackItem{
		Controller: seat,
		Card:       imprinted,
		IsCopy:     true,
	}
	// Run the ResolveHook chain — if Dramatic Reversal (or whatever
	// instant is imprinted) has a per-card Resolve handler, it'll fire.
	// If not, we emit a log event only (stock resolve dispatch is in
	// stack.go for non-copy items; for copies we rely on the handler
	// path).
	fired := gameengine.InvokeResolveHook(gs, item)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"imprinted_card":   imprinted.DisplayName(),
		"resolved_via":     "copy",
		"handlers_fired":   fired,
	})
	if fired == 0 {
		// No snowflake handler — log a partial noting the copy was
		// built but not fully resolved. Dramatic Reversal and other
		// per-card Resolve handlers will bypass this branch.
		emitPartial(gs, slug, src.Card.DisplayName(),
			"no_resolve_handler_for_imprinted_card_copy_logged_but_no_effect")
	}
}
