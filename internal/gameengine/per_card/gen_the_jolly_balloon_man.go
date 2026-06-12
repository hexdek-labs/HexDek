package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheJollyBalloonMan wires The Jolly Balloon Man.
//
// Oracle text:
//
//	Haste
//	{1}, {T}: Create a token that's a copy of another target creature you
//	control, except it's a 1/1 red Balloon creature in addition to its
//	other colors and types and it has flying and haste. Sacrifice it at
//	the beginning of the next end step. Activate only as a sorcery.
//
// Implementation:
//   - Activated ability: gates on `seat.ManaPool >= 1`, `!src.Tapped`,
//     and sorcery speed (active player + empty stack + main phase).
//     Picks the highest-power non-self controlled creature as the copy
//     target so the 1/1 floor is upside in marginal cases.
//   - Token created with type tags "balloon", "pip:R", `kw:flying`,
//     `kw:haste`. Sacrifice queued via DelayedTrigger at next end step.
func registerTheJollyBalloonMan(r *Registry) {
	// R53 batch O: dropped the OnETB hook. The previous body emitted a
	// misleading per_card_partial / parser_gap event every ETB saying
	// "haste static handled by AST engine; per_card stub for
	// registration tracking" — but haste IS handled by the AST keyword
	// pipeline (not a per_card gap), and "registration tracking" is
	// already satisfied via OnActivated below. The emit polluted the
	// parser_gap audit channel with a false gap signal on every
	// Balloon Man ETB.
	r.OnActivated("The Jolly Balloon Man", theJollyBalloonManCopy)
}

func theJollyBalloonManCopy(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_jolly_balloon_man_copy"
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
	// Cost gate 1: sorcery speed.
	if !isSorcerySpeed(gs, seatIdx) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}
	// Cost gate 2: tap (already-tapped activation is illegal).
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	// Cost gate 3: {1} mana.
	if !payDefensiveManaCost(src, seat, abilityIdx, 1) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  1,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	// Pick the best another-target creature.
	var pick *gameengine.Permanent
	bestPower := -1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			pick = p
		}
	}
	if pick == nil {
		// Refund the mana — cost can't be paid without a legal target.
		seat.ManaPool += 1
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_creature", nil)
		return
	}
	src.Tapped = true
	tokenCard := &gameengine.Card{
		Name:          pick.Card.DisplayName() + " (Balloon token)",
		Owner:         seatIdx,
		Types:         append([]string{"token", "creature", "balloon", "pip:R"}, pick.Card.Types...),
		BasePower:     1,
		BaseToughness: 1,
	}
	tokenPerm := &gameengine.Permanent{
		Card:       tokenCard,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{"kw:flying": 1, "kw:haste": 1},
	}
	seat.Battlefield = append(seat.Battlefield, tokenPerm)
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.MoveCard(gs, tokenCard, seatIdx, "battlefield", "graveyard", "balloon_eos_sacrifice")
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"copied":   pick.Card.DisplayName(),
		"token_pt": "1/1",
	})
}
