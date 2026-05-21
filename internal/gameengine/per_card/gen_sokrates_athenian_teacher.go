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
// R49 stub-batch-E port (defensive utility — conditional self-hexproof):
//
//	The R37 breadcrumb stamped Flags["kw:hexproof"] one-shot at ETB
//	and let tap-state drift. The HasHexproof / target-legality path
//	reads Flags["kw:hexproof"] directly, so a conditional grant must
//	keep the Flag in sync with src.Tapped.
//
//	This port keeps the Flag-driven fast-path (the engine surface
//	HasHexproof reads) and adds the missing toggles:
//	  - permanent_tapped → clear the flag when Sokrates becomes tapped
//	  - upkeep             → re-stamp on every turn upkeep, since the
//	                          untap step that ran moments earlier might
//	                          have untapped Sokrates and the flag was
//	                          cleared by the previous activation
//
//	Defender + Sokratic Dialogue ({T}) activated ability are kept on
//	their existing shapes. The activated ability still clears the
//	hexproof flag inline when paying the tap cost so observers reading
//	Flags between the activation and the next layer recompute see the
//	synced state.
func registerSokratesAthenianTeacher(r *Registry) {
	r.OnETB("Sokrates, Athenian Teacher", sokratesETBStampHexproof)
	r.OnTrigger("Sokrates, Athenian Teacher", "permanent_tapped", sokratesTappedClearHexproof)
	r.OnTrigger("Sokrates, Athenian Teacher", "upkeep", sokratesUpkeepRestampHexproof)
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
	if gs != nil && perm != nil {
		// R54: drop any dialogue damage-replacement closures this
		// Sokrates registered. The flag on the targeted creature is
		// cleared by the EOT flag-sweep regardless of Sokrates'
		// presence (the printed "until end of turn" duration outlives
		// the source per CR §611.2c).
		gs.UnregisterDamageReplacementsForPermanent(perm)
	}
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

func sokratesETBStampHexproof(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sokrates_hexproof_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	sokratesSyncHexproof(perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tapped": perm.Tapped,
	})
}

func sokratesTappedClearHexproof(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sokrates_hexproof_clear_on_tap"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// permanent_tapped is an alias of tap_event; payload typically
	// includes the tapped Permanent. Filter to Sokrates self.
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil {
		// Fall back to ctx["perm"].
		target, _ = ctx["perm"].(*gameengine.Permanent)
	}
	if target != nil && target != perm {
		return
	}
	if perm.Flags != nil {
		delete(perm.Flags, "kw:hexproof")
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func sokratesUpkeepRestampHexproof(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	sokratesSyncHexproof(perm)
}

func sokratesSyncHexproof(perm *gameengine.Permanent) {
	if perm == nil {
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

	// Pay tap cost. Tapping clears the hexproof flag — keep them in sync
	// inline so a same-tick observer reading Flags directly (before the
	// permanent_tapped trigger fires) sees the synced state.
	src.Tapped = true
	if src.Flags != nil {
		delete(src.Flags, "kw:hexproof")
	}
	gs.InvalidateCharacteristicsCache()

	// Stamp the target with the dialogue flag so the damage
	// replacement closure below can detect the convert-damage-to-draws
	// state. Cleaned up by the existing end-of-turn flag-sweep.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["sokrates_dialogue_until_eot"] = 1

	// R54: register a damage replacement keyed to Sokrates the source.
	// Fires whenever the stamped target would deal COMBAT damage to a
	// player — prevents the damage and routes draw-half-rounded-down
	// to both Sokrates' controller and the damaged player. The closure
	// self-no-ops once the dialogue flag clears at EOT (existing
	// flag-sweep). The duplicate-registration guard via HandlerID lets
	// repeat activations (multi-turn lifelink builds) re-tag without
	// double-firing.
	sourceController := src.Controller
	srcName := src.Card.DisplayName()
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: src,
		HandlerID:  "sokrates_dialogue_prevent_and_draw",
		Fn: func(gs *gameengine.GameState, dctx *gameengine.DamageContext) {
			if dctx == nil || dctx.Source == nil {
				return
			}
			if dctx.Kind != gameengine.DamageCombatPlayer {
				return
			}
			if dctx.Source.Flags == nil || dctx.Source.Flags["sokrates_dialogue_until_eot"] != 1 {
				return
			}
			amount := dctx.Amount
			if amount <= 0 {
				return
			}
			half := amount / 2 // round down per oracle
			dctx.Prevented = true
			gs.LogEvent(gameengine.Event{
				Kind:   "damage_prevented",
				Seat:   sourceController,
				Target: dctx.TargetSeat,
				Source: srcName,
				Amount: amount,
				Details: map[string]interface{}{
					"reason": "sokrates_dialogue_convert",
				},
			})
			if half > 0 {
				for i := 0; i < half; i++ {
					drawOne(gs, sourceController, srcName)
				}
				if dctx.TargetSeat != sourceController {
					for i := 0; i < half; i++ {
						drawOne(gs, dctx.TargetSeat, srcName)
					}
				}
			}
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": target.Card.DisplayName(),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"combat-damage→draws conversion needs engine-side replacement effect on the dialogue flag")
}
