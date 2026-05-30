package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDocksideExtortionist wires up Dockside Extortionist.
//
// Oracle text:
//
//	When Dockside Extortionist enters the battlefield, create X
//	Treasure tokens, where X is the number of artifacts and
//	enchantments your opponents control.
//
// A 2-mana red creature that often generates 3-6+ mana of treasures in
// cEDH pods, especially when opponents have signets/rocks/Rhystic Study
// etc. on the battlefield. Single-card combo enabler when paired with
// Deadeye Navigator / Displacer Kitten / Cloudstone Curio (flicker for
// repeat treasure ETB).
//
// Batch #2 scope:
//   - OnETB: count opponent artifacts + enchantments. Create N treasure
//     tokens. The token generator is already implemented in
//     gameengine/cast_counts.go (createTreasureToken). We call it N
//     times.
func registerDocksideExtortionist(r *Registry) {
	r.OnETB("Dockside Extortionist", docksideExtortionistETB)
}

func docksideExtortionistETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dockside_extortionist_treasure_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Count artifacts + enchantments controlled by opponents.
	x := 0
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		for _, p := range os.Battlefield {
			if p == nil {
				continue
			}
			if p.IsArtifact() || p.IsEnchantment() {
				x++
			}
		}
	}
	// Create X Treasure tokens via the canonical engine helper. Routes
	// through MintTokenInstanceID + bumps Turn.TreasuresCreated /
	// Turn.TokensCreated / Turn.ArtifactsEntered for downstream
	// observers (treasure-payoff cards, artifact-ETB counters).
	for i := 0; i < x; i++ {
		gameengine.CreateTreasureToken(gs, seat)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"x":               x,
		"treasures_made":  x,
		"opponents_count": len(gs.Opponents(seat)),
	})
}
