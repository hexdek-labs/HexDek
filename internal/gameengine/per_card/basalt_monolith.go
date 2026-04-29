package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBasaltMonolith wires up Basalt Monolith.
//
// Oracle text:
//
//	Basalt Monolith doesn't untap during your untap step.
//	{T}: Add {C}{C}{C}.
//	{3}: Untap Basalt Monolith.
//
// The canonical cEDH combo piece: with Rings of Brighthearth (copy the
// untap activation for {2}), Basalt Monolith produces infinite colorless
// mana. WITH KINNAN: Basalt produces {C}{C}{C} + Kinnan's extra 1 of
// any = 4 mana → pay {3} to untap → net +1 per loop → infinite.
//
// Batch #3 scope:
//   - OnActivated(0, ...) — {T}: Add {C}{C}{C}. Uses
//     AddManaFromPermanent so Kinnan and other mana-augmenters can
//     trigger off this tap. If an untap_exempt is set by the untap
//     step, we honor it (Basalt doesn't untap during controller's
//     untap step).
//   - OnActivated(1, ...) — {3}: Untap self. Cost is assumed paid.
//
// Note: the stock mana-artifacts path (gameengine/mana_artifacts.go
// -> ApplyArtifactMana) also handles Basalt Monolith via the legacy
// mana-filling loop. Both paths coexist: ApplyArtifactMana is for
// auto-fill during "need mana" pressure; this activated handler is
// for explicit activations by tests + tournament code.
func registerBasaltMonolith(r *Registry) {
	r.OnETB("Basalt Monolith", basaltMonolithETB)
	r.OnActivated("Basalt Monolith", basaltMonolithActivate)
}

func basaltMonolithETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Mark the permanent as "doesn't untap during your untap step".
	// phases.go untap loop consults perm.Flags["skip_untap"] + DoesNotUntap.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["skip_untap"] = 1
	perm.DoesNotUntap = true
	emit(gs, "basalt_monolith_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func basaltMonolithActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	switch abilityIdx {
	case 0:
		// {T}: Add {C}{C}{C}.
		const slug = "basalt_monolith_tap"
		if src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
			return
		}
		src.Tapped = true
		gameengine.AddManaFromPermanent(gs, s, src, "C", 3)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"added":    3,
			"color":    "C",
			"new_pool": s.ManaPool,
		})
	case 1:
		// {3}: Untap. Cost is caller-paid.
		const slug = "basalt_monolith_untap"
		if !src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_untapped", nil)
			return
		}
		src.Tapped = false
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat": seat,
		})
	}
}
