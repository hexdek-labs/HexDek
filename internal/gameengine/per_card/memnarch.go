package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Memnarch — {7} 4/5 Legendary Artifact Creature — Sphinx Golem.
//
//   {1}{U}{U}: Target permanent becomes an artifact in addition to its
//   other types. (This effect lasts indefinitely.)
//   {3}{U}: Gain control of target artifact. (This effect lasts
//   indefinitely.)
//
// Implementation:
//   - Ability 0: append "artifact" to target.Card.Types if not already
//     present (Clavileño type-add pattern at clavileno_first_blessed.go:64).
//     Per the parenthetical "(This effect lasts indefinitely)" — the
//     type stays appended permanently, which matches the Clavileño
//     pattern semantics.
//   - Ability 1: control-grab. Indefinite-duration control change is
//     engine continuous-effect territory; emitPartial.

func init() {
	registerMemnarch(Global())
	AddResetHook(registerMemnarch)
}

func registerMemnarch(r *Registry) {
	r.OnActivated("Memnarch", memnarchActivate)
}

func memnarchActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		memnarchAddArtifactType(gs, src, ctx)
	case 1:
		emitPartial(gs, "memnarch_gain_control_artifact", src.Card.DisplayName(),
			"indefinite_duration_control_change_needs_engine_continuous_effect")
	}
}

func memnarchAddArtifactType(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "memnarch_becomes_artifact"
	if ctx == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	if cardHasType(target.Card, "artifact") {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"target_card":   target.Card.DisplayName(),
			"already_artifact": true,
		})
		return
	}
	target.Card.Types = append(target.Card.Types, "artifact")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"target_card": target.Card.DisplayName(),
		"target_seat": target.Controller,
	})
}
