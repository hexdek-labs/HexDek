package per_card

// registerShadowheartDarkJusticiar is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real activated ability ({1}{B}, {T}, sac another creature: draw
// X cards where X = sacrificed creature's power) lives in
// custom_shadowheart_dark_justiciar.go and is wired via
// registerShadowheartDarkJusticiarCustom
// (zz_activated_stubs_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Shadowheart activation even though the custom handler
// already implements the sac-for-cards effect. Neutered here (R50
// batch G) so the partial channel doesn't surface a duplicate
// parser-gap signal.
func registerShadowheartDarkJusticiar(r *Registry) {
	_ = r
}
