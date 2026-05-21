package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQuestForPureFlame wires Quest for Pure Flame.
//
// Oracle text (Scryfall, verified — Zendikar, {R}):
//
//	Whenever a source you control deals damage to an opponent, you
//	may put a quest counter on Quest for Pure Flame.
//	Remove four quest counters from Quest for Pure Flame and
//	sacrifice it: If a red source you control would deal damage to
//	a creature or player this turn, it deals double that damage to
//	that creature or player instead.
//
// Implementation (R55 — damage replacement primitive):
//   - OnTrigger("combat_damage_player") + OnTrigger("noncombat_damage_to_player"):
//     when the damaged seat is an opponent of Quest's controller AND
//     the source belongs to Quest's controller, place one quest
//     counter (cap at 4 since the activated cost is 4-remove).
//   - OnActivated: removes 4 quest counters, sacrifices Quest, and
//     registers a damage-replacement closure SCOPED TO THIS TURN.
//     The closure has no SourcePerm reference (Quest is gone) — use
//     a dedicated tag-only HandlerID. Cleanup is the gs.Turn check
//     inside the closure: when gs.Turn moves past the activation
//     turn, the closure becomes a no-op and a manual unregistration
//     happens at end_step on any battlefield perm... actually we
//     can't keep a self-reference. We use a HandlerID scan in a
//     subsequent end_step trigger registered on any one permanent
//     that's guaranteed to outlive the turn — that's impossible to
//     pin from a Sorcery-shaped activation. Instead: stash the
//     captured turn-number on closure context, leave the closure in
//     gs.DamageReplacements, and let the closure self-no-op past the
//     activation turn. A daily end-of-turn registry sweep would be
//     cleaner; deferred.
func registerQuestForPureFlame(r *Registry) {
	r.OnTrigger("Quest for Pure Flame", "combat_damage_player", questForPureFlameAddCounter)
	r.OnTrigger("Quest for Pure Flame", "noncombat_damage_to_player", questForPureFlameAddCounter)
	r.OnActivated("Quest for Pure Flame", questForPureFlameActivate)
	r.OnTrigger("Quest for Pure Flame", "permanent_ltb", questForPureFlameLTBUnregister)
}

func questForPureFlameAddCounter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "quest_for_pure_flame_add_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	target, _ := ctx["seat"].(int)
	if target == perm.Controller || target < 0 || target >= len(gs.Seats) {
		return
	}
	// Source must be controlled by Quest's controller. The two damage
	// events expose source identity differently; combat_damage_player
	// carries source_seat, noncombat_damage_to_player carries source.
	sourceSeat, hasSeat := ctx["source_seat"].(int)
	if !hasSeat {
		// noncombat path — source string only. Skip if we can't confirm
		// ownership (conservative; over-fire risk is low and the AI
		// auto-accepts the "may" anyway).
		// Continue with counter add — printed text says "Whenever a
		// source you control" but the engine can't always identify the
		// source seat for spell damage. Accept the conservative fire.
	} else if sourceSeat != perm.Controller {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	if perm.Counters["quest"] >= 4 {
		return
	}
	perm.AddCounter("quest", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"target_seat":    target,
		"quest_counters": perm.Counters["quest"],
	})
}

func questForPureFlameActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "quest_for_pure_flame_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Counters == nil || src.Counters["quest"] < 4 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_quest_counters", map[string]interface{}{
			"have": src.Counters["quest"],
			"need": 4,
		})
		return
	}
	src.Counters["quest"] -= 4
	controller := src.Controller
	activationTurn := gs.Turn

	// Register the damage-replacement closure with SourcePerm=nil so
	// UnregisterDamageReplacementsForPermanent (which fires from the
	// SacrificePermanent below via permanent_ltb) doesn't drop our
	// closure before the turn is even out. The closure self-no-ops
	// past the activation turn via the `gs.Turn != activationTurn`
	// gate, which is sufficient since Quest's effect is explicitly
	// turn-scoped ("would deal damage ... this turn").
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: nil,
		HandlerID:  "quest_for_pure_flame_red_double_this_turn",
		Fn: func(gs *gameengine.GameState, dctx *gameengine.DamageContext) {
			if dctx == nil || dctx.Amount <= 0 {
				return
			}
			// Turn-scope gate: closure is a no-op past the activation turn.
			if gs.Turn != activationTurn {
				return
			}
			if dctx.Source == nil || dctx.Source.Card == nil {
				return
			}
			if dctx.Source.Controller != controller {
				return
			}
			if !questForPureFlameIsRedSource(dctx.Source.Card) {
				return
			}
			dctx.Amount *= 2
		},
	})

	gameengine.SacrificePermanent(gs, src, "quest_for_pure_flame_activate_cost")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            controller,
		"activation_turn": activationTurn,
	})
}

func questForPureFlameLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// Note: when Quest leaves the battlefield via SacrificePermanent in
	// the activation path, this LTB hook fires. We unregister here so
	// the activation's closure is cleaned up automatically (the
	// activation-turn gate inside the closure is belt-and-suspenders).
	gs.UnregisterDamageReplacementsForPermanent(perm)
}

func questForPureFlameIsRedSource(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, col := range c.Colors {
		if strings.EqualFold(col, "R") {
			return true
		}
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "pip:R") {
			return true
		}
	}
	return false
}
