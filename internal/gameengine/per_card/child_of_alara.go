package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChildOfAlara wires Child of Alara.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Trample
//	When Child of Alara dies, destroy all nonland permanents.
//	They can't be regenerated.
//
// Implementation:
//   - Trample — handled by the AST keyword pipeline.
//   - OnTrigger("creature_dies"): the death trigger is gated on the dying
//     card being Child of Alara itself (ctx["card"] name match). When it
//     fires, snapshot all nonland permanents across every seat and destroy
//     them via DestroyPermanent (respects indestructible, fires dies/LTB
//     triggers). "Can't be regenerated" is a no-regeneration clause; since
//     the engine does not model regeneration shields, this is a no-op gap
//     flagged via emitPartial.
//   - The self-death trigger is also dispatched by the AST path
//     (fireSelfZoneChangeTriggers) from the child's own "die" trigger in
//     the parsed AST. This per-card handler supplements that path for
//     coverage and explicit logging.
//
// Note: the observer pattern (OnTrigger scanning the battlefield) means
// this handler fires when a creature named "Child of Alara" dies while the
// observing perm is on the battlefield. In the common case where there is
// only one Child of Alara in play, the self-death trigger is driven by the
// AST; this handler provides belt-and-suspenders coverage and an explicit
// emit record.
func registerChildOfAlara(r *Registry) {
	r.OnTrigger("Child of Alara", "creature_dies", childOfAlaraDies)
}

func childOfAlaraDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "child_of_alara_wipe"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: the dying card must be Child of Alara itself.
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}
	if dyingCard.DisplayName() != "Child of Alara" {
		return
	}

	// "They can't be regenerated" — regeneration is not modeled; flag gap.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"no_regeneration_clause_not_enforced")

	// Snapshot all nonland permanents across all seats before destroying.
	// We snapshot first so that mid-loop removal doesn't affect iteration.
	var toDestroy []*gameengine.Permanent
	for _, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "land") {
				continue
			}
			toDestroy = append(toDestroy, p)
		}
	}

	destroyed := 0
	for _, p := range toDestroy {
		if gameengine.DestroyPermanent(gs, p, nil) {
			destroyed++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"destroyed": destroyed,
		"rule":      "when_child_of_alara_dies",
	})
}
