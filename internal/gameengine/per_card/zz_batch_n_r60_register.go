package per_card

// Batch N (R60) — 5 high-impact unstubbed cards.
//
//   - Krark-Clan Ironworks ({4} sac-artifact-for-CC mana engine)
//   - Mystic Confluence ({3}{U}{U} modal counter/bounce/draw choose-3)
//   - Treasure Cruise ({7}{U} delve draw-3)
//   - Stifle + Trickbind ({U} counter-target-activated-or-triggered)
//   - Yisan, the Wanderer Bard ({2}{G} verse-counter tutor-to-battlefield)
//
// Pattern mirrors zz_batch_l_r60_register.go: init() registers against
// the global registry and adds a Reset hook so handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchNR60(Global())
	AddResetHook(RegisterBatchNR60)
}

// RegisterBatchNR60 registers the batch-N R60 handlers.
func RegisterBatchNR60(r *Registry) {
	if r == nil {
		return
	}
	registerKrarkClanIronworks(r)
	registerMysticConfluence(r)
	registerTreasureCruise(r)
	registerStifle(r)
	registerYisanTheWandererBard(r)
}
