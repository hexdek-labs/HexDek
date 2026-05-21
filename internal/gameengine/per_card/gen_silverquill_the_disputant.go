package per_card

// registerSilverquillTheDisputant is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real ETB (casualty 1 grant on instant/sorcery spells the
// controller casts, with the drain-and-copy rider) lives in
// custom_silverquill_the_disputant.go and is wired via
// registerSilverquillTheDisputantCustom (registered from
// registry.go).
//
// The auto-gen body emitted a misleading "static abilities handled
// by AST engine" partial breadcrumb on every Silverquill ETB even
// though the custom handler already implements the casualty grant.
// Neutered here (R50 batch G) so the partial channel doesn't surface
// a fake static-AST gap.
func registerSilverquillTheDisputant(r *Registry) {
	_ = r
}
