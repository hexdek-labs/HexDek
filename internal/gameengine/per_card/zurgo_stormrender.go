package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZurgoStormrender wires Zurgo Stormrender (Tarkir: Dragonstorm
// Commander).
//
// Oracle text:
//
//	Mobilize 1 (Whenever this creature attacks, create a tapped and
//	attacking 1/1 red Warrior creature token. Sacrifice it at the
//	beginning of the next end step.)
//	Whenever a creature token you control leaves the battlefield, draw
//	a card if it was attacking. Otherwise, each opponent loses 1 life.
//
// Implementation:
//   - Mobilize 1 — engine has only a stub for §702.181, so we model the
//     keyword inline. On creature_attacks where the attacker is Zurgo, we
//     create a 1/1 red Warrior token entering tapped and attacking the
//     same defender, flagged "zurgo_mobilize_sac" so the end-step pass
//     sacrifices it. Token also gets a "zurgo_was_attacking" Flag we read
//     during LTB (the engine wipes Flags["attacking"] in EndOfCombatStep
//     before the end-step phase, so we need a sticky marker).
//   - End-step trigger — sacrifices every flagged mobilize token Zurgo's
//     controller still controls. The sacrifice fires permanent_ltb, which
//     bounces back into the LTB handler below.
//   - LTB trigger — when a creature token controlled by Zurgo's controller
//     leaves the battlefield, drawn-vs-drain is decided by the
//     "zurgo_was_attacking" sticky flag (mobilize tokens) OR the engine's
//     live attacking flag (combat-time deaths). Otherwise, each living
//     opponent loses 1 life.
func registerZurgoStormrender(r *Registry) {
	r.OnTrigger("Zurgo Stormrender", "creature_attacks", zurgoMobilizeOnAttack)
	r.OnTrigger("Zurgo Stormrender", "end_step", zurgoEndStepSacMobilize)
	r.OnTrigger("Zurgo Stormrender", "permanent_ltb", zurgoTokenLeaves)
}

func zurgoMobilizeOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zurgo_stormrender_mobilize"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}

	defenderSeat := -1
	if d, ok := gameengine.AttackerDefender(perm); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_defender", nil)
		return
	}

	tokenCard := &gameengine.Card{
		Name:          "Warrior Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "warrior", "pip:R"},
		Colors:        []string{"R"},
		BasePower:     1,
		BaseToughness: 1,
	}
	token := &gameengine.Permanent{
		Card:          tokenCard,
		Controller:    perm.Controller,
		Owner:         perm.Controller,
		Tapped:        true,
		SummoningSick: false,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags: map[string]int{
			"attacking":           1,
			"zurgo_was_attacking": 1,
			"zurgo_mobilize_sac":  1,
		},
	}
	gs.Seats[perm.Controller].Battlefield = append(gs.Seats[perm.Controller].Battlefield, token)
	gameengine.SetAttackerDefender(token, defenderSeat)
	gameengine.RegisterReplacementsForPermanent(gs, token)
	gameengine.FirePermanentETBTriggers(gs, token)

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":         "Warrior Token",
			"reason":        "zurgo_mobilize",
			"power":         1,
			"tough":         1,
			"defender_seat": defenderSeat,
			"tapped":        true,
			"attacking":     true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
	})
}

func zurgoEndStepSacMobilize(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zurgo_stormrender_mobilize_endstep_sac"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	var victims []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		if p.Flags["zurgo_mobilize_sac"] == 1 {
			victims = append(victims, p)
		}
	}
	if len(victims) == 0 {
		return
	}
	sacced := make([]string, 0, len(victims))
	for _, v := range victims {
		name := ""
		if v.Card != nil {
			name = v.Card.DisplayName()
		}
		sacced = append(sacced, name)
		gameengine.SacrificePermanent(gs, v, "zurgo_mobilize_endstep")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": sacced,
		"count":      len(sacced),
	})
}

func zurgoTokenLeaves(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zurgo_stormrender_token_ltb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leavingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	leavingCard, _ := ctx["card"].(*gameengine.Card)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	if leavingPerm == perm {
		return
	}
	if leavingCard == nil && leavingPerm != nil {
		leavingCard = leavingPerm.Card
	}
	if leavingCard == nil {
		return
	}
	if !cardHasType(leavingCard, "creature") {
		return
	}
	if !cardHasType(leavingCard, "token") {
		return
	}

	wasAttacking := false
	if leavingPerm != nil && leavingPerm.Flags != nil {
		if leavingPerm.Flags["attacking"] > 0 || leavingPerm.Flags["zurgo_was_attacking"] > 0 {
			wasAttacking = true
		}
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	if wasAttacking {
		drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		drewName := ""
		if drawn != nil {
			drewName = drawn.DisplayName()
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"token":  leavingCard.DisplayName(),
			"mode":   "draw",
			"card":   drewName,
		})
		return
	}

	hits := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		s.Life--
		hits++
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Target: -1,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"amount": -1,
				"cause":  "zurgo_stormrender_drain",
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"token":          leavingCard.DisplayName(),
		"mode":           "drain",
		"opponents_hit":  hits,
		"opponents_lost": strconv.Itoa(hits) + " life",
	})
}
