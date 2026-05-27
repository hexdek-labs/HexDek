package per_card

// Batch P (R60) — 5 high-impact unstubbed cards.
//
//   - Cryptic Command ({1}{U}{U}{U} 4-mode choose-2 flex spell)
//   - Dig Through Time ({6}{U}{U} delve look-7-keep-2)
//   - Demonic Pact ({2}{B}{B} 4-mode upkeep cycle that ends in game loss)
//   - Carpet of Flowers ({G} per-turn ramp scaling with opp Islands)
//   - Praetor's Grasp ({1}{B}{B} opp-library exile-and-cast hate)
//
// Pattern mirrors zz_batch_n_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests.

func init() {
	RegisterBatchPR60(Global())
	AddResetHook(RegisterBatchPR60)
}

// RegisterBatchPR60 registers the batch-P R60 handlers.
func RegisterBatchPR60(r *Registry) {
	if r == nil {
		return
	}
	registerCrypticCommand(r)
	registerDigThroughTime(r)
	registerDemonicPact(r)
	registerCarpetOfFlowers(r)
	registerPraetorsGrasp(r)
}
