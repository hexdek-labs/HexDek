package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVihaanGoldwaker wires Vihaan, Goldwaker.
//
// Oracle text (Scryfall, verified 2026-05-30, {1}{R}{W}{B}, 3/3
// Legendary Creature — Human Warlock):
//
//	Other outlaws you control have vigilance and haste. (Assassins,
//	Mercenaries, Pirates, Rogues, and Warlocks are outlaws.)
//	At the beginning of combat on your turn, you may have Treasures you
//	control become 3/3 Construct Assassin artifact creatures in addition
//	to their other types until end of turn.
//
// Implementation (R60 stub sweep batch 2):
//   - "combat_begin" on Vihaan's controller's turn: animate every Treasure
//     we control as a 3/3 Construct Assassin until end of turn (already
//     wired). Cleanup at next_end_step restores original types.
//   - Outlaw anthem (this batch): Abzan-Falconer-style flag sweep stamps
//     Flags["kw:vigilance"] + Flags["kw:haste"] on every other outlaw
//     creature controlled by Vihaan's controller, marking each with
//     Flags["vihaan_goldwaker_grant"] so LTB can strip cleanly without
//     clobbering native AST vigilance/haste. Triggered on ETB (initial
//     sweep), permanent_etb (newly-entered outlaws), and stripped on
//     Vihaan's own permanent_ltb. The animated-treasures path (now
//     creature-type Assassin while pumped) also picks up the grant for
//     the combat phase it was animated in — outlaw subtype Assassin
//     was added by the animate step, so the very next permanent_etb /
//     sweep tick catches it.
func registerVihaanGoldwaker(r *Registry) {
	r.OnETB("Vihaan, Goldwaker", vihaanETBGrantSweep)
	r.OnTrigger("Vihaan, Goldwaker", "combat_begin", vihaanCombatBegin)
	r.OnTrigger("Vihaan, Goldwaker", "permanent_etb", vihaanPermETBSweep)
	r.OnTrigger("Vihaan, Goldwaker", "permanent_ltb", vihaanOwnLTBStrip)
}

func vihaanETBGrantSweep(gs *gameengine.GameState, perm *gameengine.Permanent) {
	vihaanGrantSweep(gs, perm)
}

func vihaanPermETBSweep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	vihaanGrantSweep(gs, perm)
}

func vihaanOwnLTBStrip(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vihaan_goldwaker_grant_strip"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stripped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		if p.Flags["vihaan_goldwaker_grant"] != 1 {
			continue
		}
		if p.Flags["vihaan_grant_vig"] == 1 {
			delete(p.Flags, "kw:vigilance")
			delete(p.Flags, "vihaan_grant_vig")
		}
		if p.Flags["vihaan_grant_haste"] == 1 {
			delete(p.Flags, "kw:haste")
			delete(p.Flags, "vihaan_grant_haste")
		}
		delete(p.Flags, "vihaan_goldwaker_grant")
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"stripped": stripped,
	})
}

func vihaanGrantSweep(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "vihaan_goldwaker_outlaw_anthem"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	granted := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !gameengine.IsOutlaw(p.Card) {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["vihaan_goldwaker_grant"] == 1 {
			continue
		}
		// Honor native keywords: don't stamp our flag over a card that
		// already has vigilance/haste via AST, else LTB strip would
		// remove the native keyword on the way out.
		needVig := !p.HasKeyword("vigilance")
		needHaste := !p.HasKeyword("haste")
		if !needVig && !needHaste {
			continue
		}
		if needVig {
			p.Flags["kw:vigilance"] = 1
			p.Flags["vihaan_grant_vig"] = 1
		}
		if needHaste {
			p.Flags["kw:haste"] = 1
			p.Flags["vihaan_grant_haste"] = 1
		}
		p.Flags["vihaan_goldwaker_grant"] = 1
		granted++
	}
	if granted > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"granted": granted,
		})
	}
}

func vihaanCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vihaan_goldwaker_animate_treasures"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	type savedTypes struct {
		perm *gameengine.Permanent
		old  []string
	}
	var saved []savedTypes
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isTreasure := false
		for _, t := range p.Card.Types {
			if t == "treasure" || t == "Treasure" {
				isTreasure = true
				break
			}
		}
		if !isTreasure {
			continue
		}
		// Snapshot original types then add creature/construct/assassin.
		old := append([]string(nil), p.Card.Types...)
		saved = append(saved, savedTypes{perm: p, old: old})
		needAdd := []string{"creature", "construct", "assassin"}
		for _, want := range needAdd {
			has := false
			for _, t := range p.Card.Types {
				if t == want {
					has = true
					break
				}
			}
			if !has {
				p.Card.Types = append(p.Card.Types, want)
			}
		}
		// Set base P/T to 3/3 via modification.
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     3 - p.Card.BasePower,
			Toughness: 3 - p.Card.BaseToughness,
			Duration:  "until_end_of_turn",
		})
		count++
	}
	if count == 0 {
		return
	}
	gs.InvalidateCharacteristicsCache()

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			for _, s := range saved {
				if s.perm == nil || s.perm.Card == nil {
					continue
				}
				s.perm.Card.Types = s.old
			}
			gs.InvalidateCharacteristicsCache()
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"animated": count,
	})
}
