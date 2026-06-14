package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPakoArcaneRetriever wires Pako, Arcane Retriever.
//
// Oracle text:
//
//	Partner with Haldan, Avid Arcanist
//	Haste
//	Whenever Pako attacks, exile the top card of each player's library
//	and put a fetch counter on each of them. Put a +1/+1 counter on
//	Pako for each noncreature card exiled this way.
func registerPakoArcaneRetriever(r *Registry) {
	r.OnTrigger("Pako, Arcane Retriever", "attacks", pakoAttacks)
}

func pakoAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pako_attacks_exile"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever ~ attacks" fires only when this creature
	// itself is the attacker. The creature_attacks event is dispatched once per
	// declared attacker, so the old seat check (ctx["seat"] is never populated by
	// the attack fire → it only matched seat 0) let the effect multiply per
	// attacker. Mirrors the canonical gate and the Aang and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	noncreature := 0
	exiled := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || len(s.Library) == 0 {
			continue
		}
		c := s.Library[0]
		gameengine.MoveCard(gs, c, i, "library", "exile", "pako_fetch")
		exiled++
		if c != nil && !cardHasType(c, "creature") {
			noncreature++
		}
	}
	if noncreature > 0 {
		perm.AddCounter("+1/+1", noncreature)
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"exiled":      exiled,
		"noncreature": noncreature,
	})
}
