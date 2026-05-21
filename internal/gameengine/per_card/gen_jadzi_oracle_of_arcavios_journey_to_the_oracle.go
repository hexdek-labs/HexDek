package per_card

// registerJadziOracleOfArcaviosJourneyToTheOracle is the auto-generated
// entry point retained so the batch_generated.go registry call keeps
// compiling. The real implementation (magecraft top-of-library reveal
// + Discard-a-card bounce activation) lives in
// custom_jadzi_oracle_of_arcavios.go and is wired via
// registerJadziOracleOfArcaviosCustom (registered from registry.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Jadzi activation even though the custom handler already
// implemented both lines of oracle text. Neutered here (R49 batch C)
// so the partial channel reflects the real (custom-side) gap, not a
// duplicate parser-gap signal.
func registerJadziOracleOfArcaviosJourneyToTheOracle(r *Registry) {
	_ = r
}
