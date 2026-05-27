package per_card

// Batch J (R60) — 5 high-impact unstubbed cards.
//
//   - Counterspell ({U}{U} the OG counter)
//   - Skullclamp ({1} equipment + dies-draw-2)
//   - Solemn Simulacrum ({4} ETB ramp + dies-draw)
//   - Hullbreacher ({2}{U} opp-extra-draw → treasure replacement)
//   - Pyroblast / Red Elemental Blast ({R} modal blue hoser)
//
// Pattern mirrors zz_batch_i_r60_register.go: init() registers against
// the global registry and adds a Reset hook so the handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchJR60(Global())
	AddResetHook(RegisterBatchJR60)
}

// RegisterBatchJR60 registers the batch-J R60 handlers.
func RegisterBatchJR60(r *Registry) {
	if r == nil {
		return
	}
	registerCounterspell(r)
	registerSkullclamp(r)
	registerSolemnSimulacrum(r)
	registerHullbreacher(r)
	registerPyroblast(r)
}
