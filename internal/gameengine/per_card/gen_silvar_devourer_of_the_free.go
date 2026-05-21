package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSilvarDevourerOfTheFree wires Silvar, Devourer of the Free.
//
// Oracle text (Scryfall, verified):
//
//	Partner with Trynn, Champion of Freedom (When this creature
//	enters, target player may put Trynn into their hand from their
//	library, then shuffle.)
//	Menace
//	Sacrifice a Human: Put a +1/+1 counter on Silvar. It gains
//	indestructible until end of turn.
//
// Implementation (R49 stub port):
//   - Partner-with ETB tutor: scan controller's library for "Trynn,
//     Champion of Freedom"; on hit, move to hand and mark library
//     dirty so the engine's shuffle pass picks it up. The "target
//     player may" clause defaults to self — the wording lets the
//     opponent decline, and no opponent would tutor a card into a
//     teammate's hand. emitPartial for the multiplayer-give path.
//   - Menace: AST keyword pipeline.
//   - Activated sac-cost: pick the smallest-PT Human on the
//     controller's battlefield (preserve big Humans). The cost can't
//     pay itself (Silvar isn't a Human in printed type) — defensive
//     skip for src and for non-Humans.
//   - Effect: +1/+1 counter on Silvar + indestructible UEOT via
//     Flags["kw:indestructible"] + next_end_step delayed cleanup.
func registerSilvarDevourerOfTheFree(r *Registry) {
	r.OnETB("Silvar, Devourer of the Free", silvarPartnerETB)
	r.OnActivated("Silvar, Devourer of the Free", silvarDevourerOfTheFreeActivate)
}

func silvarPartnerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "silvar_partner_etb_tutor"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	for _, c := range seat.Library {
		if c != nil && c.DisplayName() == "Trynn, Champion of Freedom" {
			gameengine.MoveCard(gs, c, seatIdx, "library", "hand", "silvar_partner_tutor")
			emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
				"seat":   seatIdx,
				"tutored": "Trynn, Champion of Freedom",
			})
			emitPartial(gs, slug, perm.Card.DisplayName(),
				"target_player_choice_defaults_to_self_no_decline_pathway")
			return
		}
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"trynn_not_in_library_no_tutor_target")
}

func silvarDevourerOfTheFreeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "silvar_devourer_sac_human"
	if gs == nil || src == nil || src.Card == nil {
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

	// Find smallest Human to sacrifice — Silvar himself is a
	// Wolf/Werewolf, not a Human, so he's never eligible.
	var victim *gameengine.Permanent
	bestPT := 1 << 30
	bestTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "human") {
			continue
		}
		pt := gs.PowerOf(p) + gs.ToughnessOf(p)
		if pt < bestPT || (pt == bestPT && p.Timestamp > bestTS) {
			bestPT = pt
			bestTS = p.Timestamp
			victim = p
		}
	}
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_human_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "silvar_sac_cost")

	src.AddCounter("+1/+1", 1)
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:indestructible"] = 1
	captured := src
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:indestructible")
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seatIdx,
		"sacrificed": victimName,
	})
}
