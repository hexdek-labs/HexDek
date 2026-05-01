package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNajeelaBladeBlossom wires Najeela, the Blade-Blossom.
//
// Oracle text (Scryfall):
//
//	Whenever a Warrior attacks, create a 1/1 white Warrior creature
//	token tapped and attacking.
//	{W}{U}{B}{R}{G}: Untap each attacking creature. They gain trample,
//	lifelink, and haste until end of turn. After this combat phase,
//	there is an additional combat phase. Activate only during combat.
//
// Implementation:
//   - "creature_attacks" trigger: fires per declared attacker. If the
//     attacker has Warrior subtype and is controlled by Najeela's
//     controller, mint a 1/1 white Warrior token that enters tapped and
//     attacking the same defender. Najeela attacking herself triggers it
//     too (she is a Warrior).
//   - OnActivated(0): WUBRG cost is paid via the AST cost path in
//     activation.go. Untap each attacking creature Najeela's controller
//     controls, give them trample+lifelink+haste UEOT (modeled via
//     keyword grants on perm.Modifications), and bump
//     gs.PendingExtraCombats so the turn loop adds one more combat.
func registerNajeelaBladeBlossom(r *Registry) {
	r.OnTrigger("Najeela, the Blade-Blossom", "creature_attacks", najeelaWarriorAttacks)
	r.OnActivated("Najeela, the Blade-Blossom", najeelaActivate)
}

func najeelaWarriorAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "najeela_warrior_attack_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil {
		return
	}
	// "Whenever a Warrior attacks" — any Warrior on the battlefield, not
	// only Najeela's controller's. Token goes to Najeela's controller.
	isWarrior := false
	for _, t := range atk.Card.Types {
		if strings.EqualFold(t, "warrior") {
			isWarrior = true
			break
		}
	}
	if !isWarrior {
		return
	}

	defenderSeat := -1
	if d, ok := gameengine.AttackerDefender(atk); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	tokenCard := &gameengine.Card{
		Name:          "Warrior Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "warrior", "pip:W"},
		Colors:        []string{"W"},
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
		Flags:         map[string]int{"attacking": 1, "najeela_attack_token": 1},
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
			"reason":        "najeela_warrior_attack",
			"power":         1,
			"tough":         1,
			"defender_seat": defenderSeat,
			"tapped":        true,
			"attacking":     true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker":      atk.Card.DisplayName(),
		"defender_seat": defenderSeat,
	})
}

func najeelaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "najeela_extra_combat"
	if gs == nil || src == nil {
		return
	}
	if gs.Phase != "combat" {
		emitFail(gs, slug, src.Card.DisplayName(), "not_combat_phase", map[string]interface{}{
			"phase": gs.Phase,
		})
		return
	}

	untapped := 0
	buffed := 0
	captured := []*gameengine.Permanent{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if !p.IsAttacking() {
				continue
			}
			if p.Tapped {
				p.Tapped = false
				untapped++
			}
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:trample"] = 1
			p.Flags["kw:lifelink"] = 1
			p.Flags["kw:haste"] = 1
			p.SummoningSick = false
			captured = append(captured, p)
			buffed++
		}
	}
	if len(captured) > 0 {
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: src.Controller,
			SourceCardName: src.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				for _, p := range captured {
					if p == nil || p.Flags == nil {
						continue
					}
					delete(p.Flags, "kw:trample")
					delete(p.Flags, "kw:lifelink")
					delete(p.Flags, "kw:haste")
				}
			},
		})
	}

	gs.PendingExtraCombats++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_combat",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "najeela_activated",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"untapped":       untapped,
		"buffed":         buffed,
		"extra_combats":  gs.PendingExtraCombats,
	})
}
