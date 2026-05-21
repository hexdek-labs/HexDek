package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSokratesAthenianTeacher wires Sokrates, Athenian Teacher.
//
// Oracle text:
//
//	Defender
//	Sokrates has hexproof as long as it's untapped.
//	Sokratic Dialogue — {T}: Until end of turn, target creature gains
//	"If this creature would deal combat damage to a player, prevent
//	that damage. This creature's controller and that player each draw
//	half that many cards, rounded down."
//
// Implementation:
//   - Defender: AST keyword pipeline.
//   - Hexproof-while-untapped: a continuous effect that adds the
//     hexproof keyword while !src.Tapped. Refreshed on activation
//     since tap state matters; for the runtime fast-path we toggle
//     Flags["kw:hexproof"] from the ETB hook and keep it in sync via
//     a tiny check at activation time. Engine-side proper continuous
//     "while X" predicate would be cleaner; flagged as a partial.
//   - Activated ability ({T}): pick a target creature (heuristic: a
//     friendly creature with high power, since the dialogue trade
//     converts power into draws), tag it with the runtime flag
//     "sokrates_dialogue_until_eot" so the engine combat-damage path
//     can detect the convert-damage-to-draws state. The actual draw
//     conversion happens in combat code we don't have a hook into;
//     we emit a partial breadcrumb. The tap cost is enforced.
func registerSokratesAthenianTeacher(r *Registry) {
	r.OnETB("Sokrates, Athenian Teacher", sokratesETBHexproof)
	r.OnActivated("Sokrates, Athenian Teacher", sokratesDialogue)
	// R49 batch C: re-stamp the hexproof-while-untapped flag at the
	// start of each of our untap steps (cheapest "tap state may have
	// changed" tick), and clear it on LTB so a stolen/exiled Sokrates
	// can't leave stale kw:hexproof on a transient permanent.
	r.OnTrigger("Sokrates, Athenian Teacher", "upkeep_controller", sokratesUpkeepRefreshHexproof)
	r.OnTrigger("Sokrates, Athenian Teacher", "permanent_ltb", sokratesLTBClearHexproof)
}

func sokratesUpkeepRefreshHexproof(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if !perm.Tapped {
		perm.Flags["kw:hexproof"] = 1
	} else {
		delete(perm.Flags, "kw:hexproof")
	}
}

func sokratesLTBClearHexproof(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if perm.Flags != nil {
		delete(perm.Flags, "kw:hexproof")
	}
}

func sokratesETBHexproof(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if !perm.Tapped {
		perm.Flags["kw:hexproof"] = 1
	}
	emitPartial(gs, "sokrates_hexproof_while_untapped", perm.Card.DisplayName(),
		"runtime flag stamped at ETB; tap state changes don't auto-clear hexproof — engine continuous effect with tap-predicate would be cleaner")
}

func sokratesDialogue(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sokrates_dialogue_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}

	// Pick the target. Caller can override via ctx; otherwise prefer a
	// high-power friendly creature so the damage→draws conversion is
	// upside.
	var target *gameengine.Permanent
	if ctx != nil {
		if t, ok := ctx["target_perm"].(*gameengine.Permanent); ok {
			target = t
		}
	}
	if target == nil {
		bestPow := -1
		seat := gs.Seats[src.Controller]
		if seat != nil {
			for _, p := range seat.Battlefield {
				if p == nil || p.Card == nil || !p.IsCreature() {
					continue
				}
				if p.Power() > bestPow {
					bestPow = p.Power()
					target = p
				}
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_creature", nil)
		return
	}

	// Pay tap cost. Tapping clears the hexproof flag — keep them in sync.
	src.Tapped = true
	if src.Flags != nil {
		delete(src.Flags, "kw:hexproof")
	}

	// Stamp the target with the dialogue flag so combat-damage code can
	// detect the convert-damage-to-draws state. Cleaned up by the
	// existing end-of-turn flag-sweep.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["sokrates_dialogue_until_eot"] = 1
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"combat-damage→draws conversion needs engine-side replacement effect on the dialogue flag")
}
