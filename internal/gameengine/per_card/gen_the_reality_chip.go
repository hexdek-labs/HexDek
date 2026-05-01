package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheRealityChip wires The Reality Chip.
//
// Oracle text:
//
//   You may look at the top card of your library any time.
//   As long as The Reality Chip is attached to a creature, you may play lands and cast spells from the top of your library.
//   Reconfigure {2}{U} ({2}{U}: Attach to target creature you control; or unattach from a creature. Reconfigure only as a sorcery. While attached, this isn't a creature.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheRealityChip(r *Registry) {
	r.OnETB("The Reality Chip", theRealityChipStaticETB)
}

func theRealityChipStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_reality_chip_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
