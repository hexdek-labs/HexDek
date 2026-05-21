package per_card

// registerPhenaxGodOfDeception is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real implementation (granted "{T}: target player mills X" mill
// activation on every creature the controller controls) lives in
// custom_phenax_god_of_deception.go and is wired via
// registerPhenaxGodOfDeceptionCustom
// (zz_activated_stubs_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Phenax activation even though the custom handler already
// implements the granted mill ability. Neutered here (R50 batch G)
// so the partial channel doesn't surface a duplicate parser-gap
// signal. Indestructible + god-creature toggle are AST-keyword
// pipeline / devotion-checker territory and remain there.
func registerPhenaxGodOfDeception(r *Registry) {
	_ = r
}
