package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMahaItsFeathersNight wires Maha, Its Feathers Night.
//
// Oracle text:
//
//	Flying, trample
//	Ward—Discard a card.
//	Creatures your opponents control have base toughness 1.
//
// R49 stub-batch-E port (defensive utility — mass-toughness removal):
//
//	The R37 breadcrumb stamped a `maha_base_tough_one_active` flag on
//	gs.Flags with no actual layer-7 consumer. This R49 port replaces
//	the breadcrumb with a real RegisterContinuousEffect at layer 7b
//	(§613.4b base-PT SET) whose predicate matches opponents'
//	creatures and whose ApplyFn writes BaseToughness=1.
//
//	Mirrors Kudo's layer-7b SET base 2/2 (gen_kudo_king_among_bears.go).
//	SourcePerm-scoped so UnregisterContinuousEffectsForPermanent on
//	LTB tears down automatically; we keep the legacy gs.Flags
//	breadcrumb intact for any downstream observer that still reads it.
//
//	Flying / trample / Ward: AST keyword pipeline.
func registerMahaItsFeathersNight(r *Registry) {
	r.OnETB("Maha, Its Feathers Night", mahaETBRegisterBaseTough)
}

func mahaETBRegisterBaseTough(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "maha_base_toughness_one"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	src := perm
	pred := func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
		if t == nil || t.Card == nil {
			return false
		}
		if t == src {
			return false
		}
		if !t.IsCreature() {
			return false
		}
		return t.Controller != src.Controller
	}
	ts := perm.Timestamp
	suffix := strconv.Itoa(ts)
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "b",
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Maha, Its Feathers Night",
		ControllerSeat: perm.Controller,
		HandlerID:      "Maha, Its Feathers Night:base_tough_1:" + suffix,
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate:      pred,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			if chars == nil {
				return
			}
			chars.Toughness = 1
			chars.BaseToughness = 1
		},
	})
	// Keep the legacy seat-keyed breadcrumb flag so any downstream
	// consumer still reading it doesn't regress; the real work now lives
	// in the continuous effect above.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["maha_base_tough_one_active"] = perm.Controller + 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"layer":          "7b",
		"base_toughness": 1,
	})
}

// ClearMahaBaseToughness is the legacy reset (mostly for tests + for
// hypothetical board-wipe-then-recast scenarios) that drops the
// Maha base-toughness-1 breadcrumb flag. The real continuous effect
// auto-clears via UnregisterContinuousEffectsForPermanent.
func ClearMahaBaseToughness(gs *gameengine.GameState) {
	if gs == nil || gs.Flags == nil {
		return
	}
	delete(gs.Flags, "maha_base_tough_one_active")
}

// mahaETBSetBaseToughnessFlag is the R37 legacy name; R49 redirects it
// to the new registration path so existing tests keep compiling.
func mahaETBSetBaseToughnessFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	mahaETBRegisterBaseTough(gs, perm)
}

// mahaLTBClearBaseToughnessFlag is the R37 legacy name. The R49
// continuous-effect-driven path auto-clears via
// UnregisterContinuousEffectsForPermanent on LTB; this shim still
// drops the legacy breadcrumb flag for tests that explicitly call it.
func mahaLTBClearBaseToughnessFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || gs.Flags == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	delete(gs.Flags, "maha_base_tough_one_active")
}
