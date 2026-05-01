package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLathrilBladeOfTheElves wires Lathril, Blade of the Elves
// (Foundations reprint of Kaldheim Commander, verified Scryfall 2026-05-01).
//
// Oracle text:
//
//	{2}{B}{G}, 2/3 Legendary Creature — Elf Noble
//	Menace
//	Whenever Lathril deals combat damage to a player, create that many
//	  1/1 green Elf Warrior creature tokens.
//	{T}, Tap ten untapped Elves you control: Each opponent loses 10 life
//	  and you gain 10 life.
//
// Implementation:
//   - Menace — intrinsic AST keyword, intrinsic to combat resolution.
//   - "combat_damage_player": gates on (a) source seat == Lathril's
//     controller, (b) source_card == "Lathril, Blade of the Elves"
//     (the trigger only fires for Lathril herself, not any creature).
//     Mints `amount` 1/1 green Elf Warrior tokens via the standard ETB
//     cascade so token-doubler effects (Anointed Procession, Parallel
//     Lives, Doubling Season) interact correctly.
//   - OnActivated(0): Tap ten untapped Elves cost. The engine pays the
//     activation cost (the {T} on Lathril) before invoking the handler;
//     we then pick ten untapped Elves controlled by the activator
//     (Lathril counts — she's an Elf), tap them, and apply the drain:
//     each living opponent loses 10 life and Lathril's controller gains
//     10 life. Failure modes (fewer than ten untapped Elves available)
//     emitFail and bail without paying the additional cost.
func registerLathrilBladeOfTheElves(r *Registry) {
	r.OnTrigger("Lathril, Blade of the Elves", "combat_damage_player", lathrilCombatDamage)
	r.OnActivated("Lathril, Blade of the Elves", lathrilDrain)
}

func lathrilCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lathril_combat_damage_elf_tokens"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	amount, _ := ctx["amount"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	// Trigger only fires off Lathril herself.
	if !strings.EqualFold(sourceName, perm.Card.DisplayName()) {
		return
	}
	if amount <= 0 {
		return
	}

	for i := 0; i < amount; i++ {
		token := &gameengine.Card{
			Name:          "Elf Warrior Token",
			Owner:         perm.Controller,
			Types:         []string{"creature", "token", "elf", "warrior", "pip:G"},
			Colors:        []string{"G"},
			BasePower:     1,
			BaseToughness: 1,
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Elf Warrior Token",
				"reason": "lathril_combat_damage",
				"power":  1,
				"tough":  1,
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"damage_dealt":   amount,
		"tokens_created": amount,
	})
}

func lathrilDrain(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lathril_tap_ten_elves_drain"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Find ten untapped Elves the activator controls. Lathril is an Elf
	// so she's eligible — but the engine pays the {T} activation cost
	// first, which leaves Lathril tapped, so she's no longer "untapped"
	// when this handler runs. We therefore look only at OTHER untapped
	// Elves to satisfy the additional-cost.
	var elves []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p == src {
			continue
		}
		if !p.IsCreature() || p.Tapped {
			continue
		}
		if !cardHasType(p.Card, "elf") {
			continue
		}
		elves = append(elves, p)
		if len(elves) >= 10 {
			break
		}
	}
	if len(elves) < 10 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_ten_untapped_elves", map[string]interface{}{
			"seat":      seat,
			"available": len(elves),
		})
		return
	}
	for _, p := range elves {
		p.Tapped = true
	}

	drained := 0
	for i, opp := range gs.Seats {
		if opp == nil || i == seat || opp.Lost {
			continue
		}
		opp.Life -= 10
		drained++
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   seat,
			Target: i,
			Source: src.Card.DisplayName(),
			Amount: -10,
			Details: map[string]interface{}{
				"slug":   slug,
				"reason": "lathril_drain",
			},
		})
	}
	gameengine.GainLife(gs, seat, 10, src.Card.DisplayName())

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"elves_tapped":      len(elves),
		"opponents_drained": drained,
		"life_gained":       10,
	})
	_ = gs.CheckEnd()
}
