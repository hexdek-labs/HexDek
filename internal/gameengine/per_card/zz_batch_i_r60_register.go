package per_card

// Batch I (R60) — 5 high-impact unstubbed cards.
//
//   - Esper Sentinel ({W} 1/1 cEDH tax cantrip)
//   - Up the Beanstalk ({1}{G} CMC>=5 value engine)
//   - Grim Tutor ({1}{B}{B} tutor + 3 life)
//   - Scroll Rack ({2} hand-sculpt activator)
//   - Trouble in Pairs ({2}{W}{W} second-event draw engine)
//
// Pattern mirrors zz_r60_register.go: init() registers against the
// global registry and adds a Reset hook so the handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchIR60(Global())
	AddResetHook(RegisterBatchIR60)
}

// RegisterBatchIR60 registers the batch-I R60 handlers.
func RegisterBatchIR60(r *Registry) {
	if r == nil {
		return
	}
	registerEsperSentinel(r)
	registerUpTheBeanstalk(r)
	registerGrimTutor(r)
	registerScrollRack(r)
	registerTroubleInPairs(r)
}
