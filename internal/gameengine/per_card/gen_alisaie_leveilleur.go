package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlisaieLeveilleur wires Alisaie Leveilleur.
//
// Oracle text (Scryfall, verified):
//
//	Partner with Alphinaud Leveilleur
//	First strike
//	Dualcast — The second spell you cast each turn costs {2} less
//	to cast.
//
// Implementation (R44 stub port):
//   - First strike: AST keyword pipeline.
//   - Partner with: outside-the-game tutor on ETB; per_card layer
//     would need an Alphinaud library probe — emitPartial breadcrumb
//     here since the partner-tutor surface isn't currently exposed.
//   - Dualcast cost reduction: engine-side via the new
//     "Alisaie Leveilleur" case in cost_modifiers.go ScanCostModifiers.
//     Discount {2} when seat.Turn.SpellsCast == 1 (the spell being
//     cost-scanned is about to be the 2nd cast — SpellsCast increments
//     post-cast in cast_counts.go).
//   - The register hook here is a breadcrumb so the registration-
//     coverage lint stays satisfied; the engine surface owns the math.
func registerAlisaieLeveilleur(r *Registry) {
	r.OnETB("Alisaie Leveilleur", alisaieLeveilleurETB)
}

func alisaieLeveilleurETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "alisaie_leveilleur_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"partner_with_alphinaud_tutor_surface_not_wired_at_per_card_layer")
}
