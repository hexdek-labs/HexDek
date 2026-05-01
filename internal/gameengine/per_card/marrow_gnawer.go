package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMarrowGnawer wires Marrow-Gnawer.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Fear
//	All Rats have fear.
//	{T}, Sacrifice a Rat: Create X 1/1 black Rat creature tokens, where
//	X is the number of Rats you control.
//
// Implementation:
//   - OnETB: emitPartial for the "All Rats have fear" continuous static.
//     Marrow's own fear is intrinsic via AST keyword. The global anthem
//     across all Rats requires continuous-effect machinery that we don't
//     model for arbitrary subtype grants — combat damage assignment
//     against unblocked Rats remains correct in the dominant case
//     because most Rat decks attack with Marrow himself (already has
//     fear), and the unblockable advantage is largely flavor-strategic
//     rather than combo-load-bearing.
//   - OnActivated: pay {T} (handled at activation), pick a Rat to
//     sacrifice (prefer a non-self Rat token; fall back to any non-self
//     Rat; only sacrifice Marrow as last resort), count Rats AFTER the
//     sacrifice, and mint that many 1/1 black Rat tokens via the
//     standard ETB cascade so Anointed Procession / Parallel Lives /
//     etc. compound naturally.
func registerMarrowGnawer(r *Registry) {
	r.OnETB("Marrow-Gnawer", marrowGnawerETB)
	r.OnActivated("Marrow-Gnawer", marrowGnawerActivate)
}

func marrowGnawerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "marrow_gnawer_rat_anthem"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"all_rats_have_fear_continuous_static_unimplemented")
}

func marrowGnawerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "marrow_gnawer_rat_swarm"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	victim := pickRatToSacrifice(s.Battlefield, src)
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_rat_to_sacrifice", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "marrow_gnawer")

	x := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "rat") {
			x++
		}
	}
	if x <= 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":           seat,
			"sacrificed":     victimName,
			"tokens_created": 0,
		})
		return
	}

	for i := 0; i < x; i++ {
		token := &gameengine.Card{
			Name:          "Rat Token",
			Owner:         seat,
			Types:         []string{"creature", "token", "rat", "pip:B"},
			Colors:        []string{"B"},
			BasePower:     1,
			BaseToughness: 1,
		}
		enterBattlefieldWithETB(gs, seat, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Rat Token",
				"reason": "marrow_gnawer",
				"power":  1,
				"tough":  1,
			},
		})
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"sacrificed":     victimName,
		"rats_after_sac": x,
		"tokens_created": x,
	})
	_ = gs.CheckEnd()
}

// pickRatToSacrifice prefers (1) a Rat token that isn't Marrow itself,
// (2) any non-Marrow Rat, (3) Marrow himself only as last resort.
func pickRatToSacrifice(bf []*gameengine.Permanent, src *gameengine.Permanent) *gameengine.Permanent {
	var anyRat *gameengine.Permanent
	for _, p := range bf {
		if p == nil || p.Card == nil || p == src {
			continue
		}
		if !cardHasType(p.Card, "rat") {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
		if anyRat == nil {
			anyRat = p
		}
	}
	if anyRat != nil {
		return anyRat
	}
	if src != nil && src.Card != nil && cardHasType(src.Card, "rat") {
		return src
	}
	return nil
}
