package per_card

// registerObekaBruteChronologist is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real activated ability ({T}: the player whose turn it is may
// end the turn — stack exile + hand-size discard + damage wear-off +
// UEOT cleanup) lives in custom_obeka_brute_chronologist.go and is
// wired via registerObekaBruteChronologistCustom
// (zz_activated_stubs_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Obeka activation even though the custom handler already
// implements the full end-the-turn sequence. Neutered here (R50
// batch G) so the partial channel doesn't surface a duplicate
// parser-gap signal.
func registerObekaBruteChronologist(r *Registry) {
	_ = r
}
