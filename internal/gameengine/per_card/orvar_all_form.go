package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOrvarAllForm wires Orvar, the All-Form.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Changeling
//	Whenever you cast an instant or sorcery spell, if it targets one or
//	  more other permanents you control, create a token that's a copy of
//	  one of those permanents.
//	When a spell or ability an opponent controls causes you to discard
//	  this card, create a token that's a copy of target permanent.
//
// Implementation:
//   - "instant_or_sorcery_cast": gate on caster_seat == perm.Controller.
//     Engine target tracking for "targets one or more permanents you
//     control" isn't readily exposed at the per-card layer — emitPartial.
//   - "card_discarded": if the discarded card is Orvar himself and the
//     cause is opponent-driven, create a token copy of the highest-power
//     creature we control.
//   - Changeling handled by AST keyword pipeline.
func registerOrvarAllForm(r *Registry) {
	r.OnTrigger("Orvar, the All-Form", "instant_or_sorcery_cast", orvarSpellCast)
	r.OnTrigger("Orvar, the All-Form", "card_discarded", orvarDiscarded)
}

func orvarSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "orvar_copy_target_permanent"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"target_tracking_for_orvar_copy_creation_not_modeled")
}

func orvarDiscarded(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "orvar_discarded_copy_target"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card != perm.Card {
		return
	}
	cause, _ := ctx["cause"].(string)
	// Heuristic: only fire on opponent-driven discard.
	if cause == "self" {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p == perm {
			continue
		}
		if pw := p.Power(); pw > bestPow {
			bestPow = pw
			pick = p
		}
	}
	if pick == nil {
		return
	}
	// Phase 5 chokepoint: route through MintTokenAsCopyOf so the token's
	// InstanceID is freshly minted (TK provenance) rather than inheriting
	// the source's OG ID — same fix shape as PR #853 (Drafna).
	cp := gameengine.MintTokenAsCopyOf(gs, pick.Card, perm.Controller, gameengine.CurrentMintEnablerID(gs))
	if cp == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "mint_token_returned_nil", nil)
		return
	}
	cp.IsCopy = true
	enterBattlefieldWithETB(gs, perm.Controller, cp, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copyOf": pick.Card.DisplayName(),
	})
}
