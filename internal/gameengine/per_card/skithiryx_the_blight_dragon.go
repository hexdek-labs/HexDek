package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSkithiryxTheBlightDragon wires Skithiryx, the Blight Dragon.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying
//	Infect
//	{B}: Skithiryx gains haste until end of turn.
//	{B}{B}: Regenerate Skithiryx.
//
// Implementation:
//   - Flying — handled by the AST keyword pipeline.
//   - Infect — handled by the AST keyword pipeline.
//   - abilityIdx 0 ({B}): check controller has at least 1 mana, spend it,
//     grant kw:haste flag + clear SummoningSick, register a next_end_step
//     delayed trigger to remove kw:haste (unless Skithiryx had haste
//     intrinsically via the AST, in which case the flag deletion is a no-op).
//   - abilityIdx 1 ({B}{B}): check controller has at least 2 mana, spend it,
//     set regeneration_shield flag on Skithiryx. The engine's SBA loop
//     checks this flag before destroying a creature; if set, the destroy is
//     replaced by tap + remove all damage + remove from combat per CR §701.15.
//     The flag is consumed (cleared) on redemption by the SBA.
//
// Note: the hat's activated-ability evaluator will inspect abilityIdx 0 and 1
// to decide when paying {B} for haste (e.g., when attacking is beneficial this
// turn) or {B}{B} for regeneration (e.g., when a board wipe is on the stack)
// is correct. Both abilities are cost-gated on ManaPool.
func registerSkithiryxTheBlightDragon(r *Registry) {
	r.OnActivated("Skithiryx, the Blight Dragon", skithiryxActivate)
}

func skithiryxActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		skithiryxGrantHaste(gs, src)
	case 1:
		skithiryxRegenerate(gs, src)
	}
}

// skithiryxGrantHaste implements {B}: Skithiryx gains haste until end of turn.
func skithiryxGrantHaste(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "skithiryx_b_haste"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Require {B} — at least 1 generic mana in pool.
	const cost = 1
	if s.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seat,
			"required":  cost,
			"available": s.ManaPool,
		})
		return
	}
	s.ManaPool -= cost
	gameengine.SyncManaAfterSpend(s)

	// Grant haste and clear summoning sickness.
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	alreadyHasted := src.Flags["kw:haste"] == 1
	src.Flags["kw:haste"] = 1
	src.SummoningSick = false

	// Note: Flying and Infect are AST keywords — no per-card handling needed.
	emitPartial(gs, slug, src.Card.DisplayName(),
		"flying_and_infect_handled_by_ast_keyword_pipeline")

	// Schedule end-of-turn cleanup. We only remove the flag if we were
	// the ones to add it (i.e., it wasn't already set by the AST).
	if !alreadyHasted {
		captured := src
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: seat,
			SourceCardName: src.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				if captured == nil || captured.Flags == nil {
					return
				}
				delete(captured.Flags, "kw:haste")
			},
		})
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"cost_paid": cost,
		"haste":     true,
	})
}

// skithiryxRegenerate implements {B}{B}: Regenerate Skithiryx.
func skithiryxRegenerate(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "skithiryx_bb_regenerate"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Require {B}{B} — at least 2 generic mana in pool.
	const cost = 2
	if s.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      seat,
			"required":  cost,
			"available": s.ManaPool,
		})
		return
	}
	s.ManaPool -= cost
	gameengine.SyncManaAfterSpend(s)

	// Set regeneration shield. The SBA loop checks Flags["regeneration_shield"]
	// before destroying a creature and applies CR §701.15 replacement:
	// tap, remove all damage, remove from combat.
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["regeneration_shield"] = 1

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":                seat,
		"cost_paid":           cost,
		"regeneration_shield": true,
	})
}
