package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEmetSelch wires Emet-Selch, Unsundered // Hades, Sorcerer of Eld.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	  Emet-Selch (front):
//		Vigilance
//		Whenever Emet-Selch enters or attacks, draw a card, then discard a
//		  card.
//		At the beginning of your upkeep, if there are fourteen or more cards
//		  in your graveyard, you may transform Emet-Selch.
//
//	  Hades (back, Avatar):
//		Vigilance
//		Echo of the Lost — During your turn, you may play cards from your
//		  graveyard.
//		If a card or token would be put into your graveyard from anywhere,
//		  exile it instead.
//
// Implementation:
//   - "permanent_etb" / "creature_attacks": draw 1, then discard the
//     least useful card (highest CMC, defensive: hand[len-1]) — graveyard
//     filling is the goal.
//   - "upkeep_controller": if our graveyard has 14+ cards and front face
//     is up, transform via gameengine.TransformPermanent if available;
//     otherwise emitPartial.
//   - Hades back face: "play cards from graveyard" + "graveyard
//     replacement to exile" are static replacement effects not modeled.
func registerEmetSelch(r *Registry) {
	r.OnETB("Emet-Selch, Unsundered", emetSelchETB)
	r.OnETB("Emet-Selch, Unsundered // Hades, Sorcerer of Eld", emetSelchETB)
	r.OnTrigger("Emet-Selch, Unsundered", "creature_attacks", emetSelchAttacks)
	r.OnTrigger("Emet-Selch, Unsundered // Hades, Sorcerer of Eld", "creature_attacks", emetSelchAttacks)
	r.OnTrigger("Emet-Selch, Unsundered", "upkeep_controller", emetSelchUpkeep)
	r.OnTrigger("Emet-Selch, Unsundered // Hades, Sorcerer of Eld", "upkeep_controller", emetSelchUpkeep)
}

func emetSelchETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	emetSelchLoot(gs, perm, "etb")
}

func emetSelchAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	emetSelchLoot(gs, perm, "attack")
}

func emetSelchLoot(gs *gameengine.GameState, perm *gameengine.Permanent, reason string) {
	const slug = "emet_selch_loot"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	drawn := drawOne(gs, seat, perm.Card.DisplayName())
	discardName := ""
	if len(s.Hand) > 0 {
		// Pick highest CMC in hand to discard (graveyard-fill heuristic).
		idx := 0
		bestCMC := -1
		for i, c := range s.Hand {
			if c == nil {
				continue
			}
			if cm := gameengine.ManaCostOf(c); cm > bestCMC {
				bestCMC = cm
				idx = i
			}
		}
		card := s.Hand[idx]
		discardName = card.DisplayName()
		// Route through DiscardCard for §702.34a Madness / §702.187
		// Mayhem / Necropotence reroute / card_discarded trigger /
		// Turn.Discarded stat. Pre-r60-normalize Emet-Selch direct-
		// MoveCarded the highest-CMC card to graveyard, silently
		// bypassing every one of those — a Madness card discarded by
		// Emet-Selch should be Madness-cast, not graveyarded.
		gameengine.DiscardCard(gs, card, seat)
		gs.LogEvent(gameengine.Event{
			Kind:   "discard",
			Seat:   seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":   slug,
				"card":   discardName,
				"reason": reason,
			},
		})
	}
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"reason":  reason,
		"drawn":   drawnName,
		"discard": discardName,
	})
}

func emetSelchUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "emet_selch_transform_check"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if len(seat.Graveyard) < 14 {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"graveyard_size": len(seat.Graveyard),
	})
	emitPartial(gs, "emet_selch_transform", perm.Card.DisplayName(),
		"transform_to_hades_back_face_not_implemented")
}
