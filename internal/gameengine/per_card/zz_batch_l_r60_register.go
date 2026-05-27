package per_card

// Batch L (R60) — 5 high-impact unstubbed cards.
//
//   - Lotus Petal ({0} mana ability tap+sac for any color)
//   - Strip Mine + Wasteland (land destruction utility lands, sibling
//     handlers sharing the LD-target picker)
//   - Faithless Looting ({R} sorcery draw-2-discard-2)
//   - Dryad of the Ilysian Grove ({2}{G} extra land + all-basic-types)
//   - Captain Sisay ({2}{G}{W} legendary tutor on tap)
//
// Pattern mirrors zz_batch_j_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests.

func init() {
	RegisterBatchLR60(Global())
	AddResetHook(RegisterBatchLR60)
}

// RegisterBatchLR60 registers the batch-L R60 handlers.
func RegisterBatchLR60(r *Registry) {
	if r == nil {
		return
	}
	registerLotusPetal(r)
	registerStripMineFamily(r)
	registerFaithlessLooting(r)
	registerDryadOfTheIlysianGrove(r)
	registerCaptainSisay(r)
}
