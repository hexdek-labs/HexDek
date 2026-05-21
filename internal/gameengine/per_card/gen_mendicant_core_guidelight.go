package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMendicantCoreGuidelight wires Mendicant Core, Guidelight.
//
// Oracle text:
//
//	Mendicant Core's power is equal to the number of artifacts you
//	control.
//	Start your engines! (If you have no speed, it starts at 1. It
//	increases once on each of your turns when an opponent loses life.
//	Max speed is 4.)
//	Max speed — Whenever you cast an artifact spell, you may pay {1}.
//	If you do, copy it.
//
// Implementation:
//   - "Power = artifact count" CDA: recomputed in
//     mendicantRefreshPower at ETB and on every begin_combat /
//     spell_cast trigger fire. Sets perm.Card.BasePower; the engine's
//     layered characteristics calculator picks it up via
//     InvalidateCharacteristicsCache.
//   - "Start your engines" speed system: tracked on perm.Flags["speed"]
//     (1..4). begin_combat checks if any opponent's life is lower
//     than at the start of the turn (we snapshot it on
//     active-seat-turn begin) and bumps speed.
//   - "spell_cast" trigger: at max speed (4), gate to caster ==
//     controller AND cast spell is artifact, then emit a copy event.
//
// emitPartial: actual spell-copy + token-creation routing requires
// the engine's spell-copy pipeline; we surface the trigger so any
// downstream observer can dispatch.
func registerMendicantCoreGuidelight(r *Registry) {
	r.OnETB("Mendicant Core, Guidelight", mendicantETB)
	r.OnTrigger("Mendicant Core, Guidelight", "combat_begin", mendicantBeginCombat)
	r.OnTrigger("Mendicant Core, Guidelight", "upkeep_controller", mendicantSpeedTick)
	r.OnTrigger("Mendicant Core, Guidelight", "spell_cast", mendicantSpellCast)
}

func mendicantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Speed starts at 1 per the keyword.
	if perm.Flags["speed"] <= 0 {
		perm.Flags["speed"] = 1
	}
	// R55: power = artifacts you control. Layer 7b dynamic power-only.
	// Replaces the prior Card.BasePower mutation (which polluted every
	// shared copy of the Card).
	gameengine.RegisterDynamicSetPower(gs, perm, mendicantCountArtifacts,
		gameengine.DurationUntilSourceLeaves, "Mendicant Core, Guidelight", "cda_power")
	gs.InvalidateCharacteristicsCache()
}

func mendicantCountArtifacts(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "artifact") {
			n++
		}
	}
	return n
}

func mendicantBeginCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	// R55: CDA is now a continuous effect; power recomputes on each
	// layer pass. No combat-begin refresh needed. Kept as a no-op for
	// trigger-coverage parity with the pre-R55 registration shape.
	_ = gs
	_ = perm
	_ = ctx
}

func mendicantSpeedTick(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mendicant_speed_tick"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Snapshot opponent life totals for this turn into the perm flags;
	// later opp-life-loss events compared against this snapshot will
	// bump the speed. Heuristic: bump speed once per turn if any
	// opponent currently has less life than the highest seen.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Read prior snapshot, take a new one.
	bumped := false
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller {
			continue
		}
		key := snapshotKey(i)
		prev := perm.Flags[key]
		if prev > 0 && s.Life < prev && !bumped && perm.Flags["speed"] < 4 {
			perm.Flags["speed"]++
			bumped = true
			emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
				"new_speed":   perm.Flags["speed"],
				"opponent":    i,
				"life_change": s.Life - prev,
			})
		}
		perm.Flags[key] = s.Life
	}
}

func mendicantSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mendicant_max_speed_copy_artifact"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["speed"] < 4 {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "artifact") {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"copied": card.DisplayName(),
	})
	// The actual copy + pay-{1} decision is wired in
	// custom_mendicant_core_guidelight.go (R49 batchD); it observes the
	// same spell_cast event and pushes a copy StackItem when at max
	// speed and mana is available.
}

func snapshotKey(seatIdx int) string {
	return "mendicant_life_snap_seat_" + intToA(seatIdx)
}

func intToA(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
