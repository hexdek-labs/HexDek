package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEzrimAgencyChief wires Ezrim, Agency Chief.
//
// Oracle text (MKM, {1}{W}{W}{U}{U}, 5/5):
//
//	Flying
//	When Ezrim enters, investigate twice.
//	{1}, Sacrifice an artifact: Ezrim gains your choice of vigilance,
//	lifelink, or hexproof until end of turn.
//
// Implementation:
//   - ETB creates two Clue tokens (the standard investigate effect).
//   - Activated grant ({1}, sacrifice an artifact): gates on
//     `seat.ManaPool >= 1` and the existence of a non-Ezrim artifact to
//     sacrifice. Picks lifelink (the highest-value of the three keyword
//     options for combat math). Sets `kw:lifelink` on Ezrim and queues
//     a delayed cleanup at end of turn to enforce the duration.
func registerEzrimAgencyChief(r *Registry) {
	r.OnETB("Ezrim, Agency Chief", ezrimETB)
	// ezrimETB FULLY implements the "When Ezrim enters, investigate twice"
	// trigger (two Clue tokens). Declare ownership so the engine's ETB
	// dispatch skips the generic AST investigate-twice push — otherwise BOTH
	// fire and Ezrim makes 4 Clues instead of 2 (live grinder OUTCOME
	// over-count). Mirrors every other OnETB handler that reimplements a
	// parsed ETB trigger.
	r.OwnsETBTrigger("Ezrim, Agency Chief")
	r.OnActivated("Ezrim, Agency Chief", ezrimSacGrantKeyword)
}

func ezrimETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ezrim_etb_investigate_twice"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.CreateClueToken(gs, seat)
	gameengine.CreateClueToken(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "investigate",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: 2,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"clues": 2,
	})
}

func ezrimSacGrantKeyword(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ezrim_sac_artifact_grant_keyword"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// Find an artifact to sacrifice (prefer cheapest non-Ezrim, prefer
	// non-creature so we don't tank board state).
	var sac *gameengine.Permanent
	bestCMC := 1<<31 - 1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		cmc := cardCMC(p.Card)
		if p.IsCreature() {
			cmc += 100 // de-prioritize artifact creatures
		}
		if cmc < bestCMC {
			bestCMC = cmc
			sac = p
		}
	}
	if sac == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_to_sacrifice", nil)
		return
	}
	if !payDefensiveManaCost(src, seat, abilityIdx, 1) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  1,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	gameengine.SacrificePermanent(gs, sac, "ezrim_activation_cost")
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:lifelink"] = 1
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if src.Flags != nil {
				delete(src.Flags, "kw:lifelink")
			}
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"sacrificed": sac.Card.DisplayName(),
		"granted":   "lifelink",
	})
}
