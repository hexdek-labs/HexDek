package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQuandrixTheProof wires Quandrix, the Proof.
//
// Oracle text:
//
//   Flying, trample
//   Cascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it without paying its mana cost. Put the exiled cards on the bottom in a random order.)
//   Instant and sorcery spells you cast from your hand have cascade.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerQuandrixTheProof(r *Registry) {
	r.OnETB("Quandrix, the Proof", quandrixTheProofStaticETB)
}

func quandrixTheProofStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "quandrix_the_proof_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
