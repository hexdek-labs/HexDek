package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagusOfTheWill wires Magus of the Will (R60).
//
// Oracle text:
//
//	{2}{B}, {T}, Exile this creature: Until end of turn, you may
//	play lands and cast spells from your graveyard. If a card would
//	be put into your graveyard from anywhere this turn, exile that
//	card instead.
//
// The Time Spiral Remastered "Magus of —" reprint of Yawgmoth's
// Will as an activated creature ability. Slower (sorcery-speed
// activation, summoning sickness) but recursable, sacrifices
// itself rather than going to a graveyard. The activation cost
// already exiles Magus, so the §614 GY→exile replacement we install
// doesn't need to redirect Magus itself.
//
// Implementation:
//   - OnActivated(abilityIdx 0): enforce {T} legality (untapped, not
//     summoning sick), pay {2}{B} from the mana pool, exile the
//     creature (the activation cost's third element), then invoke
//     the R60 primitive with Permanent=false / SourcePerm=nil so the
//     grant survives Magus leaving the battlefield.
//
// The 3-mana cost is paid as a generic-3 deduction from the seat's
// mana pool — the engine's mana-pool model doesn't carry per-color
// granularity, so {2}{B} is approximated as 3 generic mana (same
// shortcut every other per_card activated handler takes).
func registerMagusOfTheWill(r *Registry) {
	r.OnActivated("Magus of the Will", magusOfTheWillActivate)
}

func magusOfTheWillActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "magus_of_the_will"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seatIdx]
	if s == nil || s.Lost {
		return
	}
	// {T} legality (CR §602.1).
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", map[string]interface{}{
			"seat": seatIdx,
		})
		return
	}
	if src.SummoningSick {
		emitFail(gs, slug, src.Card.DisplayName(), "summoning_sick", map[string]interface{}{
			"seat": seatIdx,
		})
		return
	}
	// {2}{B} mana cost — represented as 3 generic against the seat's
	// mana pool (engine doesn't model per-color pool granularity).
	const manaCost = 3
	if s.ManaPool < manaCost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seatIdx,
			"have":      s.ManaPool,
			"want":      manaCost,
		})
		return
	}
	s.ManaPool -= manaCost
	gameengine.SyncManaAfterSpend(s)
	src.Tapped = true
	// Exile this creature — the third element of the activation cost.
	// ExilePermanent handles the §614 would-be-exiled chain, dies/LTB
	// triggers, commander redirect, and aura detach. Source=nil since
	// the exiler is the cost itself, not another permanent.
	if !gameengine.ExilePermanent(gs, src, nil) {
		emitFail(gs, slug, src.Card.DisplayName(), "self_exile_failed", map[string]interface{}{
			"seat": seatIdx,
		})
		return
	}
	// Effect: install the R60 grant bundle. The Magus is now in
	// exile; SourcePerm stays nil so cleanup happens at EOT via
	// ExpirePlayFromGraveyardForTurn rather than via LTB.
	granted := gameengine.RegisterPlayFromGraveyard(gs, gameengine.PlayFromGraveyardOptions{
		SeatIdx:    seatIdx,
		SourceName: src.Card.DisplayName(),
		SourcePerm: nil,
		Permanent:  false,
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seatIdx,
		"mana_paid":       manaCost,
		"per_card_grants": granted,
		"duration":        "until_end_of_turn",
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"land_play_from_graveyard consumer not yet wired in tryPlayLand")
}
