package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLaboratoryManiac wires up Laboratory Maniac.
//
// Oracle text:
//
//	If you would draw a card and your library is empty, you win the
//	game instead.
//
// This is the third leg of the Thoracle-less empty-library wincon
// family (Thassa's Oracle + Consultation/Pact is batch #1; Laboratory
// Maniac completes it for Lab-Man lines like Doomsday + Maniac or
// Demonic Consultation + Maniac + a 0-cost draw).
//
// The actual CR §614 replacement effect lives in
// internal/gameengine/replacement.go (RegisterLaboratoryManiac). It's
// wired into the RegisterReplacementsForPermanent dispatcher, which
// stack.go calls on every ETB. So the replacement itself is already
// live — this per_card handler is a thin ETB observer that:
//
//   - Logs a per_card_handler breadcrumb so the registry smoke test
//     passes and audit tooling can confirm the card is recognized.
//   - Re-asserts the replacement registration (idempotent via handler
//     ID dedup in replacement.RegisterReplacement) as a safety net in
//     case the card is created outside the normal CastSpell path
//     (test fixtures, flicker destinations, token copies, etc.).
func registerLaboratoryManiac(r *Registry) {
	r.OnETB("Laboratory Maniac", laboratoryManiacETB)
}

func laboratoryManiacETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "laboratory_maniac_altwin"
	if gs == nil || perm == nil {
		return
	}
	// Re-register the draw-from-empty alt-win replacement. The replacement
	// dispatcher on stack.go:810 already did this at the normal ETB path;
	// calling it again is harmless — RegisterReplacement dedupes by
	// HandlerID built from (card_name, discriminator, perm_ptr).
	gameengine.RegisterLaboratoryManiac(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"rule":      "614",
		"replaces":  "would_draw_from_empty_library",
		"outcome":   "controller_wins",
	})
}
