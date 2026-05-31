package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYunaGrandSummoner wires Yuna, Grand Summoner.
//
// Oracle text (Scryfall, verified 2026-05-30, {2}{G}{W}, 3/3
// Legendary Creature — Human Summoner):
//
//	Grand Summon — {T}: Add one mana of any color. When you next cast a
//	creature spell this turn, that creature enters with two additional
//	+1/+1 counters on it.
//	Whenever another permanent you control is put into a graveyard from
//	the battlefield, if it had one or more counters on it, you may put
//	that number of +1/+1 counters on target creature.
//
// Implementation (R60 stub sweep batch 2):
//   - "permanent_ltb" (existing): if a non-Yuna permanent controlled by
//     Yuna's controller dies with one or more counters on it, copy that
//     count of +1/+1 counters to our highest-power creature.
//   - OnActivated (this batch): tap, add one mana of any color (use
//     gameengine.AddMana with "any" — the cost-pipeline resolves it
//     to the AI's preferred color downstream), then stamp
//     seat.Flags["yuna_next_creature_bonus_turn"] = gs.Turn+1 so the
//     creature_etb handler below can recognize "this turn's first
//     post-grant creature spell." +1 sentinel because 0 is the empty
//     map default.
//   - permanent_etb (this batch): if the entering permanent is a
//     creature controlled by Yuna's controller AND seat flag is set
//     for the current turn, add 2 +1/+1 counters and clear the flag
//     (one-shot per activation). Self-ETB excluded.
func registerYunaGrandSummoner(r *Registry) {
	r.OnTrigger("Yuna, Grand Summoner", "permanent_ltb", yunaPermLTB)
	r.OnTrigger("Yuna, Grand Summoner", "permanent_etb", yunaPermETBCounterBonus)
	r.OnActivated("Yuna, Grand Summoner", yunaActivate)
}

func yunaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yuna_grand_summon_mana"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	gameengine.AddMana(gs, seat, "any", 1, src.Card.DisplayName())
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["yuna_next_creature_bonus_turn"] = gs.Turn + 1
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"bonus_armed":  true,
		"bonus_for_turn": gs.Turn,
	})
}

func yunaPermETBCounterBonus(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yuna_next_creature_counter_bonus"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil {
		enteringPerm, _ = ctx["permanent"].(*gameengine.Permanent)
	}
	if enteringPerm == nil || enteringPerm == perm || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	if !enteringPerm.IsCreature() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags["yuna_next_creature_bonus_turn"] != gs.Turn+1 {
		return
	}
	enteringPerm.AddCounter("+1/+1", 2)
	delete(seat.Flags, "yuna_next_creature_bonus_turn")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"target":    enteringPerm.Card.DisplayName(),
		"counters":  2,
	})
}

func yunaPermLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yuna_perm_with_counters_dies"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dest, _ := ctx["to_zone"].(string)
	if dest != "graveyard" {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if dyingPerm == nil || dyingPerm == perm {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	totalCounters := 0
	if dyingPerm.Counters != nil {
		for _, n := range dyingPerm.Counters {
			if n > 0 {
				totalCounters += n
			}
		}
	}
	if totalCounters <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}
	if best == nil {
		return
	}
	best.AddCounter("+1/+1", totalCounters)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   best.Card.DisplayName(),
		"counters": totalCounters,
	})
}
