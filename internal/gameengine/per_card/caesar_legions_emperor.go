package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaesarLegionsEmperor wires Caesar, Legion's Emperor.
//
// Implemented oracle text (per task spec):
//
//	Whenever you attack, create a 1/1 red Soldier creature token
//	tapped and attacking.
//	Whenever a nontoken creature dies during combat, draw a card.
//
// Note: The current Scryfall printing of "Caesar, Legion's Emperor" reads
// "Whenever you attack, you may sacrifice another creature. When you do,
// choose two — [tokens / draw + lose 1 / Caesar damages opponent]." The
// simplified spec above is what this engine implements; if the printed
// version is later wired up, replace this handler.
//
// Implementation notes:
//   - "Whenever you attack" fires once per attack declaration. The engine
//     emits creature_attacks per attacker, so we de-dupe per (turn,
//     controller) flag.
//   - The token enters tapped and attacking the same defender as the
//     attacker that triggered the ability. The token is given haste so it
//     does not need vigilance to be a meaningful attacker the turn it is
//     created, mirroring the "tapped and attacking" idiom (CR §506.3).
//   - The combat-death draw uses creature_dies, gated on (a) the dying
//     card was a nontoken creature, and (b) gs.Phase == "combat".
func registerCaesarLegionsEmperor(r *Registry) {
	r.OnTrigger("Caesar, Legion's Emperor", "creature_attacks", caesarAttackToken)
	r.OnTrigger("Caesar, Legion's Emperor", "creature_dies", caesarCombatDeathDraw)
}

func caesarAttackToken(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "caesar_legions_emperor_attack_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupe := "caesar_attack_token_t" + strconv.Itoa(gs.Turn)
	if perm.Flags[dedupe] == 1 {
		return
	}
	perm.Flags[dedupe] = 1

	defenderSeat := -1
	if atk != nil {
		if d, ok := gameengine.AttackerDefender(atk); ok {
			defenderSeat = d
		}
	}
	if defenderSeat < 0 {
		// Fall back to first living opponent.
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	tokenCard := &gameengine.Card{
		Name:          "Soldier Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "soldier", "pip:R"},
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
		Flags:         map[string]int{"attacking": 1, "caesar_attack_token": 1},
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
			"token":         "Soldier Token",
			"reason":        "caesar_legions_emperor_attack",
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

func caesarCombatDeathDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "caesar_legions_emperor_combat_death_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if gs.Phase != "combat" {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		dead, _ := ctx["perm"].(*gameengine.Permanent)
		if dead != nil {
			card = dead.Card
		}
	}
	if card == nil {
		return
	}
	if !cardHasType(card, "creature") {
		return
	}
	if cardHasType(card, "token") {
		return
	}

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"died":       card.DisplayName(),
		"drawn_card": drawnName,
	})
}
