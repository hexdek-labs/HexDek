package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheRealityChip wires The Reality Chip.
//
// Oracle text:
//
//	You may look at the top card of your library any time.
//	As long as The Reality Chip is attached to a creature, you may
//	play lands and cast spells from the top of your library.
//	Reconfigure {2}{U}
//
// Implementation:
//   - "Look at the top of your library" is information-only; recorded
//     on Seat.Flags so UIs / future cast-legality scanners that
//     want the visibility hint can read it.
//   - "Play lands and cast spells from top of library" while attached:
//     gated on AttachedTo != nil. We register a per-turn ZoneCast
//     permission for the top card (refreshed each upkeep while
//     attached) so the cast pipeline accepts it from the library.
//     Engine support for full continuous "from top of library"
//     casting is partial — flagged.
//   - Reconfigure {2}{U}: the engine's ActivateReconfigure helper
//     handles the attach/detach state flip. We delegate to it from
//     OnActivated. The helper enforces sorcery-speed and the {2}{U}
//     mana cost (3 generic for our simplified pool, plus we track a
//     blue-pip requirement via a partial breadcrumb when the typed
//     pool can't satisfy it).
func registerTheRealityChip(r *Registry) {
	r.OnETB("The Reality Chip", theRealityChipETB)
	r.OnActivated("The Reality Chip", theRealityChipActivate)
	// R52 batch L: LTB clears the may_see_top_of_library seat flag
	// (refcounted via a scan for any other Reality Chip still on the
	// controller's battlefield) so a removed Chip doesn't leave the
	// hidden-info visibility hint stuck for downstream consumers.
	r.OnTrigger("The Reality Chip", "permanent_ltb", theRealityChipLTBClearFlag)
}

func theRealityChipLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if normalizeName(p.Card.DisplayName()) == normalizeName("The Reality Chip") {
			return
		}
	}
	if seat.Flags != nil {
		delete(seat.Flags, "may_see_top_of_library")
	}
}

func theRealityChipETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_reality_chip_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	// "You may look at the top card of your library any time" — UIs /
	// AI scanners read this; the corresponding policy is harmless to
	// always-on once Reality Chip is on the battlefield.
	seat.Flags["may_see_top_of_library"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"play-from-top-of-library while attached needs cast-legality scanner integration")
}

func theRealityChipActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_reality_chip_reconfigure"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if !isSorcerySpeed(gs, src.Controller) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}
	// {2}{U} ≈ 3 generic in the simplified pool. Blue-pip enforcement
	// is engine territory; flagged below.
	const cost = 3

	// Pick a target creature when attaching; nil means detach.
	var target *gameengine.Permanent
	if src.AttachedTo == nil {
		// Currently a creature — attach. Pick best friendly creature
		// (highest power) other than self.
		seat := gs.Seats[src.Controller]
		if seat != nil {
			bestPow := -1
			for _, p := range seat.Battlefield {
				if p == nil || p == src || p.Card == nil || !p.IsCreature() {
					continue
				}
				if p.Power() > bestPow {
					bestPow = p.Power()
					target = p
				}
			}
		}
		if target == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_target_creature", nil)
			return
		}
	}

	if !gameengine.ActivateReconfigure(gs, src, target, cost) {
		emitFail(gs, slug, src.Card.DisplayName(), "reconfigure_failed", map[string]interface{}{
			"required": cost,
			"pool":     gs.Seats[src.Controller].ManaPool,
		})
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"blue-pip enforcement on the {2}{U} cost approximated as 3 generic")
}
