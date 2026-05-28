package per_card

// Batch S (R60) — 5 high-impact unstubbed cards.
//
//   - Beast Within ({2}{G} instant — destroy any permanent + 3/3 token)
//   - Time Warp ({3}{U}{U} sorcery — target player takes extra turn)
//   - Karmic Guide ({3}{W}{W} 2/2 Angel — ETB graveyard-to-battlefield)
//   - Fellwar Stone ({2} artifact — opp-color mana rock)
//   - Anguished Unmaking ({1}{W}{B} instant — exile nonland, lose 3 life)
//
// Pattern mirrors zz_batch_p_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests.

func init() {
	RegisterBatchSR60(Global())
	AddResetHook(RegisterBatchSR60)
}

// RegisterBatchSR60 registers the batch-S R60 handlers.
func RegisterBatchSR60(r *Registry) {
	if r == nil {
		return
	}
	registerBeastWithin(r)
	registerTimeWarp(r)
	registerKarmicGuide(r)
	registerFellwarStone(r)
	registerAnguishedUnmaking(r)
}
