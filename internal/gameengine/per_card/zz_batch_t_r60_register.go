package per_card

// Batch T (R60) — 5 high-impact unstubbed tutor handlers.
//
//   - Diabolic Tutor ({2}{B}{B} sorcery — vanilla 4-mana hand tutor)
//   - Eldritch Evolution ({1}{G/P} sorcery — sac creature, tutor +2 CMC to bf, exile self)
//   - Green Sun's Zenith ({X}{G} instant — tutor green creature CMC<=X to bf, shuffle into library)
//   - Buried Alive ({2}{B} sorcery — tutor up to 3 creatures to graveyard)
//   - Final Parting ({3}{B} sorcery — tutor 2 cards: one to hand, one to graveyard)
//
// Pattern mirrors zz_batch_s_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests. All five reuse the shared
// tutorToHand / shuffleLibraryPerCard helpers from tutors.go where
// applicable; the more elaborate cases (Eldritch Evolution sac-cost
// CMC, GSZ shuffle-self into library, Buried Alive multi-pick,
// Final Parting hand+graveyard split) carry their own picker logic.

func init() {
	RegisterBatchTR60(Global())
	AddResetHook(RegisterBatchTR60)
}

// RegisterBatchTR60 registers the batch-T R60 handlers.
func RegisterBatchTR60(r *Registry) {
	if r == nil {
		return
	}
	registerDiabolicTutor(r)
	registerEldritchEvolution(r)
	registerGreenSunsZenith(r)
	registerBuriedAlive(r)
	registerFinalParting(r)
}
