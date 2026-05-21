package per_card

// registerGyrudaDoomOfDepths is the auto-generated entry point retained
// so the batch_generated.go registry call keeps compiling. The real ETB
// (each player mills four; reanimate an even-MV creature from the
// milled cards) lives in custom_gyruda_doom_of_depths.go and is wired
// via registerGyrudaDoomOfDepthsCustom (zz_handler_q45_register.go).
//
// The auto-gen body emitted TWO misleading partial breadcrumbs on every
// Gyruda ETB even though the custom handler already implemented the
// mill-and-reanimate. Neutered here (R49 batch C) so the partial
// channel doesn't surface duplicate parser-gap signals.
func registerGyrudaDoomOfDepths(r *Registry) {
	_ = r
}
