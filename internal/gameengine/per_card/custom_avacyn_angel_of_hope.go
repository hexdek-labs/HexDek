package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAvacynAngelOfHopeCustom replaces the auto-generated stub with
// a real implementation of Avacyn's "Other permanents you control have
// indestructible" anthem.
//
// Oracle text:
//
//	Flying, vigilance, indestructible
//	Other permanents you control have indestructible.
//
// Avacyn herself has indestructible via the AST keyword pipeline. The
// anthem grant is wired here as a layer-6 ContinuousEffect with
// DurationUntilSourceLeaves so all of Avacyn's controller's other
// permanents pick up the keyword while she's on the battlefield. The
// engine's HasKeyword path reads chars.Keywords (post-layer resolution)
// AND the runtime "kw:indestructible" flag — we set the flag for the
// fast-path AND append the keyword to chars for engine consistency.
func registerAvacynAngelOfHopeCustom(r *Registry) {
	r.OnETB("Avacyn, Angel of Hope", avacynGrantIndestructible)
}

func avacynGrantIndestructible(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	source := perm
	const grant = "indestructible"
	pred := func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
		if t == nil || t.Card == nil {
			return false
		}
		if t == source {
			return false
		}
		return t.Controller == source.Controller
	}
	apply := func(_ *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
		// chars-level grant for layer-aware consumers when chars supplied.
		if chars != nil {
			already := false
			for _, k := range chars.Keywords {
				if k == grant {
					already = true
					break
				}
			}
			if !already {
				chars.Keywords = append(chars.Keywords, grant)
			}
		}
		// Runtime flag fast-path so SBA / damage-resolution code that
		// short-circuits via Permanent.Flags also picks it up. Runs
		// regardless of whether chars was supplied so the immediate-
		// apply loop below stamps existing permanents.
		if target != nil {
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			gameengine.SetKeywordFlags(gs, target, "indestructible")
		}
	}
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Timestamp:      gs.NextTimestamp(),
		SourcePerm:     source,
		SourceCardName: "Avacyn, Angel of Hope",
		ControllerSeat: source.Controller,
		HandlerID:      "avacyn_indestructible_grant_" + perm.Card.DisplayName(),
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate:      pred,
		ApplyFn:        apply,
	})
	// Immediately stamp existing matching permanents so the runtime
	// IsIndestructible() flag-fast-path picks them up before the next
	// chars recompute.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, t := range s.Battlefield {
			if pred(gs, t) {
				apply(gs, t, nil)
			}
		}
	}
	emit(gs, "avacyn_indestructible_grant_registered", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
