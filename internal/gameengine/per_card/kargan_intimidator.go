package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarganIntimidator wires Kargan Intimidator (Human Warrior).
//
// Oracle text:
//
//	Cowards can't block Warriors.
//	{1}: Choose one that hasn't been chosen this turn —
//	  • This creature gets +1/+1 until end of turn.
//	  • Target creature becomes a Coward until end of turn.
//	  • Target Warrior gains trample until end of turn.
//
// Implementation (reuses existing patterns only — no new primitives):
//   - Static "Cowards can't block Warriors": stamp the shared coward-block
//     marker (Gornog the Red Reaper's seat-level pattern) plus the
//     per-permanent grant flag the combat.go canBlockGS enforcement scans
//     for (mirrors the Sidar Kondo block-restriction wiring).
//   - {1} modal activated ability: the handler self-selects a mode (like
//     Demonic Pact's mode picker), but gated "hasn't been chosen THIS TURN"
//     via a per-permanent turn key (Gornog's gs.Turn+1 marker idiom) so each
//     mode is usable at most once per turn and the slate resets every turn.
//     A caller may force a specific mode via ctx["kargan_mode"]; otherwise the
//     first still-available mode (pump → coward → trample) is taken.
//   - Mode payloads via existing layer primitives:
//     * pump:    until-EOT +1/+1 Modification (the Gornog anthem idiom).
//     * coward:  RegisterAddTypes(..., DurationEndOfTurn) — layer-4 type add.
//     * trample: RegisterGrantKeyword(..., DurationEndOfTurn) — layer-6 grant.
func registerKarganIntimidator(r *Registry) {
	r.OnETB("Kargan Intimidator", karganIntimidatorETB)
	r.OnActivated("Kargan Intimidator", karganIntimidatorActivate)
}

// karganModeOrder is the deterministic pick order for the auto-select path.
var karganModeOrder = []string{"pump", "coward", "trample"}

func karganModeTurnKey(mode string) string { return "kargan_mode_" + mode + "_turn" }

// karganModeAvailable reports whether `mode` is still choosable this turn.
func karganModeAvailable(src *gameengine.Permanent, mode string, turn int) bool {
	if src == nil || src.Flags == nil {
		return true
	}
	// gs.Turn+1 stored so turn 0 isn't confused with the zero value
	// (matches Gornog's turn-key idiom).
	return src.Flags[karganModeTurnKey(mode)] != turn+1
}

func karganIntimidatorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kargan_intimidator_block_restriction"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.MarkCowardsCantBlockWarriors(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func karganIntimidatorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kargan_intimidator_modal"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}

	// Choose the mode: an explicit request (ctx["kargan_mode"]) must still be
	// available this turn; otherwise auto-pick the first available mode.
	mode := ""
	if req, _ := ctx["kargan_mode"].(string); req != "" {
		if !karganModeAvailable(src, req, gs.Turn) {
			return // that mode was already chosen this turn — can't pick it
		}
		mode = req
	} else {
		for _, m := range karganModeOrder {
			if karganModeAvailable(src, m, gs.Turn) {
				mode = m
				break
			}
		}
	}
	if mode == "" {
		return // all three modes already chosen this turn
	}
	src.Flags[karganModeTurnKey(mode)] = gs.Turn + 1

	switch mode {
	case "pump":
		// This creature gets +1/+1 until end of turn.
		src.Modifications = append(src.Modifications, gameengine.Modification{
			Power: 1, Toughness: 1, Duration: "until_end_of_turn", Timestamp: gs.NextTimestamp(),
		})
		gs.InvalidateCharacteristicsCache()
	case "coward":
		// Target creature becomes a Coward until end of turn.
		if tgt := karganTargetCreature(gs, src, ctx); tgt != nil {
			gameengine.RegisterAddTypes(gs, tgt, []string{"coward"},
				gameengine.DurationEndOfTurn, "Kargan Intimidator", "kargan_coward")
			gs.InvalidateCharacteristicsCache()
		}
	case "trample":
		// Target Warrior gains trample until end of turn.
		if tgt := karganTargetWarrior(gs, src, ctx); tgt != nil {
			gameengine.RegisterGrantKeyword(gs, tgt, "trample",
				gameengine.DurationEndOfTurn, "Kargan Intimidator", "kargan_trample")
			gs.InvalidateCharacteristicsCache()
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
		"mode": mode,
	})
}

// karganTargetFromCtx extracts a single permanent target from the resolution
// ctx — either an explicit ctx["target_perm"] (test/AI convenience) or the
// first permanent in ctx["targets"] (the stack-supplied target list).
func karganTargetFromCtx(ctx map[string]interface{}) *gameengine.Permanent {
	if ctx == nil {
		return nil
	}
	if p, _ := ctx["target_perm"].(*gameengine.Permanent); p != nil {
		return p
	}
	if ts, ok := ctx["targets"].([]gameengine.Target); ok {
		for _, t := range ts {
			if t.Permanent != nil {
				return t.Permanent
			}
		}
	}
	return nil
}

// karganTargetCreature returns the chosen "becomes a Coward" target: the
// supplied target if it's a creature, else a deterministic heuristic (an
// opponent's highest-power non-Coward creature — a relevant blocker to shut
// off, mirroring Gornog's coward-tag intent).
func karganTargetCreature(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if t := karganTargetFromCtx(ctx); t != nil && t.IsCreature() {
		return t
	}
	var best *gameengine.Permanent
	bestPow := -1
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == src.Controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if gs.HasTypeOf(p, "coward") {
				continue
			}
			if pw := gs.PowerOf(p); pw > bestPow {
				bestPow = pw
				best = p
			}
		}
	}
	return best
}

// karganTargetWarrior returns the chosen "gains trample" target: the supplied
// target if it's a Warrior you control, else a deterministic heuristic (the
// controller's highest-power Warrior).
func karganTargetWarrior(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if t := karganTargetFromCtx(ctx); t != nil && t.IsCreature() && gs.HasTypeOf(t, "warrior") {
		return t
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !gs.HasTypeOf(p, "warrior") {
			continue
		}
		if pw := gs.PowerOf(p); pw > bestPow {
			bestPow = pw
			best = p
		}
	}
	return best
}
